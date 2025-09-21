package handlers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/env-data-platform/internal/database"
	"github.com/env-data-platform/internal/models"
)

// FileHandler 文件处理器
type FileHandler struct {
	logger    *zap.Logger
	uploadDir string
}

// NewFileHandler 创建文件处理器
func NewFileHandler(logger *zap.Logger) *FileHandler {
	uploadDir := "./uploads"
	if dir := os.Getenv("UPLOAD_DIR"); dir != "" {
		uploadDir = dir
	}

	// 确保上传目录存在
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		logger.Error("Failed to create upload directory", zap.Error(err))
	}

	return &FileHandler{
		logger:    logger,
		uploadDir: uploadDir,
	}
}

// UploadFileRequest 文件上传请求
type UploadFileRequest struct {
	Description string `form:"description"`
	Tags        string `form:"tags"`
}

// FileListQuery 文件列表查询参数
type FileListQuery struct {
	models.PaginationQuery
	Category *string `form:"category"`
	FileType *string `form:"file_type"`
	IsPublic *bool   `form:"is_public"`
	UserID   *uint   `form:"user_id"`
}

// UploadFile 文件上传
// @Summary 文件上传
// @Description 上传文件到服务器
// @Tags 文件管理
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param file formData file true "上传的文件"
// @Param category formData string false "文件分类"
// @Param description formData string false "文件描述"
// @Param is_public formData bool false "是否公开"
// @Success 200 {object} models.Response{data=models.FileRecord} "上传成功"
// @Router /api/v1/files/upload [post]
func (h *FileHandler) UploadFile(c *gin.Context) {
	// 获取当前用户ID
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse(http.StatusUnauthorized, "未授权"))
		return
	}

	// 绑定表单参数
	var req UploadFileRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "参数错误"))
		return
	}

	// 获取上传的文件
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "请选择要上传的文件"))
		return
	}

	// 验证文件大小 (最大50MB)
	const maxFileSize = 50 * 1024 * 1024
	if file.Size > maxFileSize {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "文件大小不能超过50MB"))
		return
	}

	// 验证文件类型
	allowedTypes := map[string]bool{
		".txt":  true,
		".csv":  true,
		".json": true,
		".xml":  true,
		".xlsx": true,
		".xls":  true,
		".pdf":  true,
		".doc":  true,
		".docx": true,
		".jpg":  true,
		".jpeg": true,
		".png":  true,
		".gif":  true,
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	if !allowedTypes[ext] {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "不支持的文件类型"))
		return
	}

	// 生成唯一文件名
	timestamp := time.Now().Unix()
	filename := fmt.Sprintf("%d_%s", timestamp, file.Filename)
	filePath := filepath.Join(h.uploadDir, filename)

	// 保存文件
	if err := c.SaveUploadedFile(file, filePath); err != nil {
		h.logger.Error("Failed to save uploaded file", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "文件保存失败"))
		return
	}

	// 创建文件记录
	fileRecord := models.FileRecord{
		OriginalName: file.Filename,
		StoredName:   filename,
		FilePath:     filePath,
		FileSize:     file.Size,
		FileType:     models.GetFileTypeByMime(file.Header.Get("Content-Type")),
		MimeType:     file.Header.Get("Content-Type"),
		Description:  req.Description,
		Tags:         req.Tags,
		Status:       models.FileStatusActive,
	}
	fileRecord.CreatedBy = userID.(uint)

	if err := database.DB.Create(&fileRecord).Error; err != nil {
		// 如果数据库保存失败，删除已上传的文件
		os.Remove(filePath)
		h.logger.Error("Failed to create file record", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "文件记录创建失败"))
		return
	}

	h.logger.Info("File uploaded successfully",
		zap.Uint("file_id", fileRecord.ID),
		zap.String("filename", file.Filename),
		zap.Int64("size", file.Size))

	c.JSON(http.StatusOK, models.SuccessResponse(fileRecord))
}

