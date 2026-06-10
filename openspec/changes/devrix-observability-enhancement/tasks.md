# Tasks: Devrix 可观察层增强 — AI 排查就绪

**Change ID:** devrix-observability-enhancement
**Demand ID:** DM-20260610-001
**Status:** S4 In Progress (P0 complete 2026-06-10)
**Review Note:** T1-T3 大部分已在代码完成；本版重排为 P0 层级修复 + P1 AI 就绪 + P2 文档

---

## 代码基线（已完成，无需重复开发）

| ID | Task | Status | Evidence |
|----|------|--------|----------|
| T0.1 | LLM span events 接入 | **DONE** | `pev_engine.go:273/353/558` |
| T0.2 | PEV iteration span | **DONE** | `pev_engine.go:253` |
| T0.3 | compression/recall/store span | **DONE** | `engine.go:188-240, 314` |
| T0.4 | synthesis/plan/milestone span | **DONE** | `pev_engine.go:146, 186, 554` |
| T0.5 | llm.adapter.stream span | **DONE** | `gateway.go:194` |
| T0.6 | Registry 44 operations | **DONE** | `coverage/registry.go` |
| T0.7 | active_sessions gauge | **DONE** | `bridge.go:164`, `gateway.go:131` |
| T0.8 | 集成测试 span 名称存在性 | **DONE** | `full_chain_trace_test.go` |

---

## Task 1: Span 层级修复（P0）— L5-OBS-TRACE-04

**Change:** 修复 ctx 传播 + loop defer，满足 Canonical Trace Tree
**Files:** `contextengine/pev_engine.go`
**Effort:** 5h
**Covers:** L5-OBS-TRACE-02, L5-OBS-TRACE-04, L5-OBS-TRACE-07

| ID | Task | Status | Notes |
|----|------|--------|-------|
| T1.1 | PEV 循环改用 `ctx, iterSpan :=` 并向下传递 | **DONE** | iterCtx 传播 |
| T1.2 | 移除 loop 内 `defer iterSpan.End()`，改为轮末 End | **DONE** | endIterSpan() |
| T1.3 | `llmSpan` ctx 传入 `ChatStream` | **DONE** | llmCtx |
| T1.4 | synthesis 路径同样修复 ctx 传播 | **DONE** | runToolSynthesis |
| T1.5 | 集成测试验证 parent-child（R1-R5） | **DONE** | obs_pev_span_hierarchy_test.go |
| T1.6 | PEV 循环 error 路径 `RecordError` + `SetStatus` | **DONE** | llm + synthesis 错误路径 |
| T1.7 | 集成测试断言错误 span 含 exception event | TODO | 后续补 error path 子测试 |

---

## Task 2: Log-Trace-LLM 关联（P0）— L5-OBS-TRACE-05

**Change:** 三轨信号可关联
**Files:** `observability/logger/`, `observability/llm_log.go`, `contextengine/llm_logger.go`
**Effort:** 4h

| ID | Task | Status | Notes |
|----|------|--------|-------|
| T2.1 | 实现 slog Handler 从 ctx 注入 traceId/spanId | **DONE** | logger/slog_bridge.go |
| T2.2 | bootstrap 注册 slog bridge | **DONE** | InstallSlogBridge + main.go |
| T2.3 | LLM JSONL 增加 trace_id / span_id 字段 | **DONE** | llm_log.go |
| T2.4 | `AddLLMRequestEvent` 传入 span context 写 JSONL | **DONE** | RecordLLMSpanPayload |
| T2.5 | 单元测试：log 含 traceId；JSONL 含 trace_id | **DONE** | slog_bridge_test + llm_log_test |

---

## Task 3: 决策语义属性（P1）— L5-OBS-DECISION-01

**Change:** span 携带「为什么」语义
**Files:** `contextengine/pev_engine.go`, `contextengine/engine.go`
**Effort:** 2h

