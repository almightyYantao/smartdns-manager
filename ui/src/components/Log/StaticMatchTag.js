import React from "react";
import { Tag, Tooltip } from "antd";
import { DesktopOutlined } from "@ant-design/icons";

export const STATIC_MATCH_GROUP = "__static_match__";

const StaticMatchTag = () => (
  <Tooltip title="采集时查询类型和返回 IP 与本节点静态映射一致；此标记是配置匹配推断，不代表已确认本地响应。原日志分组保留在原始日志中。">
    <Tag color="green" icon={<DesktopOutlined />} style={{ margin: 0 }}>
      静态映射匹配
    </Tag>
  </Tooltip>
);

export default StaticMatchTag;
