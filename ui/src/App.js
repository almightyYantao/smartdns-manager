import React from "react";
import { BrowserRouter, Routes, Route, Navigate } from "react-router-dom";
import { ConfigProvider, App as AntApp, Card } from "antd";
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
import NotificationManager from "./components/Notification/NotificationManager";
import DomainSetManager from "./components/DomainSet/DomainSetManager";
import DomainRuleManager from "./components/DomainRule/DomainRuleManager";
import NameserverManager from "./components/Nameserver/NameserverManager";
import GroupManager from "./components/GroupManager/GroupManager";

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
              <Route
                path="domain-sets"
                element={
                  <Card title="域名集管理" bordered={false}>
                    <DomainSetManager />
                  </Card>
                }
              />
              <Route
                path="domain-rules"
                element={
                  <Card title="域名规则管理" bordered={false}>
                    <DomainRuleManager />
                  </Card>
                }
              />
              <Route
                path="nameservers"
                element={
                  <Card title="命名服务器管理" bordered={false}>
                    <NameserverManager />
                  </Card>
                }
              />
              <Route
                path="groups"
                element={
                  <Card title="分组管理" bordered={false}>
                    <GroupManager />
                  </Card>
                }
              />
            </Route>

            <Route path="*" element={<Navigate to="/" replace />} />
          </Routes>
        </BrowserRouter>
      </AntApp>
    </ConfigProvider>
  );
}

export default App;
