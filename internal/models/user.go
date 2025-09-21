package models

import (
	"time"
)

// User 用户模型
type User struct {
	BaseModel
	Username    string     `gorm:"uniqueIndex;not null;size:50;comment:用户名" json:"username"`
	Email       string     `gorm:"uniqueIndex;not null;size:100;comment:邮箱" json:"email"`
	Phone       string     `gorm:"size:20;comment:手机号" json:"phone"`
	Password    string     `gorm:"not null;size:255;comment:密码" json:"-"`
	RealName    string     `gorm:"size:50;comment:真实姓名" json:"real_name"`
	Avatar      string     `gorm:"size:255;comment:头像URL" json:"avatar"`
	Status      int        `gorm:"default:1;comment:状态 1激活 0禁用" json:"status"`
	LastLoginAt *time.Time `gorm:"comment:最后登录时间" json:"last_login_at"`
	LoginIP     string     `gorm:"size:45;comment:登录IP" json:"login_ip"`
	LoginCount  int        `gorm:"default:0;comment:登录次数" json:"login_count"`
	Department  string     `gorm:"size:100;comment:部门" json:"department"`
	Position    string     `gorm:"size:100;comment:职位" json:"position"`
	Remark      string     `gorm:"type:text;comment:备注" json:"remark"`

	// 关联
	Roles       []Role       `gorm:"many2many:env_user_roles;" json:"roles,omitempty"`
	UserRoles   []UserRole   `gorm:"foreignKey:UserID" json:"-"`
	CreatedData []DataSource `gorm:"foreignKey:CreatedBy" json:"-"`
}

// TableName 指定表名
func (User) TableName() string {
	return GetTableName("users")
}

// UserInfo 用户信息响应结构
type UserInfo struct {
	ID          uint       `json:"id"`
	Username    string     `json:"username"`
	Email       string     `json:"email"`
	Phone       string     `json:"phone"`
	RealName    string     `json:"real_name"`
	Avatar      string     `json:"avatar"`
	Status      int        `json:"status"`
	LastLoginAt *time.Time `json:"last_login_at"`
	Department  string     `json:"department"`
	Position    string     `json:"position"`
	Roles       []Role     `json:"roles"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// GetPrimaryRole 获取用户的主要角色
func (u *User) GetPrimaryRole() *Role {
	if len(u.Roles) > 0 {
		return &u.Roles[0]
	}
	return nil
}

// GetRoleID 获取用户的主要角色ID
func (u *User) GetRoleID() uint {
	if role := u.GetPrimaryRole(); role != nil {
		return role.ID
	}
	return 0
}

// GetRoleName 获取用户的主要角色名称
func (u *User) GetRoleName() string {
	if role := u.GetPrimaryRole(); role != nil {
		return role.Name
	}
	return ""
}

// ToUserInfo 转换为UserInfo结构
func (u *User) ToUserInfo() *UserInfo {
	return &UserInfo{
		ID:          u.ID,
		Username:    u.Username,
		Email:       u.Email,
		Phone:       u.Phone,
		RealName:    u.RealName,
		Avatar:      u.Avatar,
		Status:      u.Status,
		LastLoginAt: u.LastLoginAt,
		Department:  u.Department,
		Position:    u.Position,
		Roles:       u.Roles,
		CreatedAt:   u.CreatedAt,
		UpdatedAt:   u.UpdatedAt,
	}
}

// Role 角色模型
type Role struct {
	BaseModel
	Name        string `gorm:"uniqueIndex;not null;size:50;comment:角色名称" json:"name"`
	Code        string `gorm:"uniqueIndex;not null;size:50;comment:角色代码" json:"code"`
	Description string `gorm:"size:255;comment:角色描述" json:"description"`
	Status      int    `gorm:"default:1;comment:状态 1激活 0禁用" json:"status"`
	Sort        int    `gorm:"default:0;comment:排序" json:"sort"`
	IsSystem    bool   `gorm:"default:false;comment:是否系统角色" json:"is_system"`

	// 关联
	Users       []User       `gorm:"many2many:env_user_roles;" json:"users,omitempty"`
	Permissions []Permission `gorm:"many2many:env_role_permissions;" json:"permissions,omitempty"`
	UserRoles   []UserRole   `gorm:"foreignKey:RoleID" json:"-"`
}

// TableName 指定表名
func (Role) TableName() string {
	return GetTableName("roles")
}

// Permission 权限模型
type Permission struct {
	BaseModel
	Name        string `gorm:"uniqueIndex;not null;size:100;comment:权限名称" json:"name"`
	Code        string `gorm:"uniqueIndex;not null;size:100;comment:权限代码" json:"code"`
	Type        string `gorm:"not null;size:20;comment:权限类型 menu/button/api" json:"type"`
	ParentID    *uint  `gorm:"default:null;comment:父权限ID" json:"parent_id"`
	Path        string `gorm:"size:255;comment:路径/URL" json:"path"`
	Method      string `gorm:"size:10;comment:HTTP方法" json:"method"`
	Icon        string `gorm:"size:100;comment:图标" json:"icon"`
	Component   string `gorm:"size:255;comment:组件路径" json:"component"`
	Sort        int    `gorm:"default:0;comment:排序" json:"sort"`
	Status      int    `gorm:"default:1;comment:状态 1激活 0禁用" json:"status"`
	IsSystem    bool   `gorm:"default:false;comment:是否系统权限" json:"is_system"`
	Description string `gorm:"size:255;comment:权限描述" json:"description"`

	// 关联
	Roles    []Role       `gorm:"many2many:env_role_permissions;" json:"roles,omitempty"`
	Children []Permission `gorm:"foreignKey:ParentID" json:"children,omitempty"`
	Parent   *Permission  `gorm:"foreignKey:ParentID" json:"parent,omitempty"`
}

// TableName 指定表名
func (Permission) TableName() string {
	return GetTableName("permissions")
}

// UserRole 用户角色关联模型
type UserRole struct {
	UserID uint `gorm:"primaryKey;not null;comment:用户ID" json:"user_id"`
	RoleID uint `gorm:"primaryKey;not null;comment:角色ID" json:"role_id"`

	// 关联
	User *User `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Role *Role `gorm:"foreignKey:RoleID" json:"role,omitempty"`
}

