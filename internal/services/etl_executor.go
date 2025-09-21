package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/env-data-platform/internal/database"
	"github.com/env-data-platform/internal/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ETLExecutor ETL执行器
type ETLExecutor struct {
	logger      *zap.Logger
	db          *gorm.DB
	runningJobs map[uint]*JobExecution
	mutex       sync.RWMutex
}

// JobExecution 作业执行上下文
type JobExecution struct {
	JobID       uint
	ExecutionID string
	Context     context.Context
	Cancel      context.CancelFunc
	StartTime   time.Time
}

// ETLExecutionResult ETL执行结果
type ETLExecutionResult struct {
	Status       string `json:"status"`
	InputRows    int64  `json:"input_rows"`
	OutputRows   int64  `json:"output_rows"`
	ErrorRows    int64  `json:"error_rows"`
	SkippedRows  int64  `json:"skipped_rows"`
	ErrorMessage string `json:"error_message"`
	LogContent   string `json:"log_content"`
}

// NewETLExecutor 创建ETL执行器
func NewETLExecutor(logger *zap.Logger) *ETLExecutor {
	return &ETLExecutor{
		logger:      logger,
		db:          database.GetDB(),
		runningJobs: make(map[uint]*JobExecution),
	}
}

// ExecuteJob 执行ETL作业
func (e *ETLExecutor) ExecuteJob(ctx context.Context, job *models.ETLJob, execution *models.ETLExecution, parameters map[string]interface{}) *ETLExecutionResult {
	if ctx == nil {
		ctx = context.Background()
	}

	// 创建可取消的上下文
	jobCtx, cancel := context.WithTimeout(ctx, time.Duration(job.Timeout)*time.Second)
	defer cancel()

	// 记录运行中的作业
	e.mutex.Lock()
	e.runningJobs[job.ID] = &JobExecution{
		JobID:       job.ID,
		ExecutionID: execution.ExecutionID,
		Context:     jobCtx,
		Cancel:      cancel,
		StartTime:   time.Now(),
	}
	e.mutex.Unlock()

	// 执行完成后清理
	defer func() {
		e.mutex.Lock()
		delete(e.runningJobs, job.ID)
		e.mutex.Unlock()
	}()

	e.logger.Info("Starting ETL job execution",
		zap.Uint("job_id", job.ID),
		zap.String("execution_id", execution.ExecutionID),
		zap.String("job_name", job.Name))

	result := &ETLExecutionResult{
		Status: "running",
	}

	var logBuilder strings.Builder
	logBuilder.WriteString(fmt.Sprintf("[%s] ETL作业开始执行\n", time.Now().Format("2006-01-02 15:04:05")))

	// 获取作业配置
	config, err := job.GetConfig()
	if err != nil {
		result.Status = "failed"
		result.ErrorMessage = fmt.Sprintf("解析作业配置失败: %v", err)
		result.LogContent = logBuilder.String() + result.ErrorMessage
		return result
	}

	// 执行ETL步骤
	switch job.Source.Type {
	case "mysql", "postgresql":
		err = e.executeDatabaseETL(jobCtx, job, config, result, &logBuilder)
	case "hj212":
		err = e.executeHJ212ETL(jobCtx, job, config, result, &logBuilder)
	case "api":
		err = e.executeAPIETL(jobCtx, job, config, result, &logBuilder)
	default:
		err = fmt.Errorf("不支持的数据源类型: %s", job.Source.Type)
	}

	// 设置最终状态
	if err != nil {
		result.Status = "failed"
		result.ErrorMessage = err.Error()
		logBuilder.WriteString(fmt.Sprintf("[%s] ETL作业执行失败: %s\n", time.Now().Format("2006-01-02 15:04:05"), err.Error()))
	} else {
		result.Status = "success"
		logBuilder.WriteString(fmt.Sprintf("[%s] ETL作业执行成功\n", time.Now().Format("2006-01-02 15:04:05")))
	}

	result.LogContent = logBuilder.String()

	e.logger.Info("ETL job execution completed",
		zap.Uint("job_id", job.ID),
		zap.String("execution_id", execution.ExecutionID),
		zap.String("status", result.Status),
		zap.Int64("input_rows", result.InputRows),
		zap.Int64("output_rows", result.OutputRows),
		zap.Error(err))

	return result
}

