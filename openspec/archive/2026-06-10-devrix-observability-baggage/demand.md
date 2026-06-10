---
demand-id: DM-20260610-005
title: 可观察层 P3 — W3C Baggage 业务上下文传播
source: observability P0-P2 deferred Baggage
priority: P2
status: ACCEPTED
l1-domain: observability
created: 2026-06-10
parent-demand: DM-20260610-004
---

# Baggage 业务上下文传播

`baggage.go` 已实现但未接入 propagator 与业务入口。本 change 启用 W3C `baggage` 头传播，并在 gateway 入口与 CLI agent 子进程间传递 `session.id` / `user.id`。

## 验收

- Propagator Inject/Extract 往返 `baggage` 头
- Gateway 入站请求设置 baggage 并与 span attributes 一致
- CLI agent 子进程继承 `TRACEPARENT` + `BAGGAGE` 环境变量
- L5-OBS-TRACE-03 测试通过

## 范围外

- OTLP tail-sampling
- `cache_read` / `reasoning` token metrics
