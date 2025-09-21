package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/env-data-platform/internal/database"
	"github.com/env-data-platform/internal/models"
	"github.com/env-data-platform/internal/services"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ETLHandler ETL处理器
type ETLHandler struct {
	db         *gorm.DB
	logger     *zap.Logger
	scheduler  *services.ETLScheduler
	executor   *services.ETLExecutor
}

// NewETLHandler 创建ETL处理器
func NewETLHandler(logger *zap.Logger) *ETLHandler {
	return &ETLHandler{
		db:        database.GetDB(),
		logger:    logger,
		scheduler: services.NewETLScheduler(logger),
		executor:  services.NewETLExecutor(logger),
	}
}

// ListETLJobs 获取ETL作业列表
func (h *ETLHandler) ListETLJobs(c *gin.Context) {
	var req struct {
		Page     int    `form:"page" binding:"required,min=1"`
		PageSize int    `form:"page_size" binding:"required,min=1,max=100"`
		Name     string `form:"name"`
		Status   string `form:"status"`
		SourceID uint   `form:"source_id"`
		TargetID uint   `form:"target_id"`
	}

	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "参数错误"))
		return
	}

	query := h.db.Model(&models.ETLJob{})

	if req.Name != "" {
		query = query.Where("name LIKE ?", "%"+req.Name+"%")
	}
	if req.Status != "" {
		query = query.Where("status = ?", req.Status)
	}
	if req.SourceID > 0 {
		query = query.Where("source_id = ?", req.SourceID)
	}
	if req.TargetID > 0 {
		query = query.Where("target_id = ?", req.TargetID)
	}

	var total int64
	query.Count(&total)

	var jobs []models.ETLJob
	offset := (req.Page - 1) * req.PageSize
	if err := query.Offset(offset).Limit(req.PageSize).
		Preload("Source").Preload("Target").Preload("Creator").
		Order("created_at DESC").
		Find(&jobs).Error; err != nil {
		h.logger.Error("Failed to list ETL jobs", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询失败"))
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse(gin.H{
		"list":      jobs,
		"total":     total,
		"page":      req.Page,
		"page_size": req.PageSize,
	}))
}

// CreateETLJob 创建ETL作业
func (h *ETLHandler) CreateETLJob(c *gin.Context) {
	var req models.ETLJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "参数错误"))
		return
	}

	userID := c.GetUint("user_id")

	// 验证数据源是否存在
	var sourceExists, targetExists bool
	h.db.Model(&models.DataSource{}).Where("id = ?", req.SourceID).Select("count(*) > 0").Find(&sourceExists)
	if !sourceExists {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "源数据源不存在"))
		return
	}

	if req.TargetID > 0 {
		h.db.Model(&models.DataSource{}).Where("id = ?", req.TargetID).Select("count(*) > 0").Find(&targetExists)
		if !targetExists {
			c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "目标数据源不存在"))
			return
		}
	}

	// 创建ETL作业
	job := models.ETLJob{
		Name:        req.Name,
		Description: req.Description,
		SourceID:    req.SourceID,
		TargetID:    req.TargetID,
		CronExpr:    req.CronExpr,
		Status:      "created",
		IsEnabled:   req.IsEnabled,
		Priority:    req.Priority,
		MaxRetries:  req.MaxRetries,
		Timeout:     req.Timeout,
	}
	job.CreatedBy = userID
	job.UpdatedBy = userID

	// 设置配置数据
	if err := job.SetConfig(req.Config); err != nil {
		h.logger.Error("Failed to set job config", zap.Error(err))
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "配置格式错误"))
		return
	}

	if err := h.db.Create(&job).Error; err != nil {
		h.logger.Error("Failed to create ETL job", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "创建失败"))
		return
	}

	// 如果作业启用并且有cron表达式，则添加到调度器
	if job.IsEnabled && job.CronExpr != "" {
		if err := h.scheduler.ScheduleJob(&job); err != nil {
			h.logger.Warn("Failed to schedule job", zap.Error(err), zap.Uint("job_id", job.ID))
		}
	}

	c.JSON(http.StatusOK, models.SuccessResponse(job))
}

