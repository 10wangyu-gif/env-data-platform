package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/env-data-platform/internal/models"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
)

// ConnectionTestService 数据源连接测试服务
type ConnectionTestService struct{}

// NewConnectionTestService 创建连接测试服务实例
func NewConnectionTestService() *ConnectionTestService {
	return &ConnectionTestService{}
}

// ConnectionTestResult 连接测试结果
type ConnectionTestResult struct {
	Success   bool                   `json:"success"`
	Message   string                 `json:"message"`
	Latency   time.Duration          `json:"latency"`
	Details   map[string]interface{} `json:"details,omitempty"`
	TestedAt  time.Time              `json:"tested_at"`
}

// TestConnection 测试数据源连接
func (s *ConnectionTestService) TestConnection(ctx context.Context, dataSource *models.DataSource) *ConnectionTestResult {
	startTime := time.Now()

	result := &ConnectionTestResult{
		TestedAt: startTime,
		Details:  make(map[string]interface{}),
	}

	switch dataSource.Type {
	case "mysql":
		result = s.testMySQLConnection(ctx, dataSource, result)
	case "postgresql":
		result = s.testPostgreSQLConnection(ctx, dataSource, result)
	case "hj212":
		result = s.testHJ212Connection(ctx, dataSource, result)
	case "api":
		result = s.testAPIConnection(ctx, dataSource, result)
	default:
		result.Success = false
		result.Message = fmt.Sprintf("不支持的数据源类型: %s", dataSource.Type)
	}

	result.Latency = time.Since(startTime)
	return result
}

// testMySQLConnection 测试MySQL连接
func (s *ConnectionTestService) testMySQLConnection(ctx context.Context, dataSource *models.DataSource, result *ConnectionTestResult) *ConnectionTestResult {
	// 解析配置
	var config map[string]interface{}
	if err := json.Unmarshal(dataSource.ConfigData, &config); err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("解析配置失败: %v", err)
		return result
	}

	// 构建MySQL连接字符串
	host := config["host"].(string)
	port := int(config["port"].(float64))
	username := config["username"].(string)
	password := config["password"].(string)
	database := config["database"].(string)

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&charset=utf8mb4",
		username, password, host, port, database)

	// 测试连接
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("MySQL连接失败: %v", err)
		return result
	}
	defer db.Close()

	// 设置连接超时
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// 验证连接
	if err := db.PingContext(ctx); err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("MySQL ping失败: %v", err)
		return result
	}

	// 获取数据库版本信息
	var version string
	if err := db.QueryRowContext(ctx, "SELECT VERSION()").Scan(&version); err == nil {
		result.Details["version"] = version
	}

	// 获取数据库状态
	var dbSize int64
	query := "SELECT ROUND(SUM(data_length + index_length) / 1024 / 1024, 1) as db_size FROM information_schema.tables WHERE table_schema = ?"
	if err := db.QueryRowContext(ctx, query, database).Scan(&dbSize); err == nil {
		result.Details["database_size_mb"] = dbSize
	}

	result.Success = true
	result.Message = "MySQL连接成功"
	return result
}

// testPostgreSQLConnection 测试PostgreSQL连接
func (s *ConnectionTestService) testPostgreSQLConnection(ctx context.Context, dataSource *models.DataSource, result *ConnectionTestResult) *ConnectionTestResult {
	// 解析配置
	var config map[string]interface{}
	if err := json.Unmarshal(dataSource.ConfigData, &config); err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("解析配置失败: %v", err)
		return result
	}

	// 构建PostgreSQL连接字符串
	host := config["host"].(string)
	port := int(config["port"].(float64))
	username := config["username"].(string)
	password := config["password"].(string)
	database := config["database"].(string)

	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		host, port, username, password, database)

	// 测试连接
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("PostgreSQL连接失败: %v", err)
		return result
	}
	defer db.Close()

	// 设置连接超时
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// 验证连接
	if err := db.PingContext(ctx); err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("PostgreSQL ping失败: %v", err)
		return result
	}

	// 获取数据库版本信息
	var version string
	if err := db.QueryRowContext(ctx, "SELECT version()").Scan(&version); err == nil {
		result.Details["version"] = version
	}

	// 获取数据库大小
	var dbSize int64
	query := "SELECT pg_database_size($1)"
	if err := db.QueryRowContext(ctx, query, database).Scan(&dbSize); err == nil {
		result.Details["database_size_bytes"] = dbSize
		result.Details["database_size_mb"] = dbSize / 1024 / 1024
	}

	result.Success = true
	result.Message = "PostgreSQL连接成功"
	return result
}

// testHJ212Connection 测试HJ212设备连接
func (s *ConnectionTestService) testHJ212Connection(ctx context.Context, dataSource *models.DataSource, result *ConnectionTestResult) *ConnectionTestResult {
	// 解析配置
	var config map[string]interface{}
	if err := json.Unmarshal(dataSource.ConfigData, &config); err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("解析配置失败: %v", err)
		return result
	}

	host := config["host"].(string)
	port := int(config["port"].(float64))

	// 测试TCP连接
	address := fmt.Sprintf("%s:%d", host, port)

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// 记录设备ID（无论连接是否成功）
	if deviceId, ok := config["device_id"].(string); ok {
		result.Details["device_id"] = deviceId
	}

	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("HJ212设备连接失败: %v", err)
		return result
	}
	defer conn.Close()

	// 记录连接信息
	result.Details["remote_address"] = conn.RemoteAddr().String()
	result.Details["local_address"] = conn.LocalAddr().String()

	result.Success = true
	result.Message = "HJ212设备连接成功"
	return result
}

// testAPIConnection 测试API接口连接
func (s *ConnectionTestService) testAPIConnection(ctx context.Context, dataSource *models.DataSource, result *ConnectionTestResult) *ConnectionTestResult {
	// 解析配置
	var config map[string]interface{}
	if err := json.Unmarshal(dataSource.ConfigData, &config); err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("解析配置失败: %v", err)
		return result
	}

	url := config["url"].(string)
	method := "GET"
	if m, ok := config["method"].(string); ok {
		method = m
	}

	// 创建HTTP客户端
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("创建API请求失败: %v", err)
		return result
	}

	// 添加认证头
	if authType, ok := config["auth_type"].(string); ok && authType != "" {
		switch authType {
		case "bearer":
			if token, ok := config["token"].(string); ok {
				req.Header.Set("Authorization", "Bearer "+token)
			}
		case "basic":
			if username, ok := config["username"].(string); ok {
				if password, ok := config["password"].(string); ok {
					req.SetBasicAuth(username, password)
				}
			}
		case "api_key":
			if apiKey, ok := config["api_key"].(string); ok {
				if headerName, ok := config["api_key_header"].(string); ok {
					req.Header.Set(headerName, apiKey)
				} else {
					req.Header.Set("X-API-Key", apiKey)
				}
			}
		}
	}

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("API请求失败: %v", err)
		return result
	}
	defer resp.Body.Close()

	// 记录响应信息
	result.Details["status_code"] = resp.StatusCode
	result.Details["status"] = resp.Status
	result.Details["content_type"] = resp.Header.Get("Content-Type")
	result.Details["content_length"] = resp.ContentLength

	// 检查响应状态
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		result.Success = true
		result.Message = "API接口连接成功"
	} else {
		result.Success = false
		result.Message = fmt.Sprintf("API接口返回错误状态: %s", resp.Status)
	}

	return result
}