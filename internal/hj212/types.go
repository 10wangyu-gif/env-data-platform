package hj212

import (
	"time"
)

// Packet HJ212数据包
type Packet struct {
	// 原始数据
	RawData []byte

	// 头部信息
	Header map[string]string

	// 数据区
	DataArea map[string]string

	// 核心字段
	QN       string    // 请求编号 (时间戳+序列号)
	ST       string    // 系统编码
	CN       string    // 命令编码
	PW       string    // 访问密码
	MN       string    // 设备唯一标识
	Flag     int       // 标志位
	CP       string    // 指令参数/数据内容
	CRC      uint16    // CRC校验码
	DataTime time.Time // 数据时间

	// 监测因子数据
	Factors map[string]*FactorData

	// 告警数据
	AlarmData *AlarmData

	// 响应数据
	ExeRtn  string // 执行结果
	RtnInfo string // 返回信息
}

// FactorData 监测因子数据
type FactorData struct {
	Code  string  // 因子编码
	Name  string  // 因子名称
	Rtd   float64 // 实时数据
	Avg   float64 // 平均值
	Max   float64 // 最大值
	Min   float64 // 最小值
	Cou   float64 // 累计值
	Flag  string  // 数据标记
	EFlag string  // 异常标记
	Unit  string  // 单位
}

// AlarmData 告警数据
type AlarmData struct {
	DataTime  time.Time                // 数据时间
	AlarmTime time.Time                // 告警时间
	AlarmType string                   // 告警类型
	Factors   map[string]*AlarmFactor  // 告警因子
}

// AlarmFactor 告警因子
type AlarmFactor struct {
	Code       string  // 因子编码
	Name       string  // 因子名称
	Value      float64 // 当前值
	UpperLimit float64 // 上限
	LowerLimit float64 // 下限
	AlarmType  string  // 告警类型
}

// 命令编码常量
const (
	// 初始化命令
	CN_SetTimeout         = "1000" // 设置超时时间及重发次数
	CN_GetTime            = "1011" // 提取现场机时间
	CN_SetTime            = "1012" // 设置现场机时间
	CN_GetRealtimeInterval = "1061" // 提取实时数据间隔
	CN_SetRealtimeInterval = "1062" // 设置实时数据间隔
	CN_GetMinuteInterval   = "1063" // 提取分钟数据间隔
	CN_SetMinuteInterval   = "1064" // 设置分钟数据间隔

	// 参数命令
	CN_GetParams          = "1001" // 提取参数
	CN_SetParams          = "1002" // 设置参数
	CN_GetDataTime        = "1011" // 提取数采仪时间
	CN_SetDataTime        = "1012" // 设置数采仪时间
	CN_GetRtdInterval     = "1061" // 提取实时数据间隔
	CN_SetRtdInterval     = "1062" // 设置实时数据间隔
	CN_GetMinInterval     = "1063" // 提取分钟数据间隔
	CN_SetMinInterval     = "1064" // 设置分钟数据间隔
	CN_SetPassword        = "1072" // 设置现场机密码

	// 数据命令
	CN_GetRtdData         = "2011" // 取污染物实时数据
	CN_StopRtdData        = "2012" // 停止察看污染物实时数据
	CN_GetDeviceStatus    = "2021" // 取设备运行状态数据
	CN_StopDeviceStatus   = "2022" // 停止察看设备运行状态
	CN_GetDayData         = "2031" // 取污染物日历史数据
	CN_GetRunTimeData     = "2041" // 取设备运行时间日历史数据
	CN_GetMinuteData      = "2051" // 取污染物分钟数据
	CN_GetHourData        = "2061" // 取污染物小时数据
	CN_GetDayData2        = "2061" // 取污染物日数据
	CN_GetFactorHourData  = "2071" // 取监测因子小时数据
	CN_GetFactorDayData   = "2081" // 取监测因子日数据

	// 控制命令
	CN_ZeroCal            = "3011" // 零点校准量程校准
	CN_RtdSample          = "3012" // 即时采样
	CN_StartClearDevice   = "3013" // 启动清洗/反吹
	CN_ComparisonSample   = "3014" // 比对采样
	CN_PhotoSample        = "3015" // 超标留样
	CN_SetSamplePeriod    = "3016" // 设置设备采样时间周期
	CN_GetSamplePeriod    = "3017" // 提取设备采样时间周期
	CN_GetSampleTime      = "3018" // 提取设备采样时间
	CN_GetDeviceInfo      = "3019" // 提取设备唯一标识
	CN_GetSceneInfo       = "3020" // 提取现场机信息

	// 交互命令
	CN_Response           = "9011" // 请求应答
	CN_ExecuteResponse    = "9012" // 执行结果
	CN_Notice             = "9013" // 通知应答
	CN_DataResponse       = "9014" // 数据应答
)

