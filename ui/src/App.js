import React from "react";
import { BrowserRouter, Routes, Route, Navigate } from "react-router-dom";
import { ConfigProvider, App as AntApp } from "antd";
import zhCN from "antd/locale/zh_CN";
import moment from "moment";
import "moment/locale/zh-cn";
import "./App.css";

import { isAuthenticated } from "./utils/auth";
import Login from "./pages/Login";
import MainLayout from "./components/Layout/MainLayout";
import Dashboard from "./pages/Dashboard";
import Nodes from "./pages/Nodes";
import Addresses from "./pages/Addresses";
import Servers from "./pages/Servers";
import NodeConfig from "./pages/NodeConfig";
import Settings from "./pages/Settings";
import NotificationManager from './components/Notification/NotificationManager';

moment.locale("zh-cn");

// 受保护的路由组件
const ProtectedRoute = ({ children }) => {
  if (!isAuthenticated()) {
    return <Navigate to="/login" replace />;
  }
  return children;
};

function App() {
  return (
    <ConfigProvider locale={zhCN}>
      <AntApp>
        <BrowserRouter>
          <Routes>
            <Route path="/login" element={<Login />} />

            <Route
              path="/"
              element={
                <ProtectedRoute>
                  <MainLayout />
                </ProtectedRoute>
              }
            >
              <Route index element={<Dashboard />} />
              <Route path="nodes" element={<Nodes />} />
              <Route path="nodes/:id/config" element={<NodeConfig />} />
              <Route path="addresses" element={<Addresses />} />
              <Route path="servers" element={<Servers />} />
              <Route path="notifications" element={<NotificationManager />} />
              <Route path="settings" element={<Settings />} />
            </Route>

            <Route path="*" element={<Navigate to="/" replace />} />
          </Routes>
        </BrowserRouter>
      </AntApp>
    </ConfigProvider>
  );
}

export default App;
