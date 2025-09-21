package hj212

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/env-data-platform/internal/config"
	"github.com/env-data-platform/internal/database"
	"github.com/env-data-platform/internal/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ServerV2 增强版HJ212服务器
type ServerV2 struct {
	config      *config.HJ212Config
	logger      *zap.Logger
	listener    net.Listener
	parser      *Parser
	connections map[string]net.Conn
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
	db          *gorm.DB

	// 数据处理通道
	dataChannel  chan *Packet
	alarmChannel chan *AlarmData

	// 统计信息
	stats *ServerStats
}

// ServerStats 服务器统计信息
type ServerStats struct {
	mu              sync.RWMutex
	TotalPackets    uint64
	ValidPackets    uint64
	InvalidPackets  uint64
	TotalBytes      uint64
	Connections     uint32
	LastPacketTime  time.Time
	StartTime       time.Time
}

// NewServerV2 创建增强版服务器
func NewServerV2(cfg *config.HJ212Config, logger *zap.Logger) *ServerV2 {
	ctx, cancel := context.WithCancel(context.Background())

	return &ServerV2{
		config:       cfg,
		logger:       logger,
		parser:       NewParser("2017"),
		connections:  make(map[string]net.Conn),
		ctx:          ctx,
		cancel:       cancel,
		db:           database.GetDB(),
		dataChannel:  make(chan *Packet, 1000),
		alarmChannel: make(chan *AlarmData, 100),
		stats: &ServerStats{
			StartTime: time.Now(),
		},
	}
}

// Start 启动服务器
func (s *ServerV2) Start() error {
	if !s.config.Enabled {
		s.logger.Info("HJ212 server is disabled")
		return nil
	}

	// 启动数据处理协程
	go s.dataProcessor()
	go s.alarmProcessor()

	// 监听端口
	addr := fmt.Sprintf(":%d", s.config.TCPPort)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	s.listener = listener
	s.logger.Info("HJ212 server v2 started", zap.String("address", addr))

	// 接受连接
	go s.acceptConnections()

	// 启动定时任务
	go s.periodicTasks()

	return nil
}

// acceptConnections 接受新连接
func (s *ServerV2) acceptConnections() {
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			conn, err := s.listener.Accept()
			if err != nil {
				if s.ctx.Err() != nil {
					return
				}
				s.logger.Error("Failed to accept connection", zap.Error(err))
				continue
			}

			// 限制最大连接数
			s.mu.RLock()
			connCount := len(s.connections)
			s.mu.RUnlock()

			if connCount >= s.config.MaxConnections {
				s.logger.Warn("Max connections reached, rejecting new connection",
					zap.Int("max", s.config.MaxConnections))
				conn.Close()
				continue
			}

			go s.handleConnection(conn)
		}
	}
}

// handleConnection 处理连接
func (s *ServerV2) handleConnection(conn net.Conn) {
	defer conn.Close()

	deviceID := conn.RemoteAddr().String()
	s.logger.Info("New connection", zap.String("device", deviceID))

	// 注册连接
	s.mu.Lock()
	s.connections[deviceID] = conn
	s.stats.Connections++
	s.mu.Unlock()

	// 清理连接
	defer func() {
		s.mu.Lock()
		delete(s.connections, deviceID)
		s.stats.Connections--
		s.mu.Unlock()
		s.logger.Info("Connection closed", zap.String("device", deviceID))
	}()

	// 读取缓冲区
	buffer := make([]byte, s.config.BufferSize)
	var dataBuffer []byte

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			// 设置读取超时
			conn.SetReadDeadline(time.Now().Add(s.config.Timeout))

			n, err := conn.Read(buffer)
			if err != nil {
				if err != io.EOF && !isTimeout(err) {
					s.logger.Error("Read error", zap.String("device", deviceID), zap.Error(err))
				}
				return
			}

			// 累积数据
			dataBuffer = append(dataBuffer, buffer[:n]...)
			s.stats.TotalBytes += uint64(n)

			// 尝试解析数据包
			for len(dataBuffer) > 0 {
				// 查找包结束标记
				endIndex := findPacketEnd(dataBuffer)
				if endIndex == -1 {
					break // 不完整的包，等待更多数据
				}

				// 提取完整的数据包
				packetData := dataBuffer[:endIndex]
				dataBuffer = dataBuffer[endIndex:]

				// 处理数据包
				s.processPacket(conn, deviceID, packetData)
			}
		}
	}
}

