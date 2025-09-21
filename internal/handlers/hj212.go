package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/env-data-platform/internal/database"
	"github.com/env-data-platform/internal/hj212"
	"github.com/env-data-platform/internal/models"
)

// HJ212Handler HJ212数据处理器
type HJ212Handler struct {
	logger *zap.Logger
	server *hj212.Server
}

// NewHJ212Handler 创建HJ212处理器
func NewHJ212Handler(logger *zap.Logger, server *hj212.Server) *HJ212Handler {
	return &HJ212Handler{
		logger: logger,
		server: server,
	}
}

// HJ212DataQuery HJ212数据查询参数
type HJ212DataQuery struct {
	models.PaginationQuery
	DeviceID    *string    `form:"device_id"`
	DataType    *string    `form:"data_type"`
	StartTime   *time.Time `form:"start_time" time_format:"2006-01-02 15:04:05"`
	EndTime     *time.Time `form:"end_time" time_format:"2006-01-02 15:04:05"`
	CommandCode *string    `form:"command_code"`
}

// HJ212StatsQuery HJ212统计查询参数
type HJ212StatsQuery struct {
	DeviceID  *string    `form:"device_id"`
	StartTime *time.Time `form:"start_time" time_format:"2006-01-02 15:04:05"`
	EndTime   *time.Time `form:"end_time" time_format:"2006-01-02 15:04:05"`
	GroupBy   *string    `form:"group_by"` // hour, day, month
}

// SendCommandRequest 发送命令请求
type SendCommandRequest struct {
	DeviceID string `json:"device_id" binding:"required"`
	Command  string `json:"command" binding:"required"`
}

