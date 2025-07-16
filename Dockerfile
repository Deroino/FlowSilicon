# 使用多平台 Golang 镜像作为构建环境
FROM --platform=$BUILDPLATFORM golang:1.23.7-bookworm AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH \
    go build -ldflags "-s -w" -o flowsilicon ./cmd/flowsilicon/linux/main_linux.go

# 使用 Debian 稳定版作为运行时
FROM debian:stable-slim

# 设置环境变量
ENV TZ=Asia/Shanghai \
    DEBIAN_FRONTEND=noninteractive

# 安装基础依赖（带重试机制）
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
    ca-certificates \
    tzdata && \
    rm -rf /var/lib/apt/lists/* && \
    ln -snf /usr/share/zoneinfo/$TZ /etc/localtime && \
    echo $TZ > /etc/timezone

# 创建应用用户和目录
RUN useradd -u 1000 -U -d /app -s /bin/false flowsilicon && \
    mkdir -p /app/data /app/logs

COPY --from=builder --chown=flowsilicon /app/flowsilicon /app/
RUN chown -R flowsilicon:flowsilicon /app

USER flowsilicon
WORKDIR /app
EXPOSE 3016
ENV FLOWSILICON_GUI=0
CMD ["./flowsilicon"]