package gateway

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config 网关配置
type Config struct {
	Server      ServerConfig      `yaml:"server"`
	Auth        AuthConfig        `yaml:"auth"`
	RateLimit   RateLimitConfig   `yaml:"rate_limit"`
	LoadBalance LoadBalanceConfig `yaml:"load_balance"`
	Metrics     MetricsConfig     `yaml:"metrics"`
	Logging     LoggingConfig     `yaml:"logging"`
	Redis       RedisConfig       `yaml:"redis"`
	Routes      []RouteConfig     `yaml:"routes"`
	Services    []ServiceConfig   `yaml:"services"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Host            string        `yaml:"host" default:"0.0.0.0"`
	Port            int           `yaml:"port" default:"8080"`
	ReadTimeout     time.Duration `yaml:"read_timeout" default:"30s"`
	WriteTimeout    time.Duration `yaml:"write_timeout" default:"30s"`
	IdleTimeout     time.Duration `yaml:"idle_timeout" default:"120s"`
	MaxHeaderBytes  int           `yaml:"max_header_bytes" default:"1048576"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout" default:"10s"`
	TLS             TLSConfig     `yaml:"tls"`
}

// TLSConfig TLS配置
type TLSConfig struct {
	Enabled  bool   `yaml:"enabled" default:"false"`
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
	CAFile   string `yaml:"ca_file"`
}

// AuthConfig 认证配置
type AuthConfig struct {
	Strategy    string        `yaml:"strategy" default:"none"`
	JWTSecret   string        `yaml:"jwt_secret"`
	TokenExpiry time.Duration `yaml:"token_expiry" default:"24h"`
	Issuer      string        `yaml:"issuer" default:"env-data-platform"`
	Audience    string        `yaml:"audience" default:"api"`
}

// RateLimitConfig 限流配置
type RateLimitConfig struct {
	Enabled   bool          `yaml:"enabled" default:"true"`
	Strategy  string        `yaml:"strategy" default:"token_bucket"`
	Rate      int           `yaml:"rate" default:"100"`
	Burst     int           `yaml:"burst" default:"200"`
	Window    time.Duration `yaml:"window" default:"1m"`
	KeyFunc   string        `yaml:"key_func" default:"ip"`
	Redis     bool          `yaml:"redis" default:"false"`
}

// LoadBalanceConfig 负载均衡配置
type LoadBalanceConfig struct {
	Strategy      string        `yaml:"strategy" default:"round_robin"`
	HealthCheck   HealthCheckConfig `yaml:"health_check"`
	VirtualNodes  int           `yaml:"virtual_nodes" default:"100"`
}

// HealthCheckConfig 健康检查配置
type HealthCheckConfig struct {
	Enabled  bool          `yaml:"enabled" default:"true"`
	Interval time.Duration `yaml:"interval" default:"30s"`
	Timeout  time.Duration `yaml:"timeout" default:"5s"`
	Path     string        `yaml:"path" default:"/health"`
	Method   string        `yaml:"method" default:"GET"`
}

// MetricsConfig 指标配置
type MetricsConfig struct {
	Enabled    bool   `yaml:"enabled" default:"true"`
	Path       string `yaml:"path" default:"/metrics"`
	Prometheus bool   `yaml:"prometheus" default:"true"`
}

// LoggingConfig 日志配置
type LoggingConfig struct {
	Level      string `yaml:"level" default:"info"`
	Format     string `yaml:"format" default:"json"`
	Output     string `yaml:"output" default:"stdout"`
	File       string `yaml:"file"`
	MaxSize    int    `yaml:"max_size" default:"100"`
	MaxBackups int    `yaml:"max_backups" default:"3"`
	MaxAge     int    `yaml:"max_age" default:"28"`
	Compress   bool   `yaml:"compress" default:"true"`
}

// RedisConfig Redis配置
type RedisConfig struct {
	Host     string `yaml:"host" default:"localhost"`
	Port     int    `yaml:"port" default:"6379"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db" default:"0"`
	PoolSize int    `yaml:"pool_size" default:"10"`
}

