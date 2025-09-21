# 多阶段构建 - 构建阶段
FROM golang:1.21-alpine AS builder

# 设置工作目录
WORKDIR /app

# 安装构建依赖
RUN apk add --no-cache git ca-certificates tzdata make

# 复制go模块文件
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制源代码
COPY . .

# 构建应用
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -o env-data-platform \
    cmd/server/main.go

# 运行阶段
FROM alpine:3.18

# 安装运行时依赖
RUN apk --no-cache add ca-certificates tzdata curl

# 创建应用用户
RUN addgroup -g 1001 app && \
    adduser -D -s /bin/sh -u 1001 -G app app

# 设置工作目录
WORKDIR /app

# 从构建阶段复制文件
COPY --from=builder /app/env-data-platform /app/
COPY --from=builder /app/config /app/config/

# 创建必要目录
RUN mkdir -p /app/logs /app/uploads /app/temp /app/static && \
    chown -R app:app /app

# 设置环境变量
ENV TZ=Asia/Shanghai
ENV CONFIG_PATH=/app/config/config.yaml

# 切换到应用用户
USER app

# 暴露端口
EXPOSE 8888 8212

# 健康检查
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8888/health || exit 1

# 启动应用
ENTRYPOINT ["/app/env-data-platform"]
CMD ["-config", "/app/config/config.yaml"]

# 构建信息标签
LABEL maintainer="env-data-platform team <dev@env-data-platform.com>"
LABEL version="1.0.0"
LABEL description="环保数据集成平台主服务"