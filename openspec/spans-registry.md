# Devrix Spans 注册表（全局索引）

**Status:** Active
**Version:** 3.0.0
**Last Updated:** 2026-06-14
**Canonical Source:** `internal/layers/observability/telemetry/names.go` · `internal/layers/observability/coverage/registry.go`

---

## 命名规范 {D}{S}{A}_{F}

所有 Span Operation 名称遵循 `{D}{S}_{A}_{F}` 格式：

| 组件 | 说明 | 示例 |
|------|------|------|
| D | 域编号 | D1, D2, D3, D4, D7 |
| S | Scenario 场景名 | Capture, Context, LLM, Agent, Orchestration |
| A | Activity 活动名 | Process, Stream, Route, Execute |
| F | Function 功能名（可选） | Receive, Load, Schedule |

**格式规则：**
- 全部大写
- 组件间用下划线分隔
- 示例：`D1_Capture_Message_Receive` → D1 + Capture + Message + Receive

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




## 全局 Metrics 索引


| Metric                               | Type      | Labels                   | 写入域 |
| ------------------------------------ | --------- | ------------------------ | --- |
| `devrix_tool_latency`                | Histogram | tool, risk_level, status | D2  |
| `devrix_compression_ratio`           | Histogram | —                        | D2  |
| `devrix_gen_ai.client.token.usage`   | Counter   | token_type, model        | D3  |
| `devrix_active_sessions`             | Gauge     | adapter                  | D1  |
| `devrix_runtime_path_resolved_total` | Counter   | path                     | D5  |


