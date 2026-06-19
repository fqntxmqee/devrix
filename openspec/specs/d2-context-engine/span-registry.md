# D2 Context Engine Span 注册表

**Domain:** D2 Context Engine
**Version:** 2.3.0
**Status:** Active (2026-06-19) — v2.2 structure closure paths synced (P3-T2, P4, P5)
**Canonical Source:** `internal/layers/observability/instrument/telemetry/names.go` · `internal/layers/observability/diagnose/coverage/registry.go`
**Complements:** `observability-guide.md`（Trace 树 + P0 Runbook）

---

## Operations

### Context Engine（10 ops）

| Operation | Kind | Component | Canonical S | Since |
|-----------|------|-----------|-------------|-------|
| `context.process` | INTERNAL | context_engine | S15–S17 | 1.2.0 |
| `context.snapshot.load` | INTERNAL | context_engine | S15 | 1.2.0 |
| `context.system_prompt.load` | INTERNAL | context_engine | S15 | 2.0.0 |
| `context.compression.run` | INTERNAL | context_engine | S15 | 1.2.0 |
| `context.compression.step` | INTERNAL | context_engine | S15 | 2.1.0 |
| `context.longterm.recall` | INTERNAL | context_engine | S15 | 1.3.0 |
| `context.longterm.store` | INTERNAL | context_engine | S17 | 1.3.0 |
| `context.tools.register` | INTERNAL | context_engine | S18 | 2.0.0 |
| `context.memory.snapshot.save` | INTERNAL | context_engine | S17 | 2.0.0 |

### Harness Bootstrap（5 ops）— **REMOVED（D2-S20 v6.5.0）**

> 生产路径不应出现下列 span。追溯登记保留；DM-020 主路径见 `observability-guide.md` §2.1。

| Operation | Kind | Component | Since | Key Attributes |
|-----------|------|-----------|-------|----------------|
| `context.harness.bootstrap.run` | INTERNAL | harness | 5.0.0 | session_id |
| `context.harness.bootstrap.stage` | INTERNAL | harness | 5.0.0 | stage (prefetch\|guards\|setup\|deferred_init\|tool_pool) |
| `context.harness.tool_pool` | INTERNAL | harness | 5.0.0 | — |
| `context.harness.preflight` | INTERNAL | harness | 5.0.0 | — |
| `context.harness.route` | INTERNAL | harness | 5.0.0 | — |
| `context.system_prompt.build` | INTERNAL | harness | 5.0.0 | version, template_hash, agents_md_hash |

### QueryLoop（3 ops）— **REMOVED (DM-20260618-010)**

> LLM↔Tool span 归 D7 `orchestration.turn.*`。下列 op 不再出现在生产路径。

| Operation | Kind | Component | Canonical S | Since | Status |
|-----------|------|-----------|-------------|-------|--------|
| `query.loop.run` | INTERNAL | query_loop | ~~S16~~ | 2.1.0 | **REMOVED** |
| `query.loop.turn` | INTERNAL | query_loop | ~~S16~~ | 2.1.0 | **REMOVED** |
| `query.loop.llm.call` | CLIENT | query_loop | ~~S16~~ | 2.1.0 | **REMOVED** |

### Tool Execution（2 ops）

| Operation | Kind | Component | Since | Key Attributes |
|-----------|------|-----------|-------|----------------|
| `tool.execute.single` | INTERNAL | tool_runner | 2.1.0 | tool_name, risk_level |
| `tool.execute.permission` | INTERNAL | tool_runner | 2.1.0 | tool_name, risk_level, result |

### Task / Plan（7 ops）

| Operation | Kind | Component | Since | Key Attributes |
|-----------|------|-----------|-------|----------------|
| `task.plan.generate` | INTERNAL | plan_agent | 2.1.0 | — |
| `task.plan_mode.enter` | INTERNAL | plan_mode | 2.1.0 | session_id |
| `task.plan_mode.execute` | INTERNAL | plan_mode | 2.1.0 | session_id |
| `task.plan_mode.approve` | INTERNAL | plan_mode | 2.1.0 | session_id |
| `task.plan_mode.reject` | INTERNAL | plan_mode | 2.1.0 | session_id |
| `task.manager.create` | INTERNAL | task_manager | 2.1.0 | task_id |
| `task.manager.update` | INTERNAL | task_manager | 2.1.0 | task_id, status |

---

## Trace Tree

### 主路径：D7 Turn（经 PreparedTurnRunner）

```
context.process
├── context.snapshot.load
├── context.system_prompt.load
├── context.longterm.recall                  [if longterm.enabled]
├── context.compression.run                  [if shouldCompress at entry]
│   └── context.compression.step             [per pipeline step]
├── (→ D7 orchestration.turn.run)            [PreparedTurnRunner]
│   └── orchestration.turn.iteration         [per turn — see D7 span-registry]
│       ├── orchestration.llm.invoke
│       └── tool.execute.single              [if tool_calls]
│           └── tool.execute.permission      [if CRITICAL]
├── context.memory.snapshot.save
└── context.longterm.store                   [if auto_store]
```

### 历史路径（已删除）

<details>
<summary>QueryLoop / Legacy Harness（pre-v8.0.0）</summary>

```
# REMOVED: query.loop.run / harness bootstrap paths (DM-20260618-010)
```
</details>

---

## 压缩延迟目标

| 指标 | 目标 | 关联 Span |
|------|------|-----------|
| 压缩延迟 | P99 < 100ms（不含 LLM） | `context.compression.run` |
| Process 启动 | P99 < 50ms（含快照加载） | `context.process` |

---

## 关联文档

- `observability-guide.md` — Trace 树 + P0 Runbook
- D5 全局 Trace Tree：`openspec/specs/d5-observability/span-registry.md`
- 全局 Spans 索引：`openspec/spans-registry.md`