// processPacket 处理数据包
func (s *ServerV2) processPacket(conn net.Conn, deviceID string, data []byte) {
	s.stats.TotalPackets++
	s.stats.LastPacketTime = time.Now()

	// 记录原始数据
	s.logger.Debug("Received packet",
		zap.String("device", deviceID),
		zap.Int("size", len(data)),
		zap.String("data", string(data)))

	// 解析数据包
	packet, err := s.parser.Parse(data)
	if err != nil {
		s.stats.InvalidPackets++
		s.logger.Error("Parse error",
			zap.String("device", deviceID),
			zap.Error(err),
			zap.String("raw", string(data)))

		// 发送错误响应
		s.sendErrorResponse(conn, packet, err)
		return
	}

	// 验证数据包
	if err := s.parser.ValidatePacket(packet); err != nil {
		s.stats.InvalidPackets++
		s.logger.Warn("Invalid packet",
			zap.String("device", deviceID),
			zap.Error(err))
		return
	}

	s.stats.ValidPackets++

	// 更新设备连接映射
	if packet.MN != "" {
		s.mu.Lock()
		delete(s.connections, deviceID)
		s.connections[packet.MN] = conn
		s.mu.Unlock()
	}

	// 根据命令类型处理
	switch {
	case IsDataCommand(packet.CN):
		s.handleDataCommand(conn, packet)
	case IsControlCommand(packet.CN):
		s.handleControlCommand(conn, packet)
	case IsResponseCommand(packet.CN):
		s.handleResponseCommand(conn, packet)
	default:
		s.logger.Warn("Unknown command",
			zap.String("device", deviceID),
			zap.String("cn", packet.CN))
	}
}

// handleDataCommand 处理数据命令
func (s *ServerV2) handleDataCommand(conn net.Conn, packet *Packet) {
	s.logger.Info("Data command received",
		zap.String("mn", packet.MN),
		zap.String("cn", packet.CN),
		zap.String("st", packet.ST))

	// 保存数据
	s.savePacketData(packet)

	// 发送到数据处理通道
	select {
	case s.dataChannel <- packet:
	default:
		s.logger.Warn("Data channel full, dropping packet")
	}

	// 检查是否有告警
	if packet.AlarmData != nil {
		select {
		case s.alarmChannel <- packet.AlarmData:
		default:
			s.logger.Warn("Alarm channel full")
		}
	}

	// 发送确认响应
	if packet.Flag&Flag_Confirm != 0 {
		s.sendSuccessResponse(conn, packet)
	}
}

// handleControlCommand 处理控制命令
func (s *ServerV2) handleControlCommand(conn net.Conn, packet *Packet) {
	s.logger.Info("Control command received",
		zap.String("mn", packet.MN),
		zap.String("cn", packet.CN))

	// TODO: 实现控制命令处理逻辑
	// 例如：设置参数、校准、采样等

	// 发送执行结果
	s.sendExecutionResponse(conn, packet, ExeRtn_Success, "Command executed")
}

// handleResponseCommand 处理响应命令
func (s *ServerV2) handleResponseCommand(conn net.Conn, packet *Packet) {
	s.logger.Debug("Response received",
		zap.String("mn", packet.MN),
		zap.String("cn", packet.CN),
		zap.String("result", packet.ExeRtn))

	// 更新设备状态
	s.updateDeviceStatus(packet)
}

