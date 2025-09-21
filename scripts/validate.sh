#!/bin/bash

# 项目结构验证脚本

set -e

echo "🔍 环保数据集成平台 - 项目结构验证"
echo "=================================="

# 检查关键文件
echo "📁 检查项目文件结构..."

REQUIRED_FILES=(
    "go.mod"
    "Dockerfile"
    "docker-compose.dev.yml"
    "docker-compose.prod.yml"
    "cmd/server/main.go"
    "internal/config/config.go"
    "internal/database/database.go"
    "internal/server/server.go"
    "internal/auth/jwt.go"
    "internal/auth/password.go"
    "internal/hj212/protocol.go"
    "internal/hj212/server.go"
    "internal/handlers/auth.go"
    "internal/handlers/user.go"
    "internal/handlers/hj212.go"
    "internal/middleware/auth.go"
    "internal/routes/routes.go"
    "internal/models/base.go"
    "internal/models/user.go"
    "internal/models/datasource.go"
    "internal/models/etl.go"
    "internal/logger/logger.go"
    "config/config.dev.yaml"
)

MISSING_FILES=()

for file in "${REQUIRED_FILES[@]}"; do
    if [ -f "$file" ]; then
        echo "✅ $file"
    else
        echo "❌ $file"
        MISSING_FILES+=("$file")
    fi
done

if [ ${#MISSING_FILES[@]} -eq 0 ]; then
    echo ""
    echo "🎉 所有关键文件都存在！"
else
    echo ""
    echo "⚠️  缺失 ${#MISSING_FILES[@]} 个文件:"
    printf '   - %s\n' "${MISSING_FILES[@]}"
fi

# 检查配置文件
echo ""
echo "⚙️  检查配置文件..."

if [ -f "config/config.dev.yaml" ]; then
    echo "✅ 开发环境配置文件存在"

    # 检查配置文件关键字段
    REQUIRED_CONFIGS=(
        "app:"
        "server:"
        "database:"
        "redis:"
        "jwt:"
        "log:"
        "hj212:"
    )

    for config in "${REQUIRED_CONFIGS[@]}"; do
        if grep -q "$config" config/config.dev.yaml; then
            echo "✅ 配置项: $config"
        else
            echo "❌ 配置项: $config"
        fi
    done
else
    echo "❌ 开发环境配置文件不存在"
fi

# 检查Docker配置
echo ""
echo "🐳 检查Docker配置..."

if [ -f "Dockerfile" ]; then
    echo "✅ Dockerfile存在"
    if grep -q "FROM golang" Dockerfile; then
        echo "✅ 使用Go基础镜像"
    fi
    if grep -q "WORKDIR /app" Dockerfile; then
        echo "✅ 设置工作目录"
    fi
    if grep -q "EXPOSE" Dockerfile; then
        echo "✅ 暴露端口"
    fi
else
    echo "❌ Dockerfile不存在"
fi

if [ -f "docker-compose.dev.yml" ]; then
    echo "✅ 开发环境Docker Compose配置存在"
    if grep -q "mysql:" docker-compose.dev.yml; then
        echo "✅ 包含MySQL服务"
    fi
    if grep -q "redis:" docker-compose.dev.yml; then
        echo "✅ 包含Redis服务"
    fi
    if grep -q "app:" docker-compose.dev.yml; then
        echo "✅ 包含应用服务"
    fi
else
    echo "❌ 开发环境Docker Compose配置不存在"
fi

# 检查脚本权限
echo ""
echo "🔧 检查脚本权限..."

SCRIPT_FILES=(
    "scripts/dev-start.sh"
    "scripts/dev-stop.sh"
    "scripts/build.sh"
    "scripts/deploy.sh"
)

for script in "${SCRIPT_FILES[@]}"; do
    if [ -f "$script" ]; then
        if [ -x "$script" ]; then
            echo "✅ $script (可执行)"
        else
            echo "⚠️  $script (需要执行权限)"
        fi
    else
        echo "❌ $script (不存在)"
    fi
done

# 总结
echo ""
echo "📊 验证总结"
echo "=========="

TOTAL_FILES=${#REQUIRED_FILES[@]}
EXISTING_FILES=$((TOTAL_FILES - ${#MISSING_FILES[@]}))
COMPLETION_RATE=$(echo "scale=1; $EXISTING_FILES * 100 / $TOTAL_FILES" | bc -l 2>/dev/null || echo "N/A")

echo "📁 文件完整性: $EXISTING_FILES/$TOTAL_FILES ($COMPLETION_RATE%)"

if [ ${#MISSING_FILES[@]} -eq 0 ]; then
    echo "🟢 状态: 项目结构完整，可以进行编译测试"
    echo ""
    echo "🚀 下一步操作建议："
    echo "   1. 确保安装了Go 1.21+"
    echo "   2. 运行: go mod tidy"
    echo "   3. 运行: go build ./cmd/server"
    echo "   4. 或使用Docker: ./scripts/dev-start.sh"
else
    echo "🟡 状态: 项目结构基本完整，但有缺失文件"
    echo ""
    echo "🔧 需要处理："
    printf '   - 补充缺失文件: %s\n' "${MISSING_FILES[@]}"
fi

echo ""
echo "✨ 验证完成！"