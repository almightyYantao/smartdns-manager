# SmartDNS Log Agent

[![Author](https://img.shields.io/badge/Author-yantao-blue.svg?style=flat-square)](https://github.com/almightyyantao)
[![Stars](https://img.shields.io/github/stars/almightyyantao/smartdns-manager?style=flat-square&logo=github)](https://github.com/almightyyantao/smartdns-manager/stargazers)

SmartDNS 日志采集代理，用于实时收集 SmartDNS DNS 查询日志并存储到 ClickHouse 数据库中，提供高性能的日志分析和查询能力。

## 📋 功能特性

- 🚀 **实时日志采集** - 监控 SmartDNS 日志文件变化，实时解析和上报
- 📊 **高性能存储** - 基于 ClickHouse 列式数据库，支持海量日志存储和快速查询
- 🔄 **批量处理** - 智能批量插入，减少数据库压力，提高写入性能
- 🛡️ **故障恢复** - 自动重连机制，支持日志文件轮转，确保数据不丢失
- 🐳 **多种部署** - 支持 systemd 服务和 Docker 容器两种部署方式
- 🔧 **零配置启动** - 自动创建 ClickHouse 表结构和物化视图
- 📈 **多节点支持** - 支持多节点统一管理，便于分布式部署
- 🎯 **轻量级** - 单个二进制文件，资源占用少，部署简单

## 🏗️ 架构图

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   SmartDNS      │    │  Log Agent      │    │   ClickHouse    │
│                 │    │                 │    │                 │
│ DNS Query Logs  │───▶│  实时监控解析    │───▶│   高性能存储     │
│ audit.log       │    │  批量发送        │    │   自动建表      │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                                                        │
                                                        ▼
                                              ┌─────────────────┐
                                              │   管理后端      │
                                              │  数据查询分析   │
                                              └─────────────────┘
```

## 🚀 快速开始

### 一键安装

```bash
curl -sSL https://raw.githubusercontent.com/almightyYantao/smartdns-manager/refs/heads/main/agent/install.sh | sudo bash -s -- -n 1 -H your-clickhouse-host -u smartdns -d smartdns_logs -p your-password
```

**参数说明：**
- `-n 1`：节点ID
- `-H your-clickhouse-host`：ClickHouse 主机地址
- `-u smartdns`：ClickHouse 用户名
- `-d smartdns_logs`：ClickHouse 数据库名
- `-p your-password`：ClickHouse 密码

### 交互式安装

```bash
curl -sSL https://raw.githubusercontent.com/almightyYantao/smartdns-manager/refs/heads/main/agent/install.sh | sudo bash
```

## 📦 安装方式

### 方式一：systemd 服务（推荐）

适用于传统 Linux 服务器：

```bash
# 完整参数安装
sudo ./install.sh \
  --mode systemd \
  --node-id 1 \
  --node-name "主节点" \
  --log-file "/var/log/audit/audit.log" \
  --clickhouse-host "192.168.1.100" \
  --clickhouse-user "smartdns" \
  --clickhouse-db "smartdns_logs" \
  --clickhouse-password "your-password"

# 服务管理
sudo systemctl start smartdns-log-agent
sudo systemctl enable smartdns-log-agent
sudo systemctl status smartdns-log-agent
```

### 方式二：Docker 容器

适用于容器化环境：

```bash
# Docker 方式安装
sudo ./install.sh --mode docker -n 1 -H clickhouse-host -p password

# 服务管理
cd /opt/smartdns-log-agent
docker-compose up -d
docker-compose logs -f
```

### 方式三：手动部署

1. **下载二进制文件**

```bash
# 下载最新版本
wget https://github.com/almightyyantao/smartdns-log-agent/releases/latest/download/smartdns-log-agent-linux-amd64.tar.gz

# 解压
tar -xzf smartdns-log-agent-linux-amd64.tar.gz
```

2. **配置环境变量**

```bash
export NODE_ID=1
export NODE_NAME="node-1"
export LOG_FILE="/var/log/audit/audit.log"
export CLICKHOUSE_HOST="192.168.1.100"
export CLICKHOUSE_PORT=9000
export CLICKHOUSE_DB="smartdns_logs"
export CLICKHOUSE_USER="smartdns"
export CLICKHOUSE_PASSWORD="your-password"
```

3. **运行**

```bash
sudo ./smartdns-log-agent-linux-amd64
```

## ⚙️ 配置说明

### 环境变量

| 变量名 | 默认值 | 说明 |
|--------|--------|------|
| `NODE_ID` | - | 节点ID（必需） |
| `NODE_NAME` | `node-{id}` | 节点名称 |
| `LOG_FILE` | `/var/log/audit/audit.log` | SmartDNS 日志文件路径 |
| `BATCH_SIZE` | `1000` | 批量插入大小 |
| `FLUSH_INTERVAL_SEC` | `2` | 刷新间隔（秒） |
| `CLICKHOUSE_HOST` | - | ClickHouse 主机地址（必需） |
| `CLICKHOUSE_PORT` | `9000` | ClickHouse 端口 |
| `CLICKHOUSE_DB` | `smartdns_logs` | ClickHouse 数据库 |
| `CLICKHOUSE_USER` | `default` | ClickHouse 用户名 |
| `CLICKHOUSE_PASSWORD` | - | ClickHouse 密码 |

### SmartDNS 日志格式

Agent 支持解析以下格式的 SmartDNS 日志：

```
[2025-11-21 05:33:18,910] 10.1.102.201 query v2ray.com, type 1, time 63ms, speed: 29.4ms, result 172.67.149.148
[2025-11-21 05:33:19,011] 10.1.102.201 query v2raycn.com, type 1, time 99ms, speed: 28.8ms, result 172.67.180.29
```

## 📊 数据库表结构

Agent 会自动创建以下表结构：

### 主表：`dns_query_log`

```sql
CREATE TABLE dns_query_log (
    timestamp DateTime64(3) COMMENT '查询时间',
    date Date DEFAULT toDate(timestamp) COMMENT '日期分区',
    node_id UInt32 COMMENT '节点ID',
    client_ip String COMMENT '客户端IP',
    domain String COMMENT '查询域名',
    query_type UInt16 COMMENT '查询类型',
    time_ms UInt32 COMMENT '查询耗时(ms)',
    speed_ms Float32 COMMENT '速度检查耗时(ms)',
    result_count UInt8 COMMENT '返回IP数量',
    result_ips Array(String) COMMENT 'IP列表',
    raw_log String COMMENT '原始日志'
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(date)
ORDER BY (date, node_id, timestamp)
TTL date + INTERVAL 30 DAY;
```

### 物化视图（自动创建）

- `dns_hourly_stats` - 按小时统计
- `dns_top_domains` - 热门域名统计  
- `dns_client_stats` - 客户端统计

## 🔧 管理命令

### systemd 方式

```bash
# 查看状态
sudo systemctl status smartdns-log-agent

# 查看实时日志
sudo journalctl -u smartdns-log-agent -f

# 启动/停止/重启
sudo systemctl start smartdns-log-agent
sudo systemctl stop smartdns-log-agent
sudo systemctl restart smartdns-log-agent

# 开机自启
sudo systemctl enable smartdns-log-agent
```

### Docker 方式

```bash
cd /opt/smartdns-log-agent

# 查看状态
docker-compose ps

# 查看实时日志
docker-compose logs -f

# 启动/停止/重启
docker-compose up -d
docker-compose down
docker-compose restart

# 使用管理脚本
./manage.sh {start|stop|restart|logs|status|update}
```

## 📈 性能优化

### 批量配置

```bash
# 高吞吐量配置
export BATCH_SIZE=5000
export FLUSH_INTERVAL_SEC=1

# 低延迟配置  
export BATCH_SIZE=100
export FLUSH_INTERVAL_SEC=1
```

### ClickHouse 优化

```sql
-- 优化配置示例
SET max_insert_threads = 4;
SET max_memory_usage = 10000000000;
```

## 🛠️ 故障排除

### 常见问题

1. **日志文件权限问题**
```bash
# 检查文件权限
ls -la /var/log/audit/audit.log

# 如果需要，调整权限
sudo chmod 644 /var/log/audit/audit.log
```

2. **ClickHouse 连接失败**
```bash
# 测试连接
telnet clickhouse-host 9000

# 检查防火墙
sudo ufw status
```

3. **服务无法启动**
```bash
# 查看详细错误日志
sudo journalctl -u smartdns-log-agent -n 50

# 检查配置文件
sudo cat /etc/smartdns-log-agent/config
```

### 调试模式

```bash
# 启用调试日志
export DEBUG=1
./smartdns-log-agent-linux-amd64
```

## 📋 系统要求

### 最低要求

- **操作系统**: Linux (x86_64, ARM64, ARMv7)
- **内存**: 64MB
- **磁盘**: 100MB
- **网络**: 能访问 ClickHouse 服务

### 支持平台

- ✅ Ubuntu 16.04+
- ✅ CentOS/RHEL 7+
- ✅ Debian 8+
- ✅ Alpine Linux
- ✅ Arch Linux

## 🔄 更新升级

### 自动更新

```bash
# systemd 方式
sudo ./install.sh --update

# Docker 方式  
cd /opt/smartdns-log-agent
./manage.sh update
```

### 手动更新

```bash
# 下载新版本
wget https://github.com/almightyyantao/smartdns-log-agent/releases/latest/download/smartdns-log-agent-linux-amd64.tar.gz

# 停止服务
sudo systemctl stop smartdns-log-agent

# 替换二进制文件
sudo cp smartdns-log-agent-linux-amd64 /usr/local/bin/smartdns-log-agent

# 启动服务
sudo systemctl start smartdns-log-agent
```

## 🗑️ 卸载

```bash
# 自动卸载
curl -sSL https://raw.githubusercontent.com/almightyyantao/smartdns-log-agent/main/install.sh | sudo bash -s -- --uninstall

# 或使用本地脚本
sudo ./install.sh --uninstall
```

## 📊 监控指标

Agent 运行时会输出以下关键指标：

```
📊 节点 1 已处理 5000 行日志
✅ 发送 1000 条日志到 ClickHouse, 耗时: 45ms
💾 成功插入 1000/1000 条日志到 ClickHouse (节点1), 耗时: 45ms, 速度: 22222 条/秒
```

## 📄 许可证

本项目采用 MIT 许可证 - 查看 [LICENSE](../LICENSE) 文件了解详情。

---

⭐ 如果这个项目对你有帮助，请给我们一个 Star！