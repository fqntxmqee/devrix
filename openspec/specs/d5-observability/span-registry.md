# D5 Observability Span 注册表

**Domain:** D5 Observability
**Version:** 2.0.0
**Status:** Active (2026-06-14)
**Canonical Source:** `internal/layers/observability/instrument/telemetry/names.go` · `internal/layers/observability/diagnose/coverage/registry.go`

> D5 是可观测性元域。本文件为全局 Span/Operation 注册表的权威索引，汇总所有 57 个已注册 Operation 及其按域/组件的分布。

---

## 全局 Trace Tree（D7 Turn 主路径 — DM-20260618-010）

```
gateway.message.receive                          [SERVER]
└── orchestration.turn.run                       [INTERNAL]
    └── orchestration.turn.iteration             [per turn]
        ├── orchestration.llm.invoke             [CLIENT]
        │   └── llm.stream                       [CLIENT]
        └── context.process                      [INTERNAL, caller=d7]
            ├── context.snapshot.load
            ├── context.compression.run          [if entry compress]
            └── tool.execute.single              [if tool_calls]
                └── tool.execute.permission      [if CRITICAL]
```

<details>
<summary>历史 QueryLoop Trace Tree（已删除）</summary>

```
# REMOVED: query.loop.run / query.loop.llm.call (DM-20260618-010)
```
</details>

## 条件路径：Legacy Harness — **REMOVED (D2-S20 v6.5.0)**

```
context.process
├── context.harness.bootstrap.run
│   └── context.harness.bootstrap.stage  (prefetch|guards|setup|deferred_init|tool_pool)
├── context.harness.preflight
├── context.harness.tool_pool
├── context.harness.route
└── context.system_prompt.build
```

## 跨域 Agent / Orchestration / Evolution

```
agent.run → agent.tool.call → agent.fork|join|terminate
  └── D6_S4_Validation_Decision              [if evolution.orchestration.enabled]
orchestration.wave.schedule → orchestration.wave.task.execute
orchestration.flow.event.publish
```

---

## Operation 注册表（57 ops，按 Layer 分组）

| Layer | Component | Count | Operations |
|-------|-----------|-------|------------|
| communication | gateway | 12 | `gateway.message.receive`, `gateway.session.lifecycle`, `gateway.session.create`, `gateway.session.get`, `gateway.session.expire`, `gateway.store.create`, `gateway.store.get`, `gateway.store.update`, `gateway.store.delete`, `gateway.permission.check`, `gateway.agent.create`, `gateway.engine_event.handle` |
| communication | adapter | 3 | `adapter.message.receive`, `adapter.cli.send`, `adapter.feishu.outbound` |
| context | context_engine | 9 | `context.process`, `context.snapshot.load`, `context.system_prompt.load`, `context.compression.run`, `context.compression.step`, `context.longterm.recall`, `context.longterm.store`, `context.tools.register`, `context.memory.snapshot.save` |
| context | harness | 6 | `context.harness.bootstrap.run`, `context.harness.bootstrap.stage`, `context.harness.tool_pool`, `context.harness.preflight`, `context.harness.route`, `context.system_prompt.build` |
| context | query_loop | 3 | `query.loop.run`, `query.loop.turn`, `query.loop.llm.call` |
| context | tool_runner | 2 | `tool.execute.single`, `tool.execute.permission` |
| context | plan_* | 7 | `task.plan.generate`, `task.plan_mode.enter`, `task.plan_mode.execute`, `task.plan_mode.approve`, `task.plan_mode.reject`, `task.manager.create`, `task.manager.update` |
| llm | llm_gateway | 4 | `llm.stream`, `llm.provider.route`, `llm.circuit_breaker`, `llm.retry` |
| llm | llm_adapter | 1 | `llm.adapter.stream` |
| agent | agent_tool | 6 | `agent.tool.call`, `agent.run`, `agent.fork`, `agent.join`, `agent.terminate`, `agent.state.transition` |
| orchestration | orchestrator | 3 | `orchestration.wave.schedule`, `orchestration.wave.task.execute`, `orchestration.flow.event.publish` |
| evolution | validation | 1 | `D6_S4_Validation_Decision` |

---

## 全局 Metrics

| Metric | Type | Labels | 写入域 |
|--------|------|--------|--------|
| `devrix_tool_latency` | Histogram | tool, risk_level, status | D2 QueryLoop |
| `devrix_compression_ratio` | Histogram | — | D2 Compression |
| `devrix_gen_ai.client.token.usage` | Counter | token_type, model | D3 LLM Gateway |
| `devrix_active_sessions` | Gauge | adapter | D1 SessionBridge |
| `devrix_runtime_path_resolved_total` | Counter | path | D5 Runtime |

---

## Span 创建规范

所有域统一通过 `observability.Bridge` 创建 Span：

```go
ctx, span := bridge.Tracer().Start(ctx, telemetry.OpQueryLoopLLMCall,
    tracer.WithSpanKind(tracer.SpanKindClient),
    tracer.WithSpanAttributes(telemetry.SpanAttrs(telemetry.OpQueryLoopLLMCall)...),
)
defer span.End()
```

Span 属性通过 `telemetry.SpanAttrs()` 自动注入 `devrix.layer` 和 `devrix.component`。

---

## 域级 Span 注册表

| 域 | 路径 | Operations |
|----|------|------------|
| D1 Communication | `openspec/specs/d1-communication/span-registry.md` | 15 |
| D2 Context Engine | `openspec/specs/d2-context-engine/span-registry.md` | 27 |
| D3 LLM Gateway | `openspec/specs/d3-llm-gateway/span-registry.md` | 5 |
| D4 Multi-Agent | `openspec/specs/d4-multi-agent/span-registry.md` | 6 |
| D6 Evolution | `openspec/specs/d6-evolution/span-registry.md` | 1 |
| D7 Orchestration | `openspec/specs/d7-orchestration/span-registry.md` | 3 |
| **D5 (本文件)** | — | **57 (全部)** |

---

## 关联文档

- 全局 Spans 索引：`openspec/spans-registry.md`
- Operation 常量定义：`internal/layers/observability/instrument/telemetry/names.go`
- Coverage Registry：`internal/layers/observability/diagnose/coverage/registry.go`
