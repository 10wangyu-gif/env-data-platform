package alarm

import (
	"encoding/json"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/env-data-platform/internal/database"
	"github.com/env-data-platform/internal/models"
)

// AlarmLevel 告警级别
type AlarmLevel string

const (
	AlarmLevelInfo     AlarmLevel = "info"     // 信息
	AlarmLevelWarning  AlarmLevel = "warning"  // 警告
	AlarmLevelCritical AlarmLevel = "critical" // 严重
	AlarmLevelFatal    AlarmLevel = "fatal"    // 致命
)

// AlarmRule 告警规则
type AlarmRule struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	FactorCode  string     `json:"factor_code"`  // 监测因子代码
	DeviceID    string     `json:"device_id"`    // 设备ID，空表示所有设备
	Operator    string     `json:"operator"`     // 操作符：>, <, >=, <=, ==, !=
	Threshold   float64    `json:"threshold"`    // 阈值
	Level       AlarmLevel `json:"level"`        // 告警级别
	Enabled     bool       `json:"enabled"`      // 是否启用
	CooldownMin int        `json:"cooldown_min"` // 冷却时间（分钟）
}

// AlarmEvent 告警事件
type AlarmEvent struct {
	ID          string                 `json:"id"`
	RuleID      string                 `json:"rule_id"`
	DeviceID    string                 `json:"device_id"`
	FactorCode  string                 `json:"factor_code"`
	FactorName  string                 `json:"factor_name"`
	Value       float64                `json:"value"`
	Threshold   float64                `json:"threshold"`
	Operator    string                 `json:"operator"`
	Level       AlarmLevel             `json:"level"`
	Message     string                 `json:"message"`
	RawData     map[string]interface{} `json:"raw_data"`
	TriggeredAt time.Time              `json:"triggered_at"`
	Status      string                 `json:"status"` // pending, acknowledged, resolved
}

// Detector 告警检测器
type Detector struct {
	logger *zap.Logger
	rules  map[string]*AlarmRule
	wsHub  WSHub // WebSocket集线器接口
}

// WSHub WebSocket集线器接口
type WSHub interface {
	BroadcastHJ212Data(data *models.HJ212Data)
	BroadcastDeviceStatus(deviceID string, status map[string]interface{})
	BroadcastAlarm(event interface{})
}

// NewDetector 创建新的告警检测器
func NewDetector(logger *zap.Logger, wsHub WSHub) *Detector {
	detector := &Detector{
		logger: logger,
		rules:  make(map[string]*AlarmRule),
		wsHub:  wsHub,
	}

	// 加载默认规则
	detector.loadDefaultRules()

	return detector
}

// loadDefaultRules 加载默认告警规则
func (d *Detector) loadDefaultRules() {
	defaultRules := []*AlarmRule{
		// 空气质量告警
		{
			ID:          "air_so2_high",
			Name:        "SO2浓度过高",
			Description: "二氧化硫浓度超过国家标准",
			FactorCode:  "a21001",
			Operator:    ">",
			Threshold:   0.5, // mg/m³
			Level:       AlarmLevelWarning,
			Enabled:     true,
			CooldownMin: 30,
		},
		{
			ID:          "air_pm25_critical",
			Name:        "PM2.5严重污染",
			Description: "PM2.5浓度达到严重污染级别",
			FactorCode:  "a34004",
			Operator:    ">",
			Threshold:   150, // μg/m³
			Level:       AlarmLevelCritical,
			Enabled:     true,
			CooldownMin: 15,
		},
		{
			ID:          "air_no_high",
			Name:        "NO浓度异常",
			Description: "一氧化氮浓度超标",
			FactorCode:  "a21002",
			Operator:    ">",
			Threshold:   0.2, // mg/m³
			Level:       AlarmLevelWarning,
			Enabled:     true,
			CooldownMin: 60,
		},
		// 水质告警
		{
			ID:          "water_ph_low",
			Name:        "pH值过低",
			Description: "水体pH值低于安全范围",
			FactorCode:  "w01001",
			Operator:    "<",
			Threshold:   6.0,
			Level:       AlarmLevelWarning,
			Enabled:     true,
			CooldownMin: 30,
		},
		{
			ID:          "water_ph_high",
			Name:        "pH值过高",
			Description: "水体pH值高于安全范围",
			FactorCode:  "w01001",
			Operator:    ">",
			Threshold:   9.0,
			Level:       AlarmLevelWarning,
			Enabled:     true,
			CooldownMin: 30,
		},
		{
			ID:          "water_cod_critical",
			Name:        "COD严重超标",
			Description: "化学需氧量严重超标",
			FactorCode:  "w01009",
			Operator:    ">",
			Threshold:   50.0, // mg/L
			Level:       AlarmLevelCritical,
			Enabled:     true,
			CooldownMin: 15,
		},
		{
			ID:          "water_do_low",
			Name:        "溶解氧过低",
			Description: "水体溶解氧浓度过低",
			FactorCode:  "w01003",
			Operator:    "<",
			Threshold:   3.0, // mg/L
			Level:       AlarmLevelWarning,
			Enabled:     true,
			CooldownMin: 30,
		},
	}

	for _, rule := range defaultRules {
		d.rules[rule.ID] = rule
	}

	d.logger.Info("Loaded default alarm rules", zap.Int("count", len(defaultRules)))
}