| ID | Task | Status | Notes |
|----|------|--------|-------|
| T3.1 | verify 失败写入 `verify.failure_reason` | **DONE** | pev_engine.go |
| T3.2 | compression 写入 `compression.trigger_reason` | TODO | |
| T3.3 | synthesis/fallback 写入 `pev.synthesis_source` | TODO | |
| T3.4 | 错误 span 统一 `error.code` | TODO | SentinelError |
| T3.5 | 集成测试验证属性存在 | TODO | |

---

## Task 3a: `gen_ai.*` 语义属性双写（P0）— L5-OBS-GENAI-ATTR

**Change:** LLM span 双写 OTel 标准 `gen_ai.*` 属性，与自定义属性并存
**Files:** `contextengine/pev_engine.go`, `observability/telemetry/names.go`
**Effort:** 1h
**Covers:** L5-OBS-DECISION-02

| ID | Task | Status | Notes |
|----|------|--------|-------|
| T3a.1 | LLM call span 增加 `gen_ai.*` 属性 | **DONE** | GenAIUsageAttrs |
| T3a.2 | 增加 `gen_ai.agent.name`, `gen_ai.conversation.id` | **DONE** | GenAIUsageAttrs |
| T3a.3 | 集成测试断言 gen_ai.* 属性存在 | **DONE** | obs_pev_span_hierarchy_test |

---

## Task 3b: SpanKind 标注（P1）— L5-OBS-SPANKIND

**Change:** 各 span 按 OTel 规范标注 SpanKind
**Files:** `contextengine/pev_engine.go`, `observability/telemetry/names.go`, 各 startSpan 调用处
**Effort:** 1h
**Covers:** L5-OBS-TRACE-06

| ID | Task | Status | Notes |
|----|------|--------|-------|
| T3b.1 | `startSpan` 签名扩展 `opts ...tracer.SpanOption` | TODO | |
| T3b.2 | CLIENT 标记：`llm.adapter.stream`, `context.pev.llm_call` | TODO | 跨进程 LLM API 调用 |
| T3b.3 | SERVER 标记：`gateway.message.receive` | TODO | 接收外部请求 |
| T3b.4 | INTERNAL 标记：其余所有 operation | TODO | 本进程内操作 |
| T3b.5 | 集成测试断言 SpanKind 正确 | TODO | |

---

## Task 3c: Prompt 版本标记（P1）— L5-OBS-DECISION-02

**Change:** system prompt build span 记录版本哈希
**Files:** `contextengine/harness/system_prompt_assembler.go` 或等价位置
**Effort:** 0.5h
**Covers:** L5-OBS-DECISION-02

| ID | Task | Status | Notes |
|----|------|--------|-------|
| T3c.1 | `SystemPromptAssembler.Build` 求四层模板内容 hash | TODO | |
| T3c.2 | span 写入 `gen_ai.prompt.version`, `gen_ai.prompt.template_hash` | TODO | |
| T3c.3 | 单元测试：相同输入产生相同 hash | TODO | |

---

## Task 4: Metrics 补齐（P1）— L5-OBS-METRICS-01/02/03

**Change:** 真正缺失的 histogram
**Files:** `observability/bridge.go`, `contextengine/pev_engine.go`, `contextengine/engine.go`
**Effort:** 4h
**Covers:** L5-OBS-METRICS-01, L5-OBS-METRICS-02, L5-OBS-METRICS-03

| ID | Task | Status | Notes |
|----|------|--------|-------|
| T4.1 | `ToolBridge.InitLatencyMetrics` + `tool_latency` | TODO | labels: tool, risk_level, status |
| T4.2 | PEV 工具执行处 Observe latency | TODO | |
| T4.3 | `compression_ratio` Histogram | TODO | engine.go 压缩成功后 |
| T4.4 | 单元/集成测试验证 metrics 输出 | TODO | |
| T4.5 | `gen_ai.client.token.usage` Counter 按 token_type 分别 Add | TODO | input/output/cache_read/reasoning |
| ~~T4.6~~ | ~~session_active_count~~ | **DONE** | 已有 `active_sessions` |

