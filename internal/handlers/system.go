package handlers

import (
	"net/http"
	"runtime"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/env-data-platform/internal/database"
	"github.com/env-data-platform/internal/models"
)

// SystemHandler 系统处理器
type SystemHandler struct {
	logger *zap.Logger
}

// NewSystemHandler 创建系统处理器
func NewSystemHandler(logger *zap.Logger) *SystemHandler {
	return &SystemHandler{
		logger: logger,
	}
}

// SystemInfo 系统信息
type SystemInfo struct {
	Version      string    `json:"version"`
	BuildTime    string    `json:"build_time"`
	GoVersion    string    `json:"go_version"`
	Platform     string    `json:"platform"`
	StartTime    time.Time `json:"start_time"`
	Uptime       string    `json:"uptime"`
	MemoryUsage  int64     `json:"memory_usage"`
	CPUCount     int       `json:"cpu_count"`
	DatabaseInfo struct {
		Connected bool   `json:"connected"`
		Version   string `json:"version"`
	} `json:"database_info"`
}

// SystemStats 系统统计信息
type SystemStats struct {
	TotalUsers        int64 `json:"total_users"`
	ActiveUsers       int64 `json:"active_users"`
	TotalDataSources  int64 `json:"total_data_sources"`
	ActiveDataSources int64 `json:"active_data_sources"`
	TotalETLJobs      int64 `json:"total_etl_jobs"`
	RunningETLJobs    int64 `json:"running_etl_jobs"`
	QualityRules      int64 `json:"quality_rules"`
	TodayOperations   int64 `json:"today_operations"`
	TodayLogins       int64 `json:"today_logins"`
}

// OperationLogQuery 操作日志查询参数
type OperationLogQuery struct {
	models.PaginationQuery
	UserID     *uint      `form:"user_id"`
	Action     *string    `form:"action"`
	Resource   *string    `form:"resource"`
	StartTime  *time.Time `form:"start_time" time_format:"2006-01-02 15:04:05"`
	EndTime    *time.Time `form:"end_time" time_format:"2006-01-02 15:04:05"`
	IPAddress  *string    `form:"ip_address"`
	StatusCode *int       `form:"status_code"`
}

// LoginLogQuery 登录日志查询参数
type LoginLogQuery struct {
	models.PaginationQuery
	UserID    *uint      `form:"user_id"`
	Username  *string    `form:"username"`
	StartTime *time.Time `form:"start_time" time_format:"2006-01-02 15:04:05"`
	EndTime   *time.Time `form:"end_time" time_format:"2006-01-02 15:04:05"`
	IPAddress *string    `form:"ip_address"`
	Status    *string    `form:"status"`
}

var startTime = time.Now()

// GetSystemInfo 获取系统信息
// @Summary 获取系统信息
// @Description 获取系统版本、运行时间、资源使用等信息
// @Tags 系统管理
// @Produce json
// @Security BearerAuth
// @Success 200 {object} models.Response{data=SystemInfo} "获取成功"
// @Router /api/v1/system/info [get]
func (h *SystemHandler) GetSystemInfo(c *gin.Context) {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// 检查数据库连接
	dbConnected := true
	dbVersion := "Unknown"
	if sqlDB, err := database.DB.DB(); err == nil {
		if err := sqlDB.Ping(); err == nil {
			// 获取数据库版本
			var version string
			if err := database.DB.Raw("SELECT VERSION()").Scan(&version).Error; err == nil {
				dbVersion = version
			}
		} else {
			dbConnected = false
		}
	} else {
		dbConnected = false
	}

	info := SystemInfo{
		Version:     "1.0.0",
		BuildTime:   "2024-01-01 00:00:00",
		GoVersion:   runtime.Version(),
		Platform:    runtime.GOOS + "/" + runtime.GOARCH,
		StartTime:   startTime,
		Uptime:      time.Since(startTime).String(),
		MemoryUsage: int64(memStats.Alloc),
		CPUCount:    runtime.NumCPU(),
	}

	info.DatabaseInfo.Connected = dbConnected
	info.DatabaseInfo.Version = dbVersion

	c.JSON(http.StatusOK, models.SuccessResponse(info))
}

