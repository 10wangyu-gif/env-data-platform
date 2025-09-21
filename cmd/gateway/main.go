package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/env-data-platform/internal/api/handlers"
	"github.com/env-data-platform/internal/gateway"
	"github.com/env-data-platform/internal/gateway/auth"
	"github.com/env-data-platform/internal/gateway/metrics"
	"github.com/env-data-platform/internal/gateway/ratelimit"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	// 加载配置
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "config/gateway.yaml"
	}

	config, err := gateway.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 初始化日志
	logger, err := setupLogger(config)
	if err != nil {
		log.Fatalf("Failed to setup logger: %v", err)
	}
	defer logger.Sync()

	logger.Info("Starting API Gateway",
		zap.String("version", "1.0.0"),
		zap.String("config", configPath),
		zap.Bool("tls_enabled", config.Server.TLS.Enabled))

	// 初始化Redis客户端
	var redisClient *redis.Client
	if config.RateLimit.Redis {
		redisClient = redis.NewClient(&redis.Options{
			Addr:     config.GetRedisAddress(),
			Password: config.Redis.Password,
			DB:       config.Redis.DB,
			PoolSize: config.Redis.PoolSize,
		})

		// 测试Redis连接
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := redisClient.Ping(ctx).Err(); err != nil {
			logger.Warn("Redis connection failed, falling back to in-memory rate limiting",
				zap.Error(err))
			redisClient = nil
		} else {
			logger.Info("Redis connected successfully",
				zap.String("addr", config.GetRedisAddress()))
		}
	}

	// 创建核心组件
	gatewayRouter := gateway.NewRouter(logger, nil, nil)
	loadBalancer := gateway.NewLoadBalancer(logger)
	serviceDiscovery := gateway.NewServiceDiscovery(&config.LoadBalance.HealthCheck, logger)
	metricsCollector := metrics.NewCollector(logger)

	// 初始化认证器
	authenticator := auth.NewAuthenticator(&auth.AuthConfig{
		Strategy:    auth.AuthStrategy(config.Auth.Strategy),
		JWTSecret:   config.Auth.JWTSecret,
		TokenExpiry: config.Auth.TokenExpiry,
		Issuer:      config.Auth.Issuer,
		Audience:    config.Auth.Audience,
	}, logger)

	// 创建限流器
	rateLimiterConfig := &ratelimit.LimitConfig{
		Strategy: ratelimit.LimitStrategy(config.RateLimit.Strategy),
		Rate:     config.RateLimit.Rate,
		Burst:    config.RateLimit.Burst,
		Window:   config.RateLimit.Window,
		KeyFunc:  getKeyFunc(config.RateLimit.KeyFunc),
	}

	rateLimiter, err := ratelimit.CreateRateLimiter(
		ratelimit.LimitStrategy(config.RateLimit.Strategy),
		rateLimiterConfig,
		redisClient,
		logger,
	)
	if err != nil {
		logger.Fatal("Failed to create rate limiter", zap.Error(err))
	}

	// 加载配置中的路由和服务
	if err := loadRoutesFromConfig(gatewayRouter, config); err != nil {
		logger.Fatal("Failed to load routes", zap.Error(err))
	}

	if err := loadServicesFromConfig(loadBalancer, serviceDiscovery, config); err != nil {
		logger.Fatal("Failed to load services", zap.Error(err))
	}

	// 启动服务发现
	ctx := context.Background()
	if err := serviceDiscovery.Start(ctx); err != nil {
		logger.Fatal("Failed to start service discovery", zap.Error(err))
	}

	// 创建处理器
	gatewayHandler := handlers.NewGatewayHandler(
		gatewayRouter,
		loadBalancer,
		authenticator,
		metricsCollector,
		rateLimiter,
		logger,
	)

	// 设置Gin模式
	if config.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	// 创建HTTP服务器
	router := setupRouter(config, gatewayHandler, gatewayRouter, authenticator, rateLimiter, rateLimiterConfig, metricsCollector, logger)

	server := &http.Server{
		Addr:           config.GetServerAddress(),
		Handler:        router,
		ReadTimeout:    config.Server.ReadTimeout,
		WriteTimeout:   config.Server.WriteTimeout,
		IdleTimeout:    config.Server.IdleTimeout,
		MaxHeaderBytes: config.Server.MaxHeaderBytes,
	}

	// 启动服务器
	go func() {
		logger.Info("Starting HTTP server",
			zap.String("addr", server.Addr))

		var err error
		if config.Server.TLS.Enabled {
			err = server.ListenAndServeTLS(config.Server.TLS.CertFile, config.Server.TLS.KeyFile)
		} else {
			err = server.ListenAndServe()
		}

		if err != nil && err != http.ErrServerClosed {
			logger.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	// 优雅关闭
	shutdownCtx, cancel := context.WithTimeout(context.Background(), config.Server.ShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("Server forced to shutdown", zap.Error(err))
	} else {
		logger.Info("Server shutdown complete")
	}

	// 停止服务发现
	if err := serviceDiscovery.Stop(); err != nil {
		logger.Error("Failed to stop service discovery", zap.Error(err))
	}

	// 关闭Redis连接
	if redisClient != nil {
		if err := redisClient.Close(); err != nil {
			logger.Error("Failed to close Redis connection", zap.Error(err))
		}
	}

	logger.Info("Gateway stopped")
}

// setupLogger 设置日志器
func setupLogger(config *gateway.Config) (*zap.Logger, error) {
	var zapConfig zap.Config

	if config.IsProduction() {
		zapConfig = zap.NewProductionConfig()
	} else {
		zapConfig = zap.NewDevelopmentConfig()
	}

	// 设置日志级别
	level, err := zapcore.ParseLevel(config.Logging.Level)
	if err != nil {
		return nil, fmt.Errorf("invalid log level: %w", err)
	}
	zapConfig.Level = zap.NewAtomicLevelAt(level)

	// 设置输出格式
	if config.Logging.Format == "console" {
		zapConfig.Encoding = "console"
	} else {
		zapConfig.Encoding = "json"
	}

	// 设置输出路径
	if config.Logging.File != "" {
		zapConfig.OutputPaths = []string{config.Logging.File}
		zapConfig.ErrorOutputPaths = []string{config.Logging.File}
	}

	return zapConfig.Build()
}

// setupRouter 设置路由
func setupRouter(
	config *gateway.Config,
	gatewayHandler *handlers.GatewayHandler,
	gatewayRouter *gateway.Router,
	authenticator *auth.Authenticator,
	rateLimiter ratelimit.RateLimiter,
	rateLimiterConfig *ratelimit.LimitConfig,
	collector *metrics.Collector,
	logger *zap.Logger,
) *gin.Engine {
	router := gin.New()

	// 中间件
	router.Use(gin.Recovery())
	router.Use(collector.Middleware())

	// 健康检查（不需要认证和限流）
	router.GET("/health", gatewayRouter.HealthCheck())
	router.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "pong"})
	})

	// 指标端点（如果启用）
	if config.Metrics.Enabled && config.Metrics.Prometheus {
		router.GET(config.Metrics.Path, gin.WrapH(promhttp.Handler()))
	}

	// 管理API（需要认证）
	admin := router.Group("/admin")
	if config.Auth.Strategy != "none" {
		admin.Use(authenticator.Middleware())
		admin.Use(auth.RequireScope("admin"))
	}
	{
		// 路由管理
		admin.GET("/routes", gatewayHandler.ListRoutes)
		admin.POST("/routes", gatewayHandler.CreateRoute)
		admin.GET("/routes/:method/*path", gatewayHandler.GetRoute)
		admin.PUT("/routes/:method/*path", gatewayHandler.UpdateRoute)
		admin.DELETE("/routes/:method/*path", gatewayHandler.DeleteRoute)

		// 负载均衡管理
		admin.GET("/loadbalancer/stats", gatewayHandler.GetLoadBalancerStats)
		admin.PUT("/loadbalancer/groups/:groupId/targets/:targetId/health", gatewayHandler.UpdateTargetHealth)

		// 认证管理
		admin.POST("/auth/apikeys", gatewayHandler.CreateAPIKey)
		admin.GET("/auth/apikeys", gatewayHandler.ListAPIKeys)
		admin.DELETE("/auth/apikeys/:key", gatewayHandler.RevokeAPIKey)

		// 指标管理
		admin.GET("/metrics", gatewayHandler.GetMetrics)
		admin.GET("/metrics/health", gatewayHandler.GetHealthMetrics)
		if config.IsDevelopment() {
			admin.POST("/metrics/reset", gatewayHandler.ResetMetrics)
		}

		// 限流管理
		admin.GET("/ratelimit/stats", gatewayHandler.GetRateLimitStats)
		admin.DELETE("/ratelimit/:key", gatewayHandler.ResetRateLimit)

		// 系统信息
		admin.GET("/system/info", gatewayHandler.GetSystemInfo)
		admin.GET("/system/health", gatewayHandler.HealthCheck)
	}

	// 代理路由（需要认证和限流）
	proxy := router.Group("/")

	// 认证中间件
	if config.Auth.Strategy != "none" {
		proxy.Use(authenticator.Middleware())
	}

	// 限流中间件
	if config.RateLimit.Enabled {
		proxy.Use(ratelimit.Middleware(rateLimiter, rateLimiterConfig))
	}

	// 代理处理器
	proxy.Any("/*path", gatewayRouter.HandleRequest())

	return router
}

