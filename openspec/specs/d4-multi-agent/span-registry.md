# D4 Multi-Agent Span 注册表

**Domain:** D4 Multi-Agent
**Version:** 2.0.0
**Status:** Active (2026-06-14)
**Canonical Source:** `internal/layers/observability/telemetry/names.go` · `internal/layers/observability/coverage/registry.go`

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

## 关联文档

- D5 全局 Trace Tree：`openspec/specs/d5-observability/span-registry.md`
- 全局 Spans 索引：`openspec/spans-registry.md`
