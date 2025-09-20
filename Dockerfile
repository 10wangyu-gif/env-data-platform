# 多阶段构建 - 构建阶段
FROM golang:1.21-alpine AS builder

# 设置工作目录
WORKDIR /app

# 安装构建依赖
RUN apk add --no-cache git ca-certificates tzdata

# 复制go模块文件
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制源代码
COPY . .

# 构建应用
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -o gateway \
    cmd/gateway/main.go

# 运行阶段
FROM scratch

# 从构建阶段复制必要文件
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /app/gateway /gateway
COPY --from=builder /app/config /config

# 设置环境变量
ENV TZ=Asia/Shanghai
ENV CONFIG_PATH=/config/gateway.yaml

# 暴露端口
EXPOSE 8080

# 健康检查
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD ["/gateway", "--health-check"] || exit 1

# 设置用户（安全最佳实践）
USER 65534

# 启动应用
ENTRYPOINT ["/gateway"]

# 构建信息标签
LABEL maintainer="env-data-platform team <dev@env-data-platform.com>"
LABEL version="1.0.0"
LABEL description="环保数据集成平台 API网关"