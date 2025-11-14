import React, { useState, useEffect } from 'react';
import {
  Table,
  Button,
  Space,
  Tag,
  Modal,
  Form,
  Input,
  message,
  Popconfirm,
} from 'antd';
import {
  PlusOutlined,
  EditOutlined,
  DeleteOutlined,
  SyncOutlined,
} from '@ant-design/icons';
import { getGroups, addGroup, updateGroup, deleteGroup } from '../../api';

const GroupManager = () => {
  const [groups, setGroups] = useState([]);
  const [loading, setLoading] = useState(false);
  const [modalVisible, setModalVisible] = useState(false);
  const [editingGroup, setEditingGroup] = useState(null);
  const [form] = Form.useForm();

  useEffect(() => {
    loadGroups();
  }, []);

  const loadGroups = async () => {
    try {
      setLoading(true);
      const response = await getGroups();
      setGroups(response.data || []);
    } catch (error) {
      message.error('加载分组失败');
    } finally {
      setLoading(false);
    }
  };

  const handleAdd = () => {
    setEditingGroup(null);
    form.resetFields();
    form.setFieldsValue({ color: '#1890ff' });
    setModalVisible(true);
  };

  const handleEdit = (record) => {
    setEditingGroup(record);
    form.setFieldsValue(record);
    setModalVisible(true);
  };

  const handleDelete = async (id) => {
    try {
      await deleteGroup(id);
      message.success('删除成功');
      loadGroups();
    } catch (error) {
      message.error(error.response?.data?.message || '删除失败');
    }
  };

  const handleSubmit = async () => {
    try {
      const values = await form.validateFields();
      
      if (editingGroup) {
        await updateGroup(editingGroup.id, values);
        message.success('更新成功');
      } else {
        await addGroup(values);
        message.success('添加成功');
      }
      
      setModalVisible(false);
      loadGroups();
    } catch (error) {
      message.error(error.response?.data?.message || '操作失败');
    }
  };

  const columns = [
    {
      title: '分组名称',
      dataIndex: 'name',
      key: 'name',
      render: (text, record) => (
        <Space>
          <Tag color={record.color}>{text}</Tag>
          {record.is_system && <Tag color="gold">系统</Tag>}
        </Space>
      ),
    },
    {
      title: '描述',
      dataIndex: 'description',
      key: 'description',
    },
    {
      title: '创建时间',
      dataIndex: 'created_at',
      key: 'created_at',
      render: (time) => new Date(time).toLocaleString(),
    },
    {
      title: '操作',
      key: 'action',
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
          {!record.is_system && (
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
          )}
        </Space>
      ),
    },
  ];

  return (
    <div>
      <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between' }}>
        <Space>
          <Button type="primary" icon={<PlusOutlined />} onClick={handleAdd}>
            添加分组
          </Button>
          <Button icon={<SyncOutlined />} onClick={loadGroups}>
            刷新
          </Button>
        </Space>
        <span style={{ color: '#666' }}>共 {groups.length} 个分组</span>
      </div>

      <Table
        columns={columns}
        dataSource={groups}
        rowKey="id"
        loading={loading}
        pagination={false}
      />

      <Modal
        title={editingGroup ? '编辑分组' : '添加分组'}
        open={modalVisible}
        onOk={handleSubmit}
        onCancel={() => setModalVisible(false)}
        okText="确定"
        cancelText="取消"
      >
        <Form form={form} layout="vertical">
          <Form.Item
            name="name"
            label="分组名称"
            rules={[
              { required: true, message: '请输入分组名称' },
              { pattern: /^[a-zA-Z0-9_-]+$/, message: '只能包含字母、数字、下划线和横线' },
            ]}
          >
            <Input
              placeholder="例如: custom_group"
              disabled={editingGroup?.is_system}
            />
          </Form.Item>

          <Form.Item name="description" label="描述">
            <Input.TextArea rows={3} placeholder="分组描述" />
          </Form.Item>

          <Form.Item name="color" label="颜色">
            <ColorPicker />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

// 简单的颜色选择器组件
const ColorPicker = ({ value, onChange }) => {
  const [showPicker, setShowPicker] = useState(false);

  const presetColors = [
    '#1890ff', '#52c41a', '#faad14', '#f5222d',
    '#722ed1', '#13c2c2', '#eb2f96', '#fa8c16',
  ];

  return (
    <div>
      <Space>
        <div
          style={{
            width: 36,
            height: 36,
            borderRadius: 4,
            border: '1px solid #d9d9d9',
            backgroundColor: value || '#1890ff',
            cursor: 'pointer',
          }}
          onClick={() => setShowPicker(!showPicker)}
        />
        {presetColors.map((color) => (
          <div
            key={color}
            style={{
              width: 24,
              height: 24,
              borderRadius: 4,
              backgroundColor: color,
              cursor: 'pointer',
              border: value === color ? '2px solid #000' : '1px solid #d9d9d9',
            }}
            onClick={() => onChange(color)}
          />
        ))}
      </Space>
    </div>
  );
};

export default GroupManager;