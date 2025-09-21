/**
 * 环保数据集成平台 - 认证管理器
 * 统一处理用户认证、token管理、API调用等功能
 */

class AuthManager {
    constructor() {
        this.baseURL = API_CONFIG.API_BASE_URL;
        this.token = localStorage.getItem('token');
        this.user = JSON.parse(localStorage.getItem('user') || 'null');
        this.refreshTimer = null;

        // 如果有token，启动自动刷新
        if (this.token) {
            this.startTokenRefresh();
        }
    }

    /**
     * 检查用户是否已登录
     */
    isLoggedIn() {
        return !!this.token && !!this.user;
    }

    /**
     * 获取当前用户信息
     */
    getCurrentUser() {
        return this.user;
    }

    /**
     * 获取认证头
     */
    getAuthHeaders() {
        return {
            'Authorization': `Bearer ${this.token}`,
            'Content-Type': 'application/json'
        };
    }

    /**
     * 登录
     */
    async login(username, password) {
        try {
            const response = await fetch(getApiUrl(API_CONFIG.ENDPOINTS.AUTH.LOGIN), {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify({ username, password })
            });

            const data = await response.json();

            if (response.ok && data.code === 200) {
                this.token = data.data.token;
                this.user = data.data.user;

                // 保存到localStorage
                localStorage.setItem('token', this.token);
                localStorage.setItem('user', JSON.stringify(this.user));

                // 启动token刷新
                this.startTokenRefresh();

                return { success: true, data: data.data };
            } else {
                return { success: false, message: data.message || '登录失败' };
            }
        } catch (error) {
            console.error('登录请求失败:', error);
            return { success: false, message: '网络错误，请检查服务器连接' };
        }
    }

    /**
     * 登出
     */
    async logout() {
        try {
            // 调用后端登出接口
            if (this.token) {
                await fetch(getApiUrl(API_CONFIG.ENDPOINTS.AUTH.LOGOUT), {
                    method: 'POST',
                    headers: this.getAuthHeaders()
                });
            }
        } catch (error) {
            console.error('登出请求失败:', error);
        } finally {
            // 清除本地数据
            this.token = null;
            this.user = null;
            localStorage.removeItem('token');
            localStorage.removeItem('user');

            // 停止token刷新
            if (this.refreshTimer) {
                clearInterval(this.refreshTimer);
                this.refreshTimer = null;
            }

            // 跳转到登录页
            window.location.href = '/static/login.html';
        }
    }

    /**
     * 刷新token
     */
    async refreshToken() {
        try {
            const response = await fetch(getApiUrl(API_CONFIG.ENDPOINTS.AUTH.REFRESH), {
                method: 'POST',
                headers: this.getAuthHeaders()
            });

            if (response.ok) {
                const data = await response.json();
                if (data.code === 200 && data.data.token) {
                    this.token = data.data.token;
                    localStorage.setItem('token', this.token);
                    return true;
                }
            }
        } catch (error) {
            console.error('刷新token失败:', error);
        }

        // 刷新失败，清除认证信息
        this.logout();
        return false;
    }

    /**
     * 启动token自动刷新
     */
    startTokenRefresh() {
        // 每50分钟刷新一次token（假设token有效期是1小时）
        this.refreshTimer = setInterval(() => {
            this.refreshToken();
        }, 50 * 60 * 1000);
    }

    /**
     * 验证当前token是否有效
     */
    async validateToken() {
        if (!this.token) {
            return false;
        }

        try {
            const response = await fetch(getApiUrl(API_CONFIG.ENDPOINTS.AUTH.ME), {
                headers: this.getAuthHeaders()
            });

            if (response.ok) {
                const data = await response.json();
                if (data.code === 200) {
                    // 更新用户信息
                    this.user = data.data;
                    localStorage.setItem('user', JSON.stringify(this.user));
                    return true;
                }
            }
        } catch (error) {
            console.error('验证token失败:', error);
        }

        // 验证失败，清除认证信息
        this.token = null;
        this.user = null;
        localStorage.removeItem('token');
        localStorage.removeItem('user');
        return false;
    }

    /**
     * 检查用户权限
     */
    hasPermission(permission) {
        if (!this.user || !this.user.permissions) {
            return false;
        }
        return this.user.permissions.includes(permission);
    }

    /**
     * 检查用户角色
     */
    hasRole(role) {
        if (!this.user || !this.user.roles) {
            return false;
        }
        return this.user.roles.some(r => r.name === role);
    }

    /**
     * 修改密码
     */
    async changePassword(oldPassword, newPassword) {
        try {
            const response = await fetch(getApiUrl(API_CONFIG.ENDPOINTS.AUTH.CHANGE_PASSWORD), {
                method: 'PUT',
                headers: this.getAuthHeaders(),
                body: JSON.stringify({
                    old_password: oldPassword,
                    new_password: newPassword
                })
            });

            const data = await response.json();

            if (response.ok && data.code === 200) {
                return { success: true };
            } else {
                return { success: false, message: data.message || '修改密码失败' };
            }
        } catch (error) {
            console.error('修改密码失败:', error);
            return { success: false, message: '网络错误' };
        }
    }
}

/**
 * API客户端类
 * 统一处理API请求，自动添加认证信息和错误处理
 */
class APIClient {
    constructor(authManager) {
        this.auth = authManager;
        this.baseURL = authManager.baseURL;
    }

