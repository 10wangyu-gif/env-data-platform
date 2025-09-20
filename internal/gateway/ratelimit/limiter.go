package ratelimit

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

// LimitStrategy 限流策略
type LimitStrategy string

const (
	TokenBucket   LimitStrategy = "token_bucket"
	SlidingWindow LimitStrategy = "sliding_window"
	FixedWindow   LimitStrategy = "fixed_window"
)

// LimitConfig 限流配置
type LimitConfig struct {
	Strategy    LimitStrategy `json:"strategy" yaml:"strategy"`
	Rate        int           `json:"rate" yaml:"rate"`           // 每秒请求数
	Burst       int           `json:"burst" yaml:"burst"`         // 突发请求数
	Window      time.Duration `json:"window" yaml:"window"`       // 时间窗口
	KeyFunc     KeyFunc       `json:"-" yaml:"-"`                 // 键生成函数
	SkipFunc    SkipFunc      `json:"-" yaml:"-"`                 // 跳过函数
	Message     string        `json:"message" yaml:"message"`     // 限流消息
	StatusCode  int           `json:"status_code" yaml:"status_code"` // 状态码
}

// KeyFunc 生成限流键的函数
type KeyFunc func(c *gin.Context) string

// SkipFunc 跳过限流检查的函数
type SkipFunc func(c *gin.Context) bool

// RateLimiter 限流器接口
type RateLimiter interface {
	Allow(ctx context.Context, key string) (bool, error)
	GetStats(ctx context.Context, key string) (*LimitStats, error)
	Reset(ctx context.Context, key string) error
}

// LimitStats 限流统计
type LimitStats struct {
	Key           string        `json:"key"`
	Remaining     int           `json:"remaining"`
	Limit         int           `json:"limit"`
	ResetTime     time.Time     `json:"reset_time"`
	Window        time.Duration `json:"window"`
	RequestCount  int64         `json:"request_count"`
	BlockedCount  int64         `json:"blocked_count"`
}

// TokenBucketLimiter 令牌桶限流器
type TokenBucketLimiter struct {
	limiters map[string]*rate.Limiter
	mutex    sync.RWMutex
	rate     rate.Limit
	burst    int
	logger   *zap.Logger
}

// SlidingWindowLimiter 滑动窗口限流器
type SlidingWindowLimiter struct {
	redis    *redis.Client
	window   time.Duration
	limit    int
	logger   *zap.Logger
}

// FixedWindowLimiter 固定窗口限流器
type FixedWindowLimiter struct {
	redis    *redis.Client
	window   time.Duration
	limit    int
	logger   *zap.Logger
}

// NewTokenBucketLimiter 创建令牌桶限流器
func NewTokenBucketLimiter(rateLimit int, burst int, logger *zap.Logger) *TokenBucketLimiter {
	return &TokenBucketLimiter{
		limiters: make(map[string]*rate.Limiter),
		rate:     rate.Limit(rateLimit),
		burst:    burst,
		logger:   logger,
	}
}

// NewSlidingWindowLimiter 创建滑动窗口限流器
func NewSlidingWindowLimiter(redis *redis.Client, window time.Duration, limit int, logger *zap.Logger) *SlidingWindowLimiter {
	return &SlidingWindowLimiter{
		redis:  redis,
		window: window,
		limit:  limit,
		logger: logger,
	}
}

// NewFixedWindowLimiter 创建固定窗口限流器
func NewFixedWindowLimiter(redis *redis.Client, window time.Duration, limit int, logger *zap.Logger) *FixedWindowLimiter {
	return &FixedWindowLimiter{
		redis:  redis,
		window: window,
		limit:  limit,
		logger: logger,
	}
}

// Allow 检查是否允许请求
func (tbl *TokenBucketLimiter) Allow(ctx context.Context, key string) (bool, error) {
	tbl.mutex.Lock()
	limiter, exists := tbl.limiters[key]
	if !exists {
		limiter = rate.NewLimiter(tbl.rate, tbl.burst)
		tbl.limiters[key] = limiter
	}
	tbl.mutex.Unlock()

	return limiter.Allow(), nil
}

