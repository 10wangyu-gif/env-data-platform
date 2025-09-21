/**
 * 环保数据集成平台 - 配置管理
 * 统一管理API地址、环境配置等
 */

// 环境配置
const ENV_CONFIG = {
    development: {
        API_BASE_URL: 'http://localhost:8888',
        WS_BASE_URL: 'ws://localhost:8888',
        DEBUG: true
    },
    production: {
        API_BASE_URL: window.location.origin,
        WS_BASE_URL: window.location.origin.replace('http', 'ws'),
        DEBUG: false
    },
    test: {
        API_BASE_URL: 'http://localhost:8888',
        WS_BASE_URL: 'ws://localhost:8888',
        DEBUG: true
    }
};

// 自动检测环境
function detectEnvironment() {
    const hostname = window.location.hostname;

    // 本地开发环境
    if (hostname === 'localhost' || hostname === '127.0.0.1') {
        return 'development';
    }

    // 测试环境（可根据实际域名调整）
    if (hostname.includes('test') || hostname.includes('staging')) {
        return 'test';
    }

    // 生产环境
    return 'production';
}

// 当前环境
const CURRENT_ENV = detectEnvironment();

// 当前配置
const CONFIG = ENV_CONFIG[CURRENT_ENV];

// API 版本前缀
const API_VERSION = '/api/v1';

