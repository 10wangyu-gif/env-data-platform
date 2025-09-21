package hj212

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// HJ212Message HJ212协议消息
type HJ212Message struct {
	QN       string            `json:"qn"`        // 请求编码
	ST       string            `json:"st"`        // 系统编码
	CN       string            `json:"cn"`        // 命令编码
	PW       string            `json:"pw"`        // 访问密码
	MN       string            `json:"mn"`        // 设备唯一标识
	CP       map[string]string `json:"cp"`        // 指令参数
	Flag     int               `json:"flag"`      // 拆分包及应答标志
	RawData  string            `json:"raw_data"`  // 原始数据
	ParsedAt time.Time         `json:"parsed_at"` // 解析时间
}

// ParseHJ212 解析HJ212协议数据
func ParseHJ212(data string) (*HJ212Message, error) {
	// 移除可能的换行符和空白字符
	data = strings.TrimSpace(data)

	// 检查数据包格式：##数据段长度数据段&CRC校验码\r\n
	if !strings.HasPrefix(data, "##") {
		return nil, errors.New("invalid HJ212 format: missing header")
	}

	// 查找数据段长度
	lengthEndIndex := -1
	for i := 2; i < len(data) && i < 8; i++ {
		if data[i] < '0' || data[i] > '9' {
			lengthEndIndex = i
			break
		}
	}

	if lengthEndIndex == -1 {
		return nil, errors.New("invalid HJ212 format: invalid length")
	}

	// 解析数据段长度
	lengthStr := data[2:lengthEndIndex]
	dataLength, err := strconv.Atoi(lengthStr)
	if err != nil {
		return nil, fmt.Errorf("invalid data length: %v", err)
	}

	// 提取数据段
	dataSegmentStart := lengthEndIndex
	dataSegmentEnd := dataSegmentStart + dataLength

	if dataSegmentEnd > len(data) {
		return nil, errors.New("data length mismatch")
	}

	dataSegment := data[dataSegmentStart:dataSegmentEnd]

	// 检查CRC校验（简化处理，实际项目中需要验证CRC）
	if dataSegmentEnd+1 < len(data) && data[dataSegmentEnd] != '&' {
		return nil, errors.New("invalid HJ212 format: missing CRC separator")
	}

	// 解析数据段
	message, err := parseDataSegment(dataSegment)
	if err != nil {
		return nil, fmt.Errorf("failed to parse data segment: %v", err)
	}

	message.RawData = data
	message.ParsedAt = time.Now()

	return message, nil
}

// parseDataSegment 解析数据段
func parseDataSegment(dataSegment string) (*HJ212Message, error) {
	message := &HJ212Message{
		CP: make(map[string]string),
	}

	// 按分号分割参数
	params := strings.Split(dataSegment, ";")

	for _, param := range params {
		if param == "" {
			continue
		}

		// 按等号分割键值对
		parts := strings.SplitN(param, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "QN":
			message.QN = value
		case "ST":
			message.ST = value
		case "CN":
			message.CN = value
		case "PW":
			message.PW = value
		case "MN":
			message.MN = value
		case "Flag":
			if flag, err := strconv.Atoi(value); err == nil {
				message.Flag = flag
			}
		case "CP":
			// CP参数可能包含子参数
			message.CP = parseCPParameter(value)
		default:
			// 其他参数放入CP中
			message.CP[key] = value
		}
	}

	return message, nil
}

// parseCPParameter 解析CP参数
func parseCPParameter(cpValue string) map[string]string {
	cp := make(map[string]string)

	// CP参数格式：&&key1=value1&&key2=value2
	if strings.HasPrefix(cpValue, "&&") {
		cpValue = cpValue[2:] // 移除开头的&&
	}

	// 按&&分割子参数
	subParams := strings.Split(cpValue, "&&")

	for _, subParam := range subParams {
		if subParam == "" {
			continue
		}

		// 按等号分割键值对
		parts := strings.SplitN(subParam, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			cp[key] = value
		}
	}

	return cp
}

// GetDataType 获取数据类型
func (m *HJ212Message) GetDataType() string {
	switch m.CN {
	case "2011":
		return "实时数据"
	case "2051":
		return "分钟数据"
	case "2061":
		return "小时数据"
	case "2031":
		return "日数据"
	case "3020":
		return "报警数据"
	case "9011":
		return "心跳包"
	case "9012":
		return "设备信息"
	default:
		return "未知类型"
	}
}

// IsValid 验证消息是否有效
func (m *HJ212Message) IsValid() bool {
	return m.QN != "" && m.ST != "" && m.CN != "" && m.MN != ""
}

// GetMonitoringData 获取监测数据
func (m *HJ212Message) GetMonitoringData() map[string]interface{} {
	data := make(map[string]interface{})

	// 解析CP中的监测数据
	for key, value := range m.CP {
		// 监测因子格式：因子编码-Rtd（实时值）、因子编码-Avg（平均值）等
		if strings.Contains(key, "-") {
			parts := strings.Split(key, "-")
			if len(parts) >= 2 {
				factor := parts[0]
				dataType := parts[1]

				// 转换数值
				if numValue, err := strconv.ParseFloat(value, 64); err == nil {
					if data[factor] == nil {
						data[factor] = make(map[string]interface{})
					}
					if factorData, ok := data[factor].(map[string]interface{}); ok {
						factorData[dataType] = numValue
					}
				} else {
					if data[factor] == nil {
						data[factor] = make(map[string]interface{})
					}
					if factorData, ok := data[factor].(map[string]interface{}); ok {
						factorData[dataType] = value
					}
				}
			}
		} else {
			// 其他参数直接添加
			if numValue, err := strconv.ParseFloat(value, 64); err == nil {
				data[key] = numValue
			} else {
				data[key] = value
			}
		}
	}

	return data
}

// BuildResponse 构建响应消息
func BuildResponse(originalMessage *HJ212Message, execResult string) string {
	qn := time.Now().Format("20060102150405000")

	// 构建响应数据段
	dataSegment := fmt.Sprintf("QN=%s;ST=%s;CN=9011;PW=%s;MN=%s;Flag=0;CP=&&ExeRtn=%s",
		qn, originalMessage.ST, originalMessage.PW, originalMessage.MN, execResult)

	// 计算数据段长度
	length := fmt.Sprintf("%04d", len(dataSegment))

	// 构建完整响应（简化CRC校验）
	response := fmt.Sprintf("##%s%s&%04X\r\n", length, dataSegment, calculateCRC(dataSegment))

	return response
}

// calculateCRC 计算CRC校验码（简化实现）
func calculateCRC(data string) uint16 {
	var crc uint16 = 0xFFFF

	for _, b := range []byte(data) {
		crc ^= uint16(b)
		for i := 0; i < 8; i++ {
			if crc&1 != 0 {
				crc = (crc >> 1) ^ 0xA001
			} else {
				crc = crc >> 1
			}
		}
	}

	return crc
}