import request from "../../utils/request";

export const getNodeBackups = (id, params) => 
  request.get(`/nodes/${id}/backups`, { params });

export const createNodeBackup = (id, data = {}) => 
  request.post(`/nodes/${id}/backups`, data);

export const previewBackup = (nodeId, data) => 
  request.post(`/nodes/${nodeId}/backups/preview`, data);

export const restoreNodeBackup = (nodeId, data) =>
  request.post(`/nodes/${nodeId}/backups/restore`, data);

export const deleteNodeBackup = (nodeId, data) =>
  request.delete(`/nodes/${nodeId}/backups`, { data });

export const downloadBackup = (nodeId, backupId) => {
  const url = `/nodes/${nodeId}/backups/download?backup_id=${backupId}`;
  return request.get(url, { responseType: "blob" });
};