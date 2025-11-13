import React from 'react';
import { Card } from 'antd';
import ServerManager from '../components/Config/ServerManager';

const Servers = () => {
  return (
    <Card title="DNS服务器管理" bordered={false}>
      <ServerManager />
    </Card>
  );
};

export default Servers;