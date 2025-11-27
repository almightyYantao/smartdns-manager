import React, { useState, useEffect } from "react";
import {
  Card,
  Table,
  Button,
  Space,
  Tag,
  Modal,
  Form,
  Input,
  Select,
  Switch,
  message,
  Popconfirm,
  Descriptions,
  Drawer,
  Statistic,
  Row,
  Col,
  Typography,
  Tooltip,
  Badge,
  Dropdown,
} from "antd";
import {
  PlusOutlined,
  EditOutlined,
  DeleteOutlined,
  PlayCircleOutlined,
  PauseCircleOutlined,
  EyeOutlined,
  HistoryOutlined,
  DatabaseOutlined,
  DeleteFilled,
  RadarChartOutlined,
  ClockCircleOutlined,
  ThunderboltOutlined,
  MoreOutlined,
  CodeOutlined,
} from "@ant-design/icons";
import {
  getTasks,
  createTask,
  updateTask,
  deleteTask,
  toggleTask,
  executeTask,
  getTaskExecutions,
  getSchedulerStats,
  getTaskTemplates,
  createQuickTask,
} from "../api/modules/scheduler";
import CronBuilder from "../components/CronBuilder/CronBuilder";

const { Title, Text } = Typography;
const { Option } = Select;
const { TextArea } = Input;

