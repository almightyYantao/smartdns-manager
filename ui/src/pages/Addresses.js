import React from 'react';
import { Card } from 'antd';
import AddressManager from '../components/Config/AddressManager';

const Addresses = () => {
  return (
    <Card title="地址映射管理" bordered={false}>
      <AddressManager />
    </Card>
  );
};

export default Addresses;