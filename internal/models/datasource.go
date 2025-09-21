package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

// DataSource 数据源模型
type DataSource struct {
	BaseModelWithOperator
	Name         string          `gorm:"not null;size:100;comment:数据源名称" json:"name"`
	Type         string          `gorm:"not null;size:20;comment:数据源类型" json:"type"`
	Description  string          `gorm:"size:500;comment:描述" json:"description"`
	DeviceID     string          `gorm:"size:50;comment:设备ID" json:"device_id"`
	Config       string          `gorm:"type:text;comment:配置信息JSON" json:"-"`
	ConfigData   json.RawMessage `gorm:"-" json:"config"`
	Status       string          `gorm:"size:20;default:active;comment:状态" json:"status"`
	IsConnected  bool            `gorm:"default:false;comment:是否已连接" json:"is_connected"`
	LastTestAt   *time.Time      `gorm:"comment:最后测试时间" json:"last_test_at"`
	LastSyncAt   *time.Time      `gorm:"comment:最后同步时间" json:"last_sync_at"`
	LastActiveAt *time.Time      `gorm:"comment:最后活跃时间" json:"last_active_at"`
	Tags         string          `gorm:"size:500;comment:标签，逗号分隔" json:"tags"`
	Priority     int             `gorm:"default:0;comment:优先级" json:"priority"`
	ErrorCount   int             `gorm:"default:0;comment:错误次数" json:"error_count"`
	LastError    string          `gorm:"type:text;comment:最后错误信息" json:"last_error"`

	// 关联
	Creator      *User           `gorm:"foreignKey:CreatedBy" json:"creator,omitempty"`
	Updater      *User           `gorm:"foreignKey:UpdatedBy" json:"updater,omitempty"`
	DataTables   []DataTable     `gorm:"foreignKey:DataSourceID" json:"data_tables,omitempty"`
	ETLJobs      []ETLJob        `gorm:"foreignKey:SourceID" json:"etl_jobs,omitempty"`
	QualityRules []QualityRule   `gorm:"foreignKey:DataSourceID" json:"quality_rules,omitempty"`
}

// TableName 指定表名
func (DataSource) TableName() string {
	return GetTableName("data_sources")
}

// DataTable 数据表模型
type DataTable struct {
	BaseModel
	DataSourceID uint   `gorm:"not null;comment:数据源ID" json:"data_source_id"`
	Name         string `gorm:"not null;size:100;comment:表名" json:"table_name"`
	TableComment string `gorm:"size:500;comment:表注释" json:"table_comment"`
	Schema       string `gorm:"size:100;comment:数据库schema" json:"schema"`
	RowCount     int64  `gorm:"default:0;comment:行数" json:"row_count"`
	SizeBytes    int64  `gorm:"default:0;comment:大小(字节)" json:"size_bytes"`
	LastSyncAt   *time.Time `gorm:"comment:最后同步时间" json:"last_sync_at"`
	IsActive     bool   `gorm:"default:true;comment:是否活跃" json:"is_active"`

	// 关联
	DataSource *DataSource   `gorm:"foreignKey:DataSourceID" json:"data_source,omitempty"`
	Columns    []DataColumn  `gorm:"foreignKey:TableID" json:"columns,omitempty"`
}

// TableName 指定表名
func (DataTable) TableName() string {
	return GetTableName("data_tables")
}

// DataColumn 数据列模型
type DataColumn struct {
	BaseModel
	TableID      uint   `gorm:"not null;comment:表ID" json:"table_id"`
	ColumnName   string `gorm:"not null;size:100;comment:列名" json:"column_name"`
	DataType     string `gorm:"not null;size:50;comment:数据类型" json:"data_type"`
	Length       int    `gorm:"comment:长度" json:"length"`
	Precision    int    `gorm:"comment:精度" json:"precision"`
	Scale        int    `gorm:"comment:小数位数" json:"scale"`
	IsNullable   bool   `gorm:"comment:是否可空" json:"is_nullable"`
	IsPrimaryKey bool   `gorm:"comment:是否主键" json:"is_primary_key"`
	DefaultValue string `gorm:"size:255;comment:默认值" json:"default_value"`
	Comment      string `gorm:"size:500;comment:列注释" json:"comment"`
	Position     int    `gorm:"comment:列位置" json:"position"`

	// 关联
	Table *DataTable `gorm:"foreignKey:TableID" json:"table,omitempty"`
}

