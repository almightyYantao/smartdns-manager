import React, { useEffect, useState } from 'react';
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
  Upload,
} from 'antd';
import {
  UserOutlined,
  LockOutlined,
  SettingOutlined,
  BellOutlined,
  DatabaseOutlined,
} from '@ant-design/icons';
import { useSearchParams } from 'react-router-dom';
import { getUserInfo } from '../utils/auth';
import request from '../utils/request';
import DatabaseBackupManager from '../components/Backup/DatabaseBackupManager';

const Settings = () => {
  const [searchParams] = useSearchParams();
  const [form] = Form.useForm();
  const [passwordForm] = Form.useForm();
  const userInfo = getUserInfo();
  const [coreSourceForm] = Form.useForm();
  const [coreSourceReady, setCoreSourceReady] = useState(false);
  const [savingCoreSource, setSavingCoreSource] = useState(false);
  const [offlineVersion, setOfflineVersion] = useState('');
  const [bundleBusy, setBundleBusy] = useState(false);

  useEffect(() => {
    let active = true;
    request.get('/settings/core-source').then(({ data }) => {
      if (active) {
        coreSourceForm.setFieldsValue(data);
        setOfflineVersion(data.offline_version || '');
        setCoreSourceReady(true);
      }
    }).catch(() => {});
    return () => { active = false; };
  }, [coreSourceForm]);

  const importCoreBundle = async ({ file, onSuccess, onError }) => {
    setBundleBusy(true);
    try {
      const { data } = await request.post('/settings/core-source/bundle', file, {
        headers: { 'Content-Type': 'application/zip' },
      });
      setOfflineVersion(data);
      message.success(`已导入 ${data}`);
      onSuccess();
    } catch (error) {
      onError(error);
    } finally {
      setBundleBusy(false);
    }
  };

  const useOnlineCoreSource = async () => {
    setBundleBusy(true);
    try {
      await request.delete('/settings/core-source/bundle');
      setOfflineVersion('');
      message.success('已切换到在线发布源');
    } catch {
      // 请求层已提示失败原因。
    } finally {
      setBundleBusy(false);
    }
  };

  const saveCoreSource = async (values) => {
    setSavingCoreSource(true);
    try {
      const { data } = await request.put('/settings/core-source', values);
      message.success(`发布源校验通过并已保存 · ${data}`);
    } catch (error) {
      message.error(error.response?.data?.message || error.message);
    } finally {
      setSavingCoreSource(false);
    }
  };


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
        <Card title="SmartDNS 核心发布源" size="small">
          <Space wrap style={{ marginBottom: 16 }}>
            <Upload accept=".zip" showUploadList={false} customRequest={importCoreBundle}
              disabled={!coreSourceReady || bundleBusy || savingCoreSource}>
              <Button type="primary" loading={bundleBusy}>上传离线升级包</Button>
            </Upload>
            {offlineVersion && <>
              <span>当前使用离线版本 {offlineVersion}</span>
              <Button disabled={bundleBusy} onClick={useOnlineCoreSource}>切换到在线发布源</Button>
            </>}
          </Space>
          <Form form={coreSourceForm} layout="vertical" onFinish={saveCoreSource}
            disabled={!coreSourceReady || savingCoreSource || bundleBusy || Boolean(offlineVersion)}>
            <Form.Item name="metadata_url" label="版本元数据地址"
              extra="兼容 GitHub Releases 的 JSON 地址，提供稳定版标签和资产 SHA-256。请选择可信来源。"
              rules={[{ required: true, message: '请输入版本元数据地址' }]}>
              <Input placeholder="https://api.github.com/repos/pymumu/smartdns/releases/latest" />
            </Form.Item>
            <Form.Item name="asset_base_url" label="升级包下载目录"
              extra="控制台会在目录后追加 /发行标签/资产名，并将校验通过的升级包通过 SSH 传给节点。"
              rules={[{ required: true, message: '请输入升级包下载目录' }]}>
              <Input placeholder="https://github.com/pymumu/smartdns/releases/download" />
            </Form.Item>
            <Button type="primary" htmlType="submit" loading={savingCoreSource}>验证并保存</Button>
          </Form>
        </Card>
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
        defaultActiveKey={searchParams.get('tab') === 'system' ? 'system' : 'profile'}
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