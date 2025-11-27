import React, { useState, useEffect } from 'react';
import { Row, Col, Select, InputNumber, Checkbox, Space, Typography, Alert, Input } from 'antd';

const { Option } = Select;
const { Text } = Typography;

// Cron表达式构建器组件
const CronBuilder = ({ value, onChange }) => {
  const [cronType, setCronType] = useState('simple');
  const [simpleConfig, setSimpleConfig] = useState({
    type: 'daily',
    hour: 2,
    minute: 0,
    weekdays: [],
    dayOfMonth: 1
  });
  const [initialized, setInitialized] = useState(false);

  // 预设的常用Cron表达式
  const presets = {
    'every-minute': { expr: '0 * * * * ?', desc: '每分钟执行' },
    'every-5-minutes': { expr: '0 */5 * * * ?', desc: '每5分钟执行' },
    'every-10-minutes': { expr: '0 */10 * * * ?', desc: '每10分钟执行' },
    'every-30-minutes': { expr: '0 */30 * * * ?', desc: '每30分钟执行' },
    'every-hour': { expr: '0 0 * * * ?', desc: '每小时执行' },
    'every-2-hours': { expr: '0 0 */2 * * ?', desc: '每2小时执行' },
    'every-6-hours': { expr: '0 0 */6 * * ?', desc: '每6小时执行' },
    'daily-2am': { expr: '0 0 2 * * ?', desc: '每天凌晨2点执行' },
    'daily-midnight': { expr: '0 0 0 * * ?', desc: '每天午夜执行' },
    'weekly-sunday-2am': { expr: '0 0 2 ? * SUN', desc: '每周日凌晨2点执行' },
    'monthly-1st-2am': { expr: '0 0 2 1 * ?', desc: '每月1号凌晨2点执行' }
  };

  const weekDays = [
    { label: '周一', value: 'MON' },
    { label: '周二', value: 'TUE' },
    { label: '周三', value: 'WED' },
    { label: '周四', value: 'THU' },
    { label: '周五', value: 'FRI' },
    { label: '周六', value: 'SAT' },
    { label: '周日', value: 'SUN' }
  ];

  // 生成简单配置的Cron表达式
  const generateSimpleCron = (config) => {
    const { type, hour, minute, weekdays, dayOfMonth } = config;
    
    switch (type) {
      case 'daily':
        return `0 ${minute} ${hour} * * ?`;
      case 'weekly':
        if (weekdays.length === 0) return '0 0 2 ? * SUN';
        return `0 ${minute} ${hour} ? * ${weekdays.join(',')}`;
      case 'monthly':
        return `0 ${minute} ${hour} ${dayOfMonth} * ?`;
      default:
        return '0 0 2 * * ?';
    }
  };

  // 解析Cron表达式到可读描述
  const parseCronDescription = (cronExpr) => {
    if (!cronExpr) return '';
    
    // 检查是否是预设表达式
    const preset = Object.entries(presets).find(([key, preset]) => preset.expr === cronExpr);
    if (preset) {
      return preset[1].desc;
    }

    const parts = cronExpr.split(' ');
    if (parts.length !== 6) return '自定义表达式';

    const [second, minute, hour, dayOfMonth, month, dayOfWeek] = parts;
    
    try {
      let description = '执行时间: ';
      
      // 处理时间部分
      if (hour !== '*' && minute !== '*') {
        description += `${hour}:${minute.padStart(2, '0')}`;
      } else if (hour !== '*') {
        description += `每小时的${hour}点`;
      } else if (minute !== '*') {
        description += `每分钟的${minute}秒`;
      }

      // 处理日期部分
      if (dayOfWeek !== '?') {
        const weekMap = { 'SUN': '周日', 'MON': '周一', 'TUE': '周二', 'WED': '周三', 'THU': '周四', 'FRI': '周五', 'SAT': '周六' };
        if (weekMap[dayOfWeek]) {
          description += ` (${weekMap[dayOfWeek]})`;
        } else {
          description += ` (每周)`;
        }
      } else if (dayOfMonth !== '*') {
        description += ` (每月${dayOfMonth}号)`;
      } else {
        description += ` (每天)`;
      }

      return description;
    } catch (error) {
      return '自定义表达式';
    }
  };

  // 解析现有Cron表达式到简单配置
  const parseCronToSimpleConfig = (cronExpr) => {
    if (!cronExpr) return null;
    
    const parts = cronExpr.split(' ');
    if (parts.length !== 6) return null;
    
    const [second, minute, hour, dayOfMonth, month, dayOfWeek] = parts;
    
    try {
      // 辅助函数：安全地解析数字，如果无法解析则返回默认值
      const safeParseInt = (value, defaultValue) => {
        if (!value || value === '*' || value.includes('/') || value.includes(',') || value.includes('-')) {
          return defaultValue;
        }
        const parsed = parseInt(value, 10);
        return isNaN(parsed) ? defaultValue : parsed;
      };

      // 检查是否匹配每天模式: 0 minute hour * * ?
      if (second === '0' && dayOfMonth === '*' && month === '*' && dayOfWeek === '?') {
        const parsedHour = safeParseInt(hour, 2);
        const parsedMinute = safeParseInt(minute, 0);
        
        // 只有当hour和minute都是简单数字时才解析为简单配置
        if (!hour.includes('*') && !hour.includes('/') && !minute.includes('*') && !minute.includes('/')) {
          return {
            type: 'daily',
            hour: parsedHour,
            minute: parsedMinute,
            weekdays: [],
            dayOfMonth: 1
          };
        }
      }
      
      // 检查是否匹配每周模式: 0 minute hour ? * weekdays
      if (second === '0' && dayOfMonth === '?' && month === '*') {
        const parsedHour = safeParseInt(hour, 2);
        const parsedMinute = safeParseInt(minute, 0);
        
        // 只有当hour和minute都是简单数字时才解析为简单配置
        if (!hour.includes('*') && !hour.includes('/') && !minute.includes('*') && !minute.includes('/')) {
          const weekdays = dayOfWeek.includes(',') ? dayOfWeek.split(',') : [dayOfWeek];
          return {
            type: 'weekly',
            hour: parsedHour,
            minute: parsedMinute,
            weekdays: weekdays,
            dayOfMonth: 1
          };
        }
      }
      
      // 检查是否匹配每月模式: 0 minute hour dayOfMonth * ?
      if (second === '0' && month === '*' && dayOfWeek === '?') {
        const parsedHour = safeParseInt(hour, 2);
        const parsedMinute = safeParseInt(minute, 0);
        const parsedDayOfMonth = safeParseInt(dayOfMonth, 1);
        
        // 只有当hour、minute和dayOfMonth都是简单数字时才解析为简单配置
        if (!hour.includes('*') && !hour.includes('/') && !minute.includes('*') && !minute.includes('/') && !dayOfMonth.includes('*') && !dayOfMonth.includes('/')) {
          return {
            type: 'monthly',
            hour: parsedHour,
            minute: parsedMinute,
            weekdays: [],
            dayOfMonth: parsedDayOfMonth
          };
        }
      }
    } catch (e) {
      // 解析失败，返回null
    }
    
    return null;
  };

  // 初始化组件时解析现有值
  useEffect(() => {
    if (value && !initialized) {
      // 检查是否是预设表达式
      const preset = Object.entries(presets).find(([key, preset]) => preset.expr === value);
      if (preset) {
        setCronType('preset');
        setInitialized(true);
        return;
      }
      
      // 尝试解析为简单配置
      const parsedConfig = parseCronToSimpleConfig(value);
      if (parsedConfig) {
        setCronType('simple');
        setSimpleConfig(parsedConfig);
        setInitialized(true);
        return;
      }
      
      // 无法解析，使用手动模式
      setCronType('manual');
      setInitialized(true);
    }
  }, [value, initialized]);

  // 当简单配置改变时更新Cron表达式
  useEffect(() => {
    if (cronType === 'simple' && initialized) {
      const newCron = generateSimpleCron(simpleConfig);
      onChange?.(newCron);
    }
  }, [cronType, simpleConfig, onChange, initialized]);

  // 处理预设选择
  const handlePresetChange = (presetKey) => {
    const preset = presets[presetKey];
    if (preset) {
      onChange?.(preset.expr);
    }
  };

  // 处理简单配置更新
  const updateSimpleConfig = (updates) => {
    setSimpleConfig(prev => ({ ...prev, ...updates }));
  };

  return (
    <div>
      <Row gutter={[16, 16]}>
        <Col span={24}>
          <Text strong>选择配置方式:</Text>
          <Select
            value={cronType}
            onChange={setCronType}
            style={{ width: 200, marginLeft: 8 }}
            options={[
              { label: '预设模板', value: 'preset' },
              { label: '简单配置', value: 'simple' },
              { label: '手动输入', value: 'manual' }
            ]}
          />
        </Col>

        {cronType === 'preset' && (
          <Col span={24}>
            <Text>选择预设:</Text>
            <Select
              placeholder="选择常用的执行时间"
              style={{ width: '100%', marginTop: 8 }}
              onChange={handlePresetChange}
              options={Object.entries(presets).map(([key, preset]) => ({
                label: preset.desc,
                value: key
              }))}
            />
          </Col>
        )}

        {cronType === 'simple' && (
          <>
            <Col span={24}>
              <Text>执行频率:</Text>
              <Select
                value={simpleConfig.type}
                onChange={(type) => updateSimpleConfig({ type })}
                style={{ width: 200, marginLeft: 8 }}
                options={[
                  { label: '每天', value: 'daily' },
                  { label: '每周', value: 'weekly' },
                  { label: '每月', value: 'monthly' }
                ]}
              />
            </Col>

            <Col span={12}>
              <Text>小时:</Text>
              <InputNumber
                value={simpleConfig.hour}
                onChange={(hour) => updateSimpleConfig({ hour })}
                min={0}
                max={23}
                style={{ width: '100%', marginLeft: 8 }}
              />
            </Col>

            <Col span={12}>
              <Text>分钟:</Text>
              <InputNumber
                value={simpleConfig.minute}
                onChange={(minute) => updateSimpleConfig({ minute })}
                min={0}
                max={59}
                style={{ width: '100%', marginLeft: 8 }}
              />
            </Col>

            {simpleConfig.type === 'weekly' && (
              <Col span={24}>
                <Text>选择星期:</Text>
                <div style={{ marginTop: 8 }}>
                  <Checkbox.Group
                    value={simpleConfig.weekdays}
                    onChange={(weekdays) => updateSimpleConfig({ weekdays })}
                  >
                    <Space wrap>
                      {weekDays.map(day => (
                        <Checkbox key={day.value} value={day.value}>
                          {day.label}
                        </Checkbox>
                      ))}
                    </Space>
                  </Checkbox.Group>
                </div>
              </Col>
            )}

            {simpleConfig.type === 'monthly' && (
              <Col span={12}>
                <Text>日期:</Text>
                <InputNumber
                  value={simpleConfig.dayOfMonth}
                  onChange={(dayOfMonth) => updateSimpleConfig({ dayOfMonth })}
                  min={1}
                  max={31}
                  style={{ width: '100%', marginLeft: 8 }}
                />
              </Col>
            )}
          </>
        )}

        {cronType === 'manual' && (
          <>
            <Col span={24}>
              <Alert
                message="Cron表达式格式"
                description={
                  <div>
                    <p>格式: 秒 分 时 日 月 周</p>
                    <p>示例:</p>
                    <ul>
                      <li>0 0 2 * * ? - 每天凌晨2点</li>
                      <li>0 */10 * * * ? - 每10分钟</li>
                      <li>0 0 2 ? * SUN - 每周日凌晨2点</li>
                      <li>0 0 2 1 * ? - 每月1号凌晨2点</li>
                    </ul>
                  </div>
                }
                type="info"
                showIcon
                style={{ marginBottom: 16 }}
              />
            </Col>
            <Col span={24}>
              <Text>Cron表达式:</Text>
              <Input
                value={value}
                onChange={(e) => onChange?.(e.target.value)}
                placeholder="0 0 2 * * ?"
                style={{ marginTop: 8 }}
              />
            </Col>
          </>
        )}

        {value && (
          <Col span={24}>
            <Alert
              message="当前配置"
              description={
                <div>
                  <p><Text strong>Cron表达式:</Text> <Text code>{value}</Text></p>
                  <p><Text strong>执行时间:</Text> {parseCronDescription(value)}</p>
                </div>
              }
              type="success"
              showIcon
            />
          </Col>
        )}
      </Row>
    </div>
  );
};

export default CronBuilder;