// TableName 指定表名
func (UserRole) TableName() string {
	return GetTableName("user_roles")
}

// RolePermission 角色权限关联模型
type RolePermission struct {
	RoleID       uint `gorm:"primaryKey;not null;comment:角色ID" json:"role_id"`
	PermissionID uint `gorm:"primaryKey;not null;comment:权限ID" json:"permission_id"`

	// 关联
	Role       *Role       `gorm:"foreignKey:RoleID" json:"role,omitempty"`
	Permission *Permission `gorm:"foreignKey:PermissionID" json:"permission,omitempty"`
}

// TableName 指定表名
func (RolePermission) TableName() string {
	return GetTableName("role_permissions")
}

// LoginLog 登录日志模型
type LoginLog struct {
	BaseModel
	UserID    uint   `gorm:"not null;comment:用户ID" json:"user_id"`
	Username  string `gorm:"not null;size:50;comment:用户名" json:"username"`
	IP        string `gorm:"not null;size:45;comment:登录IP" json:"ip"`
	UserAgent string `gorm:"size:500;comment:用户代理" json:"user_agent"`
	Status    int    `gorm:"not null;comment:登录状态 1成功 0失败" json:"status"`
	Message   string `gorm:"size:255;comment:登录信息" json:"message"`
	Location  string `gorm:"size:100;comment:登录地点" json:"location"`

	// 关联
	User *User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// TableName 指定表名
func (LoginLog) TableName() string {
	return GetTableName("login_logs")
}

// OperationLog 操作日志模型
type OperationLog struct {
	BaseModel
	UserID      uint   `gorm:"comment:操作用户ID" json:"user_id"`
	Username    string `gorm:"size:50;comment:操作用户名" json:"username"`
	Module      string `gorm:"not null;size:50;comment:操作模块" json:"module"`
	Action      string `gorm:"not null;size:50;comment:操作动作" json:"action"`
	Method      string `gorm:"not null;size:10;comment:请求方法" json:"method"`
	URL         string `gorm:"not null;size:500;comment:请求URL" json:"url"`
	IP          string `gorm:"not null;size:45;comment:操作IP" json:"ip"`
	UserAgent   string `gorm:"size:500;comment:用户代理" json:"user_agent"`
	RequestBody string `gorm:"type:text;comment:请求参数" json:"request_body"`
	Response    string `gorm:"type:text;comment:响应结果" json:"response"`
	Status      int    `gorm:"not null;comment:操作状态 1成功 0失败" json:"status"`
	ErrorMsg    string `gorm:"type:text;comment:错误信息" json:"error_msg"`
	Duration    int64  `gorm:"comment:执行时长(毫秒)" json:"duration"`

	// 关联
	User *User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// TableName 指定表名
func (OperationLog) TableName() string {
	return GetTableName("operation_logs")
}

// UserRequest 用户请求结构
type UserRequest struct {
	Username   string `json:"username" binding:"required,min=3,max=50"`
	Email      string `json:"email" binding:"required,email"`
	Phone      string `json:"phone"`
	Password   string `json:"password" binding:"required,min=6"`
	RealName   string `json:"real_name"`
	Department string `json:"department"`
	Position   string `json:"position"`
	RoleIDs    []uint `json:"role_ids"`
	Status     int    `json:"status"`
	Remark     string `json:"remark"`
}

// LoginRequest 登录请求结构
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Captcha  string `json:"captcha"`
}

// ChangePasswordRequest 修改密码请求结构
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6"`
}

// ResetPasswordRequest 重置密码请求结构
type ResetPasswordRequest struct {
	UserID      uint   `json:"user_id" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6"`
}

// UserResponse 用户响应结构
type UserResponse struct {
	ID          uint      `json:"id"`
	Username    string    `json:"username"`
	Email       string    `json:"email"`
	Phone       string    `json:"phone"`
	RealName    string    `json:"real_name"`
	Avatar      string    `json:"avatar"`
	Status      int       `json:"status"`
	Department  string    `json:"department"`
	Position    string    `json:"position"`
	LastLoginAt *time.Time `json:"last_login_at"`
	LoginCount  int       `json:"login_count"`
	Roles       []Role    `json:"roles"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// LoginResponse 登录响应结构
type LoginResponse struct {
	Token     string       `json:"token"`
	ExpiresAt int64        `json:"expires_at"`
	User      UserResponse `json:"user"`
}

// 权限检查方法
func (u *User) HasRole(roleCode string) bool {
	for _, role := range u.Roles {
		if role.Code == roleCode {
			return true
		}
	}
	return false
}

// 检查用户是否有指定权限
func (u *User) HasPermission(permissionCode string) bool {
	for _, role := range u.Roles {
		for _, permission := range role.Permissions {
			if permission.Code == permissionCode {
				return true
			}
		}
	}
	return false
}

// 检查用户是否为管理员
func (u *User) IsAdmin() bool {
	return u.HasRole(RoleAdmin)
}