import request from "../../utils/request";

// 任务管理
export const getTasks = (params) => {
  return request({
    url: '/scheduler/tasks',
    method: 'GET',
    params
  });
};

export const createTask = (data) => {
  return request({
    url: '/scheduler/tasks',
    method: 'POST',
    data
  });
};

export const getTask = (id) => {
  return request({
    url: `/scheduler/tasks/${id}`,
    method: 'GET'
  });
};

export const updateTask = (id, data) => {
  return request({
    url: `/scheduler/tasks/${id}`,
    method: 'PUT',
    data
  });
};

export const deleteTask = (id) => {
  return request({
    url: `/scheduler/tasks/${id}`,
    method: 'DELETE'
  });
};

export const toggleTask = (id) => {
  return request({
    url: `/scheduler/tasks/${id}/toggle`,
    method: 'POST'
  });
};

export const executeTask = (id) => {
  return request({
    url: `/scheduler/tasks/${id}/execute`,
    method: 'POST'
  });
};

// 任务执行历史
export const getTaskExecutions = (id, params) => {
  return request({
    url: `/scheduler/tasks/${id}/executions`,
    method: 'GET',
    params
  });
};

export const getRunningTasks = () => {
  return request({
    url: '/scheduler/running',
    method: 'GET'
  });
};

export const getSchedulerStats = () => {
  return request({
    url: '/scheduler/stats',
    method: 'GET'
  });
};

// 快速任务创建
export const createQuickTask = (data) => {
  return request({
    url: '/scheduler/quick-task',
    method: 'POST',
    data
  });
};

// 遥测目标管理
export const getTelemetryTargets = () => {
  return request({
    url: '/scheduler/telemetry/targets',
    method: 'GET'
  });
};

export const createTelemetryTarget = (data) => {
  return request({
    url: '/scheduler/telemetry/targets',
    method: 'POST',
    data
  });
};

export const updateTelemetryTarget = (id, data) => {
  return request({
    url: `/scheduler/telemetry/targets/${id}`,
    method: 'PUT',
    data
  });
};

export const deleteTelemetryTarget = (id) => {
  return request({
    url: `/scheduler/telemetry/targets/${id}`,
    method: 'DELETE'
  });
};

// 遥测结果和统计
export const getTelemetryResults = (params) => {
  return request({
    url: '/scheduler/telemetry/results',
    method: 'GET',
    params
  });
};

export const getTelemetryStats = () => {
  return request({
    url: '/scheduler/telemetry/stats',
    method: 'GET'
  });
};

// 测试遥测目标
export const testTelemetryTarget = (id) => {
  return request({
    url: `/scheduler/telemetry/targets/${id}/test`,
    method: 'POST'
  });
};

// 脚本模板管理
export const getScriptTemplates = () => {
  return request({
    url: '/scheduler/script-templates',
    method: 'GET'
  });
};

