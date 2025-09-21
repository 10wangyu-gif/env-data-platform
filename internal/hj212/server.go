package hj212

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/env-data-platform/internal/config"
	"github.com/env-data-platform/internal/database"
	"github.com/env-data-platform/internal/models"
)

// WSHub WebSocket集线器接口
type WSHub interface {
	BroadcastHJ212Data(data *models.HJ212Data)
	BroadcastDeviceStatus(deviceID string, status map[string]interface{})
	BroadcastAlarm(event interface{})
}

// AlarmDetector 告警检测器接口
type AlarmDetector interface {
	CheckData(data *models.HJ212Data)
}

// Server HJ212协议服务器
type Server struct {
	config        *config.Config
	logger        *zap.Logger
	listener      net.Listener
	clients       sync.Map // 存储客户端连接
	ctx           context.Context
	cancel        context.CancelFunc
	parser        *Parser        // 使用新的解析器
	wsHub         WSHub          // WebSocket集线器接口
	alarmDetector AlarmDetector  // 告警检测器接口
}

// Client 客户端连接信息
type Client struct {
	Conn       net.Conn
	MN         string    // 设备唯一标识
	LastActive time.Time // 最后活跃时间
}

// NewServer 创建HJ212服务器
func NewServer(cfg *config.Config, logger *zap.Logger, wsHub WSHub, alarmDetector AlarmDetector) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	parser := NewParser("HJ212-2017") // 创建解析器实例

	return &Server{
		config:        cfg,
		logger:        logger,
		ctx:           ctx,
		cancel:        cancel,
		parser:        parser,
		wsHub:         wsHub,
		alarmDetector: alarmDetector,
	}
}

// Start 启动服务器
func (s *Server) Start() error {
	if !s.config.HJ212.Enabled {
		s.logger.Info("HJ212 server is disabled")
		return nil
	}

	addr := fmt.Sprintf(":%d", s.config.HJ212.TCPPort)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %v", addr, err)
	}

	s.listener = listener
	s.logger.Info("HJ212 server started", zap.String("address", addr))

	// 启动客户端清理协程
	go s.cleanupClients()

	// 接受连接
	for {
		select {
		case <-s.ctx.Done():
			return s.ctx.Err()
		default:
			conn, err := listener.Accept()
			if err != nil {
				if s.ctx.Err() != nil {
					return nil // 服务器正在关闭
				}
				s.logger.Error("Failed to accept connection", zap.Error(err))
				continue
			}

			// 处理新连接
			go s.handleConnection(conn)
		}
	}
}

// Stop 停止服务器
func (s *Server) Stop() error {
	s.cancel()

	if s.listener != nil {
		if err := s.listener.Close(); err != nil {
			s.logger.Error("Failed to close listener", zap.Error(err))
		}
	}

	// 关闭所有客户端连接
	s.clients.Range(func(key, value interface{}) bool {
		if client, ok := value.(*Client); ok {
			client.Conn.Close()
		}
		return true
	})

	s.logger.Info("HJ212 server stopped")
	return nil
}

// handleConnection 处理客户端连接
func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()

	clientAddr := conn.RemoteAddr().String()
	s.logger.Info("New HJ212 client connected", zap.String("address", clientAddr))

	// 设置读取超时
	conn.SetReadDeadline(time.Now().Add(s.config.HJ212.Timeout))

	buffer := make([]byte, s.config.HJ212.BufferSize)

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			// 读取数据
			n, err := conn.Read(buffer)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					s.logger.Debug("Client connection timeout", zap.String("address", clientAddr))
				} else {
					s.logger.Debug("Client disconnected", zap.String("address", clientAddr), zap.Error(err))
				}
				return
			}

			if n > 0 {
				// 重置读取超时
				conn.SetReadDeadline(time.Now().Add(s.config.HJ212.Timeout))

				// 处理接收到的数据
				data := string(buffer[:n])
				s.handleMessage(conn, clientAddr, data)
			}
		}
	}
}

// handleMessage 处理HJ212消息
func (s *Server) handleMessage(conn net.Conn, clientAddr, data string) {
	s.logger.Debug("Received HJ212 data",
		zap.String("address", clientAddr),
		zap.String("data", data))

	// 使用新解析器解析HJ212消息
	packet, err := s.parser.Parse([]byte(data))
	if err != nil {
		s.logger.Warn("Failed to parse HJ212 packet",
			zap.String("address", clientAddr),
			zap.Error(err),
			zap.String("data", data))
		return
	}

	// 验证消息有效性
	if err := s.parser.ValidatePacket(packet); err != nil {
		s.logger.Warn("Invalid HJ212 packet",
			zap.String("address", clientAddr),
			zap.Error(err),
			zap.Any("packet", packet))
		return
	}

	// 更新客户端信息
	if packet.MN != "" {
		client := &Client{
			Conn:       conn,
			MN:         packet.MN,
			LastActive: time.Now(),
		}
		s.clients.Store(packet.MN, client)
	}

	// 处理不同类型的消息
	switch packet.CN {
	case "2011", "2051", "2061", "2031": // 监测数据
		s.handleMonitoringData(conn, clientAddr, packet)
	case "2021": // 报警数据
		s.handleAlarmData(conn, clientAddr, packet)
	case "9011": // 心跳包
		s.handleHeartbeat(conn, clientAddr, packet)
	case "9012", "3019": // 设备信息
		s.handleDeviceInfo(conn, clientAddr, packet)
	default:
		s.logger.Debug("Unknown command code",
			zap.String("address", clientAddr),
			zap.String("cn", packet.CN))
	}
}

