import React, { useState, useEffect } from "react";
import {
  Table,
  Button,
  Space,
  Tag,
  Modal,
  Form,
  Input,
  message,
  Popconfirm,
  Upload,
  Select,
  Switch,
  Badge,
  Tooltip,
} from "antd";
import {
  PlusOutlined,
  EditOutlined,
  DeleteOutlined,
  UploadOutlined,
  DownloadOutlined,
  SyncOutlined,
  CloudServerOutlined,
  CheckCircleOutlined,
} from "@ant-design/icons";
import {
  getAddresses,
  addAddress,
  updateAddress,
  deleteAddress,
  batchAddAddresses,
  importAddresses,
  getNodes,
  triggerFullSync,
  batchFullSync,
} from "../../api";
import moment from "moment";
import SyncStatus from "./SyncStatus";

const { Option } = Select;

const AddressManager = () => {
  const [addresses, setAddresses] = useState([]);
  const [nodes, setNodes] = useState([]);
  const [loading, setLoading] = useState(false);
  const [modalVisible, setModalVisible] = useState(false);
  const [batchModalVisible, setBatchModalVisible] = useState(false);
  const [syncStatusVisible, setSyncStatusVisible] = useState(false);
  const [editingAddress, setEditingAddress] = useState(null);
  const [selectedNodeId, setSelectedNodeId] = useState(null);
  const [selectedRowKeys, setSelectedRowKeys] = useState([]);
  const [pagination, setPagination] = useState({
    current: 1,
    pageSize: 20,
    total: 0,
  });
  const [form] = Form.useForm();
  const [batchForm] = Form.useForm();

  useEffect(() => {
    loadAddresses();
    loadNodes();
  }, [pagination.current, pagination.pageSize]);

  const loadAddresses = async () => {
    try {
      setLoading(true);
      const response = await getAddresses({
        page: pagination.current,
        page_size: pagination.pageSize,
      });
      setAddresses(response.data || []);
      setPagination({
        ...pagination,
        total: response.total || 0,
      });
    } catch (error) {
      message.error("加载地址映射失败");
    } finally {
      setLoading(false);
    }
  };

  const loadNodes = async () => {
    try {
      const response = await getNodes();
      setNodes(response.data || []);
    } catch (error) {
      console.error("加载节点列表失败", error);
    }
  };

  const handleAdd = () => {
    setEditingAddress(null);
    form.resetFields();
    setModalVisible(true);
  };

  const handleEdit = (record) => {
    setEditingAddress(record);
    const nodeIds = record.node_ids ? JSON.parse(record.node_ids) : [];
    form.setFieldsValue({
      ...record,
      node_ids: nodeIds,
    });
    setModalVisible(true);
  };

  const handleDelete = async (id) => {
    try {
      await deleteAddress(id);
      message.success("删除成功，正在从节点移除...");
      loadAddresses();
    } catch (error) {
      message.error("删除失败");
    }
  };

  const handleSubmit = async () => {
    try {
      const values = await form.validateFields();

      // 转换 node_ids 为 JSON 字符串
      if (values.node_ids && values.node_ids.length > 0) {
        values.node_ids = JSON.stringify(values.node_ids);
      } else {
        values.node_ids = "[]";
      }

      if (editingAddress) {
        await updateAddress(editingAddress.id, values);
        message.success("更新成功，正在同步到节点...");
      } else {
        await addAddress(values);
        message.success("添加成功，正在同步到节点...");
      }

      setModalVisible(false);
      loadAddresses();
    } catch (error) {
      message.error("操作失败");
    }
  };

  const handleBatchAdd = () => {
    batchForm.resetFields();
    setBatchModalVisible(true);
  };

  const handleBatchSubmit = async () => {
    try {
      const values = await batchForm.validateFields();
      const lines = values.content.split("\n").filter((line) => line.trim());

      const addresses = lines
        .map((line) => {
          const [domain, ip] = line.split(/\s+/);
          return { domain, ip };
        })
        .filter((addr) => addr.domain && addr.ip);

      if (addresses.length === 0) {
        message.error("没有有效的地址映射");
        return;
      }

      const nodeIds = values.node_ids || [];
      await batchAddAddresses({
        addresses,
        node_ids: nodeIds,
      });

      message.success(
        `成功添加 ${addresses.length} 条地址映射，正在同步到节点...`
      );
      setBatchModalVisible(false);
      loadAddresses();
    } catch (error) {
      message.error("批量添加失败");
    }
  };

  const handleImport = async (file) => {
    const reader = new FileReader();
    reader.onload = async (e) => {
      try {
        const content = e.target.result;
        await importAddresses({ content, format: "smartdns" });
        message.success("导入成功，正在同步到节点...");
        loadAddresses();
      } catch (error) {
        message.error("导入失败");
      }
    };
    reader.readAsText(file);
    return false;
  };

  const handleExport = () => {
    const content = addresses
      .map((addr) => `address /${addr.domain}/${addr.ip}`)
      .join("\n");

    const blob = new Blob([content], { type: "text/plain" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `addresses_${moment().format("YYYYMMDD_HHmmss")}.conf`;
    a.click();
    URL.revokeObjectURL(url);
  };

  const handleSyncToNode = async (nodeId) => {
    try {
      message.loading({ content: "正在同步配置到节点...", key: "sync" });
      await triggerFullSync(nodeId);
      message.success({ content: "同步任务已启动", key: "sync" });
    } catch (error) {
      message.error({ content: "同步失败", key: "sync" });
    }
  };

  const handleBatchSync = async () => {
    if (selectedRowKeys.length === 0) {
      message.warning("请先选择要同步的节点");
      return;
    }

    Modal.confirm({
      title: "批量同步配置",
      content: `确定要将配置同步到选中的 ${selectedRowKeys.length} 个节点吗？`,
      onOk: async () => {
        try {
          message.loading({ content: "正在批量同步...", key: "batchSync" });
          await batchFullSync({ node_ids: selectedRowKeys });
          message.success({ content: "批量同步任务已启动", key: "batchSync" });
          setSelectedRowKeys([]);
        } catch (error) {
          message.error({ content: "批量同步失败", key: "batchSync" });
        }
      },
    });
  };

  const handleViewSyncStatus = (nodeId) => {
    setSelectedNodeId(nodeId);
    setSyncStatusVisible(true);
  };

  const parseNodeIds = (nodeIdsStr) => {
    if (!nodeIdsStr || nodeIdsStr === "[]") return [];
    try {
      return JSON.parse(nodeIdsStr);
    } catch {
      return [];
    }
  };

  const columns = [
    {
      title: "类型",
      dataIndex: "type",
      key: "type",
      width: 100,
      render: (type) => (
        <Tag color={type === "cname" ? "green" : "blue"}>
          {type === "cname" ? "CNAME" : "Address"}
        </Tag>
      ),
    },
    {
      title: "域名",
      dataIndex: "domain",
      key: "domain",
      width: 250,
      render: (text) => <Tag color="blue">{text}</Tag>,
    },
    {
      title: "目标",
      key: "target",
      width: 250,
      render: (_, record) =>
        record.type === "cname" ? (
          <Tag color="green">{record.cname}</Tag>
        ) : (
          <code>{record.ip}</code>
        ),
    },
    {
      title: "IP地址",
      dataIndex: "ip",
      key: "ip",
      width: 200,
      render: (text) => <code>{text}</code>,
    },
  {
    title: "应用节点",
    dataIndex: "node_ids",
    key: "node_ids",
    width: 300, // 增加宽度
    render: (nodeIdsStr, record) => {
      const nodeIds = parseNodeIds(nodeIdsStr);
      if (nodeIds.length === 0) {
        return <Tag color="green">全部节点</Tag>;
      }
      const nodeNames = nodes
        .filter((n) => nodeIds.includes(n.id))
        .map((n) => n.name);
      
      // 优化显示逻辑
      if (nodeNames.length <= 3) {
        return (
          <Space wrap size={[4, 4]}>
            {nodeNames.map((name) => (
              <Tag key={name} color="cyan" style={{ margin: '2px' }}>
                {name}
              </Tag>
            ))}
          </Space>
        );
      }
      
      return (
        <Tooltip 
          title={nodeNames.join(", ")}
          overlayStyle={{ maxWidth: '400px' }}
        >
          <Space wrap size={[4, 4]}>
            {nodeNames.slice(0, 2).map((name) => (
              <Tag key={name} color="cyan" style={{ margin: '2px' }}>
                {name}
              </Tag>
            ))}
            <Tag color="default" style={{ margin: '2px' }}>
              +{nodeNames.length - 2}个
            </Tag>
          </Space>
        </Tooltip>
      );
    },
  },
    {
      title: "状态",
      dataIndex: "enabled",
      key: "enabled",
      width: 80,
      render: (enabled) => (
        <Badge
          status={enabled ? "success" : "default"}
          text={enabled ? "启用" : "禁用"}
        />
      ),
    },
    {
      title: "备注",
      dataIndex: "comment",
      key: "comment",
      ellipsis: true,
      render: (text) => text || "-",
    },
    {
      title: "创建时间",
      dataIndex: "created_at",
      key: "created_at",
      width: 180,
      render: (time) => moment(time).format("YYYY-MM-DD HH:mm:ss"),
    },
    {
      title: "操作",
      key: "action",
      fixed: "right",
      width: 150,
      render: (_, record) => (
        <Space size="small">
          <Button
            type="link"
            size="small"
            icon={<EditOutlined />}
            onClick={() => handleEdit(record)}
          >
            编辑
          </Button>
          <Popconfirm
            title="确定要删除吗？"
            onConfirm={() => handleDelete(record.id)}
            okText="确定"
            cancelText="取消"
          >
            <Button type="link" size="small" danger icon={<DeleteOutlined />}>
              删除
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  const rowSelection = {
    selectedRowKeys,
    onChange: setSelectedRowKeys,
  };

  return (
    <div>
      <div
        style={{
          marginBottom: 16,
          display: "flex",
          justifyContent: "space-between",
        }}
      >
        <Space>
          <Button type="primary" icon={<PlusOutlined />} onClick={handleAdd}>
            添加地址映射
          </Button>
          <Button icon={<PlusOutlined />} onClick={handleBatchAdd}>
            批量添加
          </Button>
          <Upload
            accept=".conf,.txt"
            showUploadList={false}
            beforeUpload={handleImport}
          >
            <Button icon={<UploadOutlined />}>导入</Button>
          </Upload>
          <Button
            icon={<DownloadOutlined />}
            onClick={handleExport}
            disabled={addresses.length === 0}
          >
            导出
          </Button>
          <Button
            icon={<SyncOutlined />}
            onClick={() => setSyncStatusVisible(true)}
          >
            同步状态
          </Button>
        </Space>
        <span style={{ color: "#666" }}>共 {pagination.total} 条记录</span>
      </div>

      <Table
        columns={columns}
        dataSource={addresses}
        rowKey="id"
        loading={loading}
        scroll={{ x: 1400 }}
        pagination={{
          ...pagination,
          showSizeChanger: true,
          showTotal: (total) => `共 ${total} 条记录`,
          onChange: (page, pageSize) => {
            setPagination({ ...pagination, current: page, pageSize });
          },
        }}
      />

      {/* 添加/编辑地址映射 Modal */}
      <Modal
        title={editingAddress ? "编辑地址映射" : "添加地址映射"}
        open={modalVisible}
        onOk={handleSubmit}
        onCancel={() => setModalVisible(false)}
        width={600}
        okText="确定"
        cancelText="取消"
      >
        <Form form={form} layout="vertical">
          <Form.Item
            name="type"
            label="映射类型"
            rules={[{ required: true }]}
            initialValue="address"
          >
            <Select
              onChange={(value) => {
                // 切换类型时清空对应字段
                if (value === "address") {
                  form.setFieldsValue({ cname: undefined });
                } else {
                  form.setFieldsValue({ ip: undefined });
                }
              }}
            >
              <Option value="address">
                <Space>
                  <Tag color="blue">Address</Tag>
                  <span>IP 地址映射</span>
                </Space>
              </Option>
              <Option value="cname">
                <Space>
                  <Tag color="green">CNAME</Tag>
                  <span>别名映射</span>
                </Space>
              </Option>
            </Select>
          </Form.Item>

          <Form.Item
            name="domain"
            label="源域名"
            rules={[
              { required: true, message: "请输入域名" },
              {
                pattern:
                  /^[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?)*$/,
                message: "请输入有效的域名",
              },
            ]}
          >
            <Input placeholder="例如: a.com" />
          </Form.Item>

          <Form.Item
            noStyle
            shouldUpdate={(prev, curr) => prev.type !== curr.type}
          >
            {({ getFieldValue }) => {
              const type = getFieldValue("type");

              if (type === "address") {
                return (
                  <Form.Item
                    name="ip"
                    label="IP地址"
                    rules={[
                      { required: true, message: "请输入IP地址" },
                      {
                        pattern: /^(\d{1,3}\.){3}\d{1,3}$/,
                        message: "请输入有效的IP地址",
                      },
                    ]}
                  >
                    <Input placeholder="例如: 192.168.1.1" />
                  </Form.Item>
                );
              } else {
                return (
                  <Form.Item
                    name="cname"
                    label="目标域名 (CNAME)"
                    rules={[
                      { required: true, message: "请输入目标域名" },
                      {
                        pattern:
                          /^[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?)*$/,
                        message: "请输入有效的域名",
                      },
                    ]}
                    extra="查询源域名时，将使用目标域名的查询结果"
                  >
                    <Input placeholder="例如: b.com" />
                  </Form.Item>
                );
              }
            }}
          </Form.Item>

          <Form.Item
            name="node_ids"
            label="应用到节点"
            extra="不选择则应用到所有节点"
          >
            <Select mode="multiple" placeholder="选择要应用的节点" allowClear>
              {nodes.map((node) => (
                <Option key={node.id} value={node.id}>
                  <Space>
                    <CloudServerOutlined />
                    {node.name}
                    {node.status === "online" && (
                      <CheckCircleOutlined style={{ color: "#52c41a" }} />
                    )}
                  </Space>
                </Option>
              ))}
            </Select>
          </Form.Item>

          <Form.Item
            name="enabled"
            label="启用状态"
            valuePropName="checked"
            initialValue={true}
          >
            <Switch checkedChildren="启用" unCheckedChildren="禁用" />
          </Form.Item>

          <Form.Item name="tags" label="标签">
            <Input placeholder="多个标签用逗号分隔" />
          </Form.Item>

          <Form.Item name="comment" label="备注">
            <Input.TextArea rows={3} placeholder="可选的备注信息" />
          </Form.Item>
        </Form>
      </Modal>

      {/* 批量添加 Modal */}
      <Modal
        title="批量添加地址映射"
        open={batchModalVisible}
        onOk={handleBatchSubmit}
        onCancel={() => setBatchModalVisible(false)}
        width={700}
        okText="确定"
        cancelText="取消"
      >
        <Form form={batchForm} layout="vertical">
          <Form.Item
            name="node_ids"
            label="应用到节点"
            extra="不选择则应用到所有节点"
          >
            <Select mode="multiple" placeholder="选择要应用的节点" allowClear>
              {nodes.map((node) => (
                <Option key={node.id} value={node.id}>
                  {node.name}
                </Option>
              ))}
            </Select>
          </Form.Item>

          <Form.Item
            name="content"
            label="映射列表"
            rules={[{ required: true, message: "请输入映射" }]}
            extra="每行一条记录。Address格式: 域名 IP | CNAME格式: 域名 cname 目标域名"
          >
            <Input.TextArea
              rows={15}
              placeholder={`# Address 映射
example.com 192.168.1.1
test.com 192.168.1.2

# CNAME 映射
a.com cname b.com
cdn.example.com cname cdn.cloudflare.com`}
              style={{ fontFamily: "monospace" }}
            />
          </Form.Item>
        </Form>
      </Modal>

      {/* 同步状态 Modal */}
      <SyncStatus
        visible={syncStatusVisible}
        onClose={() => setSyncStatusVisible(false)}
        nodeId={selectedNodeId}
      />
    </div>
  );
};

export default AddressManager;
