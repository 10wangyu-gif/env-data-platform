package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// RateLimiter 限流器
type RateLimiter struct {
	limiters map[string]*rate.Limiter
	mutex    sync.RWMutex
	rate     rate.Limit
	burst    int
}

// NewRateLimiter 创建限流器
func NewRateLimiter(rateLimit rate.Limit, burst int) *RateLimiter {
	return &RateLimiter{
		limiters: make(map[string]*rate.Limiter),
		rate:     rateLimit,
		burst:    burst,
	}
}

// getLimiter 获取指定IP的限流器
func (rl *RateLimiter) getLimiter(ip string) *rate.Limiter {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	limiter, exists := rl.limiters[ip]
	if !exists {
		limiter = rate.NewLimiter(rl.rate, rl.burst)
		rl.limiters[ip] = limiter

		// 定期清理不活跃的限流器
		go func() {
			time.Sleep(time.Hour)
			rl.mutex.Lock()
			delete(rl.limiters, ip)
			rl.mutex.Unlock()
		}()
	}

	return limiter
}

// Allow 检查是否允许请求
func (rl *RateLimiter) Allow(ip string) bool {
	limiter := rl.getLimiter(ip)
	return limiter.Allow()
}

// 全局限流器实例
var globalRateLimiter = NewRateLimiter(100, 200) // 每秒100个请求，突发200个

// RateLimit 限流中间件
func RateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()

		if !globalRateLimiter.Allow(ip) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"code":    429,
				"message": "请求过于频繁，请稍后再试",
				"ip":      ip,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// CustomRateLimit 自定义限流中间件
func CustomRateLimit(rateLimit rate.Limit, burst int) gin.HandlerFunc {
	limiter := NewRateLimiter(rateLimit, burst)

	return func(c *gin.Context) {
		ip := c.ClientIP()

		if !limiter.Allow(ip) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"code":    429,
				"message": "请求过于频繁，请稍后再试",
				"ip":      ip,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}