/**
 * 环保数据集成平台 - 仪表板API客户端
 * 处理仪表板数据的获取和实时更新
 */

class DashboardAPI {
    constructor(authManager, apiClient) {
        this.auth = authManager;
        this.api = apiClient;
        this.baseURL = API_CONFIG.ENDPOINTS.DASHBOARD.BASE;
        this.updateInterval = null;
        this.listeners = {};
    }

    /**
     * 获取仪表板概览数据
     */
    async getOverviewData() {
        try {
            const response = await this.api.get(API_CONFIG.ENDPOINTS.DASHBOARD.OVERVIEW);
            return {
                success: true,
                data: response
            };
        } catch (error) {
            console.error('获取仪表板概览数据失败:', error);
            return {
                success: false,
                message: error.message || '获取概览数据失败'
            };
        }
    }

    /**
     * 获取实时数据
     */
    async getRealTimeData() {
        try {
            const response = await this.api.get(API_CONFIG.ENDPOINTS.DASHBOARD.REALTIME);
            return {
                success: true,
                data: response
            };
        } catch (error) {
            console.error('获取实时数据失败:', error);
            return {
                success: false,
                message: error.message || '获取实时数据失败'
            };
        }
    }

    /**
     * 获取图表数据
     */
    async getChartData(period = 'today') {
        try {
            const response = await this.api.get(API_CONFIG.ENDPOINTS.DASHBOARD.CHARTS, { period });
            return {
                success: true,
                data: response
            };
        } catch (error) {
            console.error('获取图表数据失败:', error);
            return {
                success: false,
                message: error.message || '获取图表数据失败'
            };
        }
    }

    /**
     * 启动实时数据更新
     */
    startRealTimeUpdates(interval = 30000) { // 默认30秒更新一次
        if (this.updateInterval) {
            this.stopRealTimeUpdates();
        }

        this.updateInterval = setInterval(async () => {
            const result = await this.getRealTimeData();
            if (result.success) {
                this.emit('realTimeUpdate', result.data);
            } else {
                console.error('实时数据更新失败:', result.message);
            }
        }, interval);

        console.log('实时数据更新已启动');
    }

    /**
     * 停止实时数据更新
     */
    stopRealTimeUpdates() {
        if (this.updateInterval) {
            clearInterval(this.updateInterval);
            this.updateInterval = null;
            console.log('实时数据更新已停止');
        }
    }

    /**
     * 事件监听器管理
     */
    on(event, callback) {
        if (!this.listeners[event]) {
            this.listeners[event] = [];
        }
        this.listeners[event].push(callback);
    }

    off(event, callback) {
        if (this.listeners[event]) {
            const index = this.listeners[event].indexOf(callback);
            if (index > -1) {
                this.listeners[event].splice(index, 1);
            }
        }
    }

    emit(event, data) {
        if (this.listeners[event]) {
            this.listeners[event].forEach(callback => {
                try {
                    callback(data);
                } catch (error) {
                    console.error('事件回调执行错误:', error);
                }
            });
        }
    }

    /**
     * 初始化仪表板数据
     */
    async initializeDashboard() {
        try {
            console.log('正在初始化仪表板数据...');

            // 获取概览数据
            const overviewResult = await this.getOverviewData();
            if (overviewResult.success) {
                this.emit('overviewDataLoaded', overviewResult.data);
            } else {
                throw new Error(overviewResult.message);
            }

            // 启动实时更新
            this.startRealTimeUpdates();

            console.log('仪表板数据初始化完成');
            return true;

        } catch (error) {
            console.error('仪表板初始化失败:', error);
            this.emit('initializationError', error.message);
            return false;
        }
    }

    /**
     * 刷新所有数据
     */
    async refreshAllData() {
        try {
            console.log('正在刷新仪表板数据...');

            const [overviewResult, chartResult] = await Promise.all([
                this.getOverviewData(),
                this.getChartData()
            ]);

            if (overviewResult.success) {
                this.emit('overviewDataLoaded', overviewResult.data);
            }

            if (chartResult.success) {
                this.emit('chartDataLoaded', chartResult.data);
            }

            console.log('仪表板数据刷新完成');
            return true;

        } catch (error) {
            console.error('数据刷新失败:', error);
            this.emit('refreshError', error.message);
            return false;
        }
    }

    /**
     * 获取指定时间周期的图表数据
     */
    async updateChartPeriod(period) {
        const result = await this.getChartData(period);
        if (result.success) {
            this.emit('chartDataLoaded', result.data);
        }
        return result;
    }

