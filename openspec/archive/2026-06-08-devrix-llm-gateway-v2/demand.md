---
demand-id: DM-20260608-002
title: LLM Gateway 可靠性增强（CB+Retry+Jitter+Timeout）
source: LLM Gateway V1 生产可靠性 Review
priority: P0
status: S7_ARCHIVED
l1-domain: llmgateway
created: 2026-06-08
---

# LLM Gateway 可靠性增强

## 1. 原始描述

> LLM Gateway V1 的 CircuitBreaker 与 Retry 独立运作，重试失败会快速触发熔断；流式调用超时不传播 Context，可能导致 goroutine 泄漏；Retry 指数退避无 jitter，多实例部署时易产生同步重试风暴。

## 2. 澄清记录

### Q1: CB 与 Retry 如何协调？

**A**: Gateway 仅在 Retry 链整体失败后记录一次 CB failure；`context.Canceled` / `context.DeadlineExceeded` 不触发 CB failure。 — 2026-06-08

### Q2: Half-Open 探测并发？

**A**: 新增 `halfOpenInFlight` 与 `HalfOpenMaxProbes`（默认 1），超限请求立即返回 `CircuitOpenError`。 — 2026-06-08

### Q3: 超时策略？

**A**: 父 Context 无 deadline 时 Gateway 注入 provider 级 timeout（默认 60s）；已有 deadline 不覆盖。 — 2026-06-08

## 3. L1–L5 映射草案

| 层级 | 资产 |
|------|------|
| L1 | llmgateway |
| L3 | LLM 流式调用活动 |
| L4 | L4-LLM-GATEWAY, L4-LLM-BREAKER, L4-LLM-RETRY, L4-LLM-TOKEN |
| L5 | L5-LLM-20 ~ L5-LLM-23 |

## 4. 验收标准

- P0：CB+Retry 协调、Half-Open 探测限制、Context 取消不触发 CB
- P1：Provider 超时注入、Full Jitter 退避
- P2：CJK Token 补偿系数（可选配置）
