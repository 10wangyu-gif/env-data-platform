---
name: env-data-platform
status: completed
created: 2025-09-19T11:32:51Z
completed: 2025-09-21T03:03:03Z
progress: 100%
prd: .claude/prds/env-data-platform.md
github: https://github.com/allenwang1207/env-data-platform
---

# Epic: 环保数据集成平台 (env-data-platform)

## 概述

基于Go单体分层架构和Apache Hop ETL引擎，构建商用环保数据集成平台。平台采用前后端分离设计，前端使用Vue3+TypeScript，后端使用Go+Gin单体应用框架，支持单实例部署。核心解决212协议数据接入、可视化ETL处理、数据资产目录管理、API服务管理和智能运维监控五大业务场景。

## 架构决策

### 核心技术选型
- **后端语言**：Go - 高性能并发处理，适合大数据量ETL场景，信创环境兼容性好
- **Web框架**：Gin - 轻量级高性能，中文社区活跃，生态丰富
- **数据库**：MySQL（开发）+ 达梦数据库、人大金仓数据库（信创） - 满足ACID要求和信创合规
- **ETL引擎**：Apache Hop 
- **前端框架**：Vue3 + TypeScript + Ant Design Vue - 现代化开发体验，组件库丰富

### 设计模式
- **单体分层架构**：按功能层次分层（Web层、业务层、数据层），简化部署和维护
- **模块化设计**：按业务域划分模块（用户管理、数据源管理、ETL管理、数据目录、监控运维）
- **MVC模式**：Model-View-Controller分离，清晰的职责划分
- **依赖注入**：松耦合设计，便于测试和扩展
- **事件驱动**：关键业务操作使用事件通知，异步处理

### 基础设施决策
- **容器化**：Docker单容器部署，简化运维
- **负载均衡**：Nginx反向代理（必要时）
- **监控体系**：Prometheus + Grafana指标监控
- **日志管理**：结构化日志 + ELK Stack（可选）
- **缓存策略**：Redis单实例，提高响应性能

## 技术方案

### 系统架构总览

```
┌─────────────────────────────────────────────────────────────┐
│                环保数据集成平台 (单体架构)                     │
├─────────────────────────────────────────────────────────────┤
│  前端层 (Vue3 + TypeScript)                                │
│  ├── ETL设计器           ├── 数据目录         ├── 监控面板   │
│  ├── 数据源管理           ├── API服务管理      ├── 系统设置   │
├─────────────────────────────────────────────────────────────┤
│  Web层 (Go + Gin 单体应用)                                 │
│  ├── 路由管理           ├── 中间件栈         ├── 静态资源    │
│  ├── 认证授权           ├── 限流熔断         ├── 日志审计    │
├─────────────────────────────────────────────────────────────┤
│  业务模块层 (Go Package)                                   │
│  ├── 用户管理模块        ├── 数据源管理模块   ├── ETL管理模块 │
│  ├── 数据目录模块        ├── API管理模块      ├── 监控模块   │
├─────────────────────────────────────────────────────────────┤
│  数据访问层                                                │
│  ├── GORM(数据库)       ├── Redis(缓存)      ├── ES(搜索)   │
│  └── Hop Client(ETL)    └── 文件存储         └── 外部API    │
├─────────────────────────────────────────────────────────────┤
│  外部服务 (独立部署)                                         │
│  ├── Apache Hop单实例   ├── MySQL/达梦       ├── Redis      │
│  └── Elasticsearch     └── MinIO(可选)       └── 监控组件   │
└─────────────────────────────────────────────────────────────┘
```

### 核心技术选型

**后端技术栈：**
```yaml
# 核心技术
语言: Go 1.21+
框架: Gin v1.9+
数据库: MySQL / 达梦数据库 DM8
ETL引擎: Apache Hop 2.8+
缓存: Redis 7+
搜索: Elasticsearch 8+

# 基础设施
容器化: Docker + Docker Compose
编排: Kubernetes (生产环境)
监控: Prometheus + Grafana
日志: ELK Stack
存储: MinIO (对象存储)
```

