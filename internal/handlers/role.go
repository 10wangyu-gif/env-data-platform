package handlers

import (
	"net/http"
	"strconv"

	"github.com/env-data-platform/internal/database"
	"github.com/env-data-platform/internal/models"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// RoleHandler 角色处理器
type RoleHandler struct {
	db     *gorm.DB
	logger *zap.Logger
}

// NewRoleHandler 创建角色处理器
func NewRoleHandler(logger *zap.Logger) *RoleHandler {
	return &RoleHandler{
		db:     database.GetDB(),
		logger: logger,
	}
}

// ListRoles 获取角色列表
func (h *RoleHandler) ListRoles(c *gin.Context) {
	var req struct {
		Page     int    `form:"page" binding:"required,min=1"`
		PageSize int    `form:"page_size" binding:"required,min=1,max=100"`
		Name     string `form:"name"`
		Code     string `form:"code"`
		Status   *int   `form:"status"`
	}

	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "参数错误"))
		return
	}

	query := h.db.Model(&models.Role{})

	if req.Name != "" {
		query = query.Where("name LIKE ?", "%"+req.Name+"%")
	}
	if req.Code != "" {
		query = query.Where("code LIKE ?", "%"+req.Code+"%")
	}
	if req.Status != nil {
		query = query.Where("status = ?", *req.Status)
	}

	var total int64
	query.Count(&total)

	var roles []models.Role
	offset := (req.Page - 1) * req.PageSize
	if err := query.Offset(offset).Limit(req.PageSize).
		Preload("Permissions").
		Order("sort ASC, id ASC").
		Find(&roles).Error; err != nil {
		h.logger.Error("Failed to list roles", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询失败"))
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse(gin.H{
		"list":      roles,
		"total":     total,
		"page":      req.Page,
		"page_size": req.PageSize,
	}))
}

// CreateRole 创建角色
func (h *RoleHandler) CreateRole(c *gin.Context) {
	var req struct {
		Name          string `json:"name" binding:"required,min=1,max=50"`
		Code          string `json:"code" binding:"required,min=1,max=50"`
		Description   string `json:"description"`
		Status        int    `json:"status"`
		Sort          int    `json:"sort"`
		PermissionIDs []uint `json:"permission_ids"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "参数错误"))
		return
	}

	// 检查角色名称和代码是否已存在
	var existingRole models.Role
	if err := h.db.Where("name = ? OR code = ?", req.Name, req.Code).First(&existingRole).Error; err == nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "角色名称或代码已存在"))
		return
	}

	role := models.Role{
		Name:        req.Name,
		Code:        req.Code,
		Description: req.Description,
		Status:      req.Status,
		Sort:        req.Sort,
	}

	// 开启事务
	tx := h.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 创建角色
	if err := tx.Create(&role).Error; err != nil {
		tx.Rollback()
		h.logger.Error("Failed to create role", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "创建失败"))
		return
	}

	// 分配权限
	if len(req.PermissionIDs) > 0 {
		var permissions []models.Permission
		if err := tx.Where("id IN ?", req.PermissionIDs).Find(&permissions).Error; err != nil {
			tx.Rollback()
			h.logger.Error("Failed to find permissions", zap.Error(err))
			c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "权限查找失败"))
			return
		}

		if err := tx.Model(&role).Association("Permissions").Append(&permissions); err != nil {
			tx.Rollback()
			h.logger.Error("Failed to assign permissions", zap.Error(err))
			c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "权限分配失败"))
			return
		}
	}

	tx.Commit()

	// 重新加载角色信息
	h.db.Preload("Permissions").First(&role, role.ID)

	c.JSON(http.StatusOK, models.SuccessResponse(role))
}

// GetRole 获取角色详情
func (h *RoleHandler) GetRole(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "无效的ID"))
		return
	}

	var role models.Role
	if err := h.db.Preload("Permissions").Preload("Users").
		First(&role, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, models.ErrorResponse(http.StatusNotFound, "角色不存在"))
			return
		}
		h.logger.Error("Failed to get role", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询失败"))
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse(role))
}

// UpdateRole 更新角色
func (h *RoleHandler) UpdateRole(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "无效的ID"))
		return
	}

	var req struct {
		Name          string `json:"name" binding:"required,min=1,max=50"`
		Code          string `json:"code" binding:"required,min=1,max=50"`
		Description   string `json:"description"`
		Status        int    `json:"status"`
		Sort          int    `json:"sort"`
		PermissionIDs []uint `json:"permission_ids"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "参数错误"))
		return
	}

	var role models.Role
	if err := h.db.First(&role, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, models.ErrorResponse(http.StatusNotFound, "角色不存在"))
			return
		}
		h.logger.Error("Failed to get role", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询失败"))
		return
	}

	// 检查系统角色
	if role.IsSystem {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "系统角色不允许修改"))
		return
	}

	// 检查角色名称和代码是否被其他角色使用
	var existingRole models.Role
	if err := h.db.Where("(name = ? OR code = ?) AND id != ?", req.Name, req.Code, id).First(&existingRole).Error; err == nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "角色名称或代码已存在"))
		return
	}

	// 开启事务
	tx := h.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 更新角色信息
	updates := map[string]interface{}{
		"name":        req.Name,
		"code":        req.Code,
		"description": req.Description,
		"status":      req.Status,
		"sort":        req.Sort,
	}

	if err := tx.Model(&role).Updates(updates).Error; err != nil {
		tx.Rollback()
		h.logger.Error("Failed to update role", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "更新失败"))
		return
	}

	// 清除现有权限关联
	if err := tx.Model(&role).Association("Permissions").Clear(); err != nil {
		tx.Rollback()
		h.logger.Error("Failed to clear role permissions", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "权限清理失败"))
		return
	}

	// 重新分配权限
	if len(req.PermissionIDs) > 0 {
		var permissions []models.Permission
		if err := tx.Where("id IN ?", req.PermissionIDs).Find(&permissions).Error; err != nil {
			tx.Rollback()
			h.logger.Error("Failed to find permissions", zap.Error(err))
			c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "权限查找失败"))
			return
		}

		if err := tx.Model(&role).Association("Permissions").Append(&permissions); err != nil {
			tx.Rollback()
			h.logger.Error("Failed to assign permissions", zap.Error(err))
			c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "权限分配失败"))
			return
		}
	}

	tx.Commit()

	// 重新加载角色信息
	h.db.Preload("Permissions").First(&role, role.ID)

	c.JSON(http.StatusOK, models.SuccessResponse(role))
}