// ListFiles 获取文件列表
// @Summary 获取文件列表
// @Description 分页获取文件记录列表
// @Tags 文件管理
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页大小" default(10)
// @Param category query string false "文件分类"
// @Param file_type query string false "文件类型"
// @Param is_public query bool false "是否公开"
// @Param user_id query int false "上传者ID"
// @Success 200 {object} models.Response{data=models.PaginatedList{items=[]models.FileRecord}} "获取成功"
// @Router /api/v1/files/records [get]
func (h *FileHandler) ListFiles(c *gin.Context) {
	var query FileListQuery
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
	db := database.DB.Model(&models.FileRecord{}).Preload("Uploader")

	// 应用筛选条件
	if query.Category != nil && *query.Category != "" {
		db = db.Where("category = ?", *query.Category)
	}
	if query.FileType != nil && *query.FileType != "" {
		db = db.Where("file_type = ?", *query.FileType)
	}
	if query.IsPublic != nil {
		db = db.Where("is_public = ?", *query.IsPublic)
	}
	if query.UserID != nil {
		db = db.Where("uploader_id = ?", *query.UserID)
	}

	// 只显示活跃状态的文件
	db = db.Where("status = ?", models.FileStatusActive)

	// 获取总数
	var total int64
	if err := db.Count(&total).Error; err != nil {
		h.logger.Error("Failed to count files", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询失败"))
		return
	}

	// 分页查询
	var files []models.FileRecord
	offset := (query.Page - 1) * query.PageSize
	if err := db.Offset(offset).Limit(query.PageSize).Order("created_at DESC").Find(&files).Error; err != nil {
		h.logger.Error("Failed to list files", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询失败"))
		return
	}

	result := models.NewPageResponse(files, total, query.Page, query.PageSize)
	c.JSON(http.StatusOK, models.SuccessResponse(result))
}

// DownloadFile 文件下载
// @Summary 文件下载
// @Description 根据文件ID下载文件
// @Tags 文件管理
// @Produce application/octet-stream
// @Security BearerAuth
// @Param id path int true "文件ID"
// @Success 200 {file} binary "文件内容"
// @Router /api/v1/files/{id}/download [get]
func (h *FileHandler) DownloadFile(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "文件ID无效"))
		return
	}

	// 获取当前用户ID
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse(http.StatusUnauthorized, "未授权"))
		return
	}

	// 查找文件记录
	var fileRecord models.FileRecord
	if err := database.DB.Where("id = ? AND status = ?", id, models.FileStatusActive).First(&fileRecord).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, models.ErrorResponse(http.StatusNotFound, "文件不存在"))
		} else {
			h.logger.Error("Failed to find file record", zap.Error(err))
			c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询失败"))
		}
		return
	}

	// 权限检查：只有上传者才能下载（暂时简化权限控制）
	if fileRecord.CreatedBy != userID.(uint) {
		c.JSON(http.StatusForbidden, models.ErrorResponse(http.StatusForbidden, "无权限下载此文件"))
		return
	}

	// 检查文件是否存在
	if _, err := os.Stat(fileRecord.FilePath); os.IsNotExist(err) {
		h.logger.Error("File not found on disk", zap.String("path", fileRecord.FilePath))
		c.JSON(http.StatusNotFound, models.ErrorResponse(http.StatusNotFound, "文件不存在"))
		return
	}

	// 更新下载次数
	database.DB.Model(&fileRecord).UpdateColumn("download_count", gorm.Expr("download_count + 1"))

	// 设置响应头
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileRecord.OriginalName))
	c.Header("Content-Type", "application/octet-stream")

	// 发送文件
	c.File(fileRecord.FilePath)

	h.logger.Info("File downloaded",
		zap.Uint("file_id", fileRecord.ID),
		zap.Uint("user_id", userID.(uint)),
		zap.String("filename", fileRecord.OriginalName))
}

// DeleteFile 删除文件
// @Summary 删除文件
// @Description 删除文件记录和物理文件
// @Tags 文件管理
// @Produce json
// @Security BearerAuth
// @Param id path int true "文件ID"
// @Success 200 {object} models.Response "删除成功"
// @Router /api/v1/files/{id} [delete]
func (h *FileHandler) DeleteFile(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "文件ID无效"))
		return
	}

	// 获取当前用户ID
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse(http.StatusUnauthorized, "未授权"))
		return
	}

	// 查找文件记录
	var fileRecord models.FileRecord
	if err := database.DB.Where("id = ?", id).First(&fileRecord).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, models.ErrorResponse(http.StatusNotFound, "文件不存在"))
		} else {
			h.logger.Error("Failed to find file record", zap.Error(err))
			c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询失败"))
		}
		return
	}

	// 权限检查：只有上传者才能删除文件
	if fileRecord.CreatedBy != userID.(uint) {
		c.JSON(http.StatusForbidden, models.ErrorResponse(http.StatusForbidden, "无权限删除此文件"))
		return
	}

	// 软删除：更新状态为已删除
	if err := database.DB.Model(&fileRecord).Update("status", models.FileStatusDeleted).Error; err != nil {
		h.logger.Error("Failed to delete file record", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "删除失败"))
		return
	}

	// 删除物理文件（可选，也可以通过定时任务清理）
	if err := os.Remove(fileRecord.FilePath); err != nil {
		h.logger.Warn("Failed to remove physical file", zap.Error(err), zap.String("path", fileRecord.FilePath))
		// 不返回错误，因为数据库记录已经删除
	}

	h.logger.Info("File deleted successfully",
		zap.Uint("file_id", fileRecord.ID),
		zap.Uint("user_id", userID.(uint)),
		zap.String("filename", fileRecord.OriginalName))

	c.JSON(http.StatusOK, models.SuccessResponse(nil))
}