// handleMonitoringData 处理监测数据
func (s *Server) handleMonitoringData(conn net.Conn, clientAddr string, packet *Packet) {
	s.logger.Info("Received monitoring data",
		zap.String("address", clientAddr),
		zap.String("mn", packet.MN),
		zap.String("cn", packet.CN),
		zap.Time("data_time", packet.DataTime))

	// 准备解析后的数据
	parsedData := make(models.JSONMap)

	// 添加基本信息
	parsedData["data_time"] = packet.DataTime.Format("2006-01-02 15:04:05")
	parsedData["system_code"] = packet.ST
	parsedData["qn"] = packet.QN

	// 转换因子数据为简单的map格式
	if packet.Factors != nil {
		factors := make(map[string]interface{})
		for code, factor := range packet.Factors {
			factors[code] = map[string]interface{}{
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
		parsedData["factors"] = factors
	}

	// 保存到数据库
	currentTime := time.Now()
	hj212Data := models.HJ212Data{
		DeviceID:     packet.MN,
		CommandCode:  packet.CN,
		DataType:     s.getDataTypeByCN(packet.CN),
		RawData:      string(packet.RawData),
		ParsedData:   parsedData,
		ReceivedFrom: clientAddr,
		ReceivedAt:   currentTime,
		QualityLevel: "normal",
		IsValid:      true,
		CreatedDate:  currentTime.Format("2006-01-02"),
		CreatedHour:  currentTime.Hour(),
	}

	if err := database.DB.Create(&hj212Data).Error; err != nil {
		s.logger.Error("Failed to save HJ212 data",
			zap.Error(err),
			zap.String("mn", packet.MN))
	} else {
		// 广播新数据到WebSocket客户端
		if s.wsHub != nil {
			s.wsHub.BroadcastHJ212Data(&hj212Data)
		}

		// 进行告警检测
		if s.alarmDetector != nil {
			s.alarmDetector.CheckData(&hj212Data)
		}

		s.logger.Debug("HJ212 data saved and broadcasted",
			zap.String("device_id", hj212Data.DeviceID),
			zap.String("command_code", hj212Data.CommandCode))
	}

	// 发送响应确认
	response := s.buildResponse(packet, ExeRtn_Success)
	if _, err := conn.Write(response); err != nil {
		s.logger.Error("Failed to send response",
			zap.Error(err),
			zap.String("address", clientAddr))
	}
}

// handleAlarmData 处理报警数据
func (s *Server) handleAlarmData(conn net.Conn, clientAddr string, packet *Packet) {
	s.logger.Warn("Received alarm data",
		zap.String("address", clientAddr),
		zap.String("mn", packet.MN),
		zap.Any("alarm_data", packet.AlarmData))

	// 保存报警数据
	alarmData := models.HJ212AlarmData{
		DeviceID:     packet.MN,
		AlarmType:    packet.AlarmData.AlarmType,
		AlarmLevel:   "high", // 默认高级别
		AlarmDesc:    packet.AlarmData.AlarmType,
		RawData:      string(packet.RawData),
		ReceivedFrom: clientAddr,
		ReceivedAt:   time.Now(),
		Status:       "active",
	}

	if err := database.DB.Create(&alarmData).Error; err != nil {
		s.logger.Error("Failed to save alarm data",
			zap.Error(err),
			zap.String("mn", packet.MN))
	}

	// 发送响应确认
	response := s.buildResponse(packet, ExeRtn_Success)
	if _, err := conn.Write(response); err != nil {
		s.logger.Error("Failed to send response",
			zap.Error(err),
			zap.String("address", clientAddr))
	}
}

// handleHeartbeat 处理心跳包
func (s *Server) handleHeartbeat(conn net.Conn, clientAddr string, packet *Packet) {
	s.logger.Debug("Received heartbeat",
		zap.String("address", clientAddr),
		zap.String("mn", packet.MN))

	// 更新设备最后在线时间
	lastActiveTime := time.Now()
	if err := database.DB.Model(&models.DataSource{}).
		Where("device_id = ?", packet.MN).
		Update("last_active_at", lastActiveTime).Error; err != nil {
		s.logger.Debug("Failed to update device last active time", zap.Error(err))
	} else {
		// 广播设备状态更新
		if s.wsHub != nil {
			s.wsHub.BroadcastDeviceStatus(packet.MN, map[string]interface{}{
				"status":         "online",
				"last_active_at": lastActiveTime,
				"command_code":   packet.CN,
			})
		}
	}

	// 发送心跳响应
	response := s.buildResponse(packet, ExeRtn_Success)
	if _, err := conn.Write(response); err != nil {
		s.logger.Error("Failed to send heartbeat response",
			zap.Error(err),
			zap.String("address", clientAddr))
	}
}

// handleDeviceInfo 处理设备信息
func (s *Server) handleDeviceInfo(conn net.Conn, clientAddr string, packet *Packet) {
	s.logger.Info("Received device info",
		zap.String("address", clientAddr),
		zap.String("mn", packet.MN),
		zap.Any("device_info", packet.DataArea))

	// 查找或创建数据源记录
	var dataSource models.DataSource
	err := database.DB.Where("device_id = ?", packet.MN).First(&dataSource).Error

	currentTime := time.Now()
	if err != nil {
		// 序列化配置数据
		configBytes, _ := json.Marshal(packet.DataArea)
		// 创建新的数据源记录
		dataSource = models.DataSource{
			Name:         fmt.Sprintf("HJ212设备-%s", packet.MN),
			Type:         "hj212",
			DeviceID:     packet.MN,
			Status:       "active",
			Config:       string(configBytes),
			LastActiveAt: &currentTime,
			IsConnected:  true,
		}

		if err := database.DB.Create(&dataSource).Error; err != nil {
			s.logger.Error("Failed to create data source",
				zap.Error(err),
				zap.String("mn", packet.MN))
		}
	} else {
		// 序列化配置数据并更新现有记录
		configBytes, _ := json.Marshal(packet.DataArea)
		dataSource.Config = string(configBytes)
		dataSource.LastActiveAt = &currentTime
		dataSource.IsConnected = true
		if err := database.DB.Save(&dataSource).Error; err != nil {
			s.logger.Error("Failed to update data source",
				zap.Error(err),
				zap.String("mn", packet.MN))
		}
	}

	// 发送响应确认
	response := s.buildResponse(packet, ExeRtn_Success)
	if _, err := conn.Write(response); err != nil {
		s.logger.Error("Failed to send response",
			zap.Error(err),
			zap.String("address", clientAddr))
	}
}

// cleanupClients 清理不活跃的客户端
func (s *Server) cleanupClients() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			now := time.Now()
			s.clients.Range(func(key, value interface{}) bool {
				if client, ok := value.(*Client); ok {
					// 如果客户端超过10分钟没有活动，则移除
					if now.Sub(client.LastActive) > 10*time.Minute {
						s.logger.Debug("Removing inactive client", zap.String("mn", client.MN))
						client.Conn.Close()
						s.clients.Delete(key)
					}
				}
				return true
			})
		}
	}
}

