import React, { useState, useEffect } from "react";
import {
  Card,
  Button,
  Space,
  Tag,
  Tooltip,
  Modal,
  message,
  Typography,
  Divider,
  Alert,
} from "antd";
import {
  CloudDownloadOutlined,
  DeleteOutlined,
  EyeOutlined,
  ReloadOutlined,
  CheckCircleOutlined,
  ExclamationCircleOutlined,
  CloseCircleOutlined,
} from "@ant-design/icons";
import AgentDeployModal from "./AgentDeployModal";
import AgentLogsModal from "./AgentLogsModal";
import {
  checkAgentStatus as checkAgentStatusApi,
  uninstallAgent,
} from "../../api";

const { Text, Paragraph } = Typography;

const AgentStatus = ({ node, onRefresh }) => {
  const [status, setStatus] = useState(null);
  const [loading, setLoading] = useState(false);
  const [deployModalVisible, setDeployModalVisible] = useState(false);
  const [logsModalVisible, setLogsModalVisible] = useState(false);

  useEffect(() => {
    if (node?.id) {
      checkAgentStatus();
    }
  }, [node?.id]);

  const checkAgentStatus = async () => {
    if (!node?.id) return;

    try {
      setLoading(true);
      const result = await checkAgentStatusApi(node.id);

      if (result.success) {
        setStatus(result.data);
      } else {
        setStatus({
          installed: false,
          running: false,
          error_message: result.message,
        });
      }
    } catch (error) {
      setStatus({
        installed: false,
        running: false,
        error_message: "检查状态失败: " + error.message,
      });
    } finally {
      setLoading(false);
    }
  };

  const handleUninstall = () => {
    Modal.confirm({
      title: "确认卸载",
      content: `确定要卸载节点 ${node.name} 上的 Agent 吗？此操作不可恢复。`,
      okText: "确定卸载",
      okType: "danger",
      cancelText: "取消",
      onOk: async () => {
        try {
          const result = await uninstallAgent(node.id);

          if (result.success) {
            message.success("Agent 卸载成功");
            checkAgentStatus();
            if (onRefresh) onRefresh();
          } else {
            message.error("卸载失败: " + result.message);
          }
        } catch (error) {
          message.error("卸载失败: " + error.message);
        }
      },
    });
  };

  const getStatusTag = () => {
    if (!status) return <Tag>检查中...</Tag>;

    if (!status.installed) {
      return <Tag color="default">未安装</Tag>;
    }

    if (status.running) {
      return (
        <Tag color="success" icon={<CheckCircleOutlined />}>
          运行中
        </Tag>
      );
    } else {
      return (
        <Tag color="error" icon={<CloseCircleOutlined />}>
          已停止
        </Tag>
      );
    }
  };

  return (
    <>
      <Space direction="vertical" style={{ width: "100%" }}>
        <div
          style={{
            display: "flex",
            justifyContent: "space-between",
            alignItems: "center",
          }}
        >
          <Text>状态：</Text>
          {getStatusTag()}
        </div>

        {status?.installed && (
          <>
            <div
              style={{
                display: "flex",
                justifyContent: "space-between",
                alignItems: "center",
              }}
            >
              <Text>部署模式：</Text>
              <Tag color="blue">{status.deploy_mode || "未知"}</Tag>
            </div>

            {status.version && (
              <div
                style={{
                  display: "flex",
                  justifyContent: "space-between",
                  alignItems: "center",
                }}
              >
                <Text>版本：</Text>
                <Text code>{status.version}</Text>
              </div>
            )}

            <div
              style={{
                display: "flex",
                justifyContent: "space-between",
                alignItems: "center",
              }}
            >
              <Text>最后检查：</Text>
              <Text type="secondary">{status.last_check}</Text>
            </div>
          </>
        )}

        {status?.error_message && (
          <Alert
            message="错误信息"
            description={status.error_message}
            type="error"
            size="small"
          />
        )}

        <Divider style={{ margin: "12px 0" }} />

        <Space wrap>
          {!status?.installed ? (
            <Button
              type="primary"
              size="small"
              icon={<CloudDownloadOutlined />}
              onClick={() => setDeployModalVisible(true)}
            >
              部署 Agent
            </Button>
          ) : (
            <>
              <Button
                size="small"
                icon={<EyeOutlined />}
                onClick={() => setLogsModalVisible(true)}
              >
                查看日志
              </Button>
              <Button
                size="small"
                danger
                icon={<DeleteOutlined />}
                onClick={handleUninstall}
              >
                卸载
              </Button>
            </>
          )}
        </Space>
      </Space>

      <AgentDeployModal
        visible={deployModalVisible}
        onCancel={() => setDeployModalVisible(false)}
        node={node}
        onSuccess={() => {
          setDeployModalVisible(false);
          checkAgentStatus();
          if (onRefresh) onRefresh();
        }}
      />

      <AgentLogsModal
        visible={logsModalVisible}
        onCancel={() => setLogsModalVisible(false)}
        node={node}
      />
    </>
  );
};

export default AgentStatus;
