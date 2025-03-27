# 使用官方Golang镜像作为构建环境
FROM golang:1.23.7-alpine AS builder

# 设置工作目录
WORKDIR /app

# 复制go mod和sum文件
COPY go.mod go.sum ./

# 下载依赖
RUN go env -w GOPROXY=https://mirrors.aliyun.com/goproxy/,direct
RUN go mod download

# 复制源代码
COPY . .

# 构建应用
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w" -o flowsilicon ./cmd/flowsilicon/linux/main_linux.go


# 使用轻量级的alpine镜像作为运行环境
FROM alpine:latest

# 安装必要的系统包
RUN apk --no-cache add ca-certificates tzdata

# 设置时区
ENV TZ=Asia/Shanghai

# 创建非root用户
RUN adduser -D -g '' flowsilicon

# 创建必要的目录
RUN mkdir -p /app/data /app/logs /app/web/static /app/web/templates

# 从builder阶段复制编译好的应用
COPY --from=builder /app/flowsilicon /app/
COPY --from=builder /app/web/static /app/web/static
COPY --from=builder /app/web/templates /app/web/templates

# 设置目录权限
RUN chown -R flowsilicon:flowsilicon /app

# 切换到非root用户
USER flowsilicon

# 设置工作目录
WORKDIR /app

# 暴露端口
EXPOSE 3016

# 设置环境变量
ENV FLOWSILICON_GUI=0

# 启动应用
CMD ["./flowsilicon"] 