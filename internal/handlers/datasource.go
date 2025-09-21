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

// DataSourceHandler 数据源处理器
type DataSourceHandler struct {
	db                *gorm.DB
	logger            *zap.Logger
	connectionService *services.ConnectionTestService
	metadataService   *services.MetadataSyncService
}

// NewDataSourceHandler 创建数据源处理器
func NewDataSourceHandler(logger *zap.Logger) *DataSourceHandler {
	return &DataSourceHandler{
		db:                database.GetDB(),
		logger:            logger,
		connectionService: services.NewConnectionTestService(),
		metadataService:   services.NewMetadataSyncService(),
	}
}

// ListDataSources 获取数据源列表
func (h *DataSourceHandler) ListDataSources(c *gin.Context) {
	var req struct {
		Page     int    `form:"page" binding:"required,min=1"`
		PageSize int    `form:"page_size" binding:"required,min=1,max=100"`
		Name     string `form:"name"`
		Type     string `form:"type"`
		Status   string `form:"status"`
	}

	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "参数错误"))
		return
	}

	query := h.db.Model(&models.DataSource{})

	if req.Name != "" {
		query = query.Where("name LIKE ?", "%"+req.Name+"%")
	}
	if req.Type != "" {
		query = query.Where("type = ?", req.Type)
	}
	if req.Status != "" {
		query = query.Where("status = ?", req.Status)
	}

	var total int64
	query.Count(&total)

	var dataSources []models.DataSource
	offset := (req.Page - 1) * req.PageSize
	if err := query.Offset(offset).Limit(req.PageSize).
		Preload("Creator").
		Find(&dataSources).Error; err != nil {
		h.logger.Error("Failed to list data sources", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询失败"))
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse(gin.H{
		"list":      dataSources,
		"total":     total,
		"page":      req.Page,
		"page_size": req.PageSize,
	}))
}

// CreateDataSource 创建数据源
func (h *DataSourceHandler) CreateDataSource(c *gin.Context) {
	var req models.DataSourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "参数错误"))
		return
	}

	userID := c.GetUint("user_id")

	// 将配置转换为JSON
	configBytes, err := json.Marshal(req.Config)
	if err != nil {
		h.logger.Error("Failed to marshal config", zap.Error(err))
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "配置格式错误"))
		return
	}

	dataSource := models.DataSource{
		Name:        req.Name,
		Type:        req.Type,
		Description: req.Description,
		Config:      string(configBytes),
		ConfigData:  configBytes,
		Status:      "active",
		Priority:    req.Priority,
	}
	dataSource.CreatedBy = userID
	dataSource.UpdatedBy = userID

	if err := h.db.Create(&dataSource).Error; err != nil {
		h.logger.Error("Failed to create data source", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "创建失败"))
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse(dataSource))
}

// GetDataSource 获取数据源详情
func (h *DataSourceHandler) GetDataSource(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "无效的ID"))
		return
	}

	var dataSource models.DataSource
	if err := h.db.Preload("Creator").Preload("Updater").
		First(&dataSource, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, models.ErrorResponse(http.StatusNotFound, "数据源不存在"))
			return
		}
		h.logger.Error("Failed to get data source", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询失败"))
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse(dataSource))
}

// UpdateDataSource 更新数据源
func (h *DataSourceHandler) UpdateDataSource(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "无效的ID"))
		return
	}

	var req models.DataSourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "参数错误"))
		return
	}

	var dataSource models.DataSource
	if err := h.db.First(&dataSource, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, models.ErrorResponse(http.StatusNotFound, "数据源不存在"))
			return
		}
		h.logger.Error("Failed to get data source", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询失败"))
		return
	}

	// 将配置转换为JSON
	configBytes, err := json.Marshal(req.Config)
	if err != nil {
		h.logger.Error("Failed to marshal config", zap.Error(err))
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "配置格式错误"))
		return
	}

	updates := map[string]interface{}{
		"name":        req.Name,
		"type":        req.Type,
		"description": req.Description,
		"config":      string(configBytes),
		"priority":    req.Priority,
		"updated_by":  c.GetUint("user_id"),
	}

	if err := h.db.Model(&dataSource).Updates(updates).Error; err != nil {
		h.logger.Error("Failed to update data source", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "更新失败"))
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse(dataSource))
}

