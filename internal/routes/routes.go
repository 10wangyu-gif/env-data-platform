package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/env-data-platform/internal/config"
	"github.com/env-data-platform/internal/handlers"
	"github.com/env-data-platform/internal/hj212"
	"github.com/env-data-platform/internal/middleware"
	"go.uber.org/zap"
)

// SetupAPIRoutes 设置API路由
func SetupAPIRoutes(router *gin.Engine, cfg *config.Config, logger *zap.Logger, hj212Server *hj212.Server) {
	// API版本1
	v1 := router.Group("/api/v1")
	{
		// 认证相关路由（不需要登录）
		setupAuthRoutes(v1, cfg, logger)

		// 需要认证的路由组
		authenticated := v1.Group("")
		authenticated.Use(middleware.AuthMiddleware(cfg, logger))
		{
			// 仪表板
			setupDashboardRoutes(authenticated, logger)

			// 用户管理
			setupUserRoutes(authenticated, logger)

			// 角色管理
			setupRoleRoutes(authenticated, logger)

			// 权限管理
			setupPermissionRoutes(authenticated, logger)

			// 数据源管理
			setupDataSourceRoutes(authenticated, logger, hj212Server)

			// ETL管理
			setupETLRoutes(authenticated, logger)

			// 数据质量管理
			setupQualityRoutes(authenticated, logger)

			// 文件管理
			setupFileRoutes(authenticated, logger)

			// 系统管理
			setupSystemRoutes(authenticated, logger)
		}
	}

	// API版本2（预留）
	// v2 := router.Group("/api/v2")
	// {
	//     // 未来版本的API
	// }
}

// setupAuthRoutes 设置认证路由
func setupAuthRoutes(rg *gin.RouterGroup, cfg *config.Config, logger *zap.Logger) {
	authHandler := handlers.NewAuthHandler(cfg, logger)
	auth := rg.Group("/auth")
	{
		// 登录
		auth.POST("/login", authHandler.Login)

		// 刷新token
		auth.POST("/refresh", authHandler.RefreshToken)

		// 需要认证的路由
		authRequired := auth.Group("")
		authRequired.Use(middleware.AuthMiddleware(cfg, logger))
		{
			// 登出
			authRequired.POST("/logout", authHandler.Logout)

			// 获取当前用户信息
			authRequired.GET("/me", authHandler.GetMe)

			// 修改密码
			authRequired.PUT("/password", authHandler.ChangePassword)
		}
	}
}

// setupUserRoutes 设置用户路由
func setupUserRoutes(rg *gin.RouterGroup, logger *zap.Logger) {
	userHandler := handlers.NewUserHandler(logger)
	users := rg.Group("/users")
	{
		users.GET("", userHandler.ListUsers)
		users.POST("", userHandler.CreateUser)
		users.GET("/stats", userHandler.GetUserStats)
		users.GET("/current", userHandler.GetCurrentUser)
		users.PUT("/current", userHandler.UpdateCurrentUser)
		users.GET("/:id", userHandler.GetUser)
		users.PUT("/:id", userHandler.UpdateUser)
		users.DELETE("/:id", userHandler.DeleteUser)
		users.PUT("/:id/password", userHandler.ResetPassword)
		users.GET("/:id/roles", userHandler.GetUserRoles)
		users.PUT("/:id/roles", userHandler.AssignRoles)
	}
}

// setupRoleRoutes 设置角色路由
func setupRoleRoutes(rg *gin.RouterGroup, logger *zap.Logger) {
	roleHandler := handlers.NewRoleHandler(logger)
	roles := rg.Group("/roles")
	{
		roles.GET("", roleHandler.ListRoles)
		roles.POST("", roleHandler.CreateRole)
		roles.GET("/:id", roleHandler.GetRole)
		roles.PUT("/:id", roleHandler.UpdateRole)
		roles.DELETE("/:id", roleHandler.DeleteRole)
		roles.GET("/:id/permissions", roleHandler.GetRolePermissions)
		roles.PUT("/:id/permissions", roleHandler.AssignPermissions)
		roles.GET("/:id/users", roleHandler.GetRoleUsers)
	}
}

// setupPermissionRoutes 设置权限路由
func setupPermissionRoutes(rg *gin.RouterGroup, logger *zap.Logger) {
	permissionHandler := handlers.NewPermissionHandler(logger)
	permissions := rg.Group("/permissions")
	{
		permissions.GET("", permissionHandler.ListPermissions)
		permissions.POST("", permissionHandler.CreatePermission)
		permissions.GET("/types", permissionHandler.GetPermissionTypes)
		permissions.GET("/user/menus", permissionHandler.GetUserMenus)
		permissions.GET("/user/permissions", permissionHandler.GetUserPermissions)
		permissions.GET("/:id", permissionHandler.GetPermission)
		permissions.PUT("/:id", permissionHandler.UpdatePermission)
		permissions.DELETE("/:id", permissionHandler.DeletePermission)
	}
}

