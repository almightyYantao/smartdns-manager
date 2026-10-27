// 节点视图以配置文件为准，兼容显式协议指令和 URL 协议。
export function parseNodeServers(content) {
  return content.split(/\r?\n/).flatMap((line, index) => {
    const match = line.trim().match(/^server(?:-(tcp|tls|https|quic|h3))?\s+(\S+)(?:\s+(.*))?$/);
    if (!match) return [];
    const [, protocol, address, tail = ''] = match;
    const options = tail.replace(/\s+#.*$/, '').trim();
    const scheme = address.match(/^(https|tls|tcp|quic|h3):\/\//)?.[1];
    return [{
      key: `file-${index}`,
      address,
      type: protocol || scheme || 'udp',
      groups: [...options.matchAll(/(?:^|\s)-group\s+(\S+)/g)].map((m) => m[1]),
      exclude_default: /(?:^|\s)-exclude-default-group(?:\s|$)/.test(options),
      options,
    }];
  });
}

function signature(server) {
  const options = server.options || [
    ...(server.groups || []).map((group) => `-group ${group}`),
    server.exclude_default ? '-exclude-default-group' : '',
  ].filter(Boolean).join(' ');
  const normalized = options.trim().split(/\s+(?=-)/)
    .map((option) => option.replace(/\s+/g, ' ')).filter(Boolean).sort();
  return JSON.stringify([server.type || 'udp', server.address, normalized]);
}

export function getPendingNodeServers(servers, applied, nodeId) {
  const current = new Set(applied.map(signature));
  return servers.filter((server) => {
    if (server.enabled === false) return false;
    let ids;
    try {
      ids = typeof server.node_ids === 'string'
        ? JSON.parse(server.node_ids || '[]') : (server.node_ids || []);
    } catch {
      return false;
    }
    if (!Array.isArray(ids)) return false;
    const assigned = ids.length === 0 || ids.some((id) => String(id) === String(nodeId));
    return assigned && !current.has(signature(server));
  });
}
