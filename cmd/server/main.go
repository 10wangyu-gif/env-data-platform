package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/env-data-platform/internal/config"
	"github.com/env-data-platform/internal/database"
	"github.com/env-data-platform/internal/logger"
	"github.com/env-data-platform/internal/server"
	"go.uber.org/zap"
)

var (
	configFile = flag.String("config", "config/config.yaml", "配置文件路径")
	migrate    = flag.Bool("migrate", false, "是否执行数据库迁移")
	initData   = flag.Bool("init", false, "是否初始化基础数据")
	version    = flag.Bool("version", false, "显示版本信息")
)

const (
	AppName    = "环保数据集成平台"
	AppVersion = "1.0.0"
	BuildTime  = "2024-01-01"
)

func main() {
	flag.Parse()

	// 显示版本信息
	if *version {
		fmt.Printf("%s v%s (build %s)\n", AppName, AppVersion, BuildTime)
		os.Exit(0)
	}

	// 加载配置
	cfg, err := config.Load(*configFile)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 初始化日志
	zapLogger, err := logger.New(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer zapLogger.Sync()

	zapLogger.Info("Starting application",
		zap.String("name", AppName),
		zap.String("version", AppVersion),
		zap.String("environment", cfg.App.Environment),
		zap.String("config_file", *configFile))

	// 初始化数据库
	if err := database.Initialize(cfg); err != nil {
		zapLogger.Fatal("Failed to initialize database", zap.Error(err))
	}

	// 执行数据库迁移
	if *migrate {
		zapLogger.Info("Running database migration...")
		if err := database.AutoMigrate(); err != nil {
			zapLogger.Fatal("Failed to migrate database", zap.Error(err))
		}
		zapLogger.Info("Database migration completed")
	}

	// 初始化基础数据
	if *initData {
		zapLogger.Info("Initializing default data...")
		if err := database.InitializeData(); err != nil {
			zapLogger.Fatal("Failed to initialize default data", zap.Error(err))
		}
		zapLogger.Info("Default data initialization completed")
	}

	// 如果只是执行迁移或初始化，则退出
	if *migrate || *initData {
		zapLogger.Info("Operation completed, exiting...")
		os.Exit(0)
	}

	// 创建服务器
	srv := server.NewServer(cfg, zapLogger)

	// 启动服务器
	go func() {
		zapLogger.Info("Starting HTTP server",
			zap.String("addr", cfg.Server.GetServerAddr()),
			zap.Bool("tls", cfg.Server.TLS.Enabled))

		if err := srv.Start(); err != nil {
			zapLogger.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	zapLogger.Info("Shutting down server...")

	// 优雅关闭
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Stop(ctx); err != nil {
		zapLogger.Error("Server forced to shutdown", zap.Error(err))
	} else {
		zapLogger.Info("Server shutdown complete")
	}

	// 关闭数据库连接
	if err := database.Close(); err != nil {
		zapLogger.Error("Failed to close database", zap.Error(err))
	}

	zapLogger.Info("Application stopped")
}