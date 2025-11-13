import React, { useState } from "react";
import { Layout, Menu, Avatar, Dropdown, Badge, Space } from "antd";
import {
  DashboardOutlined,
  CloudServerOutlined,
  GlobalOutlined,
  SettingOutlined,
  LogoutOutlined,
  UserOutlined,
  BellOutlined,
} from "@ant-design/icons";
import { Outlet, useNavigate, useLocation } from "react-router-dom";
import { removeToken, removeUserInfo, getUserInfo } from "../../utils/auth";
import "./MainLayout.css";

const { Header, Sider, Content } = Layout;

const MainLayout = () => {
  const [collapsed, setCollapsed] = useState(false);
  const navigate = useNavigate();
  const location = useLocation();
  const userInfo = getUserInfo();

  const menuItems = [
    {
      key: "/",
      icon: <DashboardOutlined />,
      label: "仪表板",
    },
    {
      key: "/nodes",
      icon: <CloudServerOutlined />,
      label: "节点管理",
    },
    {
      key: "/addresses",
      icon: <GlobalOutlined />,
      label: "地址映射",
    },
    {
      key: "/servers",
      icon: <SettingOutlined />,
      label: "DNS服务器",
    },
    {
      key: "/notifications", // 新增
      icon: <BellOutlined />,
      label: "通知管理",
    },
    {
      key: "/settings",
      icon: <SettingOutlined />,
      label: "系统设置",
    },
  ];

  const handleMenuClick = ({ key }) => {
    navigate(key);
  };

  const handleLogout = () => {
    removeToken();
    removeUserInfo();
    navigate("/login");
  };

  const userMenuItems = [
    {
      key: "profile",
      icon: <UserOutlined />,
      label: "个人信息",
      onClick: () => navigate("/profile"),
    },
    {
      type: "divider",
    },
    {
      key: "logout",
      icon: <LogoutOutlined />,
      label: "退出登录",
      onClick: handleLogout,
    },
  ];

  return (
    <Layout style={{ minHeight: "100vh" }}>
      <Sider
        collapsible
        collapsed={collapsed}
        onCollapse={setCollapsed}
        theme="dark"
      >
        <div className="logo">{collapsed ? "SD" : "SmartDNS"}</div>
        <Menu
          theme="dark"
          mode="inline"
          selectedKeys={[location.pathname]}
          items={menuItems}
          onClick={handleMenuClick}
        />
      </Sider>
      <Layout>
        <Header className="site-layout-header">
          <div className="header-content">
            <div className="header-title">SmartDNS 管理系统</div>
            <Space size="large">
              <Badge count={0} showZero={false}>
                <BellOutlined style={{ fontSize: "20px", cursor: "pointer" }} />
              </Badge>
              <Dropdown menu={{ items: userMenuItems }} placement="bottomRight">
                <Space style={{ cursor: "pointer" }}>
                  <Avatar icon={<UserOutlined />} />
                  <span>{userInfo?.username || "用户"}</span>
                </Space>
              </Dropdown>
            </Space>
          </div>
        </Header>
        <Content className="site-layout-content">
          <div className="content-wrapper">
            <Outlet />
          </div>
        </Content>
      </Layout>
    </Layout>
  );
};

export default MainLayout;
