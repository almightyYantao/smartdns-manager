import React, { useState, useEffect } from "react";
import {
  Table,
  Button,
  Space,
  Tag,
  Modal,
  message,
  Popconfirm,
  Tooltip,
  Badge,
  Dropdown,
  Card,
} from "antd";
import {
  PlusOutlined,
  EditOutlined,
  DeleteOutlined,
  SyncOutlined,
  ThunderboltOutlined,
  EyeOutlined,
  CodeOutlined,
  HistoryOutlined,
  SettingOutlined,
  MoreOutlined,
  PlayCircleOutlined,
  ReloadOutlined,
} from "@ant-design/icons";
import dayjs from "dayjs";
import {
  getNodes,
  deleteNode,
  testNodeConnection,
  getNodeStatus,
  triggerFullSync,
} from "../../api";
import NodeForm from "./NodeForm";
import NodeStatus from "./NodeStatus";
import SyncStatus from "../Config/SyncStatus";
import NodeInitializer from "./NodeInitializer";
import AgentStatus from "../Agent/AgentStatus";
import { useNavigate } from "react-router-dom";

const NodeList = () => {
  const [nodes, setNodes] = useState([]);
  const [loading, setLoading] = useState(false);
  const [modalVisible, setModalVisible] = useState(false);
  const [statusModalVisible, setStatusModalVisible] = useState(false);
  const [editingNode, setEditingNode] = useState(null);
  const [selectedNode, setSelectedNode] = useState(null);
  const [selectedRowKeys, setSelectedRowKeys] = useState([]);
  const [syncStatusVisible, setSyncStatusVisible] = useState(false);
  const [selectedSyncNode, setSelectedSyncNode] = useState(null);
  const [initModalVisible, setInitModalVisible] = useState(false);
  const [selectedInitNode, setSelectedInitNode] = useState(null);

  const handleInitNode = (node) => {
    setSelectedInitNode(node);
    setInitModalVisible(true);
  };
  const navigate = useNavigate();

  useEffect(() => {
    loadNodes();
  }, []);

  const handleSyncConfig = async (node) => {
    Modal.confirm({
      title: "同步配置到节点",
      content: `确定要将当前数据库配置同步到节点 "${node.name}" 吗？这将覆盖节点上的配置文件。`,
      okText: "确定",
      cancelText: "取消",
      onOk: async () => {
        try {
          message.loading({ content: "正在同步配置...", key: "sync" });
          await triggerFullSync(node.id);
          message.success({
            content: "同步任务已启动，请查看同步日志",
            key: "sync",
          });
        } catch (error) {
          message.error({ content: "同步失败", key: "sync" });
        }
      },
    });
  };

  const handleViewSyncLogs = (node) => {
    setSelectedSyncNode(node);
    setSyncStatusVisible(true);
  };

  const loadNodes = async () => {
    try {
      setLoading(true);
      const response = await getNodes();
      setNodes(response.data || []);
    } catch (error) {
      message.error("加载节点列表失败");
    } finally {
      setLoading(false);
    }
  };

  const handleAdd = () => {
    setEditingNode(null);
    setModalVisible(true);
  };

  const handleEdit = (record) => {
    setEditingNode(record);
    setModalVisible(true);
  };

  const handleDelete = async (id) => {
    try {
      await deleteNode(id);
      message.success("删除成功");
      loadNodes();
    } catch (error) {
      message.error("删除失败");
    }
  };

  const handleTest = async (record) => {
    try {
      message.loading({ content: "正在测试连接...", key: "test" });
      const response = await testNodeConnection(record.id);
      message.success({ content: "连接测试成功", key: "test" });
      loadNodes();
    } catch (error) {
      message.error({ content: "连接测试失败", key: "test" });
    }
  };

  const handleViewStatus = async (record) => {
    setSelectedNode(record);
    setStatusModalVisible(true);
  };

  const handleFormSuccess = () => {
    setModalVisible(false);
    loadNodes();
  };

  const getStatusColor = (status) => {
    const colors = {
      online: "success",
      offline: "error",
      unknown: "default",
      error: "warning",
    };
    return colors[status] || "default";
  };

  const getStatusText = (status) => {
    const texts = {
      online: "在线",
      offline: "离线",
      unknown: "未知",
      error: "错误",
    };
    return texts[status] || status;
  };

  const columns = [
    {
      title: "节点名称",
      dataIndex: "name",
      key: "name",
      fixed: "left",
      width: 150,
      render: (text, record) => (
        <Space>
          <Badge status={getStatusColor(record.status)} />
          <strong>{text}</strong>
        </Space>
      ),
    },
    {
      title: "主机地址",
      dataIndex: "host",
      key: "host",
      width: 150,
    },
    {
      title: "端口",
      dataIndex: "port",
      key: "port",
      width: 80,
    },
    {
      title: "用户名",
      dataIndex: "username",
      key: "username",
      width: 120,
    },
    {
      title: "状态",
      dataIndex: "status",
      key: "status",
      width: 100,
      render: (status) => (
        <Tag color={getStatusColor(status)}>{getStatusText(status)}</Tag>
      ),
    },
    {
      title: "配置路径",
      dataIndex: "config_path",
      key: "config_path",
      width: 200,
      ellipsis: true,
      render: (text) => (
        <Tooltip title={text}>
          <code>{text}</code>
        </Tooltip>
      ),
    },
    {
      title: "标签",
      dataIndex: "tags",
      key: "tags",
      width: 150,
      render: (tags) => {
        if (!tags) return "-";
        try {
          const tagArray = JSON.parse(tags);
          return (
            <>
              {tagArray.map((tag) => (
                <Tag key={tag} color="blue">
                  {tag}
                </Tag>
              ))}
            </>
          );
        } catch {
          return "-";
        }
      },
    },
    {
      title: "初始化状态",
      dataIndex: "init_status",
      key: "init_status",
      width: 120,
      render: (status) => {
        const colors = {
          unknown: "default",
          not_installed: "warning",
          installed: "success",
          initializing: "processing",
          failed: "error",
        };
        const texts = {
          unknown: "未知",
          not_installed: "未安装",
          installed: "已安装",
          initializing: "初始化中",
          failed: "失败",
        };
        return (
          <Tag color={colors[status] || "default"}>
            {texts[status] || status}
          </Tag>
        );
      },
    },
    {
      title: "最后检查",
      dataIndex: "last_check",
      key: "last_check",
      width: 180,
      render: (time) =>
        time ? dayjs(time).format("YYYY-MM-DD HH:mm:ss") : "-",
    },
    {
      title: "操作",
      key: "action",
      fixed: "right",
      width: 120,
      render: (_, record) => {
        const moreMenuItems = [
          {
            key: "status",
            icon: <EyeOutlined />,
            label: "查看状态",
            onClick: () => handleViewStatus(record),
          },
          {
            type: "divider",
          },
          {
            key: "init",
            icon: <SettingOutlined />,
            label: "初始化节点",
            onClick: () => handleInitNode(record),
          },
          {
            key: "sync",
            icon: <SyncOutlined />,
            label: "同步配置",
            onClick: () => handleSyncConfig(record),
          },
          {
            key: "sync-logs",
            icon: <HistoryOutlined />,
            label: "同步日志",
            onClick: () => handleViewSyncLogs(record),
          },
          {
            key: "test",
            icon: <ThunderboltOutlined />,
            label: "测试连接",
            onClick: () => handleTest(record),
          },
          {
            type: "divider",
          },
          {
            key: "delete",
            icon: <DeleteOutlined />,
            label: "删除节点",
            danger: true,
            onClick: () => {
              Modal.confirm({
                title: "删除节点",
                content: `确定要删除节点 "${record.name}" 吗？此操作不可恢复。`,
                okText: "确定删除",
                okType: "danger",
                cancelText: "取消",
                onOk: () => handleDelete(record.id),
              });
            },
          },
        ];
        return (
          <Space size="small">
            <Tooltip title="配置管理">
              <Button
                type="link"
                size="small"
                icon={<CodeOutlined />}
                onClick={() => navigate(`/nodes/${record.id}/config`)}
              />
            </Tooltip>
            <Tooltip title="编辑节点">
              <Button
                type="link"
                size="small"
                icon={<EditOutlined />}
                onClick={() => handleEdit(record)}
              />
            </Tooltip>
            <Dropdown
              menu={{ items: moreMenuItems }}
              trigger={["click"]}
              placement="bottomRight"
            >
              <Button type="link" size="small" icon={<MoreOutlined />} />
            </Dropdown>
          </Space>
        );
      },
    },
  ];

  const rowSelection = {
    selectedRowKeys,
    onChange: setSelectedRowKeys,
  };

  // 批量操作处理函数
  const handleBatchInit = () => {
    if (selectedRowKeys.length === 0) {
      message.warning("请先选择要初始化的节点");
      return;
    }
    Modal.confirm({
      title: "批量初始化节点",
      content: `确定要初始化选中的 ${selectedRowKeys.length} 个节点吗？`,
      okText: "确定",
      cancelText: "取消",
      onOk: () => {
        selectedRowKeys.forEach((nodeId) => {
          const node = nodes.find((n) => n.id === nodeId);
          if (node) {
            handleInitNode(node);
          }
        });
        setSelectedRowKeys([]);
      },
    });
  };

  const handleBatchSync = () => {
    if (selectedRowKeys.length === 0) {
      message.warning("请先选择要同步的节点");
      return;
    }
    Modal.confirm({
      title: "批量同步配置",
      content: `确定要向选中的 ${selectedRowKeys.length} 个节点同步配置吗？`,
      okText: "确定",
      cancelText: "取消",
      onOk: async () => {
        try {
          message.loading({ content: "正在批量同步...", key: "batch-sync" });
          for (const nodeId of selectedRowKeys) {
            await triggerFullSync(nodeId);
          }
          message.success({ content: "批量同步任务已启动", key: "batch-sync" });
          setSelectedRowKeys([]);
        } catch (error) {
          message.error({ content: "批量同步失败", key: "batch-sync" });
        }
      },
    });
  };

  const handleBatchTest = async () => {
    if (selectedRowKeys.length === 0) {
      message.warning("请先选择要测试的节点");
      return;
    }

    message.loading({ content: "正在批量测试连接...", key: "batch-test" });
    let successCount = 0;
    let failCount = 0;

    for (const nodeId of selectedRowKeys) {
      try {
        await testNodeConnection(nodeId);
        successCount++;
      } catch (error) {
        failCount++;
      }
    }

    message.success({
      content: `批量测试完成：${successCount} 个成功，${failCount} 个失败`,
      key: "batch-test",
    });
    setSelectedRowKeys([]);
    loadNodes();
  };

  return (
    <div>
      {/* 批量操作工具栏 */}
      {selectedRowKeys.length > 0 && (
        <Card style={{ marginBottom: 16, backgroundColor: "#f0f9ff" }}>
          <div
            style={{
              display: "flex",
              justifyContent: "space-between",
              alignItems: "center",
            }}
          >
            <Space>
              <span style={{ color: "#1890ff", fontWeight: "bold" }}>
                已选择 {selectedRowKeys.length} 个节点
              </span>
              <Button size="small" onClick={() => setSelectedRowKeys([])}>
                取消选择
              </Button>
            </Space>
            <Space>
              <Button
                size="small"
                icon={<PlayCircleOutlined />}
                onClick={handleBatchInit}
              >
                批量初始化
              </Button>
              <Button
                size="small"
                icon={<SyncOutlined />}
                onClick={handleBatchSync}
              >
                批量同步
              </Button>
              <Button
                size="small"
                icon={<ThunderboltOutlined />}
                onClick={handleBatchTest}
              >
                批量测试
              </Button>
            </Space>
          </div>
        </Card>
      )}

      {/* 主要操作工具栏 */}
      <div
        style={{
          marginBottom: 16,
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
        }}
      >
        <Space>
          <Button type="primary" icon={<PlusOutlined />} onClick={handleAdd}>
            添加节点
          </Button>
          <Button icon={<ReloadOutlined />} onClick={loadNodes}>
            刷新列表
          </Button>
        </Space>
        <Space>
          <span style={{ color: "#666" }}>
            共 {nodes.length} 个节点
            {selectedRowKeys.length > 0 &&
              ` / 已选 ${selectedRowKeys.length} 个`}
          </span>
        </Space>
      </div>

      <Table
        rowSelection={rowSelection}
        columns={columns}
        dataSource={nodes}
        rowKey="id"
        loading={loading}
        scroll={{ x: 1200 }}
        pagination={{
          pageSize: 10,
          showSizeChanger: true,
          showTotal: (total) => `共 ${total} 条记录`,
        }}
      />

      <Modal
        title={editingNode ? "编辑节点" : "添加节点"}
        open={modalVisible}
        onCancel={() => setModalVisible(false)}
        footer={null}
        width={700}
        destroyOnClose
        bodyStyle={{
          maxHeight: "calc(100vh - 200px)",
          overflowY: "auto",
          paddingRight: "8px",
        }}
      >
        <NodeForm
          node={editingNode}
          onSuccess={handleFormSuccess}
          onCancel={() => setModalVisible(false)}
        />
      </Modal>

      <Modal
        title="节点状态"
        open={statusModalVisible}
        onCancel={() => setStatusModalVisible(false)}
        footer={null}
        width={800}
        destroyOnClose
      >
        {selectedNode && <NodeStatus node={selectedNode} />}
      </Modal>
      {/* 添加同步状态 Modal */}
      <SyncStatus
        visible={syncStatusVisible}
        onClose={() => setSyncStatusVisible(false)}
        nodeId={selectedSyncNode?.id}
      />
      {/* 初始化 Modal */}
      <NodeInitializer
        visible={initModalVisible}
        onClose={() => setInitModalVisible(false)}
        node={selectedInitNode}
      />
    </div>
  );
};

export default NodeList;
