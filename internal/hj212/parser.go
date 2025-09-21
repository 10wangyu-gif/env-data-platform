package hj212

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// CRC16计算表 (CRC-16-CCITT)
var crcTable = []uint16{
	0x0000, 0x1021, 0x2042, 0x3063, 0x4084, 0x50a5, 0x60c6, 0x70e7,
	0x8108, 0x9129, 0xa14a, 0xb16b, 0xc18c, 0xd1ad, 0xe1ce, 0xf1ef,
	0x1231, 0x0210, 0x3273, 0x2252, 0x52b5, 0x4294, 0x72f7, 0x62d6,
	0x9339, 0x8318, 0xb37b, 0xa35a, 0xd3bd, 0xc39c, 0xf3ff, 0xe3de,
	0x2462, 0x3443, 0x0420, 0x1401, 0x64e6, 0x74c7, 0x44a4, 0x5485,
	0xa56a, 0xb54b, 0x8528, 0x9509, 0xe5ee, 0xf5cf, 0xc5ac, 0xd58d,
	0x3653, 0x2672, 0x1611, 0x0630, 0x76d7, 0x66f6, 0x5695, 0x46b4,
	0xb75b, 0xa77a, 0x9719, 0x8738, 0xf7df, 0xe7fe, 0xd79d, 0xc7bc,
	0x48c4, 0x58e5, 0x6886, 0x78a7, 0x0840, 0x1861, 0x2802, 0x3823,
	0xc9cc, 0xd9ed, 0xe98e, 0xf9af, 0x8948, 0x9969, 0xa90a, 0xb92b,
	0x5af5, 0x4ad4, 0x7ab7, 0x6a96, 0x1a71, 0x0a50, 0x3a33, 0x2a12,
	0xdbfd, 0xcbdc, 0xfbbf, 0xeb9e, 0x9b79, 0x8b58, 0xbb3b, 0xab1a,
	0x6ca6, 0x7c87, 0x4ce4, 0x5cc5, 0x2c22, 0x3c03, 0x0c60, 0x1c41,
	0xedae, 0xfd8f, 0xcdec, 0xddcd, 0xad2a, 0xbd0b, 0x8d68, 0x9d49,
	0x7e97, 0x6eb6, 0x5ed5, 0x4ef4, 0x3e13, 0x2e32, 0x1e51, 0x0e70,
	0xff9f, 0xefbe, 0xdfdd, 0xcffc, 0xbf1b, 0xaf3a, 0x9f59, 0x8f78,
	0x9188, 0x81a9, 0xb1ca, 0xa1eb, 0xd10c, 0xc12d, 0xf14e, 0xe16f,
	0x1080, 0x00a1, 0x30c2, 0x20e3, 0x5004, 0x4025, 0x7046, 0x6067,
	0x83b9, 0x9398, 0xa3fb, 0xb3da, 0xc33d, 0xd31c, 0xe37f, 0xf35e,
	0x02b1, 0x1290, 0x22f3, 0x32d2, 0x4235, 0x5214, 0x6277, 0x7256,
	0xb5ea, 0xa5cb, 0x95a8, 0x8589, 0xf56e, 0xe54f, 0xd52c, 0xc50d,
	0x34e2, 0x24c3, 0x14a0, 0x0481, 0x7466, 0x6447, 0x5424, 0x4405,
	0xa7db, 0xb7fa, 0x8799, 0x97b8, 0xe75f, 0xf77e, 0xc71d, 0xd73c,
	0x26d3, 0x36f2, 0x0691, 0x16b0, 0x6657, 0x7676, 0x4615, 0x5634,
	0xd94c, 0xc96d, 0xf90e, 0xe92f, 0x99c8, 0x89e9, 0xb98a, 0xa9ab,
	0x5844, 0x4865, 0x7806, 0x6827, 0x18c0, 0x08e1, 0x3882, 0x28a3,
	0xcb7d, 0xdb5c, 0xeb3f, 0xfb1e, 0x8bf9, 0x9bd8, 0xabbb, 0xbb9a,
	0x4a75, 0x5a54, 0x6a37, 0x7a16, 0x0af1, 0x1ad0, 0x2ab3, 0x3a92,
	0xfd2e, 0xed0f, 0xdd6c, 0xcd4d, 0xbdaa, 0xad8b, 0x9de8, 0x8dc9,
	0x7c26, 0x6c07, 0x5c64, 0x4c45, 0x3ca2, 0x2c83, 0x1ce0, 0x0cc1,
	0xef1f, 0xff3e, 0xcf5d, 0xdf7c, 0xaf9b, 0xbfba, 0x8fd9, 0x9ff8,
	0x6e17, 0x7e36, 0x4e55, 0x5e74, 0x2e93, 0x3eb2, 0x0ed1, 0x1ef0,
}

