package metrics

import (
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
)

// Metrics 网关指标收集器
type Collector struct {
	logger *zap.Logger

	// HTTP请求指标
	requestsTotal     *prometheus.CounterVec
	requestDuration   *prometheus.HistogramVec
	requestsInFlight  *prometheus.GaugeVec
	responseSize      *prometheus.HistogramVec

	// 错误指标
	errorsTotal       *prometheus.CounterVec

	// 上游服务指标
	upstreamDuration  *prometheus.HistogramVec
	upstreamStatus    *prometheus.CounterVec

	// 连接指标
	activeConnections prometheus.Gauge
	connectionErrors  *prometheus.CounterVec

	// 限流指标
	rateLimitHits     *prometheus.CounterVec
	rateLimitRemaining *prometheus.GaugeVec

	// 认证指标
	authAttempts      *prometheus.CounterVec
	authFailures      *prometheus.CounterVec

	// 缓存指标
	cacheHits         *prometheus.CounterVec
	cacheMisses       *prometheus.CounterVec

	// 自定义指标存储
	customMetrics     map[string]prometheus.Collector
	mutex             sync.RWMutex
}

// RequestMetrics 请求指标
type RequestMetrics struct {
	Method       string
	Path         string
	StatusCode   int
	Duration     time.Duration
	ResponseSize int64
	UserAgent    string
	ClientIP     string
	APIKey       string
	UserID       string
	Upstream     string
	Error        error
}

// NewCollector 创建指标收集器
func NewCollector(logger *zap.Logger) *Collector {
	return &Collector{
		logger: logger,
		requestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "gateway_requests_total",
				Help: "Total number of gateway requests",
			},
			[]string{"method", "path", "status", "api_key", "user_id"},
		),
		requestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "gateway_request_duration_seconds",
				Help:    "Gateway request duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "path", "status"},
		),
		requestsInFlight: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "gateway_requests_in_flight",
				Help: "Number of gateway requests currently being processed",
			},
			[]string{"method", "path"},
		),
		responseSize: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "gateway_response_size_bytes",
				Help:    "Gateway response size in bytes",
				Buckets: []float64{100, 1000, 10000, 100000, 1000000, 10000000},
			},
			[]string{"method", "path", "status"},
		),
		errorsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "gateway_errors_total",
				Help: "Total number of gateway errors",
			},
			[]string{"type", "method", "path"},
		),
		upstreamDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "gateway_upstream_duration_seconds",
				Help:    "Upstream service response duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"upstream", "method", "path", "status"},
		),
		upstreamStatus: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "gateway_upstream_requests_total",
				Help: "Total number of upstream requests",
			},
			[]string{"upstream", "method", "path", "status"},
		),
		activeConnections: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "gateway_active_connections",
				Help: "Number of active connections",
			},
		),
		connectionErrors: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "gateway_connection_errors_total",
				Help: "Total number of connection errors",
			},
			[]string{"type", "upstream"},
		),
		rateLimitHits: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "gateway_rate_limit_hits_total",
				Help: "Total number of rate limit hits",
			},
			[]string{"limiter", "key_type"},
		),
		rateLimitRemaining: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "gateway_rate_limit_remaining",
				Help: "Remaining rate limit quota",
			},
			[]string{"limiter", "key"},
		),
		authAttempts: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "gateway_auth_attempts_total",
				Help: "Total number of authentication attempts",
			},
			[]string{"method", "api_key", "user_id"},
		),
		authFailures: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "gateway_auth_failures_total",
				Help: "Total number of authentication failures",
			},
			[]string{"method", "reason"},
		),
		cacheHits: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "gateway_cache_hits_total",
				Help: "Total number of cache hits",
			},
			[]string{"cache_type", "key_pattern"},
		),
		cacheMisses: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "gateway_cache_misses_total",
				Help: "Total number of cache misses",
			},
			[]string{"cache_type", "key_pattern"},
		),
		customMetrics: make(map[string]prometheus.Collector),
	}
}

// Middleware 指标收集中间件
func (c *Collector) Middleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		start := time.Now()
		path := ctx.FullPath()
		if path == "" {
			path = ctx.Request.URL.Path
		}

		// 增加正在处理的请求数
		c.requestsInFlight.WithLabelValues(ctx.Request.Method, path).Inc()
		defer c.requestsInFlight.WithLabelValues(ctx.Request.Method, path).Dec()

		// 处理请求
		ctx.Next()

		// 收集指标
		duration := time.Since(start)
		statusCode := ctx.Writer.Status()
		responseSize := int64(ctx.Writer.Size())

		// 获取用户信息
		apiKey := ctx.GetHeader("X-API-Key")
		userID := ""
		if user, exists := ctx.Get("user_id"); exists {
			userID = user.(string)
		}

		// 记录基础指标
		c.recordRequest(&RequestMetrics{
			Method:       ctx.Request.Method,
			Path:         path,
			StatusCode:   statusCode,
			Duration:     duration,
			ResponseSize: responseSize,
			UserAgent:    ctx.GetHeader("User-Agent"),
			ClientIP:     ctx.ClientIP(),
			APIKey:       apiKey,
			UserID:       userID,
		})

		c.logger.Debug("Request processed",
			zap.String("method", ctx.Request.Method),
			zap.String("path", path),
			zap.Int("status", statusCode),
			zap.Duration("duration", duration),
			zap.Int64("response_size", responseSize))
	}
}

