import React, { useState } from "react";
import { Layout, Menu, Avatar, Dropdown, Badge, Space, Button, Drawer } from "antd";
import {
  DashboardOutlined,
  CloudServerOutlined,
  SettingOutlined,
  LogoutOutlined,
  UserOutlined,
  BellOutlined,
  MenuOutlined,
  MonitorOutlined,
} from "@ant-design/icons";
import { Outlet, useNavigate, useLocation } from "react-router-dom";
import { removeToken, removeUserInfo, getUserInfo } from "../../utils/auth";
import useMobile from "../../utils/useMobile";
import "./MainLayout.css";

const { Header, Sider, Content } = Layout;

const MainLayout = () => {
  const [collapsed, setCollapsed] = useState(false);
  const [menuOpen, setMenuOpen] = useState(false);
  const mobile = useMobile();
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
      key: "services",
      icon: <CloudServerOutlined />,
      label: "服务管理",
      children: [
        {
          key: "/nodes",
          label: "节点管理",
        },
        {
          key: "/upgrades",
          label: "升级管理",
        },
        {
          key: "/backup",
          label: "备份管理",
        },
        {
          key: "/logs",
          label: "日志管理",
        },
        {
          key: "/tasks",
          label: "任务管理",
        },
      ],
    },
    {
      key: "/telemetry",
      icon: <MonitorOutlined />,
      label: "网络遥测",
    },
    {
      key: "config",
      icon: <SettingOutlined />,
      label: "配置管理",
      children: [
        {
          key: "/groups",
          label: "分组管理",
        },
        {
          key: "/servers",
          label: "上游管理",
        },
        {
          key: "/addresses",
          label: "地址映射",
        },
        {
          key: "/domain-sets",
          label: "域名集",
        },
        {
          key: "/domain-rules",
          label: "域名规则",
        },
        {
          key: "/nameservers",
          label: "命名服务器",
        },
      ],
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
    setMenuOpen(false);
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

  const menu = (theme, items) => (
    <Menu theme={theme} mode="inline" selectedKeys={[location.pathname]}
      defaultOpenKeys={location.pathname === "/upgrades" ? ["services"] : undefined}
      items={items} onClick={handleMenuClick} />
  );

  const mobileMenuItems = menuItems.flatMap((item) => item.key === "services" ? [
    item.children.find(({ key }) => key === "/upgrades"),
    { ...item, children: item.children.filter(({ key }) => key !== "/upgrades") },
  ] : [item]);

  return (
    <Layout style={{ minHeight: "100vh" }}>
      {mobile ? (
        <Drawer title="SmartDNS 导航" placement="left" width="min(320px, 100vw)"
          open={menuOpen} onClose={() => setMenuOpen(false)} styles={{ body: { padding: 0 } }}>
          {menu("light", mobileMenuItems)}
        </Drawer>
      ) : (
        <Sider collapsible collapsed={collapsed} onCollapse={setCollapsed} theme="dark">
          <div className="logo">{collapsed ? "SD" : "SmartDNS"}</div>
          {menu("dark", menuItems)}
        </Sider>
      )}
      <Layout>
        <Header className="site-layout-header">
          <div className="header-content">
            <div className="header-brand">
              {mobile && <Button type="text" icon={<MenuOutlined />} aria-label="打开导航"
                aria-expanded={menuOpen} onClick={() => setMenuOpen(true)} />}
              <div className="header-title">{mobile ? "SmartDNS" : "SmartDNS 管理系统"}</div>
            </div>
            <Space size="large">
              {!mobile && <Badge count={0} showZero={false}>
                <BellOutlined style={{ fontSize: "20px", cursor: "pointer" }} />
              </Badge>}
              <Dropdown menu={{ items: userMenuItems }} trigger={["click"]} placement="bottomRight">
                <Button type="text" className="account-button" aria-label="账户菜单">
                  <Avatar size="small" icon={<UserOutlined />} />
                  {!mobile && <span>{userInfo?.username || "用户"}</span>}
                </Button>
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