// TableName 指定表名
func (DataColumn) TableName() string {
	return GetTableName("data_columns")
}

// HJ212Data HJ212协议数据模型
type HJ212Data struct {
	BaseModel
	DeviceID     string    `gorm:"not null;size:50;comment:设备ID" json:"device_id"`
	CommandCode  string    `gorm:"not null;size:10;comment:命令编码" json:"command_code"`
	DataType     string    `gorm:"size:50;comment:数据类型" json:"data_type"`
	RawData      string    `gorm:"type:text;comment:原始数据" json:"raw_data"`
	ParsedData   JSONMap   `gorm:"type:json;comment:解析后数据" json:"parsed_data"`
	ReceivedFrom string    `gorm:"size:100;comment:接收来源IP" json:"received_from"`
	ReceivedAt   time.Time `gorm:"not null;comment:接收时间" json:"received_at"`
	QualityLevel string    `gorm:"size:20;comment:数据质量等级" json:"quality_level"`
	IsValid      bool      `gorm:"default:true;comment:是否有效" json:"is_valid"`
	ErrorMessage string    `gorm:"type:text;comment:错误信息" json:"error_message"`

	// 索引字段
	CreatedDate string `gorm:"size:10;index;comment:创建日期YYYY-MM-DD" json:"created_date"`
	CreatedHour int    `gorm:"index;comment:创建小时0-23" json:"created_hour"`
}

// TableName 指定表名
func (HJ212Data) TableName() string {
	return GetTableName("hj212_data")
}

// FileUploadRecord 文件上传记录模型
type FileUploadRecord struct {
	BaseModelWithOperator
	FileName     string    `gorm:"not null;size:255;comment:文件名" json:"file_name"`
	FileSize     int64     `gorm:"not null;comment:文件大小" json:"file_size"`
	FileType     string    `gorm:"not null;size:50;comment:文件类型" json:"file_type"`
	FilePath     string    `gorm:"not null;size:500;comment:文件路径" json:"file_path"`
	OriginalName string    `gorm:"not null;size:255;comment:原始文件名" json:"original_name"`
	Status       string    `gorm:"not null;size:20;comment:处理状态" json:"status"`
	ProcessedAt  *time.Time `gorm:"comment:处理完成时间" json:"processed_at"`
	ErrorMessage string    `gorm:"type:text;comment:错误信息" json:"error_message"`
	RowCount     int       `gorm:"default:0;comment:数据行数" json:"row_count"`
	ColumnCount  int       `gorm:"default:0;comment:列数" json:"column_count"`
	Encoding     string    `gorm:"size:20;comment:文件编码" json:"encoding"`
	Delimiter    string    `gorm:"size:10;comment:分隔符" json:"delimiter"`

	// 关联
	Creator *User `gorm:"foreignKey:CreatedBy" json:"creator,omitempty"`
}

// TableName 指定表名
func (FileUploadRecord) TableName() string {
	return GetTableName("file_upload_records")
}

// DataSourceConfig 数据源配置结构体
type DataSourceConfig struct {
	// 数据库配置
	Host     string `json:"host,omitempty"`
	Port     int    `json:"port,omitempty"`
	Database string `json:"database,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`

	// HJ212配置
	Protocol   string `json:"protocol,omitempty"`   // TCP/UDP
	ListenPort int    `json:"listen_port,omitempty"`

	// API配置
	URL     string            `json:"url,omitempty"`
	Method  string            `json:"method,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`

	// 文件配置
	FilePath   string `json:"file_path,omitempty"`
	FileFormat string `json:"file_format,omitempty"`

	// 通用配置
	Timeout       int               `json:"timeout,omitempty"`
	RetryTimes    int               `json:"retry_times,omitempty"`
	SyncInterval  int               `json:"sync_interval,omitempty"`
	CustomConfig  map[string]interface{} `json:"custom_config,omitempty"`
}

// 数据源请求结构
type DataSourceRequest struct {
	Name        string            `json:"name" binding:"required,min=1,max=100"`
	Type        string            `json:"type" binding:"required,oneof=hj212 database file api webhook"`
	Description string            `json:"description"`
	Config      DataSourceConfig  `json:"config" binding:"required"`
	Tags        []string          `json:"tags"`
	Priority    int               `json:"priority"`
	Status      int               `json:"status"`
	Remark      string            `json:"remark"`
}

