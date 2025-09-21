package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/env-data-platform/internal/database"
	"github.com/env-data-platform/internal/models"
	"github.com/env-data-platform/internal/services"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// QualityHandler 数据质量处理器
type QualityHandler struct {
	db      *gorm.DB
	logger  *zap.Logger
	checker *services.QualityChecker
}

// NewQualityHandler 创建数据质量处理器
func NewQualityHandler(logger *zap.Logger) *QualityHandler {
	return &QualityHandler{
		db:      database.GetDB(),
		logger:  logger,
		checker: services.NewQualityChecker(logger),
	}
}

// ListQualityRules 获取数据质量规则列表
func (h *QualityHandler) ListQualityRules(c *gin.Context) {
	var req struct {
		Page         int    `form:"page" binding:"required,min=1"`
		PageSize     int    `form:"page_size" binding:"required,min=1,max=100"`
		Name         string `form:"name"`
		Type         string `form:"type"`
		DataSourceID uint   `form:"data_source_id"`
		ETLJobID     uint   `form:"etl_job_id"`
		IsEnabled    *bool  `form:"is_enabled"`
	}

	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "参数错误"))
		return
	}

	query := h.db.Model(&models.QualityRule{})

	if req.Name != "" {
		query = query.Where("name LIKE ?", "%"+req.Name+"%")
	}
	if req.Type != "" {
		query = query.Where("type = ?", req.Type)
	}
	if req.DataSourceID > 0 {
		query = query.Where("data_source_id = ?", req.DataSourceID)
	}
	if req.ETLJobID > 0 {
		query = query.Where("etl_job_id = ?", req.ETLJobID)
	}
	if req.IsEnabled != nil {
		query = query.Where("is_enabled = ?", *req.IsEnabled)
	}

	var total int64
	query.Count(&total)

	var rules []models.QualityRule
	offset := (req.Page - 1) * req.PageSize
	if err := query.Offset(offset).Limit(req.PageSize).
		Preload("DataSource").Preload("ETLJob").Preload("Creator").
		Order("created_at DESC").
		Find(&rules).Error; err != nil {
		h.logger.Error("Failed to list quality rules", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询失败"))
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse(gin.H{
		"list":      rules,
		"total":     total,
		"page":      req.Page,
		"page_size": req.PageSize,
	}))
}