**前端技术栈：**
```yaml
框架: Vue 3.4+
语言: TypeScript 5+
UI库: Ant Design Vue 4+
状态管理: Pinia
构建工具: Vite 5+
图表库: ECharts 5+
可视化: D3.js 7+ (ETL设计器)
```

### Apache Hop单实例架构

```
Apache Hop单实例架构:
┌─────────────────────────────────────────────────────────┐
│                Apache Hop Server                       │
│  ┌─────────────────────────────────────────────────────┐│
│  │              Hop Server                             ││
│  │  ├── REST API服务 (8181端口)                        ││
│  │  ├── Pipeline执行引擎                               ││
│  │  ├── 调度服务                                       ││
│  │  └── Web界面管理                                   ││
│  └─────────────────────────────────────────────────────┘│
│                          ↓                             │
├─────────────────────────────────────────────────────────┤
│                Hop元数据仓库                             │
│                (MySQL/达梦)                             │
└─────────────────────────────────────────────────────────┘
```

### 前端组件架构
```
├── src/
│   ├── views/          # 页面组件
│   │   ├── dashboard/  # 数据概览仪表板
│   │   ├── datasource/ # 数据源管理
│   │   ├── etl/        # 自研ETL设计器
│   │   ├── catalog/    # 数据目录
│   │   ├── api/        # API服务管理
│   │   └── monitoring/ # 监控运维
│   ├── components/     # 通用组件
│   │   ├── etl-designer/ # ETL可视化设计器
│   │   ├── charts/     # 图表组件
│   │   └── forms/      # 表单组件
│   ├── stores/         # Pinia状态管理
│   ├── api/           # API接口封装
│   └── utils/         # 工具函数
```

### 后端单体应用架构
```
├── cmd/               # 应用入口
│   └── server/        # 主应用入口
├── internal/          # 内部代码
│   ├── api/           # HTTP处理层
│   │   ├── handlers/  # 请求处理器
│   │   ├── middleware/ # 中间件
│   │   └── routes/    # 路由定义
│   ├── service/       # 业务服务层
│   │   ├── auth/      # 认证服务
│   │   ├── datasource/ # 数据源管理
│   │   ├── etl/       # ETL管理
│   │   ├── catalog/   # 数据目录
│   │   └── monitor/   # 监控服务
│   ├── repository/    # 数据访问层
│   │   ├── mysql/     # 数据库访问
│   │   ├── redis/     # 缓存访问
│   │   └── es/        # 搜索引擎
│   ├── model/         # 数据模型
│   └── config/        # 配置管理
├── pkg/               # 公共库
│   ├── protocol212/   # 212协议解析
│   ├── hopclient/     # Hop客户端
│   ├── logger/        # 日志工具
│   └── utils/         # 工具函数
├── web/               # 前端静态资源
└── docs/              # 文档
```

### 数据处理流程设计

**212协议准实时处理：**
```
数据源 → TCP监听服务 → 数据缓冲池 → Hop Pipeline(1分钟调度) → 数据验证 → 入库
```

**批量文件处理：**
```
文件上传 → MinIO存储 → Hop Pipeline → 数据解析 → 清洗转换 → 数据仓库
```

**数据库ETL：**
```
源数据库 → Hop Pipeline → 增量检测 → 数据转换 → 目标数据库
```

## 实施策略

### 开发阶段规划（Apache Hop单一引擎）

**Phase 1 (3个月)** - 基础架构搭建
- Go微服务基础框架和公共库开发
- Apache Hop集群环境搭建和配置
- PostgreSQL/达梦数据库架构设计
- 用户认证授权系统实现
- 前端Vue3基础框架和UI组件库
- CI/CD流水线建立

**Phase 2 (3个月)** - 核心功能实现
- 自研ETL可视化设计器开发（Vue3 + D3.js）
- Hop Pipeline XML生成器和管理系统
- 212协议数据接入完整实现
- 多源数据接入组件开发
- 数据资产目录基础功能
- Hop REST API封装和调度引擎