// RouteConfig 路由配置
type RouteConfig struct {
	ID          string            `yaml:"id"`
	Path        string            `yaml:"path"`
	Method      string            `yaml:"method"`
	Target      string            `yaml:"target"`
	Service     string            `yaml:"service"`
	StripPrefix bool              `yaml:"strip_prefix" default:"false"`
	Headers     map[string]string `yaml:"headers"`
	Timeout     time.Duration     `yaml:"timeout" default:"30s"`
	Retries     int               `yaml:"retries" default:"3"`
	Auth        *RouteAuthConfig  `yaml:"auth"`
	RateLimit   *RouteRateLimitConfig `yaml:"rate_limit"`
}

// RouteAuthConfig 路由认证配置
type RouteAuthConfig struct {
	Required bool     `yaml:"required" default:"false"`
	Scopes   []string `yaml:"scopes"`
	Strategy string   `yaml:"strategy"`
}

// RouteRateLimitConfig 路由限流配置
type RouteRateLimitConfig struct {
	Enabled  bool          `yaml:"enabled" default:"false"`
	Rate     int           `yaml:"rate"`
	Burst    int           `yaml:"burst"`
	Window   time.Duration `yaml:"window"`
	KeyFunc  string        `yaml:"key_func"`
}

// ServiceConfig 服务配置
type ServiceConfig struct {
	ID          string         `yaml:"id"`
	Name        string         `yaml:"name"`
	Targets     []TargetConfig `yaml:"targets"`
	Strategy    string         `yaml:"strategy" default:"round_robin"`
	HealthCheck HealthCheckConfig `yaml:"health_check"`
}

// TargetConfig 目标配置
type TargetConfig struct {
	ID       string            `yaml:"id"`
	URL      string            `yaml:"url"`
	Weight   int               `yaml:"weight" default:"1"`
	Metadata map[string]string `yaml:"metadata"`
}

// LoadConfig 加载配置文件
func LoadConfig(configPath string) (*Config, error) {
	// 设置默认配置
	config := &Config{
		Server: ServerConfig{
			Host:            "0.0.0.0",
			Port:            8080,
			ReadTimeout:     30 * time.Second,
			WriteTimeout:    30 * time.Second,
			IdleTimeout:     120 * time.Second,
			MaxHeaderBytes:  1 << 20, // 1MB
			ShutdownTimeout: 10 * time.Second,
		},
		Auth: AuthConfig{
			Strategy:    "none",
			TokenExpiry: 24 * time.Hour,
			Issuer:      "env-data-platform",
			Audience:    "api",
		},
		RateLimit: RateLimitConfig{
			Enabled:  true,
			Strategy: "token_bucket",
			Rate:     100,
			Burst:    200,
			Window:   time.Minute,
			KeyFunc:  "ip",
		},
		LoadBalance: LoadBalanceConfig{
			Strategy: "round_robin",
			HealthCheck: HealthCheckConfig{
				Enabled:  true,
				Interval: 30 * time.Second,
				Timeout:  5 * time.Second,
				Path:     "/health",
				Method:   "GET",
			},
			VirtualNodes: 100,
		},
		Metrics: MetricsConfig{
			Enabled:    true,
			Path:       "/metrics",
			Prometheus: true,
		},
		Logging: LoggingConfig{
			Level:      "info",
			Format:     "json",
			Output:     "stdout",
			MaxSize:    100,
			MaxBackups: 3,
			MaxAge:     28,
			Compress:   true,
		},
		Redis: RedisConfig{
			Host:     "localhost",
			Port:     6379,
			DB:       0,
			PoolSize: 10,
		},
		Routes:   []RouteConfig{},
		Services: []ServiceConfig{},
	}

	// 如果配置文件存在，加载配置
	if configPath != "" {
		if _, err := os.Stat(configPath); err == nil {
			data, err := os.ReadFile(configPath)
			if err != nil {
				return nil, fmt.Errorf("failed to read config file: %w", err)
			}

			if err := yaml.Unmarshal(data, config); err != nil {
				return nil, fmt.Errorf("failed to parse config file: %w", err)
			}
		}
	}

	// 从环境变量覆盖配置
	config.overrideFromEnv()

	// 验证配置
	if err := config.validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return config, nil
}

