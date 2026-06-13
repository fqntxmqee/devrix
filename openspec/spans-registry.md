# Devrix Spans 注册表（全局索引）

**Status:** Active
**Version:** 2.0.0
**Last Updated:** 2026-06-14
**Canonical Source:** `internal/layers/observability/telemetry/names.go` · `internal/layers/observability/coverage/registry.go`

---

## Overview

本文档为 Devrix 全局 Span / Operation 注册表的索引入口。各域的 Span 注册表已拆分为独立文件。

**总计：56 Operations，5 Layers，11 Components**

---

## 域级 Span 注册表


| 域                 | 路径                                                  | Operations | Components                                                              |
| ----------------- | --------------------------------------------------- | ---------- | ----------------------------------------------------------------------- |
| D1 Communication  | `openspec/specs/d1-communication/span-registry.md`  | 15         | gateway(12), adapter(3)                                                 |
| D2 Context Engine | `openspec/specs/d2-context-engine/span-registry.md` | 27         | context_engine(9), harness(6), query_loop(3), tool_runner(2), plan_*(7) |
| D3 LLM Gateway    | `openspec/specs/d3-llm-gateway/span-registry.md`    | 5          | llm_gateway(4), llm_adapter(1)                                          |
| D4 Multi-Agent    | `openspec/specs/d4-multi-agent/span-registry.md`    | 6          | agent_tool(6)                                                           |
| D5 Observability  | `openspec/specs/d5-observability/span-registry.md`  | 56 (全部)    | all                                                                     |
| D7 Orchestration  | `openspec/specs/d7-orchestration/span-registry.md`  | 3          | orchestrator(3)                                                         |


---

## 快速索引（按 领域）


| Layer           | Operations                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  |
| --------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `communication` | `gateway.message.receive`, `gateway.session.lifecycle`, `gateway.session.create`, `gateway.session.get`, `gateway.session.expire`, `gateway.store.create`, `gateway.store.get`, `gateway.store.update`, `gateway.store.delete`, `gateway.permission.check`, `gateway.agent.create`, `gateway.engine_event.handle`, `adapter.message.receive`, `adapter.cli.send`, `adapter.feishu.outbound`                                                                                                                                                                                                                                                                                                                                 |
| `context`       | `context.process`, `context.snapshot.load`, `context.system_prompt.load`, `context.compression.run`, `context.compression.step`, `context.longterm.recall`, `context.longterm.store`, `context.tools.register`, `context.memory.snapshot.save`, `context.harness.bootstrap.run`, `context.harness.bootstrap.stage`, `context.harness.tool_pool`, `context.harness.preflight`, `context.harness.route`, `context.system_prompt.build`, `query.loop.run`, `query.loop.turn`, `query.loop.llm.call`, `tool.execute.single`, `tool.execute.permission`, `task.plan.generate`, `task.plan_mode.enter`, `task.plan_mode.execute`, `task.plan_mode.approve`, `task.plan_mode.reject`, `task.manager.create`, `task.manager.update` |
| `llm`           | `llm.stream`, `llm.provider.route`, `llm.circuit_breaker`, `llm.retry`, `llm.adapter.stream`                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                |
| `agent`         | `agent.tool.call`, `agent.run`, `agent.fork`, `agent.join`, `agent.terminate`, `agent.state.transition`                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     |
| `orchestration` | `orchestration.wave.schedule`, `orchestration.wave.task.execute`, `orchestration.flow.event.publish`                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        |


---

## 全局 Metrics 索引


| Metric                               | Type      | Labels                   | 写入域 |
| ------------------------------------ | --------- | ------------------------ | --- |
| `devrix_tool_latency`                | Histogram | tool, risk_level, status | D2  |
| `devrix_compression_ratio`           | Histogram | —                        | D2  |
| `devrix_gen_ai.client.token.usage`   | Counter   | token_type, model        | D3  |
| `devrix_active_sessions`             | Gauge     | adapter                  | D1  |
| `devrix_runtime_path_resolved_total` | Counter   | path                     | D5  |


---

## 关联文档

- Operation 常量定义：`internal/layers/observability/telemetry/names.go`
- Coverage Registry：`internal/layers/observability/coverage/registry.go`
- D5 全局 Trace Tree：`openspec/specs/d5-observability/span-registry.md`