// CreateQualityRule 创建数据质量规则
func (h *QualityHandler) CreateQualityRule(c *gin.Context) {
	var req struct {
		Name         string                 `json:"name" binding:"required,min=1,max=100"`
		Description  string                 `json:"description"`
		Type         string                 `json:"type" binding:"required,oneof=completeness uniqueness validity consistency accuracy freshness"`
		DataSourceID uint                   `json:"data_source_id"`
		ETLJobID     uint                   `json:"etl_job_id"`
		TargetTable  string                 `json:"target_table"`
		ColumnName   string                 `json:"column_name"`
		Config       map[string]interface{} `json:"config"`
		Threshold    float64                `json:"threshold" binding:"min=0,max=100"`
		IsEnabled    bool                   `json:"is_enabled"`
		Priority     int                    `json:"priority"`
		AlertLevel   string                 `json:"alert_level" binding:"required,oneof=info warning critical fatal"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "参数错误"))
		return
	}

	userID := c.GetUint("user_id")

	// 验证数据源是否存在
	if req.DataSourceID > 0 {
		var dataSourceExists bool
		h.db.Model(&models.DataSource{}).Where("id = ?", req.DataSourceID).Select("count(*) > 0").Find(&dataSourceExists)
		if !dataSourceExists {
			c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "数据源不存在"))
			return
		}
	}

	// 验证ETL作业是否存在
	if req.ETLJobID > 0 {
		var etlJobExists bool
		h.db.Model(&models.ETLJob{}).Where("id = ?", req.ETLJobID).Select("count(*) > 0").Find(&etlJobExists)
		if !etlJobExists {
			c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "ETL作业不存在"))
			return
		}
	}

	// 序列化配置
	configBytes, err := json.Marshal(req.Config)
	if err != nil {
		h.logger.Error("Failed to marshal rule config", zap.Error(err))
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "配置格式错误"))
		return
	}

	// 创建质量规则
	rule := models.QualityRule{
		Name:         req.Name,
		Description:  req.Description,
		Type:         req.Type,
		DataSourceID: req.DataSourceID,
		ETLJobID:     req.ETLJobID,
		TargetTable:  req.TargetTable,
		ColumnName:   req.ColumnName,
		RuleConfig:   string(configBytes),
		Config:       configBytes,
		Threshold:    req.Threshold,
		IsEnabled:    req.IsEnabled,
		Priority:     req.Priority,
		AlertLevel:   req.AlertLevel,
	}
	rule.CreatedBy = userID
	rule.UpdatedBy = userID

	if err := h.db.Create(&rule).Error; err != nil {
		h.logger.Error("Failed to create quality rule", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "创建失败"))
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse(rule))
}

// GetQualityRule 获取数据质量规则详情
func (h *QualityHandler) GetQualityRule(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "无效的ID"))
		return
	}

	var rule models.QualityRule
	if err := h.db.Preload("DataSource").Preload("ETLJob").Preload("Creator").
		First(&rule, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, models.ErrorResponse(http.StatusNotFound, "质量规则不存在"))
			return
		}
		h.logger.Error("Failed to get quality rule", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询失败"))
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse(rule))
}

// UpdateQualityRule 更新数据质量规则
func (h *QualityHandler) UpdateQualityRule(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "无效的ID"))
		return
	}

	var req struct {
		Name         string                 `json:"name" binding:"required,min=1,max=100"`
		Description  string                 `json:"description"`
		Type         string                 `json:"type" binding:"required,oneof=completeness uniqueness validity consistency accuracy freshness"`
		DataSourceID uint                   `json:"data_source_id"`
		ETLJobID     uint                   `json:"etl_job_id"`
		TargetTable  string                 `json:"target_table"`
		ColumnName   string                 `json:"column_name"`
		Config       map[string]interface{} `json:"config"`
		Threshold    float64                `json:"threshold" binding:"min=0,max=100"`
		IsEnabled    bool                   `json:"is_enabled"`
		Priority     int                    `json:"priority"`
		AlertLevel   string                 `json:"alert_level" binding:"required,oneof=info warning critical fatal"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "参数错误"))
		return
	}

	var rule models.QualityRule
	if err := h.db.First(&rule, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, models.ErrorResponse(http.StatusNotFound, "质量规则不存在"))
			return
		}
		h.logger.Error("Failed to get quality rule", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询失败"))
		return
	}

	// 验证数据源是否存在
	if req.DataSourceID > 0 {
		var dataSourceExists bool
		h.db.Model(&models.DataSource{}).Where("id = ?", req.DataSourceID).Select("count(*) > 0").Find(&dataSourceExists)
		if !dataSourceExists {
			c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "数据源不存在"))
			return
		}
	}

	// 验证ETL作业是否存在
	if req.ETLJobID > 0 {
		var etlJobExists bool
		h.db.Model(&models.ETLJob{}).Where("id = ?", req.ETLJobID).Select("count(*) > 0").Find(&etlJobExists)
		if !etlJobExists {
			c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "ETL作业不存在"))
			return
		}
	}

	// 序列化配置
	configBytes, err := json.Marshal(req.Config)
	if err != nil {
		h.logger.Error("Failed to marshal rule config", zap.Error(err))
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "配置格式错误"))
		return
	}

	// 更新规则
	updates := map[string]interface{}{
		"name":           req.Name,
		"description":    req.Description,
		"type":           req.Type,
		"data_source_id": req.DataSourceID,
		"etl_job_id":     req.ETLJobID,
		"target_table":   req.TargetTable,
		"column_name":    req.ColumnName,
		"rule_config":    string(configBytes),
		"threshold":      req.Threshold,
		"is_enabled":     req.IsEnabled,
		"priority":       req.Priority,
		"alert_level":    req.AlertLevel,
		"updated_by":     c.GetUint("user_id"),
	}

	if err := h.db.Model(&rule).Updates(updates).Error; err != nil {
		h.logger.Error("Failed to update quality rule", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "更新失败"))
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse(rule))
}

