# 环保数据集成平台 Makefile

.PHONY: build run test clean docker help

# 默认目标
.DEFAULT_GOAL := help

# 变量定义
APP_NAME := env-data-platform
BUILD_DIR := build
MAIN_FILE := cmd/server/main.go
CONFIG_FILE := config/config.yaml

# Git信息
GIT_COMMIT := $(shell git rev-parse --short HEAD)
GIT_TAG := $(shell git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
BUILD_TIME := $(shell date +%Y-%m-%d-%H:%M:%S)

# Go构建参数
LDFLAGS := -ldflags "-X main.AppVersion=$(GIT_TAG) -X main.BuildTime=$(BUILD_TIME)"

# 帮助信息
help: ## 显示帮助信息
	@echo "环保数据集成平台 - 可用命令："
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

# 构建相关
build: ## 构建应用程序
	@echo "构建应用程序..."
	@mkdir -p $(BUILD_DIR)
	@go build $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME) $(MAIN_FILE)
	@echo "构建完成: $(BUILD_DIR)/$(APP_NAME)"

build-linux: ## 构建Linux版本
	@echo "构建Linux版本..."
	@mkdir -p $(BUILD_DIR)
	@GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME)-linux $(MAIN_FILE)
	@echo "构建完成: $(BUILD_DIR)/$(APP_NAME)-linux"

build-windows: ## 构建Windows版本
	@echo "构建Windows版本..."
	@mkdir -p $(BUILD_DIR)
	@GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME).exe $(MAIN_FILE)
	@echo "构建完成: $(BUILD_DIR)/$(APP_NAME).exe"

build-all: build build-linux build-windows ## 构建所有平台版本

# 运行相关
run: ## 运行应用程序
	@echo "启动应用程序..."
	@go run $(MAIN_FILE) -config $(CONFIG_FILE)

run-migrate: ## 运行数据库迁移
	@echo "执行数据库迁移..."
	@go run $(MAIN_FILE) -config $(CONFIG_FILE) -migrate

run-init: ## 初始化基础数据
	@echo "初始化基础数据..."
	@go run $(MAIN_FILE) -config $(CONFIG_FILE) -init

run-all: run-migrate run-init run ## 执行完整启动流程

# 开发相关
dev: ## 开发模式运行
	@echo "开发模式启动..."
	@air -c .air.toml

install-air: ## 安装air热重载工具
	@echo "安装air..."
	@go install github.com/cosmtrek/air@latest

install-swag: ## 安装swag API文档工具
	@echo "安装swag..."
	@go install github.com/swaggo/swag/cmd/swag@latest

gen-docs: ## 生成API文档
	@echo "生成API文档..."
	@swag init -g $(MAIN_FILE) -o docs

# 测试相关
test: ## 运行单元测试
	@echo "运行单元测试..."
	@go test -v ./...

test-coverage: ## 运行测试并生成覆盖率报告
	@echo "运行测试覆盖率检查..."
	@go test -v -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "覆盖率报告已生成: coverage.html"

benchmark: ## 运行性能测试
	@echo "运行性能测试..."
	@go test -bench=. -benchmem ./...

# 代码质量
lint: ## 代码检查
	@echo "运行代码检查..."
	@golangci-lint run

install-lint: ## 安装golangci-lint
	@echo "安装golangci-lint..."
	@curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell go env GOPATH)/bin v1.54.2

fmt: ## 格式化代码
	@echo "格式化代码..."
	@go fmt ./...

# 依赖管理
deps: ## 下载依赖
	@echo "下载依赖..."
	@go mod download

deps-update: ## 更新依赖
	@echo "更新依赖..."
	@go mod tidy

deps-vendor: ## 生成vendor目录
	@echo "生成vendor目录..."
	@go mod vendor

# Docker相关
docker-build: ## 构建Docker镜像
	@echo "构建Docker镜像..."
	@docker build -t $(APP_NAME):$(GIT_TAG) .
	@docker tag $(APP_NAME):$(GIT_TAG) $(APP_NAME):latest

docker-run: ## 运行Docker容器
	@echo "运行Docker容器..."
	@docker run -d \
		--name $(APP_NAME) \
		-p 8080:8080 \
		-v $(PWD)/config:/app/config \
		-v $(PWD)/logs:/app/logs \
		$(APP_NAME):latest

docker-stop: ## 停止Docker容器
	@echo "停止Docker容器..."
	@docker stop $(APP_NAME) || true
	@docker rm $(APP_NAME) || true

docker-logs: ## 查看Docker容器日志
	@docker logs -f $(APP_NAME)

# 部署相关
deploy-dev: ## 部署到开发环境
	@echo "部署到开发环境..."
	@docker-compose -f docker-compose.dev.yml up -d

deploy-prod: ## 部署到生产环境
	@echo "部署到生产环境..."
	@docker-compose -f docker-compose.prod.yml up -d

# 数据库相关
db-mysql: ## 启动MySQL数据库
	@echo "启动MySQL数据库..."
	@docker run -d \
		--name env-mysql \
		-e MYSQL_ROOT_PASSWORD=password \
		-e MYSQL_DATABASE=env_data_platform \
		-p 3306:3306 \
		mysql:8.0

db-redis: ## 启动Redis缓存
	@echo "启动Redis缓存..."
	@docker run -d \
		--name env-redis \
		-p 6379:6379 \
		redis:7-alpine

db-start: db-mysql db-redis ## 启动所有数据库服务

db-stop: ## 停止数据库服务
	@echo "停止数据库服务..."
	@docker stop env-mysql env-redis || true
	@docker rm env-mysql env-redis || true

# 清理
clean: ## 清理构建文件
	@echo "清理构建文件..."
	@rm -rf $(BUILD_DIR)
	@rm -f coverage.out coverage.html

clean-docker: ## 清理Docker资源
	@echo "清理Docker资源..."
	@docker system prune -f

clean-all: clean clean-docker ## 清理所有文件

# 工具
version: ## 显示版本信息
	@go run $(MAIN_FILE) -version

info: ## 显示项目信息
	@echo "项目名称: $(APP_NAME)"
	@echo "Git标签: $(GIT_TAG)"
	@echo "Git提交: $(GIT_COMMIT)"
	@echo "构建时间: $(BUILD_TIME)"
	@echo "Go版本: $(shell go version)"

# 快速命令
install: deps install-air install-swag install-lint ## 安装所有开发工具
check: fmt lint test ## 执行代码检查和测试
setup: install db-start run-migrate run-init ## 完整开发环境设置