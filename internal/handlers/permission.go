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

// PermissionHandler 权限处理器
type PermissionHandler struct {
	db     *gorm.DB
	logger *zap.Logger
}

// NewPermissionHandler 创建权限处理器
func NewPermissionHandler(logger *zap.Logger) *PermissionHandler {
	return &PermissionHandler{
		db:     database.GetDB(),
		logger: logger,
	}
}

// ListPermissions 获取权限列表
func (h *PermissionHandler) ListPermissions(c *gin.Context) {
	var req struct {
		Page     int    `form:"page"`
		PageSize int    `form:"page_size"`
		Name     string `form:"name"`
		Code     string `form:"code"`
		Type     string `form:"type"`
		Status   *int   `form:"status"`
		ParentID *uint  `form:"parent_id"`
		Tree     bool   `form:"tree"` // 是否返回树形结构
	}

	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "参数错误"))
		return
	}

	query := h.db.Model(&models.Permission{})

	if req.Name != "" {
		query = query.Where("name LIKE ?", "%"+req.Name+"%")
	}
	if req.Code != "" {
		query = query.Where("code LIKE ?", "%"+req.Code+"%")
	}
	if req.Type != "" {
		query = query.Where("type = ?", req.Type)
	}
	if req.Status != nil {
		query = query.Where("status = ?", *req.Status)
	}
	if req.ParentID != nil {
		query = query.Where("parent_id = ?", *req.ParentID)
	}

	// 如果请求树形结构，则返回根节点及其子节点
	if req.Tree {
		var permissions []models.Permission
		if err := query.Order("sort ASC, id ASC").Find(&permissions).Error; err != nil {
			h.logger.Error("Failed to list permissions", zap.Error(err))
			c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询失败"))
			return
		}

		// 构建树形结构
		tree := h.buildPermissionTree(permissions, nil)
		c.JSON(http.StatusOK, models.SuccessResponse(tree))
		return
	}

	// 分页查询
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}

	var total int64
	query.Count(&total)

	var permissions []models.Permission
	offset := (req.Page - 1) * req.PageSize
	if err := query.Offset(offset).Limit(req.PageSize).
		Preload("Parent").
		Order("sort ASC, id ASC").
		Find(&permissions).Error; err != nil {
		h.logger.Error("Failed to list permissions", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询失败"))
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse(gin.H{
		"list":      permissions,
		"total":     total,
		"page":      req.Page,
		"page_size": req.PageSize,
	}))
}

// buildPermissionTree 构建权限树
func (h *PermissionHandler) buildPermissionTree(permissions []models.Permission, parentID *uint) []models.Permission {
	var tree []models.Permission

	for _, permission := range permissions {
		if (parentID == nil && permission.ParentID == nil) ||
			(parentID != nil && permission.ParentID != nil && *permission.ParentID == *parentID) {

			children := h.buildPermissionTree(permissions, &permission.ID)
			if len(children) > 0 {
				permission.Children = children
			}
			tree = append(tree, permission)
		}
	}

	return tree
}

