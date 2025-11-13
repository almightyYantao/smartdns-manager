import React, { useState, useEffect } from 'react';
import {
  Modal,
  Steps,
  Button,
  Alert,
  Spin,
  Timeline,
  Tag,
  message,
  Space,
  Descriptions,
  Card,
} from 'antd';
import {
  CheckCircleOutlined,
  CloseCircleOutlined,
  LoadingOutlined,
  SyncOutlined,
  InfoCircleOutlined,
} from '@ant-design/icons';
import {
  initNode,
  checkNodeInit,
  getInitLogs,
  uninstallSmartDNS,
  reinstallSmartDNS,
} from '../../api';
import moment from 'moment';

const { Step } = Steps;

const NodeInitializer = ({ visible, onClose, node }) => {
  const [loading, setLoading] = useState(false);
  const [initStatus, setInitStatus] = useState(null);
  const [logs, setLogs] = useState([]);
  const [currentStep, setCurrentStep] = useState(0);
  const [polling, setPolling] = useState(false);

  const steps = [
    { key: 'detect', title: '检测系统', description: '检测操作系统和架构' },
    { key: 'download', title: '下载程序', description: '下载 SmartDNS 安装包' },
    { key: 'install', title: '安装程序', description: '执行安装脚本' },
    { key: 'configure', title: '初始化配置', description: '创建默认配置文件' },
    { key: 'start', title: '启动服务', description: '启动 SmartDNS 服务' },
  ];

  useEffect(() => {
    if (visible && node) {
      loadInitStatus();
      loadInitLogs();
    }
  }, [visible, node]);

  useEffect(() => {
    let interval;
    if (polling) {
      interval = setInterval(() => {
        loadInitStatus();
        loadInitLogs();
      }, 3000); // 每3秒轮询一次
    }
    return () => {
      if (interval) clearInterval(interval);
    };
  }, [polling]);

  const loadInitStatus = async () => {
    if (!node) return;

    try {
      const response = await checkNodeInit(node.id);
      setInitStatus(response.data);

      // 根据状态更新当前步骤
      if (response.data.init_status === 'initializing') {
        setPolling(true);
        updateCurrentStep();
      } else {
        setPolling(false);
      }
    } catch (error) {
      console.error('加载初始化状态失败', error);
    }
  };

  const loadInitLogs = async () => {
    if (!node) return;

    try {
      const response = await getInitLogs(node.id);
      setLogs(response.data || []);
    } catch (error) {
      console.error('加载初始化日志失败', error);
    }
  };

  const updateCurrentStep = () => {
    if (logs.length === 0) return;

    const latestLog = logs[0];
    const stepIndex = steps.findIndex(s => s.key === latestLog.step);
    if (stepIndex !== -1) {
      setCurrentStep(stepIndex);
    }
  };

  const handleInit = async () => {
    try {
      setLoading(true);
      await initNode(node.id);
      message.success('初始化已开始');
      setPolling(true);
      loadInitLogs();
    } catch (error) {
      message.error('启动初始化失败');
    } finally {
      setLoading(false);
    }
  };

  const handleUninstall = async () => {
    Modal.confirm({
      title: '确认卸载',
      content: `确定要卸载节点 "${node.name}" 上的 SmartDNS 吗？这将删除所有配置文件。`,
      okText: '确定卸载',
      okType: 'danger',
      cancelText: '取消',
      onOk: async () => {
        try {
          await uninstallSmartDNS(node.id);
          message.success('卸载任务已开始');
          setPolling(true);
          setTimeout(() => {
            loadInitStatus();
            loadInitLogs();
          }, 2000);
        } catch (error) {
          message.error('卸载失败');
        }
      },
    });
  };

  const handleReinstall = async () => {
    Modal.confirm({
      title: '确认重新安装',
      content: `确定要重新安装节点 "${node.name}" 上的 SmartDNS 吗？`,
      okText: '确定',
      cancelText: '取消',
      onOk: async () => {
        try {
          await reinstallSmartDNS(node.id);
          message.success('重新安装任务已开始');
          setPolling(true);
          setTimeout(() => {
            loadInitStatus();
            loadInitLogs();
          }, 2000);
        } catch (error) {
          message.error('重新安装失败');
        }
      },
    });
  };

  const getStepStatus = (stepKey) => {
    const log = logs.find(l => l.step === stepKey);
    if (!log) return 'wait';
    if (log.status === 'success') return 'finish';
    if (log.status === 'failed') return 'error';
    if (log.status === 'running') return 'process';
    return 'wait';
  };

  const getStatusIcon = (status) => {
    switch (status) {
      case 'success':
        return <CheckCircleOutlined style={{ color: '#52c41a' }} />;
      case 'failed':
        return <CloseCircleOutlined style={{ color: '#f5222d' }} />;
      case 'running':
        return <LoadingOutlined style={{ color: '#1890ff' }} />;
      default:
        return <InfoCircleOutlined style={{ color: '#d9d9d9' }} />;
    }
  };

  const getStatusColor = (status) => {
    const colors = {
      unknown: 'default',
      not_installed: 'warning',
      installed: 'success',
      initializing: 'processing',
      failed: 'error',
    };
    return colors[status] || 'default';
  };

  const getStatusText = (status) => {
    const texts = {
      unknown: '未知',
      not_installed: '未安装',
      installed: '已安装',
      initializing: '初始化中',
      failed: '失败',
    };
    return texts[status] || status;
  };

  const renderActions = () => {
    if (!initStatus) return null;

    const status = initStatus.init_status;

    return (
      <Space>
        {status === 'not_installed' && (
          <Button type="primary" onClick={handleInit} loading={loading}>
            开始初始化
          </Button>
        )}
        {status === 'installed' && (
          <>
            <Button onClick={handleReinstall}>
              重新安装
            </Button>
            <Button danger onClick={handleUninstall}>
              卸载
            </Button>
          </>
        )}
        {status === 'initializing' && (
          <Button disabled>
            <LoadingOutlined /> 初始化中...
          </Button>
        )}
        {status === 'failed' && (
          <Button type="primary" onClick={handleInit} loading={loading}>
            重试初始化
          </Button>
        )}
        <Button onClick={() => { loadInitStatus(); loadInitLogs(); }}>
          <SyncOutlined /> 刷新
        </Button>
      </Space>
    );
  };

  return (
    <Modal
      title={`节点初始化 - ${node?.name}`}
      open={visible}
      onCancel={onClose}
      width={900}
      footer={[
        <Button key="close" onClick={onClose}>
          关闭
        </Button>,
        renderActions(),
      ]}
    >
      {initStatus && (
        <Card size="small" style={{ marginBottom: 16 }}>
          <Descriptions column={3} size="small">
            <Descriptions.Item label="初始化状态">
              <Tag color={getStatusColor(initStatus.init_status)}>
                {getStatusText(initStatus.init_status)}
              </Tag>
            </Descriptions.Item>
            <Descriptions.Item label="SmartDNS 版本">
              {initStatus.smartdns_version || '-'}
            </Descriptions.Item>
            <Descriptions.Item label="操作系统">
              {initStatus.os_type ? (
                `${initStatus.os_type} ${initStatus.os_version}`
              ) : (
                '-'
              )}
            </Descriptions.Item>
            <Descriptions.Item label="架构">
              {initStatus.architecture || '-'}
            </Descriptions.Item>
          </Descriptions>
        </Card>
      )}

      {initStatus?.init_status === 'initializing' && (
        <Alert
          message="正在初始化"
          description="SmartDNS 正在安装中，请稍候..."
          type="info"
          showIcon
          icon={<LoadingOutlined />}
          style={{ marginBottom: 16 }}
        />
      )}

      {initStatus?.init_status === 'failed' && (
        <Alert
          message="初始化失败"
          description="SmartDNS 安装失败，请查看日志了解详情"
          type="error"
          showIcon
          style={{ marginBottom: 16 }}
        />
      )}

      {initStatus?.init_status === 'installed' && (
        <Alert
          message="已安装 SmartDNS"
          description={`版本: ${initStatus.smartdns_version}`}
          type="success"
          showIcon
          style={{ marginBottom: 16 }}
        />
      )}

      <Steps current={currentStep} style={{ marginBottom: 24 }}>
        {steps.map((step, index) => (
          <Step
            key={step.key}
            title={step.title}
            description={step.description}
            status={getStepStatus(step.key)}
          />
        ))}
      </Steps>

      <Card title="初始化日志" size="small">
        {logs.length > 0 ? (
          <Timeline mode="left">
            {logs.map((log) => (
              <Timeline.Item
                key={log.id}
                color={
                  log.status === 'success'
                    ? 'green'
                    : log.status === 'failed'
                    ? 'red'
                    : 'blue'
                }
                dot={getStatusIcon(log.status)}
              >
                <div>
                  <div style={{ marginBottom: 4 }}>
                    <Space>
                      <Tag color={getStatusColor(log.status)}>
                        {steps.find(s => s.key === log.step)?.title || log.step}
                      </Tag>
                      <span>{log.message}</span>
                    </Space>
                  </div>
                  {log.detail && (
                    <div style={{ color: '#666', fontSize: '12px', marginBottom: 4 }}>
                      {log.detail}
                    </div>
                  )}
                  {log.error && (
                    <Alert
                      message="错误信息"
                      description={log.error}
                      type="error"
                      showIcon
                      style={{ marginTop: 8 }}
                    />
                  )}
                  <div style={{ color: '#999', fontSize: '12px' }}>
                    {moment(log.created_at).format('YYYY-MM-DD HH:mm:ss')}
                  </div>
                </div>
              </Timeline.Item>
            ))}
          </Timeline>
        ) : (
          <div style={{ textAlign: 'center', padding: '40px 0', color: '#999' }}>
            暂无日志
          </div>
        )}
      </Card>
    </Modal>
  );
};

export default NodeInitializer;