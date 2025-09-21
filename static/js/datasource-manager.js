/**
 * 环保数据集成平台 - 数据源管理API客户端
 * 处理数据源的CRUD操作、连接测试、同步等功能
 */

class DataSourceManager {
    constructor(authManager, apiClient) {
        this.auth = authManager;
        this.api = apiClient;
        this.baseURL = API_CONFIG.ENDPOINTS.DATASOURCES.BASE;
        this.currentPage = 1;
        this.pageSize = 10;
        this.currentFilters = {};
    }

    /**
     * 获取数据源列表
     */
    async getDataSources(page = 1, pageSize = 10, filters = {}) {
        try {
            const params = {
                page,
                page_size: pageSize,
                ...filters
            };

            const response = await this.api.get(this.baseURL, params);
            return {
                success: true,
                data: response
            };
        } catch (error) {
            console.error('获取数据源列表失败:', error);
            return {
                success: false,
                message: error.message || '获取数据源列表失败'
            };
        }
    }

    /**
     * 获取数据源统计信息
     */
    async getDataSourceStats() {
        try {
            // 获取所有数据源并计算统计信息
            const response = await this.api.get(this.baseURL, { page: 1, page_size: 1000 });
            const dataSources = response.list || [];

            const stats = {
                total: dataSources.length,
                hj212: dataSources.filter(ds => ds.type === 'hj212').length,
                database: dataSources.filter(ds => ds.type === 'database').length,
                file: dataSources.filter(ds => ds.type === 'file').length,
                api: dataSources.filter(ds => ds.type === 'api').length,
                online: dataSources.filter(ds => ds.is_connected).length,
                offline: dataSources.filter(ds => !ds.is_connected).length,
                error: dataSources.filter(ds => ds.error_count > 0).length
            };

            return {
                success: true,
                data: stats
            };
        } catch (error) {
            console.error('获取数据源统计失败:', error);
            return {
                success: false,
                message: error.message || '获取统计信息失败'
            };
        }
    }

    /**
     * 创建数据源
     */
    async createDataSource(dataSourceData) {
        try {
            const response = await this.api.post(this.baseURL, dataSourceData);
            return {
                success: true,
                data: response,
                message: '数据源创建成功'
            };
        } catch (error) {
            console.error('创建数据源失败:', error);
            return {
                success: false,
                message: error.message || '创建数据源失败'
            };
        }
    }

    /**
     * 获取数据源详情
     */
    async getDataSource(id) {
        try {
            const response = await this.api.get(`${this.baseURL}/${id}`);
            return {
                success: true,
                data: response
            };
        } catch (error) {
            console.error('获取数据源详情失败:', error);
            return {
                success: false,
                message: error.message || '获取数据源详情失败'
            };
        }
    }

    /**
     * 更新数据源
     */
    async updateDataSource(id, dataSourceData) {
        try {
            const response = await this.api.put(`${this.baseURL}/${id}`, dataSourceData);
            return {
                success: true,
                data: response,
                message: '数据源更新成功'
            };
        } catch (error) {
            console.error('更新数据源失败:', error);
            return {
                success: false,
                message: error.message || '更新数据源失败'
            };
        }
    }

    /**
     * 删除数据源
     */
    async deleteDataSource(id) {
        try {
            await this.api.delete(`${this.baseURL}/${id}`);
            return {
                success: true,
                message: '数据源删除成功'
            };
        } catch (error) {
            console.error('删除数据源失败:', error);
            return {
                success: false,
                message: error.message || '删除数据源失败'
            };
        }
    }

    /**
     * 测试数据源连接
     */
    async testDataSource(id) {
        try {
            const response = await this.api.post(`${this.baseURL}/${id}/test`);
            return {
                success: true,
                data: response,
                message: '连接测试成功'
            };
        } catch (error) {
            console.error('测试数据源连接失败:', error);
            return {
                success: false,
                message: error.message || '连接测试失败'
            };
        }
    }

    /**
     * 同步数据源元数据
     */
    async syncDataSource(id) {
        try {
            const response = await this.api.post(`${this.baseURL}/${id}/sync`);
            return {
                success: true,
                data: response,
                message: '元数据同步成功'
            };
        } catch (error) {
            console.error('同步数据源元数据失败:', error);
            return {
                success: false,
                message: error.message || '元数据同步失败'
            };
        }
    }

    /**
     * 获取数据源表列表
     */
    async getDataSourceTables(id) {
        try {
            const response = await this.api.get(`${this.baseURL}/${id}/tables`);
            return {
                success: true,
                data: response
            };
        } catch (error) {
            console.error('获取数据源表列表失败:', error);
            return {
                success: false,
                message: error.message || '获取表列表失败'
            };
        }
    }

