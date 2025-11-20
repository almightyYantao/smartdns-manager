import request from "../../utils/request";

export const login = (data) => request.post("/login", data);
export const register = (data) => request.post("/register", data);
export const getCurrentUser = () => request.get("/user/current");