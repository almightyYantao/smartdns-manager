import React, { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import {
  Card,
  Tabs,
  Button,
  Space,
  message,
  Modal,
  Spin,
  Alert,
  Descriptions,
} from 'antd';
import {
  SaveOutlined,
  ReloadOutlined,
  ArrowLeftOutlined,
  HistoryOutlined,
  CodeOutlined,
} from '@ant-design/icons';
import MonacoEditor from '@monaco-editor/react';
import {
  getNodeConfig,
  saveNodeConfig,
  restartNodeService,
  getNodeBackups,
  restoreNodeBackup,
  getNodes,
} from '../api';
import ServerManager from '../components/Config/ServerManager';
import AddressManager from '../components/Config/AddressManager';

const NodeConfig = () => {
  const { id } = useParams();
  const navigate = useNavigate();
  const [node, setNode] = useState(null);
  const [config, setConfig] = useState(null);
  const [rawContent, setRawContent] = useState('');
  const [backups, setBackups] = useState([]);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [activeTab, setActiveTab] = useState('editor');

  useEffect(() => {
    loadNodeInfo();
    loadConfig();
    loadBackups();
  }, [id]);

  const loadNodeInfo = async () => {
    try {
      const response = await getNodes();
      const currentNode = response.data.find((n) => n.id === parseInt(id));
      setNode(currentNode);
    } catch (error) {
      message.error('加载节点信息失败');
    }
  };

  const loadConfig = async () => {
    try {
      setLoading(true);
      const response = await getNodeConfig(id);
      setConfig(response.data);
      setRawContent(response.rawContent);
    } catch (error) {
      message.error('加载配置失败');
    } finally {
      setLoading(false);
    }
  };

  const loadBackups = async () => {
    try {
      const response = await getNodeBackups(id);
      setBackups(response.data || []);
    } catch (error) {
      console.error('加载备份列表失败', error);
    }
  };

  const handleSave = async () => {
    console.log('handleSave');
    Modal.confirm({
      title: '确认保存配置',
      content: '保存配置会创建备份，但不会重启服务。确定要继续吗？',
      okText: '确定',
      cancelText: '取消',
      onOk: async () => {
        try {
          setSaving(true);
          await saveNodeConfig(id, {
            config,
            raw_content: rawContent,
          });
          message.success('配置保存成功');
          loadBackups();
        } catch (error) {
          message.error('配置保存失败');
        } finally {
          setSaving(false);
        }
      },
    });
  };

  const handleRestart = async () => {
    Modal.confirm({
      title: '确认重启服务',
      content: '重启 SmartDNS 服务会短暂中断 DNS 解析。确定要继续吗？',
      okText: '确定',
      cancelText: '取消',
      okType: 'danger',
      onOk: async () => {
        try {
          message.loading({ content: '正在重启服务...', key: 'restart' });
          await restartNodeService(id);
          message.success({ content: '服务重启成功', key: 'restart' });
        } catch (error) {
          message.error({ content: '服务重启失败', key: 'restart' });
        }
      },
    });
  };

  const handleRestore = (backupPath) => {
    Modal.confirm({
      title: '确认恢复备份',
      content: `确定要恢复到此备份吗？当前配置将被备份。`,
      okText: '确定',
      cancelText: '取消',
      onOk: async () => {
        try {
          await restoreNodeBackup(id, { backup_path: backupPath });
          message.success('配置恢复成功');
          loadConfig();
          loadBackups();
        } catch (error) {
          message.error('配置恢复失败');
        }
      },
    });
  };

  const editorTab = (
    <Spin spinning={loading}>
      <Space direction="vertical" style={{ width: '100%' }} size="large">
        <Alert
          message="配置编辑器"
          description="直接编辑 SmartDNS 配置文件。保存后不会自动重启服务，需要手动重启。"
          type="info"
          showIcon
        />
        
        <MonacoEditor
          height="600px"
          language="ini"
          theme="vs-dark"
          value={rawContent}
          onChange={(value) => setRawContent(value)}
          options={{
            minimap: { enabled: false },
            fontSize: 14,
            lineNumbers: 'on',
            scrollBeyondLastLine: false,
            automaticLayout: true,
            tabSize: 2,
          }}
        />

        <Space>
          <Button
            type="primary"
            icon={<SaveOutlined />}
            onClick={handleSave}
            loading={saving}
          >
            保存配置
          </Button>
          <Button
            icon={<ReloadOutlined />}
            onClick={handleRestart}
          >
            重启服务
          </Button>
          <Button onClick={loadConfig}>
            重新加载
          </Button>
        </Space>
      </Space>
    </Spin>
  );

  const serversTab = (
    <Space direction="vertical" style={{ width: '100%' }} size="large">
      <Alert
        message="DNS服务器管理"
        description="管理此节点的上游DNS服务器配置。"
        type="info"
        showIcon
      />
      <ServerManager />
    </Space>
  );

  const addressesTab = (
    <Space direction="vertical" style={{ width: '100%' }} size="large">
      <Alert
        message="地址映射管理"
        description="管理此节点的域名到IP地址的映射规则。"
        type="info"
        showIcon
      />
      <AddressManager />
    </Space>
  );

  const backupsTab = (
    <Space direction="vertical" style={{ width: '100%' }} size="large">
      <Alert
        message="备份管理"
        description="查看和恢复配置文件的历史备份。"
        type="info"
        showIcon
      />
      
      <Card>
        <Space direction="vertical" style={{ width: '100%' }}>
          {backups.length > 0 ? (
            backups.map((backup, index) => (
              <Card
                key={index}
                size="small"
                extra={
                  <Button
                    type="link"
                    onClick={() => handleRestore(backup)}
                  >
                    恢复
                  </Button>
                }
              >
                <code>{backup}</code>
              </Card>
            ))
          ) : (
            <div style={{ textAlign: 'center', padding: '40px 0', color: '#999' }}>
              暂无备份
            </div>
          )}
        </Space>
      </Card>
    </Space>
  );

  return (
    <div>
      <Card
        title={
          <Space>
            <Button
              type="text"
              icon={<ArrowLeftOutlined />}
              onClick={() => navigate('/nodes')}
            />
            <span>节点配置管理</span>
            {node && <span style={{ color: '#999' }}>- {node.name}</span>}
          </Space>
        }
        extra={
          node && (
            <Descriptions size="small" column={3}>
              <Descriptions.Item label="主机">{node.host}</Descriptions.Item>
              <Descriptions.Item label="端口">{node.port}</Descriptions.Item>
              <Descriptions.Item label="状态">
                {node.status === 'online' ? (
                  <span style={{ color: '#52c41a' }}>● 在线</span>
                ) : (
                  <span style={{ color: '#f5222d' }}>● 离线</span>
                )}
              </Descriptions.Item>
            </Descriptions>
          )
        }
      >
        <Tabs
          activeKey={activeTab}
          onChange={setActiveTab}
          items={[
            {
              key: 'editor',
              label: (
                <span>
                  <CodeOutlined />
                  配置编辑器
                </span>
              ),
              children: editorTab,
            },
            {
              key: 'servers',
              label: 'DNS服务器',
              children: serversTab,
            },
            {
              key: 'addresses',
              label: '地址映射',
              children: addressesTab,
            },
          ]}
        />
      </Card>
    </div>
  );
};

export default NodeConfig;