// 任务类型和配置模板
export const getTaskTemplates = () => {
  return Promise.resolve({
    data: [
      {
        type: 'db_backup',
        name: '数据库备份',
        description: '自动备份SQLite数据库到S3',
        icon: 'database',
        defaultCron: '0 2 * * *', // 每天凌晨2点
        configSchema: {
          s3_config: {
            access_key: '',
            secret_key: '',
            region: 'us-east-1',
            bucket: '',
            endpoint: '',
            prefix: 'database-backups'
          },
          compression: true,
          encryption: false,
          retention_days: 30
        }
      },
      {
        type: 'node_backup',
        name: '节点备份',
        description: '备份SmartDNS节点配置文件',
        icon: 'server',
        defaultCron: '0 3 * * 0', // 每周日凌晨3点
        configSchema: {
          storage_type: 'local',
          local_path: '/etc/smartdns/backups',
          s3_config: {
            access_key: '',
            secret_key: '',
            region: 'us-east-1',
            bucket: '',
            endpoint: '',
            prefix: 'node-backups'
          },
          node_ids: [],
          backup_configs: true,
          backup_logs: false,
          compression: true,
          retention_days: 90
        }
      },
      {
        type: 'log_cleanup',
        name: '日志清理',
        description: '定时清理过期日志文件',
        icon: 'delete',
        defaultCron: '0 4 * * *', // 每天凌晨4点
        configSchema: {
          agent_log_days: 7,
          backend_log_days: 30,
          smartdns_log_days: 7,
          log_paths: []
        }
      },
      {
        type: 'telemetry',
        name: '网络遥测',
        description: '定期检测网络连通性和延迟',
        icon: 'radar-chart',
        defaultCron: '*/5 * * * *', // 每5分钟
        configSchema: {
          targets: [],
          result_retention: 30,
          alert_threshold: 3
        },
        examples: [
          {
            name: '基础网络监控',
            description: '监控常用网站和内网服务器',
            config: {
              targets: [],
              result_retention: 30,
              alert_threshold: 3
            },
            targets_examples: [
              { name: 'Google DNS', type: 'ping', target: '8.8.8.8', timeout: 5000 },
              { name: 'Cloudflare DNS', type: 'ping', target: '1.1.1.1', timeout: 5000 },
              { name: '百度首页', type: 'http', target: 'baidu.com', timeout: 10000 },
              { name: 'GitHub', type: 'https', target: 'github.com', timeout: 15000 },
              { name: '内网服务器', type: 'tcp', target: '10.1.1.1:80', timeout: 8000 }
            ]
          },
          {
            name: '内网设备监控',
            description: '专门监控内网设备和服务',
            config: {
              targets: [],
              result_retention: 90,
              alert_threshold: 5
            },
            targets_examples: [
              { name: '路由器管理', type: 'http', target: '192.168.1.1', timeout: 10000 },
              { name: 'NAS存储', type: 'tcp', target: '192.168.1.100:22', timeout: 8000 },
              { name: '打印服务器', type: 'tcp', target: '192.168.1.200:9100', timeout: 5000 }
            ]
          }
        ]
      },
      {
        type: 'custom_script',
        name: '自定义脚本',
        description: '在选定节点上执行自定义Shell脚本',
        icon: 'terminal',
        defaultCron: '0 1 * * *', // 每天凌晨1点
        configSchema: {
          script: '#!/bin/bash\n# 在此处编写您的脚本\necho "Hello, SmartDNS!"\n',
          node_ids: [],
          timeout: 300,
          run_as_user: 'root',
          working_dir: '/tmp',
          env_vars: {}
        },
        examples: [
          {
            name: '系统信息收集',
            description: '收集系统基本信息和状态',
            config: {
              script: '#!/bin/bash\necho "=== 系统信息 ==="\nuname -a\necho ""\necho "=== 内存使用 ==="\nfree -h\necho ""\necho "=== 磁盘使用 ==="\ndf -h\necho ""\necho "=== 系统负载 ==="\nuptime',
              node_ids: [],
              timeout: 60,
              run_as_user: 'root',
              working_dir: '/tmp',
              env_vars: {}
            }
          },
          {
            name: 'SmartDNS服务重启',
            description: '重启SmartDNS服务并检查状态',
            config: {
              script: '#!/bin/bash\necho "重启SmartDNS服务..."\nsystemctl restart smartdns\necho "等待服务启动..."\nsleep 3\necho "检查服务状态:"\nsystemctl status smartdns --no-pager',
              node_ids: [],
              timeout: 120,
              run_as_user: 'root',
              working_dir: '/tmp',
              env_vars: {}
            }
          },
          {
            name: '日志清理脚本',
            description: '清理系统和应用日志',
            config: {
              script: '#!/bin/bash\necho "清理系统日志..."\nfind /var/log -name "*.log" -mtime +7 -exec rm -f {} \\;\necho "清理SmartDNS日志..."\nif [ -d "/var/log/smartdns" ]; then\n  find /var/log/smartdns -name "*.log" -mtime +7 -exec rm -f {} \\;\nfi\necho "日志清理完成"',
              node_ids: [],
              timeout: 300,
              run_as_user: 'root',
              working_dir: '/tmp',
              env_vars: {}
            }
          }
        ]
      }
    ]
  });
};