// GetSystemStats 获取系统统计信息
// @Summary 获取系统统计信息
// @Description 获取用户数、数据源数、ETL作业数等统计信息
// @Tags 系统管理
// @Produce json
// @Security BearerAuth
// @Success 200 {object} models.Response{data=SystemStats} "获取成功"
// @Router /api/v1/system/stats [get]
func (h *SystemHandler) GetSystemStats(c *gin.Context) {
	var stats SystemStats

	// 用户统计
	database.DB.Model(&models.User{}).Count(&stats.TotalUsers)
	database.DB.Model(&models.User{}).Where("status = ?", models.UserStatusActive).Count(&stats.ActiveUsers)

	// 数据源统计
	database.DB.Model(&models.DataSource{}).Count(&stats.TotalDataSources)
	database.DB.Model(&models.DataSource{}).Where("status = 'active'").Count(&stats.ActiveDataSources)

	// ETL作业统计
	database.DB.Model(&models.ETLJob{}).Count(&stats.TotalETLJobs)
	database.DB.Model(&models.ETLJob{}).Where("status = 'running'").Count(&stats.RunningETLJobs)

	// 质量规则统计
	database.DB.Model(&models.QualityRule{}).Count(&stats.QualityRules)

	// 今日操作和登录统计
	today := time.Now().Format("2006-01-02")
	database.DB.Model(&models.OperationLog{}).Where("DATE(created_at) = ?", today).Count(&stats.TodayOperations)
	database.DB.Model(&models.LoginLog{}).Where("DATE(created_at) = ?", today).Count(&stats.TodayLogins)

	c.JSON(http.StatusOK, models.SuccessResponse(stats))
}

// GetOperationLogs 获取操作日志
// @Summary 获取操作日志
// @Description 分页获取系统操作日志
// @Tags 系统管理
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页大小" default(10)
// @Param user_id query int false "用户ID"
// @Param action query string false "操作类型"
// @Param resource query string false "资源类型"
// @Param start_time query string false "开始时间"
// @Param end_time query string false "结束时间"
// @Param ip_address query string false "IP地址"
// @Param status_code query int false "状态码"
// @Success 200 {object} models.Response{data=models.PaginatedList{items=[]models.OperationLog}} "获取成功"
// @Router /api/v1/system/logs/operation [get]
func (h *SystemHandler) GetOperationLogs(c *gin.Context) {
	var query OperationLogQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "查询参数错误"))
		return
	}

	// 设置默认分页参数
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.PageSize <= 0 {
		query.PageSize = 10
	}

	// 构建查询
	db := database.DB.Model(&models.OperationLog{}).Preload("User")

	// 应用筛选条件
	if query.UserID != nil {
		db = db.Where("user_id = ?", *query.UserID)
	}
	if query.Action != nil && *query.Action != "" {
		db = db.Where("action LIKE ?", "%"+*query.Action+"%")
	}
	if query.Resource != nil && *query.Resource != "" {
		db = db.Where("resource LIKE ?", "%"+*query.Resource+"%")
	}
	if query.StartTime != nil {
		db = db.Where("created_at >= ?", *query.StartTime)
	}
	if query.EndTime != nil {
		db = db.Where("created_at <= ?", *query.EndTime)
	}
	if query.IPAddress != nil && *query.IPAddress != "" {
		db = db.Where("ip_address = ?", *query.IPAddress)
	}
	if query.StatusCode != nil {
		db = db.Where("status_code = ?", *query.StatusCode)
	}

	// 获取总数
	var total int64
	if err := db.Count(&total).Error; err != nil {
		h.logger.Error("Failed to count operation logs", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询失败"))
		return
	}

	// 分页查询
	var logs []models.OperationLog
	offset := (query.Page - 1) * query.PageSize
	if err := db.Offset(offset).Limit(query.PageSize).Order("created_at DESC").Find(&logs).Error; err != nil {
		h.logger.Error("Failed to list operation logs", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询失败"))
		return
	}

	result := models.NewPageResponse(logs, total, query.Page, query.PageSize)
	c.JSON(http.StatusOK, models.SuccessResponse(result))
}

// GetLoginLogs 获取登录日志
// @Summary 获取登录日志
// @Description 分页获取用户登录日志
// @Tags 系统管理
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页大小" default(10)
// @Param user_id query int false "用户ID"
// @Param username query string false "用户名"
// @Param start_time query string false "开始时间"
// @Param end_time query string false "结束时间"
// @Param ip_address query string false "IP地址"
// @Param status query string false "登录状态"
// @Success 200 {object} models.Response{data=models.PaginatedList{items=[]models.LoginLog}} "获取成功"
// @Router /api/v1/system/logs/login [get]
func (h *SystemHandler) GetLoginLogs(c *gin.Context) {
	var query LoginLogQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "查询参数错误"))
		return
	}

	// 设置默认分页参数
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.PageSize <= 0 {
		query.PageSize = 10
	}

	// 构建查询
	db := database.DB.Model(&models.LoginLog{}).Preload("User")

	// 应用筛选条件
	if query.UserID != nil {
		db = db.Where("user_id = ?", *query.UserID)
	}
	if query.Username != nil && *query.Username != "" {
		db = db.Joins("JOIN users ON users.id = login_logs.user_id").
			Where("users.username LIKE ?", "%"+*query.Username+"%")
	}
	if query.StartTime != nil {
		db = db.Where("created_at >= ?", *query.StartTime)
	}
	if query.EndTime != nil {
		db = db.Where("created_at <= ?", *query.EndTime)
	}
	if query.IPAddress != nil && *query.IPAddress != "" {
		db = db.Where("ip_address = ?", *query.IPAddress)
	}
	if query.Status != nil && *query.Status != "" {
		db = db.Where("status = ?", *query.Status)
	}

	// 获取总数
	var total int64
	if err := db.Count(&total).Error; err != nil {
		h.logger.Error("Failed to count login logs", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询失败"))
		return
	}

	// 分页查询
	var logs []models.LoginLog
	offset := (query.Page - 1) * query.PageSize
	if err := db.Offset(offset).Limit(query.PageSize).Order("created_at DESC").Find(&logs).Error; err != nil {
		h.logger.Error("Failed to list login logs", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询失败"))
		return
	}

	result := models.NewPageResponse(logs, total, query.Page, query.PageSize)
	c.JSON(http.StatusOK, models.SuccessResponse(result))
}

// ClearOldLogs 清理旧日志
// @Summary 清理旧日志
// @Description 清理指定天数之前的操作日志和登录日志
// @Tags 系统管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param days query int false "保留天数" default(30)
// @Success 200 {object} models.Response "清理成功"
// @Router /api/v1/system/logs/clear [delete]
func (h *SystemHandler) ClearOldLogs(c *gin.Context) {
	daysStr := c.DefaultQuery("days", "30")
	days, err := strconv.Atoi(daysStr)
	if err != nil || days <= 0 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "保留天数参数无效"))
		return
	}

	cutoffTime := time.Now().AddDate(0, 0, -days)

	// 删除旧的操作日志
	deleteOpsResult := database.DB.Where("created_at < ?", cutoffTime).Delete(&models.OperationLog{})
	if deleteOpsResult.Error != nil {
		h.logger.Error("Failed to clear old operation logs", zap.Error(deleteOpsResult.Error))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "清理操作日志失败"))
		return
	}
	deletedOps := deleteOpsResult.RowsAffected

	// 删除旧的登录日志
	deleteLoginsResult := database.DB.Where("created_at < ?", cutoffTime).Delete(&models.LoginLog{})
	if deleteLoginsResult.Error != nil {
		h.logger.Error("Failed to clear old login logs", zap.Error(deleteLoginsResult.Error))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "清理登录日志失败"))
		return
	}
	deletedLogins := deleteLoginsResult.RowsAffected

	result := map[string]interface{}{
		"deleted_operation_logs": deletedOps,
		"deleted_login_logs":     deletedLogins,
		"cutoff_time":            cutoffTime,
	}

	h.logger.Info("Old logs cleared successfully",
		zap.Int64("deleted_operation_logs", deletedOps),
		zap.Int64("deleted_login_logs", deletedLogins),
		zap.Time("cutoff_time", cutoffTime))

	c.JSON(http.StatusOK, models.SuccessResponse(result))
}

