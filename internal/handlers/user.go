package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/env-data-platform/internal/auth"
	"github.com/env-data-platform/internal/database"
	"github.com/env-data-platform/internal/models"
)

// UserHandler 用户处理器
type UserHandler struct {
	logger          *zap.Logger
	passwordManager *auth.PasswordManager
}

// NewUserHandler 创建用户处理器
func NewUserHandler(logger *zap.Logger) *UserHandler {
	return &UserHandler{
		logger:          logger,
		passwordManager: auth.NewPasswordManager(),
	}
}

// CreateUserRequest 创建用户请求
type CreateUserRequest struct {
	Username string `json:"username" binding:"required" example:"user001"`
	Email    string `json:"email" binding:"required,email" example:"user@example.com"`
	RealName string `json:"real_name" binding:"required" example:"张三"`
	Phone    string `json:"phone" example:"13800138000"`
	RoleID   uint   `json:"role_id" binding:"required" example:"2"`
	Password string `json:"password" binding:"required" example:"Password123!"`
}

// UpdateUserRequest 更新用户请求
type UpdateUserRequest struct {
	Email    *string `json:"email,omitempty" binding:"omitempty,email"`
	RealName *string `json:"real_name,omitempty"`
	Phone    *string `json:"phone,omitempty"`
	RoleID   *uint   `json:"role_id,omitempty"`
	Status   *string `json:"status,omitempty"`
}

// UserListQuery 用户列表查询参数
type UserListQuery struct {
	models.PaginationQuery
	Username *string `form:"username"`
	Email    *string `form:"email"`
	RealName *string `form:"real_name"`
	RoleID   *uint   `form:"role_id"`
	Status   *string `form:"status"`
}

// ResetPasswordRequest 重置密码请求
type ResetPasswordRequest struct {
	Password string `json:"password" binding:"required"`
}

// UpdateCurrentUserRequest 更新当前用户请求
type UpdateCurrentUserRequest struct {
	Email    *string `json:"email,omitempty" binding:"omitempty,email"`
	RealName *string `json:"real_name,omitempty"`
	Phone    *string `json:"phone,omitempty"`
}

// AssignRolesRequest 分配角色请求
type AssignRolesRequest struct {
	RoleIDs []uint `json:"role_ids" binding:"required" example:"[1,2,3]"`
}

// ListUsers 获取用户列表
// @Summary 获取用户列表
// @Description 分页获取用户列表，支持多条件筛选
// @Tags 用户管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页大小" default(10)
// @Param username query string false "用户名"
// @Param email query string false "邮箱"
// @Param real_name query string false "真实姓名"
// @Param role_id query int false "角色ID"
// @Param status query string false "状态"
// @Success 200 {object} models.Response{data=models.PaginatedList{items=[]models.UserInfo}} "获取成功"
// @Router /api/v1/users [get]
func (h *UserHandler) ListUsers(c *gin.Context) {
	var query UserListQuery
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
	db := database.DB.Model(&models.User{}).Preload("Role")

	// 应用筛选条件
	if query.Username != nil && *query.Username != "" {
		db = db.Where("username LIKE ?", "%"+*query.Username+"%")
	}
	if query.Email != nil && *query.Email != "" {
		db = db.Where("email LIKE ?", "%"+*query.Email+"%")
	}
	if query.RealName != nil && *query.RealName != "" {
		db = db.Where("real_name LIKE ?", "%"+*query.RealName+"%")
	}
	if query.RoleID != nil {
		db = db.Where("role_id = ?", *query.RoleID)
	}
	if query.Status != nil && *query.Status != "" {
		db = db.Where("status = ?", *query.Status)
	}

	// 获取总数
	var total int64
	if err := db.Count(&total).Error; err != nil {
		h.logger.Error("Failed to count users", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询失败"))
		return
	}

	// 分页查询
	var users []models.User
	offset := (query.Page - 1) * query.PageSize
	if err := db.Offset(offset).Limit(query.PageSize).Order("created_at DESC").Find(&users).Error; err != nil {
		h.logger.Error("Failed to list users", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询失败"))
		return
	}

	// 转换为用户信息
	userInfos := make([]*models.UserInfo, len(users))
	for i, user := range users {
		userInfos[i] = user.ToUserInfo()
	}

	result := models.NewPageResponse(userInfos, total, query.Page, query.PageSize)

	c.JSON(http.StatusOK, models.SuccessResponse(result))
}

// GetUser 获取用户详情
// @Summary 获取用户详情
// @Description 根据用户ID获取详细信息
// @Tags 用户管理
// @Produce json
// @Security BearerAuth
// @Param id path int true "用户ID"
// @Success 200 {object} models.Response{data=models.UserInfo} "获取成功"
// @Failure 404 {object} models.Response "用户不存在"
// @Router /api/v1/users/{id} [get]
func (h *UserHandler) GetUser(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "用户ID无效"))
		return
	}

	var user models.User
	if err := database.DB.Where("id = ?", id).Preload("Role").First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, models.ErrorResponse(http.StatusNotFound, "用户不存在"))
		} else {
			h.logger.Error("Failed to get user", zap.Error(err))
			c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询失败"))
		}
		return
	}

	userInfo := user.ToUserInfo()
	c.JSON(http.StatusOK, models.SuccessResponse(userInfo))
}

