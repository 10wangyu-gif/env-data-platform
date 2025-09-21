package database

import (
	"fmt"
	"log"
	"time"

	"github.com/env-data-platform/internal/config"
	"github.com/env-data-platform/internal/models"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DB 全局数据库连接
var DB *gorm.DB

// Initialize 初始化数据库连接
func Initialize(cfg *config.Config) error {
	var err error

	// 构建数据库连接字符串
	dsn := cfg.Database.GetDSN()

	// 配置GORM
	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	}

	// 开发环境启用SQL日志
	if cfg.App.Debug {
		gormConfig.Logger = logger.Default.LogMode(logger.Info)
	}

	// 连接数据库
	DB, err = gorm.Open(mysql.Open(dsn), gormConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	// 获取底层的*sql.DB
	sqlDB, err := DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	// 设置连接池参数
	sqlDB.SetMaxIdleConns(cfg.Database.MaxIdleConns)
	sqlDB.SetMaxOpenConns(cfg.Database.MaxOpenConns)

	// 解析连接最大生命周期
	if cfg.Database.ConnMaxLifetime != "" {
		if lifetime, err := time.ParseDuration(cfg.Database.ConnMaxLifetime); err == nil {
			sqlDB.SetConnMaxLifetime(lifetime)
		}
	}

	// 测试连接
	if err := sqlDB.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	log.Println("Database connected successfully")
	return nil
}

// AutoMigrate 自动迁移数据库表结构
func AutoMigrate() error {
	if DB == nil {
		return fmt.Errorf("database not initialized")
	}

	// 第一阶段：创建基础表（不包含外键约束）
	baseModels := []interface{}{
		// 用户和权限相关
		&models.User{},
		&models.Role{},
		&models.Permission{},
		&models.LoginLog{},
		&models.OperationLog{},

		// 数据源相关
		&models.DataSource{},
		&models.DataTable{},
		&models.DataColumn{},
		&models.HJ212Data{},
		&models.HJ212AlarmData{},
		&models.FileUploadRecord{},

		// ETL相关（基础表）
		&models.ETLJob{},
		// &models.ETLExecution{}, // 暂时移除
		&models.ETLTemplate{},
		&models.QualityRule{},
		&models.QualityReport{},
	}

	// 第一阶段迁移
	for _, model := range baseModels {
		log.Printf("Migrating base model: %T", model)
		if err := DB.AutoMigrate(model); err != nil {
			log.Printf("Migration failed for %T: %v", model, err)
			return fmt.Errorf("failed to migrate %T: %w", model, err)
		}
		log.Printf("Successfully migrated: %T", model)
	}

	// 第二阶段：创建关联表
	relationModels := []interface{}{
		&models.UserRole{},
		&models.RolePermission{},
		// &models.ETLExecutionStep{}, // 暂时移除
	}

	// 第二阶段迁移
	for _, model := range relationModels {
		log.Printf("Migrating relation model: %T", model)
		if err := DB.AutoMigrate(model); err != nil {
			log.Printf("Migration failed for %T: %v", model, err)
			return fmt.Errorf("failed to migrate %T: %w", model, err)
		}
		log.Printf("Successfully migrated: %T", model)
	}

	log.Println("Database migration completed successfully")
	return nil
}

// InitializeData 初始化基础数据
func InitializeData() error {
	if DB == nil {
		return fmt.Errorf("database not initialized")
	}

	// 创建默认权限
	if err := createDefaultPermissions(); err != nil {
		return fmt.Errorf("failed to create default permissions: %w", err)
	}

	// 创建默认角色
	if err := createDefaultRoles(); err != nil {
		return fmt.Errorf("failed to create default roles: %w", err)
	}

	// 创建默认管理员用户
	if err := createDefaultAdmin(); err != nil {
		return fmt.Errorf("failed to create default admin: %w", err)
	}

	log.Println("Default data initialized successfully")
	return nil
}

// createDefaultPermissions 创建默认权限
func createDefaultPermissions() error {
	// 第一级：顶级菜单（无父级）
	topLevelPermissions := []models.Permission{
		{Name: "系统管理", Code: "system", Type: "menu", Path: "/system", Icon: "system", Sort: 1, IsSystem: true},
		{Name: "数据源管理", Code: "datasource", Type: "menu", Path: "/datasource", Icon: "datasource", Sort: 2, IsSystem: true},
		{Name: "ETL管理", Code: "etl", Type: "menu", Path: "/etl", Icon: "etl", Sort: 3, IsSystem: true},
		{Name: "数据质量", Code: "quality", Type: "menu", Path: "/quality", Icon: "quality", Sort: 4, IsSystem: true},
		{Name: "监控管理", Code: "monitor", Type: "menu", Path: "/monitor", Icon: "monitor", Sort: 5, IsSystem: true},
	}

	// 创建顶级权限
	parentIds := make(map[string]uint)
	for _, permission := range topLevelPermissions {
		var count int64
		DB.Model(&models.Permission{}).Where("code = ?", permission.Code).Count(&count)
		if count == 0 {
			if err := DB.Create(&permission).Error; err != nil {
				return err
			}
			parentIds[permission.Code] = permission.ID
		} else {
			// 如果已存在，获取其ID
			var existing models.Permission
			DB.Where("code = ?", permission.Code).First(&existing)
			parentIds[permission.Code] = existing.ID
		}
	}

	// 第二级：子菜单和按钮
	childPermissions := []struct {
		models.Permission
		ParentCode string
	}{
		// 系统管理子项
		{models.Permission{Name: "用户管理", Code: "system:user", Type: "menu", Path: "/system/user", Icon: "user", Sort: 1, IsSystem: true}, "system"},
		{models.Permission{Name: "角色管理", Code: "system:role", Type: "menu", Path: "/system/role", Icon: "role", Sort: 2, IsSystem: true}, "system"},
		{models.Permission{Name: "权限管理", Code: "system:permission", Type: "menu", Path: "/system/permission", Icon: "permission", Sort: 3, IsSystem: true}, "system"},

		// 数据源管理子项
		{models.Permission{Name: "数据源列表", Code: "datasource:list", Type: "menu", Path: "/datasource/list", Icon: "list", Sort: 1, IsSystem: true}, "datasource"},
		{models.Permission{Name: "数据源创建", Code: "datasource:create", Type: "button", Sort: 2, IsSystem: true}, "datasource"},
		{models.Permission{Name: "数据源编辑", Code: "datasource:edit", Type: "button", Sort: 3, IsSystem: true}, "datasource"},
		{models.Permission{Name: "数据源删除", Code: "datasource:delete", Type: "button", Sort: 4, IsSystem: true}, "datasource"},

		// ETL管理子项
		{models.Permission{Name: "ETL作业", Code: "etl:job", Type: "menu", Path: "/etl/job", Icon: "job", Sort: 1, IsSystem: true}, "etl"},
		{models.Permission{Name: "ETL执行", Code: "etl:execution", Type: "menu", Path: "/etl/execution", Icon: "execution", Sort: 2, IsSystem: true}, "etl"},
		{models.Permission{Name: "ETL模板", Code: "etl:template", Type: "menu", Path: "/etl/template", Icon: "template", Sort: 3, IsSystem: true}, "etl"},

		// 数据质量子项
		{models.Permission{Name: "质量规则", Code: "quality:rule", Type: "menu", Path: "/quality/rule", Icon: "rule", Sort: 1, IsSystem: true}, "quality"},
		{models.Permission{Name: "质量报告", Code: "quality:report", Type: "menu", Path: "/quality/report", Icon: "report", Sort: 2, IsSystem: true}, "quality"},

		// 监控管理子项
		{models.Permission{Name: "系统监控", Code: "monitor:system", Type: "menu", Path: "/monitor/system", Icon: "system", Sort: 1, IsSystem: true}, "monitor"},
		{models.Permission{Name: "数据监控", Code: "monitor:data", Type: "menu", Path: "/monitor/data", Icon: "data", Sort: 2, IsSystem: true}, "monitor"},
	}

	// 创建子权限
	for _, childPerm := range childPermissions {
		var count int64
		DB.Model(&models.Permission{}).Where("code = ?", childPerm.Permission.Code).Count(&count)
		if count == 0 {
			parentID := parentIds[childPerm.ParentCode]
			childPerm.Permission.ParentID = &parentID
			if err := DB.Create(&childPerm.Permission).Error; err != nil {
				return err
			}
		}
	}

	return nil
}

// createDefaultRoles 创建默认角色
func createDefaultRoles() error {
	roles := []models.Role{
		{Name: "超级管理员", Code: "admin", Description: "系统超级管理员，拥有所有权限", IsSystem: true, Sort: 1},
		{Name: "操作员", Code: "operator", Description: "系统操作员，负责日常数据处理", IsSystem: true, Sort: 2},
		{Name: "开发者", Code: "developer", Description: "系统开发者，负责ETL开发", IsSystem: true, Sort: 3},
		{Name: "查看者", Code: "viewer", Description: "只读用户，只能查看数据", IsSystem: true, Sort: 4},
	}

	for _, role := range roles {
		var count int64
		DB.Model(&models.Role{}).Where("code = ?", role.Code).Count(&count)
		if count == 0 {
			if err := DB.Create(&role).Error; err != nil {
				return err
			}

			// 为管理员角色分配所有权限
			if role.Code == "admin" {
				var permissions []models.Permission
				DB.Find(&permissions)
				for _, permission := range permissions {
					rolePermission := models.RolePermission{
						RoleID:       role.ID,
						PermissionID: permission.ID,
					}
					DB.Create(&rolePermission)
				}
			}
		}
	}

	return nil
}

// createDefaultAdmin 创建默认管理员
func createDefaultAdmin() error {
	var count int64
	DB.Model(&models.User{}).Where("username = ?", "admin").Count(&count)
	if count > 0 {
		return nil // 管理员已存在
	}

	// 创建管理员用户
	admin := models.User{
		Username: "admin",
		Email:    "admin@env-data-platform.com",
		Password: "$argon2id$v=19$m=65536,t=3,p=2$erHyHlzzuHNTDetweTSOrg$vY3fq2lCYW20rxHkkYzbxtQFAPZi2qjXzvqOkfc/BLE", // password: admin123
		RealName: "系统管理员",
		Status:   models.StatusActive,
		Department: "系统部",
		Position: "系统管理员",
	}

	if err := DB.Create(&admin).Error; err != nil {
		return err
	}

	// 分配管理员角色
	var adminRole models.Role
	if err := DB.Where("code = ?", "admin").First(&adminRole).Error; err != nil {
		return err
	}

	userRole := models.UserRole{
		UserID: admin.ID,
		RoleID: adminRole.ID,
	}

	return DB.Create(&userRole).Error
}

// Close 关闭数据库连接
func Close() error {
	if DB == nil {
		return nil
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}

	return sqlDB.Close()
}

// GetDB 获取数据库连接
func GetDB() *gorm.DB {
	return DB
}

// Transaction 执行事务
func Transaction(fn func(tx *gorm.DB) error) error {
	return DB.Transaction(fn)
}