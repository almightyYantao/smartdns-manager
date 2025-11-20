import request from "../../utils/request";

export const getNodes = (params) => request.get("/nodes", { params });
export const addNode = (data) => request.post("/nodes", data);
export const updateNode = (id, data) => request.put(`/nodes/${id}`, data);
export const deleteNode = (id) => request.delete(`/nodes/${id}`);
export const testNodeConnection = (id) => request.post(`/nodes/${id}/test`);
export const getNodeStatus = (id) => request.get(`/nodes/${id}/status`);
export const getNodeLogs = (id, params) => request.get(`/nodes/${id}/logs`, { params });
export const restartNodeService = (id) => request.post(`/nodes/${id}/restart`);

// 配置
export const getNodeConfig = (id) => request.get(`/nodes/${id}/config`);
export const saveNodeConfig = (id, data) => request.post(`/nodes/${id}/config`, data);

// 批量
export const batchUpdateConfig = (data) => request.post("/nodes/batch/config", data);
export const batchRestart = (data) => request.post("/nodes/batch/restart", data);

// 初始化
export const initNode = (id) => request.post(`/nodes/${id}/init`);
export const checkNodeInit = (id) => request.get(`/nodes/${id}/init/status`);
export const getInitLogs = (id) => request.get(`/nodes/${id}/init/logs`);
export const uninstallSmartDNS = (id) => request.post(`/nodes/${id}/uninstall`);
export const reinstallSmartDNS = (id) => request.post(`/nodes/${id}/reinstall`);