// GetFileInfo 获取文件信息
// @Summary 获取文件信息
// @Description 根据文件ID获取文件详细信息
// @Tags 文件管理
// @Produce json
// @Security BearerAuth
// @Param id path int true "文件ID"
// @Success 200 {object} models.Response{data=models.FileRecord} "获取成功"
// @Router /api/v1/files/{id} [get]
func (h *FileHandler) GetFileInfo(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "文件ID无效"))
		return
	}

	// 获取当前用户ID
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse(http.StatusUnauthorized, "未授权"))
		return
	}

	// 查找文件记录
	var fileRecord models.FileRecord
	if err := database.DB.Where("id = ? AND status = ?", id, models.FileStatusActive).
		Preload("Uploader").First(&fileRecord).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, models.ErrorResponse(http.StatusNotFound, "文件不存在"))
		} else {
			h.logger.Error("Failed to find file record", zap.Error(err))
			c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询失败"))
		}
		return
	}

	// 权限检查：只有上传者才能查看详情
	if fileRecord.CreatedBy != userID.(uint) {
		c.JSON(http.StatusForbidden, models.ErrorResponse(http.StatusForbidden, "无权限查看此文件"))
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse(fileRecord))
}

// GetFileStats 获取文件统计信息
// @Summary 获取文件统计信息
// @Description 获取文件上传、下载等统计信息
// @Tags 文件管理
// @Produce json
// @Security BearerAuth
// @Success 200 {object} models.Response{data=object} "获取成功"
// @Router /api/v1/files/stats [get]
func (h *FileHandler) GetFileStats(c *gin.Context) {
	var stats struct {
		TotalFiles      int64  `json:"total_files"`
		TotalSize       int64  `json:"total_size"`
		PublicFiles     int64  `json:"public_files"`
		PrivateFiles    int64  `json:"private_files"`
		TotalDownloads  int64  `json:"total_downloads"`
		TodayUploads    int64  `json:"today_uploads"`
		PopularCategory string `json:"popular_category"`
	}

	// 总文件数
	database.DB.Model(&models.FileRecord{}).Where("status = ?", models.FileStatusActive).Count(&stats.TotalFiles)

	// 总文件大小
	database.DB.Model(&models.FileRecord{}).Where("status = ?", models.FileStatusActive).
		Select("COALESCE(SUM(file_size), 0)").Scan(&stats.TotalSize)

	// 公开/私有文件数
	database.DB.Model(&models.FileRecord{}).Where("status = ? AND is_public = ?", models.FileStatusActive, true).Count(&stats.PublicFiles)
	database.DB.Model(&models.FileRecord{}).Where("status = ? AND is_public = ?", models.FileStatusActive, false).Count(&stats.PrivateFiles)

	// 总下载次数
	database.DB.Model(&models.FileRecord{}).Where("status = ?", models.FileStatusActive).
		Select("COALESCE(SUM(download_count), 0)").Scan(&stats.TotalDownloads)

	// 今日上传数
	today := time.Now().Format("2006-01-02")
	database.DB.Model(&models.FileRecord{}).Where("DATE(created_at) = ? AND status = ?", today, models.FileStatusActive).Count(&stats.TodayUploads)

	// 最受欢迎的分类
	var result struct {
		Category string
		Count    int64
	}
	database.DB.Model(&models.FileRecord{}).
		Select("category, COUNT(*) as count").
		Where("status = ? AND category != ''", models.FileStatusActive).
		Group("category").
		Order("count DESC").
		Limit(1).
		Scan(&result)
	stats.PopularCategory = result.Category

	c.JSON(http.StatusOK, models.SuccessResponse(stats))
}