// QueryData 查询HJ212数据
// @Summary 查询HJ212监测数据
// @Description 分页查询HJ212监测数据，支持多条件筛选
// @Tags HJ212数据
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页大小" default(10)
// @Param device_id query string false "设备ID"
// @Param data_type query string false "数据类型"
// @Param start_time query string false "开始时间" format(date-time)
// @Param end_time query string false "结束时间" format(date-time)
// @Param command_code query string false "命令编码"
// @Success 200 {object} models.Response{data=models.PaginatedList{items=[]models.HJ212Data}} "查询成功"
// @Router /api/v1/hj212/data [get]
func (h *HJ212Handler) QueryData(c *gin.Context) {
	var query HJ212DataQuery
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
	db := database.DB.Model(&models.HJ212Data{})

	// 应用筛选条件
	if query.DeviceID != nil && *query.DeviceID != "" {
		db = db.Where("device_id = ?", *query.DeviceID)
	}
	if query.DataType != nil && *query.DataType != "" {
		db = db.Where("data_type = ?", *query.DataType)
	}
	if query.CommandCode != nil && *query.CommandCode != "" {
		db = db.Where("command_code = ?", *query.CommandCode)
	}
	if query.StartTime != nil {
		db = db.Where("received_at >= ?", *query.StartTime)
	}
	if query.EndTime != nil {
		db = db.Where("received_at <= ?", *query.EndTime)
	}

	// 获取总数
	var total int64
	if err := db.Count(&total).Error; err != nil {
		h.logger.Error("Failed to count HJ212 data", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询失败"))
		return
	}

	// 分页查询
	var data []models.HJ212Data
	offset := (query.Page - 1) * query.PageSize
	if err := db.Offset(offset).Limit(query.PageSize).Order("received_at DESC").Find(&data).Error; err != nil {
		h.logger.Error("Failed to query HJ212 data", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询失败"))
		return
	}

	result := models.NewPageResponse(data, total, query.Page, query.PageSize)

	c.JSON(http.StatusOK, models.SuccessResponse(result))
}

// GetStats 获取HJ212数据统计
// @Summary 获取HJ212数据统计
// @Description 获取HJ212数据的统计信息
// @Tags HJ212数据
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param device_id query string false "设备ID"
// @Param start_time query string false "开始时间" format(date-time)
// @Param end_time query string false "结束时间" format(date-time)
// @Param group_by query string false "分组方式" Enums(hour,day,month) default(day)
// @Success 200 {object} models.Response{data=map[string]interface{}} "获取成功"
// @Router /api/v1/hj212/stats [get]
func (h *HJ212Handler) GetStats(c *gin.Context) {
	var query HJ212StatsQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "查询参数错误"))
		return
	}

	// 设置默认时间范围（最近7天）
	if query.EndTime == nil {
		now := time.Now()
		query.EndTime = &now
	}
	if query.StartTime == nil {
		start := query.EndTime.AddDate(0, 0, -7)
		query.StartTime = &start
	}

	// 设置默认分组方式
	if query.GroupBy == nil {
		groupBy := "day"
		query.GroupBy = &groupBy
	}

	// 基础统计查询
	baseQuery := database.DB.Model(&models.HJ212Data{}).
		Where("received_at >= ? AND received_at <= ?", *query.StartTime, *query.EndTime)

	if query.DeviceID != nil && *query.DeviceID != "" {
		baseQuery = baseQuery.Where("device_id = ?", *query.DeviceID)
	}

	// 总数统计
	var totalCount int64
	if err := baseQuery.Count(&totalCount).Error; err != nil {
		h.logger.Error("Failed to count total data", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "统计失败"))
		return
	}

	// 按数据类型统计
	var dataTypeStats []struct {
		DataType string `json:"data_type"`
		Count    int64  `json:"count"`
	}
	if err := baseQuery.Select("data_type, COUNT(*) as count").
		Group("data_type").Scan(&dataTypeStats).Error; err != nil {
		h.logger.Error("Failed to get data type stats", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "统计失败"))
		return
	}

	// 按设备统计
	var deviceStats []struct {
		DeviceID string `json:"device_id"`
		Count    int64  `json:"count"`
	}
	deviceQuery := database.DB.Model(&models.HJ212Data{}).
		Select("device_id, COUNT(*) as count").
		Where("received_at >= ? AND received_at <= ?", *query.StartTime, *query.EndTime).
		Group("device_id")

	if query.DeviceID != nil && *query.DeviceID != "" {
		deviceQuery = deviceQuery.Where("device_id = ?", *query.DeviceID)
	}

	if err := deviceQuery.Scan(&deviceStats).Error; err != nil {
		h.logger.Error("Failed to get device stats", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "统计失败"))
		return
	}

	// 时间趋势统计
	var timeFormat string
	switch *query.GroupBy {
	case "hour":
		timeFormat = "%Y-%m-%d %H:00:00"
	case "day":
		timeFormat = "%Y-%m-%d"
	case "month":
		timeFormat = "%Y-%m"
	default:
		timeFormat = "%Y-%m-%d"
	}

	var trendStats []struct {
		Time  string `json:"time"`
		Count int64  `json:"count"`
	}
	trendQuery := baseQuery.Select("DATE_FORMAT(received_at, ?) as time, COUNT(*) as count", timeFormat).
		Group("time").Order("time")

	if err := trendQuery.Scan(&trendStats).Error; err != nil {
		h.logger.Error("Failed to get trend stats", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "统计失败"))
		return
	}

	// 构建响应
	stats := map[string]interface{}{
		"total_count":      totalCount,
		"data_type_stats":  dataTypeStats,
		"device_stats":     deviceStats,
		"trend_stats":      trendStats,
		"connected_devices": h.server.GetConnectedDevices(),
		"time_range": map[string]interface{}{
			"start_time": query.StartTime,
			"end_time":   query.EndTime,
			"group_by":   *query.GroupBy,
		},
	}

	c.JSON(http.StatusOK, models.SuccessResponse(stats))
}

// GetConnectedDevices 获取已连接设备列表
// @Summary 获取已连接设备列表
// @Description 获取当前连接到HJ212服务器的设备列表
// @Tags HJ212数据
// @Produce json
// @Security BearerAuth
// @Success 200 {object} models.Response{data=[]string} "获取成功"
// @Router /api/v1/hj212/devices [get]
func (h *HJ212Handler) GetConnectedDevices(c *gin.Context) {
	devices := h.server.GetConnectedDevices()
	c.JSON(http.StatusOK, models.SuccessResponse(devices))
}

