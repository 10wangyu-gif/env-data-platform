package server

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/env-data-platform/internal/alarm"
	"github.com/env-data-platform/internal/config"
	"github.com/env-data-platform/internal/hj212"
	"github.com/env-data-platform/internal/middleware"
	"github.com/env-data-platform/internal/routes"
	"github.com/env-data-platform/internal/websocket"
	"go.uber.org/zap"
)

// Server HTTP服务器
type Server struct {
	config       *config.Config
	logger       *zap.Logger
	httpServer   *http.Server
	router       *gin.Engine
	hj212Server  *hj212.Server
	wsHub        *websocket.Hub
	wsHandler    *websocket.Handler
}

// NewServer 创建新的服务器实例
func NewServer(cfg *config.Config, logger *zap.Logger) *Server {
	// 设置Gin运行模式
	if cfg.App.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	// 创建Gin路由器
	router := gin.New()

	// 创建WebSocket集线器
	wsHub := websocket.NewHub(logger)
	wsHandler := websocket.NewHandler(wsHub, logger)

	// 创建告警检测器
	alarmDetector := alarm.NewDetector(logger, wsHub)

	// 创建HJ212服务器
	hj212Server := hj212.NewServer(cfg, logger, wsHub, alarmDetector)

	return &Server{
		config:       cfg,
		logger:       logger,
		router:       router,
		hj212Server:  hj212Server,
		wsHub:        wsHub,
		wsHandler:    wsHandler,
	}
}

// SetupMiddleware 设置中间件
func (s *Server) SetupMiddleware() {
	// 日志中间件
	s.router.Use(middleware.Logger(s.logger))

	// 恢复中间件
	s.router.Use(middleware.Recovery(s.logger))

	// CORS中间件
	s.router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"*"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// 请求ID中间件
	s.router.Use(middleware.RequestID())

	// 限流中间件
	s.router.Use(middleware.RateLimit())

	// 监控中间件
	if s.config.Monitor.Enabled {
		s.router.Use(middleware.Metrics())
	}

	// 操作日志中间件
	s.router.Use(middleware.OperationLog(s.logger))
}

// SetupRoutes 设置路由
func (s *Server) SetupRoutes() {
	// 设置API路由
	routes.SetupAPIRoutes(s.router, s.config, s.logger, s.hj212Server)

	// 设置WebSocket路由
	s.router.GET("/ws", s.wsHandler.HandleWebSocket)

	// 设置健康检查路由
	s.router.GET("/health", s.healthCheck)
	s.router.GET("/ping", s.ping)

	// 设置监控路由
	if s.config.Monitor.Prometheus.Enabled {
		s.router.GET(s.config.Monitor.Prometheus.Path, middleware.PrometheusHandler())
	}

	// 静态文件服务
	s.router.Static("/static", "./static")
	s.router.Static("/uploads", "./uploads")

	// 404处理
	s.router.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "接口不存在",
			"path":    c.Request.URL.Path,
		})
	})

	// 405处理
	s.router.NoMethod(func(c *gin.Context) {
		c.JSON(http.StatusMethodNotAllowed, gin.H{
			"code":    405,
			"message": "请求方法不允许",
			"method":  c.Request.Method,
			"path":    c.Request.URL.Path,
		})
	})
}

// Start 启动服务器
func (s *Server) Start() error {
	// 设置中间件
	s.SetupMiddleware()

	// 设置路由
	s.SetupRoutes()

	// 启动WebSocket Hub
	go s.wsHub.Run()

	// 启动HJ212服务器
	if s.config.HJ212.Enabled {
		go func() {
			if err := s.hj212Server.Start(); err != nil {
				s.logger.Error("HJ212 server failed", zap.Error(err))
			}
		}()
	}

	// 创建HTTP服务器
	s.httpServer = &http.Server{
		Addr:         s.config.Server.GetServerAddr(),
		Handler:      s.router,
		ReadTimeout:  time.Duration(s.config.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(s.config.Server.WriteTimeout) * time.Second,
	}

	s.logger.Info("Starting HTTP server",
		zap.String("addr", s.httpServer.Addr),
		zap.Bool("tls", s.config.Server.TLS.Enabled))

	// 启动服务器
	if s.config.Server.TLS.Enabled {
		return s.httpServer.ListenAndServeTLS(
			s.config.Server.TLS.CertFile,
			s.config.Server.TLS.KeyFile,
		)
	}

	return s.httpServer.ListenAndServe()
}

// Stop 停止服务器
func (s *Server) Stop(ctx context.Context) error {
	s.logger.Info("Stopping servers...")

	// 停止HJ212服务器
	if s.hj212Server != nil {
		if err := s.hj212Server.Stop(); err != nil {
			s.logger.Error("Failed to stop HJ212 server", zap.Error(err))
		}
	}

	// 停止HTTP服务器
	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}

	return nil
}

// GetRouter 获取路由器
func (s *Server) GetRouter() *gin.Engine {
	return s.router
}

// healthCheck 健康检查处理器
func (s *Server) healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
		"version":   s.config.App.Version,
		"uptime":    time.Since(time.Now()).String(),
	})
}

// ping 简单ping处理器
func (s *Server) ping(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "pong",
		"timestamp": time.Now().Unix(),
	})
}