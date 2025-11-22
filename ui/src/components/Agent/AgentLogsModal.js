import React, { useState, useEffect } from 'react';
import {
  Modal,
  Button,
  Spin,
  Typography,
  Alert,
  Select,
  Space,
  message,
  Empty,
} from 'antd';
import {
  ReloadOutlined,
  DownloadOutlined,
  CopyOutlined,
} from '@ant-design/icons';
import { getAgentLogs } from '../../api';

const { Text } = Typography;
const { Option } = Select;

const AgentLogsModal = ({ visible, onCancel, node }) => {
  const [logs, setLogs] = useState([]);
  const [loading, setLoading] = useState(false);
  const [lines, setLines] = useState('100');
  const [autoRefresh, setAutoRefresh] = useState(false);
  const [error, setError] = useState(null);

  useEffect(() => {
    if (visible && node?.id) {
      fetchLogs();
    }
  }, [visible, node?.id, lines]);

  useEffect(() => {
    let interval;
    if (autoRefresh && visible) {
      interval = setInterval(() => {
        fetchLogs(true); // 静默刷新
      }, 3000);
    }
    return () => {
      if (interval) {
        clearInterval(interval);
      }
    };
  }, [autoRefresh, visible]);

  const fetchLogs = async (silent = false) => {
    if (!node?.id) return;

    try {
      if (!silent) setLoading(true);
      setError(null);

      const result = await getAgentLogs(node?.id);

      if (result.success) {
        setLogs(result.data.logs || []);
      } else {
        setError(result.message);
        setLogs([]);
      }
    } catch (err) {
      setError('获取日志失败: ' + err.message);
      setLogs([]);
      if (!silent) {
        message.error('获取日志失败');
      }
    } finally {
      if (!silent) setLoading(false);
    }
  };

  const handleCopyLogs = () => {
    const logText = logs.join('\n');
    navigator.clipboard.writeText(logText).then(() => {
      message.success('日志已复制到剪贴板');
    }).catch(() => {
      message.error('复制失败');
    });
  };

  const handleDownloadLogs = () => {
    const logText = logs.join('\n');
    const blob = new Blob([logText], { type: 'text/plain' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `smartdns-agent-${node.name}-${new Date().toISOString().slice(0, 19).replace(/:/g, '-')}.log`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
    message.success('日志下载成功');
  };

  const formatLogLine = (line, index) => {
    if (!line.trim()) return null;

    // 检测日志级别并设置颜色
    let color = '#333';
    let backgroundColor = 'transparent';

    if (line.includes('[ERROR]') || line.includes('❌')) {
      color = '#ff4d4f';
    } else if (line.includes('[WARN]') || line.includes('⚠️')) {
      color = '#faad14';
    } else if (line.includes('[INFO]') || line.includes('✅')) {
      color = '#52c41a';
    } else if (line.includes('[DEBUG]') || line.includes('🔍')) {
      color = '#1890ff';
    }

    // 高亮重要信息
    if (line.includes('成功') || line.includes('启动') || line.includes('连接成功')) {
      backgroundColor = '#f6ffed';
    } else if (line.includes('失败') || line.includes('错误') || line.includes('异常')) {
      backgroundColor = '#fff2f0';
    }

    return (
      <div
        key={index}
        style={{
          fontSize: '12px',
          fontFamily: 'Consolas, "Courier New", monospace',
          lineHeight: '1.4',
          padding: '2px 4px',
          marginBottom: '1px',
          color,
          backgroundColor,
          wordBreak: 'break-all',
          whiteSpace: 'pre-wrap',
        }}
      >
        {line}
      </div>
    );
  };

  return (
    <Modal
      title={
        <Space>
          <span>Agent 日志 - {node?.name}</span>
          {autoRefresh && (
            <div style={{ display: 'inline-flex', alignItems: 'center' }}>
              <div 
                style={{
                  width: 8,
                  height: 8,
                  borderRadius: '50%',
                  backgroundColor: '#52c41a',
                  animation: 'pulse 2s infinite',
                  marginRight: 4,
                }}
              />
              <Text type="secondary" style={{ fontSize: 12 }}>自动刷新</Text>
            </div>
          )}
        </Space>
      }
      open={visible}
      onCancel={() => {
        setAutoRefresh(false);
        onCancel();
      }}
      width={1000}
      style={{ top: 20 }}
      bodyStyle={{ padding: '16px' }}
      footer={
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <Space>
            <Text type="secondary">显示行数:</Text>
            <Select
              value={lines}
              onChange={setLines}
              style={{ width: 80 }}
            >
              <Option value="50">50</Option>
              <Option value="100">100</Option>
              <Option value="200">200</Option>
              <Option value="500">500</Option>
            </Select>
            
            <Button
              size="small"
              type={autoRefresh ? 'primary' : 'default'}
              onClick={() => setAutoRefresh(!autoRefresh)}
            >
              {autoRefresh ? '停止自动刷新' : '自动刷新'}
            </Button>
          </Space>

          <Space>
            <Button
              size="small"
              icon={<CopyOutlined />}
              onClick={handleCopyLogs}
              disabled={logs.length === 0}
            >
              复制
            </Button>
            <Button
              size="small"
              icon={<DownloadOutlined />}
              onClick={handleDownloadLogs}
              disabled={logs.length === 0}
            >
              下载
            </Button>
            <Button
              size="small"
              icon={<ReloadOutlined />}
              onClick={() => fetchLogs()}
              loading={loading}
            >
              刷新
            </Button>
            <Button onClick={() => {
              setAutoRefresh(false);
              onCancel();
            }}>
              关闭
            </Button>
          </Space>
        </div>
      }
    >
      <div style={{ minHeight: 400 }}>
        {error && (
          <Alert
            message="获取日志失败"
            description={error}
            type="error"
            showIcon
            style={{ marginBottom: 16 }}
            action={
              <Button size="small" onClick={() => fetchLogs()}>
                重试
              </Button>
            }
          />
        )}

        {loading && (
          <div style={{ textAlign: 'center', padding: '60px 0' }}>
            <Spin size="large" tip="加载日志中..." />
          </div>
        )}

        {!loading && !error && logs.length === 0 && (
          <Empty
            description="暂无日志数据"
            image={Empty.PRESENTED_IMAGE_SIMPLE}
            style={{ padding: '60px 0' }}
          />
        )}

        {!loading && !error && logs.length > 0 && (
          <div
            style={{
              background: '#fafafa',
              border: '1px solid #d9d9d9',
              borderRadius: '6px',
              padding: '12px',
              maxHeight: '500px',
              overflow: 'auto',
            }}
          >
            {logs.map((line, index) => formatLogLine(line, index)).filter(Boolean)}
          </div>
        )}
      </div>

      <style jsx>{`
        @keyframes pulse {
          0%, 100% {
            opacity: 1;
          }
          50% {
            opacity: 0.3;
          }
        }
      `}</style>
    </Modal>
  );
};

export default AgentLogsModal;