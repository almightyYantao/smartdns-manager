import request from "../utils/request";

// 认证相关
export const login = (data) => request.post("/login", data);
export const register = (data) => request.post("/register", data);
export const getCurrentUser = () => request.get("/user/current");

// 节点管理
export const getNodes = (params) => request.get("/nodes", { params });
export const addNode = (data) => request.post("/nodes", data);
export const updateNode = (id, data) => request.put(`/nodes/${id}`, data);
export const deleteNode = (id) => request.delete(`/nodes/${id}`);
export const testNodeConnection = (id) => request.post(`/nodes/${id}/test`);
export const getNodeStatus = (id) => request.get(`/nodes/${id}/status`);
export const getNodeLogs = (id, params) =>
  request.get(`/nodes/${id}/logs`, { params });

// 配置管理
export const getNodeConfig = (id) => request.get(`/nodes/${id}/config`);
export const saveNodeConfig = (id, data) =>
  request.post(`/nodes/${id}/config`, data);
export const restartNodeService = (id) => request.post(`/nodes/${id}/restart`);

// 批量操作
export const batchUpdateConfig = (data) =>
  request.post("/nodes/batch/config", data);
export const batchRestart = (data) =>
  request.post("/nodes/batch/restart", data);

// 备份管理
export const getNodeBackups = (id) => request.get(`/nodes/${id}/backups`);
export const createNodeBackup = (id) => request.post(`/nodes/${id}/backup`);
export const restoreNodeBackup = (id, data) =>
  request.post(`/nodes/${id}/restore`, data);

// 地址映射
export const getAddresses = (params) => request.get("/addresses", { params });
export const addAddress = (data) => request.post("/addresses", data);
export const updateAddress = (id, data) =>
  request.put(`/addresses/${id}`, data);
export const deleteAddress = (id) => request.delete(`/addresses/${id}`);
export const batchAddAddresses = (data) =>
  request.post("/addresses/batch", data);
export const importAddresses = (data) =>
  request.post("/addresses/import", data);

// DNS服务器
export const getServers = (params) => request.get("/servers", { params });
export const addServer = (data) => request.post("/servers", data);
export const updateServer = (id, data) => request.put(`/servers/${id}`, data);
export const deleteServer = (id) => request.delete(`/servers/${id}`);

// 仪表板
export const getDashboardStats = () => request.get("/dashboard/stats");
export const getNodesHealth = () => request.get("/dashboard/health");
export const getSystemOverview = () => request.get("/dashboard/overview");

// 配置同步
export const triggerFullSync = (id) => request.post(`/sync/node/${id}/full`);
export const batchFullSync = (data) => request.post("/sync/batch", data);
export const getSyncLogs = (params) => request.get("/sync/logs", { params });
export const getSyncStats = () => request.get("/sync/stats");
export const retrySyncLog = (id) => request.post(`/sync/logs/${id}/retry`);
export const clearSyncLogs = (data) => request.delete("/sync/logs", { data });

export const getNotificationChannels = (params) =>
  request.get("/notifications/channels", { params });
export const addNotificationChannel = (data) =>
  request.post("/notifications/channels", data);
export const updateNotificationChannel = (id, data) =>
  request.put(`/notifications/channels/${id}`, data);
export const deleteNotificationChannel = (id) =>
  request.delete(`/notifications/channels/${id}`);
export const testNotificationChannel = (id) =>
  request.post(`/notifications/channels/${id}/test`);
export const getNotificationLogs = (params) =>
  request.get("/notifications/logs", { params });

// 节点初始化
export const initNode = (id) => request.post(`/nodes/${id}/init`);
export const checkNodeInit = (id) => request.get(`/nodes/${id}/init/status`);
export const getInitLogs = (id) => request.get(`/nodes/${id}/init/logs`);
export const uninstallSmartDNS = (id) => request.post(`/nodes/${id}/uninstall`);
export const reinstallSmartDNS = (id) => request.post(`/nodes/${id}/reinstall`);

export default {
  login,
  register,
  getCurrentUser,
  getNodes,
  addNode,
  updateNode,
  deleteNode,
  testNodeConnection,
  getNodeStatus,
  getNodeLogs,
  getNodeConfig,
  saveNodeConfig,
  restartNodeService,
  batchUpdateConfig,
  batchRestart,
  getNodeBackups,
  createNodeBackup,
  restoreNodeBackup,
  getAddresses,
  addAddress,
  updateAddress,
  deleteAddress,
  batchAddAddresses,
  importAddresses,
  getServers,
  addServer,
  updateServer,
  deleteServer,
  getDashboardStats,
  getNodesHealth,
  getSystemOverview,
  triggerFullSync,
  batchFullSync,
  getSyncLogs,
  getSyncStats,
  retrySyncLog,
  clearSyncLogs,
  getNotificationChannels,
  addNotificationChannel,
  updateNotificationChannel,
  deleteNotificationChannel,
  testNotificationChannel,
  getNotificationLogs,
  initNode,
  checkNodeInit,
  getInitLogs,
  uninstallSmartDNS,
  reinstallSmartDNS,
};