// CreateUser 创建用户
// @Summary 创建用户
// @Description 创建新用户账户
// @Tags 用户管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body CreateUserRequest true "创建用户请求"
// @Success 201 {object} models.Response{data=models.UserInfo} "创建成功"
// @Failure 400 {object} models.Response "请求参数错误"
// @Failure 409 {object} models.Response "用户名或邮箱已存在"
// @Router /api/v1/users [post]
func (h *UserHandler) CreateUser(c *gin.Context) {
	var req CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "请求参数错误"))
		return
	}

	// 验证密码强度
	if err := h.passwordManager.ValidatePasswordStrength(req.Password); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "密码强度不足"))
		return
	}

	// 检查用户名是否存在
	var existingUser models.User
	if err := database.DB.Where("username = ? OR email = ?", req.Username, req.Email).First(&existingUser).Error; err == nil {
		if existingUser.Username == req.Username {
			c.JSON(http.StatusConflict, models.ErrorResponse(http.StatusConflict, "用户名已存在"))
		} else {
			c.JSON(http.StatusConflict, models.ErrorResponse(http.StatusConflict, "邮箱已存在"))
		}
		return
	}

	// 验证角色是否存在
	var role models.Role
	if err := database.DB.Where("id = ?", req.RoleID).First(&role).Error; err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "角色不存在"))
		return
	}

	// 哈希密码
	hashedPassword, err := h.passwordManager.HashPassword(req.Password)
	if err != nil {
		h.logger.Error("Failed to hash password", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "密码加密失败"))
		return
	}

	// 创建用户
	user := models.User{
		Username: req.Username,
		Email:    req.Email,
		RealName: req.RealName,
		Phone:    req.Phone,
		Password: hashedPassword,
		Status:   models.UserStatusActive,
	}

	if err := database.DB.Create(&user).Error; err != nil {
		h.logger.Error("Failed to create user", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "创建用户失败"))
		return
	}

	// 添加用户角色关联
	userRole := models.UserRole{
		UserID: user.ID,
		RoleID: req.RoleID,
	}
	if err := database.DB.Create(&userRole).Error; err != nil {
		h.logger.Error("Failed to create user role", zap.Error(err))
		// 不返回错误，只记录日志
	}

	// 加载角色信息
	if err := database.DB.Preload("Role").First(&user, user.ID).Error; err != nil {
		h.logger.Error("Failed to load user role", zap.Error(err))
	}

	userInfo := user.ToUserInfo()
	h.logger.Info("User created successfully", zap.Uint("user_id", user.ID), zap.String("username", user.Username))
	c.JSON(http.StatusCreated, models.SuccessResponse(userInfo))
}

