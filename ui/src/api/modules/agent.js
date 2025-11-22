import request from '../../utils/request';

// 部署 Agent
export const deployAgent = (data) => {
  return request({
    url: `/nodes/${data.node_id}/agent/deploy`,
    method: 'POST',
    data,
  });
};

// 检查 Agent 状态
export const checkAgentStatus = (nodeId) => {
  return request({
    url: `/nodes/${nodeId}/agent/status`,
    method: 'GET',
  });
};

// 卸载 Agent
export const uninstallAgent = (nodeId) => {
  return request({
    url: `/nodes/${nodeId}/agent`,
    method: 'DELETE',
  });
};

// 获取 Agent 日志
export const getAgentLogs = (nodeId, params) => {
  return request({
    url: `/nodes/${nodeId}/agent/logs`,
    method: 'GET',
    params,
  });
};

// 重启 Agent
export const restartAgent = (nodeId) => {
  return request({
    url: `/nodes/${nodeId}/agent/restart`,
    method: 'POST',
  });
};

// 更新 Agent 配置
export const updateAgentConfig = (nodeId, config) => {
  return request({
    url: `/nodes/${nodeId}/agent/config`,
    method: 'PUT',
    data: config,
  });
};

export const startAgentCollection = (nodeHost, port = 8888) => {
  return request({
    url: `http://${nodeHost}:${port}/api/v1/start`,
    method: 'POST',
    timeout: 10000,
  });
};

export const stopAgentCollection = (nodeHost, port = 8888) => {
  return request({
    url: `http://${nodeHost}:${port}/api/v1/stop`,
    method: 'POST',
    timeout: 10000,
  });
};