// 数据源响应结构
type DataSourceResponse struct {
	ID          uint             `json:"id"`
	Name        string           `json:"name"`
	Type        string           `json:"type"`
	Description string           `json:"description"`
	Config      DataSourceConfig `json:"config"`
	Status      int              `json:"status"`
	IsConnected bool             `json:"is_connected"`
	LastTestAt  *time.Time       `json:"last_test_at"`
	LastSyncAt  *time.Time       `json:"last_sync_at"`
	Tags        []string         `json:"tags"`
	Priority    int              `json:"priority"`
	ErrorCount  int              `json:"error_count"`
	LastError   string           `json:"last_error"`
	Creator     *UserResponse    `json:"creator"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
}

// 数据源测试请求结构
type DataSourceTestRequest struct {
	Type   string           `json:"type" binding:"required"`
	Config DataSourceConfig `json:"config" binding:"required"`
}

// 数据源测试响应结构
type DataSourceTestResponse struct {
	Success     bool   `json:"success"`
	Message     string `json:"message"`
	Duration    int64  `json:"duration"` // 毫秒
	Details     interface{} `json:"details,omitempty"`
}

// HJ212数据请求结构
type HJ212DataRequest struct {
	StationID   string    `json:"station_id" form:"station_id"`
	FactorCode  string    `json:"factor_code" form:"factor_code"`
	StartTime   time.Time `json:"start_time" form:"start_time"`
	EndTime     time.Time `json:"end_time" form:"end_time"`
	DataFlag    string    `json:"data_flag" form:"data_flag"`
	QualityLevel string   `json:"quality_level" form:"quality_level"`
	IsValid     *bool     `json:"is_valid" form:"is_valid"`
}

// HJ212数据统计结构
type HJ212DataStats struct {
	TotalCount    int64 `json:"total_count"`
	ValidCount    int64 `json:"valid_count"`
	InvalidCount  int64 `json:"invalid_count"`
	StationCount  int64 `json:"station_count"`
	FactorCount   int64 `json:"factor_count"`
	TodayCount    int64 `json:"today_count"`
	YesterdayCount int64 `json:"yesterday_count"`
}

// 方法：设置配置数据
func (ds *DataSource) SetConfig(config DataSourceConfig) error {
	configBytes, err := json.Marshal(config)
	if err != nil {
		return err
	}
	ds.Config = string(configBytes)
	ds.ConfigData = configBytes
	return nil
}

// 方法：获取配置数据
func (ds *DataSource) GetConfig() (*DataSourceConfig, error) {
	var config DataSourceConfig
	if ds.Config != "" {
		err := json.Unmarshal([]byte(ds.Config), &config)
		if err != nil {
			return nil, err
		}
	}
	return &config, nil
}

// 方法：检查数据源是否健康
func (ds *DataSource) IsHealthy() bool {
	return ds.Status == "active" && ds.IsConnected && ds.ErrorCount < 5
}

// JSONMap 自定义JSON类型，用于处理map[string]interface{}的JSON序列化
type JSONMap map[string]interface{}

// Value 实现driver.Valuer接口
func (j JSONMap) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// Scan 实现sql.Scanner接口
func (j *JSONMap) Scan(value interface{}) error {
	if value == nil {
		*j = make(JSONMap)
		return nil
	}

	switch v := value.(type) {
	case []byte:
		return json.Unmarshal(v, j)
	case string:
		return json.Unmarshal([]byte(v), j)
	default:
		return json.Unmarshal([]byte("{}"), j)
	}
}

// HJ212AlarmData HJ212协议报警数据模型
type HJ212AlarmData struct {
	BaseModel
	DeviceID     string    `gorm:"not null;size:50;comment:设备ID" json:"device_id"`
	AlarmType    string    `gorm:"size:50;comment:报警类型" json:"alarm_type"`
	AlarmLevel   string    `gorm:"size:20;comment:报警级别" json:"alarm_level"`
	AlarmDesc    string    `gorm:"type:text;comment:报警描述" json:"alarm_desc"`
	RawData      string    `gorm:"type:text;comment:原始数据" json:"raw_data"`
	ReceivedFrom string    `gorm:"size:100;comment:接收来源IP" json:"received_from"`
	ReceivedAt   time.Time `gorm:"not null;comment:接收时间" json:"received_at"`
	Status       string    `gorm:"size:20;comment:处理状态" json:"status"`
	ProcessedAt  *time.Time `gorm:"comment:处理时间" json:"processed_at"`
}

// TableName 指定表名
func (HJ212AlarmData) TableName() string {
	return GetTableName("hj212_alarm_data")
}