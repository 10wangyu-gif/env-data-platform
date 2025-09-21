package middleware

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/env-data-platform/internal/database"
	"github.com/env-data-platform/internal/models"
	"go.uber.org/zap"
)

// OperationLog 操作日志中间件
func OperationLog(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 跳过GET请求和健康检查等路径
		if shouldSkipLogging(c.Request.Method, c.Request.URL.Path) {
			c.Next()
			return
		}

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

		// 异步记录操作日志
		go func() {
			if err := recordOperationLog(c, start, requestBody, responseWriter.body.String()); err != nil {
				logger.Error("Failed to record operation log", zap.Error(err))
			}
		}()
	}
}

// shouldSkipLogging 判断是否跳过日志记录
func shouldSkipLogging(method, path string) bool {
	// 跳过GET请求
	if method == http.MethodGet {
		return true
	}

	// 跳过的路径
	skipPaths := []string{
		"/health",
		"/ping",
		"/metrics",
		"/favicon.ico",
	}

	for _, skipPath := range skipPaths {
		if strings.HasPrefix(path, skipPath) {
			return true
		}
	}

	return false
}

// recordOperationLog 记录操作日志
func recordOperationLog(c *gin.Context, startTime time.Time, requestBody []byte, responseBody string) error {
	db := database.GetDB()
	if db == nil {
		return nil
	}

	// 获取用户信息
	var userID uint
	var username string
	if user, exists := c.Get("user"); exists {
		if u, ok := user.(*models.User); ok {
			userID = u.ID
			username = u.Username
		}
	}

	// 确定操作模块和动作
	module, action := parseModuleAndAction(c.Request.URL.Path, c.Request.Method)

	// 计算执行时长
	duration := time.Since(startTime).Milliseconds()

	// 处理敏感信息
	requestBodyStr := string(requestBody)
	if containsSensitiveData(c.Request.URL.Path) {
		requestBodyStr = "[SENSITIVE DATA HIDDEN]"
	}

	// 限制响应体长度
	if len(responseBody) > 5000 {
		responseBody = responseBody[:5000] + "...[TRUNCATED]"
	}

	// 确定操作状态
	status := 1 // 成功
	var errorMsg string
	if c.Writer.Status() >= 400 {
		status = 0 // 失败
		if len(c.Errors) > 0 {
			errorMsg = c.Errors.String()
		}
	}

	// 创建操作日志记录
	operationLog := models.OperationLog{
		UserID:      userID,
		Username:    username,
		Module:      module,
		Action:      action,
		Method:      c.Request.Method,
		URL:         c.Request.URL.String(),
		IP:          c.ClientIP(),
		UserAgent:   c.Request.UserAgent(),
		RequestBody: requestBodyStr,
		Response:    responseBody,
		Status:      status,
		ErrorMsg:    errorMsg,
		Duration:    duration,
	}

	// 保存到数据库
	return db.Create(&operationLog).Error
}

// parseModuleAndAction 解析模块和操作
func parseModuleAndAction(path, method string) (module, action string) {
	// 移除API前缀
	path = strings.TrimPrefix(path, "/api/v1/")
	path = strings.TrimPrefix(path, "/api/")

	// 分割路径
	parts := strings.Split(path, "/")
	if len(parts) == 0 {
		return "unknown", "unknown"
	}

	// 确定模块
	module = parts[0]
	if module == "" {
		module = "system"
	}

	// 根据HTTP方法确定操作
	switch method {
	case http.MethodPost:
		action = "create"
	case http.MethodPut, http.MethodPatch:
		action = "update"
	case http.MethodDelete:
		action = "delete"
	default:
		action = "unknown"
	}

	// 特殊路径处理
	if len(parts) >= 2 {
		switch parts[1] {
		case "login":
			action = "login"
		case "logout":
			action = "logout"
		case "password":
			action = "change_password"
		case "export":
			action = "export"
		case "import":
			action = "import"
		case "test":
			action = "test"
		case "sync":
			action = "sync"
		case "execute":
			action = "execute"
		case "stop":
			action = "stop"
		case "restart":
			action = "restart"
		}
	}

	return module, action
}