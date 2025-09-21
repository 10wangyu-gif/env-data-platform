#!/bin/bash

# 开发环境停止脚本

set -e

echo "🛑 停止环保数据集成平台开发环境..."

# 停止并删除容器
docker-compose -f docker-compose.dev.yml down

echo "✅ 开发环境已停止"

# 询问是否清理数据
read -p "是否清理数据卷？(y/N): " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo "🧹 清理数据卷..."
    docker-compose -f docker-compose.dev.yml down -v
    echo "✅ 数据卷已清理"
fi

# 询问是否清理镜像
read -p "是否清理构建的镜像？(y/N): " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo "🧹 清理镜像..."
    docker rmi env-data-platform:latest 2>/dev/null || true
    echo "✅ 镜像已清理"
fi

echo "🏁 清理完成"