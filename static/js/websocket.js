/**
 * 环保数据集成平台 - WebSocket客户端
 * 处理实时数据推送、设备状态更新、告警通知等
 */

class WebSocketClient {
    constructor(authManager) {
        this.auth = authManager;
        this.url = 'ws://localhost:8888/ws';
        this.ws = null;
        this.subscribers = new Map();
        this.reconnectAttempts = 0;
        this.maxReconnectAttempts = 5;
        this.reconnectInterval = 3000; // 3秒
        this.heartbeatInterval = 30000; // 30秒心跳
        this.heartbeatTimer = null;
        this.isConnected = false;
        this.isReconnecting = false;

        // 绑定方法上下文
        this.onOpen = this.onOpen.bind(this);
        this.onMessage = this.onMessage.bind(this);
        this.onClose = this.onClose.bind(this);
        this.onError = this.onError.bind(this);
    }

    /**
     * 连接WebSocket
     */
    connect(channels = []) {
        if (this.ws && this.ws.readyState === WebSocket.OPEN) {
            console.log('WebSocket已连接');
            return;
        }

        if (!this.auth.isLoggedIn()) {
            console.error('用户未登录，无法建立WebSocket连接');
            return;
        }

        try {
            // 构建连接URL，包含认证信息和频道订阅
            let wsUrl = this.url;
            const params = new URLSearchParams();

            if (this.auth.token) {
                params.append('token', this.auth.token);
            }

            if (channels.length > 0) {
                params.append('channels', channels.join(','));
            }

            if (params.toString()) {
                wsUrl += '?' + params.toString();
            }

            console.log('正在连接WebSocket...', wsUrl);
            this.ws = new WebSocket(wsUrl);

            // 设置事件监听器
            this.ws.onopen = this.onOpen;
            this.ws.onmessage = this.onMessage;
            this.ws.onclose = this.onClose;
            this.ws.onerror = this.onError;

        } catch (error) {
            console.error('WebSocket连接失败:', error);
            this.scheduleReconnect();
        }
    }

    /**
     * 连接成功处理
     */
    onOpen(event) {
        console.log('WebSocket连接成功');
        this.isConnected = true;
        this.isReconnecting = false;
        this.reconnectAttempts = 0;

        // 启动心跳
        this.startHeartbeat();

        // 触发连接成功事件
        this.emit('connected', { timestamp: new Date() });

        // 发送认证信息（如果需要）
        if (this.auth.token) {
            this.send({
                type: 'auth',
                token: this.auth.token
            });
        }
    }

    /**
     * 消息处理
     */
    onMessage(event) {
        try {
            const data = JSON.parse(event.data);
            console.log('收到WebSocket消息:', data);

            // 处理不同类型的消息
            switch (data.type) {
                case 'auth_success':
                    console.log('WebSocket认证成功');
                    this.emit('auth_success', data);
                    break;

                case 'auth_failed':
                    console.error('WebSocket认证失败:', data.message);
                    this.emit('auth_failed', data);
                    this.disconnect();
                    break;

                case 'hj212_data':
                    this.emit('hj212', data.data);
                    break;

                case 'device_status':
                    this.emit('device_status', data.data);
                    break;

                case 'alarm':
                    this.emit('alarm', data.data);
                    this.handleAlarmNotification(data.data);
                    break;

                case 'etl_status':
                    this.emit('etl_status', data.data);
                    break;

                case 'quality_report':
                    this.emit('quality_report', data.data);
                    break;

                case 'system_stats':
                    this.emit('system_stats', data.data);
                    break;

                case 'heartbeat':
                    // 服务端心跳响应
                    break;

                default:
                    console.log('未知消息类型:', data.type);
                    this.emit('unknown', data);
            }

        } catch (error) {
            console.error('解析WebSocket消息失败:', error, event.data);
        }
    }

    /**
     * 连接关闭处理
     */
    onClose(event) {
        console.log('WebSocket连接关闭:', event.code, event.reason);
        this.isConnected = false;
        this.stopHeartbeat();

        // 触发断开连接事件
        this.emit('disconnected', {
            code: event.code,
            reason: event.reason,
            timestamp: new Date()
        });

        // 如果不是手动关闭，尝试重连
        if (event.code !== 1000 && !this.isReconnecting) {
            this.scheduleReconnect();
        }
    }

    /**
     * 错误处理
     */
    onError(error) {
        console.error('WebSocket错误:', error);
        this.emit('error', { error, timestamp: new Date() });
    }

    /**
     * 发送消息
     */
    send(data) {
        if (this.ws && this.ws.readyState === WebSocket.OPEN) {
            this.ws.send(JSON.stringify(data));
            return true;
        } else {
            console.error('WebSocket未连接，无法发送消息');
            return false;
        }
    }

    /**
     * 订阅频道
     */
    subscribe(channel, callback) {
        if (!this.subscribers.has(channel)) {
            this.subscribers.set(channel, new Set());
        }
        this.subscribers.get(channel).add(callback);

        // 如果已连接，发送订阅请求
        if (this.isConnected) {
            this.send({
                type: 'subscribe',
                channel: channel
            });
        }
    }

    /**
     * 取消订阅
     */
    unsubscribe(channel, callback) {
        if (this.subscribers.has(channel)) {
            this.subscribers.get(channel).delete(callback);

            // 如果没有回调函数了，删除整个频道
            if (this.subscribers.get(channel).size === 0) {
                this.subscribers.delete(channel);

                // 发送取消订阅请求
                if (this.isConnected) {
                    this.send({
                        type: 'unsubscribe',
                        channel: channel
                    });
                }
            }
        }
    }

