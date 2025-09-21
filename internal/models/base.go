package models

import (
	"time"

	"gorm.io/gorm"
)

// BaseModel 基础模型，包含公共字段
type BaseModel struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

// BaseModelWithOperator 包含操作人员信息的基础模型
type BaseModelWithOperator struct {
	BaseModel
	CreatedBy uint   `json:"created_by" gorm:"comment:创建人ID"`
	UpdatedBy uint   `json:"updated_by" gorm:"comment:更新人ID"`
	Remark    string `json:"remark" gorm:"type:text;comment:备注"`
}

// TableName 获取表名前缀
func GetTableName(name string) string {
	return "env_" + name
}

// 状态常量
const (
	StatusActive   = 1 // 激活
	StatusInactive = 0 // 禁用
	StatusDeleted  = -1 // 删除
)

// 用户状态常量（别名）
const (
	UserStatusActive   = StatusActive   // 激活
	UserStatusInactive = StatusInactive // 禁用
)

// 数据源类型常量
const (
	DataSourceTypeHJ212    = "hj212"    // HJ212协议
	DataSourceTypeDatabase = "database" // 数据库
	DataSourceTypeFile     = "file"     // 文件
	DataSourceTypeAPI      = "api"      // API接口
	DataSourceTypeWebhook  = "webhook"  // Webhook
)

// ETL任务状态常量
const (
	ETLStatusPending   = "pending"   // 等待执行
	ETLStatusRunning   = "running"   // 运行中
	ETLStatusSuccess   = "success"   // 执行成功
	ETLStatusFailed    = "failed"    // 执行失败
	ETLStatusCanceled  = "canceled"  // 已取消
	ETLStatusScheduled = "scheduled" // 已调度
)

// 用户角色常量
const (
	RoleAdmin     = "admin"     // 管理员
	RoleOperator  = "operator"  // 操作员
	RoleDeveloper = "developer" // 开发者
	RoleViewer    = "viewer"    // 查看者
)

// 数据质量等级常量
const (
	QualityExcellent = "excellent" // 优秀
	QualityGood     = "good"      // 良好
	QualityFair     = "fair"      // 一般
	QualityPoor     = "poor"      // 较差
	QualityBad      = "bad"       // 很差
)

// API响应结构
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// 分页请求结构
type PageRequest struct {
	Page     int    `json:"page" form:"page" binding:"min=1"`
	PageSize int    `json:"page_size" form:"page_size" binding:"min=1,max=100"`
	Sort     string `json:"sort" form:"sort"`
	Order    string `json:"order" form:"order"`
	Keyword  string `json:"keyword" form:"keyword"`
}

// PaginationQuery 分页查询参数（别名）
type PaginationQuery = PageRequest

// PaginatedList 分页列表结构（别名）
type PaginatedList = PageResponse

// 分页响应结构
type PageResponse struct {
	List      interface{} `json:"list"`
	Total     int64       `json:"total"`
	Page      int         `json:"page"`
	PageSize  int         `json:"page_size"`
	TotalPage int         `json:"total_page"`
}

// NewPageResponse 创建分页响应
func NewPageResponse(list interface{}, total int64, page, pageSize int) *PageResponse {
	totalPage := int(total) / pageSize
	if int(total)%pageSize > 0 {
		totalPage++
	}

	return &PageResponse{
		List:      list,
		Total:     total,
		Page:      page,
		PageSize:  pageSize,
		TotalPage: totalPage,
	}
}

// 成功响应
func SuccessResponse(data interface{}) *Response {
	return &Response{
		Code:    200,
		Message: "success",
		Data:    data,
	}
}

// 错误响应
func ErrorResponse(code int, message string) *Response {
	return &Response{
		Code:    code,
		Message: message,
	}
}

// 常用错误响应
func BadRequestResponse(message string) *Response {
	return ErrorResponse(400, message)
}

func UnauthorizedResponse() *Response {
	return ErrorResponse(401, "未授权访问")
}

func ForbiddenResponse() *Response {
	return ErrorResponse(403, "权限不足")
}

func NotFoundResponse(message string) *Response {
	if message == "" {
		message = "资源不存在"
	}
	return ErrorResponse(404, message)
}

func InternalErrorResponse(message string) *Response {
	if message == "" {
		message = "服务器内部错误"
	}
	return ErrorResponse(500, message)
}