// recordRequest 记录请求指标
func (c *Collector) recordRequest(metrics *RequestMetrics) {
	statusStr := strconv.Itoa(metrics.StatusCode)

	// 基础请求指标
	c.requestsTotal.WithLabelValues(
		metrics.Method,
		metrics.Path,
		statusStr,
		metrics.APIKey,
		metrics.UserID,
	).Inc()

	c.requestDuration.WithLabelValues(
		metrics.Method,
		metrics.Path,
		statusStr,
	).Observe(metrics.Duration.Seconds())

	c.responseSize.WithLabelValues(
		metrics.Method,
		metrics.Path,
		statusStr,
	).Observe(float64(metrics.ResponseSize))

	// 错误指标
	if metrics.StatusCode >= 400 {
		errorType := "client_error"
		if metrics.StatusCode >= 500 {
			errorType = "server_error"
		}
		c.errorsTotal.WithLabelValues(
			errorType,
			metrics.Method,
			metrics.Path,
		).Inc()
	}
}

// RecordUpstreamRequest 记录上游请求指标
func (c *Collector) RecordUpstreamRequest(upstream, method, path string, status int, duration time.Duration) {
	statusStr := strconv.Itoa(status)

	c.upstreamStatus.WithLabelValues(upstream, method, path, statusStr).Inc()
	c.upstreamDuration.WithLabelValues(upstream, method, path, statusStr).Observe(duration.Seconds())
}

// RecordConnectionError 记录连接错误
func (c *Collector) RecordConnectionError(errorType, upstream string) {
	c.connectionErrors.WithLabelValues(errorType, upstream).Inc()
}

// RecordRateLimitHit 记录限流命中
func (c *Collector) RecordRateLimitHit(limiter, keyType string) {
	c.rateLimitHits.WithLabelValues(limiter, keyType).Inc()
}

// UpdateRateLimitRemaining 更新剩余限流配额
func (c *Collector) UpdateRateLimitRemaining(limiter, key string, remaining int) {
	c.rateLimitRemaining.WithLabelValues(limiter, key).Set(float64(remaining))
}

// RecordAuthAttempt 记录认证尝试
func (c *Collector) RecordAuthAttempt(method, apiKey, userID string) {
	c.authAttempts.WithLabelValues(method, apiKey, userID).Inc()
}

// RecordAuthFailure 记录认证失败
func (c *Collector) RecordAuthFailure(method, reason string) {
	c.authFailures.WithLabelValues(method, reason).Inc()
}

// RecordCacheHit 记录缓存命中
func (c *Collector) RecordCacheHit(cacheType, keyPattern string) {
	c.cacheHits.WithLabelValues(cacheType, keyPattern).Inc()
}

// RecordCacheMiss 记录缓存未命中
func (c *Collector) RecordCacheMiss(cacheType, keyPattern string) {
	c.cacheMisses.WithLabelValues(cacheType, keyPattern).Inc()
}

// UpdateActiveConnections 更新活跃连接数
func (c *Collector) UpdateActiveConnections(count int) {
	c.activeConnections.Set(float64(count))
}

// RegisterCustomMetric 注册自定义指标
func (c *Collector) RegisterCustomMetric(name string, metric prometheus.Collector) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if err := prometheus.Register(metric); err != nil {
		return err
	}

	c.customMetrics[name] = metric
	c.logger.Info("Custom metric registered", zap.String("name", name))
	return nil
}

// UnregisterCustomMetric 注销自定义指标
func (c *Collector) UnregisterCustomMetric(name string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if metric, exists := c.customMetrics[name]; exists {
		prometheus.Unregister(metric)
		delete(c.customMetrics, name)
		c.logger.Info("Custom metric unregistered", zap.String("name", name))
	}
}

// GetStats 获取统计信息
func (c *Collector) GetStats() map[string]interface{} {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return map[string]interface{}{
		"custom_metrics_count": len(c.customMetrics),
		"registered_metrics": func() []string {
			names := make([]string, 0, len(c.customMetrics))
			for name := range c.customMetrics {
				names = append(names, name)
			}
			return names
		}(),
	}
}

// HealthMetrics 健康检查指标
type HealthMetrics struct {
	Timestamp         time.Time `json:"timestamp"`
	TotalRequests     int64     `json:"total_requests"`
	ErrorRate         float64   `json:"error_rate"`
	AverageLatency    float64   `json:"average_latency"`
	ActiveConnections int       `json:"active_connections"`
	UpstreamStatus    map[string]string `json:"upstream_status"`
}

// GetHealthMetrics 获取健康检查指标
func (c *Collector) GetHealthMetrics() *HealthMetrics {
	// 这里应该从Prometheus查询实际数据，简化处理
	return &HealthMetrics{
		Timestamp:         time.Now(),
		TotalRequests:     0, // 实际应该查询Prometheus
		ErrorRate:         0.0,
		AverageLatency:    0.0,
		ActiveConnections: 0,
		UpstreamStatus:    make(map[string]string),
	}
}

// ResetMetrics 重置指标（用于测试）
func (c *Collector) ResetMetrics() {
	c.logger.Warn("Resetting all metrics")

	// 注意：这会重置所有指标，生产环境慎用
	c.requestsTotal.Reset()
	c.requestDuration.Reset()
	c.requestsInFlight.Reset()
	c.responseSize.Reset()
	c.errorsTotal.Reset()
	c.upstreamDuration.Reset()
	c.upstreamStatus.Reset()
	c.connectionErrors.Reset()
	c.rateLimitHits.Reset()
	c.rateLimitRemaining.Reset()
	c.authAttempts.Reset()
	c.authFailures.Reset()
	c.cacheHits.Reset()
	c.cacheMisses.Reset()
}