// savePacketData 保存数据包数据
func (s *ServerV2) savePacketData(packet *Packet) {
	// 构建数据模型
	hj212Data := models.HJ212Data{
		DeviceID:     packet.MN,
		CommandCode:  packet.CN,
		DataType:     s.getDataType(packet.CN),
		RawData:      string(packet.RawData),
		ReceivedAt:   time.Now(),
		QualityLevel: "normal",
		IsValid:      true,
	}

	// 解析并保存因子数据
	if packet.Factors != nil && len(packet.Factors) > 0 {
		factorData := make(map[string]interface{})
		for code, factor := range packet.Factors {
			factorData[code] = map[string]interface{}{
				"name": factor.Name,
				"rtd":  factor.Rtd,
				"avg":  factor.Avg,
				"max":  factor.Max,
				"min":  factor.Min,
				"cou":  factor.Cou,
				"flag": factor.Flag,
				"unit": factor.Unit,
			}
		}
		// 添加系统编码信息
		factorData["system_code"] = packet.ST
		factorData["data_time"] = packet.DataTime.Format("2006-01-02 15:04:05")
		hj212Data.ParsedData = factorData
	}

	// 保存到数据库
	if err := s.db.Create(&hj212Data).Error; err != nil {
		s.logger.Error("Failed to save data",
			zap.String("mn", packet.MN),
			zap.Error(err))
	} else {
		s.logger.Debug("Data saved",
			zap.String("mn", packet.MN),
			zap.String("cn", packet.CN))
	}
}

// dataProcessor 数据处理协程
func (s *ServerV2) dataProcessor() {
	for {
		select {
		case <-s.ctx.Done():
			return
		case packet := <-s.dataChannel:
			// 数据质量检查
			s.checkDataQuality(packet)

			// 数据聚合
			s.aggregateData(packet)

			// 触发实时推送
			s.pushRealtimeData(packet)
		}
	}
}

// alarmProcessor 告警处理协程
func (s *ServerV2) alarmProcessor() {
	for {
		select {
		case <-s.ctx.Done():
			return
		case alarm := <-s.alarmChannel:
			s.processAlarm(alarm)
		}
	}
}

// processAlarm 处理告警
func (s *ServerV2) processAlarm(alarm *AlarmData) {
	s.logger.Warn("Processing alarm",
		zap.Time("time", alarm.AlarmTime),
		zap.String("type", alarm.AlarmType))

	// TODO: 实现告警处理逻辑
	// 1. 保存告警记录
	// 2. 发送告警通知
	// 3. 触发告警响应流程
}

// checkDataQuality 数据质量检查
func (s *ServerV2) checkDataQuality(packet *Packet) {
	// TODO: 实现数据质量检查
	// 1. 范围检查
	// 2. 变化率检查
	// 3. 完整性检查
}

// aggregateData 数据聚合
func (s *ServerV2) aggregateData(packet *Packet) {
	// TODO: 实现数据聚合逻辑
	// 1. 分钟聚合
	// 2. 小时聚合
	// 3. 日聚合
}

// pushRealtimeData 推送实时数据
func (s *ServerV2) pushRealtimeData(packet *Packet) {
	// TODO: 实现WebSocket推送
}

// updateDeviceStatus 更新设备状态
func (s *ServerV2) updateDeviceStatus(packet *Packet) {
	updates := map[string]interface{}{
		"last_active_at": time.Now(),
		"is_connected":   true,
	}

	if err := s.db.Model(&models.DataSource{}).
		Where("device_id = ?", packet.MN).
		Updates(updates).Error; err != nil {
		s.logger.Error("Failed to update device status",
			zap.String("mn", packet.MN),
			zap.Error(err))
	}
}

// sendSuccessResponse 发送成功响应
func (s *ServerV2) sendSuccessResponse(conn net.Conn, request *Packet) {
	response := &Packet{
		QN:   GenerateQN(),
		ST:   ST_System,
		CN:   CN_Response,
		PW:   request.PW,
		MN:   request.MN,
		Flag: 0,
		CP:   fmt.Sprintf("QN=%s", request.QN),
	}

	s.sendResponse(conn, response)
}

// sendErrorResponse 发送错误响应
func (s *ServerV2) sendErrorResponse(conn net.Conn, request *Packet, err error) {
	if request == nil {
		return
	}

	response := &Packet{
		QN:   GenerateQN(),
		ST:   ST_System,
		CN:   CN_ExecuteResponse,
		PW:   request.PW,
		MN:   request.MN,
		Flag: 0,
		CP:   fmt.Sprintf("QN=%s;ExeRtn=%s;RtnInfo=%s", request.QN, ExeRtn_Failed, err.Error()),
	}

	s.sendResponse(conn, response)
}

// sendExecutionResponse 发送执行结果响应
func (s *ServerV2) sendExecutionResponse(conn net.Conn, request *Packet, result, info string) {
	response := &Packet{
		QN:   GenerateQN(),
		ST:   ST_System,
		CN:   CN_ExecuteResponse,
		PW:   request.PW,
		MN:   request.MN,
		Flag: 0,
		CP:   fmt.Sprintf("QN=%s;ExeRtn=%s;RtnInfo=%s", request.QN, result, info),
	}

	s.sendResponse(conn, response)
}

