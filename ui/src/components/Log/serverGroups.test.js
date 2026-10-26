import { getConfiguredServers } from "./serverGroups";

test("命名分组与默认组可重叠，exclude_default 不影响命名组", () => {
  const cn = { groups: ["cn"], node_ids: "[]", enabled: true };
  const oversea = { groups: ["oversea"], exclude_default: true, enabled: true };
  expect(getConfiguredServers([cn, oversea], "default", 1)).toEqual([cn]);
  expect(getConfiguredServers([cn, oversea], "cn", 1)).toEqual([cn]);
  expect(getConfiguredServers([cn, oversea], "oversea", 1)).toEqual([oversea]);
});

test("仅列出节点范围有效的服务器，兼容字符串及数组节点列表", () => {
  const common = { enabled: true, node_ids: "[]" };
  const scoped = { enabled: true, node_ids: ["1"] };
  expect(getConfiguredServers([
    common, scoped,
    { enabled: true, node_ids: "[2]" },
    { enabled: true, node_ids: "invalid" },
    { enabled: true, node_ids: "null" },
    { enabled: false },
  ], "default", 1)).toEqual([common, scoped]);
});

test("识别 options 中的默认组排除标志，避免误匹配较长选项", () => {
  const kept = { options: "-example" };
  expect(getConfiguredServers([
    { options: "-e" }, { options: "-group cn -exclude-default-group" }, kept,
  ], "default", 1)).toEqual([kept]);
});
