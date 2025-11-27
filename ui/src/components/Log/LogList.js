import React, { useState, useEffect, useMemo } from "react";
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
  Switch,
} from "antd";
import {
  SearchOutlined,
  ReloadOutlined,
  ClearOutlined,
  ClockCircleOutlined,
  SyncOutlined,
  PauseOutlined,
  GlobalOutlined,
  CloudServerOutlined,
} from "@ant-design/icons";
import { getDNSLogs, getGroups, getServers } from "../../api";
import dayjs from "dayjs";

const { RangePicker } = DatePicker;
const { Option } = Select;

const LogList = ({ nodeId, nodeName }) => {
  const [logs, setLogs] = useState([]);
  const [groups, setGroups] = useState([]);
  const [servers, setServers] = useState([]);
  const [loading, setLoading] = useState(false);
  const [autoRefresh, setAutoRefresh] = useState(false);
  const [pagination, setPagination] = useState({
    current: 1,
    pageSize: 20,
    total: 0,
  });
  const [form] = Form.useForm();
  const [intervalRef, setIntervalRef] = useState(null);
  const [sortInfo, setSortInfo] = useState({
    field: 'timestamp',
    order: 'descend'
  });

  useEffect(() => {
    loadGroups();
    loadServers();
    loadLogs();
  }, [nodeId, pagination.current, pagination.pageSize, sortInfo.field, sortInfo.order]);

  // 加载分组数据
  const loadGroups = async () => {
    try {
      const response = await getGroups();
      setGroups(response.data || []);
    } catch (error) {
      console.error("加载分组失败:", error);
    }
  };

  // 加载服务器数据
  const loadServers = async () => {
    try {
      const response = await getServers();
      setServers(response.data || []);
    } catch (error) {
      console.error("加载服务器失败:", error);
    }
  };

  // 处理自动刷新开关
  useEffect(() => {
    if (autoRefresh) {
      const interval = setInterval(() => {
        loadLogs(true);
      }, 30000);
      setIntervalRef(interval);
    } else {
      if (intervalRef) {
        clearInterval(intervalRef);
        setIntervalRef(null);
      }
    }
    return () => {
      if (intervalRef) {
        clearInterval(intervalRef);
      }
    };
  }, [autoRefresh]);

  useEffect(() => {
    return () => {
      if (intervalRef) {
        clearInterval(intervalRef);
      }
    };
  }, []);

  const loadLogs = async (silent = false) => {
    try {
      if (!silent) setLoading(true);
      const values = form.getFieldsValue();
      const params = {
        node_id: nodeId,
        page: pagination.current,
        page_size: pagination.pageSize,
        sort_field: sortInfo.field,
        sort_order: sortInfo.order === 'descend' ? 'desc' : 'asc',
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

  const handleTableChange = (newPagination, filters, sorter) => {
    setPagination(newPagination);
    
    // 处理排序变化
    if (sorter && sorter.field) {
      setSortInfo({
        field: sorter.field,
        order: sorter.order || 'descend'
      });
    } else if (sorter === null || !sorter.order) {
      // 清除排序，恢复默认
      setSortInfo({
        field: 'timestamp',
        order: 'descend'
      });
    }
  };

  const handleAutoRefreshChange = (checked) => {
    setAutoRefresh(checked);
  };

  // 添加状态来管理当前选中的时间范围
  const [currentTimeRange, setCurrentTimeRange] = useState(null);
  const [selectedQuickRange, setSelectedQuickRange] = useState(null);

  // 快速时间选择处理函数
  const handleQuickTimeSelect = (minutes) => {
    // 如果点击的是当前已选中的按钮，则取消选择
    if (selectedQuickRange === minutes) {
      console.log("取消时间范围选择");

      // 清除状态
      setCurrentTimeRange(null);
      setSelectedQuickRange(null);

      // 清除表单值
      form.setFieldsValue({
        time_range: undefined,
      });

      // 立即触发搜索
      setPagination({ ...pagination, current: 1 });
      setTimeout(() => {
        loadLogs();
      }, 100);
      return;
    }

    // 否则设置新的时间范围
    const now = dayjs();
    const start = dayjs().subtract(minutes, "minutes");
    const timeRange = [start, now];

    // 更新状态
    setCurrentTimeRange(timeRange);
    setSelectedQuickRange(minutes);

    // 设置表单值
    form.setFieldsValue({
      time_range: timeRange,
    });

    // 立即触发搜索
    setPagination({ ...pagination, current: 1 });
    setTimeout(() => {
      loadLogs();
    }, 100);
  };

  // 处理RangePicker的变化
  const handleTimeRangeChange = (dates, dateStrings) => {
    console.log("时间范围改变:", dates, dateStrings);
    if (dates && dates.length === 2) {
      setCurrentTimeRange(dates);
      setSelectedQuickRange(null); // 清除快速选择状态
      form.setFieldsValue({
        time_range: dates,
      });
    } else {
      setCurrentTimeRange(null);
      setSelectedQuickRange(null);
      form.setFieldsValue({
        time_range: undefined,
      });
    }
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

  // 根据分组名称获取对应的服务器列表
  const getServersByGroup = (groupName) => {
    return servers.filter(
      (server) => server.groups && server.groups.includes(groupName)
    );
  };

  // 提取服务器地址的IP部分
  const extractServerIP = (address) => {
    if (!address) return "";

    let cleanAddress = address;
    cleanAddress = cleanAddress.replace(/^(https?|tls):\/\//, "");
    cleanAddress = cleanAddress.split("/")[0];
    cleanAddress = cleanAddress.split(":")[0];

    return cleanAddress;
  };

  // 渲染上游分组和服务器信息
  const renderUpstreamInfo = (groupName) => {
    if (!groupName) {
      return <span style={{ color: "#999" }}>-</span>;
    }

    const groupConfig = groups.find((g) => g.name === groupName);
    const groupColor = groupConfig?.color || "#1890ff";
    const groupDescription = groupConfig?.description;
    const groupServers = getServersByGroup(groupName);

    return (
      <div style={{ minWidth: 160 }}>
        <Space direction="vertical" size={4}>
          <Tooltip title={groupDescription || `分组: ${groupName}`}>
            <Tag
              color={groupColor}
              icon={<GlobalOutlined />}
              style={{ margin: 0, fontSize: "11px", fontWeight: "bold" }}
            >
              {groupName}
            </Tag>
          </Tooltip>

          {groupServers.length > 0 ? (
            <div style={{ maxHeight: "80px", overflowY: "auto" }}>
              {groupServers.map((server, index) => {
                const serverIP = extractServerIP(server.address);
                const serverType = server.type?.toUpperCase() || "UDP";

                return (
                  <Tooltip
                    key={index}
                    title={`${serverType}: ${server.address}`}
                  >
                    <div
                      style={{
                        fontSize: "10px",
                        color: "#666",
                        fontFamily: "monospace",
                        background: "#f8f9fa",
                        padding: "2px 6px",
                        margin: "1px 0",
                        borderRadius: "3px",
                        border: "1px solid #e9ecef",
                        display: "flex",
                        alignItems: "center",
                        gap: "4px",
                      }}
                    >
                      <CloudServerOutlined
                        style={{ fontSize: "8px", color: "#999" }}
                      />
                      <span
                        style={{
                          flex: 1,
                          overflow: "hidden",
                          textOverflow: "ellipsis",
                        }}
                      >
                        {serverIP}
                      </span>
                      <Tag
                        color={
                          server.type === "https"
                            ? "green"
                            : server.type === "tls"
                            ? "blue"
                            : server.type === "tcp"
                            ? "orange"
                            : "default"
                        }
                        style={{
                          fontSize: "8px",
                          margin: 0,
                          padding: "0 3px",
                          lineHeight: "12px",
                          minWidth: "auto",
                        }}
                      >
                        {serverType}
                      </Tag>
                    </div>
                  </Tooltip>
                );
              })}
            </div>
          ) : (
            <div
              style={{
                fontSize: "10px",
                color: "#999",
                fontStyle: "italic",
                padding: "2px 0",
              }}
            >
              无配置服务器
            </div>
          )}

          {groupServers.length > 0 && (
            <div
              style={{
                fontSize: "9px",
                color: "#999",
                textAlign: "center",
                borderTop: "1px solid #f0f0f0",
                paddingTop: "2px",
              }}
            >
              共 {groupServers.length} 个服务器
            </div>
          )}
        </Space>
      </div>
    );
  };

  const columns = [
    {
      title: "时间",
      dataIndex: "timestamp",
      key: "timestamp",
      width: 180,
      sorter: true,
      sortDirections: ['descend', 'ascend'],
      defaultSortOrder: sortInfo.field === 'timestamp' ? sortInfo.order : null,
      render: (time) => (
        <Tooltip title={dayjs(time).format("YYYY-MM-DD HH:mm:ss")}>
          <Space size={4}>
            <ClockCircleOutlined style={{ color: "#1890ff" }} />
            <span>{dayjs(time).format("HH:mm:ss")}</span>
          </Space>
        </Tooltip>
      ),
    },
    {
      title: "客户端IP",
      dataIndex: "client_ip",
      key: "client_ip",
      width: 130,
      sorter: true,
      sortDirections: ['descend', 'ascend'],
      defaultSortOrder: sortInfo.field === 'client_ip' ? sortInfo.order : null,
      render: (ip) => <Tag color="geekblue">{ip}</Tag>,
    },
    {
      title: "查询域名",
      dataIndex: "domain",
      key: "domain",
      ellipsis: true,
      sorter: true,
      sortDirections: ['descend', 'ascend'],
      defaultSortOrder: sortInfo.field === 'domain' ? sortInfo.order : null,
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
      sorter: true,
      sortDirections: ['descend', 'ascend'],
      defaultSortOrder: sortInfo.field === 'time_ms' ? sortInfo.order : null,
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
      sorter: true,
      sortDirections: ['descend', 'ascend'],
      defaultSortOrder: sortInfo.field === 'speed_ms' ? sortInfo.order : null,
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
      width: 180,
      render: (groupName) => renderUpstreamInfo(groupName),
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
              name="group"
              label="所属上游"
              style={{ marginBottom: 8 }}
            >
              <Select placeholder="选择分组" allowClear>
                {groups.map((group) => (
                  <Option key={group.id} value={group.name}>
                    <Space>
                      <div
                        style={{
                          width: 12,
                          height: 12,
                          borderRadius: "50%",
                          backgroundColor: group.color,
                          display: "inline-block",
                        }}
                      />
                      {group.name}
                      <span style={{ color: "#999", fontSize: "12px" }}>
                        ({getServersByGroup(group.name).length}个服务器)
                      </span>
                    </Space>
                  </Option>
                ))}
              </Select>
            </Form.Item>
          </Col>
        </Row>

        {/* 时间范围选择行 */}
        <Row gutter={16}>
          <Col xs={24} sm={12} md={8}>
            <Form.Item
              name="time_range"
              label="时间范围"
              style={{ marginBottom: 8 }}
            >
              <RangePicker
                showTime={{
                  format: "HH:mm:ss",
                }}
                format="YYYY-MM-DD HH:mm:ss"
                style={{ width: "100%" }}
                placeholder={["开始时间", "结束时间"]}
                allowClear={true}
                value={currentTimeRange}
                onChange={handleTimeRangeChange}
              />
            </Form.Item>
          </Col>
          <Col xs={24} sm={12} md={16}>
            <Form.Item label="快速选择" style={{ marginBottom: 8 }}>
              <Space wrap>
                <Button
                  size="small"
                  onClick={() => handleQuickTimeSelect(1)}
                  type={selectedQuickRange === 1 ? "primary" : "dashed"}
                >
                  1分钟
                </Button>
                <Button
                  size="small"
                  onClick={() => handleQuickTimeSelect(3)}
                  type={selectedQuickRange === 3 ? "primary" : "dashed"}
                >
                  3分钟
                </Button>
                <Button
                  size="small"
                  onClick={() => handleQuickTimeSelect(5)}
                  type={selectedQuickRange === 5 ? "primary" : "dashed"}
                >
                  5分钟
                </Button>
                <Button
                  size="small"
                  onClick={() => handleQuickTimeSelect(10)}
                  type={selectedQuickRange === 10 ? "primary" : "dashed"}
                >
                  10分钟
                </Button>
                <Button
                  size="small"
                  onClick={() => handleQuickTimeSelect(30)}
                  type={selectedQuickRange === 30 ? "primary" : "dashed"}
                >
                  30分钟
                </Button>
              </Space>
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
              <Space>
                <Switch
                  checked={autoRefresh}
                  onChange={handleAutoRefreshChange}
                  checkedChildren={<SyncOutlined />}
                  unCheckedChildren={<PauseOutlined />}
                  size="small"
                />
                <span style={{ color: autoRefresh ? "#52c41a" : "#999" }}>
                  {autoRefresh ? "自动刷新已开启" : "自动刷新已关闭"}
                </span>
              </Space>
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
        scroll={{ x: 1500 }}
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