    /**
     * 通用API请求方法
     */
    async request(endpoint, options = {}) {
        const url = `${this.baseURL}${endpoint}`;
        const config = {
            headers: {
                'Content-Type': 'application/json',
                ...options.headers
            },
            ...options
        };

        // 如果已登录，添加认证头
        if (this.auth.isLoggedIn()) {
            config.headers.Authorization = `Bearer ${this.auth.token}`;
        }

        try {
            const response = await fetch(url, config);

            // 处理401未授权错误
            if (response.status === 401) {
                // 尝试刷新token
                const refreshed = await this.auth.refreshToken();
                if (refreshed) {
                    // 重试请求
                    config.headers.Authorization = `Bearer ${this.auth.token}`;
                    const retryResponse = await fetch(url, config);
                    return await this.handleResponse(retryResponse);
                } else {
                    // 刷新失败，跳转登录页
                    this.auth.logout();
                    throw new Error('认证失败，请重新登录');
                }
            }

            return await this.handleResponse(response);
        } catch (error) {
            console.error('API请求失败:', error);
            throw error;
        }
    }

    /**
     * 处理响应
     */
    async handleResponse(response) {
        const data = await response.json();

        if (response.ok && data.code === 200) {
            return data.data;
        } else {
            throw new Error(data.message || `HTTP ${response.status}: ${response.statusText}`);
        }
    }

    /**
     * GET请求
     */
    async get(endpoint, params = {}) {
        const queryString = new URLSearchParams(params).toString();
        const url = queryString ? `${endpoint}?${queryString}` : endpoint;
        return this.request(url, { method: 'GET' });
    }

    /**
     * POST请求
     */
    async post(endpoint, data = {}) {
        return this.request(endpoint, {
            method: 'POST',
            body: JSON.stringify(data)
        });
    }

    /**
     * PUT请求
     */
    async put(endpoint, data = {}) {
        return this.request(endpoint, {
            method: 'PUT',
            body: JSON.stringify(data)
        });
    }

    /**
     * DELETE请求
     */
    async delete(endpoint) {
        return this.request(endpoint, { method: 'DELETE' });
    }
}

/**
 * 页面认证守卫
 * 在需要认证的页面中调用此函数
 */
function requireAuth() {
    const authManager = new AuthManager();

    if (!authManager.isLoggedIn()) {
        // 保存当前页面URL，登录后可以跳转回来
        localStorage.setItem('redirectAfterLogin', window.location.pathname);
        window.location.href = '/static/login.html';
        return null;
    }

    // 验证token
    authManager.validateToken().then(valid => {
        if (!valid) {
            localStorage.setItem('redirectAfterLogin', window.location.pathname);
            window.location.href = '/static/login.html';
        }
    });

    return authManager;
}

/**
 * 显示用户信息
 */
function displayUserInfo(containerId = 'userInfo') {
    const authManager = new AuthManager();
    const container = document.getElementById(containerId);

    if (!container || !authManager.isLoggedIn()) {
        return;
    }

    const user = authManager.getCurrentUser();
    container.innerHTML = `
        <div class="flex items-center space-x-3">
            <div class="w-8 h-8 bg-blue-500 rounded-full flex items-center justify-center text-white text-sm font-semibold">
                ${user.username.charAt(0).toUpperCase()}
            </div>
            <div>
                <div class="text-sm font-medium text-gray-900">${user.username}</div>
                <div class="text-xs text-gray-500">${user.roles ? user.roles.map(r => r.display_name).join(', ') : ''}</div>
            </div>
            <button onclick="logout()" class="text-gray-400 hover:text-gray-600">
                <i class="fas fa-sign-out-alt"></i>
            </button>
        </div>
    `;
}

/**
 * 全局登出函数
 */
function logout() {
    const authManager = new AuthManager();
    authManager.logout();
}

/**
 * 错误处理工具
 */
function showError(message, containerId = 'errorContainer') {
    const container = document.getElementById(containerId);
    if (!container) {
        alert(message);
        return;
    }

    container.innerHTML = `
        <div class="bg-red-50 border border-red-200 rounded-md p-4 mb-4">
            <div class="flex">
                <div class="flex-shrink-0">
                    <i class="fas fa-exclamation-circle text-red-400"></i>
                </div>
                <div class="ml-3">
                    <p class="text-sm text-red-800">${message}</p>
                </div>
                <div class="ml-auto pl-3">
                    <button onclick="this.parentElement.parentElement.parentElement.remove()" class="text-red-400 hover:text-red-600">
                        <i class="fas fa-times"></i>
                    </button>
                </div>
            </div>
        </div>
    `;

    // 自动隐藏错误信息
    setTimeout(() => {
        const errorEl = container.querySelector('.bg-red-50');
        if (errorEl) {
            errorEl.remove();
        }
    }, 5000);
}

/**
 * 成功提示工具
 */
function showSuccess(message, containerId = 'successContainer') {
    const container = document.getElementById(containerId);
    if (!container) {
        return;
    }

    container.innerHTML = `
        <div class="bg-green-50 border border-green-200 rounded-md p-4 mb-4">
            <div class="flex">
                <div class="flex-shrink-0">
                    <i class="fas fa-check-circle text-green-400"></i>
                </div>
                <div class="ml-3">
                    <p class="text-sm text-green-800">${message}</p>
                </div>
                <div class="ml-auto pl-3">
                    <button onclick="this.parentElement.parentElement.parentElement.remove()" class="text-green-400 hover:text-green-600">
                        <i class="fas fa-times"></i>
                    </button>
                </div>
            </div>
        </div>
    `;

    // 自动隐藏成功信息
    setTimeout(() => {
        const successEl = container.querySelector('.bg-green-50');
        if (successEl) {
            successEl.remove();
        }
    }, 3000);
}

// 导出全局对象供其他脚本使用
window.AuthManager = AuthManager;
window.APIClient = APIClient;
window.requireAuth = requireAuth;
window.displayUserInfo = displayUserInfo;
window.logout = logout;
window.showError = showError;
window.showSuccess = showSuccess;