import React, { useState, useEffect } from 'react';
import { Form, Input, InputNumber, Button, message, Select, Space } from 'antd';
import { addNode, updateNode } from '../../api';

const { TextArea } = Input;
const { Option } = Select;

const NodeForm = ({ node, onSuccess, onCancel }) => {
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const [authMethod, setAuthMethod] = useState('password');

  useEffect(() => {
    if (node) {
      form.setFieldsValue(node);
      setAuthMethod(node.private_key ? 'key' : 'password');
    }
  }, [node, form]);

  const onFinish = async (values) => {
    try {
      setLoading(true);
      
      if (authMethod === 'password') {
        delete values.private_key;
      } else {
        delete values.password;
      }

      if (node) {
        await updateNode(node.id, values);
        message.success('节点更新成功');
      } else {
        await addNode(values);
        message.success('节点添加成功');
      }
      
      onSuccess();
    } catch (error) {
      message.error(node ? '节点更新失败' : '节点添加失败');
    } finally {
      setLoading(false);
    }
  };

  return (
    <Form
      form={form}
      layout="vertical"
      onFinish={onFinish}
      initialValues={{
        port: 22,
        config_path: '/etc/smartdns/smartdns.conf',
      }}
    >
      <Form.Item
        name="name"
        label="节点名称"
        rules={[{ required: true, message: '请输入节点名称' }]}
      >
        <Input placeholder="例如: 主节点" />
      </Form.Item>

      <Form.Item
        name="host"
        label="主机地址"
        rules={[{ required: true, message: '请输入主机地址' }]}
      >
        <Input placeholder="例如: 192.168.1.100" />
      </Form.Item>

      <Form.Item
        name="port"
        label="SSH端口"
        rules={[{ required: true, message: '请输入SSH端口' }]}
      >
        <InputNumber min={1} max={65535} style={{ width: '100%' }} />
      </Form.Item>

      <Form.Item
        name="username"
        label="用户名"
        rules={[{ required: true, message: '请输入用户名' }]}
      >
        <Input placeholder="例如: root" />
      </Form.Item>

      <Form.Item label="认证方式">
        <Select value={authMethod} onChange={setAuthMethod}>
          <Option value="password">密码</Option>
          <Option value="key">私钥</Option>
        </Select>
      </Form.Item>

      {authMethod === 'password' ? (
        <Form.Item
          name="password"
          label="密码"
          rules={[{ required: !node, message: '请输入密码' }]}
        >
          <Input.Password placeholder="SSH登录密码" />
        </Form.Item>
      ) : (
        <Form.Item
          name="private_key"
          label="私钥"
          rules={[{ required: !node, message: '请输入私钥' }]}
        >
          <TextArea rows={6} placeholder="粘贴SSH私钥内容" />
        </Form.Item>
      )}

      <Form.Item
        name="config_path"
        label="配置文件路径"
        rules={[{ required: true, message: '请输入配置文件路径' }]}
      >
        <Input placeholder="/etc/smartdns/smartdns.conf" />
      </Form.Item>

      <Form.Item
        name="tags"
        label="标签"
      >
        <Input placeholder="标签，多个用逗号分隔" />
      </Form.Item>

      <Form.Item
        name="description"
        label="描述"
      >
        <TextArea rows={3} placeholder="节点描述信息" />
      </Form.Item>

      <Form.Item>
        <Space>
          <Button type="primary" htmlType="submit" loading={loading}>
            {node ? '更新' : '添加'}
          </Button>
          <Button onClick={onCancel}>
            取消
          </Button>
        </Space>
      </Form.Item>
    </Form>
  );
};

export default NodeForm;