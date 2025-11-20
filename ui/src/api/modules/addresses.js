import request from "../../utils/request";

export const getAddresses = (params) => request.get("/addresses", { params });
export const addAddress = (data) => request.post("/addresses", data);
export const updateAddress = (id, data) => request.put(`/addresses/${id}`, data);
export const deleteAddress = (id) => request.delete(`/addresses/${id}`);
export const batchAddAddresses = (data) => request.post("/addresses/batch", data);
export const importAddresses = (data) => request.post("/addresses/import", data);