import request from "../../utils/request";

export const getServers = (params) => request.get("/servers", { params });
export const addServer = (data) => request.post("/servers", data);
export const updateServer = (id, data) => request.put(`/servers/${id}`, data);
export const deleteServer = (id) => request.delete(`/servers/${id}`);