// GetConnectedDevices 获取当前连接的设备列表
func (s *Server) GetConnectedDevices() []string {
	var devices []string
	s.clients.Range(func(key, value interface{}) bool {
		if mn, ok := key.(string); ok {
			devices = append(devices, mn)
		}
		return true
	})
	return devices
}

// SendCommand 向设备发送命令
func (s *Server) SendCommand(deviceID, command string) error {
	if client, ok := s.clients.Load(deviceID); ok {
		if c, ok := client.(*Client); ok {
			_, err := c.Conn.Write([]byte(command))
			return err
		}
	}
	return fmt.Errorf("device %s not connected", deviceID)
}

// getDataTypeByCN 根据命令编码获取数据类型
func (s *Server) getDataTypeByCN(cn string) string {
	switch cn {
	case CN_GetRtdData:
		return "实时数据"
	case CN_GetMinuteData:
		return "分钟数据"
	case CN_GetHourData:
		return "小时数据"
	case CN_GetDayData:
		return "日数据"
	case CN_GetDeviceStatus:
		return "设备状态"
	default:
		return "未知类型"
	}
}

// buildResponse 构建响应消息
func (s *Server) buildResponse(originalPacket *Packet, execResult string) []byte {
	responsePacket := &Packet{
		QN:   GenerateQN(),
		ST:   originalPacket.ST,
		CN:   CN_Response, // 9011
		PW:   originalPacket.PW,
		MN:   originalPacket.MN,
		Flag: Flag_Confirm,
		CP:   fmt.Sprintf("&&ExeRtn=%s&&", execResult),
	}

	response, err := s.parser.Build(responsePacket)
	if err != nil {
		s.logger.Error("Failed to build response", zap.Error(err))
		// 返回一个基本的响应
		basic := fmt.Sprintf("##0050QN=%s;ST=%s;CN=9011;PW=%s;MN=%s;Flag=1;CP=&&ExeRtn=%s&&0000\r\n",
			GenerateQN(), originalPacket.ST, originalPacket.PW, originalPacket.MN, execResult)
		return []byte(basic)
	}
	return response
}