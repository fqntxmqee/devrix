# Devrix Spans 注册表（全局索引）

**Status:** Active
**Version:** 3.0.0
**Last Updated:** 2026-06-14
**Canonical Source:** `internal/layers/observability/telemetry/names.go` · `internal/layers/observability/coverage/registry.go`

---

## 命名规范：`D{N}_{场景名称}_{动作}_{细节}`

| 段 | 含义 | DSAFT 对应 | 示例 |
|----|------|-----------|------|
| `D{N}` | 域编号 | D 层 | `D1`, `D7` |
| `{场景名称}` | 场景语义名（非 S 编号） | S 层 | `Capture`, `Orchestration` |
| `{动作}` | 业务动作 | A 层 | `Message`, `Turn` |
| `{细节}` | 操作细节（可多层） | F 层 | `Receive`, `Run` |

**重要：不在运行时字符串中插入 S 编号。** 场景名称已唯一标识场景，`D1_S13_Capture_...` 中 `S13` 与 `Capture` 重复。Go 常量名保留 `OpD{N}_S{N}_...` 格式用于 DSAFT 追溯。

**格式规则：**
- 全部大写，`_` 分隔
- 示例：`D1_Capture_Message_Receive` → D1 + Capture 场景 + Message 动作 + Receive 细节

---

## Overview

本文档为 Devrix 全局 Span / Operation 注册表的索引入口。各域的 Span 注册表已拆分为独立文件。

**总计：57 Operations，6 Layers，12 Components**

---

## 域级 Span 注册表


| 域                 | 路径                                                  | Operations | Components                                                              |
| ----------------- | --------------------------------------------------- | ---------- | ----------------------------------------------------------------------- |
| D1 Communication  | `openspec/specs/d1-communication/span-registry.md`  | 15         | gateway(12), adapter(3)                                                 |
| D2 Context Engine | `openspec/specs/d2-context-engine/span-registry.md` | 27         | context_engine(9), harness(6), query_loop(3), tool_runner(2), plan_*(7) |
| D3 LLM Gateway    | `openspec/specs/d3-llm-gateway/span-registry.md`    | 5          | llm_gateway(4), llm_adapter(1)                                          |
| D4 Multi-Agent    | `openspec/specs/d4-multi-agent/span-registry.md`    | 6          | agent_tool(6)                                                           |
| D5 Observability  | `openspec/specs/d5-observability/span-registry.md`  | 57 (全部)    | all                                                                     |
| D6 Evolution      | `openspec/specs/d6-evolution/span-registry.md`      | 1          | validation(1)                                                           |
| D7 Orchestration  | `openspec/specs/d7-orchestration/span-registry.md`  | 3          | orchestrator(3)                                                         |




## 全局 Metrics 索引


| Metric                               | Type      | Labels                   | 写入域 |
| ------------------------------------ | --------- | ------------------------ | --- |
| `devrix_tool_latency`                | Histogram | tool, risk_level, status | D2  |
| `devrix_compression_ratio`           | Histogram | —                        | D2  |
| `devrix_gen_ai.client.token.usage`   | Counter   | token_type, model        | D3  |
| `devrix_active_sessions`             | Gauge     | adapter                  | D1  |
| `devrix_runtime_path_resolved_total` | Counter   | path                     | D5  |


