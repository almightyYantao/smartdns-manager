import React, { useState, useEffect } from 'react';
import {
  Card,
  Descriptions,
  Progress,
  Tag,
  Space,
  Spin,
  Button,
  Tabs,
  Empty,
} from 'antd';
import {
  CheckCircleOutlined,
  CloseCircleOutlined,
  SyncOutlined,
  CodeOutlined,
} from '@ant-design/icons';
import { getNodeStatus, getNodeLogs } from '../../api';
import dayjs from 'dayjs';

const NodeStatus = ({ node }) => {
  const [status, setStatus] = useState(null);
  const [logs, setLogs] = useState('');
  const [loading, setLoading] = useState(false);
  const [logsLoading, setLogsLoading] = useState(false);

  useEffect(() => {
    loadStatus();
    loadLogs();
  }, [node]);

  const loadStatus = async () => {
    try {
      setLoading(true);
      const response = await getNodeStatus(node.id);
      setStatus(response.data);
    } catch (error) {
      console.error('获取状态失败', error);
    } finally {
      setLoading(false);
    }
  };

  const loadLogs = async () => {
    try {
      setLogsLoading(true);
      const response = await getNodeLogs(node.id, { lines: 100 });
      setLogs(response.data);
    } catch (error) {
      console.error('获取日志失败', error);
    } finally {
      setLogsLoading(false);
    }
  };

  const getProgressColor = (value) => {
    if (value < 60) return '#52c41a';
    if (value < 80) return '#faad14';
    return '#f5222d';
  };

  const statusTab = (
    <Spin spinning={loading}>
      {status ? (
        <Space direction="vertical" style={{ width: '100%' }} size="large">
          <Card title="服务状态" size="small">
            <Descriptions column={2}>
              <Descriptions.Item label="连接状态">
                {status.is_online ? (
                  <Tag icon={<CheckCircleOutlined />} color="success">
                    在线
                  </Tag>
                ) : (
                  <Tag icon={<CloseCircleOutlined />} color="error">
                    离线
                  </Tag>
                )}
              </Descriptions.Item>
              <Descriptions.Item label="服务状态">
                {status.service_up ? (
                  <Tag icon={<CheckCircleOutlined />} color="success">
                    运行中
                  </Tag>
                ) : (
                  <Tag icon={<CloseCircleOutlined />} color="error">
                    已停止
                  </Tag>
                )}
              </Descriptions.Item>
              <Descriptions.Item label="版本">
                {status.version || '未知'}
              </Descriptions.Item>
              <Descriptions.Item label="检查时间">
                {dayjs(status.last_checked).format('YYYY-MM-DD HH:mm:ss')}
              </Descriptions.Item>
            </Descriptions>
          </Card>

          <Card title="系统资源" size="small">
            <Space direction="vertical" style={{ width: '100%' }} size="middle">
              <div>
                <div style={{ marginBottom: 8 }}>
                  <Space>
                    <span>CPU 使用率</span>
                    <span style={{ fontWeight: 'bold' }}>
                      {status.cpu_usage?.toFixed(1) || 0}%
                    </span>
                  </Space>
                </div>
                <Progress
                  percent={parseFloat((status.cpu_usage || 0).toFixed(1))}
                  strokeColor={getProgressColor(status.cpu_usage || 0)}
                  status="active"
                />
              </div>

              <div>
                <div style={{ marginBottom: 8 }}>
                  <Space>
                    <span>内存使用率</span>
                    <span style={{ fontWeight: 'bold' }}>
                      {status.memory_usage?.toFixed(1) || 0}%
                    </span>
                  </Space>
                </div>
                <Progress
                  percent={parseFloat((status.memory_usage || 0).toFixed(1))}
                  strokeColor={getProgressColor(status.memory_usage || 0)}
                  status="active"
                />
              </div>

              <div>
                <div style={{ marginBottom: 8 }}>
                  <Space>
                    <span>磁盘使用率</span>
                    <span style={{ fontWeight: 'bold' }}>
                      {status.disk_usage?.toFixed(1) || 0}%
                    </span>
                  </Space>
                </div>
                <Progress
                  percent={parseFloat((status.disk_usage || 0).toFixed(1))}
                  strokeColor={getProgressColor(status.disk_usage || 0)}
                  status="active"
                />
              </div>
            </Space>
          </Card>

          <Button
            icon={<SyncOutlined />}
            onClick={loadStatus}
            loading={loading}
            block
          >
            刷新状态
          </Button>
        </Space>
      ) : (
        <Empty description="暂无状态信息" />
      )}
    </Spin>
  );

  const logsTab = (
    <Spin spinning={logsLoading}>
      <Space direction="vertical" style={{ width: '100%' }}>
        <Button
          icon={<SyncOutlined />}
          onClick={loadLogs}
          loading={logsLoading}
        >
          刷新日志
        </Button>
        <pre
          style={{
            backgroundColor: '#1e1e1e',
            color: '#d4d4d4',
            padding: '16px',
            borderRadius: '4px',
            maxHeight: '500px',
            overflow: 'auto',
            fontSize: '12px',
            lineHeight: '1.5',
          }}
        >
          {logs || '暂无日志'}
        </pre>
      </Space>
    </Spin>
  );

  return (
    <Tabs
      defaultActiveKey="status"
      items={[
        {
          key: 'status',
          label: '状态信息',
          children: statusTab,
        },
        {
          key: 'logs',
          label: '服务日志',
          icon: <CodeOutlined />,
          children: logsTab,
        },
      ]}
    />
  );
};

export default NodeStatus;