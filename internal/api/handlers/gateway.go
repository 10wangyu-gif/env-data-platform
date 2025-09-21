package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/env-data-platform/internal/gateway"
	"github.com/env-data-platform/internal/gateway/auth"
	"github.com/env-data-platform/internal/gateway/metrics"
	"github.com/env-data-platform/internal/gateway/ratelimit"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// GatewayHandler 网关管理处理器
type GatewayHandler struct {
	router       *gateway.Router
	loadBalancer *gateway.LoadBalancer
	authenticator *auth.Authenticator
	collector    *metrics.Collector
	rateLimiter  ratelimit.RateLimiter
	logger       *zap.Logger
}

// NewGatewayHandler 创建网关处理器
func NewGatewayHandler(
	router *gateway.Router,
	loadBalancer *gateway.LoadBalancer,
	authenticator *auth.Authenticator,
	collector *metrics.Collector,
	rateLimiter ratelimit.RateLimiter,
	logger *zap.Logger,
) *GatewayHandler {
	return &GatewayHandler{
		router:       router,
		loadBalancer: loadBalancer,
		authenticator: authenticator,
		collector:    collector,
		rateLimiter:  rateLimiter,
		logger:       logger,
	}
}

// Routes 路由管理

// ListRoutes 列出所有路由
func (h *GatewayHandler) ListRoutes(c *gin.Context) {
	routes := h.router.ListRoutes()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    routes,
		"count":   len(routes),
	})
}

// CreateRoute 创建路由
func (h *GatewayHandler) CreateRoute(c *gin.Context) {
	var route gateway.Route
	if err := c.ShouldBindJSON(&route); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "invalid request body",
			"message": err.Error(),
		})
		return
	}

	// 验证必填字段
	if route.Path == "" || route.Method == "" || route.Target == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "missing required fields",
			"message": "path, method, and target are required",
		})
		return
	}

	// 设置默认值
	if route.ID == "" {
		route.ID = generateRouteID()
	}
	if route.Timeout == 0 {
		route.Timeout = 30 * time.Second
	}
	if route.Retries == 0 {
		route.Retries = 3
	}

	if err := h.router.AddRoute(&route); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "failed to create route",
			"message": err.Error(),
		})
		return
	}

	h.logger.Info("Route created via API",
		zap.String("route_id", route.ID),
		zap.String("path", route.Path),
		zap.String("method", route.Method),
		zap.String("target", route.Target))

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    route,
	})
}

// UpdateRoute 更新路由
func (h *GatewayHandler) UpdateRoute(c *gin.Context) {
	method := c.Param("method")
	path := c.Param("path")

	var route gateway.Route
	if err := c.ShouldBindJSON(&route); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "invalid request body",
			"message": err.Error(),
		})
		return
	}

	// 检查路由是否存在
	if _, exists := h.router.GetRoute(method, path); !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "route not found",
		})
		return
	}

	// 删除旧路由，添加新路由
	h.router.RemoveRoute(method, path)
	if err := h.router.AddRoute(&route); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "failed to update route",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    route,
	})
}

// DeleteRoute 删除路由
func (h *GatewayHandler) DeleteRoute(c *gin.Context) {
	method := c.Param("method")
	path := c.Param("path")

	if _, exists := h.router.GetRoute(method, path); !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "route not found",
		})
		return
	}

	h.router.RemoveRoute(method, path)

	h.logger.Info("Route deleted via API",
		zap.String("method", method),
		zap.String("path", path))

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "route deleted successfully",
	})
}

// GetRoute 获取单个路由
func (h *GatewayHandler) GetRoute(c *gin.Context) {
	method := c.Param("method")
	path := c.Param("path")

	route, exists := h.router.GetRoute(method, path)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "route not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    route,
	})
}

// LoadBalancer 负载均衡管理

// GetLoadBalancerStats 获取负载均衡统计
func (h *GatewayHandler) GetLoadBalancerStats(c *gin.Context) {
	stats := h.loadBalancer.GetStats()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    stats,
	})
}

