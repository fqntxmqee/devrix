---
demand-id: DM-20260610-002
title: Devrix 可观察层增强 P1 — Metrics 与 Incident Export
source: devrix-observability-enhancement P0 归档后续
priority: P1
status: ACCEPTED
l1-domain: observability
created: 2026-06-10
parent-demand: DM-20260610-001
---

# Devrix 可观察层增强 P1

## 1. 背景

P0（DM-20260610-001）已交付：PEV span 层级、Log-Trace 关联、gen_ai.* 属性、verify.failure_reason。

P1 剩余缺口（见归档 `tasks.md`）：
- `tool_latency` / `compression_ratio` Histogram
- 决策语义：`compression.trigger_reason`、`compression.ratio`
- Session incident export CLI

## 2. 验收标准（L5）

| L5 ID | Given-When-Then | 优先级 |
|-------|-----------------|--------|
| L5-OBS-METRICS-01 | Given 工具执行完成 When scrape metrics Then `devrix_tool_latency` 有观测值 | P0 |
| L5-OBS-METRICS-02 | Given 上下文压缩触发 When scrape metrics Then `devrix_compression_ratio` 有观测值 | P0 |
| L5-OBS-DECISION-01 | Given 压缩运行 When 查看 span Then 含 `compression.trigger_reason` 与 `compression.ratio` | P1 |
| L5-OBS-EXPORT-01 | Given session LLM JSONL When `debug-export --session` Then 输出合法 JSON bundle v1 | P1 |

## 3. 范围外

- SpanKind 全面审计（T3b）
- Prompt 版本 hash（T3c）
- gen_ai.client.token.usage Counter（T4.5）
- Baggage（P2）
