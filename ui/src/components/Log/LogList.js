import React, { useState, useEffect } from "react";
import {
  Table,
  Form,
  Input,
  Select,
  DatePicker,
  Button,
  Space,
  Tag,
  Row,
  Col,
  Badge,
  Tooltip,
  Empty,
} from "antd";
import {
  SearchOutlined,
  ReloadOutlined,
  ClearOutlined,
  ClockCircleOutlined,
} from "@ant-design/icons";
import { getDNSLogs } from "../../api";
import moment from "moment";

const { RangePicker } = DatePicker;
const { Option } = Select;

const LogList = ({ nodeId, nodeName }) => {
  const [logs, setLogs] = useState([]);
  const [loading, setLoading] = useState(false);
  const [pagination, setPagination] = useState({
    current: 1,
    pageSize: 20,
    total: 0,
  });
  const [form] = Form.useForm();

  useEffect(() => {
    loadLogs();
    const interval = setInterval(() => {
      loadLogs(true);
    }, 30000);

    return () => clearInterval(interval);
  }, [nodeId, pagination.current, pagination.pageSize]);

  const loadLogs = async (silent = false) => {
    try {
      if (!silent) setLoading(true);

      const values = form.getFieldsValue();
      const params = {
        node_id: nodeId,
        page: pagination.current,
        page_size: pagination.pageSize,
        ...values,
      };

      if (values.time_range) {
        params.start_time = values.time_range[0].toISOString();
        params.end_time = values.time_range[1].toISOString();
        delete params.time_range;
      }

      const response = await getDNSLogs(params);
      setLogs(response.data.logs || []);
      setPagination({
        ...pagination,
        total: response.data.total,
      });
    } catch (error) {
      console.error("加载日志失败:", error);
    } finally {
      if (!silent) setLoading(false);
    }
  };

  const handleSearch = () => {
    setPagination({ ...pagination, current: 1 });
    loadLogs();
  };

  const handleReset = () => {
    form.resetFields();
    setPagination({ ...pagination, current: 1 });
    loadLogs();
  };

  const handleTableChange = (newPagination) => {
    setPagination(newPagination);
  };

  const getQueryTypeTag = (type) => {
    const typeMap = {
      1: { color: "blue", text: "A" },
      28: { color: "cyan", text: "AAAA" },
      65: { color: "purple", text: "HTTPS" },
      5: { color: "green", text: "CNAME" },
      15: { color: "orange", text: "MX" },
    };
    const config = typeMap[type] || { color: "default", text: `TYPE ${type}` };
    return <Tag color={config.color}>{config.text}</Tag>;
  };

  const columns = [
    {
      title: "时间",
      dataIndex: "timestamp",
      key: "timestamp",
      width: 180,
      render: (time) => (
        <Tooltip title={moment(time).format("YYYY-MM-DD HH:mm:ss")}>
          <Space size={4}>
            <ClockCircleOutlined style={{ color: "#1890ff" }} />
            <span>{moment(time).format("HH:mm:ss")}</span>
          </Space>
        </Tooltip>
      ),
    },
    {
      title: "客户端IP",
      dataIndex: "client_ip",
      key: "client_ip",
      width: 130,
      render: (ip) => <Tag color="geekblue">{ip}</Tag>,
    },
    {
      title: "查询域名",
      dataIndex: "domain",
      key: "domain",
      ellipsis: true,
      render: (domain) => (
        <Tooltip title={domain}>
          <code style={{ fontSize: "12px" }}>{domain}</code>
        </Tooltip>
      ),
    },
    {
      title: "类型",
      dataIndex: "query_type",
      key: "query_type",
      width: 80,
      align: "center",
      render: (type) => getQueryTypeTag(type),
    },
    {
      title: "耗时",
      dataIndex: "time_ms",
      key: "time_ms",
      width: 80,
      align: "right",
      render: (time) => (
        <Tag color={time > 100 ? "red" : time > 50 ? "orange" : "green"}>
          {time}ms
        </Tag>
      ),
    },
      {
          title: "速度检查",
          dataIndex: "speed_ms",
          key: "speed_ms",
          width: 100,
          align: "right",
          render: (speed) => (
              <span style={{ color: speed < 0 ? "#999" : "#52c41a" }}>
          {speed.toFixed(1)}ms
        </span>
          ),
      },
      {
          title: "所属上游",
          dataIndex: "group",
          key: "group",
          width: 100,
          align: "right"
      },
    {
      title: "结果",
      dataIndex: "result",
      key: "result",
      ellipsis: true,
      render: (result, record) => {
        if (!result) {
          return <span style={{ color: "#999" }}>-</span>;
        }
        const ips = result.split(",").map((ip) => ip.trim());
        return (
          <Tooltip title={result}>
            <Space size={4} wrap>
              <Badge
                count={record.ip_count}
                style={{ backgroundColor: "#52c41a" }}
              />
              <span style={{ fontSize: "12px" }}>
                {ips[0]}
                {ips.length > 1 && ` +${ips.length - 1}`}
              </span>
            </Space>
          </Tooltip>
        );
      },
    },
  ];

  return (
    <div>
      <Form form={form} layout="vertical" style={{ marginBottom: 16 }}>
        <Row gutter={16}>
          <Col xs={24} sm={12} md={6}>
            <Form.Item
              name="client_ip"
              label="客户端IP"
              style={{ marginBottom: 8 }}
            >
              <Input placeholder="例如: 192.168.1.100" allowClear />
            </Form.Item>
          </Col>
          <Col xs={24} sm={12} md={6}>
            <Form.Item name="domain" label="域名" style={{ marginBottom: 8 }}>
              <Input placeholder="例如: google.com" allowClear />
            </Form.Item>
          </Col>
          <Col xs={24} sm={12} md={6}>
            <Form.Item
              name="query_type"
              label="查询类型"
              style={{ marginBottom: 8 }}
            >
              <Select placeholder="选择类型" allowClear>
                <Option value={1}>A (IPv4)</Option>
                <Option value={28}>AAAA (IPv6)</Option>
                <Option value={65}>HTTPS</Option>
                <Option value={5}>CNAME</Option>
                <Option value={15}>MX</Option>
              </Select>
            </Form.Item>
          </Col>
          <Col xs={24} sm={12} md={6}>
            <Form.Item
              name="time_range"
              label="时间范围"
              style={{ marginBottom: 8 }}
            >
              <RangePicker
                showTime
                format="YYYY-MM-DD HH:mm:ss"
                style={{ width: "100%" }}
              />
            </Form.Item>
          </Col>
        </Row>
        <Row>
          <Col span={24}>
            <Space>
              <Button
                type="primary"
                icon={<SearchOutlined />}
                onClick={handleSearch}
              >
                搜索
              </Button>
              <Button icon={<ClearOutlined />} onClick={handleReset}>
                重置
              </Button>
              <Button
                icon={<ReloadOutlined />}
                onClick={() => loadLogs()}
                loading={loading}
              >
                刷新
              </Button>
              <span style={{ color: "#999", marginLeft: 8 }}>
                共 {pagination.total} 条记录
              </span>
            </Space>
          </Col>
        </Row>
      </Form>

      <Table
        columns={columns}
        dataSource={logs}
        rowKey="id"
        loading={loading}
        pagination={{
          ...pagination,
          showSizeChanger: true,
          showQuickJumper: true,
          showTotal: (total) => `共 ${total} 条记录`,
          pageSizeOptions: ["10", "20", "50", "100", "500"],
        }}
        onChange={handleTableChange}
        scroll={{ x: 1200 }}
        size="small"
        locale={{
          emptyText: (
            <Empty
              description="暂无日志数据"
              image={Empty.PRESENTED_IMAGE_SIMPLE}
            />
          ),
        }}
      />
    </div>
  );
};

export default LogList;
