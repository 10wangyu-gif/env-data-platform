package gateway

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ServiceInfo 服务信息
type ServiceInfo struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Address     string            `json:"address"`
	Port        int               `json:"port"`
	Tags        []string          `json:"tags"`
	Metadata    map[string]string `json:"metadata"`
	Health      HealthStatus      `json:"health"`
	RegisteredAt time.Time        `json:"registered_at"`
	LastSeen    time.Time         `json:"last_seen"`
}

// HealthStatus 健康状态
type HealthStatus struct {
	Status      string    `json:"status"`      // healthy, unhealthy, unknown
	LastCheck   time.Time `json:"last_check"`
	CheckCount  int64     `json:"check_count"`
	FailCount   int64     `json:"fail_count"`
	Latency     time.Duration `json:"latency"`
	Message     string    `json:"message"`
}

// ServiceDiscovery 服务发现
type ServiceDiscovery struct {
	services    map[string]*ServiceInfo
	healthCheck *HealthChecker
	mutex       sync.RWMutex
	logger      *zap.Logger
	stopCh      chan struct{}
	wg          sync.WaitGroup
}

// HealthChecker 健康检查器
type HealthChecker struct {
	config   *HealthCheckConfig
	client   *http.Client
	logger   *zap.Logger
}

// NewServiceDiscovery 创建服务发现
func NewServiceDiscovery(config *HealthCheckConfig, logger *zap.Logger) *ServiceDiscovery {
	healthChecker := &HealthChecker{
		config: config,
		client: &http.Client{
			Timeout: config.Timeout,
		},
		logger: logger,
	}

	return &ServiceDiscovery{
		services:    make(map[string]*ServiceInfo),
		healthCheck: healthChecker,
		logger:      logger,
		stopCh:      make(chan struct{}),
	}
}

// Start 启动服务发现
func (sd *ServiceDiscovery) Start(ctx context.Context) error {
	if !sd.healthCheck.config.Enabled {
		sd.logger.Info("Health check disabled, skipping service discovery")
		return nil
	}

	sd.logger.Info("Starting service discovery",
		zap.Duration("interval", sd.healthCheck.config.Interval))

	sd.wg.Add(1)
	go sd.healthCheckLoop(ctx)

	return nil
}

// Stop 停止服务发现
func (sd *ServiceDiscovery) Stop() error {
	sd.logger.Info("Stopping service discovery")
	close(sd.stopCh)
	sd.wg.Wait()
	return nil
}

// RegisterService 注册服务
func (sd *ServiceDiscovery) RegisterService(service *ServiceInfo) {
	sd.mutex.Lock()
	defer sd.mutex.Unlock()

	service.RegisteredAt = time.Now()
	service.LastSeen = time.Now()
	service.Health = HealthStatus{
		Status:     "unknown",
		LastCheck:  time.Time{},
		CheckCount: 0,
		FailCount:  0,
	}

	sd.services[service.ID] = service

	sd.logger.Info("Service registered",
		zap.String("service_id", service.ID),
		zap.String("service_name", service.Name),
		zap.String("address", service.Address),
		zap.Int("port", service.Port))
}

// DeregisterService 注销服务
func (sd *ServiceDiscovery) DeregisterService(serviceID string) {
	sd.mutex.Lock()
	defer sd.mutex.Unlock()

	if service, exists := sd.services[serviceID]; exists {
		delete(sd.services, serviceID)
		sd.logger.Info("Service deregistered",
			zap.String("service_id", serviceID),
			zap.String("service_name", service.Name))
	}
}

// GetService 获取服务
func (sd *ServiceDiscovery) GetService(serviceID string) (*ServiceInfo, bool) {
	sd.mutex.RLock()
	defer sd.mutex.RUnlock()

	service, exists := sd.services[serviceID]
	return service, exists
}

// GetServices 获取所有服务
func (sd *ServiceDiscovery) GetServices() []*ServiceInfo {
	sd.mutex.RLock()
	defer sd.mutex.RUnlock()

	services := make([]*ServiceInfo, 0, len(sd.services))
	for _, service := range sd.services {
		services = append(services, service)
	}
	return services
}

// GetHealthyServices 获取健康的服务
func (sd *ServiceDiscovery) GetHealthyServices() []*ServiceInfo {
	sd.mutex.RLock()
	defer sd.mutex.RUnlock()

	services := make([]*ServiceInfo, 0)
	for _, service := range sd.services {
		if service.Health.Status == "healthy" {
			services = append(services, service)
		}
	}
	return services
}

// GetServicesByTag 根据标签获取服务
func (sd *ServiceDiscovery) GetServicesByTag(tag string) []*ServiceInfo {
	sd.mutex.RLock()
	defer sd.mutex.RUnlock()

	services := make([]*ServiceInfo, 0)
	for _, service := range sd.services {
		for _, t := range service.Tags {
			if t == tag {
				services = append(services, service)
				break
			}
		}
	}
	return services
}

