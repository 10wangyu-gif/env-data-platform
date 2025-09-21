# 环保数据集成平台

一个现代化的环保数据集成和管理平台，支持多种数据源接入、数据质量管理、ETL处理和实时监控。

## 🚀 快速开始

### 开发环境

1. **环境要求**
   ```bash
   # 需要安装
   - Docker >= 20.0
   - docker-compose >= 1.29
   - Go >= 1.21 (开发时需要)
   ```

2. **克隆项目**
   ```bash
   git clone <repository-url>
   cd env-data-platform
   ```

3. **启动开发环境**
   ```bash
   # 使用快速启动脚本
   ./scripts/dev-start.sh

   # 或手动启动
   docker-compose -f docker-compose.dev.yml up -d
   ```

4. **访问服务**
   - 应用程序: http://localhost:8080
   - Prometheus: http://localhost:9090
   - Grafana: http://localhost:3000 (admin/dev_admin)

### 生产环境

```bash
# 部署到生产环境
./scripts/deploy.sh
```

## 📁 项目结构

```
env-data-platform/
├── cmd/                    # 应用程序入口
│   └── server/
│       └── main.go
├── internal/               # 内部包
│   ├── config/            # 配置管理
│   ├── database/          # 数据库操作
│   ├── handlers/          # HTTP处理器
│   ├── middleware/        # 中间件
│   ├── models/           # 数据模型
│   ├── routes/           # 路由定义
│   ├── server/           # 服务器
│   └── logger/           # 日志组件
├── config/                # 配置文件
├── scripts/              # 脚本工具
├── docker-compose.dev.yml   # 开发环境
├── docker-compose.prod.yml  # 生产环境
└── Dockerfile
```

## 🏗️ 架构设计

### 技术栈

- **后端**: Go 1.21 + Gin
- **数据库**: MySQL 8.0
- **缓存**: Redis 7
- **监控**: Prometheus + Grafana
- **容器化**: Docker + docker-compose

### 核心模块

1. **用户管理**: 认证、授权、RBAC权限控制
2. **数据源管理**: 多种数据源连接和配置
3. **ETL引擎**: 数据提取、转换、加载
4. **数据质量**: 质量规则、监控、报告
5. **HJ212协议**: 环保数据标准协议支持
6. **监控系统**: 实时监控、告警、日志

## 🔧 开发指南

### 本地开发

```bash
# 安装依赖
go mod download

# 运行测试
make test

# 代码检查
make lint

# 本地运行
make run
```

### 构建

```bash
# 构建所有平台
./scripts/build.sh

# 或使用 Make
make build-all
```

### 数据库操作

```bash
# 运行迁移
go run cmd/server/main.go -config config/config.dev.yaml -migrate

# 初始化数据
go run cmd/server/main.go -config config/config.dev.yaml -init
```

## 📊 API文档

### 认证接口

- `POST /api/v1/auth/login` - 用户登录
- `POST /api/v1/auth/logout` - 用户登出
- `GET /api/v1/auth/me` - 获取当前用户信息

### 数据源接口

- `GET /api/v1/datasources` - 获取数据源列表
- `POST /api/v1/datasources` - 创建数据源
- `GET /api/v1/datasources/:id` - 获取数据源详情
- `PUT /api/v1/datasources/:id` - 更新数据源
- `DELETE /api/v1/datasources/:id` - 删除数据源

### HJ212接口

- `GET /api/v1/hj212/data` - 查询HJ212数据
- `GET /api/v1/hj212/stats` - 获取统计信息

更多API详情请查看 Swagger 文档 (待完善)

## 🐳 Docker部署

### 开发环境

```bash
# 启动完整开发环境
docker-compose -f docker-compose.dev.yml up -d

# 查看日志
docker-compose -f docker-compose.dev.yml logs -f

# 停止环境
docker-compose -f docker-compose.dev.yml down
```

### 生产环境

```bash
# 构建镜像
docker build -t env-data-platform:latest .

# 启动生产环境
docker-compose -f docker-compose.prod.yml up -d
```

## 📈 监控

### Prometheus指标

- HTTP请求统计
- 数据库连接池状态
- 应用性能指标
- 自定义业务指标

### Grafana仪表板

- 应用程序监控
- 数据库性能
- 系统资源使用
- 业务数据可视化

## 🔒 安全

- JWT Token认证
- RBAC权限控制
- API访问频率限制
- 敏感数据加密存储
- Docker安全最佳实践

## 🧪 测试

```bash
# 运行所有测试
make test

# 生成测试覆盖率
make test-coverage

# 性能测试
make benchmark
```

## 📦 依赖管理

主要依赖包：

- `gin-gonic/gin` - Web框架
- `gorm.io/gorm` - ORM
- `go.uber.org/zap` - 日志
- `spf13/viper` - 配置管理
- `golang-jwt/jwt` - JWT认证
- `prometheus/client_golang` - 监控指标

## 🤝 贡献指南

1. Fork 项目
2. 创建功能分支
3. 提交更改
4. 推送到分支
5. 创建 Pull Request

## 📄 许可证

本项目采用 MIT 许可证 - 查看 [LICENSE](LICENSE) 文件了解详情

## 🆘 故障排除

### 常见问题

1. **Docker启动失败**
   ```bash
   # 检查Docker状态
   docker info

   # 清理容器和卷
   docker-compose down -v
   ```

2. **数据库连接失败**
   ```bash
   # 检查MySQL容器状态
   docker-compose logs mysql

   # 重置数据库
   docker-compose down -v
   docker-compose up -d mysql
   ```

3. **端口冲突**
   ```bash
   # 查看端口占用
   lsof -i :8080

   # 修改配置文件中的端口
   ```

### 日志查看

```bash
# 应用日志
docker-compose logs -f app

# 数据库日志
docker-compose logs -f mysql

# 所有服务日志
docker-compose logs -f
```

## 📞 联系方式

- 项目维护者: [团队邮箱]
- 问题反馈: [GitHub Issues]
- 文档更新: [文档仓库]
