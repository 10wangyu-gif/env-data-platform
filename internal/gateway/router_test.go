package gateway

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestRouter(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	router := NewRouter(logger, nil, nil)

	// 测试添加路由
	route := &Route{
		ID:     "test-route",
		Path:   "/api/test",
		Method: "GET",
		Target: "http://example.com",
	}

	err := router.AddRoute(route)
	assert.NoError(t, err)

	// 测试获取路由
	retrievedRoute, exists := router.GetRoute("GET", "/api/test")
	assert.True(t, exists)
	assert.Equal(t, route.ID, retrievedRoute.ID)

	// 测试删除路由
	router.RemoveRoute("GET", "/api/test")
	_, exists = router.GetRoute("GET", "/api/test")
	assert.False(t, exists)
}

func TestRouterHealthCheck(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	router := NewRouter(logger, nil, nil)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/health", nil)

	handler := router.HealthCheck()
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRouteWithHeaders(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	router := NewRouter(logger, nil, nil)

	route := &Route{
		ID:     "test-route-headers",
		Path:   "/api/headers",
		Method: "POST",
		Target: "http://example.com",
		Headers: map[string]string{
			"X-Custom-Header": "test-value",
		},
		Timeout: 10 * time.Second,
		Retries: 2,
	}

	err := router.AddRoute(route)
	assert.NoError(t, err)

	routes := router.ListRoutes()
	assert.Len(t, routes, 1)
	assert.Equal(t, "test-value", routes[0].Headers["X-Custom-Header"])
}

func TestRouteStripPrefix(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	router := NewRouter(logger, nil, nil)

	route := &Route{
		ID:          "test-strip-prefix",
		Path:        "/api/v1",
		Method:      "GET",
		Target:      "http://example.com",
		StripPrefix: true,
	}

	err := router.AddRoute(route)
	assert.NoError(t, err)

	// 测试前缀剥离功能需要实际的HTTP请求处理
	// 这里主要测试配置正确性
	retrievedRoute, exists := router.GetRoute("GET", "/api/v1")
	assert.True(t, exists)
	assert.True(t, retrievedRoute.StripPrefix)
}

func TestRouterMetrics(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	router := NewRouter(logger, nil, nil)

	// 添加一些路由
	routes := []*Route{
		{ID: "route1", Path: "/api/test1", Method: "GET", Target: "http://example.com"},
		{ID: "route2", Path: "/api/test2", Method: "POST", Target: "http://example.com"},
	}

	for _, route := range routes {
		err := router.AddRoute(route)
		assert.NoError(t, err)
	}

	metrics := router.GetMetrics()
	assert.NotNil(t, metrics)
	assert.Equal(t, 2, metrics["total_routes"])
}

func BenchmarkRouterAddRoute(b *testing.B) {
	logger, _ := zap.NewDevelopment()
	router := NewRouter(logger, nil, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		route := &Route{
			ID:     fmt.Sprintf("route-%d", i),
			Path:   fmt.Sprintf("/api/test-%d", i),
			Method: "GET",
			Target: "http://example.com",
		}
		router.AddRoute(route)
	}
}

func BenchmarkRouterGetRoute(b *testing.B) {
	logger, _ := zap.NewDevelopment()
	router := NewRouter(logger, nil, nil)

	// 预先添加一些路由
	for i := 0; i < 1000; i++ {
		route := &Route{
			ID:     fmt.Sprintf("route-%d", i),
			Path:   fmt.Sprintf("/api/test-%d", i),
			Method: "GET",
			Target: "http://example.com",
		}
		router.AddRoute(route)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		router.GetRoute("GET", "/api/test-500")
	}
}

func TestRouterConcurrency(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	router := NewRouter(logger, nil, nil)

	// 并发添加路由
	done := make(chan bool, 100)
	for i := 0; i < 100; i++ {
		go func(index int) {
			route := &Route{
				ID:     fmt.Sprintf("concurrent-route-%d", index),
				Path:   fmt.Sprintf("/api/concurrent-%d", index),
				Method: "GET",
				Target: "http://example.com",
			}
			err := router.AddRoute(route)
			assert.NoError(t, err)
			done <- true
		}(i)
	}

	// 等待所有goroutine完成
	for i := 0; i < 100; i++ {
		<-done
	}

	// 验证所有路由都添加成功
	routes := router.ListRoutes()
	assert.Len(t, routes, 100)
}