// overrideFromEnv 从环境变量覆盖配置
func (c *Config) overrideFromEnv() {
	if host := os.Getenv("GATEWAY_HOST"); host != "" {
		c.Server.Host = host
	}
	if port := os.Getenv("GATEWAY_PORT"); port != "" {
		if p, err := parsePort(port); err == nil {
			c.Server.Port = p
		}
	}
	if secret := os.Getenv("GATEWAY_JWT_SECRET"); secret != "" {
		c.Auth.JWTSecret = secret
	}
	if redisHost := os.Getenv("REDIS_HOST"); redisHost != "" {
		c.Redis.Host = redisHost
	}
	if redisPort := os.Getenv("REDIS_PORT"); redisPort != "" {
		if p, err := parsePort(redisPort); err == nil {
			c.Redis.Port = p
		}
	}
	if redisPassword := os.Getenv("REDIS_PASSWORD"); redisPassword != "" {
		c.Redis.Password = redisPassword
	}
}

// validate 验证配置
func (c *Config) validate() error {
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}

	if c.Auth.Strategy == "jwt" && c.Auth.JWTSecret == "" {
		return fmt.Errorf("JWT secret is required when auth strategy is jwt")
	}

	if c.Server.TLS.Enabled {
		if c.Server.TLS.CertFile == "" || c.Server.TLS.KeyFile == "" {
			return fmt.Errorf("TLS cert file and key file are required when TLS is enabled")
		}
	}

	// 验证路由配置
	for i, route := range c.Routes {
		if route.Path == "" {
			return fmt.Errorf("route[%d]: path is required", i)
		}
		if route.Method == "" {
			return fmt.Errorf("route[%d]: method is required", i)
		}
		if route.Target == "" && route.Service == "" {
			return fmt.Errorf("route[%d]: either target or service is required", i)
		}
	}

	// 验证服务配置
	for i, service := range c.Services {
		if service.ID == "" {
			return fmt.Errorf("service[%d]: id is required", i)
		}
		if len(service.Targets) == 0 {
			return fmt.Errorf("service[%d]: at least one target is required", i)
		}
		for j, target := range service.Targets {
			if target.URL == "" {
				return fmt.Errorf("service[%d].target[%d]: url is required", i, j)
			}
		}
	}

	return nil
}

// SaveConfig 保存配置到文件
func (c *Config) SaveConfig(configPath string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// GetServerAddress 获取服务器地址
func (c *Config) GetServerAddress() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}

// GetRedisAddress 获取Redis地址
func (c *Config) GetRedisAddress() string {
	return fmt.Sprintf("%s:%d", c.Redis.Host, c.Redis.Port)
}

// IsProduction 是否生产环境
func (c *Config) IsProduction() bool {
	return os.Getenv("ENVIRONMENT") == "production"
}

// IsDevelopment 是否开发环境
func (c *Config) IsDevelopment() bool {
	env := os.Getenv("ENVIRONMENT")
	return env == "" || env == "development"
}

// 辅助函数

func parsePort(s string) (int, error) {
	var port int
	if _, err := fmt.Sscanf(s, "%d", &port); err != nil {
		return 0, err
	}
	if port <= 0 || port > 65535 {
		return 0, fmt.Errorf("invalid port: %d", port)
	}
	return port, nil
}

// GetDefaultConfig 获取默认配置
func GetDefaultConfig() *Config {
	config, _ := LoadConfig("")
	return config
}

// ExampleConfig 示例配置
func ExampleConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host: "0.0.0.0",
			Port: 8080,
		},
		Auth: AuthConfig{
			Strategy:    "jwt",
			JWTSecret:   "your-secret-key",
			TokenExpiry: 24 * time.Hour,
		},
		RateLimit: RateLimitConfig{
			Enabled:  true,
			Strategy: "token_bucket",
			Rate:     100,
			Burst:    200,
		},
		Routes: []RouteConfig{
			{
				ID:     "data-api",
				Path:   "/api/v1/data/*",
				Method: "GET",
				Target: "http://localhost:8081",
				Auth: &RouteAuthConfig{
					Required: true,
					Scopes:   []string{"data:read"},
				},
			},
		},
		Services: []ServiceConfig{
			{
				ID:   "data-service",
				Name: "Data Processing Service",
				Targets: []TargetConfig{
					{
						ID:     "data-1",
						URL:    "http://localhost:8081",
						Weight: 1,
					},
					{
						ID:     "data-2",
						URL:    "http://localhost:8082",
						Weight: 1,
					},
				},
			},
		},
	}
}