// SendCommand 向设备发送命令
// @Summary 向设备发送命令
// @Description 向指定HJ212设备发送控制命令
// @Tags HJ212数据
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body SendCommandRequest true "发送命令请求"
// @Success 200 {object} models.Response "发送成功"
// @Failure 400 {object} models.Response "请求参数错误"
// @Failure 404 {object} models.Response "设备未连接"
// @Router /api/v1/hj212/command [post]
func (h *HJ212Handler) SendCommand(c *gin.Context) {
	var req SendCommandRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "请求参数错误"))
		return
	}

	// 发送命令
	if err := h.server.SendCommand(req.DeviceID, req.Command); err != nil {
		h.logger.Error("Failed to send command",
			zap.Error(err),
			zap.String("device_id", req.DeviceID),
			zap.String("command", req.Command))
		c.JSON(http.StatusNotFound, models.ErrorResponse(http.StatusNotFound, "设备未连接或命令发送失败"))
		return
	}

	h.logger.Info("Command sent successfully",
		zap.String("device_id", req.DeviceID),
		zap.String("command", req.Command))
	c.JSON(http.StatusOK, models.SuccessResponse(nil))
}

// GetDataDetail 获取数据详情
// @Summary 获取HJ212数据详情
// @Description 根据ID获取HJ212数据的详细信息
// @Tags HJ212数据
// @Produce json
// @Security BearerAuth
// @Param id path int true "数据ID"
// @Success 200 {object} models.Response{data=models.HJ212Data} "获取成功"
// @Failure 404 {object} models.Response "数据不存在"
// @Router /api/v1/hj212/data/{id} [get]
func (h *HJ212Handler) GetDataDetail(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "数据ID无效"))
		return
	}

	var data models.HJ212Data
	if err := database.DB.Where("id = ?", id).First(&data).Error; err != nil {
		h.logger.Error("Failed to get HJ212 data detail", zap.Error(err))
		c.JSON(http.StatusNotFound, models.ErrorResponse(http.StatusNotFound, "数据不存在"))
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse(data))
}

// GetAlarmData 查询报警数据
// @Summary 查询HJ212报警数据
// @Description 分页查询HJ212报警数据
// @Tags HJ212数据
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页大小" default(10)
// @Param device_id query string false "设备ID"
// @Param status query string false "状态"
// @Param start_time query string false "开始时间" format(date-time)
// @Param end_time query string false "结束时间" format(date-time)
// @Success 200 {object} models.Response{data=models.PaginatedList{items=[]models.HJ212AlarmData}} "查询成功"
// @Router /api/v1/hj212/alarms [get]
func (h *HJ212Handler) GetAlarmData(c *gin.Context) {
	var query struct {
		models.PaginationQuery
		DeviceID  *string    `form:"device_id"`
		Status    *string    `form:"status"`
		StartTime *time.Time `form:"start_time" time_format:"2006-01-02 15:04:05"`
		EndTime   *time.Time `form:"end_time" time_format:"2006-01-02 15:04:05"`
	}

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
	db := database.DB.Model(&models.HJ212AlarmData{})

	// 应用筛选条件
	if query.DeviceID != nil && *query.DeviceID != "" {
		db = db.Where("device_id = ?", *query.DeviceID)
	}
	if query.Status != nil && *query.Status != "" {
		db = db.Where("status = ?", *query.Status)
	}
	if query.StartTime != nil {
		db = db.Where("received_at >= ?", *query.StartTime)
	}
	if query.EndTime != nil {
		db = db.Where("received_at <= ?", *query.EndTime)
	}

	// 获取总数
	var total int64
	if err := db.Count(&total).Error; err != nil {
		h.logger.Error("Failed to count alarm data", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询失败"))
		return
	}

	// 分页查询
	var alarms []models.HJ212AlarmData
	offset := (query.Page - 1) * query.PageSize
	if err := db.Offset(offset).Limit(query.PageSize).Order("received_at DESC").Find(&alarms).Error; err != nil {
		h.logger.Error("Failed to query alarm data", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询失败"))
		return
	}

	result := models.NewPageResponse(alarms, total, query.Page, query.PageSize)

	c.JSON(http.StatusOK, models.SuccessResponse(result))
}