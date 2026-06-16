# D4 Multi-Agent Span 注册表

**Domain:** D4 Multi-Agent
**Version:** 3.0.0
**Status:** Active (2026-06-14)
**Change ID:** devrix-d4-sa-refine
**Demand ID:** DM-20260614-018
**Canonical Source:** `internal/layers/observability/instrument/telemetry/names.go` · `internal/layers/observability/diagnose/coverage/registry.go`

---

## Operations

### Agent Tool（6 ops）

| Operation | Kind | Component | Since | Key Attributes |
|-----------|------|-----------|-------|----------------|
| `agent.tool.call` | INTERNAL | agent_tool | 2.0.0 | agent_id, tool_name, risk_level |
| `agent.run` | INTERNAL | agent_tool | 2.0.0 | agent_id, mode, max_iter |
| `agent.fork` | INTERNAL | agent_tool | 2.0.0 | agent_id, child_id, child_count |
| `agent.join` | INTERNAL | agent_tool | 2.0.0 | agent_id, child_id |
| `agent.terminate` | INTERNAL | agent_tool | 2.0.0 | agent_id, reason, duration_ms |
| `agent.state.transition` | INTERNAL | agent_tool | 2.0.0 | agent_id, from_state, to_state |

---

## Trace Tree

```
agent.run
├── agent.state.transition (created → running)
├── agent.tool.call
│   └── agent.state.transition (running → waiting_permission)
├── agent.fork
│   └── agent.run (child)
├── agent.join
└── agent.terminate
    └── agent.state.transition (running → terminated)
```

---

## Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `agent.created_total` | Counter | mode | Agent 创建总数 |
| `agent.active` | Gauge | — | 当前活跃 Agent 数 |
| `agent.duration_ms` | Histogram | mode | Agent 执行耗时分布 |
| `agent.fork.created_total` | Counter | — | Fork 创建子 Agent 总数 |
| `agent.permission.requests_total` | Counter | tool_name, risk_level | 权限请求总数 |
| `agent.permission.granted_total` | Counter | tool_name | 权限批准总数 |
| `agent.permission.denied_total` | Counter | tool_name | 权限拒绝总数 |
| `agent.errors_total` | Counter | error_code | Agent 错误总数 |

---

## v1.1 迁移声明（D4-S8 → D5）

> **v1.0 不变性**：本节 span/metric **名字不变**；仅归属与注册 SoT 迁移。

| 资产 | v1.0 现状 | v1.1 目标 | 说明 |
|------|-----------|-----------|------|
| `agent.*` operations | D4 emit | **D5 SoT** | D4 仅保留 `SetObservabilitySink` hook |
| `agent.*` metrics | D4 `observability/instrument/metrics/` | **D5 span-registry** | 含 `agent.fork.created_total` 等 8 项 |
| Hub-Spoke Flow span | D4 `delegate/bridge` + D2 `flow_report` | **D7 `orchestration.flow.*`** | v2.0 代码迁 D7 `hubspoke/` |
| D4-S8-A01 | RecordForkPolicyMetrics | **Deprecated → D5** | Phase D1 执行 |

**D4 保留（Execution Follower）：**

- `agent.run` / `agent.tool.call` / `agent.fork` / `agent.join` / `agent.terminate` / `agent.state.transition` — emit only
- Permission metrics — 仍由 D4 emit，v1.1 注册归 D5

---

## 关联文档

- `observability-guide.md` — Worker Trace 树 + P0 Runbook
- D5 全局 Trace Tree：`openspec/specs/d5-observability/span-registry.md`
- 全局 Spans 索引：`openspec/spans-registry.md`
- D4↔D7 边界：`openspec/specs/d4-multi-agent/d7-boundary.md`

---

## Revision History

| Version | Date | Changes |
|---------|------|---------|
| 2.0.0 | 2026-06-14 | 初版：6 ops + 8 metrics |
| 3.0.0 | 2026-06-14 | S8→D5 迁移声明；Hub-Spoke span 归 D7 声明（DM-20260614-018） |