---

## Task 5: Session Incident Export（P1）— L5-OBS-EXPORT-01

**Change:** AI 友好 JSON bundle
**Files:** `cmd/debug/` 或 `cmd/coverage/` 扩展
**Effort:** 5h
**Covers:** L5-OBS-EXPORT-01

| ID | Task | Status | Notes |
|----|------|--------|-------|
| T5.1 | 定义 incident bundle schema v1 | TODO | 见 design.md §7.2 |
| T5.2 | 实现 trace 树序列化（从 exporter/memory 或 OTLP 读） | TODO | |
| T5.3 | 合并 LLM JSONL rounds | TODO | 按 session_id |
| T5.4 | CLI `devrix debug export --session` | TODO | |
| T5.5 | export 包含 eval_scores / prompt_versions 字段 | TODO | 见 design.md §7.2 |
| T5.6 | `coverage_hits` 每条含 operation/layer/hit_count/last_hit | TODO | |
| T5.7 | 集成测试：export 输出合法 JSON + schema 校验 | TODO | |

---

## Task 6: Baggage（P2 — 降级，可选）

**Status:** **DEFERRED** — 等多服务需求

| ID | Task | Status | Notes |
|----|------|--------|-------|
| T6.1 | Baggage 业务接入 | DEFERRED | 见 design.md §8 |

---

## Task 7: 文档与 Registry 同步（P2）

**Effort:** 2h

| ID | Task | Status | Notes |
|----|------|--------|-------|
| T7.1 | 更新 `docs/observability-design.md` Canonical Trace Tree | TODO | |
| T7.2 | 更新 `docs/coverage.md` 层级验收说明 | TODO | |
| T7.3 | 登记 L5-OBS-TRACE-04/05 等到 `openspec/l5-registry.md` | TODO | |
| T7.4 | 同步 proposal/design 与代码 DONE 状态 | TODO | 本 task 文档 |
| T7.5 | 撰写采样策略文档（当前全采 + tail-sampling 触发条件） | TODO | `docs/observability-design.md` §采样策略 |

---

## Task 8: Runtime Coverage 分 Layer 统计（P2）

| ID | Task | Status | Notes |
|----|------|--------|-------|
| T8.1 | 按 Layer 输出 coverage report | TODO | 不设全局 80% 硬门槛 |
| T8.2 | 文档说明低频 operation 排除规则 | TODO | agent.* 等 |

---

## Task Summary（修订）

| Priority | Tasks | Effort | 说明 |
|----------|-------|--------|------|
| **P0** | T1, T1.6-1.7, T2, T3a | 10h | AI RCA 核心 blocker + gen_ai.* 双写 + 错误记录标准 |
| **P1** | T3, T3b, T3c, T4, T5 | 10.5h | 决策语义 + SpanKind + prompt 版本 + metrics + export |
| **P2** | T6(DEFERRED), T7, T7.5, T8 | 2.5h | 文档 + coverage + 采样策略 |
| ~~已完成~~ | T0.* | — | 勿重复开发 |
| **Total 剩余** | | **~23h** | |

---

## 二次 Review 检查清单

供其他模型逐项验证：

- [ ] 是否认同「代码基线 T0.* 已完成」的判断？
- [ ] Canonical Trace Tree R1-R5 是否合理？
- [ ] P0 是否应包含 Log-Trace 关联（T2）？
- [ ] P0 是否应包含 `gen_ai.*` 语义属性双写（T3a）？
- [ ] P0 是否应包含错误记录 OTel 标准对齐（T1.6-1.7）？
- [ ] Incident export schema（含 eval_scores + prompt_versions）是否足够 AI 消费？
- [ ] SpanKind 标注（T3b）是否为 P1 合适？
- [ ] Baggage 降级 P2 是否可接受？
- [ ] 采样策略仅文档，不实现，是否可接受？
- [ ] Agent 路径 Out of Scope 是否需要独立 demand？
