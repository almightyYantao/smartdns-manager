import request from "../../utils/request";

export const getDomainRules = (params) => request.get("/domain-rules", { params });
export const addDomainRule = (data) => request.post("/domain-rules", data);
export const updateDomainRule = (id, data) => request.put(`/domain-rules/${id}`, data);
export const deleteDomainRule = (id) => request.delete(`/domain-rules/${id}`);