// setupDataSourceRoutes 设置数据源路由
func setupDataSourceRoutes(rg *gin.RouterGroup, logger *zap.Logger, hj212Server *hj212.Server) {
	dataSourceHandler := handlers.NewDataSourceHandler(logger)
	dataSources := rg.Group("/datasources")
	{
		dataSources.GET("", dataSourceHandler.ListDataSources)
		dataSources.POST("", dataSourceHandler.CreateDataSource)
		dataSources.GET("/:id", dataSourceHandler.GetDataSource)
		dataSources.PUT("/:id", dataSourceHandler.UpdateDataSource)
		dataSources.DELETE("/:id", dataSourceHandler.DeleteDataSource)
		dataSources.POST("/:id/test", dataSourceHandler.TestDataSource)
		dataSources.POST("/:id/sync", dataSourceHandler.SyncDataSource)
		dataSources.GET("/:id/tables", dataSourceHandler.GetDataSourceTables)
	}

	// HJ212数据查询
	hj212Handler := handlers.NewHJ212Handler(logger, hj212Server)
	hj212 := rg.Group("/hj212")
	{
		hj212.GET("/data", hj212Handler.QueryData)
		hj212.GET("/data/:id", hj212Handler.GetDataDetail)
		hj212.GET("/stats", hj212Handler.GetStats)
		hj212.GET("/devices", hj212Handler.GetConnectedDevices)
		hj212.GET("/alarms", hj212Handler.GetAlarmData)
		hj212.POST("/command", hj212Handler.SendCommand)
	}
}

// setupETLRoutes 设置ETL路由
func setupETLRoutes(rg *gin.RouterGroup, logger *zap.Logger) {
	etlHandler := handlers.NewETLHandler(logger)
	etl := rg.Group("/etl")
	{
		// ETL统计信息
		etl.GET("/stats", etlHandler.GetETLStats)

		// ETL作业管理
		jobs := etl.Group("/jobs")
		{
			jobs.GET("", etlHandler.ListETLJobs)
			jobs.POST("", etlHandler.CreateETLJob)
			jobs.GET("/:id", etlHandler.GetETLJob)
			jobs.PUT("/:id", etlHandler.UpdateETLJob)
			jobs.DELETE("/:id", etlHandler.DeleteETLJob)
			jobs.POST("/:id/execute", etlHandler.ExecuteETLJob)
			jobs.POST("/:id/stop", etlHandler.StopETLJob)
		}

		// ETL执行记录
		executions := etl.Group("/executions")
		{
			executions.GET("", etlHandler.ListETLExecutions)
			executions.GET("/:id", etlHandler.GetETLExecution)
			executions.GET("/:id/logs", etlHandler.GetETLExecutionLogs)
		}

		// ETL模板
		templates := etl.Group("/templates")
		{
			templates.GET("", etlHandler.ListETLTemplates)
			templates.POST("", etlHandler.CreateETLTemplate)
			templates.GET("/:id", etlHandler.GetETLTemplate)
			templates.PUT("/:id", etlHandler.UpdateETLTemplate)
			templates.DELETE("/:id", etlHandler.DeleteETLTemplate)
			templates.POST("/:id/create-job", etlHandler.CreateJobFromTemplate)
		}
	}
}

// setupQualityRoutes 设置数据质量路由
func setupQualityRoutes(rg *gin.RouterGroup, logger *zap.Logger) {
	qualityHandler := handlers.NewQualityHandler(logger)
	quality := rg.Group("/quality")
	{
		// 质量统计信息
		quality.GET("/stats", qualityHandler.GetQualityStats)

		// 质量规则
		rules := quality.Group("/rules")
		{
			rules.GET("", qualityHandler.ListQualityRules)
			rules.POST("", qualityHandler.CreateQualityRule)
			rules.GET("/:id", qualityHandler.GetQualityRule)
			rules.PUT("/:id", qualityHandler.UpdateQualityRule)
			rules.DELETE("/:id", qualityHandler.DeleteQualityRule)
			rules.POST("/:id/check", qualityHandler.ExecuteQualityCheck)
			rules.POST("/batch-check", qualityHandler.BatchExecuteQualityCheck)
		}

		// 质量报告
		reports := quality.Group("/reports")
		{
			reports.GET("", qualityHandler.ListQualityReports)
			reports.GET("/:id", qualityHandler.GetQualityReport)
		}
	}
}

// setupFileRoutes 设置文件路由
func setupFileRoutes(rg *gin.RouterGroup, logger *zap.Logger) {
	fileHandler := handlers.NewFileHandler(logger)
	files := rg.Group("/files")
	{
		files.POST("/upload", fileHandler.UploadFile)
		files.GET("/records", fileHandler.ListFiles)
		files.GET("/:id/download", fileHandler.DownloadFile)
		files.DELETE("/:id", fileHandler.DeleteFile)
		files.GET("/:id/info", fileHandler.GetFileInfo)
		files.GET("/stats", fileHandler.GetFileStats)
	}
}

// setupSystemRoutes 设置系统路由
func setupSystemRoutes(rg *gin.RouterGroup, logger *zap.Logger) {
	systemHandler := handlers.NewSystemHandler(logger)
	system := rg.Group("/system")
	{
		system.GET("/info", systemHandler.GetSystemInfo)
		system.GET("/stats", systemHandler.GetSystemStats)
		system.GET("/health", systemHandler.GetSystemHealth)

		// 操作日志
		logs := system.Group("/logs")
		{
			logs.GET("/operation", systemHandler.GetOperationLogs)
			logs.GET("/login", systemHandler.GetLoginLogs)
			logs.DELETE("/clear", systemHandler.ClearOldLogs)
		}
	}
}

// setupDashboardRoutes 设置仪表板路由
func setupDashboardRoutes(rg *gin.RouterGroup, logger *zap.Logger) {
	dashboardHandler := handlers.NewDashboardHandler(logger)
	dashboard := rg.Group("/dashboard")
	{
		dashboard.GET("/overview", dashboardHandler.GetDashboardOverview)
		dashboard.GET("/realtime", dashboardHandler.GetRealTimeData)
		dashboard.GET("/charts", dashboardHandler.GetChartData)
	}
}