const Tasks = () => {
  const [tasks, setTasks] = useState([]);
  const [loading, setLoading] = useState(false);
  const [modalVisible, setModalVisible] = useState(false);
  const [detailVisible, setDetailVisible] = useState(false);
  const [executionsVisible, setExecutionsVisible] = useState(false);
  const [editingTask, setEditingTask] = useState(null);
  const [selectedTask, setSelectedTask] = useState(null);
  const [executions, setExecutions] = useState([]);
  const [stats, setStats] = useState({});
  const [templates, setTemplates] = useState([]);
  const [form] = Form.useForm();

  // 任务类型图标映射
  const typeIcons = {
    db_backup: <DatabaseOutlined style={{ color: "#1890ff" }} />,
    node_backup: <DatabaseOutlined style={{ color: "#52c41a" }} />,
    log_cleanup: <DeleteFilled style={{ color: "#fa541c" }} />,
    telemetry: <RadarChartOutlined style={{ color: "#722ed1" }} />,
    custom_script: <CodeOutlined style={{ color: "#13c2c2" }} />,
  };
  // 任务状态颜色映射
  const statusColors = {
    pending: "default",
    running: "processing",
    success: "success",
    failed: "error",
    skipped: "warning",
  };

  // 获取配置示例
  const getConfigExample = (taskType) => {
    const examples = {
      telemetry: `{
  "targets": [],
  "result_retention": 30,
  "alert_threshold": 3
}

遥测任务配置说明：
- targets: 遥测目标ID列表，空数组表示检测所有启用的目标
- result_retention: 结果保留天数，超过此天数的结果将被清理
- alert_threshold: 连续失败告警阈值

注意：需要先在遥测管理页面创建目标，然后在这里配置任务`,

      db_backup: `{
  "s3_config": {
    "access_key": "your_access_key",
    "secret_key": "your_secret_key",
    "region": "us-east-1",
    "bucket": "your_bucket_name",
    "endpoint": "",
    "prefix": "database-backups"
  },
  "compression": true,
  "encryption": false,
  "retention_days": 30
}

数据库备份配置说明：
- s3_config: S3存储配置
- compression: 是否压缩备份文件
- encryption: 是否加密备份文件
- retention_days: 备份保留天数`,

      node_backup: `{
  "storage_type": "local",
  "local_path": "/etc/smartdns/backups",
  "node_ids": [],
  "backup_configs": true,
  "backup_logs": false,
  "compression": true,
  "retention_days": 90
}

节点备份配置说明：
- storage_type: 存储类型，"local"(本地存储) 或 "s3"(云存储)
- local_path: 本地存储路径(当storage_type为local时使用)
- node_ids: 要备份的节点ID列表，空数组表示所有节点
- backup_configs: 是否备份配置文件
- backup_logs: 是否备份日志文件
- compression: 是否压缩备份文件
- retention_days: 备份保留天数

本地存储示例：
{
  "storage_type": "local",
  "local_path": "/etc/smartdns/backups",
  "node_ids": [],
  "backup_configs": true,
  "backup_logs": false,
  "compression": true,
  "retention_days": 30
}`,

      log_cleanup: `{
  "agent_log_days": 7,
  "backend_log_days": 30,
  "smartdns_log_days": 7,
  "log_paths": []
}

日志清理配置说明：
- agent_log_days: Agent日志保留天数
- backend_log_days: 后端日志保留天数
- smartdns_log_days: SmartDNS日志保留天数
- log_paths: 自定义日志路径列表`,

      custom_script: `{
  "node_ids": [],
  "script": "#!/bin/bash\\necho 'Hello World'\\ndate\\necho 'Script completed'",
  "timeout": 300,
  "working_dir": "/tmp",
  "env_vars": {
    "PATH": "/usr/local/bin:/usr/bin:/bin"
  },
  "run_as_user": "root"
}

自定义脚本配置说明：
- node_ids: 要执行脚本的节点ID列表，空数组表示所有节点
- script: 要执行的Shell脚本内容
- timeout: 脚本执行超时时间（秒），默认300秒
- working_dir: 脚本执行的工作目录
- env_vars: 环境变量设置
- run_as_user: 执行脚本的用户，默认为root

常用脚本示例：

系统信息收集：
{
  "node_ids": [],
  "script": "#!/bin/bash\\n# 系统信息收集\\necho '=== System Info ==='\\nuname -a\\necho '=== Disk Usage ==='\\ndf -h\\necho '=== Memory Usage ==='\\nfree -h\\necho '=== CPU Info ==='\\nlscpu | head -20",
  "timeout": 60,
  "working_dir": "/tmp"
}

SmartDNS重启：
{
  "node_ids": [],
  "script": "#!/bin/bash\\n# 重启SmartDNS服务\\nsystemctl restart smartdns\\nsystemctl status smartdns",
  "timeout": 30,
  "working_dir": "/tmp"
}

日志清理：
{
  "node_ids": [],
  "script": "#!/bin/bash\\n# 清理旧日志文件\\nfind /var/log -name '*.log' -mtime +7 -delete\\necho 'Log cleanup completed'",
  "timeout": 120,
  "working_dir": "/tmp"
}`,
    };

    return examples[taskType] || "{}";
  };

  useEffect(() => {
    fetchTasks();
    fetchStats();
    fetchTemplates();
  }, []);

  const fetchTasks = async () => {
    setLoading(true);
    try {
      const response = await getTasks();
      // 确保设置为数组，处理不同的响应格式
      const tasksData = response.data?.tasks || response.data || [];
      setTasks(Array.isArray(tasksData) ? tasksData : []);
    } catch (error) {
      message.error("获取任务列表失败");
      setTasks([]); // 确保在错误时设置为空数组
    } finally {
      setLoading(false);
    }
  };

  const fetchStats = async () => {
    try {
      const response = await getSchedulerStats();
      setStats(response.data || {});
    } catch (error) {
      console.error("获取统计信息失败:", error);
      setStats({}); // 确保在错误时设置为空对象
    }
  };

  const fetchTemplates = async () => {
    try {
      const response = await getTaskTemplates();
      const templatesData = response.data || [];
      setTemplates(Array.isArray(templatesData) ? templatesData : []);
    } catch (error) {
      console.error("获取任务模板失败:", error);
      setTemplates([]); // 确保在错误时设置为空数组
    }
  };

  const fetchExecutions = async (taskId) => {
    try {
      const response = await getTaskExecutions(taskId, {
        page: 1,
        pageSize: 20,
      });
      const executionsData = response.data?.executions || response.data || [];
      setExecutions(Array.isArray(executionsData) ? executionsData : []);
    } catch (error) {
      message.error("获取执行历史失败");
      setExecutions([]); // 确保在错误时设置为空数组
    }
  };

  const handleCreateTask = () => {
    setEditingTask(null);
    setModalVisible(true);
    form.resetFields();
  };

  const handleEditTask = (task) => {
    setEditingTask(task);
    setModalVisible(true);

    // 安全地处理config字段
    let configValue = "";
    try {
      const config = task.config || "{}";
      if (typeof config === "string") {
        // 如果config是字符串，先解析再格式化
        configValue = JSON.stringify(JSON.parse(config), null, 2);
      } else {
        // 如果config已经是对象，直接格式化
        configValue = JSON.stringify(config, null, 2);
      }
    } catch (e) {
      // 如果解析失败，使用原始值或空对象
      configValue = task.config || "{}";
    }

    form.setFieldsValue({
      ...task,
      config: configValue,
    });
  };

  const handleDeleteTask = async (id) => {
    try {
      await deleteTask(id);
      message.success("删除成功");
      fetchTasks();
    } catch (error) {
      message.error("删除失败");
    }
  };

  const handleToggleTask = async (id) => {
    try {
      await toggleTask(id);
      message.success("状态更新成功");
      fetchTasks();
      fetchStats();
    } catch (error) {
      message.error("状态更新失败");
    }
  };

  const handleExecuteTask = async (id) => {
    try {
      await executeTask(id);
      message.success("任务已开始执行");
      fetchTasks();
      fetchStats();
    } catch (error) {
      message.error(
        "执行任务失败: " + (error.response?.data?.message || error.message)
      );
    }
  };

  const handleSubmit = async (values) => {
    try {
      let config = {};
      if (values.config) {
        try {
          config = JSON.parse(values.config);
        } catch (e) {
          message.error("配置格式错误，请输入有效的JSON");
          return;
        }
      }

      const data = {
        ...values,
        config: JSON.stringify(config),
      };

      if (editingTask) {
        await updateTask(editingTask.id, data);
        message.success("更新成功");
      } else {
        await createTask(data);
        message.success("创建成功");
      }

      setModalVisible(false);
      fetchTasks();
      fetchStats();
    } catch (error) {
      message.error(editingTask ? "更新失败" : "创建失败");
    }
  };

  const handleQuickCreate = async (template) => {
    try {
      const data = {
        type: template.type,
        name: `${template.name}_${Date.now()}`,
        cron: template.defaultCron,
        config: template.configSchema,
      };

      await createQuickTask(data);
      message.success("快速创建成功");
      fetchTasks();
      fetchStats();
    } catch (error) {
      message.error("快速创建失败");
    }
  };

  const showTaskDetail = (task) => {
    setSelectedTask(task);
    setDetailVisible(true);
  };

  const showExecutions = (task) => {
    setSelectedTask(task);
    fetchExecutions(task.id);
    setExecutionsVisible(true);
  };

  const columns = [
    {
      title: "任务名称",
      dataIndex: "name",
      key: "name",
      render: (text, record) => (
        <Space>
          {typeIcons[record.type]}
          <span>{text}</span>
        </Space>
      ),
    },
    {
      title: "类型",
      dataIndex: "type",
      key: "type",
      render: (type) => {
        const template = templates.find((t) => t.type === type);
        return <Tag>{template?.name || type}</Tag>;
      },
    },
    {
      title: "Cron表达式",
      dataIndex: "cron_expr",
      key: "cron_expr",
      render: (text) => <code>{text}</code>,
    },
    {
      title: "状态",
      key: "status",
      render: (_, record) => (
        <Space>
          <Badge
            status={record.enabled ? "success" : "default"}
            text={record.enabled ? "启用" : "禁用"}
          />
          {record.last_status && (
            <Tag color={statusColors[record.last_status]}>
              {record.last_status}
            </Tag>
          )}
        </Space>
      ),
    },
    {
      title: "执行统计",
      key: "stats",
      render: (_, record) => (
        <Space direction="vertical" size="small">
          <Text type="secondary">
            成功: {record.success_count}/{record.run_count}
          </Text>
          {record.last_run_at && (
            <Text type="secondary">
              <ClockCircleOutlined />{" "}
              {new Date(record.last_run_at).toLocaleString()}
            </Text>
          )}
        </Space>
      ),
    },
    {
      title: "操作",
      key: "actions",
      render: (_, record) => {
        const moreMenuItems = [
          {
            key: "execute",
            label: "立即执行",
            icon: <ThunderboltOutlined />,
            onClick: () => handleExecuteTask(record.id),
          },
          {
            key: "detail",
            label: "查看详情",
            icon: <EyeOutlined />,
            onClick: () => showTaskDetail(record),
          },
          {
            key: "history",
            label: "执行历史",
            icon: <HistoryOutlined />,
            onClick: () => showExecutions(record),
          },
          {
            type: "divider",
          },
          {
            key: "delete",
            label: "删除任务",
            icon: <DeleteOutlined />,
            danger: true,
            onClick: () => {
              Modal.confirm({
                title: "确定删除这个任务吗？",
                content: "删除后无法恢复",
                okText: "确定",
                cancelText: "取消",
                okType: "danger",
                onOk: () => handleDeleteTask(record.id),
              });
            },
          },
        ];

        return (
          <Space>
            <Tooltip title={record.enabled ? "暂停" : "启用"}>
              <Button
                type="text"
                icon={
                  record.enabled ? (
                    <PauseCircleOutlined />
                  ) : (
                    <PlayCircleOutlined />
                  )
                }
                onClick={() => handleToggleTask(record.id)}
              />
            </Tooltip>
            <Button
              type="text"
              icon={<EditOutlined />}
              onClick={() => handleEditTask(record)}
            />
            <Dropdown
              menu={{
                items: moreMenuItems,
                onClick: ({ key, domEvent }) => {
                  domEvent.stopPropagation();
                  const item = moreMenuItems.find((item) => item.key === key);
                  if (item?.onClick) {
                    item.onClick();
                  }
                },
              }}
              trigger={["click"]}
              placement="bottomRight"
            >
              <Button
                type="text"
                icon={<MoreOutlined />}
                onClick={(e) => e.stopPropagation()}
              />
            </Dropdown>
          </Space>
        );
      },
    },
  ];

  const executionColumns = [
    {
      title: "开始时间",
      dataIndex: "started_at",
      key: "started_at",
      render: (time) => new Date(time).toLocaleString(),
    },
    {
      title: "状态",
      dataIndex: "status",
      key: "status",
      render: (status) => <Tag color={statusColors[status]}>{status}</Tag>,
    },
    {
      title: "耗时",
      dataIndex: "duration",
      key: "duration",
      render: (duration) => (duration ? `${duration}ms` : "-"),
    },
    {
      title: "输出",
      dataIndex: "output",
      key: "output",
      ellipsis: true,
      width: 300,
    },
    {
      title: "错误信息",
      dataIndex: "error",
      key: "error",
      ellipsis: true,
      width: 200,
      render: (error) => error && <Text type="danger">{error}</Text>,
    },
  ];

  return (
    <div>
      {/* 统计卡片 */}
      <Row gutter={16} style={{ marginBottom: 16 }}>
        <Col span={6}>
          <Card>
            <Statistic
              title="总任务数"
              value={stats.total_tasks || 0}
              prefix={<ClockCircleOutlined />}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic
              title="启用任务"
              value={stats.enabled_tasks || 0}
              prefix={<PlayCircleOutlined />}
              valueStyle={{ color: "#3f8600" }}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic
              title="正在运行"
              value={stats.running_tasks || 0}
              prefix={<RadarChartOutlined />}
              valueStyle={{ color: "#1890ff" }}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic
              title="成功率"
              value={stats.success_rate || 0}
              suffix="%"
              precision={1}
              valueStyle={{
                color: stats.success_rate > 90 ? "#3f8600" : "#cf1322",
              }}
            />
          </Card>
        </Col>
      </Row>

      {/* 快速创建按钮 */}
      <Card style={{ marginBottom: 16 }}>
        <Title level={5}>快速创建任务</Title>
        <Space wrap>
          {templates.map((template) => (
            <Button
              key={template.type}
              onClick={() => handleQuickCreate(template)}
              icon={typeIcons[template.type]}
            >
              {template.name}
            </Button>
          ))}
        </Space>
      </Card>

      {/* 任务列表 */}
      <Card
        title="定时任务"
        extra={
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={handleCreateTask}
          >
            创建任务
          </Button>
        }
      >
        <Table
          columns={columns}
          dataSource={tasks}
          loading={loading}
          rowKey="id"
          pagination={{ pageSize: 10 }}
        />
      </Card>

      {/* 创建/编辑任务对话框 */}
      <Modal
        title={editingTask ? "编辑任务" : "创建任务"}
        open={modalVisible}
        onCancel={() => setModalVisible(false)}
        onOk={() => form.submit()}
        width={800}
      >
        <Form form={form} onFinish={handleSubmit} layout="vertical">
          <Form.Item
            name="name"
            label="任务名称"
            rules={[{ required: true, message: "请输入任务名称" }]}
          >
            <Input placeholder="请输入任务名称" />
          </Form.Item>

          <Form.Item
            name="type"
            label="任务类型"
            rules={[{ required: true, message: "请选择任务类型" }]}
          >
            <Select placeholder="请选择任务类型">
              {templates.map((template) => (
                <Option key={template.type} value={template.type}>
                  <Space>
                    {typeIcons[template.type]}
                    {template.name}
                  </Space>
                </Option>
              ))}
            </Select>
          </Form.Item>

          <Form.Item
            name="cron_expr"
            label="执行时间设置"
            rules={[{ required: true, message: "请设置执行时间" }]}
          >
            <CronBuilder />
          </Form.Item>

          <Form.Item name="description" label="描述">
            <TextArea rows={2} placeholder="任务描述" />
          </Form.Item>

          <Form.Item name="config" label="配置 (JSON格式)">
            <TextArea rows={8} placeholder='{"key": "value"}' />
          </Form.Item>

          {/* 根据任务类型显示配置示例 */}
          {form.getFieldValue("type") && (
            <div style={{ marginTop: 8 }}>
              <Text type="secondary" style={{ fontSize: "12px" }}>
                配置示例：
              </Text>
              <pre
                style={{
                  fontSize: "11px",
                  backgroundColor: "#f5f5f5",
                  padding: "8px",
                  borderRadius: "4px",
                  marginTop: "4px",
                  maxHeight: "200px",
                  overflow: "auto",
                }}
              >
                {getConfigExample(form.getFieldValue("type"))}
              </pre>
            </div>
          )}
          <Form.Item name="enabled" valuePropName="checked" label="启用">
            <Switch />
          </Form.Item>
        </Form>
      </Modal>

      {/* 任务详情抽屉 */}
      <Drawer
        title="任务详情"
        width={600}
        open={detailVisible}
        onClose={() => setDetailVisible(false)}
      >
        {selectedTask && (
          <Descriptions column={1} bordered>
            <Descriptions.Item label="任务名称">
              {selectedTask.name}
            </Descriptions.Item>
            <Descriptions.Item label="任务类型">
              <Space>
                {typeIcons[selectedTask.type]}
                {templates.find((t) => t.type === selectedTask.type)?.name ||
                  selectedTask.type}
              </Space>
            </Descriptions.Item>
            <Descriptions.Item label="Cron表达式">
              <code>{selectedTask.cron_expr}</code>
            </Descriptions.Item>
            <Descriptions.Item label="描述">
              {selectedTask.description || "-"}
            </Descriptions.Item>
            <Descriptions.Item label="状态">
              <Badge
                status={selectedTask.enabled ? "success" : "default"}
                text={selectedTask.enabled ? "启用" : "禁用"}
              />
            </Descriptions.Item>
            <Descriptions.Item label="执行统计">
              总计: {selectedTask.run_count} | 成功:{" "}
              {selectedTask.success_count}
            </Descriptions.Item>
            <Descriptions.Item label="最后执行">
              {selectedTask.last_run_at
                ? new Date(selectedTask.last_run_at).toLocaleString()
                : "未执行"}
            </Descriptions.Item>
            <Descriptions.Item label="最后状态">
              {selectedTask.last_status ? (
                <Tag color={statusColors[selectedTask.last_status]}>
                  {selectedTask.last_status}
                </Tag>
              ) : (
                "-"
              )}
            </Descriptions.Item>
            <Descriptions.Item label="配置">
              <pre
                style={{
                  fontSize: "12px",
                  maxHeight: "200px",
                  overflow: "auto",
                  backgroundColor: "#f5f5f5",
                  padding: "8px",
                  borderRadius: "4px",
                }}
              >
                {(() => {
                  try {
                    const config = selectedTask.config || "{}";
                    if (typeof config === "string") {
                      return JSON.stringify(JSON.parse(config), null, 2);
                    }
                    return JSON.stringify(config, null, 2);
                  } catch (e) {
                    return selectedTask.config || "配置解析失败";
                  }
                })()}
              </pre>
            </Descriptions.Item>
            {selectedTask.last_error && (
              <Descriptions.Item label="最后错误">
                <Text type="danger">{selectedTask.last_error}</Text>
              </Descriptions.Item>
            )}
          </Descriptions>
        )}
      </Drawer>

      {/* 执行历史抽屉 */}
      <Drawer
        title="执行历史"
        width={800}
        open={executionsVisible}
        onClose={() => setExecutionsVisible(false)}
      >
        <Table
          columns={executionColumns}
          dataSource={executions}
          rowKey="id"
          pagination={{ pageSize: 10 }}
          size="small"
        />
      </Drawer>
    </div>
  );
};

export default Tasks;
