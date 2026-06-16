# D4 Multi-Agent — 可观测性与验收指南

**Capability:** d4-multi-agent
**Status:** Active
**Version:** 1.0.0
**Last Updated:** 2026-06-16
**Parent:** `d4-domain.md` · `span-registry.md` · `t-registry.md`
**Complements:** `terminal-state-guide.md` · `../d7-orchestration/observability-guide.md`

---

## 0. 文档定位

| 本文 | SoT |
|------|-----|
| Span ↔ S11–S16 绑定 | `span-registry.md` |
| Worker 派发 Trace 树 | `telemetry/names.go` |
| T 摘要 + P0 Runbook | `t-registry.md`（38/38 IMPLEMENTED，19 P0） |

> **Hub-Spoke Flow span** 已迁 **D7 `orchestration.flow.*`**；D4 仅保留 `agent.*` emit。

---

## 1. Canonical Span ↔ S 绑定

| Operation | S | 备注 |
|-----------|---|------|
| `agent.run` | S12 | 主循环 |
| `agent.tool.call` | S12 | 含 permission 子 span |
| `agent.fork` / `agent.join` | S13 | COW 隔离 |
| `agent.state.transition` | S12 | 状态机 |
| `agent.terminate` | S12 | 超时/取消 |
| `agent.tool.call`（external） | S15 | CLI/Cursor |

**DM-018 规则：** Flow 相关 span 不应出现在 D4 根下；应在 D7 Turn 子树。

---

## 2. Trace 树

### 2.1 D7 派发 Worker（挂在 D7 下）

```text
D7_S2_Orchestration_Delegate_Dispatch
└── D4_S14_Execute_Worker
    ├── agent.fork
    ├── agent.run
    │   ├── agent.state.transition
    │   ├── agent.tool.call
    │   │   └── agent.state.transition (waiting_permission)
    │   └── agent.terminate
    └── agent.join
```

### 2.2 External Agent Tool（S15）

```text
agent.tool.call (external)
└── agent.run (subprocess/stream parse)
```

---

## 3. T 层验收矩阵（按 S 摘要）

| S | 覆盖重点 | 代表 T |
|---|----------|--------|
| S11 | 配额、CreateWithView | D4-S11-A01-T01, T05 |
| S12 | 生命周期、PermissionGate | D4-S12-A01-T01, D4-S12-A02-T02 |
| S13 | Fork COW、Join dedup | D4-S13-A01-T05, T06 |
| S14 | Worker fork→run→join | D4-S14-A01-T* |
| S15 | External tool P0 | D4-S15-A02-T* acceptance |
| S16 | multi_agent config | D4-S16-A01-T01 |

### P0 必跑清单

```bash
# Factory + 配额
go test ./internal/layers/multiagent/provision/ -v

# Agent 生命周期 + Permission
go test ./internal/layers/multiagent/run/ -run 'Agent|Permission' -v

# Fork/Join 隔离
go test ./internal/layers/multiagent/run/ -run ForkJoin -v
go test ./internal/layers/multiagent/isolate/ -v

# Delegate Worker
go test ./internal/layers/multiagent/execute/ -v

# External Agent Tool（acceptance）
go test ./tests/acceptance/p0/ -run AgentTool -v
```

---

## 4. 生产检查清单

| 检查项 | 期望 |
|--------|------|
| 无 D4 根下 `orchestration.flow.*` | Flow 仅在 D7 |
| Fork 不污染父 Session | COW 测试绿 |
| CRITICAL 工具走 PermissionGate | perm_gate 测试绿 |
| Worker 经 D7 入口 | 无旁路 `delegate` 直调 |

---

## 5. 相关文档

| 文档 | 关系 |
|------|------|
| `span-registry.md` | operation 全表 + D5 迁移声明 |
| `t-registry.md` | T 全表 |
| `terminal-state-guide.md` | 派发时序 |
