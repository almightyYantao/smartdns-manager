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
} from 'antd';
import {
  PlusOutlined,
  EditOutlined,
  DeleteOutlined,
} from '@ant-design/icons';
import {
  getNameservers,
  addNameserver,
  updateNameserver,
  deleteNameserver,
  getDomainSets,
  getNodes,
  getServers,
} from '../../api';

const { Option } = Select;

const NameserverManager = () => {
  const [nameservers, setNameservers] = useState([]);
  const [domainSets, setDomainSets] = useState([]);
  const [nodes, setNodes] = useState([]);
  const [serverGroups, setServerGroups] = useState([]);
  const [loading, setLoading] = useState(false);
  const [modalVisible, setModalVisible] = useState(false);
  const [editingNameserver, setEditingNameserver] = useState(null);
  const [form] = Form.useForm();
  const [isDomainSet, setIsDomainSet] = useState(false);

  useEffect(() => {
    loadNameservers();
    loadDomainSets();
    loadNodes();
    loadServerGroups();
  }, []);

  const loadNameservers = async () => {
    try {
      setLoading(true);
      const response = await getNameservers();
      setNameservers(response.data || []);
    } catch (error) {
      message.error('加载命名服务器规则失败');
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
    setEditingNameserver(null);
    form.resetFields();
    setIsDomainSet(false);
    setModalVisible(true);
  };

  const handleEdit = (record) => {
    setEditingNameserver(record);
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
      await deleteNameserver(id);
      message.success('删除成功');
      loadNameservers();
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

      if (editingNameserver) {
        await updateNameserver(editingNameserver.id, data);
        message.success('更新成功');
      } else {
        await addNameserver(data);
        message.success('添加成功');
      }

      setModalVisible(false);
      loadNameservers();
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
      width: 300,
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
      title: '服务器组',
      dataIndex: 'group',
      key: 'group',
      width: 150,
      render: (text) => <Tag color="cyan">{text}</Tag>,
    },
    {
      title: '描述',
      dataIndex: 'description',
      key: 'description',
      ellipsis: true,
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
          添加命名服务器规则
        </Button>
        <span style={{ marginLeft: 16, color: '#666' }}>
          共 {nameservers.length} 条规则
        </span>
      </div>

      <Table
        columns={columns}
        dataSource={nameservers}
        rowKey="id"
        loading={loading}
        scroll={{ x: 1200 }}
        pagination={{
          pageSize: 20,
          showSizeChanger: true,
          showTotal: (total) => `共 ${total} 条记录`,
        }}
      />

      <Modal
        title={editingNameserver ? '编辑命名服务器规则' : '添加命名服务器规则'}
        open={modalVisible}
        onOk={handleSubmit}
        onCancel={() => setModalVisible(false)}
        width={600}
        okText="确定"
        cancelText="取消"
      >
        <Form form={form} layout="vertical">
          <Form.Item label="规则类型">
            <Select
              value={isDomainSet}
              onChange={setIsDomainSet}
              disabled={!!editingNameserver}
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

          <Form.Item
            name="group"
            label="服务器组"
            rules={[{ required: true, message: '请选择服务器组' }]}
            extra="指定该域名使用的上游DNS服务器组"
          >
            <Select placeholder="选择服务器组">
              {serverGroups.map((group) => (
                <Option key={group} value={group}>
                  {group}
                </Option>
              ))}
            </Select>
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

export default NameserverManager;