// GetETLJob 获取ETL作业详情
func (h *ETLHandler) GetETLJob(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "无效的ID"))
		return
	}

	var job models.ETLJob
	if err := h.db.Preload("Source").Preload("Target").Preload("Creator").
		First(&job, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, models.ErrorResponse(http.StatusNotFound, "ETL作业不存在"))
			return
		}
		h.logger.Error("Failed to get ETL job", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询失败"))
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse(job))
}

// UpdateETLJob 更新ETL作业
func (h *ETLHandler) UpdateETLJob(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "无效的ID"))
		return
	}

	var req models.ETLJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "参数错误"))
		return
	}

	var job models.ETLJob
	if err := h.db.First(&job, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, models.ErrorResponse(http.StatusNotFound, "ETL作业不存在"))
			return
		}
		h.logger.Error("Failed to get ETL job", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询失败"))
		return
	}

	// 如果作业正在运行，不允许修改关键配置
	if job.Status == "running" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "作业正在运行，无法修改"))
		return
	}

	// 验证数据源是否存在
	var sourceExists, targetExists bool
	h.db.Model(&models.DataSource{}).Where("id = ?", req.SourceID).Select("count(*) > 0").Find(&sourceExists)
	if !sourceExists {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "源数据源不存在"))
		return
	}

	if req.TargetID > 0 {
		h.db.Model(&models.DataSource{}).Where("id = ?", req.TargetID).Select("count(*) > 0").Find(&targetExists)
		if !targetExists {
			c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "目标数据源不存在"))
			return
		}
	}

	// 设置配置数据
	if err := job.SetConfig(req.Config); err != nil {
		h.logger.Error("Failed to set job config", zap.Error(err))
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "配置格式错误"))
		return
	}

	// 更新作业信息
	updates := map[string]interface{}{
		"name":         req.Name,
		"description":  req.Description,
		"source_id":    req.SourceID,
		"target_id":    req.TargetID,
		"config_data":  job.ConfigData,
		"cron_expr":    req.CronExpr,
		"is_enabled":   req.IsEnabled,
		"priority":     req.Priority,
		"max_retries":  req.MaxRetries,
		"timeout":      req.Timeout,
		"updated_by":   c.GetUint("user_id"),
	}

	if err := h.db.Model(&job).Updates(updates).Error; err != nil {
		h.logger.Error("Failed to update ETL job", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "更新失败"))
		return
	}

	// 重新调度作业
	h.scheduler.UnscheduleJob(uint(id))
	if req.IsEnabled && req.CronExpr != "" {
		if err := h.scheduler.ScheduleJob(&job); err != nil {
			h.logger.Warn("Failed to reschedule job", zap.Error(err), zap.Uint("job_id", job.ID))
		}
	}

	c.JSON(http.StatusOK, models.SuccessResponse(job))
}

// DeleteETLJob 删除ETL作业
func (h *ETLHandler) DeleteETLJob(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "无效的ID"))
		return
	}

	var job models.ETLJob
	if err := h.db.First(&job, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, models.ErrorResponse(http.StatusNotFound, "ETL作业不存在"))
			return
		}
		h.logger.Error("Failed to get ETL job", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询失败"))
		return
	}

	// 如果作业正在运行，不允许删除
	if job.Status == "running" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "作业正在运行，无法删除"))
		return
	}

	// 检查是否有执行记录
	var executionCount int64
	h.db.Model(&models.ETLExecution{}).Where("job_id = ?", id).Count(&executionCount)
	if executionCount > 0 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "作业有执行记录，无法删除"))
		return
	}

	// 从调度器中移除
	h.scheduler.UnscheduleJob(uint(id))

	if err := h.db.Delete(&job).Error; err != nil {
		h.logger.Error("Failed to delete ETL job", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "删除失败"))
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse(gin.H{"message": "删除成功"}))
}

