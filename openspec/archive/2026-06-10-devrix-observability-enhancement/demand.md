---
demand-id: DM-20260610-001
title: Devrix 可观察层增强 — AI 排查就绪
source: 技术团队 / observability-design 深度分析 + Code Review（2026-06-10）
priority: P1
status: ACCEPTED
l1-domain: observability
created: 2026-06-10
---

# Devrix 可观察层增强 — AI 排查就绪

## 1. 原始描述

> Devrix 可观察层（Tracer / Metrics / Logger / Coverage）已具备基础能力，但经深度分析与代码对照发现：
> 1. 设计文档与代码实现严重脱节（大量 P0 埋点已实现，但 proposal 仍标记 TODO）
> 2. 从 **核心链路调用可视化** 和 **AI 辅助排查** 角度，现有能力不足以支撑 AI 自主 RCA
>
> 需求目标从「补埋点、提覆盖率」升级为：**让 AI（及人类）能基于可观察数据可靠还原单次请求的因果链并完成根因分析**。

## 2. 澄清记录

### Q1: 当前代码与 proposal 是否一致？
**A**: 不一致。截至 2026-06-10 代码审查：
- ✅ 已实现：`AddLLMRequestEvent/ResponseEvent`、`context.pev.iteration`、`context.compression.run`、`context.longterm.*`、`context.pev.synthesis`、`context.plan.generate`、`context.milestone.run`、`llm.adapter.stream`；Registry 共 44 个 operation
- ❌ 未实现：`tool_latency` / `compression_ratio` metrics；Baggage 业务接入；Span 层级契约；Log-Trace 关联；Session incident 导出
- ⚠️ 有缺陷：PEV 循环 `ctx` 未向下传递；循环内 `defer iterSpan.End()`；`ChatStream` 未使用 `llmSpan` ctx — Jaeger 火焰图扁平化

### Q2: 「满足 AI 未来排查」的验收标准是什么？
**A**: 分三档（见 proposal §Goals）：
- **L1 辅助**：人工/AI 读 Jaeger + LLM JSONL，中等复杂度问题可分析 → 当前 ~70%
- **L2 基本满足**：给定 session_id，AI 可还原完整因果链（trace 树 + LLM 轮次 + 关联日志）→ 需本需求 P0/P1 完成后
- **L3 自主闭环**：AI Agent 自动 export → analyze → 输出 RCA → 需 L2 + error-biased sampling + Agent 路径统一（部分 Out of Scope）

### Q3: Baggage 是否必须做？
**A**: 单进程 monolith 场景下，span attributes 已携带 `session.id`；Baggage 降为 **P2**，等多服务拆分或 adapter 独立进程时再启用 OTel 标准 propagation。

### Q4: 覆盖率 ≥80% 是否仍是核心目标？
**A**: 降级为 **P2 辅助指标**。Registry 静态注册已基本完成；Runtime Hit 受流量路径影响（compression 不触发则 zero-hit）。验收重心转为 **Span 层级契约** 和 **AI incident 可读性**。

### Q5: 谁 Review 过本需求？
**A**: Cursor Agent 于 2026-06-10 完成首轮 review（核心链路可视化 + AI 排查就绪评估），结论已写入本 change 全套文档，供其他模型二次 review。

### Q6: 业界最佳实践对照后的补充建议？
**A**: 2026-06-10 二次 review 参考 OTel GenAI 语义公约、SigNoz、Uptrace、FutureAGI 等 2025-2026 实践，补充以下 7 项：

| # | 建议 | 优先级 | 理由 |
|---|------|--------|------|
| R1 | LLM span 双写 `gen_ai.*` 语义属性（与自定义属性并存） | P0 | 与 OTel GenAI 生态互操作，避免未来迁移成本 |
| R2 | 明确 SpanKind（CLIENT/INTERNAL/SERVER） | P1 | Jaeger UI 呈现质量 |
| R3 | 错误记录对齐 OTel 标准（`span.RecordError` + `SetStatus`） | P0 | 当前缺口导致 AI 无法区分正常错误 vs 崩溃 |
| R4 | Prompt 版本哈希标记（`gen_ai.prompt.version` / `template_hash`） | P1 | AI 排查入口：判断 prompt 是否与 release 一致 |
| R5 | Token 类型细化（cache_read / reasoning 拆分） | P1 | 成本分析与 prompt cache 效率评估 |
| R6 | 采样策略文档（tail-sampling 规划，当前全采） | P2 | 避免 OTLP export 成本失控 |
| R7 | Incident export schema 增加 eval_scores / prompt_versions | P1 | AI 排查需要评分上下文 |

## 3. 澄清范围

### 3.1 L1-L5 映射

