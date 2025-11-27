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
  InputNumber,
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
  Divider,
  Alert,
  Tabs,
} from "antd";
import {
  PlusOutlined,
  EditOutlined,
  DeleteOutlined,
  PlayCircleOutlined,
  EyeOutlined,
  HistoryOutlined,
  RadarChartOutlined,
  ClockCircleOutlined,
  CheckCircleOutlined,
  ExclamationCircleOutlined,
  ThunderboltOutlined,
} from "@ant-design/icons";
import {
  getTelemetryTargets,
  createTelemetryTarget,
  updateTelemetryTarget,
  deleteTelemetryTarget,
  getTelemetryResults,
  getTelemetryStats,
  testTelemetryTarget,
} from "../api/modules/scheduler";

const { Title, Text } = Typography;
const { Option } = Select;
const { TextArea } = Input;
const { TabPane } = Tabs;

const Telemetry = () => {
  const [targets, setTargets] = useState([]);
  const [results, setResults] = useState([]);
  const [stats, setStats] = useState({});
  const [loading, setLoading] = useState(false);
  const [modalVisible, setModalVisible] = useState(false);
  const [detailVisible, setDetailVisible] = useState(false);
  const [resultsVisible, setResultsVisible] = useState(false);
  const [editingTarget, setEditingTarget] = useState(null);
  const [selectedTarget, setSelectedTarget] = useState(null);
  const [testingTargets, setTestingTargets] = useState(new Set());
  const [form] = Form.useForm();

  // 检测类型配置
  const typeOptions = [
    {
      value: "ping",
      label: "PING检测",
      description: "基于网络连通性的检测，不使用端口",
    },
    {
      value: "http",
      label: "HTTP检测",
      description: "HTTP协议检测，默认端口80",
    },
    {
      value: "https",
      label: "HTTPS检测",
      description: "HTTPS协议检测，默认端口443",
    },
    {
      value: "tcp",
      label: "TCP检测",
      description: "TCP端口连接检测，需要指定端口",
    },
  ];

  // 预设配置模板
  const presetTemplates = [
    {
      name: "Google DNS",
      type: "ping",
      target: "8.8.8.8",
      timeout: 5000,
      description: "Google公共DNS服务器",
    },
    {
      name: "Cloudflare DNS",
      type: "ping",
      target: "1.1.1.1",
      timeout: 5000,
      description: "Cloudflare公共DNS服务器",
    },
    {
      name: "百度首页",
      type: "http",
      target: "baidu.com",
      timeout: 10000,
      description: "百度网站HTTP检测",
    },
    {
      name: "GitHub",
      type: "https",
      target: "github.com",
      timeout: 15000,
      description: "GitHub网站HTTPS检测",
    },
    {
      name: "本地路由器",
      type: "http",
      target: "192.168.1.1",
      timeout: 8000,
      description: "本地网关设备检测",
    },
  ];

  useEffect(() => {
    fetchTargets();
    fetchStats();
    fetchResults();
  }, []);

  const fetchTargets = async () => {
    setLoading(true);
    try {
      const response = await getTelemetryTargets();
      setTargets(response.data || []);
    } catch (error) {
      message.error("获取遥测目标失败");
      setTargets([]);
    } finally {
      setLoading(false);
    }
  };

  const fetchStats = async () => {
    try {
      const response = await getTelemetryStats();
      setStats(response.data || {});
    } catch (error) {
      console.error("获取统计信息失败:", error);
      setStats({});
    }
  };

  const fetchResults = async (targetId = null) => {
    try {
      const params = targetId
        ? { target_id: targetId }
        : { page: 1, pageSize: 50 };
      const response = await getTelemetryResults(params);
      const resultsData = response.data?.results || response.data || [];
      setResults(Array.isArray(resultsData) ? resultsData : []);
    } catch (error) {
      console.error("获取遥测结果失败:", error);
      setResults([]);
    }
  };

  const handleCreateTarget = () => {
    setEditingTarget(null);
    setModalVisible(true);
    form.resetFields();
    // 设置默认值
    form.setFieldsValue({
      enabled: true,
      timeout: 10000,
    });
  };

  const handleEditTarget = (target) => {
    setEditingTarget(target);
    setModalVisible(true);
    form.setFieldsValue(target);
  };

  const handleDeleteTarget = async (id) => {
    try {
      await deleteTelemetryTarget(id);
      message.success("删除成功");
      fetchTargets();
      fetchStats();
    } catch (error) {
      message.error("删除失败");
    }
  };

  const handleTestTarget = async (id) => {
    if (testingTargets.has(id)) return;

    const newTestingTargets = new Set(testingTargets);
    newTestingTargets.add(id);
    setTestingTargets(newTestingTargets);

    try {
      const response = await testTelemetryTarget(id);
      const result = response.data;

      if (result.success) {
        message.success(`测试成功！延迟: ${result.latency}ms`);
      } else {
        message.error(`测试失败: ${result.error || "未知错误"}`);
      }

      // 刷新数据
      fetchTargets();
      fetchResults();
      fetchStats();
    } catch (error) {
      message.error(
        "测试失败: " + (error.response?.data?.message || error.message)
      );
    } finally {
      setTimeout(() => {
        const newTestingTargets = new Set(testingTargets);
        newTestingTargets.delete(id);
        setTestingTargets(newTestingTargets);
      }, 1000);
    }
  };

  const handleSubmit = async (values) => {
    try {
      // 验证配置
      if (values.type === "ping" && values.target.includes(":")) {
        message.warning("PING检测不需要端口号，已自动移除");
        values.target = values.target.split(":")[0];
      }

      if (values.type === "tcp" && !values.target.includes(":")) {
        message.error("TCP检测需要指定端口号，格式：IP:端口");
        return;
      }

      // 确保超时时间在合理范围内
      if (values.timeout < 1000) {
        values.timeout = 1000;
        message.warning("超时时间不能少于1秒，已自动调整为1秒");
      }
      if (values.timeout > 60000) {
        values.timeout = 60000;
        message.warning("超时时间不能超过60秒，已自动调整为60秒");
      }

      if (editingTarget) {
        await updateTelemetryTarget(editingTarget.id, values);
        message.success("更新成功");
      } else {
        await createTelemetryTarget(values);
        message.success("创建成功");
      }

      setModalVisible(false);
      fetchTargets();
      fetchStats();
    } catch (error) {
      message.error(editingTarget ? "更新失败" : "创建失败");
    }
  };

  const handleUseTemplate = (template) => {
    form.setFieldsValue({
      ...template,
      enabled: true,
    });
  };

  const showTargetDetail = (target) => {
    setSelectedTarget(target);
    setDetailVisible(true);
  };

  const showResults = (target) => {
    setSelectedTarget(target);
    fetchResults(target.id);
    setResultsVisible(true);
  };

  const getTypeColor = (type) => {
    const colors = {
      ping: "#52c41a",
      http: "#1890ff",
      https: "#722ed1",
      tcp: "#fa8c16",
    };
    return colors[type] || "#666";
  };

  const columns = [
    {
      title: "目标名称",
      dataIndex: "name",
      key: "name",
      render: (text, record) => (
        <Space>
          <RadarChartOutlined style={{ color: getTypeColor(record.type) }} />
          <span>{text}</span>
        </Space>
      ),
    },
    {
      title: "检测类型",
      dataIndex: "type",
      key: "type",
      render: (type) => (
        <Tag color={getTypeColor(type)}>{type.toUpperCase()}</Tag>
      ),
    },
    {
      title: "目标地址",
      dataIndex: "target",
      key: "target",
      render: (text) => <code>{text}</code>,
    },
    {
      title: "超时设置",
      dataIndex: "timeout",
      key: "timeout",
      render: (timeout) => (
        <Space>
          <ClockCircleOutlined />
          <span>{timeout}ms</span>
        </Space>
      ),
    },
    {
      title: "状态",
      key: "status",
      render: (_, record) => (
        <Space direction="vertical" size="small">
          <Badge
            status={record.enabled ? "success" : "default"}
            text={record.enabled ? "启用" : "禁用"}
          />
          {record.last_check_at && (
            <Badge
              status={record.last_status ? "success" : "error"}
              text={record.last_status ? "正常" : "异常"}
            />
          )}
        </Space>
      ),
    },
    {
      title: "统计信息",
      key: "stats",
      render: (_, record) => (
        <Space direction="vertical" size="small">
          {record.last_latency !== undefined && (
            <Text type="secondary">延迟: {record.last_latency}ms</Text>
          )}
          {record.check_count > 0 && (
            <Text type="secondary">
              成功率:{" "}
              {((record.success_count / record.check_count) * 100).toFixed(1)}%
            </Text>
          )}
          {record.last_check_at && (
            <Text type="secondary">
              <ClockCircleOutlined />{" "}
              {new Date(record.last_check_at).toLocaleString()}
            </Text>
          )}
        </Space>
      ),
    },
    {
      title: "操作",
      key: "actions",
      render: (_, record) => (
        <Space>
          <Tooltip title="手动测试">
            <Button
              type="text"
              icon={<ThunderboltOutlined />}
              onClick={() => handleTestTarget(record.id)}
              loading={testingTargets.has(record.id)}
            />
          </Tooltip>
          <Tooltip title="查看详情">
            <Button
              type="text"
              icon={<EyeOutlined />}
              onClick={() => showTargetDetail(record)}
            />
          </Tooltip>
          <Tooltip title="检测历史">
            <Button
              type="text"
              icon={<HistoryOutlined />}
              onClick={() => showResults(record)}
            />
          </Tooltip>
          <Button
            type="text"
            icon={<EditOutlined />}
            onClick={() => handleEditTarget(record)}
          />
          <Popconfirm
            title="确定删除这个遥测目标吗？"
            onConfirm={() => handleDeleteTarget(record.id)}
          >
            <Button type="text" danger icon={<DeleteOutlined />} />
          </Popconfirm>
        </Space>
      ),
    },
  ];

  const resultColumns = [
    {
      title: "检测时间",
      dataIndex: "checked_at",
      key: "checked_at",
      render: (time) => new Date(time).toLocaleString(),
    },
    {
      title: "状态",
      dataIndex: "success",
      key: "success",
      render: (success) => (
        <Badge
          status={success ? "success" : "error"}
          text={success ? "成功" : "失败"}
        />
      ),
    },
    {
      title: "延迟",
      dataIndex: "latency",
      key: "latency",
      render: (latency) => (latency ? `${latency}ms` : "-"),
    },
    {
      title: "响应",
      dataIndex: "response",
      key: "response",
      ellipsis: true,
      width: 150,
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
              title="总目标数"
              value={stats.total_targets || 0}
              prefix={<RadarChartOutlined />}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic
              title="在线目标"
              value={stats.online_targets || 0}
              prefix={<CheckCircleOutlined />}
              valueStyle={{ color: "#3f8600" }}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic
              title="平均延迟"
              value={stats.avg_latency || 0}
              suffix="ms"
              precision={1}
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
                color: stats.success_rate > 95 ? "#3f8600" : "#cf1322",
              }}
            />
          </Card>
        </Col>
      </Row>

      {/* 遥测目标列表 */}
      <Card
        title="遥测目标管理"
        extra={
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={handleCreateTarget}
          >
            添加目标
          </Button>
        }
      >
        <Table
          columns={columns}
          dataSource={targets}
          loading={loading}
          rowKey="id"
          pagination={{ pageSize: 10 }}
        />
      </Card>

      {/* 创建/编辑目标对话框 */}
      <Modal
        title={editingTarget ? "编辑遥测目标" : "创建遥测目标"}
        open={modalVisible}
        onCancel={() => setModalVisible(false)}
        onOk={() => form.submit()}
        width={800}
      >
        <Form form={form} onFinish={handleSubmit} layout="vertical">
          <Alert
            message="配置说明"
            description={
              <div>
                <p>
                  <strong>PING检测:</strong> 不需要端口号，只填写IP地址或域名
                </p>
                <p>
                  <strong>HTTP/HTTPS检测:</strong>{" "}
                  可以是域名或IP，会自动添加协议前缀
                </p>
                <p>
                  <strong>TCP检测:</strong>{" "}
                  必须指定端口，格式：IP:端口或域名:端口
                </p>
              </div>
            }
            type="info"
            style={{ marginBottom: 16 }}
          />

          <Tabs defaultActiveKey="basic">
            <TabPane tab="基本配置" key="basic">
              <Form.Item
                name="name"
                label="目标名称"
                rules={[{ required: true, message: "请输入目标名称" }]}
              >
                <Input placeholder="请输入目标名称" />
              </Form.Item>

              <Form.Item
                name="type"
                label="检测类型"
                rules={[{ required: true, message: "请选择检测类型" }]}
              >
                <Select placeholder="请选择检测类型">
                  {typeOptions.map((option) => (
                    <Option key={option.value} value={option.value}>
                      <span>{option.label}</span>
                      <Text
                        type="secondary"
                        style={{ fontSize: "12px", marginLeft: 8 }}
                      >
                        ---({option.description})
                      </Text>
                    </Option>
                  ))}
                </Select>
              </Form.Item>

              <Form.Item
                name="target"
                label="目标地址"
                rules={[{ required: true, message: "请输入目标地址" }]}
                extra="根据检测类型填写相应格式的地址"
              >
                <Input placeholder="例如: 8.8.8.8 或 baidu.com 或 192.168.1.1:80" />
              </Form.Item>

              <Form.Item
                name="timeout"
                label="超时时间 (毫秒)"
                rules={[{ required: true, message: "请输入超时时间" }]}
                extra="建议范围：1000-60000毫秒"
              >
                <InputNumber
                  min={1000}
                  max={60000}
                  step={1000}
                  style={{ width: "100%" }}
                  placeholder="10000"
                />
              </Form.Item>

              <Form.Item name="description" label="描述">
                <TextArea rows={2} placeholder="目标描述" />
              </Form.Item>

              <Form.Item name="enabled" valuePropName="checked" label="启用">
                <Switch />
              </Form.Item>
            </TabPane>

            <TabPane tab="快速模板" key="templates">
              <div style={{ marginBottom: 16 }}>
                <Title level={5}>选择预设模板</Title>
                <Row gutter={[16, 16]}>
                  {presetTemplates.map((template, index) => (
                    <Col span={12} key={index}>
                      <Card
                        size="small"
                        title={template.name}
                        extra={
                          <Button
                            size="small"
                            type="link"
                            onClick={() => handleUseTemplate(template)}
                          >
                            使用
                          </Button>
                        }
                      >
                        <Space
                          direction="vertical"
                          size="small"
                          style={{ width: "100%" }}
                        >
                          <div>
                            <Tag color={getTypeColor(template.type)}>
                              {template.type.toUpperCase()}
                            </Tag>
                            <code>{template.target}</code>
                          </div>
                          <Text type="secondary" style={{ fontSize: "12px" }}>
                            {template.description}
                          </Text>
                        </Space>
                      </Card>
                    </Col>
                  ))}
                </Row>
              </div>
            </TabPane>
          </Tabs>
        </Form>
      </Modal>

      {/* 目标详情抽屉 */}
      <Drawer
        title="遥测目标详情"
        width={600}
        open={detailVisible}
        onClose={() => setDetailVisible(false)}
      >
        {selectedTarget && (
          <Descriptions column={1} bordered>
            <Descriptions.Item label="目标名称">
              {selectedTarget.name}
            </Descriptions.Item>
            <Descriptions.Item label="检测类型">
              <Tag color={getTypeColor(selectedTarget.type)}>
                {selectedTarget.type.toUpperCase()}
              </Tag>
            </Descriptions.Item>
            <Descriptions.Item label="目标地址">
              <code>{selectedTarget.target}</code>
            </Descriptions.Item>
            <Descriptions.Item label="超时设置">
              {selectedTarget.timeout}ms
            </Descriptions.Item>
            <Descriptions.Item label="描述">
              {selectedTarget.description || "-"}
            </Descriptions.Item>
            <Descriptions.Item label="状态">
              <Badge
                status={selectedTarget.enabled ? "success" : "default"}
                text={selectedTarget.enabled ? "启用" : "禁用"}
              />
            </Descriptions.Item>
            <Descriptions.Item label="检测统计">
              总计: {selectedTarget.check_count} | 成功:{" "}
              {selectedTarget.success_count}
            </Descriptions.Item>
            <Descriptions.Item label="最后检测">
              {selectedTarget.last_check_at
                ? new Date(selectedTarget.last_check_at).toLocaleString()
                : "未检测"}
            </Descriptions.Item>
            <Descriptions.Item label="最后状态">
              {selectedTarget.last_status !== undefined ? (
                <Badge
                  status={selectedTarget.last_status ? "success" : "error"}
                  text={selectedTarget.last_status ? "正常" : "异常"}
                />
              ) : (
                "-"
              )}
            </Descriptions.Item>
            {selectedTarget.last_latency !== undefined && (
              <Descriptions.Item label="最后延迟">
                {selectedTarget.last_latency}ms
              </Descriptions.Item>
            )}
            {selectedTarget.avg_latency && (
              <Descriptions.Item label="平均延迟">
                {selectedTarget.avg_latency.toFixed(1)}ms
              </Descriptions.Item>
            )}
          </Descriptions>
        )}
      </Drawer>

      {/* 检测历史抽屉 */}
      <Drawer
        title="检测历史"
        width={900}
        open={resultsVisible}
        onClose={() => setResultsVisible(false)}
      >
        <Table
          columns={resultColumns}
          dataSource={results}
          rowKey="id"
          pagination={{ pageSize: 20 }}
          size="small"
        />
      </Drawer>
    </div>
  );
};

export default Telemetry;