// DeleteQualityRule 删除数据质量规则
func (h *QualityHandler) DeleteQualityRule(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "无效的ID"))
		return
	}

	var rule models.QualityRule
	if err := h.db.First(&rule, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, models.ErrorResponse(http.StatusNotFound, "质量规则不存在"))
			return
		}
		h.logger.Error("Failed to get quality rule", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询失败"))
		return
	}

	// 检查是否有关联的质量报告
	var reportCount int64
	h.db.Model(&models.QualityReport{}).Where("rule_id = ?", id).Count(&reportCount)
	if reportCount > 0 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "该规则有关联的质量报告，无法删除"))
		return
	}

	if err := h.db.Delete(&rule).Error; err != nil {
		h.logger.Error("Failed to delete quality rule", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "删除失败"))
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse(gin.H{"message": "删除成功"}))
}

// ExecuteQualityCheck 执行数据质量检查
func (h *QualityHandler) ExecuteQualityCheck(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "无效的ID"))
		return
	}

	var rule models.QualityRule
	if err := h.db.Preload("DataSource").First(&rule, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, models.ErrorResponse(http.StatusNotFound, "质量规则不存在"))
			return
		}
		h.logger.Error("Failed to get quality rule", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询失败"))
		return
	}

	if !rule.IsEnabled {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "质量规则已禁用"))
		return
	}

	// 执行质量检查
	ctx := context.Background()
	report, err := h.checker.ExecuteQualityCheck(ctx, &rule)
	if err != nil {
		h.logger.Error("Failed to execute quality check", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "执行质量检查失败"))
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse(gin.H{
		"rule":   rule,
		"report": report,
	}))
}

// ListQualityReports 获取数据质量报告列表
func (h *QualityHandler) ListQualityReports(c *gin.Context) {
	var req struct {
		Page      int    `form:"page" binding:"required,min=1"`
		PageSize  int    `form:"page_size" binding:"required,min=1,max=100"`
		RuleID    uint   `form:"rule_id"`
		Status    string `form:"status"`
		StartDate string `form:"start_date"`
		EndDate   string `form:"end_date"`
	}

	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "参数错误"))
		return
	}

	query := h.db.Model(&models.QualityReport{})

	if req.RuleID > 0 {
		query = query.Where("rule_id = ?", req.RuleID)
	}
	if req.Status != "" {
		query = query.Where("status = ?", req.Status)
	}
	if req.StartDate != "" {
		query = query.Where("check_time >= ?", req.StartDate)
	}
	if req.EndDate != "" {
		query = query.Where("check_time <= ?", req.EndDate)
	}

	var total int64
	query.Count(&total)

	var reports []models.QualityReport
	offset := (req.Page - 1) * req.PageSize
	if err := query.Offset(offset).Limit(req.PageSize).
		Preload("Rule").
		Order("check_time DESC").
		Find(&reports).Error; err != nil {
		h.logger.Error("Failed to list quality reports", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询失败"))
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse(gin.H{
		"list":      reports,
		"total":     total,
		"page":      req.Page,
		"page_size": req.PageSize,
	}))
}

// GetQualityReport 获取数据质量报告详情
func (h *QualityHandler) GetQualityReport(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "无效的ID"))
		return
	}

	var report models.QualityReport
	if err := h.db.Preload("Rule").Preload("Rule.DataSource").
		First(&report, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, models.ErrorResponse(http.StatusNotFound, "质量报告不存在"))
			return
		}
		h.logger.Error("Failed to get quality report", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询失败"))
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse(report))
}

