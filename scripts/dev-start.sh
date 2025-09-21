#!/bin/bash

# 开发环境启动脚本

set -e

echo "🚀 启动环保数据集成平台开发环境..."

# 检查Docker是否运行
if ! docker info &> /dev/null; then
    echo "❌ Docker未运行，请先启动Docker"
    exit 1
fi

# 检查docker-compose是否存在
if ! command -v docker-compose &> /dev/null; then
    echo "❌ docker-compose未安装"
    exit 1
fi

# 创建必要的目录
echo "📁 创建必要的目录..."
mkdir -p logs uploads temp static
mkdir -p secrets

# 生成开发环境密钥文件（如果不存在）
if [ ! -f ".env" ]; then
    echo "📝 创建环境变量文件..."
    cp .env.example .env
    echo "⚠️  请检查并修改 .env 文件中的配置"
fi

# 停止现有容器
echo "🛑 停止现有容器..."
docker-compose -f docker-compose.dev.yml down

# 清理旧的卷（可选）
# echo "🧹 清理旧数据..."
# docker-compose -f docker-compose.dev.yml down -v

# 构建镜像
echo "🔨 构建应用镜像..."
docker-compose -f docker-compose.dev.yml build

# 启动服务
echo "▶️  启动开发环境..."
docker-compose -f docker-compose.dev.yml up -d

# 等待服务启动
echo "⏳ 等待服务启动..."
sleep 10

# 检查服务状态
echo "🔍 检查服务状态..."
docker-compose -f docker-compose.dev.yml ps

# 等待数据库就绪
echo "⏳ 等待数据库就绪..."
until docker-compose -f docker-compose.dev.yml exec mysql mysqladmin ping -h localhost -u root -pdev_password --silent; do
    echo "等待MySQL启动..."
    sleep 2
done

# 运行数据库迁移
echo "🗄️  执行数据库迁移..."
docker-compose -f docker-compose.dev.yml exec app /app/env-data-platform -config /app/config/config.dev.yaml -migrate

# 初始化基础数据
echo "📊 初始化基础数据..."
docker-compose -f docker-compose.dev.yml exec app /app/env-data-platform -config /app/config/config.dev.yaml -init

echo ""
echo "✅ 开发环境启动完成！"
echo ""
echo "🌐 服务地址："
echo "   应用程序: http://localhost:8080"
echo "   Prometheus: http://localhost:9090"
echo "   Grafana: http://localhost:3000 (admin/dev_admin)"
echo ""
echo "📝 查看日志："
echo "   docker-compose -f docker-compose.dev.yml logs -f"
echo ""
echo "🛑 停止环境："
echo "   docker-compose -f docker-compose.dev.yml down"