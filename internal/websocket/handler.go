package websocket

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Handler WebSocket处理器
type Handler struct {
	hub      *Hub
	upgrader websocket.Upgrader
	logger   *zap.Logger
}

// NewHandler 创建新的WebSocket处理器
func NewHandler(hub *Hub, logger *zap.Logger) *Handler {
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			// 在生产环境中应该检查Origin
			return true
		},
	}

	return &Handler{
		hub:      hub,
		upgrader: upgrader,
		logger:   logger,
	}
}

// HandleWebSocket 处理WebSocket连接
func (h *Handler) HandleWebSocket(c *gin.Context) {
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.logger.Error("Failed to upgrade WebSocket connection", zap.Error(err))
		return
	}

	// 生成客户端ID
	clientID := uuid.New().String()

	// 获取查询参数中的频道订阅
	channels := c.QueryArray("channel")

	// 创建客户端
	client := NewClient(h.hub, conn, clientID)

	// 订阅指定频道
	for _, channel := range channels {
		client.subscribe(channel)
	}

	h.logger.Info("New WebSocket connection established",
		zap.String("client_id", clientID),
		zap.Strings("channels", channels),
		zap.String("remote_addr", c.ClientIP()))

	// 启动客户端
	client.Start()
}

// GetHub 获取WebSocket集线器
func (h *Handler) GetHub() *Hub {
	return h.hub
}