// ExecuteETLJob 执行ETL作业
func (h *ETLHandler) ExecuteETLJob(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "无效的ID"))
		return
	}

	var req struct {
		TriggerType string                 `json:"trigger_type" binding:"required,oneof=manual schedule api"`
		Parameters  map[string]interface{} `json:"parameters"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "参数错误"))
		return
	}

	var job models.ETLJob
	if err := h.db.Preload("Source").Preload("Target").First(&job, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, models.ErrorResponse(http.StatusNotFound, "ETL作业不存在"))
			return
		}
		h.logger.Error("Failed to get ETL job", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询失败"))
		return
	}

	if !job.IsEnabled {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "作业已禁用"))
		return
	}

	if job.Status == "running" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "作业正在运行"))
		return
	}

	// 创建执行记录
	execution := models.ETLExecution{
		JobID:       job.ID,
		ExecutionID: generateExecutionID(),
		Status:      "running",
		StartTime:   time.Now(),
		TriggerType: req.TriggerType,
		TriggerBy:   c.GetUint("user_id"),
	}

	if err := h.db.Create(&execution).Error; err != nil {
		h.logger.Error("Failed to create execution record", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "创建执行记录失败"))
		return
	}

	// 更新作业状态
	h.db.Model(&job).Updates(map[string]interface{}{
		"status":       "running",
		"last_run_at":  gorm.Expr("NOW()"),
		"run_count":    gorm.Expr("run_count + 1"),
	})

	// 异步执行ETL作业
	go h.executeJobAsync(&job, &execution, req.Parameters)

	c.JSON(http.StatusOK, models.SuccessResponse(gin.H{
		"execution_id": execution.ExecutionID,
		"message":      "作业已启动",
	}))
}

// StopETLJob 停止ETL作业
func (h *ETLHandler) StopETLJob(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "无效的ID"))
		return
	}

	var job models.ETLJob
	if err := h.db.First(&job, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, models.ErrorResponse(http.StatusNotFound, "ETL作业不存在"))
			return
		}
		h.logger.Error("Failed to get ETL job", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询失败"))
		return
	}

	if job.Status != "running" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "作业未在运行"))
		return
	}

	// 通知执行器停止作业
	if err := h.executor.StopJob(job.ID); err != nil {
		h.logger.Error("Failed to stop job", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "停止作业失败"))
		return
	}

	// 更新作业状态
	h.db.Model(&job).Update("status", "stopped")

	// 更新当前执行记录
	now := time.Now()
	h.db.Model(&models.ETLExecution{}).
		Where("job_id = ? AND status = 'running'", job.ID).
		Updates(map[string]interface{}{
			"status":        "cancelled",
			"end_time":      now,
			"error_message": "手动停止",
		})

	c.JSON(http.StatusOK, models.SuccessResponse(gin.H{"message": "作业已停止"}))
}

// ListETLExecutions 获取ETL执行记录列表
func (h *ETLHandler) ListETLExecutions(c *gin.Context) {
	var req struct {
		Page        int    `form:"page" binding:"required,min=1"`
		PageSize    int    `form:"page_size" binding:"required,min=1,max=100"`
		JobID       uint   `form:"job_id"`
		Status      string `form:"status"`
		TriggerType string `form:"trigger_type"`
		StartDate   string `form:"start_date"`
		EndDate     string `form:"end_date"`
	}

	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "参数错误"))
		return
	}

	query := h.db.Model(&models.ETLExecution{})

	if req.JobID > 0 {
		query = query.Where("job_id = ?", req.JobID)
	}
	if req.Status != "" {
		query = query.Where("status = ?", req.Status)
	}
	if req.TriggerType != "" {
		query = query.Where("trigger_type = ?", req.TriggerType)
	}
	if req.StartDate != "" {
		query = query.Where("start_time >= ?", req.StartDate)
	}
	if req.EndDate != "" {
		query = query.Where("start_time <= ?", req.EndDate)
	}

	var total int64
	query.Count(&total)

	var executions []models.ETLExecution
	offset := (req.Page - 1) * req.PageSize
	if err := query.Offset(offset).Limit(req.PageSize).
		Preload("Job").Preload("Trigger").
		Order("start_time DESC").
		Find(&executions).Error; err != nil {
		h.logger.Error("Failed to list ETL executions", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询失败"))
		return
	}

	// 计算执行时长
	for i := range executions {
		executions[i].CalculateDuration()
	}

	c.JSON(http.StatusOK, models.SuccessResponse(gin.H{
		"list":      executions,
		"total":     total,
		"page":      req.Page,
		"page_size": req.PageSize,
	}))
}

// GetETLExecution 获取ETL执行记录详情
func (h *ETLHandler) GetETLExecution(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "无效的ID"))
		return
	}

	var execution models.ETLExecution
	if err := h.db.Preload("Job").Preload("Trigger").
		First(&execution, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, models.ErrorResponse(http.StatusNotFound, "执行记录不存在"))
			return
		}
		h.logger.Error("Failed to get ETL execution", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询失败"))
		return
	}

	execution.CalculateDuration()

	c.JSON(http.StatusOK, models.SuccessResponse(execution))
}

// GetETLExecutionLogs 获取ETL执行日志
func (h *ETLHandler) GetETLExecutionLogs(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "无效的ID"))
		return
	}

	var execution models.ETLExecution
	if err := h.db.Select("log_content").First(&execution, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, models.ErrorResponse(http.StatusNotFound, "执行记录不存在"))
			return
		}
		h.logger.Error("Failed to get ETL execution logs", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询失败"))
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse(gin.H{
		"logs": execution.LogContent,
	}))
}

// ListETLTemplates 获取ETL模板列表
func (h *ETLHandler) ListETLTemplates(c *gin.Context) {
	var req struct {
		Page     int    `form:"page" binding:"required,min=1"`
		PageSize int    `form:"page_size" binding:"required,min=1,max=100"`
		Category string `form:"category"`
		IsPublic *bool  `form:"is_public"`
	}

	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "参数错误"))
		return
	}

	query := h.db.Model(&models.ETLTemplate{})

	if req.Category != "" {
		query = query.Where("category = ?", req.Category)
	}
	if req.IsPublic != nil {
		query = query.Where("is_public = ?", *req.IsPublic)
	}

	var total int64
	query.Count(&total)

	var templates []models.ETLTemplate
	offset := (req.Page - 1) * req.PageSize
	if err := query.Offset(offset).Limit(req.PageSize).
		Preload("Creator").
		Order("use_count DESC, created_at DESC").
		Find(&templates).Error; err != nil {
		h.logger.Error("Failed to list ETL templates", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询失败"))
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse(gin.H{
		"list":      templates,
		"total":     total,
		"page":      req.Page,
		"page_size": req.PageSize,
	}))
}

// GetETLStats 获取ETL统计信息
func (h *ETLHandler) GetETLStats(c *gin.Context) {
	var stats models.ETLExecutionStats

	// 总作业数
	h.db.Model(&models.ETLJob{}).Count(&stats.TotalJobs)

	// 活跃作业数
	h.db.Model(&models.ETLJob{}).Where("is_enabled = ? AND status != ?", true, "error").Count(&stats.ActiveJobs)

	// 总执行次数
	h.db.Model(&models.ETLExecution{}).Count(&stats.TotalExecutions)

	// 运行中的执行
	h.db.Model(&models.ETLExecution{}).Where("status = ?", "running").Count(&stats.RunningExecutions)

	// 今日执行次数
	h.db.Model(&models.ETLExecution{}).Where("DATE(start_time) = CURDATE()").Count(&stats.TodayExecutions)

	// 成功率计算
	var successCount int64
	h.db.Model(&models.ETLExecution{}).Where("status = ?", "success").Count(&successCount)
	if stats.TotalExecutions > 0 {
		stats.SuccessRate = float64(successCount) / float64(stats.TotalExecutions) * 100
	}

	c.JSON(http.StatusOK, models.SuccessResponse(stats))
}

// 辅助函数

// generateExecutionID 生成执行ID
func generateExecutionID() string {
	return "exec_" + strconv.FormatInt(time.Now().UnixNano(), 36)
}

// executeJobAsync 异步执行ETL作业
func (h *ETLHandler) executeJobAsync(job *models.ETLJob, execution *models.ETLExecution, parameters map[string]interface{}) {
	ctx := context.Background()
	result := h.executor.ExecuteJob(ctx, job, execution, parameters)

	// 更新执行记录
	endTime := time.Now()
	updates := map[string]interface{}{
		"status":     result.Status,
		"end_time":   endTime,
		"duration":   endTime.Sub(execution.StartTime).Milliseconds(),
		"input_rows": result.InputRows,
		"output_rows": result.OutputRows,
		"error_rows": result.ErrorRows,
		"error_message": result.ErrorMessage,
		"log_content": result.LogContent,
	}

	h.db.Model(execution).Updates(updates)

	// 更新作业统计
	jobUpdates := map[string]interface{}{
		"status": "idle",
	}

	if result.Status == "success" {
		jobUpdates["success_count"] = gorm.Expr("success_count + 1")
	} else {
		jobUpdates["failure_count"] = gorm.Expr("failure_count + 1")
	}

	h.db.Model(job).Updates(jobUpdates)

	h.logger.Info("ETL job execution completed",
		zap.Uint("job_id", job.ID),
		zap.String("execution_id", execution.ExecutionID),
		zap.String("status", result.Status),
		zap.Int64("duration_ms", endTime.Sub(execution.StartTime).Milliseconds()),
	)
}

// CreateETLTemplateRequest 创建ETL模板请求
type CreateETLTemplateRequest struct {
	Name        string   `json:"name" binding:"required"`
	Description string   `json:"description"`
	Category    string   `json:"category" binding:"required"`
	TemplateXML string   `json:"template_xml" binding:"required"`
	Version     string   `json:"version"`
	Preview     string   `json:"preview"`
	Tags        []string `json:"tags"`
}

// UpdateETLTemplateRequest 更新ETL模板请求
type UpdateETLTemplateRequest struct {
	Name        *string   `json:"name,omitempty"`
	Description *string   `json:"description,omitempty"`
	Category    *string   `json:"category,omitempty"`
	TemplateXML *string   `json:"template_xml,omitempty"`
	Version     *string   `json:"version,omitempty"`
	Preview     *string   `json:"preview,omitempty"`
	Tags        *[]string `json:"tags,omitempty"`
	IsActive    *bool     `json:"is_active,omitempty"`
}

// CreateJobFromTemplateRequest 从模板创建作业请求
type CreateJobFromTemplateRequest struct {
	Name        string            `json:"name" binding:"required"`
	Description string            `json:"description"`
	SourceID    uint              `json:"source_id" binding:"required"`
	TargetID    uint              `json:"target_id"`
	Schedule    string            `json:"schedule"`
	Variables   map[string]string `json:"variables"`
}

// CreateETLTemplate 创建ETL模板
func (h *ETLHandler) CreateETLTemplate(c *gin.Context) {
	var req CreateETLTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "请求参数错误"))
		return
	}

	// 验证模板XML
	if req.TemplateXML == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "模板XML不能为空"))
		return
	}

	template := models.ETLTemplate{
		Name:        req.Name,
		Description: req.Description,
		Category:    req.Category,
		TemplateXML: req.TemplateXML,
		Version:     req.Version,
		Preview:     req.Preview,
		IsPublic:    true,
		Tags:        strings.Join(req.Tags, ","),
	}

	if err := database.DB.Create(&template).Error; err != nil {
		h.logger.Error("Failed to create ETL template", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "创建模板失败"))
		return
	}

	h.logger.Info("ETL template created successfully", zap.Uint("template_id", template.ID))
	c.JSON(http.StatusCreated, models.SuccessResponse(template))
}

// GetETLTemplate 获取ETL模板详情
func (h *ETLHandler) GetETLTemplate(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "无效的ID"))
		return
	}

	var template models.ETLTemplate
	if err := database.DB.Where("id = ?", id).First(&template).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, models.ErrorResponse(http.StatusNotFound, "模板不存在"))
		} else {
			h.logger.Error("Failed to get ETL template", zap.Error(err))
			c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询失败"))
		}
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse(template))
}

// UpdateETLTemplate 更新ETL模板
func (h *ETLHandler) UpdateETLTemplate(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "无效的ID"))
		return
	}

	var req UpdateETLTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "请求参数错误"))
		return
	}

	var template models.ETLTemplate
	if err := database.DB.Where("id = ?", id).First(&template).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, models.ErrorResponse(http.StatusNotFound, "模板不存在"))
		} else {
			h.logger.Error("Failed to find ETL template", zap.Error(err))
			c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询失败"))
		}
		return
	}

	// 更新字段
	if req.Name != nil {
		template.Name = *req.Name
	}
	if req.Description != nil {
		template.Description = *req.Description
	}
	if req.Category != nil {
		template.Category = *req.Category
	}
	if req.TemplateXML != nil {
		template.TemplateXML = *req.TemplateXML
	}
	if req.Version != nil {
		template.Version = *req.Version
	}
	if req.Preview != nil {
		template.Preview = *req.Preview
	}
	if req.IsActive != nil {
		template.IsPublic = *req.IsActive
	}
	if req.Tags != nil {
		template.Tags = strings.Join(*req.Tags, ",")
	}

	if err := database.DB.Save(&template).Error; err != nil {
		h.logger.Error("Failed to update ETL template", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "更新失败"))
		return
	}

	h.logger.Info("ETL template updated successfully", zap.Uint("template_id", template.ID))
	c.JSON(http.StatusOK, models.SuccessResponse(template))
}

// DeleteETLTemplate 删除ETL模板
func (h *ETLHandler) DeleteETLTemplate(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "无效的ID"))
		return
	}

	// 检查是否有作业使用此模板
	var jobCount int64
	if err := database.DB.Model(&models.ETLJob{}).Where("template_id = ?", id).Count(&jobCount).Error; err != nil {
		h.logger.Error("Failed to check template usage", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "检查失败"))
		return
	}

	if jobCount > 0 {
		c.JSON(http.StatusConflict, models.ErrorResponse(http.StatusConflict, "模板正在被作业使用，无法删除"))
		return
	}

	if err := database.DB.Delete(&models.ETLTemplate{}, id).Error; err != nil {
		h.logger.Error("Failed to delete ETL template", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "删除失败"))
		return
	}

	h.logger.Info("ETL template deleted successfully", zap.Uint64("template_id", id))
	c.JSON(http.StatusOK, models.SuccessResponse(nil))
}

// CreateJobFromTemplate 从模板创建作业
func (h *ETLHandler) CreateJobFromTemplate(c *gin.Context) {
	templateID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "无效的模板ID"))
		return
	}

	var req CreateJobFromTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "请求参数错误"))
		return
	}

	// 获取模板
	var template models.ETLTemplate
	if err := database.DB.Where("id = ? AND is_active = ?", templateID, true).First(&template).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, models.ErrorResponse(http.StatusNotFound, "模板不存在或已禁用"))
		} else {
			h.logger.Error("Failed to get ETL template", zap.Error(err))
			c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询失败"))
		}
		return
	}

	// 使用模板XML并应用变量替换
	templateXML := template.TemplateXML
	for key, value := range req.Variables {
		// 简单的变量替换，实际应该使用模板引擎
		templateXML = strings.ReplaceAll(templateXML, "{{"+key+"}}", value)
	}

	// 创建作业
	job := models.ETLJob{
		Name:        req.Name,
		Description: req.Description,
		SourceID:    req.SourceID,
		TargetID:    req.TargetID,
		PipelineXML: templateXML,
		CronExpr:    req.Schedule,
		IsEnabled:   true,
		Status:      models.ETLStatusCreated,
	}

	if err := database.DB.Create(&job).Error; err != nil {
		h.logger.Error("Failed to create ETL job from template", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "创建作业失败"))
		return
	}

	h.logger.Info("ETL job created from template successfully",
		zap.Uint("job_id", job.ID),
		zap.Uint("template_id", template.ID))

	c.JSON(http.StatusCreated, models.SuccessResponse(job))
}

// validateTemplateConfig 验证模板配置
func (h *ETLHandler) validateTemplateConfig(config json.RawMessage) error {
	var configData map[string]interface{}
	if err := json.Unmarshal(config, &configData); err != nil {
		return fmt.Errorf("配置格式错误: %v", err)
	}

	// 基本配置验证
	if _, ok := configData["type"]; !ok {
		return fmt.Errorf("缺少配置类型")
	}

	if _, ok := configData["source"]; !ok {
		return fmt.Errorf("缺少数据源配置")
	}

	if _, ok := configData["target"]; !ok {
		return fmt.Errorf("缺少目标配置")
	}

	return nil
}