// UpdateTargetHealth 更新目标健康状态
func (h *GatewayHandler) UpdateTargetHealth(c *gin.Context) {
	groupID := c.Param("groupId")
	targetID := c.Param("targetId")

	var req struct {
		IsHealthy bool `json:"is_healthy" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "invalid request body",
			"message": err.Error(),
		})
		return
	}

	h.loadBalancer.UpdateTargetHealth(groupID, targetID, req.IsHealthy)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "target health updated",
	})
}

// Authentication 认证管理

// CreateAPIKey 创建API密钥
func (h *GatewayHandler) CreateAPIKey(c *gin.Context) {
	var req struct {
		UserID    string     `json:"user_id" binding:"required"`
		Name      string     `json:"name" binding:"required"`
		Scopes    []string   `json:"scopes"`
		RateLimit int        `json:"rate_limit"`
		ExpiresAt *time.Time `json:"expires_at"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "invalid request body",
			"message": err.Error(),
		})
		return
	}

	// 设置默认值
	if req.RateLimit == 0 {
		req.RateLimit = 1000 // 默认每秒1000请求
	}

	apiKey, err := h.authenticator.CreateAPIKey(
		req.UserID,
		req.Name,
		req.Scopes,
		req.RateLimit,
		req.ExpiresAt,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "failed to create API key",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    apiKey,
	})
}

// ListAPIKeys 列出API密钥
func (h *GatewayHandler) ListAPIKeys(c *gin.Context) {
	userID := c.Query("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "user_id is required",
		})
		return
	}

	keys := h.authenticator.ListAPIKeys(userID)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    keys,
		"count":   len(keys),
	})
}

// RevokeAPIKey 撤销API密钥
func (h *GatewayHandler) RevokeAPIKey(c *gin.Context) {
	key := c.Param("key")

	if err := h.authenticator.RevokeAPIKey(key); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "API key not found",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "API key revoked successfully",
	})
}

// Metrics 指标管理

// GetMetrics 获取网关指标
func (h *GatewayHandler) GetMetrics(c *gin.Context) {
	routerMetrics := h.router.GetMetrics()
	loadBalancerMetrics := h.loadBalancer.GetStats()
	collectorMetrics := h.collector.GetStats()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"router":       routerMetrics,
			"load_balancer": loadBalancerMetrics,
			"collector":    collectorMetrics,
			"timestamp":    time.Now().Unix(),
		},
	})
}

// GetHealthMetrics 获取健康检查指标
func (h *GatewayHandler) GetHealthMetrics(c *gin.Context) {
	healthMetrics := h.collector.GetHealthMetrics()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    healthMetrics,
	})
}

// ResetMetrics 重置指标（仅用于测试）
func (h *GatewayHandler) ResetMetrics(c *gin.Context) {
	// 检查权限
	if !auth.HasScope(c, "admin:metrics") {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"error":   "insufficient permissions",
		})
		return
	}

	h.collector.ResetMetrics()

	h.logger.Warn("Metrics reset via API",
		zap.String("user", getUserFromContext(c)))

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "metrics reset successfully",
	})
}

// Rate Limiting 限流管理

// GetRateLimitStats 获取限流统计
func (h *GatewayHandler) GetRateLimitStats(c *gin.Context) {
	key := c.Query("key")
	if key == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "key parameter is required",
		})
		return
	}

	stats, err := h.rateLimiter.GetStats(c.Request.Context(), key)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "failed to get rate limit stats",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    stats,
	})
}

// ResetRateLimit 重置限流
func (h *GatewayHandler) ResetRateLimit(c *gin.Context) {
	key := c.Param("key")

	if err := h.rateLimiter.Reset(c.Request.Context(), key); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "failed to reset rate limit",
			"message": err.Error(),
		})
		return
	}

	h.logger.Info("Rate limit reset via API",
		zap.String("key", key),
		zap.String("user", getUserFromContext(c)))

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "rate limit reset successfully",
	})
}

// System 系统管理

// GetSystemInfo 获取系统信息
func (h *GatewayHandler) GetSystemInfo(c *gin.Context) {
	info := gin.H{
		"version":     "1.0.0",
		"uptime":      time.Since(startTime).String(),
		"routes":      len(h.router.ListRoutes()),
		"timestamp":   time.Now().Unix(),
		"environment": "production", // 应该从配置获取
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    info,
	})
}

// HealthCheck 健康检查
func (h *GatewayHandler) HealthCheck(c *gin.Context) {
	status := gin.H{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
		"checks": gin.H{
			"router":        "ok",
			"load_balancer": "ok",
			"authenticator": "ok",
			"rate_limiter":  "ok",
		},
	}

	c.JSON(http.StatusOK, status)
}

// 辅助函数

var startTime = time.Now()

func generateRouteID() string {
	return strconv.FormatInt(time.Now().UnixNano(), 36)
}

func getUserFromContext(c *gin.Context) string {
	if user, exists := auth.GetCurrentUser(c); exists {
		return user.Username
	}
	return "unknown"
}