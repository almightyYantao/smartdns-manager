import React, { useEffect, useState } from "react";
import { Alert, Button, Card, Modal, Space, Table, Tag, Typography } from "antd";
import { Link } from "react-router-dom";
import { getNodes, getNodeCoreVersions, upgradeNodeCore } from "../api";

export default function CoreUpgrades() {
  const [nodes, setNodes] = useState([]);
  const [loadingNodes, setLoadingNodes] = useState(true);
  const [nodeLoadFailed, setNodeLoadFailed] = useState(false);
  const [coreVersions, setCoreVersions] = useState({});
  const [checkingCores, setCheckingCores] = useState(false);
  const [coreNode, setCoreNode] = useState(null);
  const [upgradingCore, setUpgradingCore] = useState(false);
  const [coreResult, setCoreResult] = useState(null);

  const checkCoreVersions = async () => {
    setCheckingCores(true);
    setCoreVersions({});
    try {
      const response = await getNodeCoreVersions();
      setCoreVersions(response.data || {});
    } finally {
      setCheckingCores(false);
    }
  };

  const openCoreUpgrade = (node) => {
    setCoreResult(null);
    setCoreNode({ ...node, core: coreVersions[node.id] });
  };

  const submitCoreUpgrade = async () => {
    if (upgradingCore || !coreNode?.core.can_upgrade) return;
    const selected = coreNode.core;
    setUpgradingCore(true);
    try {
      const response = await upgradeNodeCore(coreNode.id, {
        current: selected.tag, target: selected.latest,
      });
      setCoreResult({ type: "success", text: response.data });
    } catch (error) {
      setCoreResult({ type: "error", text: error.response?.data?.message || error.message });
    } finally {
      setUpgradingCore(false);
      checkCoreVersions().catch(() => {});
    }
  };


  const loadNodes = async () => {
    setLoadingNodes(true);
    setNodeLoadFailed(false);
    try {
      const { data } = await getNodes();
      setNodes(data || []);
    } catch {
      setNodeLoadFailed(true);
    } finally {
      setLoadingNodes(false);
    }
  };

  useEffect(() => { loadNodes(); }, []);

  const targetVersion = Object.values(coreVersions).find((core) => core.latest)?.latest;
  const columns = [
    {
      title: "节点",
      key: "node",
      render: (_, node) => <Space direction="vertical" size={0}>
        <strong>{node.name}</strong>
        <Typography.Text type="secondary" style={{ overflowWrap: "anywhere" }}>{node.host}</Typography.Text>
      </Space>,
    },
    {
      title: "当前版本",
      key: "version",
      render: (_, node) => {
        const core = coreVersions[node.id];
        return <span style={{ overflowWrap: "anywhere" }}>{core?.tag || core?.version || "—"}</span>;
      },
    },
    {
      title: "升级状态",
      key: "upgrade",
      render: (_, node) => {
        const core = coreVersions[node.id];
        return <Space wrap size={4}>
          <Tag color={core?.can_upgrade ? "blue" : "default"} style={{ whiteSpace: "normal" }}>
            {core?.state || (checkingCores ? "检测中" : "待检测")}
          </Tag>
          {core?.can_upgrade && <Button size="small" disabled={upgradingCore} onClick={() => openCoreUpgrade(node)}>升级</Button>}
        </Space>;
      },
    },
  ];

  return (
    <Card title="升级管理">
      <Space wrap style={{ marginBottom: 16 }}>
        <Button type="primary" loading={checkingCores} disabled={loadingNodes || nodeLoadFailed || upgradingCore}
          onClick={() => checkCoreVersions().catch(() => {})}>检测版本</Button>
        {targetVersion && <span>目标版本 {targetVersion}</span>}
        <Link to="/settings?tab=system">发布源设置</Link>
      </Space>
      {nodeLoadFailed && <Alert showIcon type="error" message="读取节点列表失败" action={<Button onClick={loadNodes}>重试</Button>} />}
      <Table rowKey="id" size="small" tableLayout="fixed" loading={loadingNodes}
        columns={columns} dataSource={nodes} pagination={false} />
      <Modal
        title="确认单节点升级"
        open={Boolean(coreNode)}
        onCancel={() => setCoreNode(null)}
        onOk={() => coreResult ? setCoreNode(null) : submitCoreUpgrade()}
        okText={coreResult ? "关闭" : "确认升级并重启"}
        cancelText="取消"
        confirmLoading={upgradingCore}
        closable={!upgradingCore}
        maskClosable={false}
        keyboard={!upgradingCore}
        cancelButtonProps={{ disabled: upgradingCore, style: coreResult ? { display: "none" } : undefined }}
      >
        {coreNode && <p>{coreNode.name} · {coreNode.host}</p>}
        {coreResult ? <Alert type={coreResult.type} message={coreResult.text} showIcon /> : <>
          <Alert type="warning" showIcon message={`${coreNode?.core.tag || ""} → ${coreNode?.core.latest || ""}。升级会重启当前节点，DNS 查询可能短暂中断。`} />
          <p style={{ marginTop: 12 }}>系统会在升级前后自动检查 DNS 解析。验证失败时会尝试恢复旧版本。</p>
        </>}
      </Modal>

    </Card>
  );
}