// UpdateUser 更新用户
// @Summary 更新用户
// @Description 更新用户信息
// @Tags 用户管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "用户ID"
// @Param request body UpdateUserRequest true "更新用户请求"
// @Success 200 {object} models.Response{data=models.UserInfo} "更新成功"
// @Failure 400 {object} models.Response "请求参数错误"
// @Failure 404 {object} models.Response "用户不存在"
// @Router /api/v1/users/{id} [put]
func (h *UserHandler) UpdateUser(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "用户ID无效"))
		return
	}

	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "请求参数错误"))
		return
	}

	// 查找用户
	var user models.User
	if err := database.DB.Where("id = ?", id).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, models.ErrorResponse(http.StatusNotFound, "用户不存在"))
		} else {
			h.logger.Error("Failed to find user", zap.Error(err))
			c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询失败"))
		}
		return
	}

	// 更新字段
	if req.Email != nil {
		// 检查邮箱是否已存在
		var existingUser models.User
		if err := database.DB.Where("email = ? AND id != ?", *req.Email, id).First(&existingUser).Error; err == nil {
			c.JSON(http.StatusConflict, models.ErrorResponse(http.StatusConflict, "邮箱已存在"))
			return
		}
		user.Email = *req.Email
	}
	if req.RealName != nil {
		user.RealName = *req.RealName
	}
	if req.Phone != nil {
		user.Phone = *req.Phone
	}
	if req.RoleID != nil {
		// 验证角色是否存在
		var role models.Role
		if err := database.DB.Where("id = ?", *req.RoleID).First(&role).Error; err != nil {
			c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "角色不存在"))
			return
		}

		// 更新用户角色关联
		// 先删除旧的角色关联
		database.DB.Where("user_id = ?", id).Delete(&models.UserRole{})
		// 创建新的角色关联
		userRole := models.UserRole{
			UserID: uint(id),
			RoleID: *req.RoleID,
		}
		if err := database.DB.Create(&userRole).Error; err != nil {
			h.logger.Error("Failed to update user role", zap.Error(err))
		}
	}
	if req.Status != nil {
		var status int
		switch *req.Status {
		case "active":
			status = models.UserStatusActive
		case "inactive":
			status = models.UserStatusInactive
		default:
			c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "状态值无效"))
			return
		}
		user.Status = status
	}

	// 保存更新
	if err := database.DB.Save(&user).Error; err != nil {
		h.logger.Error("Failed to update user", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "更新失败"))
		return
	}

	// 加载角色信息
	if err := database.DB.Preload("Role").First(&user, user.ID).Error; err != nil {
		h.logger.Error("Failed to load user role", zap.Error(err))
	}

	userInfo := user.ToUserInfo()
	h.logger.Info("User updated successfully", zap.Uint("user_id", user.ID))
	c.JSON(http.StatusOK, models.SuccessResponse(userInfo))
}

// DeleteUser 删除用户
// @Summary 删除用户
// @Description 软删除用户账户
// @Tags 用户管理
// @Produce json
// @Security BearerAuth
// @Param id path int true "用户ID"
// @Success 200 {object} models.Response "删除成功"
// @Failure 400 {object} models.Response "用户ID无效"
// @Failure 404 {object} models.Response "用户不存在"
// @Router /api/v1/users/{id} [delete]
func (h *UserHandler) DeleteUser(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "用户ID无效"))
		return
	}

	// 获取当前用户ID，防止删除自己
	currentUserID, _ := c.Get("user_id")
	if currentUserID == uint(id) {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "不能删除自己"))
		return
	}

	// 查找用户
	var user models.User
	if err := database.DB.Where("id = ?", id).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, models.ErrorResponse(http.StatusNotFound, "用户不存在"))
		} else {
			h.logger.Error("Failed to find user", zap.Error(err))
			c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询失败"))
		}
		return
	}

	// 软删除用户
	if err := database.DB.Delete(&user).Error; err != nil {
		h.logger.Error("Failed to delete user", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "删除失败"))
		return
	}

	h.logger.Info("User deleted successfully", zap.Uint("user_id", user.ID))
	c.JSON(http.StatusOK, models.SuccessResponse(nil))
}

