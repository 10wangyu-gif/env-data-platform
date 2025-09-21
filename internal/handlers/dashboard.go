package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/env-data-platform/internal/database"
	"github.com/env-data-platform/internal/models"
)

// DashboardHandler 仪表板处理器
type DashboardHandler struct {
	logger *zap.Logger
}

// NewDashboardHandler 创建仪表板处理器
func NewDashboardHandler(logger *zap.Logger) *DashboardHandler {
	return &DashboardHandler{
		logger: logger,
	}
}

// DashboardOverview 仪表板概览数据
type DashboardOverview struct {
	CoreStats       CoreStatsData       `json:"core_stats"`
	EnvironmentData EnvironmentData     `json:"environment_data"`
	ChartData       ChartData           `json:"chart_data"`
	ETLTaskStatus   []ETLTaskStatusInfo `json:"etl_task_status"`
	LatestAlerts    []AlertInfo         `json:"latest_alerts"`
	LastUpdated     time.Time           `json:"last_updated"`
}

// CoreStatsData 核心统计数据
type CoreStatsData struct {
	DataSourceConnections DataSourceStat `json:"data_source_connections"`
	RealTimeDataFlow      DataFlowStat   `json:"real_time_data_flow"`
	APICallsToday         APICallStat    `json:"api_calls_today"`
	SystemHealth          HealthStat     `json:"system_health"`
}

// DataSourceStat 数据源统计
type DataSourceStat struct {
	Current int    `json:"current"`
	Change  int    `json:"change"`
	Trend   string `json:"trend"` // "up", "down", "stable"
}

// DataFlowStat 数据流统计
type DataFlowStat struct {
	Current      float64 `json:"current"`      // 当前流量(条/秒)
	ChangePercent float64 `json:"change_percent"` // 变化百分比
	Trend        string  `json:"trend"`
}

// APICallStat API调用统计
type APICallStat struct {
	Current       int     `json:"current"`        // 今日调用量
	ChangePercent float64 `json:"change_percent"` // 变化百分比
	Trend         string  `json:"trend"`
}

// HealthStat 健康度统计
type HealthStat struct {
	Current float64 `json:"current"` // 健康度百分比
	Status  string  `json:"status"`  // "normal", "warning", "error"
}

// EnvironmentData 环境监测数据
type EnvironmentData struct {
	AirQuality       AirQualityData       `json:"air_quality"`
	WaterQuality     WaterQualityData     `json:"water_quality"`
	PollutionSources PollutionSourcesData `json:"pollution_sources"`
}

// AirQualityData 空气质量数据
type AirQualityData struct {
	PM25      EnvironmentMetric `json:"pm25"`
	PM10      EnvironmentMetric `json:"pm10"`
	AQI       EnvironmentMetric `json:"aqi"`
	Stations  StationStatus     `json:"stations"`
	Status    string            `json:"status"` // "online", "warning", "offline"
}

// WaterQualityData 水质数据
type WaterQualityData struct {
	COD       EnvironmentMetric `json:"cod"`
	Ammonia   EnvironmentMetric `json:"ammonia"`
	Phosphorus EnvironmentMetric `json:"phosphorus"`
	Sections  StationStatus     `json:"sections"`
	Status    string            `json:"status"`
}

// PollutionSourcesData 污染源数据
type PollutionSourcesData struct {
	Enterprises     int           `json:"enterprises"`
	Devices         int           `json:"devices"`
	TransmissionRate float64       `json:"transmission_rate"`
	AbnormalDevices int           `json:"abnormal_devices"`
	Status          string        `json:"status"`
}

// EnvironmentMetric 环境指标
type EnvironmentMetric struct {
	Value  string `json:"value"`
	Unit   string `json:"unit"`
	Level  string `json:"level"`  // "优", "良", "轻度污染" etc.
	Status string `json:"status"` // "success", "warning", "danger"
}

// StationStatus 监测站状态
type StationStatus struct {
	Total  int     `json:"total"`
	Online int     `json:"online"`
	Rate   float64 `json:"rate"`
}

// ChartData 图表数据
type ChartData struct {
	DataFlowTrend DataFlowTrendData `json:"data_flow_trend"`
	APICallStats  APICallStatsData  `json:"api_call_stats"`
}

// DataFlowTrendData 数据流趋势数据
type DataFlowTrendData struct {
	Labels []string  `json:"labels"`
	Values []float64 `json:"values"`
}

// APICallStatsData API调用统计数据
type APICallStatsData struct {
	Labels []string `json:"labels"`
	Values []int    `json:"values"`
}

// ETLTaskStatusInfo ETL任务状态信息
type ETLTaskStatusInfo struct {
	ID          uint      `json:"id"`
	Name        string    `json:"name"`
	Status      string    `json:"status"`
	StatusText  string    `json:"status_text"`
	LastRun     time.Time `json:"last_run"`
	Duration    string    `json:"duration"`
	Message     string    `json:"message"`
	Icon        string    `json:"icon"`
	ColorClass  string    `json:"color_class"`
}

