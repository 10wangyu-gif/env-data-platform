package models

import (
	"encoding/json"
	"time"
)

// ETLJob ETL作业模型
type ETLJob struct {
	BaseModelWithOperator
	Name         string          `gorm:"not null;size:100;comment:作业名称" json:"name"`
	Description  string          `gorm:"size:500;comment:作业描述" json:"description"`
	SourceID     uint            `gorm:"not null;comment:数据源ID" json:"source_id"`
	TargetID     uint            `gorm:"comment:目标数据源ID" json:"target_id"`
	PipelineXML  string          `gorm:"type:longtext;comment:Hop Pipeline XML" json:"-"`
	ConfigData   string          `gorm:"type:text;comment:配置数据JSON" json:"-"`
	Config       json.RawMessage `gorm:"-" json:"config"`
	CronExpr     string          `gorm:"size:100;comment:定时表达式" json:"cron_expr"`
	Status       string          `gorm:"not null;size:20;comment:作业状态" json:"status"`
	IsEnabled    bool            `gorm:"default:true;comment:是否启用" json:"is_enabled"`
	Priority     int             `gorm:"default:0;comment:优先级" json:"priority"`
	MaxRetries   int             `gorm:"default:3;comment:最大重试次数" json:"max_retries"`
	Timeout      int             `gorm:"default:3600;comment:超时时间(秒)" json:"timeout"`
	LastRunAt    *time.Time      `gorm:"comment:最后运行时间" json:"last_run_at"`
	NextRunAt    *time.Time      `gorm:"comment:下次运行时间" json:"next_run_at"`
	RunCount     int             `gorm:"default:0;comment:运行次数" json:"run_count"`
	SuccessCount int             `gorm:"default:0;comment:成功次数" json:"success_count"`
	FailureCount int             `gorm:"default:0;comment:失败次数" json:"failure_count"`

	// 关联
	Source      *DataSource      `gorm:"foreignKey:SourceID" json:"source,omitempty"`
	Target      *DataSource      `gorm:"foreignKey:TargetID" json:"target,omitempty"`
	Creator     *User            `gorm:"foreignKey:CreatedBy" json:"creator,omitempty"`
	Executions  []ETLExecution   `gorm:"foreignKey:JobID" json:"executions,omitempty"`
	QualityRules []QualityRule   `gorm:"foreignKey:ETLJobID" json:"quality_rules,omitempty"`
}

// TableName 指定表名
func (ETLJob) TableName() string {
	return GetTableName("etl_jobs")
}

// ETLExecution ETL执行记录模型
type ETLExecution struct {
	BaseModel
	JobID        uint       `gorm:"not null;comment:作业ID" json:"job_id"`
	ExecutionID  string     `gorm:"not null;size:100;comment:执行ID" json:"execution_id"`
	Status       string     `gorm:"not null;size:20;comment:执行状态" json:"status"`
	StartTime    time.Time  `gorm:"comment:开始时间" json:"start_time"`
	EndTime      *time.Time `gorm:"comment:结束时间" json:"end_time"`
	Duration     int64      `gorm:"comment:执行时长(毫秒)" json:"duration"`
	InputRows    int64      `gorm:"default:0;comment:输入行数" json:"input_rows"`
	OutputRows   int64      `gorm:"default:0;comment:输出行数" json:"output_rows"`
	ErrorRows    int64      `gorm:"default:0;comment:错误行数" json:"error_rows"`
	SkippedRows  int64      `gorm:"default:0;comment:跳过行数" json:"skipped_rows"`
	ErrorMessage string     `gorm:"type:text;comment:错误信息" json:"error_message"`
	LogContent   string     `gorm:"type:longtext;comment:日志内容" json:"log_content"`
	TriggerType  string     `gorm:"size:20;comment:触发类型 manual/schedule/api" json:"trigger_type"`
	TriggerBy    uint       `gorm:"comment:触发人ID" json:"trigger_by"`

	// 关联
	Job     *ETLJob `gorm:"foreignKey:JobID" json:"job,omitempty"`
	Trigger *User   `gorm:"foreignKey:TriggerBy" json:"trigger,omitempty"`
	// Steps   []ETLExecutionStep `gorm:"foreignKey:ExecutionID" json:"steps,omitempty"` // 暂时注释
}

// TableName 指定表名
func (ETLExecution) TableName() string {
	return GetTableName("etl_executions")
}