// GetStats 获取统计信息
func (tbl *TokenBucketLimiter) GetStats(ctx context.Context, key string) (*LimitStats, error) {
	tbl.mutex.RLock()
	limiter, exists := tbl.limiters[key]
	tbl.mutex.RUnlock()

	if !exists {
		return &LimitStats{
			Key:       key,
			Remaining: tbl.burst,
			Limit:     tbl.burst,
		}, nil
	}

	tokens := int(limiter.Tokens())
	return &LimitStats{
		Key:       key,
		Remaining: tokens,
		Limit:     tbl.burst,
	}, nil
}

// Reset 重置限流器
func (tbl *TokenBucketLimiter) Reset(ctx context.Context, key string) error {
	tbl.mutex.Lock()
	defer tbl.mutex.Unlock()

	delete(tbl.limiters, key)
	return nil
}

// Allow 检查是否允许请求
func (swl *SlidingWindowLimiter) Allow(ctx context.Context, key string) (bool, error) {
	now := time.Now()
	windowStart := now.Add(-swl.window)

	pipe := swl.redis.Pipeline()

	// 删除窗口外的记录
	pipe.ZRemRangeByScore(ctx, key, "0", strconv.FormatInt(windowStart.UnixNano(), 10))

	// 添加当前请求
	pipe.ZAdd(ctx, key, redis.Z{
		Score:  float64(now.UnixNano()),
		Member: fmt.Sprintf("%d", now.UnixNano()),
	})

	// 获取窗口内的请求数
	pipe.ZCard(ctx, key)

	// 设置过期时间
	pipe.Expire(ctx, key, swl.window)

	results, err := pipe.Exec(ctx)
	if err != nil {
		swl.logger.Error("Redis pipeline failed", zap.Error(err))
		return false, err
	}

	count := results[2].(*redis.IntCmd).Val()

	swl.logger.Debug("Sliding window check",
		zap.String("key", key),
		zap.Int64("count", count),
		zap.Int("limit", swl.limit))

	return count <= int64(swl.limit), nil
}

// GetStats 获取统计信息
func (swl *SlidingWindowLimiter) GetStats(ctx context.Context, key string) (*LimitStats, error) {
	now := time.Now()
	windowStart := now.Add(-swl.window)

	count, err := swl.redis.ZCount(ctx, key,
		strconv.FormatInt(windowStart.UnixNano(), 10),
		strconv.FormatInt(now.UnixNano(), 10)).Result()
	if err != nil {
		return nil, err
	}

	return &LimitStats{
		Key:          key,
		Remaining:    swl.limit - int(count),
		Limit:        swl.limit,
		ResetTime:    now.Add(swl.window),
		Window:       swl.window,
		RequestCount: count,
	}, nil
}

// Reset 重置限流器
func (swl *SlidingWindowLimiter) Reset(ctx context.Context, key string) error {
	return swl.redis.Del(ctx, key).Err()
}

// Allow 检查是否允许请求
func (fwl *FixedWindowLimiter) Allow(ctx context.Context, key string) (bool, error) {
	now := time.Now()
	window := now.Truncate(fwl.window).Unix()
	windowKey := fmt.Sprintf("%s:%d", key, window)

	pipe := swl.redis.Pipeline()
	pipe.Incr(ctx, windowKey)
	pipe.Expire(ctx, windowKey, fwl.window)

	results, err := pipe.Exec(ctx)
	if err != nil {
		fwl.logger.Error("Redis pipeline failed", zap.Error(err))
		return false, err
	}

	count := results[0].(*redis.IntCmd).Val()

	fwl.logger.Debug("Fixed window check",
		zap.String("key", key),
		zap.Int64("count", count),
		zap.Int("limit", fwl.limit))

	return count <= int64(fwl.limit), nil
}

// GetStats 获取统计信息
func (fwl *FixedWindowLimiter) GetStats(ctx context.Context, key string) (*LimitStats, error) {
	now := time.Now()
	window := now.Truncate(fwl.window).Unix()
	windowKey := fmt.Sprintf("%s:%d", key, window)

	count, err := fwl.redis.Get(ctx, windowKey).Int64()
	if err != nil && err != redis.Nil {
		return nil, err
	}

	resetTime := time.Unix(window, 0).Add(fwl.window)

	return &LimitStats{
		Key:          key,
		Remaining:    fwl.limit - int(count),
		Limit:        fwl.limit,
		ResetTime:    resetTime,
		Window:       fwl.window,
		RequestCount: count,
	}, nil
}

