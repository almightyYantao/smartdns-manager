import React, { useState, useEffect } from 'react';
import { Card, Switch, Button, Space, Statistic, Row, Col, message, Spin, Badge } from 'antd';
import { PlayCircleOutlined, PauseCircleOutlined, SyncOutlined } from '@ant-design/icons';
import {
  startNodeLogMonitor,
  stopNodeLogMonitor,
  getNodeLogMonitorStatus,
} from '../../api';

const LogMonitorControl = ({ nodeId, nodeName }) => {
  const [monitoring, setMonitoring] = useState(false);
  const [loading, setLoading] = useState(false);
  const [checking, setChecking] = useState(true);

  useEffect(() => {
    checkMonitorStatus();
  }, [nodeId]);

  const checkMonitorStatus = async () => {
    try {
      setChecking(true);
      const response = await getNodeLogMonitorStatus(nodeId);
      setMonitoring(response.data.is_running);
    } catch (error) {
      console.error('获取监控状态失败:', error);
    } finally {
      setChecking(false);
    }
  };

  const handleToggleMonitor = async (checked) => {
    try {
      setLoading(true);
      if (checked) {
        await startNodeLogMonitor(nodeId);
        message.success(`已启动 ${nodeName} 的日志监控`);
        setMonitoring(true);
      } else {
        await stopNodeLogMonitor(nodeId);
        message.success(`已停止 ${nodeName} 的日志监控`);
        setMonitoring(false);
      }
    } catch (error) {
      message.error(checked ? '启动监控失败' : '停止监控失败');
      console.error(error);
    } finally {
      setLoading(false);
    }
  };

  if (checking) {
    return (
      <Card size="small">
        <Spin tip="检查监控状态..." />
      </Card>
    );
  }

  return (
    <Card 
      size="small"
      title={
        <Space>
          <span>日志监控控制</span>
          <Badge 
            status={monitoring ? "processing" : "default"} 
            text={monitoring ? "运行中" : "已停止"} 
          />
        </Space>
      }
      extra={
        <Button
          type="link"
          size="small"
          icon={<SyncOutlined />}
          onClick={checkMonitorStatus}
          loading={checking}
        >
          刷新状态
        </Button>
      }
    >
      <Space direction="vertical" style={{ width: '100%' }}>
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
          <span>监控状态：</span>
          <Switch
            checked={monitoring}
            onChange={handleToggleMonitor}
            loading={loading}
            checkedChildren={<PlayCircleOutlined />}
            unCheckedChildren={<PauseCircleOutlined />}
          />
        </div>
        <div style={{ fontSize: '12px', color: '#666' }}>
          {monitoring 
            ? `正在实时监控节点 ${nodeName} 的 DNS 查询日志` 
            : '启用后将实时采集 DNS 查询日志'}
        </div>
      </Space>
    </Card>
  );
};

export default LogMonitorControl;