| 层级 | 资产 ID | 名称 | 状态 |
|------|---------|------|------|
| L1 | observability | 可观察层 | 已有 |
| L2 | L2-OBS-DEBUG | 问题排查与 RCA | 草拟 |
| L3-BE | L3-BE-OBS-TRACE | 分布式追踪 | 已有 |
| L3-BE | L3-BE-OBS-EXPORT | Session Incident 导出 | 新增 |
| L4-BE | L4-OBS-SPAN-TREE | Span 层级契约 | 新增 |
| L4-BE | L4-OBS-LOG-CORR | Log-Trace-LLM 关联 | 新增 |
| L4-BE | L4-OBS-TOOL-METRICS | 工具延迟 Metrics | 新增 |
| L4-BE | L4-OBS-DECISION-ATTR | 决策语义 Span 属性 | 新增 |
| L4-BE | L4-OBS-GENAI-ATTR | GenAI 语义属性标准化 | 新增 |
| L4-BE | L4-OBS-SPANKIND | SpanKind 标注 | 新增 |
| L4-BE | L4-OBS-ERROR-REC | 错误记录模式（RecordError + SetStatus） | 新增 |
| L5 | L5-OBS-TRACE-01 | LLM 输入输出完整 trace | 已有（代码） |
| L5 | L5-OBS-TRACE-02 | PEV 迭代独立 span | 已有（代码，层级待修） |
| L5 | L5-OBS-TRACE-04 | Span 父子层级契约 | 新增 |
| L5 | L5-OBS-TRACE-05 | Log-Trace 关联 | 新增 |
| L5 | L5-OBS-TRACE-06 | SpanKind 合规 | 新增 |
| L5 | L5-OBS-TRACE-07 | 错误记录 OTel 标准一致 | 新增 |
| L5 | L5-OBS-METRICS-01 | tool_latency histogram | 新增 |
| L5 | L5-OBS-METRICS-02 | compression_ratio histogram | 新增 |
| L5 | L5-OBS-METRICS-03 | token_usage breakdown（cache/reasoning） | 新增 |
| L5 | L5-OBS-EXPORT-01 | Session incident bundle 导出 | 新增 |
| L5 | L5-OBS-DECISION-01 | Verify 失败原因可观测 | 新增 |
| L5 | L5-OBS-DECISION-02 | Prompt 版本可追溯 | 新增 |

### 3.2 In Scope

1. **P0** — Span ctx 传播规范落地 + PEV 循环 defer 修复 + 层级集成测试
2. **P0** — slog 自动注入 trace_id / session_id；LLM JSONL 增加 trace_id
3. **P0** — LLM span 双写 OTel `gen_ai.*` 语义属性（gen_ai.request.model, gen_ai.usage.*, gen_ai.response.finish_reasons），与自定义属性并存
4. **P0** — 错误记录对齐 OTel 标准：`span.RecordError` + `span.SetStatus(codes.Error)`
5. **P1** — `tool_latency` / `compression_ratio` metrics
6. **P1** — 决策语义 span 属性（verify 失败原因、compression 触发原因）
7. **P1** — Session incident export（CLI 或 API，JSON 格式），含 eval_scores + prompt_versions 字段
8. **P1** — SpanKind 标注（CLIENT / INTERNAL / SERVER）
9. **P1** — Prompt 版本哈希标记（`gen_ai.prompt.version`, `gen_ai.prompt.template_hash`）
10. **P1** — Token 类型细化：cache_read / reasoning tokens 分开记录
11. **P2** — 文档与代码对齐；Runtime coverage 分 Layer 统计
12. **P2** — Baggage（可选，等多服务需求）
13. **P2** — 采样策略文档（当前全采，规划 tail-sampling 触发条件）

### 3.3 Out of Scope

- 新增 exporter（OTLP/Prometheus 已有）
- 前端 Jaeger UI 定制
- Error-biased sampling（独立 change）
- Agent 路径 trace 统一（独立 change：`devrix-observability-agent-trace`）
- 长期 trace 存储 / 采样策略生产调优

## 4. AI 排查就绪评估（Review 结论摘要）

| 能力维度 | 现状评分 | 目标 |
|----------|----------|------|
| 完整因果链 (trace tree) | ■■■■□□ | P0 修复 ctx 传播 |
| LLM 决策可还原 | ■■■■■□ | 已有 JSONL + span events |
| Tool I/O 可还原 | ■■■□□□ | span preview 500 字符，需 JSONL 关联 |
| Log-Trace-Metric 关联 | ■■□□□□ | P0 slog 注入 trace_id |
| 结构化机器导出 | ■■□□□□ | P1 incident bundle |
| 聚合异常检测 | ■■■□□□ | P1 tool_latency |
| 多路径一致性 | ■■■□□□ | Out of Scope（Agent） |

**Verdict**: 底座可用，**不足以支撑 AI 自主排查**；本需求聚焦「可推理因果链 + 可关联多源信号 + 可机器消费」。
