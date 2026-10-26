# SmartDNS Log Agent

## 静态映射匹配标记（可选）

此功能属于 Manager 的日志采集器，不修改 SmartDNS 程序、DNS 配置、原始审计日志或 ClickHouse 配置/表结构。
默认关闭；启用后，仅将指定时间之后、A/AAAA 返回 IP 与本节点 `address` 规则一致的新记录的结构化 `group` 写为 `__static_match__`。
`raw_log` 保留 SmartDNS 原始内容和分组，已入库的历史记录不更新、不回填。页面将该保留标识显示为“静态映射匹配”，并支持筛选。

这是采集时的配置匹配推断，不能证明一次查询没有上游流量，也不用于判断缓存命中。
HTTPS 等其他查询类型保留原分组；配置缺失、读取失败、晚于日志时间，或存在 `conf-file`、规则组、跳过地址规则等尚未支持的作用范围时，不进行标记。
目前仅识别普通域名及其子域的直接地址规则；域名集、通配符、CNAME 和 SOA 等复杂规则不推断。
配置最多每秒读取一次，无需重启 SmartDNS。`__static_match__` 是日志保留值，请勿用作真实 DNS 上游分组名。

在 Agent 的环境文件中配置（启用时间应固定保存，后续重启继续沿用）：

```ini
STATIC_ADDRESS_TAGGING=true
SMARTDNS_CONFIG_FILE=/etc/smartdns/smartdns.conf
STATIC_ADDRESS_TAG_FROM=2026-10-26T12:00:00+08:00
```

`STATIC_ADDRESS_TAG_FROM` 使用 RFC3339；未指定时以 Agent 进程启动时间为界。示例时间需改为实际启用时间。
升级前备份 Agent 二进制、环境配置和 `/var/lib/smartdns-agent/position-node-*.json`，仅重启 Agent 服务，并恢复原来的采集启停状态。
不要重置读取位置或重新导入旧日志。回退时恢复旧二进制和配置，或将 `STATIC_ADDRESS_TAGGING=false` 后重启 Agent；已产生的标记无需清理。
Docker 部署须把节点配置目录只读挂载到 Agent，并传入上述环境变量。

依赖 Go 1.24 或以上。以下本地校验不连接 DNS、ClickHouse 或生产节点，也不写入业务数据：

```bash
go test ./...
go vet ./...
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /tmp/smartdns-log-agent-static-match .
```

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
  --log-file "/var/log/smartdns/audit.log" \
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
export LOG_FILE="/var/log/smartdns/audit.log"
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
| `LOG_FILE` | `/var/log/smartdns/audit.log` | SmartDNS 日志文件路径 |
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
ls -la /var/log/smartdns/audit.log

# 如果需要，调整权限
sudo chmod 644 /var/log/smartdns/audit.log
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
