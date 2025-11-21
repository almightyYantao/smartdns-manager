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
  Select,
  Badge,
  Tooltip,
  Spin
} from "antd";
import {
  PlusOutlined,
  EditOutlined,
  DeleteOutlined,
  DownloadOutlined,
  UploadOutlined,
  EyeOutlined,
} from "@ant-design/icons";
import {
  getDomainSets,
  getDomainSet,
  addDomainSet,
  updateDomainSet,
  deleteDomainSet,
  importDomainSetFile,
  exportDomainSet,
  getNodes,
} from "../../api";
import moment from "moment";

const { TextArea } = Input;
const { Option } = Select;

const DomainSetManager = () => {
  const [domainSets, setDomainSets] = useState([]);
  const [nodes, setNodes] = useState([]);
  const [loading, setLoading] = useState(false);
  const [modalVisible, setModalVisible] = useState(false);
  const [viewModalVisible, setViewModalVisible] = useState(false);
  const [editingSet, setEditingSet] = useState(null);
  const [viewingSet, setViewingSet] = useState(null);
  const [domainItems, setDomainItems] = useState([]);
  const [submitLoading, setSubmitLoading] = useState(false); // 新增：提交loading
  const [importLoading, setImportLoading] = useState(false); // 新增：导入loading
  const [viewLoading, setViewLoading] = useState(false); // 新增：查看详情loading
  const [form] = Form.useForm();

  useEffect(() => {
    loadDomainSets();
    loadNodes();
  }, []);

  const loadDomainSets = async () => {
    try {
      setLoading(true);
      const response = await getDomainSets();
      setDomainSets(response.data || []);
    } catch (error) {
      message.error("加载域名集失败");
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
    setEditingSet(null);
    form.resetFields();
    setModalVisible(true);
  };

  const handleEdit = async (record) => {
    try {
      setSubmitLoading(true); // 编辑时也显示loading
      const response = await getDomainSet(record.id);
      const { domain_set, items } = response.data;
      setEditingSet(domain_set);
      const nodeIds = domain_set.node_ids
        ? JSON.parse(domain_set.node_ids)
        : [];
      const domains = items.map((item) => item.domain).join("\n");
      form.setFieldsValue({
        name: domain_set.name,
        description: domain_set.description,
        domains,
        node_ids: nodeIds,
      });
      setModalVisible(true);
    } catch (error) {
      message.error("加载域名集失败");
    } finally {
      setSubmitLoading(false);
    }
  };

  const handleView = async (record) => {
    try {
      setViewLoading(true); // 新增：查看时显示loading
      const response = await getDomainSet(record.id);
      const { domain_set, items } = response.data;
      setViewingSet(domain_set);
      setDomainItems(items);
      setViewModalVisible(true);
    } catch (error) {
      message.error("加载域名集失败");
    } finally {
      setViewLoading(false);
    }
  };

  const handleDelete = async (id) => {
    try {
      await deleteDomainSet(id);
      message.success("删除成功");
      loadDomainSets();
    } catch (error) {
      message.error("删除失败");
    }
  };

  const handleExport = async (record) => {
    try {
      const response = await exportDomainSet(record.id);
      // 下载文件
      const blob = new Blob([response.data], { type: "text/plain" });
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `${record.name}.conf`;
      a.click();
      URL.revokeObjectURL(url);
      message.success("导出成功");
    } catch (error) {
      message.error("导出失败");
    }
  };

  const handleSubmit = async () => {
    try {
      setSubmitLoading(true); // 开始loading
      const values = await form.validateFields();
      // 解析域名列表
      const domains = values.domains
        .split("\n")
        .map((d) => d.trim())
        .filter((d) => d && !d.startsWith("#"));

      const data = {
        name: values.name,
        description: values.description,
        domains,
        node_ids: values.node_ids || [],
      };

      if (editingSet) {
        await updateDomainSet(editingSet.id, {
          description: data.description,
          domains: data.domains,
          node_ids: data.node_ids,
          enabled: editingSet.enabled,
        });
        message.success("更新成功，正在同步到节点...");
      } else {
        await addDomainSet(data);
        message.success("添加成功，正在同步到节点...");
      }
      setModalVisible(false);
      loadDomainSets();
    } catch (error) {
      if (error.errorFields) {
        // 表单验证错误，不显示错误消息
        return;
      }
      message.error("操作失败");
    } finally {
      setSubmitLoading(false); // 结束loading
    }
  };

  const handleImport = async (record) => {
    let importModalRef;
    
    Modal.confirm({
      title: "导入域名列表",
      content: (
        <TextArea
          rows={15}
          placeholder="每行一个域名，支持注释（以 # 开头）"
          id="import-textarea"
        />
      ),
      width: 600,
      okText: "确定",
      cancelText: "取消",
      confirmLoading: importLoading, // 新增：显示确认按钮的loading
      onOk: async () => {
        const content = document.getElementById("import-textarea").value;
        if (!content) {
          message.warning("请输入域名列表");
          return Promise.reject(); // 阻止Modal关闭
        }
        try {
          setImportLoading(true);
          await importDomainSetFile(record.id, { content });
          message.success("导入成功");
          loadDomainSets();
        } catch (error) {
          message.error("导入失败");
          return Promise.reject(); // 出错时阻止Modal关闭
        } finally {
          setImportLoading(false);
        }
      },
    });
  };

  const columns = [
    {
      title: "名称",
      dataIndex: "name",
      key: "name",
      width: 200,
      render: (text) => <Tag color="blue">{text}</Tag>,
    },
    {
      title: "描述",
      dataIndex: "description",
      key: "description",
      ellipsis: true,
    },
    {
      title: "文件路径",
      dataIndex: "file_path",
      key: "file_path",
      width: 250,
      render: (text) => <code style={{ fontSize: "12px" }}>{text}</code>,
    },
    {
      title: "域名数量",
      dataIndex: "domain_count",
      key: "domain_count",
      width: 100,
      render: (count) => (
        <Badge count={count} showZero style={{ backgroundColor: "#52c41a" }} />
      ),
    },
    {
      title: "状态",
      dataIndex: "enabled",
      key: "enabled",
      width: 80,
      render: (enabled) => (
        <Tag color={enabled ? "success" : "default"}>
          {enabled ? "启用" : "禁用"}
        </Tag>
      ),
    },
    {
      title: "更新时间",
      dataIndex: "updated_at",
      key: "updated_at",
      width: 180,
      render: (time) => moment(time).format("YYYY-MM-DD HH:mm:ss"),
    },
    {
      title: "操作",
      key: "action",
      fixed: "right",
      width: 280,
      render: (_, record) => (
        <Space size="small">
          <Tooltip title="查看">
            <Button
              type="link"
              size="small"
              icon={<EyeOutlined />}
              onClick={() => handleView(record)}
            />
          </Tooltip>
          <Tooltip title="编辑">
            <Button
              type="link"
              size="small"
              icon={<EditOutlined />}
              onClick={() => handleEdit(record)}
            />
          </Tooltip>
          <Tooltip title="导入">
            <Button
              type="link"
              size="small"
              icon={<UploadOutlined />}
              onClick={() => handleImport(record)}
            />
          </Tooltip>
          <Tooltip title="导出">
            <Button
              type="link"
              size="small"
              icon={<DownloadOutlined />}
              onClick={() => handleExport(record)}
            />
          </Tooltip>
          <Popconfirm
            title="确定要删除吗？"
            onConfirm={() => handleDelete(record.id)}
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
      <div style={{ marginBottom: 16 }}>
        <Space>
          <Button type="primary" icon={<PlusOutlined />} onClick={handleAdd}>
            添加域名集
          </Button>
        </Space>
        <span style={{ marginLeft: 16, color: "#666" }}>
          共 {domainSets.length} 个域名集
        </span>
      </div>
      <Table
        columns={columns}
        dataSource={domainSets}
        rowKey="id"
        loading={loading}
        scroll={{ x: 1400 }}
        pagination={{
          pageSize: 10,
          showSizeChanger: true,
          showTotal: (total) => `共 ${total} 条记录`,
        }}
      />
      {/* 添加/编辑 Modal */}
      <Modal
        title={editingSet ? "编辑域名集" : "添加域名集"}
        open={modalVisible}
        onOk={handleSubmit}
        onCancel={() => setModalVisible(false)}
        width={800}
        okText="确定"
        cancelText="取消"
        confirmLoading={submitLoading} // 新增：提交时显示loading
        maskClosable={!submitLoading} // 新增：提交时禁止点击遮罩关闭
        closable={!submitLoading} // 新增：提交时禁止关闭
      >
        <Spin spinning={submitLoading} tip="正在处理，请稍候..."> {/* 新增：整体loading效果 */}
          <Form form={form} layout="vertical">
            <Form.Item
              name="name"
              label="域名集名称"
              rules={[
                { required: true, message: "请输入域名集名称" },
                {
                  pattern: /^[a-zA-Z0-9_-]+$/,
                  message: "只能包含字母、数字、下划线和横线",
                },
              ]}
            >
              <Input placeholder="例如: gfwlist" disabled={!!editingSet} />
            </Form.Item>
            <Form.Item name="description" label="描述">
              <Input placeholder="域名集的描述信息" />
            </Form.Item>
            <Form.Item
              name="node_ids"
              label="应用到节点"
              extra="不选择则应用到所有节点"
            >
              <Select mode="multiple" placeholder="选择节点" allowClear>
                {nodes.map((node) => (
                  <Option key={node.id} value={node.id}>
                    {node.name}
                  </Option>
                ))}
              </Select>
            </Form.Item>
            <Form.Item
              name="domains"
              label="域名列表"
              rules={[{ required: true, message: "请输入域名列表" }]}
              extra="每行一个域名，支持注释（以 # 开头）"
            >
              <TextArea
                rows={15}
                placeholder={`# 示例域名列表
google.com
youtube.com
facebook.com
twitter.com`}
                style={{ fontFamily: "monospace" }}
              />
            </Form.Item>
          </Form>
        </Spin>
      </Modal>
      {/* 查看详情 Modal */}
      <Modal
        title={`域名集详情 - ${viewingSet?.name}`}
        open={viewModalVisible}
        onCancel={() => setViewModalVisible(false)}
        width={800}
        footer={[
          <Button key="close" onClick={() => setViewModalVisible(false)}>
            关闭
          </Button>,
        ]}
      >
        <Spin spinning={viewLoading} tip="加载中..."> {/* 新增：查看详情loading */}
          {viewingSet && (
            <div>
              <div style={{ marginBottom: 16 }}>
                <Space direction="vertical" style={{ width: "100%" }}>
                  <div>
                    <strong>文件路径：</strong>
                    <code>{viewingSet.file_path}</code>
                  </div>
                  <div>
                    <strong>描述：</strong>
                    {viewingSet.description || "-"}
                  </div>
                  <div>
                    <strong>域名数量：</strong>
                    {domainItems.length}
                  </div>
                </Space>
              </div>
              <div
                style={{
                  maxHeight: "400px",
                  overflow: "auto",
                  background: "#f5f5f5",
                  padding: "12px",
                  borderRadius: "4px",
                }}
              >
                {domainItems.map((item, index) => (
                  <div key={item.id} style={{ marginBottom: "4px" }}>
                    <code>
                      {index + 1}. {item.domain}
                      {item.comment && (
                        <span style={{ color: "#999", marginLeft: "8px" }}>
                          # {item.comment}
                        </span>
                      )}
                    </code>
                  </div>
                ))}
              </div>
            </div>
          )}
        </Spin>
      </Modal>
    </div>
  );
};

export default DomainSetManager;