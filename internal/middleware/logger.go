package middleware

import (
	"bytes"
	"io"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// responseBodyWriter 响应体写入器
type responseBodyWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (r responseBodyWriter) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

// Logger 日志中间件
func Logger(logger *zap.Logger) gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		// 记录请求日志
		fields := []zap.Field{
			zap.String("method", param.Method),
			zap.String("path", param.Path),
			zap.String("query", param.Request.URL.RawQuery),
			zap.String("ip", param.ClientIP),
			zap.String("user_agent", param.Request.UserAgent()),
			zap.Int("status", param.StatusCode),
			zap.Duration("latency", param.Latency),
			zap.String("time", param.TimeStamp.Format(time.RFC3339)),
		}

		if param.ErrorMessage != "" {
			fields = append(fields, zap.String("error", param.ErrorMessage))
		}

		// 根据状态码选择日志级别
		if param.StatusCode >= 500 {
			logger.Error("HTTP Request", fields...)
		} else if param.StatusCode >= 400 {
			logger.Warn("HTTP Request", fields...)
		} else {
			logger.Info("HTTP Request", fields...)
		}

		return ""
	})
}

// DetailedLogger 详细日志中间件（包含请求体和响应体）
func DetailedLogger(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 记录开始时间
		start := time.Now()

		// 读取请求体
		var requestBody []byte
		if c.Request.Body != nil {
			requestBody, _ = io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewBuffer(requestBody))
		}

		// 创建响应体写入器
		responseWriter := &responseBodyWriter{
			ResponseWriter: c.Writer,
			body:          bytes.NewBufferString(""),
		}
		c.Writer = responseWriter

		// 处理请求
		c.Next()

		// 计算执行时间
		latency := time.Since(start)

		// 记录详细日志
		fields := []zap.Field{
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.String("query", c.Request.URL.RawQuery),
			zap.String("ip", c.ClientIP()),
			zap.String("user_agent", c.Request.UserAgent()),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency", latency),
			zap.String("request_id", GetRequestID(c)),
		}

		// 添加请求体（排除敏感信息）
		if len(requestBody) > 0 && !containsSensitiveData(c.Request.URL.Path) {
			fields = append(fields, zap.String("request_body", string(requestBody)))
		}

		// 添加响应体（限制长度）
		responseBody := responseWriter.body.String()
		if len(responseBody) > 0 && len(responseBody) < 1000 {
			fields = append(fields, zap.String("response_body", responseBody))
		}

		// 添加错误信息
		if len(c.Errors) > 0 {
			fields = append(fields, zap.String("errors", c.Errors.String()))
		}

		// 根据状态码记录不同级别的日志
		if c.Writer.Status() >= 500 {
			logger.Error("HTTP Request Detail", fields...)
		} else if c.Writer.Status() >= 400 {
			logger.Warn("HTTP Request Detail", fields...)
		} else {
			logger.Info("HTTP Request Detail", fields...)
		}
	}
}

// containsSensitiveData 检查路径是否包含敏感数据
func containsSensitiveData(path string) bool {
	sensitivePaths := []string{
		"/api/auth/login",
		"/api/auth/register",
		"/api/users/password",
	}

	for _, sensitivePath := range sensitivePaths {
		if path == sensitivePath {
			return true
		}
	}

	return false
}