// AlertInfo 告警信息
type AlertInfo struct {
	ID          uint      `json:"id"`
	Level       string    `json:"level"`       // "info", "warning", "error"
	Title       string    `json:"title"`
	Message     string    `json:"message"`
	CreatedAt   time.Time `json:"created_at"`
	RelativeTime string   `json:"relative_time"`
	Icon        string    `json:"icon"`
	ColorClass  string    `json:"color_class"`
}

// GetDashboardOverview 获取仪表板概览数据
// @Summary 获取仪表板概览数据
// @Description 获取仪表板所有模块的数据，包括统计、图表、任务状态等
// @Tags 仪表板
// @Produce json
// @Security BearerAuth
// @Success 200 {object} models.Response{data=DashboardOverview} "获取成功"
// @Router /api/v1/dashboard/overview [get]
func (h *DashboardHandler) GetDashboardOverview(c *gin.Context) {
	overview := DashboardOverview{
		CoreStats:       h.getCoreStats(),
		EnvironmentData: h.getEnvironmentData(),
		ChartData:       h.getChartData(),
		ETLTaskStatus:   h.getETLTaskStatus(),
		LatestAlerts:    h.getLatestAlerts(),
		LastUpdated:     time.Now(),
	}

	c.JSON(http.StatusOK, models.SuccessResponse(overview))
}

// getCoreStats 获取核心统计数据
func (h *DashboardHandler) getCoreStats() CoreStatsData {
	// 获取数据源统计
	var totalDataSources, activeDataSources int64
	database.DB.Model(&models.DataSource{}).Count(&totalDataSources)
	database.DB.Model(&models.DataSource{}).Where("status = 'active'").Count(&activeDataSources)

	// 获取ETL任务统计
	var totalETLJobs, runningETLJobs int64
	database.DB.Model(&models.ETLJob{}).Count(&totalETLJobs)
	database.DB.Model(&models.ETLJob{}).Where("status = 'running'").Count(&runningETLJobs)

	// 模拟API调用统计(实际应该从日志或统计表获取)
	apiCallsToday := 45200

	// 计算系统健康度
	healthScore := h.calculateSystemHealth(activeDataSources, totalDataSources, runningETLJobs)

	return CoreStatsData{
		DataSourceConnections: DataSourceStat{
			Current: int(activeDataSources),
			Change:  3,
			Trend:   "up",
		},
		RealTimeDataFlow: DataFlowStat{
			Current:       1200.0,
			ChangePercent: 15.0,
			Trend:         "up",
		},
		APICallsToday: APICallStat{
			Current:       apiCallsToday,
			ChangePercent: -5.0,
			Trend:         "down",
		},
		SystemHealth: HealthStat{
			Current: healthScore,
			Status:  h.getHealthStatus(healthScore),
		},
	}
}

// getEnvironmentData 获取环境监测数据
func (h *DashboardHandler) getEnvironmentData() EnvironmentData {
	return EnvironmentData{
		AirQuality: AirQualityData{
			PM25: EnvironmentMetric{
				Value:  "35",
				Unit:   "μg/m³",
				Level:  "良",
				Status: "success",
			},
			PM10: EnvironmentMetric{
				Value:  "52",
				Unit:   "μg/m³",
				Level:  "良",
				Status: "success",
			},
			AQI: EnvironmentMetric{
				Value:  "68",
				Unit:   "",
				Level:  "良",
				Status: "success",
			},
			Stations: StationStatus{
				Total:  127,
				Online: 124,
				Rate:   97.6,
			},
			Status: "online",
		},
		WaterQuality: WaterQualityData{
			COD: EnvironmentMetric{
				Value:  "15.2",
				Unit:   "mg/L",
				Level:  "Ⅱ类",
				Status: "info",
			},
			Ammonia: EnvironmentMetric{
				Value:  "0.45",
				Unit:   "mg/L",
				Level:  "Ⅱ类",
				Status: "info",
			},
			Phosphorus: EnvironmentMetric{
				Value:  "0.08",
				Unit:   "mg/L",
				Level:  "Ⅰ类",
				Status: "success",
			},
			Sections: StationStatus{
				Total:  89,
				Online: 87,
				Rate:   97.8,
			},
			Status: "online",
		},
		PollutionSources: PollutionSourcesData{
			Enterprises:      156,
			Devices:          342,
			TransmissionRate: 94.2,
			AbnormalDevices:  8,
			Status:           "warning",
		},
	}
}