**Phase 3 (2个月)** - 高级功能和优化
- 智能运维监控系统集成
- API服务网关完整实现
- 数据质量管理和血缘跟踪
- 性能优化和安全加固
- 信创环境适配测试

**Phase 4 (1个月)** - 测试部署
- 端到端集成测试
- 性能和压力测试
- 用户验收测试
- 生产环境部署和运维验证

### 风险缓解
- **技术风险**：Go+Hop技术栈培训，关键组件POC验证
- **ETL设计器风险**：D3.js可视化技术调研，参考开源方案
- **Hop集成风险**：早期Hop环境搭建，REST API集成验证
- **性能风险**：分阶段压力测试，Hop集群扩展验证
- **212协议风险**：协议解析POC，准实时处理方案验证
- **信创适配风险**：早期达梦数据库适配测试

### 测试策略
- **单元测试**：覆盖率≥80%，关键业务逻辑100%覆盖
- **集成测试**：API自动化测试，数据库集成测试
- **端到端测试**：用户关键路径自动化测试
- **性能测试**：模拟生产负载，验证性能指标
- **安全测试**：渗透测试，漏洞扫描

## 任务分解预览

基于Apache Hop单实例和Go单体架构的高级任务分类：

- [ ] **Go单体应用框架**：单体应用架构、模块化设计、配置管理、基础中间件
- [ ] **数据库架构设计**：MySQL/达梦数据库设计、迁移脚本、连接池配置
- [ ] **Apache Hop单实例部署**：Hop Server部署、REST API集成、元数据仓库配置
- [ ] **用户认证授权模块**：RBAC权限系统、JWT令牌管理、用户管理界面
- [ ] **自研ETL可视化设计器**：Vue3+D3.js可视化设计器、Hop Pipeline XML生成器
- [ ] **Hop Pipeline管理系统**：Pipeline管理、调度引擎、执行监控
- [ ] **212协议数据接入**：协议解析器、TCP监听服务、准实时处理Pipeline
- [ ] **多源数据接入组件**：数据库连接器、文件解析器、API适配器
- [ ] **数据源管理界面**：数据源配置、连接测试、状态监控界面
- [ ] **数据资产目录系统**：元数据管理、Elasticsearch搜索、数据血缘追踪
- [ ] **API服务管理**：API路由管理、文档生成、调用统计
- [ ] **开发者门户**：API文档、在线测试、SDK生成
- [ ] **前端应用开发**：Vue3应用、组件库、图表展示、响应式设计
- [ ] **智能运维监控**：Prometheus集成、监控面板、告警系统
- [ ] **部署和配置**：Docker容器化、配置管理、部署脚本

## 依赖关系

### 外部服务依赖
- **Apache Hop**：ETL引擎核心依赖，需要版本2.8+，Java Runtime 11+
- **PostgreSQL/达梦数据库**：主数据存储和Hop元数据仓库，需要支持JSON字段
- **Redis集群**：缓存和会话存储，需要支持集群模式
- **Elasticsearch**：全文搜索引擎，需要8.0+版本
- **MinIO**：对象存储，用于大文件存储和数据湖
- **Prometheus+Grafana**：监控体系，需要与Go应用和Hop集成

### 内部团队依赖
- **前端团队**：Vue3+D3.js技术栈，需要2-3名前端工程师（其中1名有可视化经验）
- **后端团队**：Go语言+Hop技能，需要4-5名后端工程师，其中1名架构师
- **运维团队**：Docker+K8s运维经验，需要1-2名DevOps工程师
- **测试团队**：自动化测试能力，需要2名测试工程师
- **业务专家**：环保行业背景，需要1名产品经理和1名环保业务专家

### 关键路径依赖
1. Apache Hop集群搭建完成 → 所有ETL功能开发
2. 自研ETL设计器完成 → 用户界面集成测试
3. 212协议解析完成 → 实时数据流测试
4. API网关完成 → 前后端联调
5. 监控系统完成 → 生产环境部署准备

## 成功标准（技术）

