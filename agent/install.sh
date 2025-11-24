#!/bin/bash
# SmartDNS Log Agent 简化安装脚本
set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# 全局变量
GITHUB_REPO="almightyyantao/smartdns-manager"
BINARY_NAME="smartdns-log-agent"
SERVICE_NAME="smartdns-log-agent"
PROXY_URL=""
ORIGINAL_PROXY=""

log() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_help() {
    cat << EOF
用法: $0 [选项]

必需参数:
  -n, --node-id ID        节点ID
  -H, --clickhouse-host HOST    ClickHouse 主机地址

可选参数:
  -N, --node-name NAME    节点名称 (默认: node-ID)
  -P, --clickhouse-port PORT    ClickHouse 端口 (默认: 9000)
  -d, --clickhouse-db DB        数据库名 (默认: smartdns_logs)
  -u, --clickhouse-user USER    用户名 (默认: default)
  -p, --clickhouse-password PWD 密码
  -l, --log-file PATH     日志文件路径 (默认: /var/log/audit/audit.log)
  -m, --mode MODE         部署模式: systemd|docker (默认: systemd)
  --proxy URL             代理地址 (格式: socks5://host:port 或 http://user:pass@host:port)
  --uninstall             卸载 Agent
  -h, --help              显示帮助

代理示例:
  --proxy socks5://127.0.0.1:1080
  --proxy http://user:pass@proxy.company.com:8080
  --proxy socks5://proxyuser:proxypass@proxy.example.com:1080

示例:
  $0 -n 1 -H 192.168.1.100 -p password123
  $0 -n 2 -H clickhouse.example.com -m docker -p secret --proxy socks5://127.0.0.1:1080
  $0 --uninstall
EOF
}

setup_proxy() {
    if [ -n "$PROXY_URL" ]; then
        log "配置代理: $PROXY_URL"

        # 备份原有代理设置
        ORIGINAL_HTTP_PROXY=${http_proxy:-}
        ORIGINAL_HTTPS_PROXY=${https_proxy:-}

        # 设置代理环境变量
        export http_proxy="$PROXY_URL"
        export https_proxy="$PROXY_URL"
        export HTTP_PROXY="$PROXY_URL"
        export HTTPS_PROXY="$PROXY_URL"

        # 测试代理连接
        test_proxy_connection
    fi
}

restore_proxy() {
    if [ -n "$PROXY_URL" ]; then
        # 恢复原有代理设置
        if [ -n "$ORIGINAL_HTTP_PROXY" ]; then
            export http_proxy="$ORIGINAL_HTTP_PROXY"
            export HTTP_PROXY="$ORIGINAL_HTTP_PROXY"
        else
            unset http_proxy HTTP_PROXY
        fi

        if [ -n "$ORIGINAL_HTTPS_PROXY" ]; then
            export https_proxy="$ORIGINAL_HTTPS_PROXY"
            export HTTPS_PROXY="$ORIGINAL_HTTPS_PROXY"
        else
            unset https_proxy HTTPS_PROXY
        fi
    fi
}

test_proxy_connection() {
    log "测试代理连接..."

    if command -v curl >/dev/null 2>&1; then
        if curl --proxy "$PROXY_URL" -s --max-time 10 --head https://www.google.com >/dev/null 2>&1; then
            log "代理连接测试成功"
        else
            warn "代理连接测试失败，但继续执行安装"
        fi
    elif command -v wget >/dev/null 2>&1; then
        # wget 的代理设置方式不同，通过环境变量已经设置
        if wget --spider --quiet --timeout=10 https://www.google.com 2>/dev/null; then
            log "代理连接测试成功"
        else
            warn "代理连接测试失败，但继续执行安装"
        fi
    fi
}

check_root() {
    if [ "$EUID" -ne 0 ]; then
        error "请使用 root 权限运行此脚本"
        exit 1
    fi
}

detect_arch() {
    ARCH=$(uname -m)
    case $ARCH in
        x86_64)
            BINARY_ARCH="linux-amd64"
            ;;
        aarch64|arm64)
            BINARY_ARCH="linux-arm64"
            ;;
        armv7l)
            BINARY_ARCH="linux-armv7"
            ;;
        *)
            error "不支持的架构: $ARCH"
            exit 1
            ;;
    esac
}

