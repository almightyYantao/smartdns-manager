import React, { useState } from 'react';
import {
  Card,
  Tabs,
  Form,
  Input,
  Button,
  Space,
  message,
  Switch,
  Divider,
  Alert,
} from 'antd';
import {
  UserOutlined,
  LockOutlined,
  SettingOutlined,
  BellOutlined,
  DatabaseOutlined,
} from '@ant-design/icons';
import { getUserInfo } from '../utils/auth';
import DatabaseBackupManager from '../components/Backup/DatabaseBackupManager';

const Settings = () => {
  const [form] = Form.useForm();
  const [passwordForm] = Form.useForm();
  const userInfo = getUserInfo();

  const handleUpdateProfile = async (values) => {
    try {
      // TODO: 调用更新用户信息API
      message.success('个人信息更新成功');
    } catch (error) {
      message.error('更新失败');
    }
  };

  const handleChangePassword = async (values) => {
    try {
      // TODO: 调用修改密码API
      message.success('密码修改成功');
      passwordForm.resetFields();
    } catch (error) {
      message.error('密码修改失败');
    }
  };

  const profileTab = (
    <Card>
      <Form
        form={form}
        layout="vertical"
        onFinish={handleUpdateProfile}
        initialValues={userInfo}
      >
        <Form.Item
          name="username"
          label="用户名"
          rules={[{ required: true, message: '请输入用户名' }]}
        >
          <Input prefix={<UserOutlined />} disabled />
        </Form.Item>

        <Form.Item
          name="email"
          label="邮箱"
          rules={[
            { required: true, message: '请输入邮箱' },
            { type: 'email', message: '请输入有效的邮箱地址' },
          ]}
        >
          <Input prefix={<UserOutlined />} />
        </Form.Item>

        <Form.Item>
          <Button type="primary" htmlType="submit">
            保存修改
          </Button>
        </Form.Item>
      </Form>
    </Card>
  );

  const securityTab = (
    <Card>
      <Alert
        message="修改密码"
        description="为了账户安全，建议定期修改密码。"
        type="info"
        showIcon
        style={{ marginBottom: 24 }}
      />
      
      <Form
        form={passwordForm}
        layout="vertical"
        onFinish={handleChangePassword}
      >
        <Form.Item
          name="old_password"
          label="当前密码"
          rules={[{ required: true, message: '请输入当前密码' }]}
        >
          <Input.Password prefix={<LockOutlined />} />
        </Form.Item>

        <Form.Item
          name="new_password"
          label="新密码"
          rules={[
            { required: true, message: '请输入新密码' },
            { min: 6, message: '密码至少6个字符' },
          ]}
        >
          <Input.Password prefix={<LockOutlined />} />
        </Form.Item>

        <Form.Item
          name="confirm_password"
          label="确认新密码"
          dependencies={['new_password']}
          rules={[
            { required: true, message: '请确认新密码' },
            ({ getFieldValue }) => ({
              validator(_, value) {
                if (!value || getFieldValue('new_password') === value) {
                  return Promise.resolve();
                }
                return Promise.reject(new Error('两次输入的密码不一致'));
              },
            }),
          ]}
        >
          <Input.Password prefix={<LockOutlined />} />
        </Form.Item>

        <Form.Item>
          <Button type="primary" htmlType="submit">
            修改密码
          </Button>
        </Form.Item>
      </Form>
    </Card>
  );

  const systemTab = (
    <Card>
      <Space direction="vertical" style={{ width: '100%' }} size="large">
        <div>
          <div style={{ marginBottom: 16 }}>
            <strong>关于系统</strong>
          </div>
          <Space direction="vertical">
            <div>作者: Almighty.Yantao</div>
            <div>GitHub: https://github.com/almightyYantao/smartdns-manager</div>
          </Space>
        </div>
      </Space>
    </Card>
  );

  const backupTab = (
    <DatabaseBackupManager />
  );

  return (
    <Card title="系统设置" bordered={false}>
      <Tabs
        items={[
          {
            key: 'profile',
            label: (
              <span>
                <UserOutlined />
                个人信息
              </span>
            ),
            children: profileTab,
          },
          {
            key: 'security',
            label: (
              <span>
                <LockOutlined />
                安全设置
              </span>
            ),
            children: securityTab,
          },
          {
            key: 'backup',
            label: (
              <span>
                <DatabaseOutlined />
                数据库备份
              </span>
            ),
            children: backupTab,
          },
          {
            key: 'system',
            label: (
              <span>
                <SettingOutlined />
                系统设置
              </span>
            ),
            children: systemTab,
          },
        ]}
      />
    </Card>
  );
};

export default Settings;