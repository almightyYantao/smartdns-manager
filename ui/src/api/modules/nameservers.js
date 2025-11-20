import request from "../../utils/request";

export const getNameservers = (params) => request.get("/nameservers", { params });
export const addNameserver = (data) => request.post("/nameservers", data);
export const updateNameserver = (id, data) => request.put(`/nameservers/${id}`, data);
export const deleteNameserver = (id) => request.delete(`/nameservers/${id}`);