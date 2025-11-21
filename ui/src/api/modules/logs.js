import request from '../../utils/request';

// 启动节点日志监控
export const startNodeLogMonitor = (nodeId) => {
  return request({
    url: `/dns-logs/${nodeId}/log-monitor/start`,
    method: 'POST',
  });
};

// 停止节点日志监控
export const stopNodeLogMonitor = (nodeId) => {
  return request({
    url: `/dns-logs/${nodeId}/log-monitor/stop`,
    method: 'POST',
  });
};

// 获取节点监控状态
export const getNodeLogMonitorStatus = (nodeId) => {
  return request({
    url: `/dns-logs/${nodeId}/log-monitor/status`,
    method: 'GET',
  });
};

// 获取DNS日志列表
export const getDNSLogs = (params) => {
  return request({
    url: '/dns-logs',
    method: 'GET',
    params,
  });
};

// 获取节点日志统计
export const getNodeLogStats = (nodeId, params) => {
  return request({
    url: `/dns-logs/${nodeId}/logs/stats`,
    method: 'GET',
    params,
  });
};

// 清理节点旧日志
export const cleanNodeLogs = (nodeId, days) => {
  return request({
    url: `/dns-logs/${nodeId}/logs/clean`,
    method: 'POST',
    params: { days },
  });
};

// 搜索域名
export const searchDomain = (keyword, limit = 10) => {
  return request({
    url: '/dns-logs/dns-logs/search',
    method: 'GET',
    params: { keyword, limit },
  });
};