### 性能基准
- **数据吞吐量**：单节点处理≥100MB/s，集群线性扩展
- **API响应时间**：P95 ≤ 200ms，P99 ≤ 500ms
- **系统可用性**：99.9%正常运行时间，故障恢复≤15分钟
- **并发处理**：支持100+并发用户，1000+API TPS

### 质量门禁
- **代码质量**：SonarQube质量门禁A级，技术债务≤8小时
- **测试覆盖**：单元测试≥80%，API测试100%覆盖
- **安全扫描**：无高危漏洞，中危漏洞修复率≥95%
- **性能测试**：通过生产负载压力测试

### 可用性标准
- **监控覆盖**：100%服务监控，关键指标告警覆盖
- **文档完整**：API文档、部署文档、运维手册齐全
- **容器化**：100%服务容器化，支持K8s部署
- **备份恢复**：数据备份策略，RTO≤4小时，RPO≤1小时

## 工作量评估

### 总体时间线
- **Phase 1: 基础架构**：3个月（Go微服务框架 + Hop集群 + 基础前端）
- **Phase 2: 核心功能**：3个月（ETL设计器 + 数据接入 + 数据目录）
- **Phase 3: 完善功能**：2个月（监控运维 + API网关 + 性能优化）
- **测试部署**：1个月（集成测试 + 性能测试 + 生产部署）
- **总计**：9个月到完整商用版本

### 资源需求
- **核心开发团队**：8人
  - 后端工程师：4人（Go + Hop技术栈）
  - 前端工程师：3人（Vue3 + D3.js，其中1人专注ETL设计器）
  - 全栈工程师：1人（集成和协调）
- **支撑团队**：4人
  - 测试工程师：2人（自动化测试 + 性能测试）
  - DevOps工程师：1人（容器化部署 + 监控）
  - 产品经理：1人（需求管理 + 用户验收）

### 关键路径项目
1. **Apache Hop集群搭建**：基础关键，预估3周
2. **自研ETL设计器开发**：技术难点，预估8周
3. **212协议解析器开发**：复杂度高，预估4周
4. **Hop Pipeline管理系统**：核心功能，预估6周
5. **数据目录搜索引擎**：性能要求高，预估4周
6. **监控运维系统**：生产必需，预估3周

### 技术风险评估
- **Hop集成复杂度**：中等风险，有成熟社区支持
- **ETL设计器开发**：高风险，需要可视化专业技能
- **212协议处理**：中等风险，准实时方案可行性验证
- **性能优化**：低风险，Hop引擎性能已验证
- **信创环境适配**：中等风险，需要达梦数据库等适配测试

## 任务创建总结

