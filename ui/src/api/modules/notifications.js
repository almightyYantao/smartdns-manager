import request from "../../utils/request";

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