download_agent() {
    log "下载 SmartDNS Log Agent..."

    TEMP_DIR=$(mktemp -d)
    cd "$TEMP_DIR"

    # 下载最新版本
    DOWNLOAD_URL="https://github.com/${GITHUB_REPO}/releases/latest/download/${BINARY_NAME}-${BINARY_ARCH}.tar.gz"

    # 获取最新版本信息
    if command -v curl >/dev/null 2>&1; then
        if [ -n "$PROXY_URL" ]; then
            LATEST_RELEASE=$(curl --proxy "$PROXY_URL" -s "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/' 2>/dev/null || echo "")
        else
            LATEST_RELEASE=$(curl -s "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/' 2>/dev/null || echo "")
        fi

        if [ -n "$LATEST_RELEASE" ]; then
            DOWNLOAD_URL="https://github.com/${GITHUB_REPO}/releases/download/${LATEST_RELEASE}/${BINARY_NAME}-${BINARY_ARCH}.tar.gz"
        fi
    fi

    # 下载文件
    DOWNLOAD_SUCCESS=false

    # 如果有代理且是 SOCKS5，优先使用 curl
    if [ -n "$PROXY_URL" ] && echo "$PROXY_URL" | grep -q "socks5://"; then
        if command -v curl >/dev/null 2>&1; then
            log "通过代理下载..."
            # 去掉 -v 参数，只显示进度
            if curl -L --proxy "$PROXY_URL" --progress-bar "$DOWNLOAD_URL" -o agent.tar.gz; then
                DOWNLOAD_SUCCESS=true
            else
                error "curl 下载失败"
            fi
        fi
    else
        # HTTP 代理或无代理
        if command -v wget >/dev/null 2>&1; then
            # wget 默认显示简洁的进度条
            if wget --progress=bar:force "$DOWNLOAD_URL" -O agent.tar.gz 2>&1; then
                DOWNLOAD_SUCCESS=true
            else
                warn "wget 下载失败，尝试 curl..."
            fi
        fi

        # 如果 wget 失败，尝试 curl
        if [ "$DOWNLOAD_SUCCESS" = false ] && command -v curl >/dev/null 2>&1; then
            if [ -n "$PROXY_URL" ]; then
                if curl -L --proxy "$PROXY_URL" --progress-bar "$DOWNLOAD_URL" -o agent.tar.gz; then
                    DOWNLOAD_SUCCESS=true
                fi
            else
                if curl -L --progress-bar "$DOWNLOAD_URL" -o agent.tar.gz; then
                    DOWNLOAD_SUCCESS=true
                fi
            fi
        fi
    fi

    # 检查下载是否成功
    if [ "$DOWNLOAD_SUCCESS" = false ]; then
        error "下载失败"
        return 1
    fi

    # 检查下载的文件
    if [ ! -f "agent.tar.gz" ]; then
        error "下载的文件不存在"
        return 1
    fi

    # 检查文件大小
    FILE_SIZE=$(stat -c%s agent.tar.gz 2>/dev/null || stat -f%z agent.tar.gz 2>/dev/null || echo 0)

    if [ "$FILE_SIZE" -lt 1000 ]; then
        error "下载的文件太小，可能下载失败"
        return 1
    fi

    log "下载完成，文件大小: ${FILE_SIZE} bytes"

    # 解压文件
    log "解压文件..."
    if ! tar -xzf agent.tar.gz 2>/dev/null; then
        error "解压失败"
        return 1
    fi

    # 检查解压后的文件
    EXPECTED_BINARY="${BINARY_NAME}-${BINARY_ARCH}"
    if [ ! -f "$EXPECTED_BINARY" ]; then
        error "找不到预期的二进制文件: $EXPECTED_BINARY"
        log "当前目录内容:"
        ls -la
        return 1
    fi

    log "文件解压成功"
    return 0
}

install_systemd() {
    log "安装 systemd 服务..."

    # 复制二进制文件
    cp "${BINARY_NAME}-${BINARY_ARCH}" "/usr/local/bin/${BINARY_NAME}"
    chmod +x "/usr/local/bin/${BINARY_NAME}"

    # 创建配置目录和文件
    mkdir -p /etc/smartdns-log-agent
    cat > /etc/smartdns-log-agent/config << EOF
NODE_ID=${NODE_ID}
NODE_NAME=${NODE_NAME}
LOG_FILE=${LOG_FILE}
BATCH_SIZE=1000
FLUSH_INTERVAL_SEC=2
CLICKHOUSE_HOST=${CLICKHOUSE_HOST}
CLICKHOUSE_PORT=${CLICKHOUSE_PORT}
CLICKHOUSE_DB=${CLICKHOUSE_DB}
CLICKHOUSE_USER=${CLICKHOUSE_USER}
CLICKHOUSE_PASSWORD=${CLICKHOUSE_PASSWORD}
TZ=Asia/Shanghai
AGENT_LOG_DIR=/var/log/smartdns-agent
AGENT_LOG_MAX_DAYS=7
AGENT_LOG_ENABLE_FILE=true
EOF

    # 如果设置了代理，添加到配置文件
    if [ -n "$PROXY_URL" ]; then
        echo "HTTP_PROXY=${PROXY_URL}" >> /etc/smartdns-log-agent/config
        echo "HTTPS_PROXY=${PROXY_URL}" >> /etc/smartdns-log-agent/config
    fi

    # 创建日志目录
    mkdir -p /var/log/smartdns-agent

    # 创建 systemd 服务文件
    cat > /etc/systemd/system/${SERVICE_NAME}.service << EOF
[Unit]
Description=SmartDNS Log Agent
After=network.target
Wants=network.target

[Service]
Type=simple
User=root
Group=root
ExecStart=/usr/local/bin/${BINARY_NAME}
Restart=always
RestartSec=5
EnvironmentFile=-/etc/smartdns-log-agent/config
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

    # 启动服务
    systemctl daemon-reload
    systemctl enable ${SERVICE_NAME}
    systemctl start ${SERVICE_NAME}

    log "systemd 服务安装完成"
}

install_docker() {
    log "安装 Docker 服务..."

    # 创建安装目录
    mkdir -p /opt/smartdns-log-agent
    cd /opt/smartdns-log-agent

    # 创建 docker-compose.yml
    cat > docker-compose.yml << EOF
version: '3.8'
services:
  smartdns-log-agent:
    image: ghcr.nju.edu.cn/almightyyantao/smartdns-log-agent:latest
    container_name: smartdns-log-agent-${NODE_ID}
    restart: unless-stopped
    environment:
      - NODE_ID=${NODE_ID}
      - NODE_NAME=${NODE_NAME}
      - LOG_FILE=/logs/audit.log
      - CLICKHOUSE_HOST=${CLICKHOUSE_HOST}
      - CLICKHOUSE_PORT=${CLICKHOUSE_PORT}
      - CLICKHOUSE_DB=${CLICKHOUSE_DB}
      - CLICKHOUSE_USER=${CLICKHOUSE_USER}
      - CLICKHOUSE_PASSWORD=${CLICKHOUSE_PASSWORD}
      - TZ=Asia/Shanghai
EOF

    # 如果设置了代理，添加到容器环境变量
    if [ -n "$PROXY_URL" ]; then
        cat >> docker-compose.yml << EOF
      - HTTP_PROXY=${PROXY_URL}
      - HTTPS_PROXY=${PROXY_URL}
      - http_proxy=${PROXY_URL}
      - https_proxy=${PROXY_URL}
EOF
    fi

    cat >> docker-compose.yml << EOF
    volumes:
      - $(dirname ${LOG_FILE}):/logs:ro
    network_mode: host
    user: "0:0"
EOF

    # 如果使用代理，先拉取镜像
    if [ -n "$PROXY_URL" ]; then
        log "通过代理拉取 Docker 镜像..."
        # 为 Docker daemon 配置代理（临时）
        mkdir -p /etc/systemd/system/docker.service.d
        cat > /etc/systemd/system/docker.service.d/http-proxy.conf << EOF
[Service]
Environment="HTTP_PROXY=${PROXY_URL}"
Environment="HTTPS_PROXY=${PROXY_URL}"
EOF
        systemctl daemon-reload
        systemctl restart docker
        sleep 5
    fi

    # 启动容器
    docker-compose up -d

    # 如果配置了临时代理，清理 Docker 代理配置
    if [ -n "$PROXY_URL" ]; then
        rm -f /etc/systemd/system/docker.service.d/http-proxy.conf
        systemctl daemon-reload
        # 不重启 Docker，避免影响正在运行的容器
    fi

    log "Docker 服务安装完成"
}

uninstall() {
    log "开始卸载..."

    # systemd 卸载
    if systemctl is-active ${SERVICE_NAME} >/dev/null 2>&1; then
        systemctl stop ${SERVICE_NAME}
        systemctl disable ${SERVICE_NAME}
        rm -f /etc/systemd/system/${SERVICE_NAME}.service
        rm -f /usr/local/bin/${BINARY_NAME}
        rm -rf /etc/smartdns-log-agent
        systemctl daemon-reload
        log "systemd 服务已卸载"
    fi

    # Docker 卸载
    if [ -f "/opt/smartdns-log-agent/docker-compose.yml" ]; then
        cd /opt/smartdns-log-agent
        docker-compose down
        cd /
        rm -rf /opt/smartdns-log-agent
        log "Docker 服务已卸载"
    fi

    log "卸载完成"
}

check_service() {
    if [ "$DEPLOY_MODE" = "docker" ]; then
        if [ -f "/opt/smartdns-log-agent/docker-compose.yml" ]; then
            cd /opt/smartdns-log-agent
            if docker-compose ps | grep -q "Up"; then
                log "✅ Docker 服务运行正常"
                return 0
            fi
        fi
    else
        if systemctl is-active ${SERVICE_NAME} >/dev/null 2>&1; then
            log "✅ systemd 服务运行正常"
            return 0
        fi
    fi

    error "❌ 服务启动失败"
    return 1
}

# 错误处理和清理
cleanup_on_error() {
    error "安装失败，正在清理..."
    restore_proxy
    rm -rf "$TEMP_DIR" 2>/dev/null || true
    exit 1
}

# 设置错误处理
trap cleanup_on_error ERR

# 解析参数
while [[ $# -gt 0 ]]; do
    case $1 in
        -n|--node-id)
            NODE_ID="$2"
            shift 2
            ;;
        -N|--node-name)
            NODE_NAME="$2"
            shift 2
            ;;
        -H|--clickhouse-host)
            CLICKHOUSE_HOST="$2"
            shift 2
            ;;
        -P|--clickhouse-port)
            CLICKHOUSE_PORT="$2"
            shift 2
            ;;
        -d|--clickhouse-db)
            CLICKHOUSE_DB="$2"
            shift 2
            ;;
        -u|--clickhouse-user)
            CLICKHOUSE_USER="$2"
            shift 2
            ;;
        -p|--clickhouse-password)
            CLICKHOUSE_PASSWORD="$2"
            shift 2
            ;;
        -l|--log-file)
            LOG_FILE="$2"
            shift 2
            ;;
        -m|--mode)
            DEPLOY_MODE="$2"
            shift 2
            ;;
        --proxy)
            PROXY_URL="$2"
            shift 2
            ;;
        --uninstall)
            check_root
            uninstall
            exit 0
            ;;
        -h|--help)
            print_help
            exit 0
            ;;
        *)
            error "未知选项: $1"
            print_help
            exit 1
            ;;
    esac
