package gateway

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Route 定义API路由配置
type Route struct {
	ID          string            `json:"id" yaml:"id"`
	Path        string            `json:"path" yaml:"path"`
	Method      string            `json:"method" yaml:"method"`
	Target      string            `json:"target" yaml:"target"`
	StripPrefix bool              `json:"strip_prefix" yaml:"strip_prefix"`
	Headers     map[string]string `json:"headers" yaml:"headers"`
	Timeout     time.Duration     `json:"timeout" yaml:"timeout"`
	Retries     int               `json:"retries" yaml:"retries"`
}

// Router API网关路由器
type Router struct {
	routes    map[string]*Route
	proxies   map[string]*httputil.ReverseProxy
	mutex     sync.RWMutex
	logger    *zap.Logger
	balancer  *LoadBalancer
	discovery *ServiceDiscovery
}

// NewRouter 创建新的路由器
func NewRouter(logger *zap.Logger, balancer *LoadBalancer, discovery *ServiceDiscovery) *Router {
	return &Router{
		routes:    make(map[string]*Route),
		proxies:   make(map[string]*httputil.ReverseProxy),
		logger:    logger,
		balancer:  balancer,
		discovery: discovery,
	}
}

// AddRoute 添加路由
func (r *Router) AddRoute(route *Route) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// 解析目标URL
	target, err := url.Parse(route.Target)
	if err != nil {
		return fmt.Errorf("invalid target URL: %w", err)
	}

	// 创建反向代理
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ModifyResponse = r.modifyResponse
	proxy.ErrorHandler = r.errorHandler

	routeKey := fmt.Sprintf("%s:%s", route.Method, route.Path)
	r.routes[routeKey] = route
	r.proxies[routeKey] = proxy

	r.logger.Info("Route added",
		zap.String("method", route.Method),
		zap.String("path", route.Path),
		zap.String("target", route.Target))

	return nil
}

// RemoveRoute 删除路由
func (r *Router) RemoveRoute(method, path string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	routeKey := fmt.Sprintf("%s:%s", method, path)
	delete(r.routes, routeKey)
	delete(r.proxies, routeKey)

	r.logger.Info("Route removed",
		zap.String("method", method),
		zap.String("path", path))
}

// GetRoute 获取路由
func (r *Router) GetRoute(method, path string) (*Route, bool) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	routeKey := fmt.Sprintf("%s:%s", method, path)
	route, exists := r.routes[routeKey]
	return route, exists
}

// ListRoutes 列出所有路由
func (r *Router) ListRoutes() []*Route {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	routes := make([]*Route, 0, len(r.routes))
	for _, route := range r.routes {
		routes = append(routes, route)
	}
	return routes
}

// HandleRequest 处理HTTP请求
func (r *Router) HandleRequest() gin.HandlerFunc {
	return func(c *gin.Context) {
		startTime := time.Now()

		// 查找匹配的路由
		route, proxy := r.findRoute(c.Request.Method, c.Request.URL.Path)
		if route == nil {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "route not found",
				"path":  c.Request.URL.Path,
			})
			return
		}

		// 记录请求信息
		r.logger.Info("Processing request",
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.String("target", route.Target),
			zap.String("client_ip", c.ClientIP()))

		// 设置请求上下文
		ctx := context.WithValue(c.Request.Context(), "route", route)
		ctx = context.WithValue(ctx, "start_time", startTime)
		c.Request = c.Request.WithContext(ctx)

		// 处理路径前缀
		if route.StripPrefix {
			c.Request.URL.Path = strings.TrimPrefix(c.Request.URL.Path, route.Path)
			if c.Request.URL.Path == "" {
				c.Request.URL.Path = "/"
			}
		}

		// 添加自定义请求头
		for key, value := range route.Headers {
			c.Request.Header.Set(key, value)
		}

		// 使用负载均衡器选择目标服务器
		if r.balancer != nil {
			if target := r.balancer.SelectTarget(route.ID); target != "" {
				if targetURL, err := url.Parse(target); err == nil {
					proxy = httputil.NewSingleHostReverseProxy(targetURL)
					proxy.ModifyResponse = r.modifyResponse
					proxy.ErrorHandler = r.errorHandler
				}
			}
		}

		// 执行代理请求
		proxy.ServeHTTP(c.Writer, c.Request)
	}
}

// findRoute 查找匹配的路由
func (r *Router) findRoute(method, path string) (*Route, *httputil.ReverseProxy) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	// 精确匹配
	routeKey := fmt.Sprintf("%s:%s", method, path)
	if route, exists := r.routes[routeKey]; exists {
		return route, r.proxies[routeKey]
	}

	// 前缀匹配
	for key, route := range r.routes {
		if strings.HasPrefix(key, method+":") &&
			strings.HasPrefix(path, strings.TrimPrefix(key, method+":")) {
			return route, r.proxies[key]
		}
	}

	return nil, nil
}

// modifyResponse 修改响应
func (r *Router) modifyResponse(resp *http.Response) error {
	// 添加网关标识头
	resp.Header.Set("X-Gateway", "env-data-platform")
	resp.Header.Set("X-Gateway-Version", "1.0.0")

	// 记录响应信息
	if route := resp.Request.Context().Value("route"); route != nil {
		r.logger.Info("Response processed",
			zap.String("status", resp.Status),
			zap.String("target", route.(*Route).Target))
	}

	return nil
}

// errorHandler 处理代理错误
func (r *Router) errorHandler(w http.ResponseWriter, req *http.Request, err error) {
	r.logger.Error("Proxy error",
		zap.Error(err),
		zap.String("method", req.Method),
		zap.String("path", req.URL.Path))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadGateway)
	w.Write([]byte(`{"error":"service unavailable","message":"` + err.Error() + `"}`))
}

// HealthCheck 健康检查处理器
func (r *Router) HealthCheck() gin.HandlerFunc {
	return func(c *gin.Context) {
		status := gin.H{
			"status": "healthy",
			"timestamp": time.Now().Unix(),
			"routes": len(r.routes),
		}

		// 检查服务发现状态
		if r.discovery != nil {
			services := r.discovery.GetServices()
			status["services"] = len(services)
		}

		c.JSON(http.StatusOK, status)
	}
}

// GetMetrics 获取路由指标
func (r *Router) GetMetrics() map[string]interface{} {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	return map[string]interface{}{
		"total_routes": len(r.routes),
		"routes":       r.routes,
	}
}