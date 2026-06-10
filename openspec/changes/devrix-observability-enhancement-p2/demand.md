---
demand-id: DM-20260610-003
title: Devrix 可观察层增强 P2 — SpanKind / Prompt Hash / Token Metrics / Debug CLI
source: devrix-observability-enhancement P1 归档后续
priority: P2
status: ACCEPTED
l1-domain: observability
created: 2026-06-10
parent-demand: DM-20260610-002
---

# Devrix 可观察层增强 P2

## 背景

P1 已交付 metrics histogram、incident export（`cmd/debug-export`）。P2 补齐 AI 排查剩余能力。

## L5 验收

| L5 ID | 描述 | 优先级 |
|-------|------|--------|
| L5-OBS-TRACE-06 | 关键 span SpanKind 契约（SERVER/CLIENT/INTERNAL） | P1 |
| L5-OBS-DECISION-03 | `gen_ai.prompt.version` + `template_hash` on system prompt build | P1 |
| L5-OBS-METRICS-03 | `gen_ai.client.token.usage` Counter 按 token_type | P1 |
| L5-OBS-EXPORT-02 | `devrix debug export --session` 子命令 | P2 |

## 范围外

- Baggage（P2 deferred from P0）
- OTLP tail-sampling 实现