done

# 检查必需参数
if [ -z "$NODE_ID" ] || [ -z "$CLICKHOUSE_HOST" ]; then
    error "缺少必需参数"
    print_help
    exit 1
fi

# 设置默认值
NODE_NAME=${NODE_NAME:-"node-$NODE_ID"}
CLICKHOUSE_PORT=${CLICKHOUSE_PORT:-9000}
CLICKHOUSE_DB=${CLICKHOUSE_DB:-"smartdns_logs"}
CLICKHOUSE_USER=${CLICKHOUSE_USER:-"default"}
LOG_FILE=${LOG_FILE:-"/var/log/audit/audit.log"}
DEPLOY_MODE=${DEPLOY_MODE:-"systemd"}

# 主安装流程
echo -e "${BLUE}SmartDNS Log Agent 安装程序${NC}"
echo "节点ID: $NODE_ID"
echo "节点名称: $NODE_NAME"
echo "ClickHouse: $CLICKHOUSE_HOST:$CLICKHOUSE_PORT/$CLICKHOUSE_DB"
echo "部署模式: $DEPLOY_MODE"
if [ -n "$PROXY_URL" ]; then
    echo "代理设置: $PROXY_URL"
fi
echo ""

check_root
setup_proxy
detect_arch

if [ "$DEPLOY_MODE" = "docker" ]; then
    # 检查 Docker
    if ! command -v docker >/dev/null 2>&1 || ! command -v docker-compose >/dev/null 2>&1; then
        error "Docker 或 docker-compose 未安装"
        exit 1
    fi
    install_docker
else
    download_agent
    install_systemd
fi

# 等待服务启动
sleep 3
check_service

# 恢复代理设置
restore_proxy

echo ""
echo -e "${GREEN}🎉 安装成功！${NC}"
echo ""
echo "管理命令:"
if [ "$DEPLOY_MODE" = "docker" ]; then
    echo "  查看日志: cd /opt/smartdns-log-agent && docker-compose logs -f"
    echo "  重启服务: cd /opt/smartdns-log-agent && docker-compose restart"
else
    echo "  查看日志: journalctl -u ${SERVICE_NAME} -f"
    echo "  重启服务: systemctl restart ${SERVICE_NAME}"
fi

# 清理
rm -rf "$TEMP_DIR" 2>/dev/null || true