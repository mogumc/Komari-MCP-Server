# ─────────────────────────────────────────────────────────────
# 构建阶段
# ─────────────────────────────────────────────────────────────
FROM --platform=$BUILDPLATFORM golang:1.22-alpine AS builder

# 安装构建依赖
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

# 预加载 go.sum（利用 Docker 缓存）
COPY go.mod go.sum* ./
RUN go mod download

# 复制源码
COPY . .

# 跨平台编译
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o komari-mcp ./cmd/server

# ─────────────────────────────────────────────────────────────
# 运行阶段
# ─────────────────────────────────────────────────────────────
FROM alpine:3.19

# 安装运行时依赖
RUN apk add --no-cache ca-certificates tzdata

# 创建非 root 用户
RUN addgroup -g 1000 komari && \
    adduser -u 1000 -G komari -s /bin/sh -D komari

WORKDIR /app

# 复制二进制文件
COPY --from=builder /app/komari-mcp /usr/local/bin/

# 复制时区数据
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# 设置时区
ENV TZ=Asia/Shanghai

# 切换到非 root 用户
USER komari

# 暴露端口
EXPOSE 8080

ENTRYPOINT ["komari-mcp"]