// ETLExecutionStep ETL执行步骤模型 (暂时完全注释掉)
/*
type ETLExecutionStep struct {
	BaseModel
	ExecutionID  uint       `gorm:"not null;comment:执行记录ID" json:"execution_id"`
	StepName     string     `gorm:"not null;size:100;comment:步骤名称" json:"step_name"`
	StepType     string     `gorm:"not null;size:50;comment:步骤类型" json:"step_type"`
	Status       string     `gorm:"not null;size:20;comment:步骤状态" json:"status"`
	StartTime    time.Time  `gorm:"comment:开始时间" json:"start_time"`
	EndTime      *time.Time `gorm:"comment:结束时间" json:"end_time"`
	Duration     int64      `gorm:"comment:执行时长(毫秒)" json:"duration"`
	InputRows    int64      `gorm:"default:0;comment:输入行数" json:"input_rows"`
	OutputRows   int64      `gorm:"default:0;comment:输出行数" json:"output_rows"`
	ErrorRows    int64      `gorm:"default:0;comment:错误行数" json:"error_rows"`
	ErrorMessage string     `gorm:"type:text;comment:错误信息" json:"error_message"`
	StepOrder    int        `gorm:"comment:步骤顺序" json:"step_order"`

	// 关联 (暂时移除外键约束)
	// Execution *ETLExecution `gorm:"foreignKey:ExecutionID" json:"execution,omitempty"`
}

// TableName 指定表名
func (ETLExecutionStep) TableName() string {
	return GetTableName("etl_execution_steps")
}
*/

// ETLTemplate ETL模板模型
type ETLTemplate struct {
	BaseModelWithOperator
	Name         string `gorm:"not null;size:100;comment:模板名称" json:"name"`
	Description  string `gorm:"size:500;comment:模板描述" json:"description"`
	Category     string `gorm:"not null;size:50;comment:模板分类" json:"category"`
	TemplateXML  string `gorm:"type:longtext;comment:模板XML" json:"-"`
	Preview      string `gorm:"type:text;comment:预览图片URL" json:"preview"`
	Version      string `gorm:"size:20;comment:版本号" json:"version"`
	IsPublic     bool   `gorm:"default:false;comment:是否公开" json:"is_public"`
	UseCount     int    `gorm:"default:0;comment:使用次数" json:"use_count"`
	Rating       float32 `gorm:"default:0;comment:评分" json:"rating"`
	Tags         string `gorm:"size:500;comment:标签" json:"tags"`

	// 关联
	Creator *User `gorm:"foreignKey:CreatedBy" json:"creator,omitempty"`
}

// TableName 指定表名
func (ETLTemplate) TableName() string {
	return GetTableName("etl_templates")
}

// QualityRule 数据质量规则模型
type QualityRule struct {
	BaseModelWithOperator
	Name         string          `gorm:"not null;size:100;comment:规则名称" json:"name"`
	Description  string          `gorm:"size:500;comment:规则描述" json:"description"`
	Type         string          `gorm:"not null;size:20;comment:规则类型" json:"type"`
	DataSourceID uint            `gorm:"comment:数据源ID" json:"data_source_id"`
	ETLJobID     uint            `gorm:"comment:ETL作业ID" json:"etl_job_id"`
	TargetTable  string          `gorm:"size:100;comment:目标表名" json:"table_name"`
	ColumnName   string          `gorm:"size:100;comment:列名" json:"column_name"`
	RuleConfig   string          `gorm:"type:text;comment:规则配置JSON" json:"-"`
	Config       json.RawMessage `gorm:"-" json:"config"`
	Threshold    float64         `gorm:"comment:阈值" json:"threshold"`
	IsEnabled    bool            `gorm:"default:true;comment:是否启用" json:"is_enabled"`
	Priority     int             `gorm:"default:0;comment:优先级" json:"priority"`
	AlertLevel   string          `gorm:"size:20;comment:告警级别" json:"alert_level"`

	// 关联
	DataSource *DataSource       `gorm:"foreignKey:DataSourceID" json:"data_source,omitempty"`
	ETLJob     *ETLJob           `gorm:"foreignKey:ETLJobID" json:"etl_job,omitempty"`
	Creator    *User             `gorm:"foreignKey:CreatedBy" json:"creator,omitempty"`
	Reports    []QualityReport   `gorm:"foreignKey:RuleID" json:"reports,omitempty"`
}

// TableName 指定表名
func (QualityRule) TableName() string {
	return GetTableName("quality_rules")
}

// QualityReport 数据质量报告模型
type QualityReport struct {
	BaseModel
	RuleID       uint      `gorm:"not null;comment:规则ID" json:"rule_id"`
	CheckTime    time.Time `gorm:"not null;comment:检查时间" json:"check_time"`
	Status       string    `gorm:"not null;size:20;comment:检查状态" json:"status"`
	Score        float64   `gorm:"comment:质量分数" json:"score"`
	TotalCount   int64     `gorm:"comment:总记录数" json:"total_count"`
	PassCount    int64     `gorm:"comment:通过记录数" json:"pass_count"`
	FailCount    int64     `gorm:"comment:失败记录数" json:"fail_count"`
	Details      string    `gorm:"type:text;comment:检查详情JSON" json:"details"`
	Suggestions  string    `gorm:"type:text;comment:改进建议" json:"suggestions"`

	// 关联
	Rule *QualityRule `gorm:"foreignKey:RuleID" json:"rule,omitempty"`
}

// TableName 指定表名
func (QualityReport) TableName() string {
	return GetTableName("quality_reports")
}