// Parser HJ212协议解析器
type Parser struct {
	// 协议版本
	Version string
}

// NewParser 创建解析器
func NewParser(version string) *Parser {
	return &Parser{
		Version: version,
	}
}

// Parse 解析HJ212数据包
func (p *Parser) Parse(data []byte) (*Packet, error) {
	// 转换为字符串
	str := string(data)

	// 基本格式验证
	if !strings.HasPrefix(str, "##") {
		return nil, errors.New("invalid packet header")
	}

	// 查找数据包结尾
	endIndex := strings.Index(str, "\r\n")
	if endIndex == -1 {
		return nil, errors.New("packet end not found")
	}

	// 提取数据段长度
	if len(str) < 6 {
		return nil, errors.New("packet too short")
	}

	lengthStr := str[2:6]
	dataLen, err := strconv.Atoi(lengthStr)
	if err != nil {
		return nil, fmt.Errorf("invalid data length: %v", err)
	}

	// 验证数据长度
	expectedLen := 2 + 4 + dataLen + 4 + 2 // ## + 长度 + 数据 + CRC + \r\n
	if len(str) < expectedLen {
		return nil, errors.New("incomplete packet")
	}

	// 提取数据段
	dataSegment := str[6 : 6+dataLen]

	// 提取并验证CRC
	crcStr := str[6+dataLen : 6+dataLen+4]
	crcExpected, err := strconv.ParseUint(crcStr, 16, 16)
	if err != nil {
		return nil, fmt.Errorf("invalid CRC: %v", err)
	}

	// 计算CRC
	crcCalculated := p.calculateCRC([]byte(dataSegment))
	if uint16(crcExpected) != crcCalculated {
		return nil, fmt.Errorf("CRC mismatch: expected %04X, got %04X", crcExpected, crcCalculated)
	}

	// 解析数据段
	packet, err := p.parseDataSegment(dataSegment)
	if err != nil {
		return nil, fmt.Errorf("failed to parse data segment: %v", err)
	}

	packet.RawData = data
	packet.CRC = uint16(crcExpected)

	return packet, nil
}