// ResetPassword 重置用户密码
// @Summary 重置用户密码
// @Description 管理员重置用户密码
// @Tags 用户管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "用户ID"
// @Param request body ResetPasswordRequest true "重置密码请求"
// @Success 200 {object} models.Response "重置成功"
// @Failure 400 {object} models.Response "请求参数错误"
// @Failure 404 {object} models.Response "用户不存在"
// @Router /api/v1/users/{id}/password [put]
func (h *UserHandler) ResetPassword(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "用户ID无效"))
		return
	}

	var req ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "请求参数错误"))
		return
	}

	// 验证密码强度
	if err := h.passwordManager.ValidatePasswordStrength(req.Password); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "密码强度不足"))
		return
	}

	// 查找用户
	var user models.User
	if err := database.DB.Where("id = ?", id).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, models.ErrorResponse(http.StatusNotFound, "用户不存在"))
		} else {
			h.logger.Error("Failed to find user", zap.Error(err))
			c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询失败"))
		}
		return
	}

	// 哈希新密码
	hashedPassword, err := h.passwordManager.HashPassword(req.Password)
	if err != nil {
		h.logger.Error("Failed to hash password", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "密码加密失败"))
		return
	}

	// 更新密码
	user.Password = hashedPassword
	if err := database.DB.Save(&user).Error; err != nil {
		h.logger.Error("Failed to reset password", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "密码重置失败"))
		return
	}

	h.logger.Info("Password reset successfully", zap.Uint("user_id", user.ID))
	c.JSON(http.StatusOK, models.SuccessResponse(nil))
}

// ChangeUserStatus 修改用户状态
// @Summary 修改用户状态
// @Description 启用或禁用用户账户
// @Tags 用户管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "用户ID"
// @Param status query string true "状态" Enums(active,inactive)
// @Success 200 {object} models.Response "修改成功"
// @Failure 400 {object} models.Response "参数错误"
// @Failure 404 {object} models.Response "用户不存在"
// @Router /api/v1/users/{id}/status [put]
func (h *UserHandler) ChangeUserStatus(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "用户ID无效"))
		return
	}

	statusStr := c.Query("status")
	var status int
	switch statusStr {
	case "active":
		status = models.UserStatusActive
	case "inactive":
		status = models.UserStatusInactive
	default:
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "状态值无效"))
		return
	}

	// 获取当前用户ID，防止禁用自己
	currentUserID, _ := c.Get("user_id")
	if currentUserID == uint(id) && status == models.UserStatusInactive {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "不能禁用自己"))
		return
	}

	// 查找用户
	var user models.User
	if err := database.DB.Where("id = ?", id).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, models.ErrorResponse(http.StatusNotFound, "用户不存在"))
		} else {
			h.logger.Error("Failed to find user", zap.Error(err))
			c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询失败"))
		}
		return
	}

	// 更新状态
	user.Status = status
	if err := database.DB.Save(&user).Error; err != nil {
		h.logger.Error("Failed to update user status", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "状态更新失败"))
		return
	}

	h.logger.Info("User status changed", zap.Uint("user_id", user.ID), zap.Int("status", status))
	c.JSON(http.StatusOK, models.SuccessResponse(nil))
}