    /**
     * 触发事件
     */
    emit(event, data) {
        if (this.subscribers.has(event)) {
            this.subscribers.get(event).forEach(callback => {
                try {
                    callback(data);
                } catch (error) {
                    console.error('WebSocket事件回调执行失败:', error);
                }
            });
        }
    }

    /**
     * 启动心跳
     */
    startHeartbeat() {
        this.stopHeartbeat();
        this.heartbeatTimer = setInterval(() => {
            if (this.isConnected) {
                this.send({ type: 'heartbeat', timestamp: Date.now() });
            }
        }, this.heartbeatInterval);
    }

    /**
     * 停止心跳
     */
    stopHeartbeat() {
        if (this.heartbeatTimer) {
            clearInterval(this.heartbeatTimer);
            this.heartbeatTimer = null;
        }
    }

    /**
     * 安排重连
     */
    scheduleReconnect() {
        if (this.reconnectAttempts >= this.maxReconnectAttempts) {
            console.error('达到最大重连次数，停止重连');
            this.emit('max_reconnect_attempts', { attempts: this.reconnectAttempts });
            return;
        }

        if (this.isReconnecting) {
            return;
        }

        this.isReconnecting = true;
        this.reconnectAttempts++;

        console.log(`${this.reconnectInterval / 1000}秒后进行第${this.reconnectAttempts}次重连...`);

        setTimeout(() => {
            if (this.isReconnecting) {
                this.connect();
            }
        }, this.reconnectInterval);
    }

    /**
     * 处理告警通知
     */
    handleAlarmNotification(alarmData) {
        // 显示桌面通知
        if ('Notification' in window && Notification.permission === 'granted') {
            new Notification('环保监测告警', {
                body: `${alarmData.message || '检测到异常数据'}`,
                icon: '/static/favicon.ico',
                tag: 'env-alarm'
            });
        }

        // 显示页面内通知
        this.showPageNotification(alarmData);
    }

    /**
     * 显示页面内通知
     */
    showPageNotification(alarmData) {
        // 创建通知元素
        const notification = document.createElement('div');
        notification.className = 'fixed top-4 right-4 bg-red-500 text-white p-4 rounded-lg shadow-lg z-50 max-w-sm';
        notification.innerHTML = `
            <div class="flex items-start">
                <i class="fas fa-exclamation-triangle text-yellow-300 mr-3 mt-1"></i>
                <div class="flex-1">
                    <h4 class="font-semibold">环保监测告警</h4>
                    <p class="text-sm mt-1">${alarmData.message || '检测到异常数据'}</p>
                    <p class="text-xs mt-1 opacity-75">
                        ${alarmData.timestamp ? new Date(alarmData.timestamp).toLocaleString() : ''}
                    </p>
                </div>
                <button onclick="this.parentElement.parentElement.remove()" class="text-white hover:text-gray-300 ml-2">
                    <i class="fas fa-times"></i>
                </button>
            </div>
        `;

        // 添加到页面
        document.body.appendChild(notification);

        // 5秒后自动消失
        setTimeout(() => {
            if (notification.parentElement) {
                notification.remove();
            }
        }, 5000);
    }

    /**
     * 请求桌面通知权限
     */
    requestNotificationPermission() {
        if ('Notification' in window && Notification.permission === 'default') {
            Notification.requestPermission().then(permission => {
                console.log('桌面通知权限:', permission);
            });
        }
    }

    /**
     * 断开连接
     */
    disconnect() {
        this.isReconnecting = false;
        this.stopHeartbeat();

        if (this.ws) {
            this.ws.close(1000, '手动关闭');
            this.ws = null;
        }

        this.isConnected = false;
        console.log('WebSocket连接已断开');
    }

    /**
     * 获取连接状态
     */
    getConnectionStatus() {
        return {
            isConnected: this.isConnected,
            isReconnecting: this.isReconnecting,
            reconnectAttempts: this.reconnectAttempts,
            subscribedChannels: Array.from(this.subscribers.keys())
        };
    }
}

/**
 * WebSocket管理器
 * 全局单例，管理WebSocket连接
 */
class WebSocketManager {
    constructor() {
        this.client = null;
        this.authManager = null;
    }

    /**
     * 初始化WebSocket连接
     */
    init(authManager, channels = ['hj212', 'device_status', 'alarm']) {
        this.authManager = authManager;

        if (!authManager.isLoggedIn()) {
            console.log('用户未登录，跳过WebSocket初始化');
            return;
        }

        this.client = new WebSocketClient(authManager);

        // 请求桌面通知权限
        this.client.requestNotificationPermission();

        // 连接WebSocket
        this.client.connect(channels);

        return this.client;
    }

    /**
     * 获取WebSocket客户端
     */
    getClient() {
        return this.client;
    }

    /**
     * 销毁WebSocket连接
     */
    destroy() {
        if (this.client) {
            this.client.disconnect();
            this.client = null;
        }
    }
}

// 创建全局WebSocket管理器实例
const wsManager = new WebSocketManager();

// 导出到全局对象
window.WebSocketClient = WebSocketClient;
window.WebSocketManager = WebSocketManager;
window.wsManager = wsManager;