// DeleteRole 删除角色
func (h *RoleHandler) DeleteRole(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "无效的ID"))
		return
	}

	var role models.Role
	if err := h.db.First(&role, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, models.ErrorResponse(http.StatusNotFound, "角色不存在"))
			return
		}
		h.logger.Error("Failed to get role", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询失败"))
		return
	}

	// 检查系统角色
	if role.IsSystem {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "系统角色不允许删除"))
		return
	}

	// 检查是否有用户关联此角色
	var userCount int64
	h.db.Model(&models.UserRole{}).Where("role_id = ?", id).Count(&userCount)
	if userCount > 0 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "该角色下还有用户，无法删除"))
		return
	}

	// 开启事务删除角色及其权限关联
	tx := h.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 清除权限关联
	if err := tx.Model(&role).Association("Permissions").Clear(); err != nil {
		tx.Rollback()
		h.logger.Error("Failed to clear role permissions", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "权限清理失败"))
		return
	}

	// 删除角色
	if err := tx.Delete(&role).Error; err != nil {
		tx.Rollback()
		h.logger.Error("Failed to delete role", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "删除失败"))
		return
	}

	tx.Commit()

	c.JSON(http.StatusOK, models.SuccessResponse(gin.H{"message": "删除成功"}))
}

// GetRolePermissions 获取角色权限
func (h *RoleHandler) GetRolePermissions(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "无效的ID"))
		return
	}

	var role models.Role
	if err := h.db.Preload("Permissions").First(&role, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, models.ErrorResponse(http.StatusNotFound, "角色不存在"))
			return
		}
		h.logger.Error("Failed to get role", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询失败"))
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse(role.Permissions))
}

// AssignPermissions 分配权限给角色
func (h *RoleHandler) AssignPermissions(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "无效的ID"))
		return
	}

	var req struct {
		PermissionIDs []uint `json:"permission_ids" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "参数错误"))
		return
	}

	var role models.Role
	if err := h.db.First(&role, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, models.ErrorResponse(http.StatusNotFound, "角色不存在"))
			return
		}
		h.logger.Error("Failed to get role", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询失败"))
		return
	}

	// 检查系统角色
	if role.IsSystem {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "系统角色不允许修改权限"))
		return
	}

	// 开启事务
	tx := h.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 清除现有权限关联
	if err := tx.Model(&role).Association("Permissions").Clear(); err != nil {
		tx.Rollback()
		h.logger.Error("Failed to clear role permissions", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "权限清理失败"))
		return
	}

	// 分配新权限
	if len(req.PermissionIDs) > 0 {
		var permissions []models.Permission
		if err := tx.Where("id IN ? AND status = 1", req.PermissionIDs).Find(&permissions).Error; err != nil {
			tx.Rollback()
			h.logger.Error("Failed to find permissions", zap.Error(err))
			c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "权限查找失败"))
			return
		}

		if len(permissions) != len(req.PermissionIDs) {
			tx.Rollback()
			c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "部分权限不存在或已禁用"))
			return
		}

		if err := tx.Model(&role).Association("Permissions").Append(&permissions); err != nil {
			tx.Rollback()
			h.logger.Error("Failed to assign permissions", zap.Error(err))
			c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "权限分配失败"))
			return
		}
	}

	tx.Commit()

	// 重新加载角色权限
	h.db.Preload("Permissions").First(&role, role.ID)

	c.JSON(http.StatusOK, models.SuccessResponse(gin.H{
		"message":     "权限分配成功",
		"permissions": role.Permissions,
	}))
}

// GetRoleUsers 获取角色下的用户
func (h *RoleHandler) GetRoleUsers(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "无效的ID"))
		return
	}

	var req struct {
		Page     int `form:"page" binding:"required,min=1"`
		PageSize int `form:"page_size" binding:"required,min=1,max=100"`
	}

	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "参数错误"))
		return
	}

	// 检查角色是否存在
	var role models.Role
	if err := h.db.First(&role, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, models.ErrorResponse(http.StatusNotFound, "角色不存在"))
			return
		}
		h.logger.Error("Failed to get role", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询失败"))
		return
	}

	var total int64
	h.db.Model(&models.User{}).
		Joins("JOIN env_user_roles ON env_users.id = env_user_roles.user_id").
		Where("env_user_roles.role_id = ?", id).
		Count(&total)

	var users []models.User
	offset := (req.Page - 1) * req.PageSize
	if err := h.db.Model(&models.User{}).
		Joins("JOIN env_user_roles ON env_users.id = env_user_roles.user_id").
		Where("env_user_roles.role_id = ?", id).
		Offset(offset).Limit(req.PageSize).
		Find(&users).Error; err != nil {
		h.logger.Error("Failed to get role users", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询失败"))
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse(gin.H{
		"list":      users,
		"total":     total,
		"page":      req.Page,
		"page_size": req.PageSize,
	}))
}