// DeleteDataSource 删除数据源
func (h *DataSourceHandler) DeleteDataSource(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "无效的ID"))
		return
	}

	var dataSource models.DataSource
	if err := h.db.First(&dataSource, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, models.ErrorResponse(http.StatusNotFound, "数据源不存在"))
			return
		}
		h.logger.Error("Failed to get data source", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询失败"))
		return
	}

	// 检查是否有关联的ETL作业
	var etlCount int64
	h.db.Model(&models.ETLJob{}).Where("source_id = ? OR target_id = ?", id, id).Count(&etlCount)
	if etlCount > 0 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "该数据源有关联的ETL作业，无法删除"))
		return
	}

	if err := h.db.Delete(&dataSource).Error; err != nil {
		h.logger.Error("Failed to delete data source", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "删除失败"))
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse(gin.H{"message": "删除成功"}))
}

// TestDataSource 测试数据源连接
func (h *DataSourceHandler) TestDataSource(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "无效的ID"))
		return
	}

	var dataSource models.DataSource
	if err := h.db.First(&dataSource, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, models.ErrorResponse(http.StatusNotFound, "数据源不存在"))
			return
		}
		h.logger.Error("Failed to get data source", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询失败"))
		return
	}

	// 执行真实的连接测试
	ctx := context.Background()
	result := h.connectionService.TestConnection(ctx, &dataSource)

	// 更新测试时间和状态
	updates := map[string]interface{}{
		"last_test_at": gorm.Expr("NOW()"),
		"is_connected": result.Success,
		"status":       func() string {
			if result.Success {
				return "active"
			}
			return "inactive"
		}(),
	}
	h.db.Model(&dataSource).Updates(updates)

	responseData := gin.H{
		"success": result.Success,
		"message": result.Message,
		"latency": result.Latency.Milliseconds(),
		"details": result.Details,
		"tested_at": result.TestedAt,
	}

	if result.Success {
		c.JSON(http.StatusOK, models.SuccessResponse(responseData))
	} else {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, result.Message))
	}
}

// SyncDataSource 同步数据源元数据
func (h *DataSourceHandler) SyncDataSource(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "无效的ID"))
		return
	}

	var dataSource models.DataSource
	if err := h.db.First(&dataSource, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, models.ErrorResponse(http.StatusNotFound, "数据源不存在"))
			return
		}
		h.logger.Error("Failed to get data source", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询失败"))
		return
	}

	// 执行真实的元数据同步
	ctx := context.Background()
	result := h.metadataService.SyncMetadata(ctx, &dataSource)

	// 更新同步时间和状态
	updates := map[string]interface{}{
		"last_sync_at": gorm.Expr("NOW()"),
		"sync_status": func() string {
			if result.Success {
				return "success"
			}
			return "failed"
		}(),
	}
	h.db.Model(&dataSource).Updates(updates)

	responseData := gin.H{
		"success":   result.Success,
		"message":   result.Message,
		"tables":    result.Tables,
		"details":   result.Details,
		"synced_at": result.SyncedAt,
	}

	if result.Success {
		c.JSON(http.StatusOK, models.SuccessResponse(responseData))
	} else {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, result.Message))
	}
}


// GetDataSourceTables 获取数据源的表列表
func (h *DataSourceHandler) GetDataSourceTables(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "无效的ID"))
		return
	}

	var tables []models.DataTable
	if err := h.db.Where("data_source_id = ?", id).
		Find(&tables).Error; err != nil {
		h.logger.Error("Failed to get data source tables", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询失败"))
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse(tables))
}