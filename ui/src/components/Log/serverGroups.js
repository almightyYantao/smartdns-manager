// 日志中的 default 是隐式默认组；命名分组并不自动排除服务器的默认组成员资格。
export const getConfiguredServers = (servers, groupName, nodeId) => servers.filter((server) => {
  if (server.enabled === false) return false;
  try {
    const nodeIds = Array.isArray(server.node_ids)
      ? server.node_ids : JSON.parse(server.node_ids || "[]");
    if (!Array.isArray(nodeIds) || (nodeIds.length > 0 && !nodeIds.some((id) => String(id) === String(nodeId)))) {
      return false;
    }
  } catch {
    return false;
  }

  if (groupName === "default") {
    return !server.exclude_default && !/(?:^|\s)-(?:exclude-default-group|e)(?:\s|$)/.test(server.options || "");
  }
  return Array.isArray(server.groups) && server.groups.includes(groupName);
});
