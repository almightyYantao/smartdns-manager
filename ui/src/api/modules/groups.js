import request from "../../utils/request";

export const getGroups = (params) => request.get("/groups", { params });
export const addGroup = (data) => request.post("/groups", data);
export const updateGroup = (id, data) => request.put(`/groups/${id}`, data);
export const deleteGroup = (id) => request.delete(`/groups/${id}`);