// CreatePermission 创建权限
func (h *PermissionHandler) CreatePermission(c *gin.Context) {
	var req struct {
		Name        string `json:"name" binding:"required,min=1,max=100"`
		Code        string `json:"code" binding:"required,min=1,max=100"`
		Type        string `json:"type" binding:"required,oneof=menu button api"`
		ParentID    *uint  `json:"parent_id"`
		Path        string `json:"path"`
		Method      string `json:"method"`
		Icon        string `json:"icon"`
		Component   string `json:"component"`
		Sort        int    `json:"sort"`
		Status      int    `json:"status"`
		Description string `json:"description"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "参数错误"))
		return
	}

	// 检查权限名称和代码是否已存在
	var existingPermission models.Permission
	if err := h.db.Where("name = ? OR code = ?", req.Name, req.Code).First(&existingPermission).Error; err == nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "权限名称或代码已存在"))
		return
	}

	// 如果有父权限，检查父权限是否存在
	if req.ParentID != nil {
		var parentPermission models.Permission
		if err := h.db.First(&parentPermission, *req.ParentID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "父权限不存在"))
				return
			}
			h.logger.Error("Failed to check parent permission", zap.Error(err))
			c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "父权限检查失败"))
			return
		}
	}

	permission := models.Permission{
		Name:        req.Name,
		Code:        req.Code,
		Type:        req.Type,
		ParentID:    req.ParentID,
		Path:        req.Path,
		Method:      req.Method,
		Icon:        req.Icon,
		Component:   req.Component,
		Sort:        req.Sort,
		Status:      req.Status,
		Description: req.Description,
	}

	if err := h.db.Create(&permission).Error; err != nil {
		h.logger.Error("Failed to create permission", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "创建失败"))
		return
	}

	// 重新加载权限信息
	h.db.Preload("Parent").First(&permission, permission.ID)

	c.JSON(http.StatusOK, models.SuccessResponse(permission))
}

// GetPermission 获取权限详情
func (h *PermissionHandler) GetPermission(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "无效的ID"))
		return
	}

	var permission models.Permission
	if err := h.db.Preload("Parent").Preload("Children").
		First(&permission, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, models.ErrorResponse(http.StatusNotFound, "权限不存在"))
			return
		}
		h.logger.Error("Failed to get permission", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询失败"))
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse(permission))
}

// UpdatePermission 更新权限
func (h *PermissionHandler) UpdatePermission(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "无效的ID"))
		return
	}

	var req struct {
		Name        string `json:"name" binding:"required,min=1,max=100"`
		Code        string `json:"code" binding:"required,min=1,max=100"`
		Type        string `json:"type" binding:"required,oneof=menu button api"`
		ParentID    *uint  `json:"parent_id"`
		Path        string `json:"path"`
		Method      string `json:"method"`
		Icon        string `json:"icon"`
		Component   string `json:"component"`
		Sort        int    `json:"sort"`
		Status      int    `json:"status"`
		Description string `json:"description"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "参数错误"))
		return
	}

	var permission models.Permission
	if err := h.db.First(&permission, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, models.ErrorResponse(http.StatusNotFound, "权限不存在"))
			return
		}
		h.logger.Error("Failed to get permission", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询失败"))
		return
	}

	// 检查系统权限
	if permission.IsSystem {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "系统权限不允许修改"))
		return
	}

	// 检查权限名称和代码是否被其他权限使用
	var existingPermission models.Permission
	if err := h.db.Where("(name = ? OR code = ?) AND id != ?", req.Name, req.Code, id).First(&existingPermission).Error; err == nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "权限名称或代码已存在"))
		return
	}

	// 如果有父权限，检查父权限是否存在，并且不能是自己或自己的子权限
	if req.ParentID != nil {
		if *req.ParentID == uint(id) {
			c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "不能设置自己为父权限"))
			return
		}

		var parentPermission models.Permission
		if err := h.db.First(&parentPermission, *req.ParentID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "父权限不存在"))
				return
			}
			h.logger.Error("Failed to check parent permission", zap.Error(err))
			c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "父权限检查失败"))
			return
		}

		// 检查是否形成循环引用
		if h.wouldCreateCycle(uint(id), *req.ParentID) {
			c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "不能设置子权限为父权限"))
			return
		}
	}

	// 更新权限信息
	updates := map[string]interface{}{
		"name":        req.Name,
		"code":        req.Code,
		"type":        req.Type,
		"parent_id":   req.ParentID,
		"path":        req.Path,
		"method":      req.Method,
		"icon":        req.Icon,
		"component":   req.Component,
		"sort":        req.Sort,
		"status":      req.Status,
		"description": req.Description,
	}

	if err := h.db.Model(&permission).Updates(updates).Error; err != nil {
		h.logger.Error("Failed to update permission", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "更新失败"))
		return
	}

	// 重新加载权限信息
	h.db.Preload("Parent").First(&permission, permission.ID)

	c.JSON(http.StatusOK, models.SuccessResponse(permission))
}

// wouldCreateCycle 检查是否会形成循环引用
func (h *PermissionHandler) wouldCreateCycle(permissionID, parentID uint) bool {
	var permission models.Permission
	if err := h.db.Preload("Children").First(&permission, permissionID).Error; err != nil {
		return false
	}

	// 递归检查所有子权限
	return h.hasChildPermission(permission.Children, parentID)
}

