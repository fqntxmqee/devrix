# D2 Context Engine Span 注册表

**Domain:** D2 Context Engine
**Version:** 2.0.0
**Status:** Active (2026-06-14)
**Canonical Source:** `internal/layers/observability/telemetry/names.go` · `internal/layers/observability/coverage/registry.go`

---

## Operations

### Context Engine（10 ops）

| Operation | Kind | Component | Since | Key Attributes |
|-----------|------|-----------|-------|----------------|
| `context.process` | INTERNAL | context_engine | 1.2.0 | session_id |
| `context.snapshot.load` | INTERNAL | context_engine | 1.2.0 | session_id |
| `context.system_prompt.load` | INTERNAL | context_engine | 2.0.0 | — |
| `context.compression.run` | INTERNAL | context_engine | 1.2.0 | — |
| `context.compression.step` | INTERNAL | context_engine | 2.1.0 | step_name |
| `context.longterm.recall` | INTERNAL | context_engine | 1.3.0 | — |
| `context.longterm.store` | INTERNAL | context_engine | 1.3.0 | — |
| `context.tools.register` | INTERNAL | context_engine | 2.0.0 | tool_count |
| `context.memory.snapshot.save` | INTERNAL | context_engine | 2.0.0 | session_id |

### Harness Bootstrap（5 ops）

| Operation | Kind | Component | Since | Key Attributes |
|-----------|------|-----------|-------|----------------|
| `context.harness.bootstrap.run` | INTERNAL | harness | 5.0.0 | session_id |
| `context.harness.bootstrap.stage` | INTERNAL | harness | 5.0.0 | stage (prefetch\|guards\|setup\|deferred_init\|tool_pool) |
| `context.harness.tool_pool` | INTERNAL | harness | 5.0.0 | — |
| `context.harness.preflight` | INTERNAL | harness | 5.0.0 | — |
| `context.harness.route` | INTERNAL | harness | 5.0.0 | — |
| `context.system_prompt.build` | INTERNAL | harness | 5.0.0 | version, template_hash, agents_md_hash |

### QueryLoop（3 ops）

| Operation | Kind | Component | Since | Key Attributes |
|-----------|------|-----------|-------|----------------|
| `query.loop.run` | INTERNAL | query_loop | 2.1.0 | session_id |
| `query.loop.turn` | INTERNAL | query_loop | 2.1.0 | turn_n |
| `query.loop.llm.call` | CLIENT | query_loop | 2.1.0 | model, turn_n |

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

### 主路径：QueryLoop

```
context.process
├── context.snapshot.load
├── context.system_prompt.load
├── context.longterm.recall                  [if longterm.enabled]
├── context.compression.run                  [if shouldCompress]
│   └── context.compression.step             [per pipeline step]
├── query.loop.run                           [if query_loop.enabled]
│   └── query.loop.turn                      [per turn]
│       ├── query.loop.llm.call
│       │   └── (→ D3 llm.stream subtree)
│       └── tool.execute.single              [if tool_calls]
│           └── tool.execute.permission      [if CRITICAL]
├── context.memory.snapshot.save
└── context.longterm.store                   [if auto_store]
```

### 条件路径：Legacy Harness（`query_loop.enabled=false`）

```
context.process
├── context.harness.bootstrap.run
│   └── context.harness.bootstrap.stage
├── context.harness.preflight
├── context.harness.tool_pool
├── context.harness.route
└── context.system_prompt.build
```

---

## 压缩延迟目标

| 指标 | 目标 | 关联 Span |
|------|------|-----------|
| 压缩延迟 | P99 < 100ms（不含 LLM） | `context.compression.run` |
| Process 启动 | P99 < 50ms（含快照加载） | `context.process` |

---

## 关联文档

- D5 全局 Trace Tree：`openspec/specs/d5-observability/span-registry.md`
- 全局 Spans 索引：`openspec/spans-registry.md`
