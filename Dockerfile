# ============================================
# 阶段 1: 构建前端
# ============================================
FROM node:18-alpine AS frontend-builder

WORKDIR /frontend

# 复制前端依赖文件
COPY ui/package*.json ./

# 安装依赖
RUN npm ci --production=false

# 复制前端源码
COPY ui/ ./

# 构建生产版本
RUN npm run build

# ============================================
# 阶段 2: 构建后端
# ============================================
FROM golang:1.21-alpine AS backend-builder

WORKDIR /build

# 安装构建依赖
RUN apk add --no-cache git ca-certificates tzdata gcc musl-dev

# 复制 Go 依赖文件
COPY backend/go.mod backend/go.sum ./

# 下载依赖
RUN go mod download

# 复制后端源码
COPY backend/ ./

# 编译 Go 程序
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo \
    -ldflags="-w -s" -o smartdns-manager .

# ============================================
# 阶段 3: 最终运行镜像
# ============================================
FROM nginx:alpine

# 安装运行时依赖
RUN apk --no-cache add ca-certificates openssh-client tzdata sqlite supervisor

# 设置时区
ENV TZ=Asia/Shanghai

# 创建应用用户
RUN addgroup -g 1000 smartdns && \
    adduser -D -u 1000 -G smartdns smartdns

# 创建必要的目录
RUN mkdir -p /app/data /app/logs && \
    chown -R smartdns:smartdns /app

# 复制前端构建产物到 Nginx 目录
COPY --from=frontend-builder /frontend/build /usr/share/nginx/html

# 复制后端二进制文件
COPY --from=backend-builder /build/smartdns-manager /app/

# 复制配置文件
COPY nginx.conf /etc/nginx/nginx.conf
COPY supervisord.conf /etc/supervisord.conf

# 暴露端口
EXPOSE 80

# 使用 Supervisor 管理多进程
CMD ["/usr/bin/supervisord", "-c", "/etc/supervisord.conf"]