// hasChildPermission 检查是否包含指定的子权限
func (h *PermissionHandler) hasChildPermission(children []models.Permission, targetID uint) bool {
	for _, child := range children {
		if child.ID == targetID {
			return true
		}

		// 加载子权限的子权限
		var childWithChildren models.Permission
		if err := h.db.Preload("Children").First(&childWithChildren, child.ID).Error; err == nil {
			if h.hasChildPermission(childWithChildren.Children, targetID) {
				return true
			}
		}
	}
	return false
}

// DeletePermission 删除权限
func (h *PermissionHandler) DeletePermission(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "无效的ID"))
		return
	}

	var permission models.Permission
	if err := h.db.First(&permission, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, models.ErrorResponse(http.StatusNotFound, "权限不存在"))
			return
		}
		h.logger.Error("Failed to get permission", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询失败"))
		return
	}

	// 检查系统权限
	if permission.IsSystem {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "系统权限不允许删除"))
		return
	}

	// 检查是否有子权限
	var childCount int64
	h.db.Model(&models.Permission{}).Where("parent_id = ?", id).Count(&childCount)
	if childCount > 0 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "该权限下还有子权限，请先删除子权限"))
		return
	}

	// 检查是否有角色关联此权限
	var roleCount int64
	h.db.Model(&models.RolePermission{}).Where("permission_id = ?", id).Count(&roleCount)
	if roleCount > 0 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(http.StatusBadRequest, "该权限被角色使用，无法删除"))
		return
	}

	if err := h.db.Delete(&permission).Error; err != nil {
		h.logger.Error("Failed to delete permission", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "删除失败"))
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse(gin.H{"message": "删除成功"}))
}

// GetPermissionTypes 获取权限类型列表
func (h *PermissionHandler) GetPermissionTypes(c *gin.Context) {
	types := []gin.H{
		{"value": "menu", "label": "菜单权限", "description": "用于控制菜单的显示"},
		{"value": "button", "label": "按钮权限", "description": "用于控制页面按钮的显示"},
		{"value": "api", "label": "接口权限", "description": "用于控制API接口的访问"},
	}

	c.JSON(http.StatusOK, models.SuccessResponse(types))
}

// GetUserPermissions 获取用户的所有权限
func (h *PermissionHandler) GetUserPermissions(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse(http.StatusUnauthorized, "用户未登录"))
		return
	}

	// 通过用户的角色获取权限
	var permissions []models.Permission
	if err := h.db.Table("env_permissions").
		Joins("JOIN env_role_permissions ON env_permissions.id = env_role_permissions.permission_id").
		Joins("JOIN env_user_roles ON env_role_permissions.role_id = env_user_roles.role_id").
		Where("env_user_roles.user_id = ? AND env_permissions.status = 1", userID).
		Group("env_permissions.id").
		Order("env_permissions.sort ASC, env_permissions.id ASC").
		Find(&permissions).Error; err != nil {
		h.logger.Error("Failed to get user permissions", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询失败"))
		return
	}

	// 构建权限树
	tree := h.buildPermissionTree(permissions, nil)

	c.JSON(http.StatusOK, models.SuccessResponse(gin.H{
		"permissions": permissions,
		"tree":        tree,
	}))
}

// GetUserMenus 获取用户的菜单权限
func (h *PermissionHandler) GetUserMenus(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse(http.StatusUnauthorized, "用户未登录"))
		return
	}

	// 获取用户的菜单权限
	var permissions []models.Permission
	if err := h.db.Table("env_permissions").
		Joins("JOIN env_role_permissions ON env_permissions.id = env_role_permissions.permission_id").
		Joins("JOIN env_user_roles ON env_role_permissions.role_id = env_user_roles.role_id").
		Where("env_user_roles.user_id = ? AND env_permissions.status = 1 AND env_permissions.type = ?", userID, "menu").
		Group("env_permissions.id").
		Order("env_permissions.sort ASC, env_permissions.id ASC").
		Find(&permissions).Error; err != nil {
		h.logger.Error("Failed to get user menus", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(http.StatusInternalServerError, "查询失败"))
		return
	}

	// 构建菜单树
	menuTree := h.buildPermissionTree(permissions, nil)

	c.JSON(http.StatusOK, models.SuccessResponse(menuTree))
}