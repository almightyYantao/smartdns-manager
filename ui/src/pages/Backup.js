import React from 'react';
import { Card } from 'antd';
import BackupManager from '../components/Backup/BackupManager';

const Backup = () => {
  return (
    <Card title="备份管理" bordered={false}>
      <BackupManager />
    </Card>
  );
};

export default Backup;