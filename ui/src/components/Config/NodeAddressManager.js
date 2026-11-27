import React, { useEffect, useState } from 'react';
import { Alert, Button, Popconfirm, Space, Table, Tag, Tooltip, message } from 'antd';
import { DeleteOutlined, PlusOutlined, SyncOutlined } from '@ant-design/icons';
import { Link } from 'react-router-dom';
import { adoptNodeAddressMapping, deleteNodeAddressMapping, getNodeAddressMappings } from '../../api';

const statusLabels = {
  unassigned: {
    color: 'red',
    label: '未分配给此节点',
    description: '节点主配置和全局都有相同映射，但全局记录未分配给当前节点，或该记录已停用。',
  },
  node_only: {
    color: 'orange',
    label: '仅节点配置',
    description: '当前节点主配置有这条映射。全局没有域名、类型和目标均相同的记录。',
  },
  pending: { color: 'gold', label: '待下发' },
  matched: { color: 'green', label: '一致' },
};

const canRemove = (row) => row.line && (row.status === 'unassigned' || row.status === 'node_only');

export default function NodeAddressManager({ nodeId, active }) {
  const [snapshot, setSnapshot] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);
  const [refresh, setRefresh] = useState(0);
  const [operation, setOperation] = useState(null);
  const reload = () => setRefresh((value) => value + 1);

  useEffect(() => {
    if (!active) return;
    let cancelled = false;
    setLoading(true);
    setError(false);
    setSnapshot(null);
    getNodeAddressMappings(nodeId).then(({ data }) => {
      if (!cancelled) setSnapshot(data);
    }).catch(() => {
      if (!cancelled) setError(true);
    }).finally(() => {
      if (!cancelled) setLoading(false);
    });
    return () => { cancelled = true; };
  }, [nodeId, active, refresh]);

  const operate = async (row, action) => {
    if (!snapshot || operation) return;
    setOperation({ line: row.line, action });
    const input = { line: row.line, checksum: snapshot.checksum };
    try {
      if (action === 'adopt') {
        const { data } = await adoptNodeAddressMapping(nodeId, input);
        message.success({
          created: '已新建全局映射，仅分配给当前节点。',
          assigned: '已将当前节点加入现有全局映射。',
          already_assigned: '当前节点已分配此映射。',
        }[data.action]);
      } else {
        const unassign = row.status === 'matched';
        await deleteNodeAddressMapping(nodeId, unassign ? { ...input, unassign: true } : input);
        message.success(unassign
          ? '已移除当前节点的全局分配和主配置映射。重启 SmartDNS 后生效。'
          : '已从当前节点主配置删除。重启 SmartDNS 后生效。');
      }
    } catch {
      // 请求拦截器展示服务端错误。重新读取以核对节点的实际状态。
    } finally {
      setSnapshot(null);
      setOperation(null);
      reload();
    }
  };

  const columns = [
    { title: '域名', dataIndex: 'domain', key: 'domain', render: (domain) => <code>{domain}</code> },
    { title: '备注 / 用途', dataIndex: 'comment', key: 'comment', width: 180, ellipsis: { showTitle: false },
      render: (comment) => comment ? <Tooltip title={comment}><span>{comment}</span></Tooltip> : '—' },
    { title: '类型', dataIndex: 'type', key: 'type', width: 110,
      render: (type) => <Tag>{type === 'cname' ? 'CNAME' : 'Address'}</Tag> },
    { title: '目标', dataIndex: 'target', key: 'target', render: (target) => <code>{target}</code> },
    { title: '配置状态', dataIndex: 'status', key: 'status', width: 175,
      filters: Object.entries(statusLabels).map(([value, { label }]) => ({ text: label, value })),
      onFilter: (value, row) => row.status === value,
      render: (status) => (
        <Tooltip title={statusLabels[status]?.description}>
          <Tag color={statusLabels[status]?.color}>{statusLabels[status]?.label}</Tag>
        </Tooltip>
      ) },
    { title: '主配置行号', dataIndex: 'line', key: 'line', width: 110, render: (line) => line ?? '—' },
    { title: '操作', key: 'action', width: 300, render: (_, row) => row.line && (
      <Space wrap size={8}>
        {canRemove(row) && <Tooltip title="有相同的全局映射就加入当前节点，否则新建一条仅分配给当前节点的记录。" trigger={['hover', 'focus']}>
          <Popconfirm title="将这条配置纳入全局地址映射？"
            description="匹配到现有映射时补上当前节点，否则新建一条仅分配给当前节点的映射。"
            okText="纳入" cancelText="取消" onConfirm={() => operate(row, 'adopt')}>
            <Button size="small" icon={<PlusOutlined />} disabled={Boolean(operation)}
              loading={operation?.line === row.line && operation.action === 'adopt'}>纳入全局映射</Button>
          </Popconfirm>
        </Tooltip>}
        <Tooltip title={row.status === 'matched'
          ? '同步撤销当前节点在全局映射中的分配，并移除当前节点的主配置。'
          : '只从当前节点的主配置移除。重启该节点的 SmartDNS 后生效。'} trigger={['hover', 'focus']}>
          <Popconfirm title={row.status === 'matched' ? '移除当前节点的配置和全局分配？' : '仅从当前节点删除这条映射？'}
            description={row.status === 'matched'
              ? '将备份并更新本节点主配置。「全部节点」会改为其余现有节点；若没有剩余节点，会删除全局记录。需手动重启当前节点的 SmartDNS。'
              : '将备份并更新此节点的主配置。需要手动重启该节点的 SmartDNS 才会生效。'}
            okText="删除" cancelText="取消" okButtonProps={{ danger: true }}
            onConfirm={() => operate(row, 'remove')}>
            <Button size="small" danger icon={<DeleteOutlined />} disabled={Boolean(operation)}
              loading={operation?.line === row.line && operation.action === 'remove'}>
              {row.status === 'matched' ? '移除当前节点' : '仅从此节点删除'}
            </Button>
          </Popconfirm>
        </Tooltip>
      </Space>
    ) },
  ];
  const rows = [...(snapshot?.mappings || [])].sort((left, right) => (left.status === 'matched') - (right.status === 'matched'));
  const extraCount = rows.filter(canRemove).length;
  return (
    <Space direction="vertical" size="middle" style={{ width: '100%' }}>
      <Alert showIcon type={extraCount ? 'warning' : 'info'}
        message={extraCount ? `当前节点主配置有 ${extraCount} 条未分配映射` : '当前节点的地址映射'}
        description="对照节点主配置和控制台分配。移除「一致」的映射时，会同时撤销当前节点的全局分配。更新主配置后需手动重启 SmartDNS。" />
      <Space wrap>
        <Button icon={<SyncOutlined />} loading={loading} disabled={Boolean(operation)} onClick={reload}>重新读取节点配置</Button>
        <Link to="/addresses">全局地址映射</Link>
      </Space>
      {error ? <Alert showIcon type="error" message="无法核对当前节点地址映射，请重新读取。" /> : <>
        {snapshot?.partial && <Alert showIcon type="info" message="仅比较主配置中直接声明的普通 Address 和 CNAME。引用文件、规则组和特殊写法需在配置编辑器核对。" />}
        <Table columns={columns} dataSource={rows} loading={loading} scroll={{ x: 1130 }}
          rowKey={(row) => row.line ? `line-${row.line}` : `pending-${row.type}-${row.domain}-${row.target}`} />
      </>}
    </Space>
  );
}
