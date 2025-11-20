import request from "../../utils/request";

export const getDashboardStats = () => request.get("/dashboard/stats");
export const getNodesHealth = () => request.get("/dashboard/health");
export const getSystemOverview = () => request.get("/dashboard/overview");