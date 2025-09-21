package websocket

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"github.com/env-data-platform/internal/models"
)

// Message WebSocket消息结构
type Message struct {
	Type      string      `json:"type"`       // 消息类型
	Data      interface{} `json:"data"`       // 消息数据
	Timestamp time.Time   `json:"timestamp"`  // 时间戳
	Channel   string      `json:"channel"`    // 频道
}

// Client WebSocket客户端
type Client struct {
	conn     *websocket.Conn
	send     chan Message
	hub      *Hub
	id       string
	channels map[string]bool // 订阅的频道
	mu       sync.RWMutex
}

// Hub WebSocket集线器
type Hub struct {
	clients    map[*Client]bool
	register   chan *Client
	unregister chan *Client
	broadcast  chan Message
	logger     *zap.Logger
	mu         sync.RWMutex
}

// NewHub 创建新的WebSocket集线器
func NewHub(logger *zap.Logger) *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan Message, 256),
		logger:     logger,
	}
}

// Run 启动WebSocket集线器
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			h.logger.Info("WebSocket client connected",
				zap.String("client_id", client.id))

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
			h.logger.Info("WebSocket client disconnected",
				zap.String("client_id", client.id))

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				// 检查客户端是否订阅了该频道
				if message.Channel == "" || client.isSubscribed(message.Channel) {
					select {
					case client.send <- message:
					default:
						delete(h.clients, client)
						close(client.send)
					}
				}
			}
			h.mu.RUnlock()
		}
	}
}

// BroadcastHJ212Data 广播HJ212数据
func (h *Hub) BroadcastHJ212Data(data *models.HJ212Data) {
	message := Message{
		Type:      "hj212_data",
		Data:      data,
		Timestamp: time.Now(),
		Channel:   "hj212",
	}

	select {
	case h.broadcast <- message:
	default:
		h.logger.Warn("Broadcast channel is full, dropping message")
	}
}

// BroadcastStats 广播统计信息
func (h *Hub) BroadcastStats(stats interface{}) {
	message := Message{
		Type:      "stats",
		Data:      stats,
		Timestamp: time.Now(),
		Channel:   "stats",
	}

	select {
	case h.broadcast <- message:
	default:
		h.logger.Warn("Broadcast channel is full, dropping message")
	}
}

// BroadcastDeviceStatus 广播设备状态
func (h *Hub) BroadcastDeviceStatus(deviceID string, status map[string]interface{}) {
	message := Message{
		Type:      "device_status",
		Data:      map[string]interface{}{
			"device_id": deviceID,
			"status":    status,
		},
		Timestamp: time.Now(),
		Channel:   "device_status",
	}

	select {
	case h.broadcast <- message:
	default:
		h.logger.Warn("Broadcast channel is full, dropping message")
	}
}

// BroadcastAlarm 广播告警事件
func (h *Hub) BroadcastAlarm(event interface{}) {
	message := Message{
		Type:      "alarm",
		Data:      event,
		Timestamp: time.Now(),
		Channel:   "alarm",
	}

	select {
	case h.broadcast <- message:
	default:
		h.logger.Warn("Broadcast channel is full, dropping message")
	}
}

// GetConnectedClientsCount 获取连接的客户端数量
func (h *Hub) GetConnectedClientsCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// isSubscribed 检查客户端是否订阅了指定频道
func (c *Client) isSubscribed(channel string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if len(c.channels) == 0 {
		return true // 如果没有指定频道，接收所有消息
	}
	return c.channels[channel]
}

// subscribe 订阅频道
func (c *Client) subscribe(channel string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.channels == nil {
		c.channels = make(map[string]bool)
	}
	c.channels[channel] = true
}

// unsubscribe 取消订阅频道
func (c *Client) unsubscribe(channel string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.channels, channel)
}

// writePump 向WebSocket连接写入消息
func (c *Client) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteJSON(message); err != nil {
				c.hub.logger.Error("Failed to write message",
					zap.Error(err),
					zap.String("client_id", c.id))
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// readPump 从WebSocket连接读取消息
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(512)
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		var msg map[string]interface{}
		err := c.conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.hub.logger.Error("WebSocket error", zap.Error(err))
			}
			break
		}

		// 处理客户端消息
		c.handleMessage(msg)
	}
}

// handleMessage 处理客户端消息
func (c *Client) handleMessage(msg map[string]interface{}) {
	msgType, ok := msg["type"].(string)
	if !ok {
		return
	}

	switch msgType {
	case "subscribe":
		if channel, ok := msg["channel"].(string); ok {
			c.subscribe(channel)
			c.hub.logger.Debug("Client subscribed to channel",
				zap.String("client_id", c.id),
				zap.String("channel", channel))
		}
	case "unsubscribe":
		if channel, ok := msg["channel"].(string); ok {
			c.unsubscribe(channel)
			c.hub.logger.Debug("Client unsubscribed from channel",
				zap.String("client_id", c.id),
				zap.String("channel", channel))
		}
	}
}

// NewClient 创建新的WebSocket客户端
func NewClient(hub *Hub, conn *websocket.Conn, clientID string) *Client {
	return &Client{
		conn:     conn,
		send:     make(chan Message, 256),
		hub:      hub,
		id:       clientID,
		channels: make(map[string]bool),
	}
}

// Start 启动客户端
func (c *Client) Start() {
	c.hub.register <- c
	go c.writePump()
	go c.readPump()
}