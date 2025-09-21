package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/env-data-platform/internal/auth"
	"github.com/env-data-platform/internal/config"
	"github.com/env-data-platform/internal/database"
	"github.com/env-data-platform/internal/models"
)

// AuthHandler 认证处理器
type AuthHandler struct {
	logger          *zap.Logger
	jwtManager      *auth.JWTManager
	passwordManager *auth.PasswordManager
}

// NewAuthHandler 创建认证处理器
func NewAuthHandler(cfg *config.Config, logger *zap.Logger) *AuthHandler {
	return &AuthHandler{
		logger:          logger,
		jwtManager:      auth.NewJWTManager(cfg),
		passwordManager: auth.NewPasswordManager(),
	}
}

// LoginRequest 登录请求
type LoginRequest struct {
	Username string `json:"username" binding:"required" example:"admin"`
	Password string `json:"password" binding:"required" example:"password"`
}

// LoginResponse 登录响应
type LoginResponse struct {
	Token     string           `json:"token"`
	ExpiresAt time.Time        `json:"expires_at"`
	User      *models.UserInfo `json:"user"`
}

// RefreshRequest 刷新令牌请求
type RefreshRequest struct {
	Token string `json:"token" binding:"required"`
}

// ChangePasswordRequest 修改密码请求
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required"`
}

// Login 用户登录
// @Summary 用户登录
// @Description 用户登录获取访问令牌
// @Tags 认证
// @Accept json
// @Produce json
// @Param request body LoginRequest true "登录请求"
// @Success 200 {object} models.Response{data=LoginResponse} "登录成功"
// @Failure 400 {object} models.Response "请求参数错误"
// @Failure 401 {object} models.Response "用户名或密码错误"
// @Router /api/v1/auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Invalid login request", zap.Error(err))
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "请求参数错误"))
		return
	}

	// 查找用户
	var user models.User
	if err := database.DB.Where("username = ? OR email = ?", req.Username, req.Username).
		Preload("Roles").First(&user).Error; err != nil {
		h.logger.Warn("User not found", zap.String("username", req.Username))
		c.JSON(http.StatusUnauthorized, models.ErrorResponse(http.StatusUnauthorized, "用户名或密码错误"))
		return
	}

	// 检查用户状态
	if user.Status != models.UserStatusActive {
		h.logger.Warn("User account disabled", zap.Uint("user_id", user.ID))
		c.JSON(http.StatusUnauthorized, models.ErrorResponse(http.StatusUnauthorized, "账户已被禁用"))
		return
	}

	// 验证密码
	valid, err := h.passwordManager.VerifyPassword(req.Password, user.Password)
	if err != nil {
		h.logger.Error("Password verification failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "密码验证失败"))
		return
	}

	if !valid {
		h.logger.Warn("Invalid password", zap.String("username", req.Username))
		c.JSON(http.StatusUnauthorized, models.ErrorResponse(http.StatusUnauthorized, "用户名或密码错误"))
		return
	}

	// 生成JWT令牌
	token, err := h.jwtManager.GenerateToken(user.ID, user.Username, user.GetRoleID(), user.GetRoleName())
	if err != nil {
		h.logger.Error("Failed to generate token", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "生成令牌失败"))
		return
	}

	// 更新最后登录时间
	now := time.Now()
	user.LastLoginAt = &now
	if err := database.DB.Save(&user).Error; err != nil {
		h.logger.Error("Failed to update last login time", zap.Error(err))
	}

	// 记录登录日志
	loginLog := models.LoginLog{
		UserID:    user.ID,
		Username:  user.Username,
		IP:        c.ClientIP(),
		UserAgent: c.GetHeader("User-Agent"),
		Status:    1, // 1表示成功
		Message:   "登录成功",
	}
	if err := database.DB.Create(&loginLog).Error; err != nil {
		h.logger.Error("Failed to create login log", zap.Error(err))
	}

	// 返回响应
	userInfo := user.ToUserInfo()
	response := LoginResponse{
		Token:     token,
		ExpiresAt: time.Now().Add(time.Hour * 24), // 这里应该从配置读取
		User:      userInfo,
	}

	h.logger.Info("User logged in successfully",
		zap.Uint("user_id", user.ID),
		zap.String("username", user.Username),
		zap.String("ip", c.ClientIP()))

	c.JSON(http.StatusOK, models.SuccessResponse(response))
}

// Logout 用户登出
// @Summary 用户登出
// @Description 用户登出（客户端清除令牌）
// @Tags 认证
// @Produce json
// @Security BearerAuth
// @Success 200 {object} models.Response "登出成功"
// @Router /api/v1/auth/logout [post]
func (h *AuthHandler) Logout(c *gin.Context) {
	// 获取当前用户信息
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse(http.StatusUnauthorized, "用户未登录"))
		return
	}

	// 记录登出日志
	loginLog := models.LoginLog{
		UserID:    userID.(uint),
		IP:        c.ClientIP(),
		UserAgent: c.GetHeader("User-Agent"),
		Status:    0, // 0表示登出（或失败）
		Message:   "登出成功",
	}
	if err := database.DB.Create(&loginLog).Error; err != nil {
		h.logger.Error("Failed to create logout log", zap.Error(err))
	}

	h.logger.Info("User logged out", zap.Any("user_id", userID))
	c.JSON(http.StatusOK, models.SuccessResponse(nil))
}

