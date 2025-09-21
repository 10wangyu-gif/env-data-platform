package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Recovery 恢复中间件
func Recovery(logger *zap.Logger) gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
		// 记录panic信息
		err := fmt.Sprintf("%v", recovered)
		stack := string(debug.Stack())

		logger.Error("Panic recovered",
			zap.String("error", err),
			zap.String("stack", stack),
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.String("ip", c.ClientIP()),
			zap.String("user_agent", c.Request.UserAgent()),
			zap.String("request_id", GetRequestID(c)),
		)

		// 返回错误响应
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":       500,
			"message":    "服务器内部错误",
			"request_id": GetRequestID(c),
		})
	})
}