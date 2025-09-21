package config

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/viper"
)

// Config 应用配置结构
type Config struct {
	App      AppConfig      `mapstructure:"app"`
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Redis    RedisConfig    `mapstructure:"redis"`
	JWT      JWTConfig      `mapstructure:"jwt"`
	Log      LogConfig      `mapstructure:"log"`
	Monitor  MonitorConfig  `mapstructure:"monitor"`
	ETL      ETLConfig      `mapstructure:"etl"`
	HJ212    HJ212Config    `mapstructure:"hj212"`
}

// AppConfig 应用基础配置
type AppConfig struct {
	Name        string `mapstructure:"name"`
	Version     string `mapstructure:"version"`
	Environment string `mapstructure:"environment"`
	Debug       bool   `mapstructure:"debug"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	ReadTimeout  int    `mapstructure:"read_timeout"`
	WriteTimeout int    `mapstructure:"write_timeout"`
	TLS          struct {
		Enabled  bool   `mapstructure:"enabled"`
		CertFile string `mapstructure:"cert_file"`
		KeyFile  string `mapstructure:"key_file"`
	} `mapstructure:"tls"`
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Driver          string `mapstructure:"driver"`
	Host            string `mapstructure:"host"`
	Port            int    `mapstructure:"port"`
	Name            string `mapstructure:"database"`
	Username        string `mapstructure:"username"`
	Password        string `mapstructure:"password"`
	Charset         string `mapstructure:"charset"`
	ParseTime       bool   `mapstructure:"parse_time"`
	Loc             string `mapstructure:"loc"`
	MaxIdleConns    int    `mapstructure:"max_idle_conns"`
	MaxOpenConns    int    `mapstructure:"max_open_conns"`
	ConnMaxLifetime string `mapstructure:"conn_max_lifetime"`
}

// RedisConfig Redis配置
type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	Database int    `mapstructure:"database"`
	PoolSize int    `mapstructure:"pool_size"`
}

// JWTConfig JWT配置
type JWTConfig struct {
	Secret string        `mapstructure:"secret"`
	Expire time.Duration `mapstructure:"expire"`
	Issuer string        `mapstructure:"issuer"`
}

// LogConfig 日志配置
type LogConfig struct {
	Level      string `mapstructure:"level"`
	Format     string `mapstructure:"format"`
	Output     string `mapstructure:"output"`
	Filename   string `mapstructure:"filename"`
	MaxSize    int    `mapstructure:"max_size"`
	MaxBackups int    `mapstructure:"max_backups"`
	MaxAge     int    `mapstructure:"max_age"`
	Compress   bool   `mapstructure:"compress"`
}

// MonitorConfig 监控配置
type MonitorConfig struct {
	Enabled    bool   `mapstructure:"enabled"`
	Host       string `mapstructure:"host"`
	Port       int    `mapstructure:"port"`
	Path       string `mapstructure:"path"`
	Prometheus struct {
		Enabled bool   `mapstructure:"enabled"`
		Path    string `mapstructure:"path"`
	} `mapstructure:"prometheus"`
}

// ETLConfig ETL配置
type ETLConfig struct {
	HopServer struct {
		Host     string `mapstructure:"host"`
		Port     int    `mapstructure:"port"`
		Username string `mapstructure:"username"`
		Password string `mapstructure:"password"`
	} `mapstructure:"hop_server"`
	Pipeline struct {
		BasePath    string `mapstructure:"base_path"`
		TempPath    string `mapstructure:"temp_path"`
		MaxParallel int    `mapstructure:"max_parallel"`
	} `mapstructure:"pipeline"`
}

// HJ212Config HJ212协议配置
type HJ212Config struct {
	Enabled        bool          `mapstructure:"enabled"`
	TCPPort        int           `mapstructure:"tcp_port"`
	BufferSize     int           `mapstructure:"buffer_size"`
	Timeout        time.Duration `mapstructure:"timeout"`
	MaxConnections int           `mapstructure:"max_connections"`
}

// GlobalConfig 全局配置实例
var GlobalConfig *Config

// Load 加载配置文件
func Load(configPath string) (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	if configPath != "" {
		viper.SetConfigFile(configPath)
	} else {
		viper.AddConfigPath("./config")
		viper.AddConfigPath(".")
	}

	// 设置环境变量前缀
	viper.SetEnvPrefix("ENV_DATA")
	viper.AutomaticEnv()

	// 设置默认值
	setDefaults()

	// 读取配置文件
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			fmt.Println("警告: 配置文件未找到，使用默认配置")
		} else {
			return nil, fmt.Errorf("读取配置文件失败: %w", err)
		}
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}

	// 从环境变量覆盖敏感配置
	overrideFromEnv(&config)

	GlobalConfig = &config
	return &config, nil
}

// setDefaults 设置默认配置值
func setDefaults() {
	// 应用配置默认值
	viper.SetDefault("app.name", "env-data-platform")
	viper.SetDefault("app.version", "1.0.0")
	viper.SetDefault("app.environment", "development")
	viper.SetDefault("app.debug", true)

	// 服务器配置默认值
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.read_timeout", 30)
	viper.SetDefault("server.write_timeout", 30)

	// 数据库配置默认值
	viper.SetDefault("database.driver", "mysql")
	viper.SetDefault("database.host", "localhost")
	viper.SetDefault("database.port", 3306)
	viper.SetDefault("database.database", "env_data_platform")
	viper.SetDefault("database.username", "root")
	viper.SetDefault("database.charset", "utf8mb4")
	viper.SetDefault("database.timezone", "Asia/Shanghai")
	viper.SetDefault("database.pool.max_idle", 10)
	viper.SetDefault("database.pool.max_open", 100)
	viper.SetDefault("database.pool.max_lifetime", 3600)

	// Redis配置默认值
	viper.SetDefault("redis.host", "localhost")
	viper.SetDefault("redis.port", 6379)
	viper.SetDefault("redis.database", 0)
	viper.SetDefault("redis.pool_size", 10)

	// JWT配置默认值
	viper.SetDefault("jwt.secret", "env-data-platform-secret-key")
	viper.SetDefault("jwt.expire_time", 86400) // 24小时
	viper.SetDefault("jwt.issuer", "env-data-platform")

	// 日志配置默认值
	viper.SetDefault("log.level", "info")
	viper.SetDefault("log.format", "json")
	viper.SetDefault("log.output", "stdout")
	viper.SetDefault("log.max_size", 100)
	viper.SetDefault("log.max_backups", 3)
	viper.SetDefault("log.max_age", 28)
	viper.SetDefault("log.compress", true)

	// 监控配置默认值
	viper.SetDefault("monitor.enabled", true)
	viper.SetDefault("monitor.host", "0.0.0.0")
	viper.SetDefault("monitor.port", 9090)
	viper.SetDefault("monitor.path", "/metrics")
	viper.SetDefault("monitor.prometheus.enabled", true)
	viper.SetDefault("monitor.prometheus.path", "/metrics")

	// ETL配置默认值
	viper.SetDefault("etl.hop_server.host", "localhost")
	viper.SetDefault("etl.hop_server.port", 8181)
	viper.SetDefault("etl.hop_server.username", "admin")
	viper.SetDefault("etl.pipeline.base_path", "./pipelines")
	viper.SetDefault("etl.pipeline.temp_path", "./temp")
	viper.SetDefault("etl.pipeline.max_parallel", 5)
}

// overrideFromEnv 从环境变量覆盖敏感配置
func overrideFromEnv(config *Config) {
	if dbPassword := os.Getenv("DB_PASSWORD"); dbPassword != "" {
		config.Database.Password = dbPassword
	}
	if redisPassword := os.Getenv("REDIS_PASSWORD"); redisPassword != "" {
		config.Redis.Password = redisPassword
	}
	if jwtSecret := os.Getenv("JWT_SECRET"); jwtSecret != "" {
		config.JWT.Secret = jwtSecret
	}
	if hopPassword := os.Getenv("HOP_PASSWORD"); hopPassword != "" {
		config.ETL.HopServer.Password = hopPassword
	}
}

// GetDSN 获取数据库连接字符串
func (c *DatabaseConfig) GetDSN() string {
	parseTime := "True"
	if !c.ParseTime {
		parseTime = "False"
	}

	var userPass string
	if c.Password != "" {
		userPass = fmt.Sprintf("%s:%s", c.Username, c.Password)
	} else {
		userPass = c.Username
	}

	return fmt.Sprintf("%s@tcp(%s:%d)/%s?charset=%s&parseTime=%s&loc=%s",
		userPass,
		c.Host,
		c.Port,
		c.Name,
		c.Charset,
		parseTime,
		c.Loc,
	)
}

// GetRedisAddr 获取Redis连接地址
func (c *RedisConfig) GetRedisAddr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// GetServerAddr 获取服务器地址
func (c *ServerConfig) GetServerAddr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// IsProduction 判断是否为生产环境
func (c *AppConfig) IsProduction() bool {
	return c.Environment == "production"
}

// IsDevelopment 判断是否为开发环境
func (c *AppConfig) IsDevelopment() bool {
	return c.Environment == "development"
}