// API 配置对象
const API_CONFIG = {
    // 基础配置
    BASE_URL: CONFIG.API_BASE_URL,
    API_BASE_URL: CONFIG.API_BASE_URL + API_VERSION,
    WS_BASE_URL: CONFIG.WS_BASE_URL,
    DEBUG: CONFIG.DEBUG,

    // API 端点配置
    ENDPOINTS: {
        // 认证相关
        AUTH: {
            LOGIN: '/auth/login',
            LOGOUT: '/auth/logout',
            REFRESH: '/auth/refresh',
            ME: '/auth/me',
            CHANGE_PASSWORD: '/auth/password'
        },

        // 数据源管理
        DATASOURCES: {
            BASE: '/datasources',
            LIST: '/datasources',
            CREATE: '/datasources',
            GET: (id) => `/datasources/${id}`,
            UPDATE: (id) => `/datasources/${id}`,
            DELETE: (id) => `/datasources/${id}`,
            TEST: (id) => `/datasources/${id}/test`,
            SYNC: (id) => `/datasources/${id}/sync`,
            TABLES: (id) => `/datasources/${id}/tables`
        },

        // 仪表板
        DASHBOARD: {
            BASE: '/dashboard',
            OVERVIEW: '/dashboard/overview',
            REALTIME: '/dashboard/realtime',
            CHARTS: '/dashboard/charts'
        },

        // 用户管理
        USERS: {
            BASE: '/users',
            LIST: '/users',
            CREATE: '/users',
            GET: (id) => `/users/${id}`,
            UPDATE: (id) => `/users/${id}`,
            DELETE: (id) => `/users/${id}`,
            STATS: '/users/stats',
            CURRENT: '/users/current'
        },

        // 角色管理
        ROLES: {
            BASE: '/roles',
            LIST: '/roles',
            CREATE: '/roles',
            GET: (id) => `/roles/${id}`,
            UPDATE: (id) => `/roles/${id}`,
            DELETE: (id) => `/roles/${id}`,
            PERMISSIONS: (id) => `/roles/${id}/permissions`,
            USERS: (id) => `/roles/${id}/users`
        },

        // 权限管理
        PERMISSIONS: {
            BASE: '/permissions',
            LIST: '/permissions',
            CREATE: '/permissions',
            GET: (id) => `/permissions/${id}`,
            UPDATE: (id) => `/permissions/${id}`,
            DELETE: (id) => `/permissions/${id}`,
            TYPES: '/permissions/types',
            USER_MENUS: '/permissions/user/menus',
            USER_PERMISSIONS: '/permissions/user/permissions'
        },

        // ETL管理
        ETL: {
            BASE: '/etl',
            STATS: '/etl/stats',
            JOBS: {
                BASE: '/etl/jobs',
                LIST: '/etl/jobs',
                CREATE: '/etl/jobs',
                GET: (id) => `/etl/jobs/${id}`,
                UPDATE: (id) => `/etl/jobs/${id}`,
                DELETE: (id) => `/etl/jobs/${id}`,
                EXECUTE: (id) => `/etl/jobs/${id}/execute`,
                STOP: (id) => `/etl/jobs/${id}/stop`
            },
            EXECUTIONS: {
                BASE: '/etl/executions',
                LIST: '/etl/executions',
                GET: (id) => `/etl/executions/${id}`,
                LOGS: (id) => `/etl/executions/${id}/logs`
            },
            TEMPLATES: {
                BASE: '/etl/templates',
                LIST: '/etl/templates',
                CREATE: '/etl/templates',
                GET: (id) => `/etl/templates/${id}`,
                UPDATE: (id) => `/etl/templates/${id}`,
                DELETE: (id) => `/etl/templates/${id}`,
                CREATE_JOB: (id) => `/etl/templates/${id}/create-job`
            }
        },

        // 数据质量
        QUALITY: {
            BASE: '/quality',
            STATS: '/quality/stats',
            RULES: {
                BASE: '/quality/rules',
                LIST: '/quality/rules',
                CREATE: '/quality/rules',
                GET: (id) => `/quality/rules/${id}`,
                UPDATE: (id) => `/quality/rules/${id}`,
                DELETE: (id) => `/quality/rules/${id}`,
                CHECK: (id) => `/quality/rules/${id}/check`,
                BATCH_CHECK: '/quality/rules/batch-check'
            },
            REPORTS: {
                BASE: '/quality/reports',
                LIST: '/quality/reports',
                GET: (id) => `/quality/reports/${id}`
            }
        },

        // 文件管理
        FILES: {
            BASE: '/files',
            UPLOAD: '/files/upload',
            LIST: '/files/records',
            DOWNLOAD: (id) => `/files/${id}/download`,
            DELETE: (id) => `/files/${id}`,
            INFO: (id) => `/files/${id}/info`,
            STATS: '/files/stats'
        },

        // 系统管理
        SYSTEM: {
            BASE: '/system',
            INFO: '/system/info',
            STATS: '/system/stats',
            HEALTH: '/system/health',
            LOGS: {
                OPERATION: '/system/logs/operation',
                LOGIN: '/system/logs/login',
                CLEAR: '/system/logs/clear'
            }
        },

        // HJ212数据
        HJ212: {
            BASE: '/hj212',
            DATA: '/hj212/data',
            DATA_DETAIL: (id) => `/hj212/data/${id}`,
            STATS: '/hj212/stats',
            DEVICES: '/hj212/devices',
            ALARMS: '/hj212/alarms',
            COMMAND: '/hj212/command'
        }
    },

    // WebSocket 端点
    WS_ENDPOINTS: {
        REALTIME_DATA: '/ws/realtime',
        NOTIFICATIONS: '/ws/notifications',
        ETL_STATUS: '/ws/etl-status'
    },

    // 请求配置
    REQUEST_CONFIG: {
        TIMEOUT: 30000, // 30秒超时
        RETRY_COUNT: 3,
        RETRY_DELAY: 1000 // 1秒重试延迟
    },

    // 分页配置
    PAGINATION: {
        DEFAULT_PAGE_SIZE: 10,
        PAGE_SIZE_OPTIONS: [10, 20, 50, 100]
    }
};

// 工具函数：获取完整API URL
function getApiUrl(endpoint) {
    if (typeof endpoint === 'function') {
        throw new Error('Endpoint is a function, please call it with required parameters');
    }
    return API_CONFIG.API_BASE_URL + endpoint;
}

// 工具函数：获取WebSocket URL
function getWsUrl(endpoint) {
    return API_CONFIG.WS_BASE_URL + endpoint;
}

// 工具函数：调试日志
function debugLog(message, data = null) {
    if (API_CONFIG.DEBUG) {
        console.log(`[ENV-DATA-PLATFORM] ${message}`, data || '');
    }
}

// 导出配置
window.ENV_CONFIG = ENV_CONFIG;
window.API_CONFIG = API_CONFIG;
window.getApiUrl = getApiUrl;
window.getWsUrl = getWsUrl;
window.debugLog = debugLog;

// 输出当前环境信息
debugLog(`Environment: ${CURRENT_ENV}`);
debugLog(`API Base URL: ${API_CONFIG.API_BASE_URL}`);
debugLog(`WebSocket Base URL: ${API_CONFIG.WS_BASE_URL}`);