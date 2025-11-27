import React, { useState, useEffect } from 'react';
import {
  Modal,
  Table,
  Tag,
  Button,
  Space,
  Progress,
  Tooltip,
  message,
  Select,
  DatePicker,
} from 'antd';
import {
  SyncOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  ClockCircleOutlined,
  ReloadOutlined,
  DeleteOutlined,
} from '@ant-design/icons';
import dayjs from 'dayjs';
import { getSyncLogs, retrySyncLog, getSyncStats } from '../../api';

const { RangePicker } = DatePicker;
const { Option } = Select;

const SyncStatus = ({ visible, onClose, nodeId }) => {
  const [logs, setLogs] = useState([]);
  const [stats, setStats] = useState(null);
  const [loading, setLoading] = useState(false);
  const [filters, setFilters] = useState({
    status: '',
    type: '',
  });
  const [pagination, setPagination] = useState({
    current: 1,
    pageSize: 20,
    total: 0,
  });

  useEffect(() => {
    if (visible) {
      loadLogs();
      loadStats();
    }
  }, [visible, nodeId, filters, pagination.current]);

  const loadLogs = async () => {
    try {
      setLoading(true);
      const response = await getSyncLogs({
        node_id: nodeId,
        status: filters.status,
        type: filters.type,
        page: pagination.current,
        page_size: pagination.pageSize,
      });
      setLogs(response.data || []);
      setPagination({
        ...pagination,
        total: response.total,
      });
    } catch (error) {
      message.error('加载同步日志失败');
    } finally {
      setLoading(false);
    }
  };

  const loadStats = async () => {
    try {
      const response = await getSyncStats();
      setStats(response.stats);
    } catch (error) {
      console.error('加载统计失败', error);
    }
  };

  const handleRetry = async (logId) => {
    try {
      await retrySyncLog(logId);
      message.success('已开始重试');
      loadLogs();
    } catch (error) {
      message.error('重试失败');
    }
  };

  const getStatusIcon = (status) => {
    switch (status) {
      case 'success':
        return <CheckCircleOutlined style={{ color: '#52c41a' }} />;
      case 'failed':
        return <CloseCircleOutlined style={{ color: '#f5222d' }} />;
      case 'pending':
        return <ClockCircleOutlined style={{ color: '#faad14' }} />;
      default:
        return null;
    }
  };

  const getStatusColor = (status) => {
    const colors = {
      success: 'success',
      failed: 'error',
      pending: 'warning',
    };
    return colors[status] || 'default';
  };

  const getTypeText = (type) => {
    const texts = {
      address: '地址映射',
      server: 'DNS服务器',
      full_sync: '完整同步',
    };
    return texts[type] || type;
  };

  const columns = [
    {
      title: '时间',
      dataIndex: 'created_at',
      key: 'created_at',
      width: 180,
      render: (time) => dayjs(time).format('YYYY-MM-DD HH:mm:ss'),
    },
    {
      title: '类型',
      dataIndex: 'type',
      key: 'type',
      width: 100,
      render: (type) => <Tag color="blue">{getTypeText(type)}</Tag>,
    },
    {
      title: '操作',
      dataIndex: 'action',
      key: 'action',
      width: 80,
      render: (action) => {
        const actions = {
          add: '添加',
          update: '更新',
          delete: '删除',
        };
        return actions[action] || action;
      },
    },
    {
      title: '内容',
      dataIndex: 'content',
      key: 'content',
      ellipsis: true,
      render: (text) => (
        <Tooltip title={text}>
          <code style={{ fontSize: '12px' }}>{text}</code>
        </Tooltip>
      ),
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 100,
      render: (status) => (
        <Space>
          {getStatusIcon(status)}
          <Tag color={getStatusColor(status)}>
            {status === 'success' ? '成功' : status === 'failed' ? '失败' : '进行中'}
          </Tag>
        </Space>
      ),
    },
    {
      title: '错误信息',
      dataIndex: 'error',
      key: 'error',
      ellipsis: true,
      render: (error) =>
        error ? (
          <Tooltip title={error}>
            <span style={{ color: '#f5222d' }}>{error}</span>
          </Tooltip>
        ) : (
          '-'
        ),
    },
    {
      title: '操作',
      key: 'action_col',
      width: 100,
      fixed: 'right',
      render: (_, record) => (
        <Space size="small">
          {record.status === 'failed' && (
            <Button
              type="link"
              size="small"
              icon={<ReloadOutlined />}
              onClick={() => handleRetry(record.id)}
            >
              重试
            </Button>
          )}
        </Space>
      ),
    },
  ];

  return (
    <Modal
      title="配置同步状态"
      open={visible}
      onCancel={onClose}
      width={1200}
      footer={[
        <Button key="close" onClick={onClose}>
          关闭
        </Button>,
        <Button
          key="refresh"
          type="primary"
          icon={<SyncOutlined />}
          onClick={loadLogs}
        >
          刷新
        </Button>,
      ]}
    >
      {stats && (
        <div style={{ marginBottom: 16, padding: 16, background: '#f5f5f5', borderRadius: 4 }}>
          <Space size="large">
            <div>
              <div style={{ color: '#666' }}>总计</div>
              <div style={{ fontSize: 24, fontWeight: 'bold' }}>{stats.total}</div>
            </div>
            <div>
              <div style={{ color: '#52c41a' }}>成功</div>
              <div style={{ fontSize: 24, fontWeight: 'bold', color: '#52c41a' }}>
                {stats.success}
              </div>
            </div>
            <div>
              <div style={{ color: '#f5222d' }}>失败</div>
              <div style={{ fontSize: 24, fontWeight: 'bold', color: '#f5222d' }}>
                {stats.failed}
              </div>
            </div>
            <div>
              <div style={{ color: '#faad14' }}>进行中</div>
              <div style={{ fontSize: 24, fontWeight: 'bold', color: '#faad14' }}>
                {stats.pending}
              </div>
            </div>
            <div style={{ flex: 1 }}>
              <Progress
                percent={stats.total > 0 ? ((stats.success / stats.total) * 100).toFixed(1) : 0}
                status="active"
              />
            </div>
          </Space>
        </div>
      )}

      <Space style={{ marginBottom: 16 }}>
        <Select
          placeholder="筛选状态"
          style={{ width: 120 }}
          allowClear
          value={filters.status}
          onChange={(value) => setFilters({ ...filters, status: value || '' })}
        >
          <Option value="success">成功</Option>
          <Option value="failed">失败</Option>
          <Option value="pending">进行中</Option>
        </Select>

        <Select
          placeholder="筛选类型"
          style={{ width: 120 }}
          allowClear
          value={filters.type}
          onChange={(value) => setFilters({ ...filters, type: value || '' })}
        >
          <Option value="address">地址映射</Option>
          <Option value="server">DNS服务器</Option>
          <Option value="full_sync">完整同步</Option>
        </Select>
      </Space>

      <Table
        columns={columns}
        dataSource={logs}
        rowKey="id"
        loading={loading}
        scroll={{ x: 1200 }}
        pagination={{
          ...pagination,
          showSizeChanger: true,
          showTotal: (total) => `共 ${total} 条记录`,
          onChange: (page, pageSize) => {
            setPagination({ ...pagination, current: page, pageSize });
          },
        }}
      />
    </Modal>
  );
};

export default SyncStatus;