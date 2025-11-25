import React, { useState, useEffect } from "react";
import {
  Modal,
  Form,
  Input,
  Select,
  Button,
  Steps,
  Alert,
  Spin,
  message,
  Typography,
  Space,
  Divider,
  InputNumber,
  Checkbox,
} from "antd";
import {
  CloudDownloadOutlined,
  CheckCircleOutlined,
  LoadingOutlined,
  ExclamationCircleOutlined,
} from "@ant-design/icons";
import { deployAgent } from "../../api";

const { Step } = Steps;
const { Option } = Select;
const { TextArea } = Input;
const { Text, Paragraph } = Typography;

const AgentDeployModal = ({ visible, onCancel, node, onSuccess }) => {
  const [form] = Form.useForm();
  const [currentStep, setCurrentStep] = useState(0);
  const [deploying, setDeploying] = useState(false);
  const [deployResult, setDeployResult] = useState(null);
  const [deployOutput, setDeployOutput] = useState([]);
  const [useProxy, setUseProxy] = useState(false);

  useEffect(() => {
    if (visible && node) {
      // 重置状态
      setCurrentStep(0);
      setDeployResult(null);
      setDeployOutput([]);
      setUseProxy(false);
      
      // 设置默认值
      form.setFieldsValue({
        deploy_mode: "systemd",
        clickhouse_port: 9000,
        clickhouse_db: "smartdns_logs",
        clickhouse_user: "default",
        log_file_path: "/var/log/audit/audit.log",
        batch_size: 1000,
        flush_interval: 2,
        proxy_type: "socks5",
        proxy_port: 1080,
      });
    }
  }, [visible, node, form]);

  const handleDeploy = async () => {
    try {
      const values = await form.validateFields();
      setDeploying(true);
      setCurrentStep(1);

      const deployData = {
        node_id: node.id,
        ...values,
      };

      // 如果不使用代理，清除代理相关字段
      if (!useProxy) {
        delete deployData.proxy_type;
        delete deployData.proxy_host;
        delete deployData.proxy_port;
        delete deployData.proxy_user;
        delete deployData.proxy_pass;
      }

      // 使用 API 方法替代 fetch
      const result = await deployAgent(deployData);
      if (result.success) {
        setDeployResult(result.data);
        setDeployOutput(result.data.output || []);
        setCurrentStep(2);
        message.success("Agent 部署成功！");
        if (onSuccess) onSuccess();
      } else {
        throw new Error(result.message);
      }
    } catch (error) {
      message.error("部署失败: " + error.message);
      setCurrentStep(0);
    } finally {
      setDeploying(false);
    }
  };

  const steps = [
    {
      title: "配置参数",
      icon: <CloudDownloadOutlined />,
    },
    {
      title: "部署中",
      icon: deploying ? <LoadingOutlined /> : <CloudDownloadOutlined />,
    },
    {
      title: "完成",
      icon: deployResult?.success ? (
        <CheckCircleOutlined />
      ) : (
        <ExclamationCircleOutlined />
      ),
    },
  ];

  return (
    <Modal
      title={`部署 SmartDNS Log Agent - ${node?.name}`}
      open={visible}
      onCancel={onCancel}
      width={800}
      footer={null}
      maskClosable={!deploying}
      closable={!deploying}
    >
      <Steps current={currentStep} items={steps} style={{ marginBottom: 24 }} />

      {/* 步骤 0: 参数配置 */}
      {currentStep === 0 && (
        <>
          <Alert
            message="部署说明"
            description="将在目标节点自动下载并安装 SmartDNS Log Agent，用于实时收集 DNS 查询日志"
            type="info"
            showIcon
            style={{ marginBottom: 16 }}
          />
          <Form form={form} layout="vertical">
            <Form.Item
              name="deploy_mode"
              label="部署模式"
              rules={[{ required: true, message: "请选择部署模式" }]}
            >
              <Select>
                <Option value="systemd">systemd 服务</Option>
                <Option value="docker">Docker 容器</Option>
              </Select>
            </Form.Item>

            <Divider orientation="left">代理配置</Divider>
            <Form.Item>
              <Checkbox
                checked={useProxy}
                onChange={(e) => setUseProxy(e.target.checked)}
              >
                使用 SOCKS5 代理连接
              </Checkbox>
            </Form.Item>

            {useProxy && (
              <>
                <Alert
                  message="代理说明"
                  description="如果目标节点需要通过代理访问，请配置SOCKS5代理信息"
                  type="warning"
                  showIcon
                  style={{ marginBottom: 16 }}
                />
                
                <Form.Item
                  name="proxy_host"
                  label="代理主机"
                  rules={useProxy ? [{ required: true, message: "请输入代理主机地址" }] : []}
                >
                  <Input placeholder="例如: 127.0.0.1 或 proxy.company.com" />
                </Form.Item>

                <Space.Compact style={{ width: "100%" }}>
                  <Form.Item
                    name="proxy_port"
                    label="代理端口"
                    style={{ width: "30%" }}
                    rules={useProxy ? [{ required: true, message: "请输入代理端口" }] : []}
                  >
                    <InputNumber 
                      min={1} 
                      max={65535} 
                      style={{ width: "100%" }} 
                      placeholder="1080"
                    />
                  </Form.Item>
                  <Form.Item
                    name="proxy_user"
                    label="代理用户名"
                    style={{ width: "35%", marginLeft: 8 }}
                  >
                    <Input placeholder="用户名（可选）" />
                  </Form.Item>
                  <Form.Item
                    name="proxy_pass"
                    label="代理密码"
                    style={{ width: "35%", marginLeft: 8 }}
                  >
                    <Input.Password placeholder="密码（可选）" />
                  </Form.Item>
                </Space.Compact>
              </>
            )}

            <Divider orientation="left">ClickHouse 配置</Divider>
            <Form.Item
              name="clickhouse_host"
              label="ClickHouse 主机"
              rules={[
                { required: true, message: "请输入 ClickHouse 主机地址" },
              ]}
            >
              <Input placeholder="例如: 192.168.1.100" />
            </Form.Item>

            <Space.Compact style={{ width: "100%" }}>
              <Form.Item
                name="clickhouse_port"
                label="端口"
                style={{ width: "30%" }}
              >
                <InputNumber min={1} max={65535} style={{ width: "100%" }} />
              </Form.Item>
              <Form.Item
                name="clickhouse_db"
                label="数据库"
                style={{ width: "35%", marginLeft: 8 }}
              >
                <Input />
              </Form.Item>
              <Form.Item
                name="clickhouse_user"
                label="用户名"
                style={{ width: "35%", marginLeft: 8 }}
              >
                <Input />
              </Form.Item>
            </Space.Compact>

            <Form.Item name="clickhouse_password" label="密码">
              <Input.Password placeholder="ClickHouse 密码（可选）" />
            </Form.Item>

            <Divider orientation="left">Agent 配置</Divider>
            <Form.Item
              name="log_file_path"
              label="日志文件路径"
              rules={[{ required: true, message: "请输入日志文件路径" }]}
            >
              <Input placeholder="/var/log/smartdns/audit.log" />
            </Form.Item>

            <Space.Compact style={{ width: "100%" }}>
              <Form.Item
                name="batch_size"
                label="批量大小"
                style={{ width: "50%" }}
              >
                <InputNumber min={100} max={10000} style={{ width: "100%" }} />
              </Form.Item>
              <Form.Item
                name="flush_interval"
                label="刷新间隔(秒)"
                style={{ width: "50%", marginLeft: 8 }}
              >
                <InputNumber min={1} max={60} style={{ width: "100%" }} />
              </Form.Item>
            </Space.Compact>
          </Form>

          <div style={{ textAlign: "right", marginTop: 16 }}>
            <Space>
              <Button onClick={onCancel}>取消</Button>
              <Button type="primary" onClick={handleDeploy}>
                开始部署
              </Button>
            </Space>
          </div>
        </>
      )}

      {/* 步骤 1: 部署中 */}
      {currentStep === 1 && (
        <div style={{ textAlign: "center", padding: "60px 0" }}>
          <Spin size="large" />
          <div style={{ marginTop: 16 }}>
            <Text>正在部署 Agent 到节点 {node?.name}</Text>
            <br />
            {useProxy && (
              <>
                <Text type="secondary">通过 SOCKS5 代理连接中...</Text>
                <br />
              </>
            )}
            <Text type="secondary">这可能需要几分钟时间，请耐心等待...</Text>
          </div>
        </div>
      )}

      {/* 步骤 2: 完成 */}
      {currentStep === 2 && deployResult && (
        <>
          <Alert
            message={deployResult.success ? "部署成功" : "部署失败"}
            description={deployResult.message}
            type={deployResult.success ? "success" : "error"}
            showIcon
            style={{ marginBottom: 16 }}
          />

          {deployResult.success && (
            <div style={{ marginBottom: 16 }}>
              <Space direction="vertical" style={{ width: "100%" }}>
                <div>
                  <Text strong>安装路径：</Text>
                  <Text code>{deployResult.install_path}</Text>
                </div>
                <div>
                  <Text strong>服务名称：</Text>
                  <Text code>{deployResult.service_name}</Text>
                </div>
                <div>
                  <Text strong>配置文件：</Text>
                  <Text code>{deployResult.config_path}</Text>
                </div>
              </Space>
            </div>
          )}

          {deployOutput.length > 0 && (
            <>
              <Divider orientation="left">部署日志</Divider>
              <div
                style={{
                  background: "#f5f5f5",
                  padding: 12,
                  borderRadius: 4,
                  maxHeight: 300,
                  overflow: "auto",
                }}
              >
                {deployOutput.map((line, index) => (
                  <div
                    key={index}
                    style={{ fontSize: 12, fontFamily: "monospace" }}
                  >
                    {line}
                  </div>
                ))}
              </div>
            </>
          )}

          <div style={{ textAlign: "right", marginTop: 16 }}>
            <Button type="primary" onClick={onCancel}>
              完成
            </Button>
          </div>
        </>
      )}
    </Modal>
  );
};

export default AgentDeployModal;