// loadRoutesFromConfig 从配置加载路由
func loadRoutesFromConfig(router *gateway.Router, config *gateway.Config) error {
	for _, routeConfig := range config.Routes {
		route := &gateway.Route{
			ID:          routeConfig.ID,
			Path:        routeConfig.Path,
			Method:      routeConfig.Method,
			Target:      routeConfig.Target,
			StripPrefix: routeConfig.StripPrefix,
			Headers:     routeConfig.Headers,
			Timeout:     routeConfig.Timeout,
			Retries:     routeConfig.Retries,
		}

		if err := router.AddRoute(route); err != nil {
			return fmt.Errorf("failed to add route %s: %w", route.ID, err)
		}
	}

	return nil
}

// loadServicesFromConfig 从配置加载服务
func loadServicesFromConfig(loadBalancer *gateway.LoadBalancer, discovery *gateway.ServiceDiscovery, config *gateway.Config) error {
	for _, serviceConfig := range config.Services {
		// 创建服务组
		targets := make([]*gateway.Target, 0, len(serviceConfig.Targets))
		for _, targetConfig := range serviceConfig.Targets {
			target := &gateway.Target{
				ID:          targetConfig.ID,
				URL:         targetConfig.URL,
				Weight:      targetConfig.Weight,
				Metadata:    targetConfig.Metadata,
				IsHealthy:   true, // 初始假设健康
			}
			targets = append(targets, target)
		}

		serviceGroup := &gateway.ServiceGroup{
			ID:       serviceConfig.ID,
			Strategy: gateway.LoadBalanceStrategy(serviceConfig.Strategy),
			Targets:  targets,
		}

		loadBalancer.AddServiceGroup(serviceGroup)

		// 注册服务到服务发现
		serviceInfos := gateway.CreateServiceFromConfig(&serviceConfig)
		for _, serviceInfo := range serviceInfos {
			discovery.RegisterService(serviceInfo)
		}
	}

	return nil
}

// getKeyFunc 获取键生成函数
func getKeyFunc(keyFuncName string) ratelimit.KeyFunc {
	switch keyFuncName {
	case "apikey":
		return ratelimit.APIKeyFunc
	case "user":
		return ratelimit.UserIDFunc
	case "path":
		return ratelimit.PathBasedKeyFunc
	default:
		return ratelimit.DefaultKeyFunc
	}
}