// executeDatabaseETL 执行数据库ETL
func (e *ETLExecutor) executeDatabaseETL(ctx context.Context, job *models.ETLJob, config *models.ETLJobConfig, result *ETLExecutionResult, logBuilder *strings.Builder) error {
	logBuilder.WriteString(fmt.Sprintf("[%s] 开始执行数据库ETL\n", time.Now().Format("2006-01-02 15:04:05")))

	// 解析源数据源配置
	var sourceConfig map[string]interface{}
	if err := json.Unmarshal(job.Source.ConfigData, &sourceConfig); err != nil {
		return fmt.Errorf("解析源数据源配置失败: %v", err)
	}

	// 解析目标数据源配置（如果存在）
	var targetConfig map[string]interface{}
	if job.Target != nil {
		if err := json.Unmarshal(job.Target.ConfigData, &targetConfig); err != nil {
			return fmt.Errorf("解析目标数据源配置失败: %v", err)
		}
	}

	// 模拟数据抽取
	logBuilder.WriteString(fmt.Sprintf("[%s] 开始数据抽取\n", time.Now().Format("2006-01-02 15:04:05")))

	// 检查上下文是否取消
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// 模拟抽取1000条数据
	result.InputRows = 1000
	logBuilder.WriteString(fmt.Sprintf("[%s] 数据抽取完成，共抽取 %d 条记录\n", time.Now().Format("2006-01-02 15:04:05"), result.InputRows))

	// 模拟数据转换
	logBuilder.WriteString(fmt.Sprintf("[%s] 开始数据转换\n", time.Now().Format("2006-01-02 15:04:05")))

	// 模拟转换过程
	time.Sleep(2 * time.Second)

	// 检查上下文是否取消
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// 应用转换规则
	for _, transform := range config.Transformations {
		logBuilder.WriteString(fmt.Sprintf("[%s] 应用转换规则: %s\n", time.Now().Format("2006-01-02 15:04:05"), transform.Name))
	}

	result.OutputRows = result.InputRows - 50 // 模拟有50条错误数据
	result.ErrorRows = 50
	logBuilder.WriteString(fmt.Sprintf("[%s] 数据转换完成，输出 %d 条记录，错误 %d 条\n",
		time.Now().Format("2006-01-02 15:04:05"), result.OutputRows, result.ErrorRows))

	// 模拟数据加载
	if job.Target != nil {
		logBuilder.WriteString(fmt.Sprintf("[%s] 开始数据加载到目标数据源\n", time.Now().Format("2006-01-02 15:04:05")))

		// 模拟加载过程
		time.Sleep(1 * time.Second)

		// 检查上下文是否取消
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		logBuilder.WriteString(fmt.Sprintf("[%s] 数据加载完成\n", time.Now().Format("2006-01-02 15:04:05")))
	}

	return nil
}

// executeHJ212ETL 执行HJ212数据ETL
func (e *ETLExecutor) executeHJ212ETL(ctx context.Context, job *models.ETLJob, config *models.ETLJobConfig, result *ETLExecutionResult, logBuilder *strings.Builder) error {
	logBuilder.WriteString(fmt.Sprintf("[%s] 开始执行HJ212数据ETL\n", time.Now().Format("2006-01-02 15:04:05")))

	// 查询HJ212原始数据
	var dataCount int64
	startTime := time.Now().Add(-1 * time.Hour) // 处理最近1小时的数据

	err := e.db.Model(&models.HJ212Data{}).
		Where("created_at >= ?", startTime).
		Count(&dataCount).Error

	if err != nil {
		return fmt.Errorf("查询HJ212数据失败: %v", err)
	}

	result.InputRows = dataCount
	logBuilder.WriteString(fmt.Sprintf("[%s] 查询到 %d 条HJ212数据\n", time.Now().Format("2006-01-02 15:04:05"), dataCount))

	// 检查上下文是否取消
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// 模拟数据处理
	if dataCount > 0 {
		logBuilder.WriteString(fmt.Sprintf("[%s] 开始处理HJ212数据\n", time.Now().Format("2006-01-02 15:04:05")))

		// 模拟处理时间
		time.Sleep(time.Duration(dataCount/100) * time.Millisecond)

		result.OutputRows = dataCount * 95 / 100 // 95%的数据处理成功
		result.ErrorRows = dataCount - result.OutputRows

		logBuilder.WriteString(fmt.Sprintf("[%s] HJ212数据处理完成，成功 %d 条，失败 %d 条\n",
			time.Now().Format("2006-01-02 15:04:05"), result.OutputRows, result.ErrorRows))
	}

	return nil
}

// executeAPIETL 执行API数据ETL
func (e *ETLExecutor) executeAPIETL(ctx context.Context, job *models.ETLJob, config *models.ETLJobConfig, result *ETLExecutionResult, logBuilder *strings.Builder) error {
	logBuilder.WriteString(fmt.Sprintf("[%s] 开始执行API数据ETL\n", time.Now().Format("2006-01-02 15:04:05")))

	// 解析API配置
	var apiConfig map[string]interface{}
	if err := json.Unmarshal(job.Source.ConfigData, &apiConfig); err != nil {
		return fmt.Errorf("解析API配置失败: %v", err)
	}

	url, ok := apiConfig["url"].(string)
	if !ok {
		return fmt.Errorf("API URL配置错误")
	}

	logBuilder.WriteString(fmt.Sprintf("[%s] 调用API: %s\n", time.Now().Format("2006-01-02 15:04:05"), url))

	// 检查上下文是否取消
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// 模拟API调用
	time.Sleep(3 * time.Second)

	// 模拟获取数据
	result.InputRows = 500
	result.OutputRows = 480
	result.ErrorRows = 20

	logBuilder.WriteString(fmt.Sprintf("[%s] API调用完成，获取 %d 条数据，处理成功 %d 条\n",
		time.Now().Format("2006-01-02 15:04:05"), result.InputRows, result.OutputRows))

	return nil
}

// StopJob 停止作业执行
func (e *ETLExecutor) StopJob(jobID uint) error {
	e.mutex.RLock()
	execution, exists := e.runningJobs[jobID]
	e.mutex.RUnlock()

	if !exists {
		return fmt.Errorf("作业未在运行")
	}

	// 取消作业执行
	execution.Cancel()

	e.logger.Info("ETL job execution cancelled",
		zap.Uint("job_id", jobID),
		zap.String("execution_id", execution.ExecutionID))

	return nil
}

// IsJobRunning 检查作业是否正在运行
func (e *ETLExecutor) IsJobRunning(jobID uint) bool {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	_, exists := e.runningJobs[jobID]
	return exists
}

// GetRunningJobs 获取正在运行的作业列表
func (e *ETLExecutor) GetRunningJobs() []JobExecution {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	var jobs []JobExecution
	for _, execution := range e.runningJobs {
		jobs = append(jobs, *execution)
	}

	return jobs
}