// GetSystemHealth 获取系统健康状态
// @Summary 获取系统健康状态
// @Description 检查系统各组件的健康状态
// @Tags 系统管理
// @Produce json
// @Security BearerAuth
// @Success 200 {object} models.Response{data=object} "获取成功"
// @Router /api/v1/system/health [get]
func (h *SystemHandler) GetSystemHealth(c *gin.Context) {
	health := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now(),
		"checks":    make(map[string]interface{}),
	}

	checks := health["checks"].(map[string]interface{})

	// 检查数据库连接
	if sqlDB, err := database.DB.DB(); err == nil {
		if err := sqlDB.Ping(); err == nil {
			checks["database"] = map[string]interface{}{
				"status": "healthy",
				"message": "Database connection is working",
			}
		} else {
			checks["database"] = map[string]interface{}{
				"status": "unhealthy",
				"message": "Database ping failed: " + err.Error(),
			}
			health["status"] = "unhealthy"
		}
	} else {
		checks["database"] = map[string]interface{}{
			"status": "unhealthy",
			"message": "Failed to get database connection: " + err.Error(),
		}
		health["status"] = "unhealthy"
	}

	// 检查内存使用
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	memUsageMB := memStats.Alloc / 1024 / 1024

	if memUsageMB < 1024 { // 小于1GB认为正常
		checks["memory"] = map[string]interface{}{
			"status": "healthy",
			"usage_mb": memUsageMB,
			"message": "Memory usage is normal",
		}
	} else {
		checks["memory"] = map[string]interface{}{
			"status": "warning",
			"usage_mb": memUsageMB,
			"message": "High memory usage detected",
		}
		if health["status"] == "healthy" {
			health["status"] = "warning"
		}
	}

	// 检查系统运行时间
	uptime := time.Since(startTime)
	checks["uptime"] = map[string]interface{}{
		"status": "healthy",
		"uptime": uptime.String(),
		"start_time": startTime,
	}

	c.JSON(http.StatusOK, models.SuccessResponse(health))
}