// 系统编码常量
const (
	ST_Water = "21" // 地表水质量监测
	ST_Air   = "22" // 空气质量监测
	ST_Noise = "23" // 声环境质量监测
	ST_Soil  = "24" // 土壤质量监测
	ST_Ocean = "25" // 海水质量监测
	ST_WasteWater = "32" // 废水污染源
	ST_WasteGas   = "31" // 废气污染源
	ST_System     = "91" // 系统交互
)

// 标志位定义
const (
	Flag_Online  = 0 // 在线监控
	Flag_Confirm = 1 // 应答确认
	Flag_NoSplit = 0 // 不拆分包
	Flag_Split   = 1 // 拆分包
)

// 执行结果定义
const (
	ExeRtn_Success         = "1"  // 执行成功
	ExeRtn_Failed          = "2"  // 执行失败
	ExeRtn_InvalidData     = "3"  // 命令参数错误
	ExeRtn_InvalidPassword = "4"  // 密码错误
	ExeRtn_InvalidMN       = "5"  // MN错误
	ExeRtn_NoData          = "100" // 无数据
)

// CommandInfo 命令信息
type CommandInfo struct {
	CN          string // 命令编码
	Name        string // 命令名称
	Type        string // 命令类型
	NeedAuth    bool   // 是否需要认证
	NeedConfirm bool   // 是否需要确认
}

// 命令信息映射
var CommandMap = map[string]*CommandInfo{
	CN_GetRtdData: {
		CN:          CN_GetRtdData,
		Name:        "取污染物实时数据",
		Type:        "data",
		NeedAuth:    false,
		NeedConfirm: false,
	},
	CN_GetMinuteData: {
		CN:          CN_GetMinuteData,
		Name:        "取污染物分钟数据",
		Type:        "data",
		NeedAuth:    false,
		NeedConfirm: false,
	},
	CN_GetHourData: {
		CN:          CN_GetHourData,
		Name:        "取污染物小时数据",
		Type:        "data",
		NeedAuth:    false,
		NeedConfirm: false,
	},
	CN_GetDayData: {
		CN:          CN_GetDayData,
		Name:        "取污染物日数据",
		Type:        "data",
		NeedAuth:    false,
		NeedConfirm: false,
	},
	CN_SetTime: {
		CN:          CN_SetTime,
		Name:        "设置现场机时间",
		Type:        "control",
		NeedAuth:    true,
		NeedConfirm: true,
	},
	CN_Response: {
		CN:          CN_Response,
		Name:        "请求应答",
		Type:        "response",
		NeedAuth:    false,
		NeedConfirm: false,
	},
}

// GetCommandInfo 获取命令信息
func GetCommandInfo(cn string) *CommandInfo {
	if info, ok := CommandMap[cn]; ok {
		return info
	}
	return nil
}

// IsDataCommand 判断是否为数据命令
func IsDataCommand(cn string) bool {
	return cn >= "2000" && cn < "3000"
}

// IsControlCommand 判断是否为控制命令
func IsControlCommand(cn string) bool {
	return cn >= "3000" && cn < "4000"
}

// IsResponseCommand 判断是否为响应命令
func IsResponseCommand(cn string) bool {
	return cn >= "9000" && cn < "9999"
}