// CheckData 检查HJ212数据是否触发告警
func (d *Detector) CheckData(data *models.HJ212Data) {
	if data.ParsedData == nil {
		return
	}

	factors, ok := data.ParsedData["factors"].(map[string]interface{})
	if !ok {
		return
	}

	for factorCode, factorData := range factors {
		factorInfo, ok := factorData.(map[string]interface{})
		if !ok {
			continue
		}

		// 获取实时值
		rtdValue, ok := factorInfo["rtd"].(float64)
		if !ok {
			continue
		}

		factorName, _ := factorInfo["name"].(string)

		// 检查所有适用的规则
		for _, rule := range d.rules {
			if !rule.Enabled {
				continue
			}

			// 检查因子代码是否匹配
			if rule.FactorCode != factorCode {
				continue
			}

			// 检查设备ID是否匹配（空表示所有设备）
			if rule.DeviceID != "" && rule.DeviceID != data.DeviceID {
				continue
			}

			// 检查是否在冷却期内
			if d.isInCooldown(rule.ID, data.DeviceID) {
				continue
			}

			// 检查是否触发告警
			if d.checkThreshold(rtdValue, rule.Operator, rule.Threshold) {
				event := &AlarmEvent{
					ID:          d.generateAlarmID(),
					RuleID:      rule.ID,
					DeviceID:    data.DeviceID,
					FactorCode:  factorCode,
					FactorName:  factorName,
					Value:       rtdValue,
					Threshold:   rule.Threshold,
					Operator:    rule.Operator,
					Level:       rule.Level,
					Message:     d.generateAlarmMessage(rule, factorName, rtdValue),
					RawData:     data.ParsedData,
					TriggeredAt: time.Now(),
					Status:      "pending",
				}

				d.triggerAlarm(event)
			}
		}
	}
}

// checkThreshold 检查阈值
func (d *Detector) checkThreshold(value float64, operator string, threshold float64) bool {
	switch operator {
	case ">":
		return value > threshold
	case ">=":
		return value >= threshold
	case "<":
		return value < threshold
	case "<=":
		return value <= threshold
	case "==":
		return value == threshold
	case "!=":
		return value != threshold
	default:
		return false
	}
}

// isInCooldown 检查是否在冷却期内
func (d *Detector) isInCooldown(ruleID, deviceID string) bool {
	// 查询最近的告警记录
	var lastAlarm models.HJ212AlarmData
	err := database.DB.Where("device_id = ? AND alarm_type = ?", deviceID, ruleID).
		Order("received_at DESC").
		First(&lastAlarm).Error

	if err != nil {
		// 没有历史记录，不在冷却期
		return false
	}

	rule, exists := d.rules[ruleID]
	if !exists {
		return false
	}

	// 检查时间间隔
	cooldownDuration := time.Duration(rule.CooldownMin) * time.Minute
	return time.Since(lastAlarm.ReceivedAt) < cooldownDuration
}

// triggerAlarm 触发告警
func (d *Detector) triggerAlarm(event *AlarmEvent) {
	d.logger.Warn("Alarm triggered",
		zap.String("rule_id", event.RuleID),
		zap.String("device_id", event.DeviceID),
		zap.String("factor_code", event.FactorCode),
		zap.Float64("value", event.Value),
		zap.Float64("threshold", event.Threshold),
		zap.String("level", string(event.Level)))

	// 保存告警到数据库
	alarmData := models.HJ212AlarmData{
		DeviceID:     event.DeviceID,
		AlarmType:    event.RuleID,
		AlarmLevel:   string(event.Level),
		AlarmDesc:    event.Message,
		RawData:      d.marshalRawData(event.RawData),
		ReceivedFrom: "system",
		ReceivedAt:   event.TriggeredAt,
		Status:       event.Status,
	}

	if err := database.DB.Create(&alarmData).Error; err != nil {
		d.logger.Error("Failed to save alarm data", zap.Error(err))
	}

	// 通过WebSocket广播告警
	if d.wsHub != nil {
		d.wsHub.BroadcastAlarm(event)
	}
}

// marshalRawData 序列化原始数据
func (d *Detector) marshalRawData(data map[string]interface{}) string {
	if data == nil {
		return ""
	}
	bytes, err := json.Marshal(data)
	if err != nil {
		return ""
	}
	return string(bytes)
}

// generateAlarmID 生成告警ID
func (d *Detector) generateAlarmID() string {
	return fmt.Sprintf("alarm_%d", time.Now().UnixNano())
}

// generateAlarmMessage 生成告警消息
func (d *Detector) generateAlarmMessage(rule *AlarmRule, factorName string, value float64) string {
	if factorName == "" {
		factorName = rule.FactorCode
	}

	return fmt.Sprintf("%s: %s当前值%.2f %s %.2f（阈值）",
		rule.Name, factorName, value, rule.Operator, rule.Threshold)
}

// AddRule 添加告警规则
func (d *Detector) AddRule(rule *AlarmRule) {
	d.rules[rule.ID] = rule
	d.logger.Info("Added alarm rule", zap.String("rule_id", rule.ID), zap.String("name", rule.Name))
}

// RemoveRule 移除告警规则
func (d *Detector) RemoveRule(ruleID string) {
	delete(d.rules, ruleID)
	d.logger.Info("Removed alarm rule", zap.String("rule_id", ruleID))
}

// GetRules 获取所有告警规则
func (d *Detector) GetRules() map[string]*AlarmRule {
	return d.rules
}

// UpdateRule 更新告警规则
func (d *Detector) UpdateRule(rule *AlarmRule) {
	d.rules[rule.ID] = rule
	d.logger.Info("Updated alarm rule", zap.String("rule_id", rule.ID))
}