// parseDataSegment 解析数据段
func (p *Parser) parseDataSegment(data string) (*Packet, error) {
	packet := &Packet{
		Header:   make(map[string]string),
		DataArea: make(map[string]string),
	}

	// 分割数据段为字段
	fields := strings.Split(data, ";")

	for _, field := range fields {
		if field == "" {
			continue
		}

		// 分割键值对
		parts := strings.SplitN(field, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := parts[0]
		value := parts[1]

		// 根据键名分类处理
		switch key {
		case "QN": // 请求编号
			packet.Header["QN"] = value
			packet.QN = value
		case "ST": // 系统编码
			packet.Header["ST"] = value
			packet.ST = value
		case "CN": // 命令编码
			packet.Header["CN"] = value
			packet.CN = value
		case "PW": // 访问密码
			packet.Header["PW"] = value
			packet.PW = value
		case "MN": // 设备唯一标识
			packet.Header["MN"] = value
			packet.MN = value
		case "Flag": // 标志位
			packet.Header["Flag"] = value
			flag, _ := strconv.Atoi(value)
			packet.Flag = flag
		case "CP": // 数据区
			// CP=&&数据内容&&
			cpData := strings.TrimPrefix(value, "&&")
			cpData = strings.TrimSuffix(cpData, "&&")
			packet.CP = cpData
			p.parseCP(cpData, packet)
		default:
			packet.DataArea[key] = value
		}
	}

	return packet, nil
}

// parseCP 解析CP数据区
func (p *Parser) parseCP(cpData string, packet *Packet) {
	if cpData == "" {
		return
	}

	// 根据命令编码解析不同格式的数据
	switch packet.CN {
	case "2011": // 实时数据上传
		p.parseRealtimeData(cpData, packet)
	case "2031": // 分钟数据
		p.parseMinuteData(cpData, packet)
	case "2051": // 小时数据
		p.parseHourData(cpData, packet)
	case "2061": // 日数据
		p.parseDayData(cpData, packet)
	case "2021": // 超标告警
		p.parseAlarmData(cpData, packet)
	case "9011", "9012": // 响应消息
		p.parseResponse(cpData, packet)
	default:
		// 通用解析
		p.parseGenericData(cpData, packet)
	}
}

// parseRealtimeData 解析实时数据
func (p *Parser) parseRealtimeData(cpData string, packet *Packet) {
	fields := strings.Split(cpData, ",")

	for _, field := range fields {
		if field == "" {
			continue
		}

		parts := strings.SplitN(field, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := parts[0]
		value := parts[1]

		// 特殊字段处理
		if key == "DataTime" {
			packet.DataTime = p.parseDateTime(value)
		}

		// 存储数据
		packet.DataArea[key] = value

		// 解析监测因子数据
		if strings.Contains(key, "-") {
			// 格式: w01018-Rtd=1.234
			p.parseFactorData(key, value, packet)
		}
	}
}

// parseFactorData 解析监测因子数据
func (p *Parser) parseFactorData(key, value string, packet *Packet) {
	// 分离因子编码和数据类型
	parts := strings.Split(key, "-")
	if len(parts) != 2 {
		return
	}

	factorCode := parts[0]
	dataType := parts[1]

	// 创建因子数据结构
	if packet.Factors == nil {
		packet.Factors = make(map[string]*FactorData)
	}

	factor, exists := packet.Factors[factorCode]
	if !exists {
		factor = &FactorData{
			Code: factorCode,
			Name: getFactorName(factorCode),
		}
		packet.Factors[factorCode] = factor
	}

	// 根据数据类型设置值
	switch dataType {
	case "Rtd": // 实时数据
		factor.Rtd, _ = strconv.ParseFloat(value, 64)
	case "Avg": // 平均值
		factor.Avg, _ = strconv.ParseFloat(value, 64)
	case "Max": // 最大值
		factor.Max, _ = strconv.ParseFloat(value, 64)
	case "Min": // 最小值
		factor.Min, _ = strconv.ParseFloat(value, 64)
	case "Cou": // 累计值
		factor.Cou, _ = strconv.ParseFloat(value, 64)
	case "Flag": // 数据标记
		factor.Flag = value
	case "EFlag": // 异常标记
		factor.EFlag = value
	}
}

// parseDateTime 解析日期时间
func (p *Parser) parseDateTime(dtStr string) time.Time {
	// 格式: 20240320154530 (yyyyMMddHHmmss)
	if len(dtStr) != 14 {
		return time.Time{}
	}

	year, _ := strconv.Atoi(dtStr[0:4])
	month, _ := strconv.Atoi(dtStr[4:6])
	day, _ := strconv.Atoi(dtStr[6:8])
	hour, _ := strconv.Atoi(dtStr[8:10])
	minute, _ := strconv.Atoi(dtStr[10:12])
	second, _ := strconv.Atoi(dtStr[12:14])

	return time.Date(year, time.Month(month), day, hour, minute, second, 0, time.Local)
}

// parseMinuteData 解析分钟数据
func (p *Parser) parseMinuteData(cpData string, packet *Packet) {
	// 与实时数据类似，但可能包含统计值
	p.parseRealtimeData(cpData, packet)
}

// parseHourData 解析小时数据
func (p *Parser) parseHourData(cpData string, packet *Packet) {
	p.parseRealtimeData(cpData, packet)
}

// parseDayData 解析日数据
func (p *Parser) parseDayData(cpData string, packet *Packet) {
	p.parseRealtimeData(cpData, packet)
}

// parseAlarmData 解析告警数据
func (p *Parser) parseAlarmData(cpData string, packet *Packet) {
	fields := strings.Split(cpData, ",")

	packet.AlarmData = &AlarmData{
		Factors: make(map[string]*AlarmFactor),
	}

	for _, field := range fields {
		if field == "" {
			continue
		}

		parts := strings.SplitN(field, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := parts[0]
		value := parts[1]

		// 特殊字段处理
		switch key {
		case "DataTime":
			packet.AlarmData.DataTime = p.parseDateTime(value)
		case "AlarmTime":
			packet.AlarmData.AlarmTime = p.parseDateTime(value)
		case "AlarmType":
			packet.AlarmData.AlarmType = value
		default:
			// 解析告警因子数据
			if strings.Contains(key, "-") {
				p.parseAlarmFactor(key, value, packet.AlarmData)
			}
		}
	}
}

// parseAlarmFactor 解析告警因子
func (p *Parser) parseAlarmFactor(key, value string, alarmData *AlarmData) {
	parts := strings.Split(key, "-")
	if len(parts) != 2 {
		return
	}

	factorCode := parts[0]
	dataType := parts[1]

	factor, exists := alarmData.Factors[factorCode]
	if !exists {
		factor = &AlarmFactor{
			Code: factorCode,
			Name: getFactorName(factorCode),
		}
		alarmData.Factors[factorCode] = factor
	}

	switch dataType {
	case "Rtd":
		factor.Value, _ = strconv.ParseFloat(value, 64)
	case "UpperLimit":
		factor.UpperLimit, _ = strconv.ParseFloat(value, 64)
	case "LowerLimit":
		factor.LowerLimit, _ = strconv.ParseFloat(value, 64)
	case "AlarmType":
		factor.AlarmType = value
	}
}

// parseResponse 解析响应消息
func (p *Parser) parseResponse(cpData string, packet *Packet) {
	fields := strings.Split(cpData, ",")

	for _, field := range fields {
		parts := strings.SplitN(field, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := parts[0]
		value := parts[1]

		switch key {
		case "ExeRtn":
			packet.ExeRtn = value
		case "RtnInfo":
			packet.RtnInfo = value
		default:
			packet.DataArea[key] = value
		}
	}
}

// parseGenericData 通用数据解析
func (p *Parser) parseGenericData(cpData string, packet *Packet) {
	fields := strings.Split(cpData, ",")

	for _, field := range fields {
		if field == "" {
			continue
		}

		parts := strings.SplitN(field, "=", 2)
		if len(parts) != 2 {
			continue
		}

		packet.DataArea[parts[0]] = parts[1]
	}
}

// calculateCRC 计算CRC16校验码 (CRC-16-CCITT)
func (p *Parser) calculateCRC(data []byte) uint16 {
	crc := uint16(0xFFFF)

	for _, b := range data {
		tbl_idx := ((crc >> 8) ^ uint16(b)) & 0xFF
		crc = (crc << 8) ^ crcTable[tbl_idx]
	}

	return crc
}

// Build 构建HJ212数据包
func (p *Parser) Build(packet *Packet) ([]byte, error) {
	// 构建数据段
	var dataSegment strings.Builder

	// 添加头部字段
	dataSegment.WriteString(fmt.Sprintf("QN=%s;", packet.QN))
	dataSegment.WriteString(fmt.Sprintf("ST=%s;", packet.ST))
	dataSegment.WriteString(fmt.Sprintf("CN=%s;", packet.CN))
	dataSegment.WriteString(fmt.Sprintf("PW=%s;", packet.PW))
	dataSegment.WriteString(fmt.Sprintf("MN=%s;", packet.MN))
	dataSegment.WriteString(fmt.Sprintf("Flag=%d;", packet.Flag))

	// 构建CP数据区
	if packet.CP != "" {
		dataSegment.WriteString(fmt.Sprintf("CP=&&%s&&", packet.CP))
	}

	// 获取数据段字符串
	data := dataSegment.String()

	// 计算CRC
	crc := p.calculateCRC([]byte(data))

	// 构建完整数据包
	result := fmt.Sprintf("##%04d%s%04X\r\n", len(data), data, crc)

	return []byte(result), nil
}

// ValidatePacket 验证数据包
func (p *Parser) ValidatePacket(packet *Packet) error {
	// 验证必需字段
	if packet.QN == "" {
		return errors.New("QN is required")
	}
	if packet.ST == "" {
		return errors.New("ST is required")
	}
	if packet.CN == "" {
		return errors.New("CN is required")
	}
	if packet.MN == "" {
		return errors.New("MN is required")
	}

	// 验证QN格式 (YYYYMMDDHHMMSSmmm)
	if len(packet.QN) != 17 {
		return errors.New("invalid QN format")
	}

	// 验证ST编码
	if len(packet.ST) != 2 {
		return errors.New("invalid ST format")
	}

	// 验证CN编码
	if len(packet.CN) != 4 {
		return errors.New("invalid CN format")
	}

	return nil
}

// getFactorName 获取监测因子名称
func getFactorName(code string) string {
	factorNames := map[string]string{
		"a01001": "温度",
		"a01002": "湿度",
		"a01006": "气压",
		"a01007": "风速",
		"a01008": "风向",
		"a21001": "SO2",
		"a21002": "NO",
		"a21003": "NO2",
		"a21004": "NOx",
		"a21005": "CO",
		"a21026": "O3",
		"a34001": "TSP",
		"a34002": "PM10",
		"a34004": "PM2.5",
		"w01001": "pH值",
		"w01003": "溶解氧",
		"w01009": "化学需氧量",
		"w01010": "五日生化需氧量",
		"w01018": "总磷",
		"w01019": "总氮",
		"w21001": "氨氮",
		"w21003": "总氮",
		"w21011": "总磷",
	}

	if name, ok := factorNames[code]; ok {
		return name
	}
	return code
}

// GenerateQN 生成请求编号
func GenerateQN() string {
	now := time.Now()
	ms := now.UnixNano() / 1000000 % 1000
	return fmt.Sprintf("%s%03d", now.Format("20060102150405"), ms)
}