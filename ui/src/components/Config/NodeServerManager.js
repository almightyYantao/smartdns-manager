import React, { useEffect, useState } from 'react';
import { Alert, Button, Space, Table, Tag, Typography } from 'antd';
import { SyncOutlined } from '@ant-design/icons';
import { Link } from 'react-router-dom';
import { getNodeConfig, getServers } from '../../api';
import { getPendingNodeServers, parseNodeServers } from './nodeServers';

const columns = [
  {
    title: '上游地址', dataIndex: 'address', key: 'address', width: 300,
    render: (address) => <code style={{ overflowWrap: 'anywhere' }}>{address}</code>,
  },
  {
    title: '协议', dataIndex: 'type', key: 'type', width: 100,
    render: (type) => <Tag>{({ https: 'DoH', tls: 'DoT' })[type] || type.toUpperCase()}</Tag>,
  },
  {
    title: '分组', dataIndex: 'groups', key: 'groups', width: 180,
    render: (groups) => groups?.length
      ? <Space size={[4, 4]} wrap>{groups.map((group) => <Tag key={group}>{group}</Tag>)}</Space>
      : '默认组',
  },
  {
    title: '参与默认组', dataIndex: 'exclude_default', key: 'default', width: 120,
    render: (excluded) => excluded ? '否' : '是',
  },
  {
    title: '附加参数', dataIndex: 'options', key: 'options', width: 360,
    render: (options = '') => {
      const extra = options.replace(/(?:^|\s)-group\s+\S+/g, '')
        .replace(/(?:^|\s)-exclude-default-group(?=\s|$)/g, '').trim();
      return extra ? <code style={{ overflowWrap: 'anywhere' }}>{extra}</code> : '—';
    },
  },
];

export default function NodeServerManager({ nodeId, active, onEditConfig }) {
  const [snapshot, setSnapshot] = useState({ applied: [], pending: [], includes: false });
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [refresh, setRefresh] = useState(0);

  useEffect(() => {
    if (!active) return;
    let cancelled = false;
    setLoading(true);
    setError('');
    setSnapshot({ applied: [], pending: [], includes: false });
    // 文件读取失败时不回退到全局列表，避免把数据库记录当成节点配置。
    Promise.all([getNodeConfig(nodeId), getServers()]).then(([config, servers]) => {
      if (typeof config.rawContent !== 'string') throw new Error('配置内容缺失');
      const applied = parseNodeServers(config.rawContent);
      if (!cancelled) setSnapshot({
        applied,
        pending: getPendingNodeServers(servers.data || [], applied, nodeId),
        includes: /^\s*conf-file\s+/m.test(config.rawContent),
      });
    }).catch(() => {
      if (!cancelled) setError('无法读取节点上游配置，请重试。');
    }).finally(() => {
      if (!cancelled) setLoading(false);
    });
    return () => { cancelled = true; };
  }, [nodeId, active, refresh]);

  return (
    <Space direction="vertical" size="middle" style={{ width: '100%' }}>
      <Alert showIcon type="info" message="当前节点的上游 DNS"
        description="列表读取自节点配置文件，不展示其他节点的上游。已写入配置不代表运行进程已加载；修改后需重启服务。" />
      <Space wrap>
        <Button icon={<SyncOutlined />} loading={loading} onClick={() => setRefresh((n) => n + 1)}>
          重新读取节点配置
        </Button>
        <Button onClick={onEditConfig}>编辑节点配置</Button>
        <Link to="/servers">全局 DNS 管理</Link>
        {!loading && !error && <Typography.Text type="secondary">
          已写入配置：{snapshot.applied.length} 个上游
        </Typography.Text>}
      </Space>
      {error ? <Alert showIcon type="error" message={error} /> : <>
        {snapshot.includes && <Alert showIcon type="warning" message="配置引用了其他文件，以下仅展示主配置文件中直接声明的上游。" />}
        <Table columns={columns} dataSource={snapshot.applied} rowKey="key" loading={loading}
          scroll={{ x: 1060 }} pagination={false}
          locale={{ emptyText: '节点主配置文件中没有直接声明的上游 DNS' }} />
        {snapshot.pending.length > 0 && <>
          <Alert showIcon type="warning" message={`待同步：${snapshot.pending.length} 条管理台记录与节点配置不一致`}
            description="以下记录已分配给此节点（含全局分配），但尚未以相同参数写入配置文件。重启服务不会自动同步这些记录。" />
          <Table columns={columns} dataSource={snapshot.pending} rowKey="id"
            scroll={{ x: 1060 }} pagination={false} />
        </>}
      </>}
    </Space>
  );
}