### 已创建任务
- [ ] [#2](https://github.com/allenwang1207/env-data-platform/issues/2) - Go单体应用基础框架搭建 (parallel: true)
- [ ] [#3](https://github.com/allenwang1207/env-data-platform/issues/3) - 数据库架构设计与实现 (parallel: true)
- [ ] [#4](https://github.com/allenwang1207/env-data-platform/issues/4) - Apache Hop单实例环境搭建 (parallel: true)
- [ ] [#5](https://github.com/allenwang1207/env-data-platform/issues/5) - 用户认证授权系统实现 (parallel: false)
- [ ] [#6](https://github.com/allenwang1207/env-data-platform/issues/6) - 自研ETL可视化设计器开发 (parallel: false)
- [ ] [#7](https://github.com/allenwang1207/env-data-platform/issues/7) - Hop Pipeline XML生成器和管理系统 (parallel: false)
- [ ] [#8](https://github.com/allenwang1207/env-data-platform/issues/8) - 212协议数据接入实现 (parallel: false)
- [ ] [#9](https://github.com/allenwang1207/env-data-platform/issues/9) - 多源数据接入组件开发 (parallel: true)
- [ ] [#10](https://github.com/allenwang1207/env-data-platform/issues/10) - 数据源管理界面开发 (parallel: true)
- [ ] [#11](https://github.com/allenwang1207/env-data-platform/issues/11) - 数据资产目录系统开发 (parallel: false)
- [ ] [#12](https://github.com/allenwang1207/env-data-platform/issues/12) - API服务网关实现 (parallel: true)
- [ ] [#13](https://github.com/allenwang1207/env-data-platform/issues/13) - API开发者门户开发 (parallel: false)
- [ ] [#14](https://github.com/allenwang1207/env-data-platform/issues/14) - 前端基础应用框架开发 (parallel: true)
- [ ] [#15](https://github.com/allenwang1207/env-data-platform/issues/15) - 前端业务页面开发 (parallel: false)
- [ ] [#16](https://github.com/allenwang1207/env-data-platform/issues/16) - 前端ETL设计器界面集成 (parallel: false)
- [ ] [#17](https://github.com/allenwang1207/env-data-platform/issues/17) - 智能运维监控系统开发 (parallel: true)
- [ ] [#18](https://github.com/allenwang1207/env-data-platform/issues/18) - 系统集成测试 (parallel: false)
- [ ] [#19](https://github.com/allenwang1207/env-data-platform/issues/19) - 容器化部署和配置管理 (parallel: true)

### 任务统计
- **总任务数**: 18个
- **并行任务**: 10个 (56%)
- **串行任务**: 8个 (44%)
- **预估总工作量**: 400-500小时
- **预估开发周期**: 6-8个月 (8人团队)

### 关键依赖路径
1. **基础架构**: 001, 002, 003 → 其他所有任务
2. **认证系统**: 004 → 010, 012 (需要权限控制)
3. **ETL设计器**: 005 → 015 (前端集成)
4. **Pipeline管理**: 006 → 007, 008 (数据接入)
5. **集成测试**: 017 依赖前面所有功能任务

## 部署和运维方案

### 容器化架构

**Docker镜像设计：**
```dockerfile
# Go微服务基础镜像
FROM golang:1.21-alpine AS builder
FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/main .
CMD ["./main"]

# Apache Hop镜像
FROM openjdk:11-jre-slim
RUN useradd -ms /bin/bash hop
COPY hop/ /opt/hop/
WORKDIR /opt/hop
USER hop
EXPOSE 8181
CMD ["./hop-server.sh"]

# 前端Nginx镜像
FROM node:18-alpine AS builder
FROM nginx:alpine
COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/nginx.conf
```

### Docker单实例部署架构

```yaml
# docker-compose.yml 部署配置
version: '3.8'
services:
  # Apache Hop单实例
  hop-server:
    image: env-platform/hop-server:2.8
    container_name: hop-server
    ports:
      - "8181:8181"
    environment:
      - HOP_SERVER_PORT=8181
      - HOP_METADATA_DB_HOST=mysql
      - HOP_METADATA_DB_USER=hop_user
      - HOP_METADATA_DB_PASSWORD=hop_password
    volumes:
      - hop-config:/opt/hop/config
      - hop-metadata:/opt/hop/metadata
      - hop-projects:/opt/hop/projects
    depends_on:
      - mysql
    restart: unless-stopped

  # 业务应用服务
  api-gateway:
    image: env-platform/api-gateway:latest
    ports:
      - "8080:8080"
    environment:
      - HOP_SERVER_URL=http://hop-server:8181
    depends_on:
      - hop-server
      - mysql
    restart: unless-stopped

volumes:
  hop-config:
  hop-metadata:
  hop-projects:
```

### 云端SaaS部署方案

**阿里云单实例架构：**
```
负载均衡 SLB
    ↓
ECS云服务器 (Docker部署)
    ├── 前端服务 (Nginx容器)
    ├── API网关 (Go容器)
    ├── 业务服务 (Go容器)
    └── Apache Hop单实例 (Docker容器)
    ↓
数据层
    ├── RDS MySQL (主数据库)
    ├── Redis单实例 (缓存)
    ├── OSS (文件存储)
    └── Elasticsearch单节点 (搜索)
```

### 私有化信创部署方案

**硬件要求（单实例部署）：**
```
最小配置（开发测试）：
- CPU: 8核心
- 内存: 32GB
- 存储: 1TB SSD
- 网络: 千兆网卡

推荐配置（生产环境）：
- CPU: 16核心
- 内存: 64GB
- 存储: 2TB SSD + 5TB 机械硬盘
- 网络: 万兆网卡

高性能配置（大数据量）：
- CPU: 24核心
- 内存: 96GB
- 存储: 3TB SSD + 10TB 机械硬盘
- 网络: 万兆网卡
```

**信创软件栈：**
```yaml
操作系统:
  - 银河麒麟 V10
  - 统信UOS V20

数据库:
  - 达梦数据库 DM8
  - 人大金仓 KingbaseES V8

中间件:
  - 东方通TongWeb
  - 金蝶Apusic

容器平台:
  - 华为云原生平台
  - 青云容器平台
```

### 监控运维体系

**监控指标设计：**
```yaml
# 系统层监控
- CPU使用率、内存使用率、磁盘IO
- 网络吞吐量、连接数统计
- Docker容器状态、K8s Pod状态

# 应用层监控
- Go服务响应时间、错误率、QPS
- Hop Pipeline执行状态、处理耗时
- 数据库连接池、SQL执行时间

# 业务层监控
- 212协议数据接入量、延迟时间
- ETL任务成功率、失败重试次数
- API调用统计、用户活跃度
- 数据质量评分、异常数据比例
```

**告警策略：**
```yaml
# 紧急告警 (5分钟内响应)
- 系统宕机、数据库连接失败
- 212协议数据中断超过10分钟
- 关键ETL任务连续失败

# 重要告警 (30分钟内响应)
- CPU使用率超过80%
- 内存使用率超过85%
- API错误率超过5%

# 一般告警 (2小时内响应)
- 磁盘使用率超过80%
- ETL任务执行时间超过预期50%
- 数据质量评分下降10%
```

### 备份恢复策略

**数据备份：**
```bash
# 数据库备份 (每日凌晨2点)
pg_dump -h postgres -U user env_platform | gzip > backup_$(date +%Y%m%d).sql.gz

# Hop元数据备份 (每日凌晨3点)
kubectl exec hop-server-0 -- tar -czf metadata_backup_$(date +%Y%m%d).tar.gz /opt/hop/metadata

# 配置文件备份 (每周)
kubectl get configmap,secret -o yaml > config_backup_$(date +%Y%m%d).yaml
```

**灾难恢复：**
```
RTO (恢复时间目标): 4小时
RPO (恢复点目标): 1小时
备份保留策略: 30天全量 + 90天增量
异地备份: 主要数据异地容灾备份
```

### 运维管理平台

**DevOps工具链：**
```
代码管理: GitLab CE
CI/CD: GitLab CI + Jenkins
镜像仓库: Harbor
配置管理: Ansible
监控告警: Prometheus + Grafana + AlertManager
日志收集: ELK Stack
APM: Jaeger + SkyWalking
```

**自动化运维：**
```bash
# 自动化部署脚本
#!/bin/bash
# deploy.sh - 一键部署脚本

echo "开始部署环保数据集成平台..."

# 1. 检查环境
check_prerequisites() {
    kubectl version --client || exit 1
    docker --version || exit 1
    helm version || exit 1
}

# 2. 部署基础服务
deploy_infrastructure() {
    helm install postgresql bitnami/postgresql
    helm install redis bitnami/redis
    helm install elasticsearch elastic/elasticsearch
}

# 3. 部署应用服务
deploy_application() {
    kubectl apply -f k8s/namespace.yaml
    kubectl apply -f k8s/configmap.yaml
    kubectl apply -f k8s/secret.yaml
    kubectl apply -f k8s/deployment.yaml
    kubectl apply -f k8s/service.yaml
    kubectl apply -f k8s/ingress.yaml
}

# 4. 健康检查
health_check() {
    kubectl wait --for=condition=ready pod -l app=env-platform --timeout=300s
    echo "部署完成，访问地址: http://platform.env.local"
}

main() {
    check_prerequisites
    deploy_infrastructure
    deploy_application
    health_check
}

main "$@"
```