    /**
     * 销毁实例
     */
    destroy() {
        this.stopRealTimeUpdates();
        this.listeners = {};
        console.log('仪表板API客户端已销毁');
    }
}

/**
 * 仪表板数据渲染器
 */
class DashboardRenderer {
    constructor(dashboardAPI) {
        this.api = dashboardAPI;
        this.charts = {};
        this.setupEventListeners();
    }

    setupEventListeners() {
        this.api.on('overviewDataLoaded', (data) => {
            this.renderOverviewData(data);
        });

        this.api.on('realTimeUpdate', (data) => {
            this.updateRealTimeData(data);
        });

        this.api.on('chartDataLoaded', (data) => {
            this.updateCharts(data);
        });
    }

    /**
     * 渲染概览数据
     */
    renderOverviewData(data) {
        try {
            // 更新核心统计
            this.updateCoreStats(data.core_stats);

            // 更新环境数据
            this.updateEnvironmentData(data.environment_data);

            // 更新ETL任务状态
            this.updateETLStatus(data.etl_task_status);

            // 更新告警信息
            this.updateAlerts(data.latest_alerts);

            // 初始化图表
            this.initializeCharts(data.chart_data);

            console.log('概览数据渲染完成');
        } catch (error) {
            console.error('概览数据渲染失败:', error);
        }
    }

    /**
     * 更新核心统计数据
     */
    updateCoreStats(coreStats) {
        // 数据源连接数
        this.updateStatCard('.data-source-stat', {
            value: coreStats.data_source_connections.current,
            change: coreStats.data_source_connections.change,
            trend: coreStats.data_source_connections.trend
        });

        // 实时数据流
        this.updateStatCard('.data-flow-stat', {
            value: coreStats.real_time_data_flow.current.toFixed(1) + 'K',
            change: coreStats.real_time_data_flow.change_percent,
            trend: coreStats.real_time_data_flow.trend,
            isPercentage: true
        });

        // API调用
        this.updateStatCard('.api-call-stat', {
            value: (coreStats.api_calls_today.current / 1000).toFixed(1) + 'K',
            change: coreStats.api_calls_today.change_percent,
            trend: coreStats.api_calls_today.trend,
            isPercentage: true
        });

        // 系统健康度
        this.updateStatCard('.system-health-stat', {
            value: coreStats.system_health.current.toFixed(1) + '%',
            status: coreStats.system_health.status
        });
    }

    /**
     * 更新统计卡片
     */
    updateStatCard(selector, data) {
        const card = document.querySelector(selector);
        if (!card) return;

        const valueElement = card.querySelector('.doubao-stat-value');
        const changeElement = card.querySelector('.doubao-stat-change');

        if (valueElement) {
            valueElement.textContent = data.value;
        }

        if (changeElement && (data.change !== undefined || data.status)) {
            if (data.status) {
                changeElement.innerHTML = `<i class="fas fa-check text-xs" style="margin-right: var(--doubao-space-4);"></i>运行正常`;
                changeElement.className = 'doubao-stat-change neutral';
            } else {
                const sign = data.change >= 0 ? '+' : '';
                const suffix = data.isPercentage ? '%' : '';
                const icon = data.trend === 'up' ? 'fa-arrow-up' : data.trend === 'down' ? 'fa-arrow-down' : 'fa-minus';
                const trendClass = data.trend === 'up' ? 'positive' : data.trend === 'down' ? 'negative' : 'neutral';

                changeElement.innerHTML = `<i class="fas ${icon} text-xs" style="margin-right: var(--doubao-space-4);"></i>${sign}${data.change}${suffix} 较昨日`;
                changeElement.className = `doubao-stat-change ${trendClass}`;
            }
        }
    }

    /**
     * 更新实时数据
     */
    updateRealTimeData(data) {
        this.updateCoreStats(data);
    }

    /**
     * 更新环境数据
     */
    updateEnvironmentData(environmentData) {
        // 空气质量数据
        this.updateEnvironmentMetric('.air-pm25', environmentData.air_quality.pm25);
        this.updateEnvironmentMetric('.air-pm10', environmentData.air_quality.pm10);
        this.updateEnvironmentMetric('.air-aqi', environmentData.air_quality.aqi);

        // 水质数据
        this.updateEnvironmentMetric('.water-cod', environmentData.water_quality.cod);
        this.updateEnvironmentMetric('.water-ammonia', environmentData.water_quality.ammonia);
        this.updateEnvironmentMetric('.water-phosphorus', environmentData.water_quality.phosphorus);

        // 监测站点状态
        this.updateStationStatus('.air-stations', environmentData.air_quality.stations);
        this.updateStationStatus('.water-sections', environmentData.water_quality.sections);
    }

