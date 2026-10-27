import React from "react";
import { render, screen, within, fireEvent, waitFor } from "@testing-library/react";
import "@testing-library/jest-dom";
import LogList from "./LogList";
import { getDNSLogs, getGroups, getServers } from "../../api";

jest.mock("../../api", () => ({
  getDNSLogs: jest.fn(), getGroups: jest.fn(), getServers: jest.fn(),
}));

const computedStyle = window.getComputedStyle;

beforeAll(() => {
  window.matchMedia = () => ({ matches: false, addListener() {}, removeListener() {} });
  global.ResizeObserver = class { observe() {} unobserve() {} disconnect() {} };
});

beforeEach(() => {
  jest.spyOn(window, "getComputedStyle").mockImplementation((element) => computedStyle(element));
  getGroups.mockResolvedValue({ data: [{ id: 1, name: "oversea" }] });
  getServers.mockResolvedValue({ data: [{ id: 1, address: "192.0.2.136", groups: ["oversea"], type: "udp" }] });
  const base = { timestamp: "2026-10-26T02:00:00Z", domain: "api.example.org", client_ip: "192.0.2.1", time_ms: 0, speed_ms: -0.1 };
  getDNSLogs.mockResolvedValue({ data: { total: 2, logs: [
    { ...base, id: 1, query_type: 1, group: "__static_match__", result: "192.0.2.10", ip_count: 1 },
    { ...base, id: 2, query_type: 65, group: "oversea", result: "", ip_count: 0 },
  ] } });
});

test("入库标记显示静态映射匹配，旧分组与 HTTPS 仍显示配置上游", async () => {
  render(<LogList nodeId={1} />);
  const staticTag = await screen.findByText("静态映射匹配");
  const staticRow = staticTag.closest("tr");
  expect(within(staticRow).queryByText("192.0.2.136")).not.toBeInTheDocument();
  expect(within(staticRow).queryByText("oversea")).not.toBeInTheDocument();
  const detailButton = await screen.findByRole("button", { name: "oversea：查看 1 个配置上游" });
  const upstreamRow = detailButton.closest("tr");
  expect(within(upstreamRow).getByText("HTTPS")).toBeInTheDocument();
  expect(within(upstreamRow).getByText("oversea")).toBeInTheDocument();
  expect(screen.queryByText("192.0.2.136")).not.toBeInTheDocument();
  fireEvent.click(detailButton);
  expect(await screen.findByText("192.0.2.136")).toBeInTheDocument();
  expect(screen.getByText("配置列表不代表此次查询的实际响应来源。")).toBeInTheDocument();
  expect(screen.queryByText("所属上游")).not.toBeInTheDocument();
});

test("特殊分组可作为原有接口的筛选值提交", async () => {
  render(<LogList nodeId={1} />);
  await screen.findByText("静态映射匹配");
  fireEvent.mouseDown(screen.getByLabelText("日志分组"));
  await screen.findByRole("option", { name: "静态映射匹配" });
  expect(screen.getByRole("option", { name: "default（默认组）" })).toBeInTheDocument();
  // Select 的虚拟无障碍选项不处理点击，选择实际显示的选项。
  fireEvent.click(screen.getByTitle("静态映射匹配"));
  fireEvent.click(screen.getByRole("button", { name: /搜\s*索/ }));
  await waitFor(() => expect(getDNSLogs).toHaveBeenLastCalledWith(expect.objectContaining({ group: "__static_match__" })));
});

test("默认组展示隐式成员，并排除禁用、其他节点和显式排除的服务器", async () => {
  getServers.mockResolvedValue({ data: [
    { id: 1, address: "192.0.2.29", groups: ["cn"], enabled: true, node_ids: "[1]" },
    { id: 2, address: "192.0.2.30", groups: [], enabled: true, node_ids: "[]" },
    { id: 3, address: "192.0.2.136", groups: ["oversea"], enabled: true, exclude_default: true },
    { id: 4, address: "192.0.2.31", groups: ["cn"], enabled: false },
    { id: 5, address: "192.0.2.32", groups: ["cn"], enabled: true, node_ids: "[2]" },
    { id: 6, address: "192.0.2.33", groups: ["cn"], enabled: true, options: "-exclude-default-group" },
  ] });
  getDNSLogs.mockResolvedValue({ data: { total: 1, logs: [
    { id: 1, timestamp: "2026-10-26T02:00:00Z", domain: "www.example.org", query_type: 1,
      client_ip: "192.0.2.1", time_ms: 0, speed_ms: 45.6, group: "default", result: "192.0.2.50", ip_count: 1 },
  ] } });
  render(<LogList nodeId={1} />);
  const row = (await screen.findByText("www.example.org")).closest("tr");
  const detailButton = await within(row).findByRole("button", { name: "default：查看 2 个配置上游" });
  expect(screen.queryByText("192.0.2.29")).not.toBeInTheDocument();
  fireEvent.click(detailButton);
  expect(await screen.findByText("192.0.2.29")).toBeInTheDocument();
  expect(screen.getByText("192.0.2.30")).toBeInTheDocument();
  for (const ip of ["192.0.2.136", "192.0.2.31", "192.0.2.32", "192.0.2.33"]) {
    expect(screen.queryByText(ip)).not.toBeInTheDocument();
  }
  expect(within(row).queryByText("无配置服务器")).not.toBeInTheDocument();
});
