import React from "react";
import { Button, Popover, Space, Tag, Tooltip } from "antd";
import { DownOutlined, GlobalOutlined } from "@ant-design/icons";

const ConfiguredUpstreams = ({ groupName, groupConfig, servers }) => {
  const content = (
    <div style={{ width: "min(320px, calc(100vw - 48px))" }}>
      <p style={{ color: "#595959", margin: "0 0 12px", fontSize: 12 }}>
        配置列表不代表此次查询的实际响应来源。
      </p>
      <ul style={{ listStyle: "none", padding: 0, margin: 0, maxHeight: 280, overflowY: "auto" }}>
        {servers.map((server) => (
          <li key={server.id || `${server.type}:${server.address}`} style={{ display: "flex", alignItems: "baseline", gap: 16, justifyContent: "space-between", padding: "6px 0" }}>
            <code style={{ fontSize: 12, overflowWrap: "anywhere", minWidth: 0 }}>{server.address}</code>
            <Tag style={{ margin: 0, flexShrink: 0 }}>{server.type?.toUpperCase() || "UDP"}</Tag>
          </li>
        ))}
      </ul>
    </div>
  );

  return (
    <Space size={4} style={{ whiteSpace: "nowrap" }}>
      <Tooltip title={groupConfig?.description || `日志分组：${groupName}`}>
        <Tag color={groupConfig?.color || "#1890ff"} style={{ margin: 0, display: "inline-flex", alignItems: "center", gap: 4 }}>
          <GlobalOutlined />
          <span style={{ maxWidth: 90, overflow: "hidden", textOverflow: "ellipsis" }}>{groupName}</span>
        </Tag>
      </Tooltip>
      {servers.length ? (
        <Popover title={`${groupName} · 当前配置上游`} content={content} trigger={["hover", "click"]} placement="bottomLeft">
          <Button type="text" size="small" aria-label={`${groupName}：查看 ${servers.length} 个配置上游`} style={{ color: "#595959", paddingInline: 6, fontSize: 12 }}>
            {servers.length} 个上游 <DownOutlined style={{ fontSize: 10 }} />
          </Button>
        </Popover>
      ) : (
        <span style={{ color: "#595959", fontSize: 12 }}>当前无配置</span>
      )}
    </Space>
  );
};

export default ConfiguredUpstreams;
