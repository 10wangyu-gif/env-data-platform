package services

import (
	"fmt"
	"sync"
	"time"

	"github.com/env-data-platform/internal/database"
	"github.com/env-data-platform/internal/models"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ETLScheduler ETL作业调度器
type ETLScheduler struct {
	cron    *cron.Cron
	jobs    map[uint]cron.EntryID
	mutex   sync.RWMutex
	logger  *zap.Logger
	db      *gorm.DB
	executor *ETLExecutor
}

// NewETLScheduler 创建ETL调度器
func NewETLScheduler(logger *zap.Logger) *ETLScheduler {
	// 创建带秒级精度的cron调度器
	c := cron.New(cron.WithSeconds())

	scheduler := &ETLScheduler{
		cron:     c,
		jobs:     make(map[uint]cron.EntryID),
		logger:   logger,
		db:       database.GetDB(),
		executor: NewETLExecutor(logger),
	}

	// 启动调度器
	c.Start()

	// 从数据库加载已启用的作业
	scheduler.LoadJobsFromDB()

	return scheduler
}

// LoadJobsFromDB 从数据库加载已启用的作业
func (s *ETLScheduler) LoadJobsFromDB() {
	var jobs []models.ETLJob
	err := s.db.Where("is_enabled = ? AND cron_expr != ''", true).Find(&jobs).Error
	if err != nil {
		s.logger.Error("Failed to load jobs from database", zap.Error(err))
		return
	}

	for _, job := range jobs {
		if err := s.ScheduleJob(&job); err != nil {
			s.logger.Error("Failed to schedule job from database",
				zap.Uint("job_id", job.ID),
				zap.String("job_name", job.Name),
				zap.Error(err))
		}
	}

	s.logger.Info("Loaded ETL jobs from database", zap.Int("count", len(jobs)))
}

// ScheduleJob 调度作业
func (s *ETLScheduler) ScheduleJob(job *models.ETLJob) error {
	if job.CronExpr == "" {
		return fmt.Errorf("cron expression is empty")
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	// 如果作业已经被调度，先移除
	if entryID, exists := s.jobs[job.ID]; exists {
		s.cron.Remove(entryID)
		delete(s.jobs, job.ID)
	}

	// 添加新的调度
	entryID, err := s.cron.AddFunc(job.CronExpr, func() {
		s.executeScheduledJob(job.ID)
	})

	if err != nil {
		return fmt.Errorf("failed to add cron job: %w", err)
	}

	s.jobs[job.ID] = entryID

	// 计算下次运行时间
	nextRun := s.cron.Entry(entryID).Next
	s.db.Model(job).Update("next_run_at", nextRun)

	s.logger.Info("ETL job scheduled",
		zap.Uint("job_id", job.ID),
		zap.String("job_name", job.Name),
		zap.String("cron_expr", job.CronExpr),
		zap.Time("next_run", nextRun))

	return nil
}

// UnscheduleJob 取消调度作业
func (s *ETLScheduler) UnscheduleJob(jobID uint) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if entryID, exists := s.jobs[jobID]; exists {
		s.cron.Remove(entryID)
		delete(s.jobs, jobID)

		s.logger.Info("ETL job unscheduled", zap.Uint("job_id", jobID))
	}
}

// executeScheduledJob 执行调度的作业
func (s *ETLScheduler) executeScheduledJob(jobID uint) {
	// 获取作业信息
	var job models.ETLJob
	if err := s.db.Preload("Source").Preload("Target").First(&job, jobID).Error; err != nil {
		s.logger.Error("Failed to get job for scheduled execution",
			zap.Uint("job_id", jobID),
			zap.Error(err))
		return
	}

	// 检查作业是否仍然启用
	if !job.IsEnabled {
		s.logger.Info("Skipping disabled job", zap.Uint("job_id", jobID))
		return
	}

	// 检查作业是否已在运行
	if job.Status == "running" {
		s.logger.Warn("Job is already running, skipping", zap.Uint("job_id", jobID))
		return
	}

	// 创建执行记录
	execution := models.ETLExecution{
		JobID:       job.ID,
		ExecutionID: generateExecutionID(),
		Status:      "running",
		StartTime:   time.Now(),
		TriggerType: "schedule",
	}

	if err := s.db.Create(&execution).Error; err != nil {
		s.logger.Error("Failed to create execution record for scheduled job",
			zap.Uint("job_id", jobID),
			zap.Error(err))
		return
	}

	// 更新作业状态
	s.db.Model(&job).Updates(map[string]interface{}{
		"status":      "running",
		"last_run_at": gorm.Expr("NOW()"),
		"run_count":   gorm.Expr("run_count + 1"),
	})

	s.logger.Info("Starting scheduled ETL job execution",
		zap.Uint("job_id", jobID),
		zap.String("execution_id", execution.ExecutionID))

	// 异步执行作业
	go s.executeJobAsync(&job, &execution)
}

// executeJobAsync 异步执行作业
func (s *ETLScheduler) executeJobAsync(job *models.ETLJob, execution *models.ETLExecution) {
	defer func() {
		if r := recover(); r != nil {
			s.logger.Error("ETL job execution panicked",
				zap.Uint("job_id", job.ID),
				zap.String("execution_id", execution.ExecutionID),
				zap.Any("error", r))

			// 更新执行记录为失败
			endTime := time.Now()
			s.db.Model(execution).Updates(map[string]interface{}{
				"status":        "failed",
				"end_time":      endTime,
				"duration":      endTime.Sub(execution.StartTime).Milliseconds(),
				"error_message": fmt.Sprintf("执行异常: %v", r),
			})

			// 更新作业状态
			s.db.Model(job).Updates(map[string]interface{}{
				"status":        "idle",
				"failure_count": gorm.Expr("failure_count + 1"),
			})
		}
	}()

	// 执行ETL作业
	result := s.executor.ExecuteJob(nil, job, execution, nil)

	// 更新执行记录
	endTime := time.Now()
	updates := map[string]interface{}{
		"status":        result.Status,
		"end_time":      endTime,
		"duration":      endTime.Sub(execution.StartTime).Milliseconds(),
		"input_rows":    result.InputRows,
		"output_rows":   result.OutputRows,
		"error_rows":    result.ErrorRows,
		"skipped_rows":  result.SkippedRows,
		"error_message": result.ErrorMessage,
		"log_content":   result.LogContent,
	}

	s.db.Model(execution).Updates(updates)

	// 更新作业统计
	jobUpdates := map[string]interface{}{
		"status": "idle",
	}

	if result.Status == "success" {
		jobUpdates["success_count"] = gorm.Expr("success_count + 1")
	} else {
		jobUpdates["failure_count"] = gorm.Expr("failure_count + 1")
	}

	s.db.Model(job).Updates(jobUpdates)

	s.logger.Info("Scheduled ETL job execution completed",
		zap.Uint("job_id", job.ID),
		zap.String("execution_id", execution.ExecutionID),
		zap.String("status", result.Status),
		zap.Int64("duration_ms", endTime.Sub(execution.StartTime).Milliseconds()),
		zap.Int64("input_rows", result.InputRows),
		zap.Int64("output_rows", result.OutputRows))
}

// GetJobStatus 获取作业调度状态
func (s *ETLScheduler) GetJobStatus(jobID uint) *JobScheduleStatus {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	status := &JobScheduleStatus{
		JobID:      jobID,
		IsScheduled: false,
	}

	if entryID, exists := s.jobs[jobID]; exists {
		entry := s.cron.Entry(entryID)
		status.IsScheduled = true
		status.NextRun = entry.Next
		status.PrevRun = entry.Prev
	}

	return status
}

// ListScheduledJobs 列出所有已调度的作业
func (s *ETLScheduler) ListScheduledJobs() []JobScheduleStatus {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	var jobs []JobScheduleStatus
	for jobID, entryID := range s.jobs {
		entry := s.cron.Entry(entryID)
		jobs = append(jobs, JobScheduleStatus{
			JobID:       jobID,
			IsScheduled: true,
			NextRun:     entry.Next,
			PrevRun:     entry.Prev,
		})
	}

	return jobs
}

// Stop 停止调度器
func (s *ETLScheduler) Stop() {
	s.logger.Info("Stopping ETL scheduler")
	s.cron.Stop()
}

// JobScheduleStatus 作业调度状态
type JobScheduleStatus struct {
	JobID       uint      `json:"job_id"`
	IsScheduled bool      `json:"is_scheduled"`
	NextRun     time.Time `json:"next_run,omitempty"`
	PrevRun     time.Time `json:"prev_run,omitempty"`
}

// generateExecutionID 生成执行ID
func generateExecutionID() string {
	return fmt.Sprintf("exec_%d", time.Now().UnixNano())
}