// GetUserStats 获取用户统计信息
// @Summary 获取用户统计信息
// @Description 获取用户总数、活跃用户数等统计信息
// @Tags 用户管理
// @Produce json
// @Security BearerAuth
// @Success 200 {object} models.Response{data=object} "获取成功"
// @Router /api/v1/users/stats [get]
func (h *UserHandler) GetUserStats(c *gin.Context) {
	// 获取用户总数
	var totalUsers int64
	if err := database.DB.Model(&models.User{}).Count(&totalUsers).Error; err != nil {
		h.logger.Error("Failed to count total users", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询失败"))
		return
	}

	// 获取活跃用户数
	var activeUsers int64
	if err := database.DB.Model(&models.User{}).Where("status = ?", models.UserStatusActive).Count(&activeUsers).Error; err != nil {
		h.logger.Error("Failed to count active users", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询失败"))
		return
	}

	// 获取非活跃用户数
	var inactiveUsers int64
	if err := database.DB.Model(&models.User{}).Where("status = ?", models.UserStatusInactive).Count(&inactiveUsers).Error; err != nil {
		h.logger.Error("Failed to count inactive users", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询失败"))
		return
	}

	// 获取今日新增用户数
	var todayNewUsers int64
	today := database.DB.Where("DATE(created_at) = DATE(NOW())")
	if err := today.Model(&models.User{}).Count(&todayNewUsers).Error; err != nil {
		h.logger.Error("Failed to count today new users", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询失败"))
		return
	}

	stats := map[string]interface{}{
		"total_users":     totalUsers,
		"active_users":    activeUsers,
		"inactive_users":  inactiveUsers,
		"today_new_users": todayNewUsers,
	}

	c.JSON(http.StatusOK, models.SuccessResponse(stats))
}

// GetCurrentUser 获取当前用户信息
// @Summary 获取当前用户信息
// @Description 获取当前登录用户的详细信息
// @Tags 用户管理
// @Produce json
// @Security BearerAuth
// @Success 200 {object} models.Response{data=models.UserInfo} "获取成功"
// @Router /api/v1/users/current [get]
func (h *UserHandler) GetCurrentUser(c *gin.Context) {
	// 从中间件获取当前用户ID
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse(http.StatusUnauthorized, "未授权"))
		return
	}

	var user models.User
	if err := database.DB.Where("id = ?", userID).Preload("Role").First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, models.ErrorResponse(http.StatusNotFound, "用户不存在"))
		} else {
			h.logger.Error("Failed to get current user", zap.Error(err))
			c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询失败"))
		}
		return
	}

	userInfo := user.ToUserInfo()
	c.JSON(http.StatusOK, models.SuccessResponse(userInfo))
}

// UpdateCurrentUser 更新当前用户信息
// @Summary 更新当前用户信息
// @Description 当前用户更新自己的基本信息
// @Tags 用户管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body UpdateCurrentUserRequest true "更新当前用户请求"
// @Success 200 {object} models.Response{data=models.UserInfo} "更新成功"
// @Router /api/v1/users/current [put]
func (h *UserHandler) UpdateCurrentUser(c *gin.Context) {
	// 从中间件获取当前用户ID
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse(http.StatusUnauthorized, "未授权"))
		return
	}

	var req UpdateCurrentUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "请求参数错误"))
		return
	}

	// 查找用户
	var user models.User
	if err := database.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, models.ErrorResponse(http.StatusNotFound, "用户不存在"))
		} else {
			h.logger.Error("Failed to find current user", zap.Error(err))
			c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询失败"))
		}
		return
	}

	// 更新允许的字段
	if req.Email != nil {
		// 检查邮箱是否已存在
		var existingUser models.User
		if err := database.DB.Where("email = ? AND id != ?", *req.Email, userID).First(&existingUser).Error; err == nil {
			c.JSON(http.StatusConflict, models.ErrorResponse(http.StatusConflict, "邮箱已存在"))
			return
		}
		user.Email = *req.Email
	}
	if req.RealName != nil {
		user.RealName = *req.RealName
	}
	if req.Phone != nil {
		user.Phone = *req.Phone
	}

	// 保存更新
	if err := database.DB.Save(&user).Error; err != nil {
		h.logger.Error("Failed to update current user", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "更新失败"))
		return
	}

	// 加载角色信息
	if err := database.DB.Preload("Role").First(&user, user.ID).Error; err != nil {
		h.logger.Error("Failed to load user role", zap.Error(err))
	}

	userInfo := user.ToUserInfo()
	h.logger.Info("Current user updated successfully", zap.Uint("user_id", user.ID))
	c.JSON(http.StatusOK, models.SuccessResponse(userInfo))
}

