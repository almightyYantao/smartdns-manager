import React, { useState, useEffect } from "react";
import {
  Table,
  Button,
  Space,
  Modal,
  message,
  Popconfirm,
  Tag,
  Descriptions,
  Alert,
  Input,
  Typography,
  Select,
  Badge,
  Tooltip,
  Form,
} from "antd";
import {
  EyeOutlined,
  RollbackOutlined,
  PlusOutlined,
  DeleteOutlined,
  DownloadOutlined,
  ReloadOutlined,
  CloudServerOutlined,
  CloudDownloadOutlined,
} from "@ant-design/icons";
import {
  getNodeBackups,
  createNodeBackup,
  restoreNodeBackup,
  deleteNodeBackup,
  previewBackup as previewBackupApi,
  getNodes,
} from "../../api";
import moment from "moment";

const { TextArea } = Input;
const { Text, Paragraph } = Typography;
const { Option } = Select;

const BackupManager = () => {
  const [nodes, setNodes] = useState([]);
  const [selectedNodeId, setSelectedNodeId] = useState(null);
  const [selectedNodeName, setSelectedNodeName] = useState("");
  const [backups, setBackups] = useState([]);
  const [loading, setLoading] = useState(false);
  const [previewVisible, setPreviewVisible] = useState(false);
  const [previewContent, setPreviewContent] = useState("");
  const [previewBackup, setPreviewBackup] = useState(null);
  const [restoreVisible, setRestoreVisible] = useState(false);
  const [createVisible, setCreateVisible] = useState(false);
  const [selectedBackup, setSelectedBackup] = useState(null);
  const [pagination, setPagination] = useState({
    current: 1,
    pageSize: 10,
    total: 0,
  });
  const [form] = Form.useForm();

  // 加载节点列表
  const loadNodes = async () => {
    try {
      const response = await getNodes();
      setNodes(response.data || []);
    } catch (error) {
      console.error("加载节点列表失败", error);
      message.error("加载节点列表失败");
    }
  };

  useEffect(() => {
    loadNodes();
  }, []);

  useEffect(() => {
    if (selectedNodeId) {
      loadBackups(1, pagination.pageSize);
    }
  }, [selectedNodeId]);

  // 加载备份列表（带分页）
  const loadBackups = async (page = 1, pageSize = 10) => {
    if (!selectedNodeId) {
      return;
    }
    try {
      setLoading(true);
      const response = await getNodeBackups(selectedNodeId, {
        page,
        page_size: pageSize,
      });
      
      const { list, total, page: currentPage, page_size } = response.data;
      
      setBackups(list || []);
      setPagination({
        current: currentPage,
        pageSize: page_size,
        total: total,
      });
    } catch (error) {
      console.error("加载备份列表失败", error);
      message.error("加载备份列表失败");
    } finally {
      setLoading(false);
    }
  };

  // 处理分页变化
  const handleTableChange = (newPagination) => {
    loadBackups(newPagination.current, newPagination.pageSize);
  };

  // 刷新列表
  const handleRefresh = () => {
    loadBackups(pagination.current, pagination.pageSize);
  };

  // 节点切换
  const handleNodeChange = (value) => {
    const node = nodes.find((n) => n.id === value);
    setSelectedNodeId(value);
    setSelectedNodeName(node?.name || "");
    // 重置分页
    setPagination({
      current: 1,
      pageSize: 10,
      total: 0,
    });
  };

  // 创建备份
  const handleCreateBackup = () => {
    if (!selectedNodeId) {
      message.warning("请先选择节点");
      return;
    }
    setCreateVisible(true);
    form.resetFields();
  };

  const confirmCreateBackup = async () => {
    try {
      const values = await form.validateFields();
      message.loading({ content: "正在创建备份...", key: "backup" });
      
      await createNodeBackup(selectedNodeId, values);
      
      message.success({ content: "备份创建成功", key: "backup" });
      setCreateVisible(false);
      loadBackups(1, pagination.pageSize); // 创建后回到第一页
    } catch (error) {
      if (error.errorFields) {
        // 表单验证错误
        return;
      }
      message.error({
        content: error.response?.data?.error || "创建备份失败",
        key: "backup",
      });
    }
  };

  // 预览备份
  const handlePreview = async (backup) => {
    try {
      message.loading({ content: "正在加载备份内容...", key: "preview" });
      const response = await previewBackupApi(selectedNodeId, {
        backup_id: backup.id,
      });
      setPreviewContent(response.data.content);
      setPreviewBackup(backup);
      setPreviewVisible(true);
      message.destroy("preview");
    } catch (error) {
      console.error("加载备份内容失败", error);
      message.error({
        content: error.response?.data?.error || "加载备份内容失败",
        key: "preview",
      });
    }
  };

  // 还原备份
  const handleRestore = (backup) => {
    setSelectedBackup(backup);
    setRestoreVisible(true);
  };

  const confirmRestore = async () => {
    try {
      message.loading({ content: "正在还原备份...", key: "restore" });
      await restoreNodeBackup(selectedNodeId, {
        backup_id: selectedBackup.id,
      });
      message.success({
        content: "备份还原成功，服务已重启",
        key: "restore",
        duration: 5,
      });
      setRestoreVisible(false);
      handleRefresh();
    } catch (error) {
      message.error({
        content: error.response?.data?.error || "还原备份失败",
        key: "restore",
      });
    }
  };

  // 删除备份
  const handleDelete = async (backup) => {
    try {
      await deleteNodeBackup(selectedNodeId, {
        backup_ids: [backup.id],
      });
      message.success("删除备份成功");
      handleRefresh();
    } catch (error) {
      message.error(error.response?.data?.error || "删除备份失败");
    }
  };

  // 下载备份
  const handleDownload = (backup) => {
    try {
      // 如果是 S3 且有下载链接，直接打开
      if (backup.storage_type === "s3" && backup.download_url) {
        window.open(backup.download_url, "_blank");
        message.success("正在下载...");
        return;
      }

      // 本地存储，通过 API 下载
      const token = localStorage.getItem("token");
      const downloadUrl = `/api/nodes/${selectedNodeId}/backups/download?backup_id=${backup.id}&token=${token}`;
      
      const a = document.createElement("a");
      a.href = downloadUrl;
      a.download = backup.name;
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      
      message.success("正在下载...");
    } catch (error) {
      message.error("下载失败");
    }
  };

  // 格式化文件大小
  const formatFileSize = (bytes) => {
    if (!bytes || bytes === 0) return "0 B";
    const k = 1024;
    const sizes = ["B", "KB", "MB", "GB"];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return Math.round((bytes / Math.pow(k, i)) * 100) / 100 + " " + sizes[i];
  };

  const columns = [
    {
      title: "备份名称",
      dataIndex: "name",
      key: "name",
      width: 280,
      render: (text, record) => (
        <Space direction="vertical" size={0}>
          <Text strong>{text}</Text>
          <Space size={4}>
            {record.is_auto && <Tag color="orange">自动</Tag>}
            {record.storage_type === "s3" ? (
              <Tag color="blue" icon={<CloudDownloadOutlined />}>
                S3
              </Tag>
            ) : (
              <Tag color="green">本地</Tag>
            )}
            {record.tags &&
              record.tags.split(",").map((tag, index) => (
                <Tag key={index} color="cyan">
                  {tag.trim()}
                </Tag>
              ))}
          </Space>
        </Space>
      ),
    },
    {
      title: "文件大小",
      dataIndex: "size",
      key: "size",
      width: 100,
      render: (size) => <Text type="secondary">{formatFileSize(size)}</Text>,
    },
    {
      title: "创建时间",
      dataIndex: "created_at",
      key: "created_at",
      width: 180,
      render: (time) => moment(time).format("YYYY-MM-DD HH:mm:ss"),
      sorter: (a, b) => new Date(a.created_at) - new Date(b.created_at),
      defaultSortOrder: "descend",
    },
    {
      title: "备注",
      dataIndex: "comment",
      key: "comment",
      ellipsis: {
        showTitle: false,
      },
      render: (text) => (
        <Tooltip title={text} placement="topLeft">
          <span>{text || "-"}</span>
        </Tooltip>
      ),
    },
    {
      title: "操作",
      key: "action",
      fixed: "right",
      width: 200,
      render: (_, record) => (
        <Space size="small">
          <Tooltip title="预览">
            <Button
              type="link"
              size="small"
              icon={<EyeOutlined />}
              onClick={() => handlePreview(record)}
            />
          </Tooltip>
          <Tooltip title="还原">
            <Button
              type="link"
              size="small"
              icon={<RollbackOutlined />}
              onClick={() => handleRestore(record)}
            />
          </Tooltip>
          <Tooltip title="下载">
            <Button
              type="link"
              size="small"
              icon={<DownloadOutlined />}
              onClick={() => handleDownload(record)}
            />
          </Tooltip>
          <Popconfirm
            title="确定要删除这个备份吗？"
            onConfirm={() => handleDelete(record)}
            okText="确定"
            cancelText="取消"
          >
            <Tooltip title="删除">
              <Button
                type="link"
                size="small"
                danger
                icon={<DeleteOutlined />}
              />
            </Tooltip>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <div>
      {/* 节点选择器 */}
      <div style={{ marginBottom: 16 }}>
        <Space>
          <Text strong>选择节点:</Text>
          <Select
            style={{ width: 300 }}
            placeholder="请选择要管理备份的节点"
            value={selectedNodeId}
            onChange={handleNodeChange}
            loading={nodes.length === 0}
            showSearch
            optionFilterProp="children"
          >
            {nodes.map((node) => (
              <Option key={node.id} value={node.id}>
                <Space>
                  <CloudServerOutlined />
                  {node.name}
                  <Badge
                    status={node.status === "online" ? "success" : "default"}
                    text={node.status === "online" ? "在线" : "离线"}
                  />
                </Space>
              </Option>
            ))}
          </Select>
        </Space>
      </div>

      {/* 操作按钮 */}
      <div
        style={{
          marginBottom: 16,
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
        }}
      >
        <Space>
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={handleCreateBackup}
            disabled={!selectedNodeId}
          >
            创建备份
          </Button>
          <Button
            icon={<ReloadOutlined />}
            onClick={handleRefresh}
            disabled={!selectedNodeId}
            loading={loading}
          >
            刷新
          </Button>
        </Space>
        <Text type="secondary">
          共 {pagination.total} 个备份
        </Text>
      </div>

      {!selectedNodeId ? (
        <Alert
          message="提示"
          description="请先选择一个节点来查看和管理备份"
          type="info"
          showIcon
        />
      ) : (
        <>
          <Alert
            message="备份说明"
            description="支持本地存储和 S3 存储。还原备份会自动重启 SmartDNS 服务。"
            type="info"
            showIcon
            closable
            style={{ marginBottom: 16 }}
          />
          <Table
            columns={columns}
            dataSource={backups}
            rowKey="id"
            loading={loading}
            scroll={{ x: 1000 }}
            pagination={{
              ...pagination,
              showSizeChanger: true,
              showQuickJumper: true,
              showTotal: (total) => `共 ${total} 个备份`,
              pageSizeOptions: ["10", "20", "50", "100"],
            }}
            onChange={handleTableChange}
          />
        </>
      )}

      {/* 创建备份 Modal */}
      <Modal
        title={
          <Space>
            <PlusOutlined />
            <span>创建备份</span>
          </Space>
        }
        open={createVisible}
        onOk={confirmCreateBackup}
        onCancel={() => setCreateVisible(false)}
        okText="创建"
        cancelText="取消"
      >
        <Form form={form} layout="vertical">
          <Form.Item
            label="备注"
            name="comment"
            rules={[{ max: 200, message: "备注不能超过200个字符" }]}
          >
            <TextArea
              rows={3}
              placeholder="请输入备份备注（可选）"
              maxLength={200}
              showCount
            />
          </Form.Item>
          <Form.Item
            label="标签"
            name="tags"
            rules={[{ max: 100, message: "标签不能超过100个字符" }]}
            extra="多个标签用逗号分隔，例如：重要,升级前"
          >
            <Input placeholder="请输入标签（可选）" maxLength={100} />
          </Form.Item>
        </Form>
        <Alert
          message="提示"
          description={`将为节点 "${selectedNodeName}" 创建配置备份`}
          type="info"
          showIcon
        />
      </Modal>

      {/* 预览备份 Modal */}
      <Modal
        title={
          <Space>
            <EyeOutlined />
            <span>预览备份</span>
          </Space>
        }
        open={previewVisible}
        onCancel={() => setPreviewVisible(false)}
        width={900}
        footer={[
          <Button key="close" onClick={() => setPreviewVisible(false)}>
            关闭
          </Button>,
          <Button
            key="restore"
            type="primary"
            icon={<RollbackOutlined />}
            onClick={() => {
              setPreviewVisible(false);
              handleRestore(previewBackup);
            }}
          >
            还原此备份
          </Button>,
        ]}
      >
        {previewBackup && (
          <>
            <Descriptions
              bordered
              size="small"
              column={2}
              style={{ marginBottom: 16 }}
            >
              <Descriptions.Item label="文件名" span={2}>
                {previewBackup.name}
              </Descriptions.Item>
              <Descriptions.Item label="存储类型">
                {previewBackup.storage_type === "s3" ? "S3 存储" : "本地存储"}
              </Descriptions.Item>
              <Descriptions.Item label="文件大小">
                {formatFileSize(previewBackup.size)}
              </Descriptions.Item>
              <Descriptions.Item label="创建时间" span={2}>
                {moment(previewBackup.created_at).format("YYYY-MM-DD HH:mm:ss")}
              </Descriptions.Item>
              {previewBackup.comment && (
                <Descriptions.Item label="备注" span={2}>
                  {previewBackup.comment}
                </Descriptions.Item>
              )}
            </Descriptions>
            <div style={{ marginBottom: 8 }}>
              <Text strong>配置内容：</Text>
            </div>
            <TextArea
              value={previewContent}
              rows={20}
              readOnly
              style={{
                fontFamily: "monospace",
                fontSize: "13px",
                backgroundColor: "#f5f5f5",
              }}
            />
          </>
        )}
      </Modal>

      {/* 还原确认 Modal */}
      <Modal
        title={
          <Space>
            <RollbackOutlined />
            <span>还原备份</span>
          </Space>
        }
        open={restoreVisible}
        onOk={confirmRestore}
        onCancel={() => setRestoreVisible(false)}
        okText="确定还原"
        cancelText="取消"
        okButtonProps={{ danger: true }}
      >
        <Alert
          message="警告"
          description="还原备份将覆盖当前配置文件，并自动重启 SmartDNS 服务。"
          type="warning"
          showIcon
          style={{ marginBottom: 16 }}
        />
        {selectedBackup && (
          <Descriptions bordered size="small" column={1}>
            <Descriptions.Item label="备份名称">
              {selectedBackup.name}
            </Descriptions.Item>
            <Descriptions.Item label="存储类型">
              {selectedBackup.storage_type === "s3" ? "S3 存储" : "本地存储"}
            </Descriptions.Item>
            <Descriptions.Item label="文件大小">
              {formatFileSize(selectedBackup.size)}
            </Descriptions.Item>
            <Descriptions.Item label="创建时间">
              {moment(selectedBackup.created_at).format("YYYY-MM-DD HH:mm:ss")}
            </Descriptions.Item>
            {selectedBackup.comment && (
              <Descriptions.Item label="备注">
                {selectedBackup.comment}
              </Descriptions.Item>
            )}
          </Descriptions>
        )}
        <Paragraph style={{ marginTop: 16, marginBottom: 0 }}>
          <Text type="secondary">确定要将配置还原到此备份吗？</Text>
        </Paragraph>
      </Modal>
    </div>
  );
};

export default BackupManager;