// UpdateServiceHealth 更新服务健康状态
func (sd *ServiceDiscovery) UpdateServiceHealth(serviceID string, health HealthStatus) {
	sd.mutex.Lock()
	defer sd.mutex.Unlock()

	if service, exists := sd.services[serviceID]; exists {
		service.Health = health
		service.LastSeen = time.Now()
	}
}

// healthCheckLoop 健康检查循环
func (sd *ServiceDiscovery) healthCheckLoop(ctx context.Context) {
	defer sd.wg.Done()

	ticker := time.NewTicker(sd.healthCheck.config.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-sd.stopCh:
			return
		case <-ticker.C:
			sd.performHealthChecks()
		}
	}
}

// performHealthChecks 执行健康检查
func (sd *ServiceDiscovery) performHealthChecks() {
	services := sd.GetServices()

	for _, service := range services {
		go sd.checkServiceHealth(service)
	}
}

// checkServiceHealth 检查单个服务健康状态
func (sd *ServiceDiscovery) checkServiceHealth(service *ServiceInfo) {
	start := time.Now()

	url := fmt.Sprintf("http://%s:%d%s",
		service.Address,
		service.Port,
		sd.healthCheck.config.Path)

	req, err := http.NewRequest(sd.healthCheck.config.Method, url, nil)
	if err != nil {
		sd.updateHealthStatus(service.ID, "unhealthy", time.Since(start), err.Error())
		return
	}

	resp, err := sd.healthCheck.client.Do(req)
	if err != nil {
		sd.updateHealthStatus(service.ID, "unhealthy", time.Since(start), err.Error())
		return
	}
	defer resp.Body.Close()

	latency := time.Since(start)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		sd.updateHealthStatus(service.ID, "healthy", latency, "")
	} else {
		sd.updateHealthStatus(service.ID, "unhealthy", latency,
			fmt.Sprintf("HTTP %d", resp.StatusCode))
	}
}

// updateHealthStatus 更新健康状态
func (sd *ServiceDiscovery) updateHealthStatus(serviceID, status string, latency time.Duration, message string) {
	sd.mutex.Lock()
	defer sd.mutex.Unlock()

	service, exists := sd.services[serviceID]
	if !exists {
		return
	}

	// 更新健康状态
	oldStatus := service.Health.Status
	service.Health.Status = status
	service.Health.LastCheck = time.Now()
	service.Health.CheckCount++
	service.Health.Latency = latency
	service.Health.Message = message

	if status == "unhealthy" {
		service.Health.FailCount++
	}

	// 记录状态变化
	if oldStatus != status {
		sd.logger.Info("Service health status changed",
			zap.String("service_id", serviceID),
			zap.String("service_name", service.Name),
			zap.String("old_status", oldStatus),
			zap.String("new_status", status),
			zap.Duration("latency", latency),
			zap.String("message", message))
	}
}

// GetStats 获取统计信息
func (sd *ServiceDiscovery) GetStats() map[string]interface{} {
	sd.mutex.RLock()
	defer sd.mutex.RUnlock()

	total := len(sd.services)
	healthy := 0
	unhealthy := 0
	unknown := 0

	for _, service := range sd.services {
		switch service.Health.Status {
		case "healthy":
			healthy++
		case "unhealthy":
			unhealthy++
		default:
			unknown++
		}
	}

	return map[string]interface{}{
		"total_services":    total,
		"healthy_services":  healthy,
		"unhealthy_services": unhealthy,
		"unknown_services":  unknown,
		"health_check_enabled": sd.healthCheck.config.Enabled,
		"check_interval":    sd.healthCheck.config.Interval.String(),
	}
}

// CreateServiceFromTarget 从目标配置创建服务信息
func CreateServiceFromTarget(target *Target) *ServiceInfo {
	// 解析URL获取地址和端口
	address := "localhost"
	port := 80

	// 这里应该解析target.URL获取实际的地址和端口
	// 简化处理，实际应该使用url.Parse

	return &ServiceInfo{
		ID:       target.ID,
		Name:     fmt.Sprintf("Target-%s", target.ID),
		Address:  address,
		Port:     port,
		Tags:     []string{"gateway", "target"},
		Metadata: target.Metadata,
		Health: HealthStatus{
			Status: "unknown",
		},
	}
}

// CreateServiceFromConfig 从配置创建服务信息
func CreateServiceFromConfig(config *ServiceConfig) []*ServiceInfo {
	services := make([]*ServiceInfo, 0, len(config.Targets))

	for _, target := range config.Targets {
		service := &ServiceInfo{
			ID:       target.ID,
			Name:     config.Name,
			Address:  target.URL, // 简化处理
			Port:     80,         // 应该从URL解析
			Tags:     []string{config.ID},
			Metadata: target.Metadata,
			Health: HealthStatus{
				Status: "unknown",
			},
		}
		services = append(services, service)
	}

	return services
}