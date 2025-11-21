import React, { useState, useEffect } from 'react';
import {
  Card,
  Row,
  Col,
  Statistic,
  Table,
  Progress,
  DatePicker,
  Space,
  Spin,
  Empty,
} from 'antd';
import {
  FileSearchOutlined,
  UserOutlined,
  GlobalOutlined,
  ClockCircleOutlined,
} from '@ant-design/icons';
import { getNodeLogStats } from '../../api';
import moment from 'moment';

const { RangePicker } = DatePicker;

const LogStats = ({ nodeId }) => {
  const [stats, setStats] = useState(null);
  const [loading, setLoading] = useState(false);
  const [timeRange, setTimeRange] = useState([
    moment().subtract(24, 'hours'),
    moment(),
  ]);

  useEffect(() => {
    if (nodeId) {
      loadStats();
    }
  }, [nodeId, timeRange]);

  const loadStats = async () => {
    if (!nodeId) return;

    try {
      setLoading(true);
      const response = await getNodeLogStats(nodeId, {
        start_time: timeRange[0].toISOString(),
        end_time: timeRange[1].toISOString(),
      });
      setStats(response.data);
    } catch (error) {
      console.error('加载统计失败:', error);
      setStats(null);
    } finally {
      setLoading(false);
    }
  };

  if (loading) {
    return (
      <Card>
        <div style={{ textAlign: 'center', padding: '60px 0' }}>
          <Spin size="large" tip="加载统计数据..." />
        </div>
      </Card>
    );
  }

  if (!stats) {
    return (
      <Card>
        <Empty description="暂无统计数据" />
      </Card>
    );
  }

  const topDomainsColumns = [
    {
      title: '排名',
      key: 'rank',
      width: 60,
      render: (_, __, index) => index + 1,
    },
    {
      title: '域名',
      dataIndex: 'domain',
      key: 'domain',
      ellipsis: true,
    },
    {
      title: '查询次数',
      dataIndex: 'count',
      key: 'count',
      width: 100,
      align: 'right',
      render: (count) => count?.toLocaleString() || 0,
    },
    {
      title: '占比',
      key: 'percentage',
      width: 150,
      render: (_, record) => {
        if (!stats.total_queries || stats.total_queries === 0) {
          return <span>0%</span>;
        }
        const percentage = (record.count / stats.total_queries) * 100;
        return (
          <Progress
            percent={Math.min(percentage, 100)}
            size="small"
            format={(percent) => `${percent.toFixed(1)}%`}
          />
        );
      },
    },
  ];

  const topClientsColumns = [
    {
      title: '排名',
      key: 'rank',
      width: 60,
      render: (_, __, index) => index + 1,
    },
    {
      title: '客户端IP',
      dataIndex: 'client_ip',
      key: 'client_ip',
    },
    {
      title: '查询次数',
      dataIndex: 'count',
      key: 'count',
      width: 100,
      align: 'right',
      render: (count) => count?.toLocaleString() || 0,
    },
    {
      title: '占比',
      key: 'percentage',
      width: 150,
      render: (_, record) => {
        if (!stats.total_queries || stats.total_queries === 0) {
          return <span>0%</span>;
        }
        const percentage = (record.count / stats.total_queries) * 100;
        return (
          <Progress
            percent={Math.min(percentage, 100)}
            size="small"
            format={(percent) => `${percent.toFixed(1)}%`}
          />
        );
      },
    },
  ];

  return (
    <div>
      <Card style={{ marginBottom: 16 }}>
        <Space style={{ marginBottom: 16 }}>
          <span>统计时间范围：</span>
          <RangePicker
            showTime
            // value={timeRange}
            onChange={(dates) => {
              if (dates && dates.length === 2) {
                setTimeRange(dates);
              }
            }}
            format="YYYY-MM-DD HH:mm"
            allowClear={false}
          />
        </Space>

        <Row gutter={16}>
          <Col xs={24} sm={12} md={6}>
            <Card>
              <Statistic
                title="总查询数"
                value={stats.total_queries || 0}
                prefix={<FileSearchOutlined />}
                valueStyle={{ color: '#3f8600' }}
              />
            </Card>
          </Col>
          <Col xs={24} sm={12} md={6}>
            <Card>
              <Statistic
                title="唯一客户端"
                value={stats.unique_clients || 0}
                prefix={<UserOutlined />}
                valueStyle={{ color: '#1890ff' }}
              />
            </Card>
          </Col>
          <Col xs={24} sm={12} md={6}>
            <Card>
              <Statistic
                title="唯一域名"
                value={stats.unique_domains || 0}
                prefix={<GlobalOutlined />}
                valueStyle={{ color: '#722ed1' }}
              />
            </Card>
          </Col>
          <Col xs={24} sm={12} md={6}>
            <Card>
              <Statistic
                title="平均查询时间"
                value={stats.avg_query_time || 0}
                suffix="ms"
                precision={2}
                prefix={<ClockCircleOutlined />}
                valueStyle={{ color: '#cf1322' }}
              />
            </Card>
          </Col>
        </Row>
      </Card>

      <Row gutter={16}>
        <Col xs={24} lg={12}>
          <Card title="热门域名 Top 10" style={{ marginBottom: 16 }}>
            <Table
              columns={topDomainsColumns}
              dataSource={stats.top_domains || []}
              rowKey="domain"
              pagination={false}
              size="small"
              locale={{
                emptyText: <Empty description="暂无数据" image={Empty.PRESENTED_IMAGE_SIMPLE} />,
              }}
            />
          </Card>
        </Col>
        <Col xs={24} lg={12}>
          <Card title="活跃客户端 Top 10" style={{ marginBottom: 16 }}>
            <Table
              columns={topClientsColumns}
              dataSource={stats.top_clients || []}
              rowKey="client_ip"
              pagination={false}
              size="small"
              locale={{
                emptyText: <Empty description="暂无数据" image={Empty.PRESENTED_IMAGE_SIMPLE} />,
              }}
            />
          </Card>
        </Col>
      </Row>
    </div>
  );
};

export default LogStats;