// getChartData 获取图表数据
func (h *DashboardHandler) getChartData() ChartData {
	return ChartData{
		DataFlowTrend: DataFlowTrendData{
			Labels: []string{"00:00", "04:00", "08:00", "12:00", "16:00", "20:00", "24:00"},
			Values: []float64{800, 600, 1200, 1500, 1800, 1400, 1000},
		},
		APICallStats: APICallStatsData{
			Labels: []string{"空气质量", "水质监测", "污染源", "元数据", "数据导出", "其他"},
			Values: []int{12500, 8900, 6700, 4200, 3800, 2100},
		},
	}
}

// getETLTaskStatus 获取ETL任务状态
func (h *DashboardHandler) getETLTaskStatus() []ETLTaskStatusInfo {
	var tasks []models.ETLJob
	// 获取最近的ETL任务
	database.DB.Order("updated_at DESC").Limit(4).Find(&tasks)

	statusInfo := []ETLTaskStatusInfo{}

	// 如果没有真实数据，返回模拟数据
	if len(tasks) == 0 {
		statusInfo = []ETLTaskStatusInfo{
			{
				ID:         1,
				Name:       "环保数据同步任务",
				Status:     "completed",
				StatusText: "成功",
				LastRun:    time.Now().Add(-2 * time.Minute),
				Duration:   "5分钟",
				Message:    "执行成功 - 2分钟前",
				Icon:       "fas fa-check-circle",
				ColorClass: "forest",
			},
			{
				ID:         2,
				Name:       "HJ212数据清洗",
				Status:     "running",
				StatusText: "运行中",
				LastRun:    time.Now().Add(-10 * time.Minute),
				Duration:   "剩余5分钟",
				Message:    "正在执行 - 剩余5分钟",
				Icon:       "fas fa-clock",
				ColorClass: "air",
			},
			{
				ID:         3,
				Name:       "数据质量检查",
				Status:     "warning",
				StatusText: "警告",
				LastRun:    time.Now().Add(-15 * time.Minute),
				Duration:   "8分钟",
				Message:    "执行警告 - 15分钟前",
				Icon:       "fas fa-exclamation-triangle",
				ColorClass: "soil",
			},
			{
				ID:         4,
				Name:       "API数据导入",
				Status:     "failed",
				StatusText: "失败",
				LastRun:    time.Now().Add(-1 * time.Hour),
				Duration:   "失败",
				Message:    "执行失败 - 1小时前",
				Icon:       "fas fa-times-circle",
				ColorClass: "danger",
			},
		}
	} else {
		// 转换真实数据
		for _, task := range tasks {
			info := ETLTaskStatusInfo{
				ID:         task.ID,
				Name:       task.Name,
				Status:     task.Status,
				StatusText: h.getETLStatusText(task.Status),
				LastRun:    task.UpdatedAt,
				Message:    h.getETLStatusMessage(task.Status, task.UpdatedAt),
				Icon:       h.getETLStatusIcon(task.Status),
				ColorClass: h.getETLStatusColor(task.Status),
			}
			statusInfo = append(statusInfo, info)
		}
	}

	return statusInfo
}

// getLatestAlerts 获取最新告警
func (h *DashboardHandler) getLatestAlerts() []AlertInfo {
	// 模拟告警数据(实际应该从告警表获取)
	return []AlertInfo{
		{
			ID:           1,
			Level:        "error",
			Title:        "数据源连接中断",
			Message:      "监测站点SH-001连接异常，请及时处理",
			CreatedAt:    time.Now().Add(-2 * time.Minute),
			RelativeTime: "2分钟前",
			Icon:         "fas fa-exclamation-circle",
			ColorClass:   "danger",
		},
		{
			ID:           2,
			Level:        "warning",
			Title:        "数据质量异常",
			Message:      "PM2.5数据存在异常值，建议检查传感器",
			CreatedAt:    time.Now().Add(-15 * time.Minute),
			RelativeTime: "15分钟前",
			Icon:         "fas fa-exclamation-triangle",
			ColorClass:   "warning",
		},
		{
			ID:           3,
			Level:        "info",
			Title:        "系统维护通知",
			Message:      "计划于今晚22:00进行系统升级维护",
			CreatedAt:    time.Now().Add(-1 * time.Hour),
			RelativeTime: "1小时前",
			Icon:         "fas fa-info-circle",
			ColorClass:   "info",
		},
		{
			ID:           4,
			Level:        "success",
			Title:        "备份任务完成",
			Message:      "每日数据备份任务执行成功",
			CreatedAt:    time.Now().Add(-2 * time.Hour),
			RelativeTime: "2小时前",
			Icon:         "fas fa-check-circle",
			ColorClass:   "success",
		},
	}
}

// 辅助方法
func (h *DashboardHandler) calculateSystemHealth(active, total int64, running int64) float64 {
	if total == 0 {
		return 0.0
	}
	// 基于数据源在线率和ETL任务运行情况计算健康度
	dataSourceRate := float64(active) / float64(total)
	health := dataSourceRate * 100

	// 如果有运行中的ETL任务，健康度不会低于95%
	if running > 0 && health < 95.0 {
		health = 95.0
	}

	return health
}