// GetUserRoles 获取用户角色列表
// @Summary 获取用户角色列表
// @Description 获取指定用户的所有角色
// @Tags 用户管理
// @Produce json
// @Security BearerAuth
// @Param id path int true "用户ID"
// @Success 200 {object} models.Response{data=[]models.Role} "获取成功"
// @Router /api/v1/users/{id}/roles [get]
func (h *UserHandler) GetUserRoles(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "用户ID无效"))
		return
	}

	// 查找用户
	var user models.User
	if err := database.DB.Where("id = ?", id).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, models.ErrorResponse(http.StatusNotFound, "用户不存在"))
		} else {
			h.logger.Error("Failed to find user", zap.Error(err))
			c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询失败"))
		}
		return
	}

	// 获取用户角色
	var roles []models.Role
	if err := database.DB.Table("roles").
		Joins("JOIN user_roles ON roles.id = user_roles.role_id").
		Where("user_roles.user_id = ?", id).
		Find(&roles).Error; err != nil {
		h.logger.Error("Failed to get user roles", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询失败"))
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse(roles))
}

// AssignRoles 分配用户角色
// @Summary 分配用户角色
// @Description 为用户分配多个角色
// @Tags 用户管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "用户ID"
// @Param request body AssignRolesRequest true "分配角色请求"
// @Success 200 {object} models.Response "分配成功"
// @Router /api/v1/users/{id}/roles [put]
func (h *UserHandler) AssignRoles(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "用户ID无效"))
		return
	}

	var req AssignRolesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "请求参数错误"))
		return
	}

	// 查找用户
	var user models.User
	if err := database.DB.Where("id = ?", id).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, models.ErrorResponse(http.StatusNotFound, "用户不存在"))
		} else {
			h.logger.Error("Failed to find user", zap.Error(err))
			c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询失败"))
		}
		return
	}

	// 验证所有角色是否存在
	var existingRoles []models.Role
	if err := database.DB.Where("id IN ?", req.RoleIDs).Find(&existingRoles).Error; err != nil {
		h.logger.Error("Failed to verify roles", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "角色验证失败"))
		return
	}

	if len(existingRoles) != len(req.RoleIDs) {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "存在无效的角色ID"))
		return
	}

	// 使用事务处理角色分配
	tx := database.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 删除用户的旧角色关联
	if err := tx.Where("user_id = ?", id).Delete(&models.UserRole{}).Error; err != nil {
		tx.Rollback()
		h.logger.Error("Failed to delete old user roles", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "删除旧角色失败"))
		return
	}

	// 创建新的角色关联
	for _, roleID := range req.RoleIDs {
		userRole := models.UserRole{
			UserID: uint(id),
			RoleID: roleID,
		}
		if err := tx.Create(&userRole).Error; err != nil {
			tx.Rollback()
			h.logger.Error("Failed to create user role", zap.Error(err))
			c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "分配角色失败"))
			return
		}
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		h.logger.Error("Failed to commit transaction", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "事务提交失败"))
		return
	}

	h.logger.Info("User roles assigned successfully", zap.Uint("user_id", uint(id)), zap.Uints("role_ids", req.RoleIDs))
	c.JSON(http.StatusOK, models.SuccessResponse(nil))
}