    /**
     * 更新环境指标
     */
    updateEnvironmentMetric(selector, metric) {
        const element = document.querySelector(selector);
        if (!element) return;

        const valueElement = element.querySelector('.metric-value');
        const levelElement = element.querySelector('.metric-level');

        if (valueElement) {
            valueElement.textContent = `${metric.value} ${metric.unit}`;
        }

        if (levelElement) {
            levelElement.textContent = metric.level;
            levelElement.className = `doubao-tag doubao-tag-${metric.status}`;
        }
    }

    /**
     * 更新监测站状态
     */
    updateStationStatus(selector, status) {
        const element = document.querySelector(selector);
        if (!element) return;

        const statusText = element.querySelector('.station-status');
        const progressBar = element.querySelector('.doubao-progress-bar');

        if (statusText) {
            statusText.textContent = `监测站点: ${status.total}个 | 在线: ${status.online}个`;
        }

        if (progressBar) {
            progressBar.style.width = `${status.rate}%`;
        }
    }

    /**
     * 初始化图表
     */
    initializeCharts(chartData) {
        this.initDataFlowChart(chartData.data_flow_trend);
        this.initAPICallChart(chartData.api_call_stats);
    }

    /**
     * 初始化数据流趋势图
     */
    initDataFlowChart(data) {
        const canvas = document.getElementById('dataFlowChart');
        if (!canvas) return;

        if (this.charts.dataFlow) {
            this.charts.dataFlow.destroy();
        }

        const ctx = canvas.getContext('2d');
        this.charts.dataFlow = new Chart(ctx, {
            type: 'line',
            data: {
                labels: data.labels,
                datasets: [{
                    label: '数据流量(条/秒)',
                    data: data.values,
                    borderColor: '#0ea5e9',
                    backgroundColor: 'rgba(14, 165, 233, 0.1)',
                    tension: 0.4,
                    fill: true
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    legend: { display: false }
                },
                scales: {
                    y: {
                        beginAtZero: true,
                        grid: { color: '#f1f5f9' }
                    },
                    x: {
                        grid: { color: '#f1f5f9' }
                    }
                }
            }
        });
    }

    /**
     * 初始化API调用统计图
     */
    initAPICallChart(data) {
        const canvas = document.getElementById('apiCallChart');
        if (!canvas) return;

        if (this.charts.apiCall) {
            this.charts.apiCall.destroy();
        }

        const ctx = canvas.getContext('2d');
        this.charts.apiCall = new Chart(ctx, {
            type: 'bar',
            data: {
                labels: data.labels,
                datasets: [{
                    label: '调用次数',
                    data: data.values,
                    backgroundColor: [
                        '#3b82f6', '#06b6d4', '#10b981',
                        '#f59e0b', '#8b5cf6', '#6b7280'
                    ],
                    borderRadius: 4
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    legend: { display: false }
                },
                scales: {
                    y: {
                        beginAtZero: true,
                        grid: { color: '#f1f5f9' }
                    },
                    x: {
                        grid: { display: false }
                    }
                }
            }
        });
    }

    /**
     * 更新图表数据
     */
    updateCharts(chartData) {
        this.updateDataFlowChart(chartData.data_flow_trend);
        this.updateAPICallChart(chartData.api_call_stats);
    }

    /**
     * 更新数据流图表
     */
    updateDataFlowChart(data) {
        if (this.charts.dataFlow) {
            this.charts.dataFlow.data.labels = data.labels;
            this.charts.dataFlow.data.datasets[0].data = data.values;
            this.charts.dataFlow.update();
        }
    }

    /**
     * 更新API调用图表
     */
    updateAPICallChart(data) {
        if (this.charts.apiCall) {
            this.charts.apiCall.data.labels = data.labels;
            this.charts.apiCall.data.datasets[0].data = data.values;
            this.charts.apiCall.update();
        }
    }

    /**
     * 更新ETL任务状态
     */
    updateETLStatus(tasks) {
        // ETL任务状态更新逻辑
        console.log('ETL任务状态已更新:', tasks);
    }

    /**
     * 更新告警信息
     */
    updateAlerts(alerts) {
        // 告警信息更新逻辑
        console.log('告警信息已更新:', alerts);
    }

    /**
     * 销毁图表
     */
    destroy() {
        Object.values(this.charts).forEach(chart => {
            if (chart) chart.destroy();
        });
        this.charts = {};
    }
}

// 导出到全局对象
window.DashboardAPI = DashboardAPI;
window.DashboardRenderer = DashboardRenderer;