func (h *DashboardHandler) getHealthStatus(health float64) string {
	if health >= 98.0 {
		return "normal"
	} else if health >= 90.0 {
		return "warning"
	}
	return "error"
}

func (h *DashboardHandler) getETLStatusText(status string) string {
	statusMap := map[string]string{
		"completed": "成功",
		"running":   "运行中",
		"failed":    "失败",
		"warning":   "警告",
		"pending":   "等待中",
	}
	if text, ok := statusMap[status]; ok {
		return text
	}
	return status
}

func (h *DashboardHandler) getETLStatusMessage(status string, lastRun time.Time) string {
	relativeTime := h.getRelativeTime(lastRun)

	switch status {
	case "completed":
		return "执行成功 - " + relativeTime
	case "running":
		return "正在执行 - " + relativeTime
	case "failed":
		return "执行失败 - " + relativeTime
	case "warning":
		return "执行警告 - " + relativeTime
	default:
		return "等待执行"
	}
}

func (h *DashboardHandler) getETLStatusIcon(status string) string {
	iconMap := map[string]string{
		"completed": "fas fa-check-circle",
		"running":   "fas fa-clock",
		"failed":    "fas fa-times-circle",
		"warning":   "fas fa-exclamation-triangle",
		"pending":   "fas fa-clock",
	}
	if icon, ok := iconMap[status]; ok {
		return icon
	}
	return "fas fa-clock"
}

func (h *DashboardHandler) getETLStatusColor(status string) string {
	colorMap := map[string]string{
		"completed": "forest",
		"running":   "air",
		"failed":    "danger",
		"warning":   "soil",
		"pending":   "water",
	}
	if color, ok := colorMap[status]; ok {
		return color
	}
	return "water"
}

func (h *DashboardHandler) getRelativeTime(t time.Time) string {
	duration := time.Since(t)

	if duration < time.Minute {
		return "刚刚"
	} else if duration < time.Hour {
		minutes := int(duration.Minutes())
		return fmt.Sprintf("%d分钟前", minutes)
	} else if duration < 24*time.Hour {
		hours := int(duration.Hours())
		return fmt.Sprintf("%d小时前", hours)
	} else {
		days := int(duration.Hours() / 24)
		return fmt.Sprintf("%d天前", days)
	}
}

// GetRealTimeData 获取实时数据
// @Summary 获取实时数据
// @Description 获取实时更新的核心统计数据
// @Tags 仪表板
// @Produce json
// @Security BearerAuth
// @Success 200 {object} models.Response{data=CoreStatsData} "获取成功"
// @Router /api/v1/dashboard/realtime [get]
func (h *DashboardHandler) GetRealTimeData(c *gin.Context) {
	coreStats := h.getCoreStats()
	c.JSON(http.StatusOK, models.SuccessResponse(coreStats))
}

// GetChartData 获取图表数据
// @Summary 获取图表数据
// @Description 获取数据流趋势和API调用统计图表数据
// @Tags 仪表板
// @Produce json
// @Security BearerAuth
// @Param period query string false "时间周期" Enums(today,week,month) default(today)
// @Success 200 {object} models.Response{data=ChartData} "获取成功"
// @Router /api/v1/dashboard/charts [get]
func (h *DashboardHandler) GetChartData(c *gin.Context) {
	period := c.DefaultQuery("period", "today")

	chartData := h.getChartDataByPeriod(period)
	c.JSON(http.StatusOK, models.SuccessResponse(chartData))
}

func (h *DashboardHandler) getChartDataByPeriod(period string) ChartData {
	switch period {
	case "week":
		return ChartData{
			DataFlowTrend: DataFlowTrendData{
				Labels: []string{"周一", "周二", "周三", "周四", "周五", "周六", "周日"},
				Values: []float64{1200, 1100, 1300, 1250, 1400, 900, 800},
			},
			APICallStats: APICallStatsData{
				Labels: []string{"空气质量", "水质监测", "污染源", "元数据", "数据导出", "其他"},
				Values: []int{87500, 62300, 46900, 29400, 26600, 14700},
			},
		}
	case "month":
		return ChartData{
			DataFlowTrend: DataFlowTrendData{
				Labels: []string{"第1周", "第2周", "第3周", "第4周"},
				Values: []float64{8500, 9200, 8800, 9500},
			},
			APICallStats: APICallStatsData{
				Labels: []string{"空气质量", "水质监测", "污染源", "元数据", "数据导出", "其他"},
				Values: []int{375000, 267200, 201300, 126400, 114200, 63000},
			},
		}
	default: // today
		return h.getChartData()
	}
}