
import React, { useState, useEffect } from 'react';
import {
  Card,
  Table,
  Button,
  Space,
  Modal,
  Form,
  Input,
  Switch,
  Select,
  InputNumber,
  message,
  Popconfirm,
  Tag,
  Tooltip,
  Badge,
  Progress,
  Alert,
  Spin,
  Divider,
  Row,
  Col,
} from 'antd';
import {
  PlusOutlined,
  EditOutlined,
  DeleteOutlined,
  PlayCircleOutlined,
  SettingOutlined,
  CloudUploadOutlined,
  HistoryOutlined,
  CheckCircleOutlined,
  ExclamationCircleOutlined,
} from '@ant-design/icons';
import {
  getBackupConfigs,
  createBackupConfig,
  updateBackupConfig,
  deleteBackupConfig,
  triggerManualBackup,
  getBackupStats,
  testS3Connection,
  getBackupHistory,
} from '../../api/modules/databaseBackup';
import dayjs from 'dayjs';

const { Option } = Select;
const { TextArea } = Input;

const DatabaseBackupManager = () => {
  const [configs, setConfigs] = useState([]);
  const [stats, setStats] = useState(null);
  const [loading, setLoading] = useState(false);
  const [modalVisible, setModalVisible] = useState(false);
  const [historyModalVisible, setHistoryModalVisible] = useState(false);
  const [editingConfig, setEditingConfig] = useState(null);
  const [s3Testing, setS3Testing] = useState(false);
  const [history, setHistory] = useState([]);
  const [historyLoading, setHistoryLoading] = useState(false);
  const [form] = Form.useForm();

  useEffect(() => {
    loadData();
  }, []);

  const loadData = async () => {
    await Promise.all([
      loadBackupConfigs(),
      loadBackupStats(),
    ]);
  };

  const loadBackupConfigs = async () => {
    try {
      setLoading(true);
      const response = await getBackupConfigs();
      setConfigs(response.data?.configs || []);
    } catch (error) {
      message.error('加载备份配置失败');
    } finally {
      setLoading(false);
    }
  };

  const loadBackupStats = async () => {
    try {
      const response = await getBackupStats();
      setStats(response.data);
    } catch (error) {
      console.error('加载备份统计失败', error);
    }
  };

  const loadBackupHistory = async (configId = '') => {
    try {
      setHistoryLoading(true);
      const response = await getBackupHistory({ 
        config_id: configId,
        page_size: 50 
      });
      setHistory(response.data?.history || []);
    } catch (error) {
      message.error('加载备份历史失败');
    } finally {
      setHistoryLoading(false);
    }
  };

  const handleAdd = () => {
    setEditingConfig(null);
    form.resetFields();
    setModalVisible(true);
  };

  const handleEdit = (record) => {
    setEditingConfig(record);
    form.setFieldsValue({
      ...record,
      notification_channels: record.notification_channels ? 
        JSON.parse(record.notification_channels) : []
    });
    setModalVisible(true);
  };

  const handleDelete = async (id) => {
    try {
      await deleteBackupConfig(id);
      message.success('删除成功');
      loadBackupConfigs();
    } catch (error) {
      message.error('删除失败');
    }
  };

  const handleSubmit = async () => {
    try {
      const values = await form.validateFields();
      
      // 处理通知渠道
      const data = {
        ...values,
        notification_channels: values.notification_channels || [],
      };

      if (editingConfig) {
        await updateBackupConfig(editingConfig.id, data);
        message.success('更新成功');
      } else {
        await createBackupConfig(data);
        message.success('创建成功');
      }
      
      setModalVisible(false);
      loadData();
    } catch (error) {
      if (error.errorFields) {
        return;
      }
      message.error('操作失败');
    }
  };

  const handleManualBackup = async (id) => {
    try {
      await triggerManualBackup(id);
      message.success('备份任务已启动');
      // 延迟刷新数据
      setTimeout(loadData, 1000);
    } catch (error) {
      message.error('启动备份失败');
    }
  };

  const handleTestS3 = async () => {
    try {
      const values = await form.validateFields([
        's3_enabled', 's3_access_key', 's3_secret_key', 
        's3_region', 's3_bucket', 's3_endpoint'
      ]);
      
      if (!values.s3_enabled) {
        message.warning('请先启用S3存储');
        return;
      }

      setS3Testing(true);
      await testS3Connection(values);
      message.success('S3连接测试成功');
    } catch (error) {
      message.error('S3连接测试失败: ' + (error.response?.data?.error || error.message));
    } finally {
      setS3Testing(false);
    }
  };

  const showHistory = () => {
    setHistoryModalVisible(true);
    loadBackupHistory();
  };

  const columns = [
    {
      title: '配置名称',
      dataIndex: 'name',
      key: 'name',
      render: (text) => <Tag color="blue">{text}</Tag>,
    },
    {
      title: '备份类型',
      dataIndex: 'backup_type',
      key: 'backup_type',
      render: (type) => (
        <Tag color={type === 'database' ? 'green' : 'orange'}>
          {type === 'database' ? '数据库' : '文件'}
        </Tag>
      ),
    },
    {
      title: '调度',
      dataIndex: 'schedule',
      key: 'schedule',
      render: (schedule) => <code>{schedule}</code>,
    },
    {
      title: '存储',
      key: 'storage',
      render: (_, record) => (
        <Space>
          {record.s3_enabled && <Tag color="blue">S3</Tag>}
          {record.local_path && <Tag color="green">本地</Tag>}
        </Space>
      ),
    },
    {
      title: '状态',
      dataIndex: 'enabled',
      key: 'enabled',
      render: (enabled) => (
        <Badge 
          status={enabled ? 'success' : 'default'} 
          text={enabled ? '启用' : '禁用'} 
        />
      ),
    },
    {
      title: '最后备份',
      key: 'last_backup',
      render: (_, record) => {
        if (!record.last_backup_at) {
          return <Tag>未执行</Tag>;
        }
        
        const statusColor = {
          'success': 'success',
          'failed': 'error',
          'running': 'processing'
        };

        return (
          <Space direction="vertical" size="small">
            <div>{dayjs(record.last_backup_at).format('MM-DD HH:mm')}</div>
            <Tag color={statusColor[record.last_backup_status] || 'default'}>
              {record.last_backup_status || '未知'}
            </Tag>
          </Space>
        );
      },
    },
    {
      title: '下次备份',
      dataIndex: 'next_backup_at',
      key: 'next_backup_at',
      render: (time) => time ? dayjs(time).format('MM-DD HH:mm') : '-',
    },
    {
      title: '操作',
      key: 'action',
      fixed: 'right',
      width: 200,
      render: (_, record) => (
        <Space size="small">
          <Tooltip title="手动备份">
            <Button
              type="link"
              size="small"
              icon={<PlayCircleOutlined />}
              onClick={() => handleManualBackup(record.id)}
            />
          </Tooltip>
          <Tooltip title="编辑">
            <Button
              type="link"
              size="small"
              icon={<EditOutlined />}
              onClick={() => handleEdit(record)}
            />
          </Tooltip>
          <Popconfirm
            title="确定要删除这个备份配置吗？"
            onConfirm={() => handleDelete(record.id)}
            okText="确定"
            cancelText="取消"
          >
            <Tooltip title="删除">
              <Button
                type="link"
                size="small"
                danger
                icon={<DeleteOutlined />}
              />
            </Tooltip>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  const historyColumns = [
    {
      title: '配置名称',
      dataIndex: ['config', 'name'],
      key: 'config_name',
      render: (text) => <Tag color="blue">{text}</Tag>,
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (status) => {
        const statusConfig = {
          'success': { color: 'success', icon: <CheckCircleOutlined /> },
          'failed': { color: 'error', icon: <ExclamationCircleOutlined /> },
          'running': { color: 'processing', icon: <Spin size="small" /> }
        };
        const config = statusConfig[status] || { color: 'default', icon: null };
        
        return (
          <Tag color={config.color} icon={config.icon}>
            {status}
          </Tag>
        );
      },
    },
    {
      title: '文件大小',
      dataIndex: 'file_size',
      key: 'file_size',
      render: (size) => size ? `${(size / 1024 / 1024).toFixed(2)} MB` : '-',
    },
    {
      title: '执行时长',
      dataIndex: 'duration',
      key: 'duration',
      render: (duration) => duration ? `${duration}s` : '-',
    },
    {
      title: '开始时间',
      dataIndex: 'started_at',
      key: 'started_at',
      render: (time) => dayjs(time).format('YYYY-MM-DD HH:mm:ss'),
    },
    {
      title: '错误信息',
      dataIndex: 'error_message',
      key: 'error_message',
      render: (error) => error ? (
        <Tooltip title={error}>
          <Tag color="red">查看错误</Tag>
        </Tooltip>
      ) : '-',
    },
  ];

  return (
    <div>
      {/* 统计信息 */}
      {stats && (
        <Row gutter={16} style={{ marginBottom: 16 }}>
          <Col span={6}>
            <Card size="small">
              <div style={{ textAlign: 'center' }}>
                <div style={{ fontSize: '24px', fontWeight: 'bold', color: '#1890ff' }}>
                  {stats.total_configs}
                </div>
                <div>总配置数</div>
              </div>
            </Card>
          </Col>
          <Col span={6}>
            <Card size="small">
              <div style={{ textAlign: 'center' }}>
                <div style={{ fontSize: '24px', fontWeight: 'bold', color: '#52c41a' }}>
                  {stats.active_configs}
                </div>
                <div>活动配置</div>
              </div>
            </Card>
          </Col>
          <Col span={6}>
            <Card size="small">
              <div style={{ textAlign: 'center' }}>
                <div style={{ fontSize: '24px', fontWeight: 'bold', color: '#faad14' }}>
                  {stats.success_rate?.toFixed(1) || 0}%
                </div>
                <div>成功率</div>
              </div>
            </Card>
          </Col>
          <Col span={6}>
            <Card size="small">
              <div style={{ textAlign: 'center' }}>
                <div style={{ fontSize: '24px', fontWeight: 'bold', color: '#722ed1' }}>
                  {((stats.total_size || 0) / 1024 / 1024).toFixed(1)} MB
                </div>
                <div>总备份大小</div>
              </div>
            </Card>
          </Col>
        </Row>
      )}

      {/* 操作按钮 */}
      <Card>
        <Space style={{ marginBottom: 16 }}>
          <Button type="primary" icon={<PlusOutlined />} onClick={handleAdd}>
            添加备份配置
          </Button>
          <Button icon={<HistoryOutlined />} onClick={showHistory}>
            查看备份历史
          </Button>
        </Space>

        {/* 配置列表 */}
        <Table
          columns={columns}
          dataSource={configs}
          rowKey="id"
          loading={loading}
          scroll={{ x: 1200 }}
          pagination={{
            pageSize: 10,
            showSizeChanger: true,
            showTotal: (total) => `共 ${total} 条记录`,
          }}
        />
      </Card>

      {/* 配置模态框 */}
      <Modal
        title={editingConfig ? '编辑备份配置' : '添加备份配置'}
        open={modalVisible}
        onOk={handleSubmit}
        onCancel={() => setModalVisible(false)}
        width={800}
        okText="保存"
        cancelText="取消"
      >
        <Form form={form} layout="vertical">
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item
                name="name"
                label="配置名称"
                rules={[{ required: true, message: '请输入配置名称' }]}
              >
                <Input placeholder="例如：每日备份" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item
                name="enabled"
                label="是否启用"
                valuePropName="checked"
              >
                <Switch />
              </Form.Item>
            </Col>
          </Row>

          <Row gutter={16}>
            <Col span={12}>
              <Form.Item
                name="backup_type"
                label="备份类型"
                rules={[{ required: true, message: '请选择备份类型' }]}
              >
                <Select placeholder="选择备份类型">
                  <Option value="database">数据库备份</Option>
                  <Option value="files">文件备份</Option>
                </Select>
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item
                name="schedule"
                label="备份计划 (Cron表达式)"
                rules={[{ required: true, message: '请输入Cron表达式' }]}
              >
                <Input placeholder="例如：0 2 * * * (每天凌晨2点)" />
              </Form.Item>
            </Col>
          </Row>

          <Row gutter={16}>
            <Col span={12}>
              <Form.Item
                name="retention_days"
                label="保留天数"
                rules={[{ required: true, message: '请输入保留天数' }]}
              >
                <InputNumber min={1} max={365} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
          </Row>

          <Divider>S3存储配置</Divider>
          
          <Form.Item
            name="s3_enabled"
            label="启用S3存储"
            valuePropName="checked"
          >
            <Switch />
          </Form.Item>

          <Form.Item dependencies={['s3_enabled']} noStyle>
            {({ getFieldValue }) =>
              getFieldValue('s3_enabled') ? (
                <>
                  <Row gutter={16}>
                    <Col span={12}>
                      <Form.Item
                        name="s3_access_key"
                        label="Access Key"
                        rules={[{ required: true, message: '请输入Access Key' }]}
                      >
                        <Input placeholder="S3 Access Key" />
                      </Form.Item>
                    </Col>
                    <Col span={12}>
                      <Form.Item
                        name="s3_secret_key"
                        label="Secret Key"
                        rules={[{ required: true, message: '请输入Secret Key' }]}
                      >
                        <Input.Password placeholder="S3 Secret Key" />
                      </Form.Item>
                    </Col>
                  </Row>

                  <Row gutter={16}>
                    <Col span={8}>
                      <Form.Item
                        name="s3_region"
                        label="区域"
                        rules={[{ required: true, message: '请输入区域' }]}
                      >
                        <Input placeholder="例如：us-east-1" />
                      </Form.Item>
                    </Col>
                    <Col span={8}>
                      <Form.Item
                        name="s3_bucket"
                        label="存储桶"
                        rules={[{ required: true, message: '请输入存储桶名称' }]}
                      >
                        <Input placeholder="存储桶名称" />
                      </Form.Item>
                    </Col>
                    <Col span={8}>
                      <Form.Item
                        name="s3_prefix"
                        label="存储前缀"
                      >
                        <Input placeholder="backup-files" />
                      </Form.Item>
                    </Col>
                  </Row>

                  <Form.Item
                    name="s3_endpoint"
                    label="自定义端点 (可选)"
                    extra="适用于MinIO等S3兼容服务"
                  >
                    <Input placeholder="https://minio.example.com" />
                  </Form.Item>

                  <Form.Item>
                    <Button
                      onClick={handleTestS3}
                      loading={s3Testing}
                      icon={<CloudUploadOutlined />}
                    >
                      测试S3连接
                    </Button>
                  </Form.Item>
                </>
              ) : null
            }
          </Form.Item>

          <Divider>本地存储配置</Divider>
          
          <Form.Item
            name="local_path"
            label="本地备份路径"
            extra="留空则不保存到本地"
          >
            <Input placeholder="/backup/database" />
          </Form.Item>

          <Divider>压缩和加密</Divider>
          
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item
                name="compression_enabled"
                label="启用压缩"
                valuePropName="checked"
              >
                <Switch />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item
                name="compression_level"
                label="压缩级别 (0-9)"
              >
                <InputNumber min={0} max={9} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
          </Row>

          <Row gutter={16}>
            <Col span={12}>
              <Form.Item
                name="encryption_enabled"
                label="启用加密"
                valuePropName="checked"
              >
                <Switch />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item dependencies={['encryption_enabled']} noStyle>
                {({ getFieldValue }) =>
                  getFieldValue('encryption_enabled') ? (
                    <Form.Item
                      name="encryption_key"
                      label="加密密钥"
                      rules={[{ required: true, message: '请输入加密密钥' }]}
                    >
                      <Input.Password placeholder="加密密钥" />
                    </Form.Item>
                  ) : null
                }
              </Form.Item>
            </Col>
          </Row>

          <Divider>通知设置</Divider>
          
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item
                name="notify_on_success"
                label="成功时通知"
                valuePropName="checked"
              >
                <Switch />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item
                name="notify_on_failure"
                label="失败时通知"
                valuePropName="checked"
                initialValue={true}
              >
                <Switch />
              </Form.Item>
            </Col>
          </Row>
        </Form>
      </Modal>

      {/* 备份历史模态框 */}
      <Modal
        title="备份历史"
        open={historyModalVisible}
        onCancel={() => setHistoryModalVisible(false)}
        width={1000}
        footer={[
          <Button key="close" onClick={() => setHistoryModalVisible(false)}>
            关闭
          </Button>
        ]}
      >
        <Table
          columns={historyColumns}
          dataSource={history}
          rowKey="id"
          loading={historyLoading}
          scroll={{ x: 800 }}
          pagination={{
            pageSize: 20,
            showSizeChanger: true,
            showTotal: (total) => `共 ${total} 条记录`,
          }}
        />
      </Modal>
    </div>
  );
};

export default DatabaseBackupManager;