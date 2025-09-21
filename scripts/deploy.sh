#!/bin/bash

# 生产环境部署脚本

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 配置
APP_NAME="env-data-platform"
COMPOSE_FILE="docker-compose.prod.yml"
BACKUP_DIR="backups"
SECRETS_DIR="secrets"

echo -e "${BLUE}🚀 部署环保数据集成平台到生产环境${NC}"
echo ""

# 检查是否为root用户
if [[ $EUID -eq 0 ]]; then
   echo -e "${RED}❌ 请不要使用root用户运行此脚本${NC}"
   exit 1
fi

# 检查Docker和docker-compose
if ! command -v docker &> /dev/null; then
    echo -e "${RED}❌ Docker未安装${NC}"
    exit 1
fi

if ! command -v docker-compose &> /dev/null; then
    echo -e "${RED}❌ docker-compose未安装${NC}"
    exit 1
fi

# 检查是否存在生产配置
if [ ! -f "${COMPOSE_FILE}" ]; then
    echo -e "${RED}❌ 生产环境配置文件 ${COMPOSE_FILE} 不存在${NC}"
    exit 1
fi

# 创建必要目录
echo -e "${BLUE}📁 创建必要目录...${NC}"
mkdir -p logs uploads temp static ${BACKUP_DIR} ${SECRETS_DIR}

# 检查密钥文件
echo -e "${BLUE}🔐 检查密钥文件...${NC}"
REQUIRED_SECRETS=(
    "mysql_root_password.txt"
    "mysql_password.txt"
    "redis_password.txt"
    "jwt_secret.txt"
    "grafana_admin_password.txt"
)

for secret in "${REQUIRED_SECRETS[@]}"; do
    if [ ! -f "${SECRETS_DIR}/${secret}" ]; then
        echo -e "${YELLOW}⚠️  生成密钥文件: ${secret}${NC}"
        case "$secret" in
            "mysql_root_password.txt")
                openssl rand -base64 32 > "${SECRETS_DIR}/${secret}"
                ;;
            "mysql_password.txt")
                openssl rand -base64 24 > "${SECRETS_DIR}/${secret}"
                ;;
            "redis_password.txt")
                openssl rand -base64 24 > "${SECRETS_DIR}/${secret}"
                ;;
            "jwt_secret.txt")
                openssl rand -base64 64 > "${SECRETS_DIR}/${secret}"
                ;;
            "grafana_admin_password.txt")
                openssl rand -base64 16 > "${SECRETS_DIR}/${secret}"
                ;;
        esac
        chmod 600 "${SECRETS_DIR}/${secret}"
    fi
done

# 备份当前数据（如果存在）
if docker volume ls | grep -q "${APP_NAME}_mysql_data_prod"; then
    echo -e "${BLUE}💾 备份当前数据...${NC}"
    BACKUP_FILE="${BACKUP_DIR}/backup-$(date +%Y%m%d-%H%M%S).sql"

    # 创建数据库备份
    docker-compose -f ${COMPOSE_FILE} exec -T mysql mysqldump \
        -u root -p$(cat ${SECRETS_DIR}/mysql_root_password.txt) \
        --all-databases --routines --triggers > "${BACKUP_FILE}" 2>/dev/null || true

    if [ -f "${BACKUP_FILE}" ]; then
        echo -e "${GREEN}✅ 数据库备份完成: ${BACKUP_FILE}${NC}"
    fi
fi

# 拉取最新镜像
echo -e "${BLUE}📥 拉取最新镜像...${NC}"
docker-compose -f ${COMPOSE_FILE} pull

# 构建应用镜像
echo -e "${BLUE}🔨 构建应用镜像...${NC}"
docker build -t ${APP_NAME}:latest .

# 停止现有服务
echo -e "${BLUE}🛑 停止现有服务...${NC}"
docker-compose -f ${COMPOSE_FILE} down

# 启动新服务
echo -e "${BLUE}▶️  启动生产服务...${NC}"
docker-compose -f ${COMPOSE_FILE} up -d

# 等待服务启动
echo -e "${BLUE}⏳ 等待服务启动...${NC}"
sleep 30

# 检查服务状态
echo -e "${BLUE}🔍 检查服务状态...${NC}"
docker-compose -f ${COMPOSE_FILE} ps

# 运行健康检查
echo -e "${BLUE}🏥 运行健康检查...${NC}"
HEALTH_URL="http://localhost:8080/health"
RETRY_COUNT=0
MAX_RETRIES=10

while [ $RETRY_COUNT -lt $MAX_RETRIES ]; do
    if curl -f -s "${HEALTH_URL}" > /dev/null; then
        echo -e "${GREEN}✅ 应用程序健康检查通过${NC}"
        break
    else
        echo -e "${YELLOW}⏳ 等待应用程序启动... (${RETRY_COUNT}/${MAX_RETRIES})${NC}"
        sleep 10
        RETRY_COUNT=$((RETRY_COUNT + 1))
    fi
done

if [ $RETRY_COUNT -eq $MAX_RETRIES ]; then
    echo -e "${RED}❌ 应用程序健康检查失败${NC}"
    echo -e "${BLUE}查看日志:${NC}"
    docker-compose -f ${COMPOSE_FILE} logs app
    exit 1
fi

# 清理旧镜像
echo -e "${BLUE}🧹 清理旧镜像...${NC}"
docker image prune -f

echo ""
echo -e "${GREEN}✅ 生产环境部署完成！${NC}"
echo ""
echo -e "${BLUE}🌐 服务信息:${NC}"
echo -e "${YELLOW}应用程序: http://localhost:8080${NC}"
echo -e "${YELLOW}监控面板: http://localhost:9090 (Prometheus)${NC}"
echo -e "${YELLOW}数据可视化: http://localhost:3000 (Grafana)${NC}"
echo ""
echo -e "${BLUE}📝 管理命令:${NC}"
echo -e "${YELLOW}查看日志: docker-compose -f ${COMPOSE_FILE} logs -f${NC}"
echo -e "${YELLOW}重启服务: docker-compose -f ${COMPOSE_FILE} restart${NC}"
echo -e "${YELLOW}停止服务: docker-compose -f ${COMPOSE_FILE} down${NC}"
echo ""
echo -e "${BLUE}🔐 密钥文件位置: ${SECRETS_DIR}/${NC}"
echo -e "${RED}⚠️  请妥善保管密钥文件！${NC}"