import React from 'react';
import { Card } from 'antd';
import NodeList from '../components/Node/NodeList';

const Nodes = () => {
  return (
    <Card title="节点管理" bordered={false}>
      <NodeList />
    </Card>
  );
};

export default Nodes;