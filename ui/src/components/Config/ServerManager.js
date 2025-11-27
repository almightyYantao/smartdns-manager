import React, { useState, useEffect } from "react";
import {
  Table,
  Button,
  Space,
  Tag,
  Modal,
  Form,
  Input,
  Select,
  message,
  Popconfirm,
  Switch,
} from "antd";
import {
  PlusOutlined,
  EditOutlined,
  DeleteOutlined,
  SyncOutlined,
} from "@ant-design/icons";
import {
  getServers,
  addServer,
  updateServer,
  deleteServer,
  getGroups,
} from "../../api";
import dayjs from "dayjs";

const { Option } = Select;

const ServerManager = () => {
  const [servers, setServers] = useState([]);
  const [groups, setGroups] = useState([]);
  const [loading, setLoading] = useState(false);
  const [modalVisible, setModalVisible] = useState(false);
  const [editingServer, setEditingServer] = useState(null);
  const [form] = Form.useForm();

  useEffect(() => {
    loadServers();
    loadGroups();
  }, []);

  const loadGroups = async () => {
    try {
      const response = await getGroups();
      setGroups(response.data || []);
    } catch (error) {
      message.error("加载分组失败");
    }
  };

  const loadServers = async () => {
    try {
      setLoading(true);
      const response = await getServers();
      setServers(response.data || []);
    } catch (error) {
      message.error("加载DNS服务器失败");
    } finally {
      setLoading(false);
    }
  };

  const handleAdd = () => {
    setEditingServer(null);
    form.resetFields();
    setModalVisible(true);
  };

  const handleEdit = (record) => {
    setEditingServer(record);
    form.setFieldsValue({
      ...record,
      groups: record.groups || [],
    });
    setModalVisible(true);
  };

  const handleDelete = async (id) => {
    try {
      await deleteServer(id);
      message.success("删除成功");
      loadServers();
    } catch (error) {
      message.error("删除失败");
    }
  };

  const handleSubmit = async () => {
    try {
      const values = await form.validateFields();

      if (editingServer) {
        await updateServer(editingServer.id, values);
        message.success("更新成功");
      } else {
        await addServer(values);
        message.success("添加成功");
      }

      setModalVisible(false);
      loadServers();
    } catch (error) {
      message.error("操作失败");
    }
  };

  const getTypeColor = (type) => {
    const colors = {
      https: "green",
      tls: "blue",
      tcp: "orange",
      udp: "default",
    };
    return colors[type] || "default";
  };

  const commonGroups = ["cn", "oversea", "local", "ad", "bootstrap"];

  const columns = [
    {
      title: "服务器地址",
      dataIndex: "address",
      key: "address",
      width: 300,
      render: (text) => <code>{text}</code>,
    },
    {
      title: "类型",
      dataIndex: "type",
      key: "type",
      width: 100,
      render: (type) => (
        <Tag color={getTypeColor(type)}>{type?.toUpperCase()}</Tag>
      ),
    },
    {
      title: "分组",
      dataIndex: "groups",
      key: "groups",
      width: 200,
      render: (groups) => (
        <>
          {groups?.map((group) => (
            <Tag key={group} color="cyan">
              {group}
            </Tag>
          ))}
          {(!groups || groups.length === 0) && "-"}
        </>
      ),
    },
    {
      title: "排除默认组",
      dataIndex: "exclude_default",
      key: "exclude_default",
      width: 120,
      render: (exclude) => (
        <Tag color={exclude ? "warning" : "default"}>
          {exclude ? "是" : "否"}
        </Tag>
      ),
    },
    {
      title: "创建时间",
      dataIndex: "created_at",
      key: "created_at",
      width: 180,
      render: (time) => dayjs(time).format("YYYY-MM-DD HH:mm:ss"),
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
            添加DNS服务器
          </Button>
          <Button icon={<SyncOutlined />} onClick={loadServers}>
            刷新
          </Button>
        </Space>
        <span style={{ color: "#666" }}>共 {servers.length} 个服务器</span>
      </div>

      <Table
        columns={columns}
        dataSource={servers}
        rowKey="id"
        loading={loading}
        scroll={{ x: 1200 }}
        pagination={{
          pageSize: 20,
          showSizeChanger: true,
          showTotal: (total) => `共 ${total} 条记录`,
        }}
      />

      <Modal
        title={editingServer ? "编辑DNS服务器" : "添加DNS服务器"}
        open={modalVisible}
        onOk={handleSubmit}
        onCancel={() => setModalVisible(false)}
        width={600}
        okText="确定"
        cancelText="取消"
      >
        <Form form={form} layout="vertical">
          <Form.Item
            name="address"
            label="服务器地址"
            rules={[{ required: true, message: "请输入服务器地址" }]}
            extra="支持格式: IP地址、域名、https://、tls://"
          >
            <Input placeholder="例如: 8.8.8.8 或 https://dns.google/dns-query" />
          </Form.Item>

          <Form.Item
            name="type"
            label="服务器类型"
            rules={[{ required: true, message: "请选择服务器类型" }]}
          >
            <Select placeholder="选择服务器类型">
              <Option value="udp">UDP</Option>
              <Option value="tcp">TCP</Option>
              <Option value="tls">TLS</Option>
              <Option value="https">HTTPS</Option>
            </Select>
          </Form.Item>

          <Form.Item
            name="groups"
            label="服务器分组"
            extra="选择或输入自定义分组"
            rules={[{ required: true, message: "请选择服务器分组" }]}
          >
            <Select
              mode="tags"
              placeholder="选择分组"
              options={groups.map((g) => ({
                label: (
                  <Space>
                    <Tag color={g.color}>{g.name}</Tag>
                  </Space>
                ),
                value: g.name,
              }))}
            />
          </Form.Item>

          <Form.Item
            name="exclude_default"
            label="排除默认组"
            valuePropName="checked"
            extra="是否排除该服务器在默认组之外"
          >
            <Switch />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default ServerManager;
