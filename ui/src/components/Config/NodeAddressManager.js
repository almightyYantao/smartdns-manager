import React, { useEffect, useState } from 'react';
import { Alert, Button, Popconfirm, Space, Table, Tag, message } from 'antd';
import { SyncOutlined } from '@ant-design/icons';
import { Link } from 'react-router-dom';
import { deleteNodeAddressMapping, getNodeAddressMappings } from '../../api';

const statusLabels = {
  unassigned: { color: 'red', label: '未分配给此节点' },
  node_only: { color: 'orange', label: '仅节点配置' },
  pending: { color: 'gold', label: '待下发' },
  matched: { color: 'green', label: '一致' },
};

const canRemove = (row) => row.line && (row.status === 'unassigned' || row.status === 'node_only');

export default function NodeAddressManager({ nodeId, active }) {
  const [snapshot, setSnapshot] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);
  const [refresh, setRefresh] = useState(0);
  const [deleting, setDeleting] = useState(null);
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

  const remove = async (row) => {
    if (!snapshot || deleting !== null) return;
    setDeleting(row.line);
    try {
      await deleteNodeAddressMapping(nodeId, { line: row.line, checksum: snapshot.checksum });
      message.success('已从当前节点主配置删除。重启 SmartDNS 后生效。');
    } catch {
      // 请求拦截器展示服务端错误。重新读取以核对节点的实际状态。
    } finally {
      setSnapshot(null);
      setDeleting(null);
      reload();
    }
  };

  const columns = [
    { title: '域名', dataIndex: 'domain', key: 'domain', render: (domain) => <code>{domain}</code> },
    { title: '类型', dataIndex: 'type', key: 'type', width: 110,
      render: (type) => <Tag>{type === 'cname' ? 'CNAME' : 'Address'}</Tag> },
    { title: '目标', dataIndex: 'target', key: 'target', render: (target) => <code>{target}</code> },
    { title: '配置状态', dataIndex: 'status', key: 'status', width: 175,
      filters: Object.entries(statusLabels).map(([value, { label }]) => ({ text: label, value })),
      onFilter: (value, row) => row.status === value,
      render: (status) => <Tag color={statusLabels[status]?.color}>{statusLabels[status]?.label}</Tag> },
    { title: '主配置行号', dataIndex: 'line', key: 'line', width: 110, render: (line) => line ?? '—' },
    { title: '操作', key: 'action', width: 170, render: (_, row) => canRemove(row) && (
      <Popconfirm title="仅从当前节点删除这条映射？"
        description="将备份并更新此节点的主配置。需要手动重启该节点的 SmartDNS 才会生效。"
        okText="删除" cancelText="取消" okButtonProps={{ danger: true }}
        onConfirm={() => remove(row)}>
        <Button type="link" danger disabled={deleting !== null} loading={deleting === row.line}>仅从此节点删除</Button>
      </Popconfirm>
    ) },
  ];
  const rows = [...(snapshot?.mappings || [])].sort((left, right) => (left.status === 'matched') - (right.status === 'matched'));
  const extraCount = rows.filter(canRemove).length;
  return (
    <Space direction="vertical" size="middle" style={{ width: '100%' }}>
      <Alert showIcon type={extraCount ? 'warning' : 'info'}
        message={extraCount ? `当前节点主配置有 ${extraCount} 条未分配映射` : '当前节点的地址映射'}
        description="对照节点主配置和控制台分配。这里的删除仅作用于当前节点，更新主配置后需手动重启 SmartDNS。" />
      <Space wrap>
        <Button icon={<SyncOutlined />} loading={loading} disabled={deleting !== null} onClick={reload}>重新读取节点配置</Button>
        <Link to="/addresses">全局地址映射</Link>
      </Space>
      {error ? <Alert showIcon type="error" message="无法核对当前节点地址映射，请重新读取。" /> : <>
        {snapshot?.partial && <Alert showIcon type="info" message="仅比较主配置中直接声明的普通 Address 和 CNAME。引用文件、规则组和特殊写法需在配置编辑器核对。" />}
        <Table columns={columns} dataSource={rows} loading={loading} scroll={{ x: 850 }}
          rowKey={(row) => row.line ? `line-${row.line}` : `pending-${row.type}-${row.domain}-${row.target}`} />
      </>}
    </Space>
  );
}
