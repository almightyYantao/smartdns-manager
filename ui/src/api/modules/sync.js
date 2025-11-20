import request from "../../utils/request";

export const triggerFullSync = (id) => request.post(`/sync/node/${id}/full`);
export const batchFullSync = (data) => request.post("/sync/batch", data);
export const getSyncLogs = (params) => request.get("/sync/logs", { params });
export const getSyncStats = () => request.get("/sync/stats");
export const retrySyncLog = (id) => request.post(`/sync/logs/${id}/retry`);
export const clearSyncLogs = (data) => request.delete("/sync/logs", { data });