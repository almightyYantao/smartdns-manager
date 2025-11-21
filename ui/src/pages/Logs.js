import React, { useState, useEffect } from 'react';
import {
  Card,
  Select,
  Space,
  Tabs,
  Button,
  Modal,
  message,
  InputNumber,
  Alert,
  Badge,
} from 'antd';
import {
  DeleteOutlined,
  ReloadOutlined,
  FileSearchOutlined,
  BarChartOutlined,
  PlayCircleOutlined,
} from '@ant-design/icons';
import { getNodes, cleanNodeLogs } from '../api';
import LogMonitorControl from '../components/Log/LogMonitorControl';
import LogList from '../components/Log/LogList';
import LogStats from '../components/Log/LogStats';

const { Option } = Select;

const Logs = () => {
  const [nodes, setNodes] = useState([]);
  const [selectedNode, setSelectedNode] = useState(null);
  const [loading, setLoading] = useState(false);
  const [activeTab, setActiveTab] = useState('logs');

  useEffect(() => {
    loadNodes();
  }, []);

  const loadNodes = async () => {
    try {
      setLoading(true);
      const response = await getNodes();
      const nodeList = response.data || [];
      setNodes(nodeList);
      
      // 默认选择第一个节点
      if (nodeList.length > 0 && !selectedNode) {
        setSelectedNode(nodeList[0].id);
      }
    } catch (error) {
      message.error('加载节点列表失败');
    } finally {
      setLoading(false);
    }
  };

  const handleCleanLogs = () => {
    let days = 30;

    Modal.confirm({
      title: '清理旧日志',
      content: (
        <div>
          <p>将删除指定天数之前的日志数据，此操作不可恢复！</p>
          <Space>
            <span>保留最近</span>
            <InputNumber
              min={1}
              max={365}
              defaultValue={30}
              onChange={(value) => (days = value)}
              style={{ width: 100 }}
            />
            <span>天的日志</span>
          </Space>
        </div>
      ),
      okText: '确定清理',
      okType: 'danger',
      cancelText: '取消',
      onOk: async () => {
        try {
          await cleanNodeLogs(selectedNode, days);
          message.success('日志清理成功');
        } catch (error) {
          message.error('日志清理失败');
        }
      },
    });
  };

  const selectedNodeInfo = nodes.find((n) => n.id === selectedNode);

  const logsTab = (
    <Space direction="vertical" style={{ width: '100%' }} size="large">
      {selectedNode ? (
        <LogList nodeId={selectedNode} nodeName={selectedNodeInfo?.name} />
      ) : (
        <Card>
          <div style={{ textAlign: 'center', padding: '40px 0', color: '#999' }}>
            请先选择节点
          </div>
        </Card>
      )}
    </Space>
  );

  const statsTab = (
    <Space direction="vertical" style={{ width: '100%' }} size="large">
      {selectedNode ? (
        <LogStats nodeId={selectedNode} />
      ) : (
        <Card>
          <div style={{ textAlign: 'center', padding: '40px 0', color: '#999' }}>
            请先选择节点
          </div>
        </Card>
      )}
    </Space>
  );

  return (
    <div>
      <Card
        title={
          <Space>
            <FileSearchOutlined />
            <span>DNS 日志管理</span>
            {selectedNodeInfo && (
              <span style={{ color: '#999', fontSize: '14px' }}>
                - {selectedNodeInfo.name}
              </span>
            )}
          </Space>
        }
        extra={
          <Space>
            <Select
              style={{ width: 200 }}
              placeholder="选择节点"
              value={selectedNode}
              onChange={setSelectedNode}
              loading={loading}
              suffixIcon={loading ? <ReloadOutlined spin /> : undefined}
            >
              {nodes.map((node) => (
                <Option key={node.id} value={node.id}>
                  <Space>
                    <Badge
                      status={node.status === 'online' ? 'success' : 'error'}
                    />
                    {node.name}
                  </Space>
                </Option>
              ))}
            </Select>
            <Button
              icon={<ReloadOutlined />}
              onClick={loadNodes}
              loading={loading}
            >
              刷新
            </Button>
            <Button
              danger
              icon={<DeleteOutlined />}
              onClick={handleCleanLogs}
              disabled={!selectedNode}
            >
              清理旧日志
            </Button>
          </Space>
        }
      >
        {selectedNode && (
          <div style={{ marginBottom: 16 }}>
            <LogMonitorControl
              nodeId={selectedNode}
              nodeName={selectedNodeInfo?.name}
            />
          </div>
        )}

        {selectedNode ? (
          <Tabs
            activeKey={activeTab}
            onChange={setActiveTab}
            items={[
              {
                key: 'logs',
                label: (
                  <span>
                    <FileSearchOutlined />
                    日志查询
                  </span>
                ),
                children: logsTab,
              },
              {
                key: 'stats',
                label: (
                  <span>
                    <BarChartOutlined />
                    统计分析
                  </span>
                ),
                children: statsTab,
              },
            ]}
          />
        ) : (
          <Card>
            <div style={{ textAlign: 'center', padding: '60px 0' }}>
              {nodes.length === 0 ? (
                <Space direction="vertical" size="large">
                  <FileSearchOutlined style={{ fontSize: 48, color: '#d9d9d9' }} />
                  <div>
                    <p style={{ color: '#999', marginBottom: 16 }}>
                      暂无节点，请先添加节点
                    </p>
                    <Button type="primary" onClick={() => window.location.href = '/nodes'}>
                      前往添加节点
                    </Button>
                  </div>
                </Space>
              ) : (
                <Space direction="vertical" size="large">
                  <PlayCircleOutlined style={{ fontSize: 48, color: '#d9d9d9' }} />
                  <div>
                    <p style={{ color: '#999' }}>请从上方下拉框选择要查看的节点</p>
                  </div>
                </Space>
              )}
            </div>
          </Card>
        )}
      </Card>
    </div>
  );
};

export default Logs;