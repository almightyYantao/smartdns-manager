import { getPendingNodeServers, parseNodeServers } from '../nodeServers';

test('从节点文件识别 DoH、DoT、TCP 和 UDP，跳过注释和其他配置', () => {
  const applied = parseNodeServers(`
# server 223.5.5.5
server 10.1.8.10 -group local -exclude-default-group
server-https https://dns.google/dns-query -host-ip 8.8.8.8 -tls-host-verify dns.google -group oversea-upstream
server-tcp 1.1.1.1 -group one -group two # 注释
server tls://9.9.9.9
nameserver /example.com/oversea
`);
  expect(applied.map((s) => s.type)).toEqual(['udp', 'https', 'tcp', 'tls']);
  expect(applied[0].exclude_default).toBe(true);
  expect(applied[2].groups).toEqual(['one', 'two']);
  expect(applied[2].options).not.toContain('#');
});

test('待同步只包含分配给当前节点且配置不同的记录，忽略参数顺序', () => {
  const applied = parseNodeServers('server-https https://dns.google/dns-query -host-ip 8.8.8.8 -tls-host-verify dns.google -group overseas');
  const records = [
    { id: 1, address: '223.5.5.5', node_ids: '[1,2]' },
    { id: 2, address: 'https://dns.google/dns-query', type: 'https', node_ids: '[3]',
      options: '-group overseas -tls-host-verify dns.google -host-ip 8.8.8.8' },
    { id: 3, address: '8.8.4.4', node_ids: '[3]' },
    { id: 4, address: '9.9.9.9', node_ids: '[]' },
    { id: 5, address: '1.1.1.1', node_ids: '[3]', enabled: false },
    { id: 6, address: '2.2.2.2', node_ids: 'invalid' },
  ];
  expect(getPendingNodeServers(records, applied, '3').map((s) => s.id)).toEqual([3, 4]);
});

test('仅在参数发生变化时标记同地址条目为待同步', () => {
  const applied = parseNodeServers('server 10.1.8.10 -group local -exclude-default-group');
  const record = { id: 1, address: '10.1.8.10', type: 'udp', groups: ['local'],
    exclude_default: true, node_ids: '[3]' };
  expect(getPendingNodeServers([record], applied, 3)).toEqual([]);
  expect(getPendingNodeServers([{ ...record, exclude_default: false }], applied, 3)).toHaveLength(1);
});
