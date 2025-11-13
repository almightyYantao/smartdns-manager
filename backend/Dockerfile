# 多阶段构建 - 构建阶段
FROM golang:1.21-alpine AS builder

# 设置工作目录
WORKDIR /build

# 安装必要的工具
RUN apk add --no-cache git ca-certificates tzdata

# 复制 go mod 文件
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制源代码
COPY . .

# 编译
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -ldflags="-w -s" -o smartdns-manager .

# 最终运行阶段
FROM alpine:latest

# 安装运行时依赖
RUN apk --no-cache add ca-certificates openssh-client tzdata sqlite

# 设置时区
ENV TZ=Asia/Shanghai

# 创建非 root 用户
RUN addgroup -g 1000 smartdns && \
    adduser -D -u 1000 -G smartdns smartdns

# 创建必要的目录
RUN mkdir -p /app/data /app/logs && \
    chown -R smartdns:smartdns /app

# 设置工作目录
WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /build/smartdns-manager .

# 切换到非 root 用户
USER smartdns

# 暴露端口
EXPOSE 3001

# 启动命令
CMD ["./smartdns-manager"]