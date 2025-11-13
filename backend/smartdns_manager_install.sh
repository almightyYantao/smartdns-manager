#!/bin/bash

# 检查是否以 root 权限运行
if [ "$EUID" -ne 0 ]; then
  echo "请以 root 权限运行此脚本"
  exit 1
fi

# 设置变量
BINARY_URL="https://it-service-cos.kujiale.com/xiaoku/smartdns_manager"
INSTALL_DIR="/usr/local/bin"
SMARTDNS_CONF_DIR="/etc/smartdns"
SERVICE_FILE="/etc/systemd/system/smartdns-manager.service"

mkdir -p /var/lib/smartdns-manager
touch /var/lib/smartdns-manager/last_sync_time

# 解析命令行参数
while [[ $# -gt 0 ]]; do
  key="$1"
  case $key in
    --type)
      NODE_TYPE="$2"
      shift 2
      ;;
    --master-url)
      MASTER_URL="$2"
      shift 2
      ;;
    --listen)
      LISTEN_ADDR="$2"
      shift 2
      ;;
    --webhook-url)
      WEBHOOK_URL="$2"
      shift 2
      ;;
    --sync-interval)
      SYNC_INTERVAL="$2"
      shift 2
      ;;
    *)
      echo "未知选项: $1"
      exit 1
      ;;
  esac
done

# 检查必要参数
if [ -z "$NODE_TYPE" ]; then
  echo "请指定节点类型 (--type master 或 --type slave)"
  exit 1
fi

if [ "$NODE_TYPE" = "slave" ] && [ -z "$MASTER_URL" ]; then
  echo "从节点必须指定主节点 URL (--master-url)"
  exit 1
fi

# 创建必要的目录
mkdir -p $SMARTDNS_CONF_DIR

# 下载 smartdns_manager 二进制文件
echo "正在下载 SmartDNS Manager..."
wget $BINARY_URL -O $INSTALL_DIR/smartdns_manager
chmod +x $INSTALL_DIR/smartdns_manager

# 创建服务文件
echo "创建系统服务..."
cat > $SERVICE_FILE <<EOL
[Unit]
Description=SmartDNS Manager
After=network.target

[Service]
Type=simple
ExecStart=$INSTALL_DIR/smartdns_manager \\
EOL

# 添加通用参数
echo "    -type=$NODE_TYPE \\" >> $SERVICE_FILE
echo "    -smartdns-conf=$SMARTDNS_CONF_DIR/smartdns.conf \\" >> $SERVICE_FILE
echo "    -oversea-list=$SMARTDNS_CONF_DIR/oversea-list.conf \\" >> $SERVICE_FILE

# 添加特定于节点类型的参数
if [ "$NODE_TYPE" = "master" ]; then
  echo "    -listen=${LISTEN_ADDR:-:8080} \\" >> $SERVICE_FILE
  [ ! -z "$WEBHOOK_URL" ] && echo "    -webhook-url=$WEBHOOK_URL" >> $SERVICE_FILE
elif [ "$NODE_TYPE" = "slave" ]; then
  echo "    -master-url=$MASTER_URL \\" >> $SERVICE_FILE
  [ ! -z "$SYNC_INTERVAL" ] && echo "    -sync-interval=$SYNC_INTERVAL" >> $SERVICE_FILE
fi

echo "Restart=always

[Install]
WantedBy=multi-user.target
" >> $SERVICE_FILE

# 重新加载 systemd，启用并启动服务
systemctl daemon-reload
systemctl enable smartdns-manager.service
systemctl start smartdns-manager.service

echo "安装完成！"
echo "SmartDNS Manager 已安装并设置为系统服务。"
echo "您可以使用以下命令管理服务："
echo "  systemctl start/stop/restart smartdns-manager"
echo "  systemctl status smartdns-manager"

if [ "$NODE_TYPE" = "master" ]; then
  echo "主节点已启动，监听地址: ${LISTEN_ADDR:-:8080}"
elif [ "$NODE_TYPE" = "slave" ]; then
  echo "从节点已启动，连接到主节点: $MASTER_URL"
fi