package logger

import (
	"os"

	"github.com/env-data-platform/internal/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// New 创建新的日志器
func New(cfg *config.Config) (*zap.Logger, error) {
	// 设置日志级别
	level := zapcore.InfoLevel
	switch cfg.Log.Level {
	case "debug":
		level = zapcore.DebugLevel
	case "info":
		level = zapcore.InfoLevel
	case "warn":
		level = zapcore.WarnLevel
	case "error":
		level = zapcore.ErrorLevel
	case "panic":
		level = zapcore.PanicLevel
	case "fatal":
		level = zapcore.FatalLevel
	}

	// 设置编码器配置
	var encoderConfig zapcore.EncoderConfig
	if cfg.App.IsProduction() {
		encoderConfig = zap.NewProductionEncoderConfig()
	} else {
		encoderConfig = zap.NewDevelopmentEncoderConfig()
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	encoderConfig.TimeKey = "timestamp"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.CallerKey = "caller"
	encoderConfig.EncodeCaller = zapcore.ShortCallerEncoder

	// 创建编码器
	var encoder zapcore.Encoder
	if cfg.Log.Format == "json" {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}

	// 设置输出
	var writeSyncer zapcore.WriteSyncer
	switch cfg.Log.Output {
	case "stdout":
		writeSyncer = zapcore.AddSync(os.Stdout)
	case "stderr":
		writeSyncer = zapcore.AddSync(os.Stderr)
	case "file":
		if cfg.Log.Filename == "" {
			cfg.Log.Filename = "logs/app.log"
		}

		// 创建日志目录
		if err := os.MkdirAll("logs", 0755); err != nil {
			return nil, err
		}

		lumberJackLogger := &lumberjack.Logger{
			Filename:   cfg.Log.Filename,
			MaxSize:    cfg.Log.MaxSize,
			MaxBackups: cfg.Log.MaxBackups,
			MaxAge:     cfg.Log.MaxAge,
			Compress:   cfg.Log.Compress,
		}
		writeSyncer = zapcore.AddSync(lumberJackLogger)
	default:
		// 同时输出到控制台和文件
		if cfg.Log.Filename == "" {
			cfg.Log.Filename = "logs/app.log"
		}

		// 创建日志目录
		if err := os.MkdirAll("logs", 0755); err != nil {
			return nil, err
		}

		lumberJackLogger := &lumberjack.Logger{
			Filename:   cfg.Log.Filename,
			MaxSize:    cfg.Log.MaxSize,
			MaxBackups: cfg.Log.MaxBackups,
			MaxAge:     cfg.Log.MaxAge,
			Compress:   cfg.Log.Compress,
		}

		writeSyncer = zapcore.NewMultiWriteSyncer(
			zapcore.AddSync(os.Stdout),
			zapcore.AddSync(lumberJackLogger),
		)
	}

	// 创建核心
	core := zapcore.NewCore(encoder, writeSyncer, level)

	// 创建日志器选项
	options := []zap.Option{
		zap.AddCaller(),
		zap.AddCallerSkip(1),
	}

	// 开发环境添加堆栈跟踪
	if cfg.App.IsDevelopment() {
		options = append(options, zap.Development(), zap.AddStacktrace(zapcore.ErrorLevel))
	}

	// 创建日志器
	logger := zap.New(core, options...)

	// 添加全局字段
	logger = logger.With(
		zap.String("service", cfg.App.Name),
		zap.String("version", cfg.App.Version),
		zap.String("environment", cfg.App.Environment),
	)

	return logger, nil
}