    /**
     * 测试数据源配置（不保存）
     */
    async testDataSourceConfig(type, config) {
        try {
            const response = await this.api.post(`${this.baseURL}/test`, {
                type,
                config
            });
            return {
                success: true,
                data: response,
                message: '配置测试成功'
            };
        } catch (error) {
            console.error('测试数据源配置失败:', error);
            return {
                success: false,
                message: error.message || '配置测试失败'
            };
        }
    }

    /**
     * 格式化数据源类型显示
     */
    formatDataSourceType(type) {
        const typeMap = {
            'hj212': 'HJ212设备',
            'database': '数据库',
            'file': '文件数据源',
            'api': 'API接口',
            'webhook': 'Webhook'
        };
        return typeMap[type] || type;
    }

    /**
     * 格式化数据源状态显示
     */
    formatDataSourceStatus(status, isConnected = false) {
        if (status === 'active') {
            return isConnected ? '在线' : '离线';
        }
        return '禁用';
    }

    /**
     * 获取数据源状态标签类型
     */
    getStatusTagType(status, isConnected = false) {
        if (status === 'active') {
            return isConnected ? 'success' : 'danger';
        }
        return 'secondary';
    }

    /**
     * 格式化文件大小
     */
    formatFileSize(bytes) {
        if (bytes === 0) return '0 B';
        const k = 1024;
        const sizes = ['B', 'KB', 'MB', 'GB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
    }

    /**
     * 格式化时间显示
     */
    formatTimeAgo(timestamp) {
        if (!timestamp) return '从未';

        const now = new Date();
        const date = new Date(timestamp);
        const diff = now - date;

        const seconds = Math.floor(diff / 1000);
        const minutes = Math.floor(seconds / 60);
        const hours = Math.floor(minutes / 60);
        const days = Math.floor(hours / 24);

        if (seconds < 60) return `${seconds}秒前`;
        if (minutes < 60) return `${minutes}分钟前`;
        if (hours < 24) return `${hours}小时前`;
        if (days < 7) return `${days}天前`;

        return date.toLocaleDateString();
    }

    /**
     * 获取数据源类型图标
     */
    getDataSourceIcon(type) {
        const iconMap = {
            'hj212': 'fas fa-broadcast-tower',
            'database': 'fas fa-database',
            'file': 'fas fa-file-alt',
            'api': 'fas fa-cloud',
            'webhook': 'fas fa-webhook'
        };
        return iconMap[type] || 'fas fa-circle';
    }

    /**
     * 获取数据源类型颜色主题
     */
    getDataSourceTheme(type) {
        const themeMap = {
            'hj212': 'air',
            'database': 'water',
            'file': 'soil',
            'api': 'forest',
            'webhook': 'noise'
        };
        return themeMap[type] || 'air';
    }

    /**
     * 验证数据源配置
     */
    validateDataSourceConfig(type, config) {
        const errors = [];

        switch (type) {
            case 'hj212':
                if (!config.host) errors.push('请输入设备IP地址');
                if (!config.listen_port) errors.push('请输入端口号');
                break;
            case 'database':
                if (!config.host) errors.push('请输入数据库主机地址');
                if (!config.port) errors.push('请输入端口号');
                if (!config.database) errors.push('请输入数据库名');
                if (!config.username) errors.push('请输入用户名');
                break;
            case 'api':
                if (!config.url) errors.push('请输入API地址');
                if (!config.method) errors.push('请选择请求方法');
                break;
            case 'file':
                if (!config.file_path) errors.push('请输入文件路径');
                if (!config.file_format) errors.push('请选择文件格式');
                break;
        }

        return {
            valid: errors.length === 0,
            errors
        };
    }

    /**
     * 生成数据源配置默认值
     */
    getDefaultConfig(type) {
        const defaults = {
            'hj212': {
                protocol: 'TCP',
                listen_port: 9001,
                timeout: 30,
                retry_times: 3
            },
            'database': {
                port: 3306,
                timeout: 30,
                retry_times: 3
            },
            'api': {
                method: 'GET',
                timeout: 30,
                retry_times: 3,
                headers: {}
            },
            'file': {
                file_format: 'csv',
                encoding: 'utf-8'
            }
        };
        return defaults[type] || {};
    }

    /**
     * 设置当前页面和过滤器（用于刷新）
     */
    setCurrentState(page, pageSize, filters) {
        this.currentPage = page;
        this.pageSize = pageSize;
        this.currentFilters = filters;
    }

    /**
     * 刷新当前页面数据
     */
    async refreshCurrentPage() {
        return await this.getDataSources(this.currentPage, this.pageSize, this.currentFilters);
    }
}

// 导出到全局对象
window.DataSourceManager = DataSourceManager;