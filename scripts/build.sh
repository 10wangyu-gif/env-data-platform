#!/bin/bash

# 构建脚本

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 项目信息
APP_NAME="env-data-platform"
BUILD_DIR="build"
MAIN_FILE="cmd/server/main.go"

# Git信息
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GIT_TAG=$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
BUILD_TIME=$(date +%Y-%m-%d-%H:%M:%S)

# 构建标志
LDFLAGS="-X main.AppVersion=${GIT_TAG} -X main.BuildTime=${BUILD_TIME} -X main.GitCommit=${GIT_COMMIT}"

echo -e "${BLUE}🔨 构建环保数据集成平台${NC}"
echo -e "${YELLOW}版本: ${GIT_TAG}${NC}"
echo -e "${YELLOW}提交: ${GIT_COMMIT}${NC}"
echo -e "${YELLOW}时间: ${BUILD_TIME}${NC}"
echo ""

# 检查Go环境
if ! command -v go &> /dev/null; then
    echo -e "${RED}❌ Go未安装或不在PATH中${NC}"
    exit 1
fi

echo -e "${BLUE}Go版本: $(go version)${NC}"
echo ""

# 创建构建目录
echo -e "${BLUE}📁 创建构建目录...${NC}"
mkdir -p ${BUILD_DIR}

# 下载依赖
echo -e "${BLUE}📦 下载依赖...${NC}"
go mod download
go mod tidy

# 代码检查
echo -e "${BLUE}🔍 代码检查...${NC}"
if command -v golangci-lint &> /dev/null; then
    golangci-lint run
else
    echo -e "${YELLOW}⚠️  golangci-lint未安装，跳过代码检查${NC}"
fi

# 运行测试
echo -e "${BLUE}🧪 运行测试...${NC}"
go test -v ./... || {
    echo -e "${RED}❌ 测试失败${NC}"
    exit 1
}

# 构建不同平台版本
echo -e "${BLUE}🔨 构建应用程序...${NC}"

# Linux AMD64
echo -e "${BLUE}构建 Linux AMD64...${NC}"
GOOS=linux GOARCH=amd64 go build -ldflags="${LDFLAGS}" -o ${BUILD_DIR}/${APP_NAME}-linux-amd64 ${MAIN_FILE}

# Linux ARM64
echo -e "${BLUE}构建 Linux ARM64...${NC}"
GOOS=linux GOARCH=arm64 go build -ldflags="${LDFLAGS}" -o ${BUILD_DIR}/${APP_NAME}-linux-arm64 ${MAIN_FILE}

# macOS AMD64
echo -e "${BLUE}构建 macOS AMD64...${NC}"
GOOS=darwin GOARCH=amd64 go build -ldflags="${LDFLAGS}" -o ${BUILD_DIR}/${APP_NAME}-darwin-amd64 ${MAIN_FILE}

# macOS ARM64
echo -e "${BLUE}构建 macOS ARM64...${NC}"
GOOS=darwin GOARCH=arm64 go build -ldflags="${LDFLAGS}" -o ${BUILD_DIR}/${APP_NAME}-darwin-arm64 ${MAIN_FILE}

# Windows AMD64
echo -e "${BLUE}构建 Windows AMD64...${NC}"
GOOS=windows GOARCH=amd64 go build -ldflags="${LDFLAGS}" -o ${BUILD_DIR}/${APP_NAME}-windows-amd64.exe ${MAIN_FILE}

# 当前平台
echo -e "${BLUE}构建当前平台...${NC}"
go build -ldflags="${LDFLAGS}" -o ${BUILD_DIR}/${APP_NAME} ${MAIN_FILE}

# 显示构建结果
echo ""
echo -e "${GREEN}✅ 构建完成！${NC}"
echo -e "${BLUE}构建文件:${NC}"
ls -la ${BUILD_DIR}/

# 文件大小统计
echo ""
echo -e "${BLUE}📊 文件大小:${NC}"
for file in ${BUILD_DIR}/${APP_NAME}*; do
    if [ -f "$file" ]; then
        size=$(ls -lh "$file" | awk '{print $5}')
        echo -e "${YELLOW}$(basename "$file"): ${size}${NC}"
    fi
done

echo ""
echo -e "${GREEN}🎉 构建流程完成！${NC}"