// GetMe 获取当前用户信息
// @Summary 获取当前用户信息
// @Description 获取当前登录用户的详细信息
// @Tags 认证
// @Produce json
// @Security BearerAuth
// @Success 200 {object} models.Response{data=models.UserInfo} "获取成功"
// @Failure 401 {object} models.Response "用户未登录"
// @Router /api/v1/auth/me [get]
func (h *AuthHandler) GetMe(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse(http.StatusUnauthorized, "用户未登录"))
		return
	}

	// 查询用户信息
	var user models.User
	if err := database.DB.Where("id = ?", userID).
		Preload("Roles").First(&user).Error; err != nil {
		h.logger.Error("Failed to get user info", zap.Error(err))
		c.JSON(http.StatusNotFound, models.ErrorResponse(http.StatusNotFound, "用户不存在"))
		return
	}

	userInfo := user.ToUserInfo()
	c.JSON(http.StatusOK, models.SuccessResponse(userInfo))
}

// RefreshToken 刷新令牌
// @Summary 刷新访问令牌
// @Description 使用现有令牌刷新获取新的访问令牌
// @Tags 认证
// @Accept json
// @Produce json
// @Param request body RefreshRequest true "刷新请求"
// @Success 200 {object} models.Response{data=LoginResponse} "刷新成功"
// @Failure 400 {object} models.Response "请求参数错误"
// @Failure 401 {object} models.Response "令牌无效"
// @Router /api/v1/auth/refresh [post]
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "请求参数错误"))
		return
	}

	// 刷新令牌
	newToken, err := h.jwtManager.RefreshToken(req.Token)
	if err != nil {
		h.logger.Warn("Failed to refresh token", zap.Error(err))
		c.JSON(http.StatusUnauthorized, models.ErrorResponse(http.StatusUnauthorized, "令牌刷新失败"))
		return
	}

	// 解析新令牌获取用户信息
	claims, err := h.jwtManager.ParseToken(newToken)
	if err != nil {
		h.logger.Error("Failed to parse new token", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "令牌解析失败"))
		return
	}

	// 查询用户信息
	var user models.User
	if err := database.DB.Where("id = ?", claims.UserID).
		Preload("Roles").First(&user).Error; err != nil {
		h.logger.Error("Failed to get user info", zap.Error(err))
		c.JSON(http.StatusNotFound, models.ErrorResponse(http.StatusNotFound, "用户不存在"))
		return
	}

	userInfo := user.ToUserInfo()
	response := LoginResponse{
		Token:     newToken,
		ExpiresAt: claims.ExpiresAt.Time,
		User:      userInfo,
	}

	h.logger.Info("Token refreshed successfully", zap.Uint("user_id", claims.UserID))
	c.JSON(http.StatusOK, models.SuccessResponse(response))
}

// ChangePassword 修改密码
// @Summary 修改密码
// @Description 修改当前用户的密码
// @Tags 认证
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body ChangePasswordRequest true "修改密码请求"
// @Success 200 {object} models.Response "修改成功"
// @Failure 400 {object} models.Response "请求参数错误"
// @Failure 401 {object} models.Response "原密码错误"
// @Router /api/v1/auth/password [put]
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse(http.StatusUnauthorized, "用户未登录"))
		return
	}

	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "请求参数错误"))
		return
	}

	// 验证新密码强度
	if err := h.passwordManager.ValidatePasswordStrength(req.NewPassword); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "密码强度不足"))
		return
	}

	// 查询用户
	var user models.User
	if err := database.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse(http.StatusNotFound, "用户不存在"))
		return
	}

	// 验证原密码
	valid, err := h.passwordManager.VerifyPassword(req.OldPassword, user.Password)
	if err != nil {
		h.logger.Error("Password verification failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "密码验证失败"))
		return
	}

	if !valid {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse(http.StatusUnauthorized, "原密码错误"))
		return
	}

	// 哈希新密码
	hashedPassword, err := h.passwordManager.HashPassword(req.NewPassword)
	if err != nil {
		h.logger.Error("Failed to hash password", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "密码加密失败"))
		return
	}

	// 更新密码
	user.Password = hashedPassword
	if err := database.DB.Save(&user).Error; err != nil {
		h.logger.Error("Failed to update password", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "密码更新失败"))
		return
	}

	h.logger.Info("Password changed successfully", zap.Uint("user_id", user.ID))
	c.JSON(http.StatusOK, models.SuccessResponse(nil))
}