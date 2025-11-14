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
  InputNumber,
  Switch,
} from 'antd';
import {
  PlusOutlined,
  EditOutlined,
  DeleteOutlined,
} from '@ant-design/icons';
import {
  getDomainRules,
  addDomainRule,
  updateDomainRule,
  deleteDomainRule,
  getDomainSets,
  getNodes,
  getServers,
} from '../../api';
import moment from 'moment';

const { Option } = Select;

const DomainRuleManager = () => {
  const [rules, setRules] = useState([]);
  const [domainSets, setDomainSets] = useState([]);
  const [nodes, setNodes] = useState([]);
  const [serverGroups, setServerGroups] = useState([]);
  const [loading, setLoading] = useState(false);
  const [modalVisible, setModalVisible] = useState(false);
  const [editingRule, setEditingRule] = useState(null);
  const [form] = Form.useForm();
  const [isDomainSet, setIsDomainSet] = useState(false);

  useEffect(() => {
    loadRules();
    loadDomainSets();
    loadNodes();
    loadServerGroups();
  }, []);

  const loadRules = async () => {
    try {
      setLoading(true);
      const response = await getDomainRules();
      setRules(response.data || []);
    } catch (error) {
      message.error('加载域名规则失败');
    } finally {
      setLoading(false);
    }
  };

  const loadDomainSets = async () => {
    try {
      const response = await getDomainSets();
      setDomainSets(response.data || []);
    } catch (error) {
      console.error('加载域名集失败', error);
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

  const loadServerGroups = async () => {
    try {
      const response = await getServers();
      const groups = [...new Set(response.data.flatMap(s => s.groups || []))];
      setServerGroups(groups);
    } catch (error) {
      console.error('加载服务器组失败', error);
    }
  };

  const handleAdd = () => {
    setEditingRule(null);
    form.resetFields();
    setIsDomainSet(false);
    setModalVisible(true);
  };

  const handleEdit = (record) => {
    setEditingRule(record);
    setIsDomainSet(record.is_domain_set);
    const nodeIds = record.node_ids ? JSON.parse(record.node_ids) : [];
    
    form.setFieldsValue({
      ...record,
      node_ids: nodeIds,
    });
    
    setModalVisible(true);
  };

  const handleDelete = async (id) => {
    try {
      await deleteDomainRule(id);
      message.success('删除成功');
      loadRules();
    } catch (error) {
      message.error('删除失败');
    }
  };

  const handleSubmit = async () => {
    try {
      const values = await form.validateFields();
      
      const data = {
        ...values,
        is_domain_set: isDomainSet,
        node_ids: values.node_ids || [],
      };

      if (editingRule) {
        await updateDomainRule(editingRule.id, data);
        message.success('更新成功');
      } else {
        await addDomainRule(data);
        message.success('添加成功');
      }

      setModalVisible(false);
      loadRules();
    } catch (error) {
      message.error('操作失败');
    }
  };

  const columns = [
    {
      title: '优先级',
      dataIndex: 'priority',
      key: 'priority',
      width: 80,
      sorter: (a, b) => b.priority - a.priority,
      render: (priority) => (
        <Tag color={priority > 0 ? 'red' : 'default'}>{priority}</Tag>
      ),
    },
    {
      title: '域名/域名集',
      dataIndex: 'domain',
      key: 'domain',
      width: 250,
      render: (text, record) => (
        <Space>
          {record.is_domain_set ? (
            <>
              <Tag color="purple">域名集</Tag>
              <code>domain-set:{record.domain_set_name}</code>
            </>
          ) : (
            <code>{text}</code>
          )}
        </Space>
      ),
    },
    {
      title: '地址',
      dataIndex: 'address',
      key: 'address',
      width: 150,
      render: (text) => text || '-',
    },
    {
      title: '命名服务器',
      dataIndex: 'nameserver',
      key: 'nameserver',
      width: 120,
      render: (text) => (text ? <Tag color="cyan">{text}</Tag> : '-'),
    },
    {
      title: '速度检查模式',
      dataIndex: 'speed_check_mode',
      key: 'speed_check_mode',
      width: 120,
      render: (text) => text || '-',
    },
    {
      title: '其他选项',
      dataIndex: 'other_options',
      key: 'other_options',
      ellipsis: true,
      render: (text) => text || '-',
    },
    {
      title: '状态',
      dataIndex: 'enabled',
      key: 'enabled',
      width: 80,
      render: (enabled) => (
        <Tag color={enabled ? 'success' : 'default'}>
          {enabled ? '启用' : '禁用'}
        </Tag>
      ),
    },
    {
      title: '操作',
      key: 'action',
      fixed: 'right',
      width: 150,
      render: (_, record) => (
        <Space size="small">
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
            <Button type="link" size="small" danger icon={<DeleteOutlined />}>
              删除
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <div>
      <div style={{ marginBottom: 16 }}>
        <Button type="primary" icon={<PlusOutlined />} onClick={handleAdd}>
          添加域名规则
        </Button>
        <span style={{ marginLeft: 16, color: '#666' }}>
          共 {rules.length} 条规则
        </span>
      </div>

      <Table
        columns={columns}
        dataSource={rules}
        rowKey="id"
        loading={loading}
        scroll={{ x: 1400 }}
        pagination={{
          pageSize: 20,
          showSizeChanger: true,
          showTotal: (total) => `共 ${total} 条记录`,
        }}
      />

      <Modal
        title={editingRule ? '编辑域名规则' : '添加域名规则'}
        open={modalVisible}
        onOk={handleSubmit}
        onCancel={() => setModalVisible(false)}
        width={700}
        okText="确定"
        cancelText="取消"
      >
        <Form form={form} layout="vertical">
          <Form.Item label="规则类型">
            <Select
              value={isDomainSet}
              onChange={setIsDomainSet}
              disabled={!!editingRule}
            >
              <Option value={false}>普通域名</Option>
              <Option value={true}>域名集</Option>
            </Select>
          </Form.Item>

          {isDomainSet ? (
            <Form.Item
              name="domain_set_name"
              label="域名集"
              rules={[{ required: true, message: '请选择域名集' }]}
            >
              <Select placeholder="选择域名集">
                {domainSets.map((ds) => (
                  <Option key={ds.name} value={ds.name}>
                    {ds.name} ({ds.domain_count} 个域名)
                  </Option>
                ))}
              </Select>
            </Form.Item>
          ) : (
            <Form.Item
              name="domain"
              label="域名"
              rules={[{ required: true, message: '请输入域名' }]}
              extra="支持通配符，例如: *.example.com"
            >
              <Input placeholder="例如: example.com 或 *.example.com" />
            </Form.Item>
          )}

          <Form.Item name="address" label="地址（-address）" extra="指定返回的IP地址">
            <Input placeholder="例如: 1.2.3.4" />
          </Form.Item>

          <Form.Item
            name="nameserver"
            label="命名服务器（-nameserver）"
            extra="指定使用的上游DNS服务器组"
          >
            <Select placeholder="选择服务器组" allowClear>
              {serverGroups.map((group) => (
                <Option key={group} value={group}>
                  {group}
                </Option>
              ))}
            </Select>
          </Form.Item>

          <Form.Item
            name="speed_check_mode"
            label="速度检查模式（-speed-check-mode）"
          >
            <Select placeholder="选择速度检查模式" allowClear>
              <Option value="ping">ping</Option>
              <Option value="tcp:80">tcp:80</Option>
              <Option value="tcp:443">tcp:443</Option>
              <Option value="none">none</Option>
            </Select>
          </Form.Item>

          <Form.Item name="other_options" label="其他选项">
            <Input placeholder="其他命令行选项" />
          </Form.Item>

          <Form.Item
            name="priority"
            label="优先级"
            extra="数字越大优先级越高"
            initialValue={0}
          >
            <InputNumber min={0} max={100} style={{ width: '100%' }} />
          </Form.Item>

          <Form.Item name="node_ids" label="应用到节点" extra="不选择则应用到所有节点">
            <Select mode="multiple" placeholder="选择节点" allowClear>
              {nodes.map((node) => (
                <Option key={node.id} value={node.id}>
                  {node.name}
                </Option>
              ))}
            </Select>
          </Form.Item>

          <Form.Item name="description" label="描述">
            <Input.TextArea rows={2} placeholder="规则描述" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default DomainRuleManager;