// ETL配置结构
type ETLJobConfig struct {
	// 数据源配置
	SourceConfig map[string]interface{} `json:"source_config"`
	TargetConfig map[string]interface{} `json:"target_config"`

	// 转换配置
	Transformations []TransformationConfig `json:"transformations"`

	// 数据质量配置
	QualityChecks []QualityCheckConfig `json:"quality_checks"`

	// 调度配置
	ScheduleConfig ScheduleConfig `json:"schedule_config"`
}

// 转换配置
type TransformationConfig struct {
	Type       string                 `json:"type"`
	Name       string                 `json:"name"`
	Parameters map[string]interface{} `json:"parameters"`
	Order      int                    `json:"order"`
}

// 质量检查配置
type QualityCheckConfig struct {
	Type      string                 `json:"type"`
	Rules     []map[string]interface{} `json:"rules"`
	Threshold float64                `json:"threshold"`
	Action    string                 `json:"action"` // fail/warn/ignore
}

// 调度配置
type ScheduleConfig struct {
	Type       string    `json:"type"`        // once/cron/interval
	CronExpr   string    `json:"cron_expr"`
	Interval   int       `json:"interval"`    // 间隔秒数
	StartTime  time.Time `json:"start_time"`
	EndTime    *time.Time `json:"end_time"`
	MaxRuns    int       `json:"max_runs"`
}

// ETL作业请求结构
type ETLJobRequest struct {
	Name        string       `json:"name" binding:"required,min=1,max=100"`
	Description string       `json:"description"`
	SourceID    uint         `json:"source_id" binding:"required"`
	TargetID    uint         `json:"target_id"`
	Config      ETLJobConfig `json:"config"`
	CronExpr    string       `json:"cron_expr"`
	IsEnabled   bool         `json:"is_enabled"`
	Priority    int          `json:"priority"`
	MaxRetries  int          `json:"max_retries"`
	Timeout     int          `json:"timeout"`
	Remark      string       `json:"remark"`
}

// ETL作业响应结构
type ETLJobResponse struct {
	ID           uint               `json:"id"`
	Name         string             `json:"name"`
	Description  string             `json:"description"`
	SourceID     uint               `json:"source_id"`
	TargetID     uint               `json:"target_id"`
	Config       ETLJobConfig       `json:"config"`
	CronExpr     string             `json:"cron_expr"`
	Status       string             `json:"status"`
	IsEnabled    bool               `json:"is_enabled"`
	Priority     int                `json:"priority"`
	MaxRetries   int                `json:"max_retries"`
	Timeout      int                `json:"timeout"`
	LastRunAt    *time.Time         `json:"last_run_at"`
	NextRunAt    *time.Time         `json:"next_run_at"`
	RunCount     int                `json:"run_count"`
	SuccessCount int                `json:"success_count"`
	FailureCount int                `json:"failure_count"`
	Source       *DataSourceResponse `json:"source"`
	Target       *DataSourceResponse `json:"target"`
	Creator      *UserResponse      `json:"creator"`
	CreatedAt    time.Time          `json:"created_at"`
	UpdatedAt    time.Time          `json:"updated_at"`
}

// ETL执行请求结构
type ETLExecutionRequest struct {
	JobID       uint   `json:"job_id" binding:"required"`
	TriggerType string `json:"trigger_type" binding:"required,oneof=manual schedule api"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// ETL执行统计结构
type ETLExecutionStats struct {
	TotalJobs      int64 `json:"total_jobs"`
	ActiveJobs     int64 `json:"active_jobs"`
	TotalExecutions int64 `json:"total_executions"`
	RunningExecutions int64 `json:"running_executions"`
	TodayExecutions int64 `json:"today_executions"`
	SuccessRate    float64 `json:"success_rate"`
}

// 方法：设置配置数据
func (job *ETLJob) SetConfig(config ETLJobConfig) error {
	configBytes, err := json.Marshal(config)
	if err != nil {
		return err
	}
	job.ConfigData = string(configBytes)
	job.Config = configBytes
	return nil
}

// 方法：获取配置数据
func (job *ETLJob) GetConfig() (*ETLJobConfig, error) {
	var config ETLJobConfig
	if job.ConfigData != "" {
		err := json.Unmarshal([]byte(job.ConfigData), &config)
		if err != nil {
			return nil, err
		}
	}
	return &config, nil
}

// 方法：计算成功率
func (job *ETLJob) GetSuccessRate() float64 {
	if job.RunCount == 0 {
		return 0
	}
	return float64(job.SuccessCount) / float64(job.RunCount) * 100
}

// 方法：计算执行时长
func (exec *ETLExecution) CalculateDuration() {
	if exec.EndTime != nil {
		exec.Duration = exec.EndTime.Sub(exec.StartTime).Milliseconds()
	}
}

// ETL状态常量 - 新增的状态
const (
	ETLStatusCreated  = "created"
	ETLStatusStopped  = "stopped"
	ETLStatusIdle     = "idle"
	ETLStatusError    = "error"
)

// 方法：检查执行是否完成
func (exec *ETLExecution) IsCompleted() bool {
	return exec.Status == ETLStatusSuccess || exec.Status == ETLStatusFailed || exec.Status == ETLStatusCanceled
}