import request from "../../utils/request";

export const getDomainSets = (params) => request.get("/domain-sets", { params });
export const getDomainSet = (id) => request.get(`/domain-sets/${id}`);
export const addDomainSet = (data) => request.post("/domain-sets", data);
export const updateDomainSet = (id, data) => request.put(`/domain-sets/${id}`, data);
export const deleteDomainSet = (id) => request.delete(`/domain-sets/${id}`);
export const importDomainSetFile = (id, data) => request.post(`/domain-sets/${id}/import`, data);
export const exportDomainSet = (id) => request.get(`/domain-sets/${id}/export`);