// sendResponse 发送响应
func (s *ServerV2) sendResponse(conn net.Conn, response *Packet) {
	data, err := s.parser.Build(response)
	if err != nil {
		s.logger.Error("Failed to build response", zap.Error(err))
		return
	}

	if _, err := conn.Write(data); err != nil {
		s.logger.Error("Failed to send response", zap.Error(err))
	} else {
		s.logger.Debug("Response sent",
			zap.String("cn", response.CN),
			zap.String("data", string(data)))
	}
}

// periodicTasks 定期任务
func (s *ServerV2) periodicTasks() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			// 清理不活跃连接
			s.cleanupInactiveConnections()

			// 更新统计信息
			s.logStats()
		}
	}
}

// cleanupInactiveConnections 清理不活跃连接
func (s *ServerV2) cleanupInactiveConnections() {
	// TODO: 实现连接清理逻辑
}

// logStats 记录统计信息
func (s *ServerV2) logStats() {
	s.stats.mu.RLock()
	defer s.stats.mu.RUnlock()

	s.logger.Info("Server statistics",
		zap.Uint64("total_packets", s.stats.TotalPackets),
		zap.Uint64("valid_packets", s.stats.ValidPackets),
		zap.Uint64("invalid_packets", s.stats.InvalidPackets),
		zap.Uint64("total_bytes", s.stats.TotalBytes),
		zap.Uint32("connections", s.stats.Connections),
		zap.Duration("uptime", time.Since(s.stats.StartTime)))
}

// Stop 停止服务器
func (s *ServerV2) Stop() error {
	s.cancel()

	if s.listener != nil {
		s.listener.Close()
	}

	// 关闭所有连接
	s.mu.Lock()
	for _, conn := range s.connections {
		conn.Close()
	}
	s.mu.Unlock()

	// 关闭通道
	close(s.dataChannel)
	close(s.alarmChannel)

	s.logger.Info("HJ212 server v2 stopped")
	return nil
}

// GetStats 获取统计信息
func (s *ServerV2) GetStats() map[string]interface{} {
	s.stats.mu.RLock()
	defer s.stats.mu.RUnlock()

	return map[string]interface{}{
		"total_packets":   s.stats.TotalPackets,
		"valid_packets":   s.stats.ValidPackets,
		"invalid_packets": s.stats.InvalidPackets,
		"total_bytes":     s.stats.TotalBytes,
		"connections":     s.stats.Connections,
		"uptime":          time.Since(s.stats.StartTime).String(),
		"last_packet":     s.stats.LastPacketTime,
	}
}

// GetConnectedDevices 获取连接的设备列表
func (s *ServerV2) GetConnectedDevices() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	devices := make([]string, 0, len(s.connections))
	for mn := range s.connections {
		devices = append(devices, mn)
	}
	return devices
}

// SendCommand 向设备发送命令
func (s *ServerV2) SendCommand(deviceMN string, packet *Packet) error {
	s.mu.RLock()
	conn, exists := s.connections[deviceMN]
	s.mu.RUnlock()

	if !exists {
		return fmt.Errorf("device %s not connected", deviceMN)
	}

	data, err := s.parser.Build(packet)
	if err != nil {
		return fmt.Errorf("failed to build packet: %w", err)
	}

	_, err = conn.Write(data)
	return err
}

// 辅助函数

func (s *ServerV2) getDataType(cn string) string {
	switch cn {
	case CN_GetRtdData:
		return "realtime"
	case CN_GetMinuteData:
		return "minute"
	case CN_GetHourData:
		return "hour"
	case CN_GetDayData:
		return "day"
	default:
		return "unknown"
	}
}

func findPacketEnd(data []byte) int {
	// 查找 \r\n 结束标记
	for i := 0; i < len(data)-1; i++ {
		if data[i] == '\r' && data[i+1] == '\n' {
			return i + 2
		}
	}
	return -1
}

func isTimeout(err error) bool {
	if netErr, ok := err.(net.Error); ok {
		return netErr.Timeout()
	}
	return false
}