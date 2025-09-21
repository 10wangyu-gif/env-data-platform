package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/env-data-platform/internal/auth"
	"github.com/env-data-platform/internal/config"
	"github.com/env-data-platform/internal/database"
	"github.com/env-data-platform/internal/models"
)

// AuthMiddleware JWT认证中间件
func AuthMiddleware(cfg *config.Config, logger *zap.Logger) gin.HandlerFunc {
	jwtManager := auth.NewJWTManager(cfg)

	return func(c *gin.Context) {
		// 获取Authorization头
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			logger.Warn("Missing authorization header")
			c.JSON(http.StatusUnauthorized, models.ErrorResponse(http.StatusUnauthorized, "缺少认证令牌"))
			c.Abort()
			return
		}

		// 检查Bearer前缀
		tokenParts := strings.SplitN(authHeader, " ", 2)
		if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
			logger.Warn("Invalid authorization header format")
			c.JSON(http.StatusUnauthorized, models.ErrorResponse(http.StatusUnauthorized, "认证令牌格式错误"))
			c.Abort()
			return
		}

		// 解析令牌
		claims, err := jwtManager.ParseToken(tokenParts[1])
		if err != nil {
			logger.Warn("Invalid token", zap.Error(err))
			c.JSON(http.StatusUnauthorized, models.ErrorResponse(http.StatusUnauthorized, "认证令牌无效"))
			c.Abort()
			return
		}

		// 将用户信息存储到上下文
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("role_id", claims.RoleID)
		c.Set("role_name", claims.RoleName)

		c.Next()
	}
}

// RequirePermission 权限检查中间件（基于角色权限）
func RequirePermission(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取用户角色ID
		roleID, exists := c.Get("role_id")
		if !exists {
			c.JSON(http.StatusUnauthorized, models.ErrorResponse(http.StatusUnauthorized, "用户未登录"))
			c.Abort()
			return
		}

		// 超级管理员拥有所有权限
		roleName, _ := c.Get("role_name")
		if roleName == "超级管理员" || roleName == "admin" {
			c.Next()
			return
		}

		// 查询角色是否拥有指定权限
		hasPermission, err := checkRolePermission(roleID.(uint), permission)
		if err != nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "权限检查失败"))
			c.Abort()
			return
		}

		if !hasPermission {
			c.JSON(http.StatusForbidden, models.ErrorResponse(http.StatusForbidden, "权限不足"))
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireUserPermission 用户权限检查中间件（检查用户直接权限和角色权限）
func RequireUserPermission(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取用户ID
		userID, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusUnauthorized, models.ErrorResponse(http.StatusUnauthorized, "用户未登录"))
			c.Abort()
			return
		}

		// 超级管理员拥有所有权限
		roleName, _ := c.Get("role_name")
		if roleName == "超级管理员" || roleName == "admin" {
			c.Next()
			return
		}

		// 查询用户是否拥有指定权限（直接权限或通过角色）
		hasPermission, err := checkUserPermission(userID.(uint), permission)
		if err != nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "权限检查失败"))
			c.Abort()
			return
		}

		if !hasPermission {
			c.JSON(http.StatusForbidden, models.ErrorResponse(http.StatusForbidden, "权限不足"))
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireRole 角色检查中间件
func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取用户角色
		roleName, exists := c.Get("role_name")
		if !exists {
			c.JSON(http.StatusUnauthorized, models.ErrorResponse(http.StatusUnauthorized, "用户未登录"))
			c.Abort()
			return
		}

		// 检查角色是否匹配
		roleStr := roleName.(string)
		for _, role := range roles {
			if roleStr == role {
				c.Next()
				return
			}
		}

		c.JSON(http.StatusForbidden, models.ErrorResponse(http.StatusForbidden, "权限不足"))
		c.Abort()
	}
}

// OptionalAuth 可选认证中间件
func OptionalAuth(cfg *config.Config, logger *zap.Logger) gin.HandlerFunc {
	jwtManager := auth.NewJWTManager(cfg)

	return func(c *gin.Context) {
		// 获取Authorization头
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		// 检查Bearer前缀
		tokenParts := strings.SplitN(authHeader, " ", 2)
		if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
			c.Next()
			return
		}

		// 解析令牌
		claims, err := jwtManager.ParseToken(tokenParts[1])
		if err != nil {
			c.Next()
			return
		}

		// 将用户信息存储到上下文
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("role_id", claims.RoleID)
		c.Set("role_name", claims.RoleName)

		c.Next()
	}
}

// checkRolePermission 检查角色是否拥有指定权限
func checkRolePermission(roleID uint, permissionCode string) (bool, error) {
	var count int64

	// 查询角色权限关联表，检查是否存在指定权限
	err := database.DB.Table("role_permissions rp").
		Joins("JOIN permissions p ON rp.permission_id = p.id").
		Where("rp.role_id = ? AND p.code = ? AND p.is_enabled = ?", roleID, permissionCode, true).
		Count(&count).Error

	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// checkUserPermission 检查用户是否拥有指定权限（直接权限或通过角色）
func checkUserPermission(userID uint, permissionCode string) (bool, error) {
	var count int64

	// 先检查用户直接权限
	err := database.DB.Table("user_permissions up").
		Joins("JOIN permissions p ON up.permission_id = p.id").
		Where("up.user_id = ? AND p.code = ? AND p.is_enabled = ?", userID, permissionCode, true).
		Count(&count).Error

	if err != nil {
		return false, err
	}

	if count > 0 {
		return true, nil
	}

	// 再检查角色权限
	err = database.DB.Table("user_roles ur").
		Joins("JOIN role_permissions rp ON ur.role_id = rp.role_id").
		Joins("JOIN permissions p ON rp.permission_id = p.id").
		Joins("JOIN roles r ON ur.role_id = r.id").
		Where("ur.user_id = ? AND p.code = ? AND p.is_enabled = ? AND r.is_enabled = ?",
			userID, permissionCode, true, true).
		Count(&count).Error

	if err != nil {
		return false, err
	}

	return count > 0, nil
}