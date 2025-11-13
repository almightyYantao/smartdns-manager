import React, { useState, useEffect } from 'react';
import {
  Row,
  Col,
  Card,
  Statistic,
  Table,
  Tag,
  Progress,
  Space,
  Spin,
  Empty,
  Timeline,
} from 'antd';
import {
  CloudServerOutlined,
  GlobalOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  WarningOutlined,
  SettingOutlined,
} from '@ant-design/icons';
import {
  AreaChart,
  Area,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
} from 'recharts';
import { getDashboardStats, getNodesHealth } from '../../api';
import moment from 'moment';

const Dashboard = () => {
  const [stats, setStats] = useState(null);
  const [health, setHealth] = useState(null);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    loadData();
    const interval = setInterval(loadData, 30000); // 每30秒刷新
    return () => clearInterval(interval);
  }, []);

  const loadData = async () => {
    try {
      setLoading(true);
      const [statsRes, healthRes] = await Promise.all([
        getDashboardStats(),
        getNodesHealth(),
      ]);
      setStats(statsRes.data);
      setHealth(healthRes.data);
    } catch (error) {
      console.error('加载数据失败', error);
    } finally {
      setLoading(false);
    }
  };

  if (loading && !stats) {
    return (
      <div style={{ textAlign: 'center', padding: '100px 0' }}>
        <Spin size="large" />
      </div>
    );
  }

  if (!stats) {
    return <Empty description="暂无数据" />;
  }

  const nodeHealthColumns = [
    {
      title: '节点名称',
      dataIndex: 'node_name',
      key: 'node_name',
      render: (text, record) => (
        <Space>
          {record.status === 'online' ? (
            <CheckCircleOutlined style={{ color: '#52c41a' }} />
          ) : (
            <CloseCircleOutlined style={{ color: '#f5222d' }} />
          )}
          <span>{text}</span>
        </Space>
      ),
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (status) => {
        const colors = {
          online: 'success',
          offline: 'error',
          error: 'warning',
        };
        return <Tag color={colors[status]}>{status}</Tag>;
      },
    },
    {
      title: 'CPU',
      dataIndex: 'health_data',
      key: 'cpu',
      render: (data) =>
        data ? (
          <Progress
            percent={parseFloat(data.cpu_usage?.toFixed(1) || 0)}
            size="small"
            status="active"
          />
        ) : (
          '-'
        ),
    },
    {
      title: '内存',
      dataIndex: 'health_data',
      key: 'memory',
      render: (data) =>
        data ? (
          <Progress
            percent={parseFloat(data.memory_usage?.toFixed(1) || 0)}
            size="small"
            status="active"
          />
        ) : (
          '-'
        ),
    },
    {
      title: '服务状态',
      dataIndex: 'health_data',
      key: 'service',
      render: (data) =>
        data ? (
          data.service_up ? (
            <Tag color="success">运行中</Tag>
          ) : (
            <Tag color="error">已停止</Tag>
          )
        ) : (
          '-'
        ),
    },
  ];

  return (
    <div>
      <Row gutter={[16, 16]}>
        <Col xs={24} sm={12} lg={6}>
          <Card>
            <Statistic
              title="节点总数"
              value={stats.total_nodes}
              prefix={<CloudServerOutlined />}
              valueStyle={{ color: '#1890ff' }}
            />
          </Card>
        </Col>
        <Col xs={24} sm={12} lg={6}>
          <Card>
            <Statistic
              title="在线节点"
              value={stats.online_nodes}
              prefix={<CheckCircleOutlined />}
              valueStyle={{ color: '#52c41a' }}
            />
          </Card>
        </Col>
        <Col xs={24} sm={12} lg={6}>
          <Card>
            <Statistic
              title="DNS服务器"
              value={stats.total_servers}
              prefix={<SettingOutlined />}
              valueStyle={{ color: '#722ed1' }}
            />
          </Card>
        </Col>
        <Col xs={24} sm={12} lg={6}>
          <Card>
            <Statistic
              title="地址映射"
              value={stats.total_addresses}
              prefix={<GlobalOutlined />}
              valueStyle={{ color: '#faad14' }}
            />
          </Card>
        </Col>
      </Row>

      <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
        <Col xs={24} lg={16}>
          <Card title="节点健康状态" extra={<Tag color="green">实时监控</Tag>}>
            <Table
              columns={nodeHealthColumns}
              dataSource={health}
              rowKey="node_id"
              size="small"
              pagination={false}
            />
          </Card>
        </Col>

        <Col xs={24} lg={8}>
          <Card title="服务器类型分布">
            {stats.servers_by_type && stats.servers_by_type.length > 0 ? (
              <Space direction="vertical" style={{ width: '100%' }}>
                {stats.servers_by_type.map((item) => (
                  <div key={item.Type}>
                    <div style={{ marginBottom: 8 }}>
                      <Space>
                        <Tag color="blue">{item.Type.toUpperCase()}</Tag>
                        <span>{item.Count} 个</span>
                      </Space>
                    </div>
                    <Progress
                      percent={
                        (item.Count / stats.total_servers) * 100
                      }
                      showInfo={false}
                    />
                  </div>
                ))}
              </Space>
            ) : (
              <Empty description="暂无服务器" />
            )}
          </Card>
        </Col>
      </Row>

      <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
        <Col xs={24} lg={12}>
          <Card title="最近添加的地址映射">
            {stats.recent_addresses && stats.recent_addresses.length > 0 ? (
              <Timeline>
                {stats.recent_addresses.map((addr) => (
                  <Timeline.Item key={addr.id}>
                    <Space>
                      <Tag color="blue">{addr.domain}</Tag>
                      <span>→</span>
                      <code>{addr.ip}</code>
                    </Space>
                    <div style={{ color: '#999', fontSize: '12px' }}>
                      {moment(addr.created_at).fromNow()}
                    </div>
                  </Timeline.Item>
                ))}
              </Timeline>
            ) : (
              <Empty description="暂无记录" />
            )}
          </Card>
        </Col>

        <Col xs={24} lg={12}>
          <Card title="最近添加的节点">
            {stats.recent_nodes && stats.recent_nodes.length > 0 ? (
              <Timeline>
                {stats.recent_nodes.map((node) => (
                  <Timeline.Item key={node.id}>
                    <Space>
                      <Tag color={node.status === 'online' ? 'success' : 'default'}>
                        {node.name}
                      </Tag>
                      <code>{node.host}</code>
                    </Space>
                    <div style={{ color: '#999', fontSize: '12px' }}>
                      {moment(node.created_at).fromNow()}
                    </div>
                  </Timeline.Item>
                ))}
              </Timeline>
            ) : (
              <Empty description="暂无记录" />
            )}
          </Card>
        </Col>
      </Row>
    </div>
  );
};

export default Dashboard;