import request from "../../utils/request";

// 获取备份配置列表
export const getBackupConfigs = (params = {}) => {
  return request({
    url: '/database-backup/configs',
    method: 'get',
    params
  });
};

// 获取单个备份配置
export const getBackupConfig = (id) => {
  return request({
    url: `/database-backup/configs/${id}`,
    method: 'get'
  });
};

// 创建备份配置
export const createBackupConfig = (data) => {
  return request({
    url: '/database-backup/configs',
    method: 'post',
    data
  });
};

// 更新备份配置
export const updateBackupConfig = (id, data) => {
  return request({
    url: `/database-backup/configs/${id}`,
    method: 'put',
    data
  });
};

// 删除备份配置
export const deleteBackupConfig = (id) => {
  return request({
    url: `/database-backup/configs/${id}`,
    method: 'delete'
  });
};

// 手动触发备份
export const triggerManualBackup = (id) => {
  return request({
    url: `/database-backup/configs/${id}/backup`,
    method: 'post'
  });
};

// 获取备份历史
export const getBackupHistory = (params = {}) => {
  return request({
    url: '/database-backup/history',
    method: 'get',
    params
  });
};

// 恢复备份
export const restoreBackup = (data) => {
  return request({
    url: '/database-backup/restore',
    method: 'post',
    data
  });
};

// 获取备份统计信息
export const getBackupStats = () => {
  return request({
    url: '/database-backup/stats',
    method: 'get'
  });
};

// 测试S3连接
export const testS3Connection = (data) => {
  return request({
    url: '/database-backup/test-s3',
    method: 'post',
    data
  });
};