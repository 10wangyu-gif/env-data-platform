package models

import (
	"time"
)

// FileRecord 文件记录模型
type FileRecord struct {
	BaseModelWithOperator
	OriginalName string    `gorm:"not null;size:255;comment:原始文件名" json:"original_name"`
	StoredName   string    `gorm:"not null;size:255;comment:存储文件名" json:"stored_name"`
	FilePath     string    `gorm:"not null;size:500;comment:文件路径" json:"file_path"`
	FileSize     int64     `gorm:"not null;comment:文件大小(字节)" json:"file_size"`
	FileType     string    `gorm:"size:100;comment:文件类型" json:"file_type"`
	MimeType     string    `gorm:"size:100;comment:MIME类型" json:"mime_type"`
	MD5Hash      string    `gorm:"size:32;comment:MD5哈希值" json:"md5_hash"`
	Status       string    `gorm:"not null;size:20;comment:文件状态" json:"status"`
	Description  string    `gorm:"size:500;comment:文件描述" json:"description"`
	Tags         string    `gorm:"size:500;comment:文件标签" json:"tags"`
	AccessCount  int       `gorm:"default:0;comment:访问次数" json:"access_count"`
	LastAccess   *time.Time `gorm:"comment:最后访问时间" json:"last_access"`

	// 关联
	Uploader *User `gorm:"foreignKey:CreatedBy" json:"uploader,omitempty"`
}

// TableName 指定表名
func (FileRecord) TableName() string {
	return GetTableName("file_records")
}

// FileStatus 文件状态常量
const (
	FileStatusActive   = "active"   // 活跃状态
	FileStatusDeleted  = "deleted"  // 已删除
	FileStatusArchived = "archived" // 已归档
)

// FileType 文件类型常量
const (
	FileTypeDocument = "document" // 文档
	FileTypeImage    = "image"    // 图片
	FileTypeVideo    = "video"    // 视频
	FileTypeAudio    = "audio"    // 音频
	FileTypeArchive  = "archive"  // 压缩包
	FileTypeOther    = "other"    // 其他
)

// FileUploadRequest 文件上传请求
type FileUploadRequest struct {
	Description string `form:"description"`
	Tags        string `form:"tags"`
}

// FileListRequest 文件列表请求
type FileListRequest struct {
	Page        int    `form:"page,default=1"`
	PageSize    int    `form:"page_size,default=10"`
	Status      string `form:"status"`
	FileType    string `form:"file_type"`
	Keyword     string `form:"keyword"`
	UploaderID  uint   `form:"uploader_id"`
	StartDate   string `form:"start_date"`
	EndDate     string `form:"end_date"`
}

// FileResponse 文件响应结构
type FileResponse struct {
	ID           uint       `json:"id"`
	OriginalName string     `json:"original_name"`
	StoredName   string     `json:"stored_name"`
	FileSize     int64      `json:"file_size"`
	FileType     string     `json:"file_type"`
	MimeType     string     `json:"mime_type"`
	MD5Hash      string     `json:"md5_hash"`
	Status       string     `json:"status"`
	Description  string     `json:"description"`
	Tags         string     `json:"tags"`
	AccessCount  int        `json:"access_count"`
	LastAccess   *time.Time `json:"last_access"`
	Uploader     *UserResponse `json:"uploader"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// FileStats 文件统计结构
type FileStats struct {
	TotalFiles   int64 `json:"total_files"`
	TotalSize    int64 `json:"total_size"`
	ActiveFiles  int64 `json:"active_files"`
	DeletedFiles int64 `json:"deleted_files"`
	TodayUploads int64 `json:"today_uploads"`
	FileTypes    map[string]int64 `json:"file_types"`
}

// UpdateAccessCount 更新访问次数
func (f *FileRecord) UpdateAccessCount() {
	f.AccessCount++
	now := time.Now()
	f.LastAccess = &now
}

// IsActive 检查文件是否活跃
func (f *FileRecord) IsActive() bool {
	return f.Status == FileStatusActive
}

// GetFileTypeByMime 根据MIME类型获取文件类型
func GetFileTypeByMime(mimeType string) string {
	switch {
	case mimeType == "application/pdf" ||
		 mimeType == "application/msword" ||
		 mimeType == "application/vnd.openxmlformats-officedocument.wordprocessingml.document" ||
		 mimeType == "application/vnd.ms-excel" ||
		 mimeType == "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" ||
		 mimeType == "text/plain" ||
		 mimeType == "text/csv":
		return FileTypeDocument
	case mimeType == "image/jpeg" ||
		 mimeType == "image/png" ||
		 mimeType == "image/gif" ||
		 mimeType == "image/webp":
		return FileTypeImage
	case mimeType == "video/mp4" ||
		 mimeType == "video/avi" ||
		 mimeType == "video/mov" ||
		 mimeType == "video/wmv":
		return FileTypeVideo
	case mimeType == "audio/mp3" ||
		 mimeType == "audio/wav" ||
		 mimeType == "audio/flac" ||
		 mimeType == "audio/aac":
		return FileTypeAudio
	case mimeType == "application/zip" ||
		 mimeType == "application/x-rar-compressed" ||
		 mimeType == "application/x-7z-compressed" ||
		 mimeType == "application/gzip":
		return FileTypeArchive
	default:
		return FileTypeOther
	}
}