// GetQualityStats 获取数据质量统计信息
func (h *QualityHandler) GetQualityStats(c *gin.Context) {
	var stats struct {
		TotalRules      int64   `json:"total_rules"`
		ActiveRules     int64   `json:"active_rules"`
		TotalReports    int64   `json:"total_reports"`
		TodayReports    int64   `json:"today_reports"`
		AverageScore    float64 `json:"average_score"`
		PassRate        float64 `json:"pass_rate"`
		RulesByType     []struct {
			Type  string `json:"type"`
			Count int64  `json:"count"`
		} `json:"rules_by_type"`
		ScoreDistribution []struct {
			Range string `json:"range"`
			Count int64  `json:"count"`
		} `json:"score_distribution"`
	}

	// 总规则数
	h.db.Model(&models.QualityRule{}).Count(&stats.TotalRules)

	// 活跃规则数
	h.db.Model(&models.QualityRule{}).Where("is_enabled = ?", true).Count(&stats.ActiveRules)

	// 总报告数
	h.db.Model(&models.QualityReport{}).Count(&stats.TotalReports)

	// 今日报告数
	h.db.Model(&models.QualityReport{}).Where("DATE(check_time) = CURDATE()").Count(&stats.TodayReports)

	// 平均分数
	h.db.Model(&models.QualityReport{}).Select("AVG(score)").Scan(&stats.AverageScore)

	// 通过率（分数>=80的报告占比）
	var passCount int64
	h.db.Model(&models.QualityReport{}).Where("score >= ?", 80).Count(&passCount)
	if stats.TotalReports > 0 {
		stats.PassRate = float64(passCount) / float64(stats.TotalReports) * 100
	}

	// 按类型统计规则
	h.db.Model(&models.QualityRule{}).
		Select("type, COUNT(*) as count").
		Group("type").
		Scan(&stats.RulesByType)

	// 分数分布统计
	scoreRanges := []struct {
		Range string
		Min   float64
		Max   float64
	}{
		{"0-20", 0, 20},
		{"21-40", 21, 40},
		{"41-60", 41, 60},
		{"61-80", 61, 80},
		{"81-100", 81, 100},
	}

	for _, sr := range scoreRanges {
		var count int64
		h.db.Model(&models.QualityReport{}).
			Where("score >= ? AND score <= ?", sr.Min, sr.Max).
			Count(&count)
		stats.ScoreDistribution = append(stats.ScoreDistribution, struct {
			Range string `json:"range"`
			Count int64  `json:"count"`
		}{
			Range: sr.Range,
			Count: count,
		})
	}

	c.JSON(http.StatusOK, models.SuccessResponse(stats))
}

// BatchExecuteQualityCheck 批量执行数据质量检查
func (h *QualityHandler) BatchExecuteQualityCheck(c *gin.Context) {
	var req struct {
		RuleIDs       []uint `json:"rule_ids" binding:"required,min=1"`
		DataSourceID  uint   `json:"data_source_id"`
		OnlyEnabled   bool   `json:"only_enabled"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "参数错误"))
		return
	}

	query := h.db.Model(&models.QualityRule{}).Where("id IN ?", req.RuleIDs)

	if req.DataSourceID > 0 {
		query = query.Where("data_source_id = ?", req.DataSourceID)
	}
	if req.OnlyEnabled {
		query = query.Where("is_enabled = ?", true)
	}

	var rules []models.QualityRule
	if err := query.Preload("DataSource").Find(&rules).Error; err != nil {
		h.logger.Error("Failed to get quality rules", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询规则失败"))
		return
	}

	if len(rules) == 0 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "未找到符合条件的规则"))
		return
	}

	// 异步执行批量检查
	go h.executeBatchQualityCheck(rules)

	c.JSON(http.StatusOK, models.SuccessResponse(gin.H{
		"message":    "批量质量检查已启动",
		"rule_count": len(rules),
	}))
}

// executeBatchQualityCheck 执行批量质量检查
func (h *QualityHandler) executeBatchQualityCheck(rules []models.QualityRule) {
	ctx := context.Background()

	for _, rule := range rules {
		if !rule.IsEnabled {
			continue
		}

		report, err := h.checker.ExecuteQualityCheck(ctx, &rule)
		if err != nil {
			h.logger.Error("Failed to execute quality check in batch",
				zap.Uint("rule_id", rule.ID),
				zap.String("rule_name", rule.Name),
				zap.Error(err))
			continue
		}

		h.logger.Info("Quality check completed in batch",
			zap.Uint("rule_id", rule.ID),
			zap.String("rule_name", rule.Name),
			zap.String("status", report.Status),
			zap.Float64("score", report.Score))
	}
}