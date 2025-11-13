import React, { useState, useEffect } from 'react';
import {
  Table,
  Button,
  Space,
  Tag,
  Modal,
  Form,
  Input,
  Select,
  message,
  Popconfirm,
  Switch,
  Card,
  Tabs,
  Badge,
} from 'antd';
import {
  PlusOutlined,
  EditOutlined,
  DeleteOutlined,
  BellOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons';
import {
  getNotificationChannels,
  addNotificationChannel,
  updateNotificationChannel,
  deleteNotificationChannel,
  testNotificationChannel,
  getNotificationLogs,
  getNodes,
} from '../../api';
import moment from 'moment';

const { Option } = Select;
const { TextArea } = Input;

const NotificationManager = () => {
  const [channels, setChannels] = useState([]);
  const [nodes, setNodes] = useState([]);
  const [logs, setLogs] = useState([]);
  const [loading, setLoading] = useState(false);
  const [modalVisible, setModalVisible] = useState(false);
  const [editingChannel, setEditingChannel] = useState(null);
  const [activeTab, setActiveTab] = useState('channels');
  const [form] = Form.useForm();

  const channelTypes = [
    { value: 'wechat', label: '企业微信', icon: '💬' },
    { value: 'dingtalk', label: '钉钉', icon: '📱' },
    { value: 'feishu', label: '飞书', icon: '🚀' },
    { value: 'slack', label: 'Slack', icon: '💼' },
  ];

  const eventTypes = [
    { value: '*', label: '全部事件', color: 'blue' },
    { value: 'sync_success', label: '同步成功', color: 'green' },
    { value: 'sync_failed', label: '同步失败', color: 'red' },
    { value: 'node_online', label: '节点上线', color: 'cyan' },
    { value: 'node_offline', label: '节点离线', color: 'orange' },
    { value: 'service_restart', label: '服务重启', color: 'purple' },
    { value: 'config_backup', label: '配置备份', color: 'geekblue' },
    { value: 'config_restore', label: '配置恢复', color: 'magenta' },
  ];

  useEffect(() => {
    loadChannels();
    loadNodes();
    loadLogs();
  }, []);

  const loadChannels = async () => {
    try {
      setLoading(true);
      const response = await getNotificationChannels();
      setChannels(response.data || []);
    } catch (error) {
      message.error('加载通知渠道失败');
    } finally {
      setLoading(false);
    }
  };

  const loadNodes = async () => {
    try {
      const response = await getNodes();
      setNodes(response.data || []);
    } catch (error) {
      console.error('加载节点列表失败', error);
    }
  };

  const loadLogs = async () => {
    try {
      const response = await getNotificationLogs({ page: 1, page_size: 50 });
      setLogs(response.data || []);
    } catch (error) {
      console.error('加载通知日志失败', error);
    }
  };

  const handleAdd = () => {
    setEditingChannel(null);
    form.resetFields();
    setModalVisible(true);
  };

  const handleEdit = (record) => {
    setEditingChannel(record);
    const events = record.events ? JSON.parse(record.events) : [];
    form.setFieldsValue({
      ...record,
      events,
    });
    setModalVisible(true);
  };

  const handleDelete = async (id) => {
    try {
      await deleteNotificationChannel(id);
      message.success('删除成功');
      loadChannels();
    } catch (error) {
      message.error('删除失败');
    }
  };

  const handleTest = async (id) => {
    try {
      message.loading({ content: '正在发送测试消息...', key: 'test' });
      await testNotificationChannel(id);
      message.success({ content: '测试消息已发送，请检查通知渠道', key: 'test' });
    } catch (error) {
      message.error({ content: '发送失败', key: 'test' });
    }
  };

  const handleSubmit = async () => {
    try {
      const values = await form.validateFields();

      // 转换 events 为 JSON 字符串
      if (values.events && values.events.length > 0) {
        values.events = JSON.stringify(values.events);
      } else {
        values.events = '[]';
      }

      if (editingChannel) {
        await updateNotificationChannel(editingChannel.id, values);
        message.success('更新成功');
      } else {
        await addNotificationChannel(values);
        message.success('添加成功');
      }

      setModalVisible(false);
      loadChannels();
    } catch (error) {
      message.error('操作失败');
    }
  };

  const getTypeIcon = (type) => {
    const found = channelTypes.find(t => t.value === type);
    return found ? found.icon : '📢';
  };

  const getTypeLabel = (type) => {
    const found = channelTypes.find(t => t.value === type);
    return found ? found.label : type;
  };

  const parseEvents = (eventsStr) => {
    if (!eventsStr || eventsStr === '[]') return ['*'];
    try {
      return JSON.parse(eventsStr);
    } catch {
      return [];
    }
  };

  const channelColumns = [
    {
      title: '名称',
      dataIndex: 'name',
      key: 'name',
      width: 200,
      render: (text, record) => (
        <Space>
          <span style={{ fontSize: 20 }}>{getTypeIcon(record.type)}</span>
          <strong>{text}</strong>
        </Space>
      ),
    },
    {
      title: '类型',
      dataIndex: 'type',
      key: 'type',
      width: 120,
      render: (type) => (
        <Tag color="blue">{getTypeLabel(type)}</Tag>
      ),
    },
    {
      title: '应用节点',
      dataIndex: 'node_id',
      key: 'node_id',
      width: 150,
      render: (nodeId) => {
        if (nodeId === 0) {
          return <Tag color="green">全局</Tag>;
        }
        const node = nodes.find(n => n.id === nodeId);
        return node ? <Tag color="cyan">{node.name}</Tag> : <Tag>-</Tag>;
      },
    },
    {
      title: '订阅事件',
      dataIndex: 'events',
      key: 'events',
      render: (eventsStr) => {
        const events = parseEvents(eventsStr);
        return (
          <Space wrap>
            {events.map(event => {
              const eventType = eventTypes.find(e => e.value === event);
              return eventType ? (
                <Tag key={event} color={eventType.color}>
                  {eventType.label}
                </Tag>
              ) : null;
            })}
          </Space>
        );
      },
    },
    {
      title: '状态',
      dataIndex: 'enabled',
      key: 'enabled',
      width: 80,
      render: (enabled) => (
        <Badge
          status={enabled ? 'success' : 'default'}
          text={enabled ? '启用' : '禁用'}
        />
      ),
    },
    {
      title: '创建时间',
      dataIndex: 'created_at',
      key: 'created_at',
      width: 180,
      render: (time) => moment(time).format('YYYY-MM-DD HH:mm:ss'),
    },
    {
      title: '操作',
      key: 'action',
      fixed: 'right',
      width: 200,
      render: (_, record) => (
        <Space size="small">
          <Button
            type="link"
            size="small"
            icon={<ThunderboltOutlined />}
            onClick={() => handleTest(record.id)}
          >
            测试
          </Button>
          <Button
            type="link"
            size="small"
            icon={<EditOutlined />}
            onClick={() => handleEdit(record)}
          >
            编辑
          </Button>
          <Popconfirm
            title="确定要删除吗？"
            onConfirm={() => handleDelete(record.id)}
            okText="确定"
            cancelText="取消"
          >
            <Button
              type="link"
              size="small"
              danger
              icon={<DeleteOutlined />}
            >
              删除
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  const logColumns = [
    {
      title: '时间',
      dataIndex: 'sent_at',
      key: 'sent_at',
      width: 180,
      render: (time) => moment(time).format('YYYY-MM-DD HH:mm:ss'),
    },
    {
      title: '节点',
      dataIndex: 'node_id',
      key: 'node_id',
      width: 120,
      render: (nodeId) => {
        const node = nodes.find(n => n.id === nodeId);
        return node ? node.name : '-';
      },
    },
    {
      title: '事件类型',
      dataIndex: 'event_type',
      key: 'event_type',
      width: 120,
      render: (eventType) => {
        const event = eventTypes.find(e => e.value === eventType);
        return event ? (
          <Tag color={event.color}>{event.label}</Tag>
        ) : (
          <Tag>{eventType}</Tag>
        );
      },
    },
    {
      title: '标题',
      dataIndex: 'title',
      key: 'title',
      ellipsis: true,
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 100,
      render: (status) => (
        <Space>
          {status === 'success' ? (
            <CheckCircleOutlined style={{ color: '#52c41a' }} />
          ) : (
            <CloseCircleOutlined style={{ color: '#f5222d' }} />
          )}
          <Tag color={status === 'success' ? 'success' : 'error'}>
            {status === 'success' ? '成功' : '失败'}
          </Tag>
        </Space>
      ),
    },
    {
      title: '错误信息',
      dataIndex: 'error',
      key: 'error',
      ellipsis: true,
      render: (error) => error || '-',
    },
  ];

  const channelsTab = (
    <>
      <div style={{ marginBottom: 16 }}>
        <Button
          type="primary"
          icon={<PlusOutlined />}
          onClick={handleAdd}
        >
          添加通知渠道
        </Button>
      </div>

      <Table
        columns={channelColumns}
        dataSource={channels}
        rowKey="id"
        loading={loading}
        scroll={{ x: 1200 }}
        pagination={{
          pageSize: 10,
          showSizeChanger: true,
          showTotal: (total) => `共 ${total} 条记录`,
        }}
      />
    </>
  );

  const logsTab = (
    <Table
      columns={logColumns}
      dataSource={logs}
      rowKey="id"
      loading={loading}
      scroll={{ x: 1000 }}
      pagination={{
        pageSize: 20,
        showSizeChanger: true,
        showTotal: (total) => `共 ${total} 条记录`,
      }}
    />
  );

  return (
    <Card title="通知管理" bordered={false}>
      <Tabs
        activeKey={activeTab}
        onChange={setActiveTab}
        items={[
          {
            key: 'channels',
            label: (
              <span>
                <BellOutlined />
                通知渠道
              </span>
            ),
            children: channelsTab,
          },
          {
            key: 'logs',
            label: '通知日志',
            children: logsTab,
          },
        ]}
      />

      <Modal
        title={editingChannel ? '编辑通知渠道' : '添加通知渠道'}
        open={modalVisible}
        onOk={handleSubmit}
        onCancel={() => setModalVisible(false)}
        width={700}
        okText="确定"
        cancelText="取消"
      >
        <Form form={form} layout="vertical">
          <Form.Item
            name="name"
            label="渠道名称"
            rules={[{ required: true, message: '请输入渠道名称' }]}
          >
            <Input placeholder="例如: 运维群机器人" />
          </Form.Item>

          <Form.Item
            name="type"
            label="渠道类型"
            rules={[{ required: true, message: '请选择渠道类型' }]}
          >
            <Select placeholder="选择通知渠道类型">
              {channelTypes.map(type => (
                <Option key={type.value} value={type.value}>
                  <Space>
                    <span>{type.icon}</span>
                    <span>{type.label}</span>
                  </Space>
                </Option>
              ))}
            </Select>
          </Form.Item>

          <Form.Item
            name="node_id"
            label="应用节点"
            extra="选择0或留空表示全局通知渠道"
            initialValue={0}
          >
            <Select placeholder="选择节点" allowClear>
              <Option value={0}>全局（所有节点）</Option>
              {nodes.map(node => (
                <Option key={node.id} value={node.id}>
                  {node.name}
                </Option>
              ))}
            </Select>
          </Form.Item>

          <Form.Item
            name="webhook_url"
            label="Webhook URL"
            rules={[
              { required: true, message: '请输入 Webhook URL' },
              { type: 'url', message: '请输入有效的 URL' },
            ]}
          >
            <Input placeholder="https://..." />
          </Form.Item>

          <Form.Item
            name="secret"
            label="签名密钥"
            extra="钉钉和飞书机器人需要配置签名密钥"
          >
            <Input.Password placeholder="安全设置中的加签密钥" />
          </Form.Item>

          <Form.Item
            name="events"
            label="订阅事件"
            extra="不选择则订阅所有事件"
          >
            <Select
              mode="multiple"
              placeholder="选择要订阅的事件类型"
              allowClear
            >
              {eventTypes.map(event => (
                <Option key={event.value} value={event.value}>
                  <Tag color={event.color}>{event.label}</Tag>
                </Option>
              ))}
            </Select>
          </Form.Item>

          <Form.Item
            name="enabled"
            label="启用状态"
            valuePropName="checked"
            initialValue={true}
          >
            <Switch checkedChildren="启用" unCheckedChildren="禁用" />
          </Form.Item>
        </Form>
      </Modal>
    </Card>
  );
};

export default NotificationManager;