// Reset 重置限流器
func (fwl *FixedWindowLimiter) Reset(ctx context.Context, key string) error {
	now := time.Now()
	window := now.Truncate(fwl.window).Unix()
	windowKey := fmt.Sprintf("%s:%d", key, window)

	return fwl.redis.Del(ctx, windowKey).Err()
}

// Middleware 限流中间件
func Middleware(limiter RateLimiter, config *LimitConfig) gin.HandlerFunc {
	if config.KeyFunc == nil {
		config.KeyFunc = DefaultKeyFunc
	}
	if config.StatusCode == 0 {
		config.StatusCode = http.StatusTooManyRequests
	}
	if config.Message == "" {
		config.Message = "Too many requests"
	}

	return func(c *gin.Context) {
		// 检查是否跳过限流
		if config.SkipFunc != nil && config.SkipFunc(c) {
			c.Next()
			return
		}

		// 生成限流键
		key := config.KeyFunc(c)

		// 检查限流
		allowed, err := limiter.Allow(c.Request.Context(), key)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "rate limiter error",
				"message": err.Error(),
			})
			c.Abort()
			return
		}

		// 获取统计信息
		stats, _ := limiter.GetStats(c.Request.Context(), key)
		if stats != nil {
			c.Header("X-RateLimit-Limit", strconv.Itoa(stats.Limit))
			c.Header("X-RateLimit-Remaining", strconv.Itoa(stats.Remaining))
			if !stats.ResetTime.IsZero() {
				c.Header("X-RateLimit-Reset", strconv.FormatInt(stats.ResetTime.Unix(), 10))
			}
		}

		if !allowed {
			c.JSON(config.StatusCode, gin.H{
				"error": "rate limit exceeded",
				"message": config.Message,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// DefaultKeyFunc 默认键生成函数（基于IP）
func DefaultKeyFunc(c *gin.Context) string {
	return fmt.Sprintf("ratelimit:ip:%s", c.ClientIP())
}

// APIKeyFunc API密钥限流键生成函数
func APIKeyFunc(c *gin.Context) string {
	if apiKey := c.GetHeader("X-API-Key"); apiKey != "" {
		return fmt.Sprintf("ratelimit:apikey:%s", apiKey)
	}
	return DefaultKeyFunc(c)
}

// UserIDFunc 用户ID限流键生成函数
func UserIDFunc(c *gin.Context) string {
	if userID, exists := c.Get("user_id"); exists {
		return fmt.Sprintf("ratelimit:user:%s", userID)
	}
	return DefaultKeyFunc(c)
}

// PathBasedKeyFunc 基于路径的限流键生成函数
func PathBasedKeyFunc(c *gin.Context) string {
	return fmt.Sprintf("ratelimit:path:%s:%s", c.Request.Method, c.Request.URL.Path)
}

// SkipInternalFunc 跳过内部请求
func SkipInternalFunc(c *gin.Context) bool {
	return c.GetHeader("X-Internal-Request") == "true"
}

// SkipHealthCheckFunc 跳过健康检查
func SkipHealthCheckFunc(c *gin.Context) bool {
	return c.Request.URL.Path == "/health" || c.Request.URL.Path == "/ping"
}

// CreateRateLimiter 创建限流器
func CreateRateLimiter(strategy LimitStrategy, config *LimitConfig, redis *redis.Client, logger *zap.Logger) (RateLimiter, error) {
	switch strategy {
	case TokenBucket:
		return NewTokenBucketLimiter(config.Rate, config.Burst, logger), nil
	case SlidingWindow:
		return NewSlidingWindowLimiter(redis, config.Window, config.Rate, logger), nil
	case FixedWindow:
		return NewFixedWindowLimiter(redis, config.Window, config.Rate, logger), nil
	default:
		return nil, fmt.Errorf("unsupported limit strategy: %s", strategy)
	}
}