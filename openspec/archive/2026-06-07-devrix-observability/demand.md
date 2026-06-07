# Demand: Devrix 可观察层

**Demand ID:** DM-20260607-001
**Status:** Clarified
**Priority:** P1
**Created:** 2026-06-07
**Last Updated:** 2026-06-07

---

## 基本信息

| 字段 | 值 |
|------|---|
| 需求 ID | DM-20260607-001 |
| 来源 | 架构规划 / L5 层缺失 |
| 优先级 | P1 |
| 提出方 | 架构 |
| 关联 Issue | — |

---

## 原始描述

Devrix 项目当前缺少可观察性基础设施（Observability Layer），无法：
- 追踪消息从入口到 LLM 的完整调用链
- 量化 LLM 调用次数、Token 消耗、延迟等指标
- 关联日志与请求上下文（traceId）
- 导出 Prometheus 指标供监控系统消费

需要按照 OpenTelemetry 标准实现 Layer 5 可观察层，对齐 `openspec/project.md` 中定义的六层架构。

---

## 澄清记录

### Q1: 为什么不能使用现有的 metrics/collector.go？

**Q**: `internal/layers/communication/metrics/collector.go` 已有基础实现，为什么不直接扩展？

**A**: 现有实现是孤立的，仅服务 Communication 层。需要：
1. 统一 Tracer/Meter 接口，供所有层使用
2. 对齐 OpenTelemetry 标准，支持 OTLP 导出
3. 可观察模块应独立于业务层

**决策**: 重构为独立 `layers/observability/` 模块，复用 OTel 库。

---

### Q2: V1 是否需要支持 Jaeger/Zipkin？

**Q**: 是否 V1 就需要支持 Jaeger 等追踪系统？

**A**: V1 通过 Console Exporter 输出 JSON 格式 trace 数据，满足开发调试需求。V2 通过 OTLP Exporter 桥接到 Jaeger。

**决策**: V1 不内置 Jaeger，V2 支持 OTLP 导出。

---

### Q3: 如何避免对现有业务代码的侵入性修改？

**Q**: 在 Gateway、LLM Gateway、Tool Registry 中添加 Tracer 调用是否侵入性太强？

**A**: 通过依赖注入（Interface Injection）实现最小侵入：
- `CommunicationGateway` 持有 `Tracer` 接口
- `LLMGateway` 持有 `Tracer` + `Meter` 接口
- `ToolRegistry` 持有 `Tracer` + `Meter` 接口

**决策**: 接口注入，非侵入式。

---

## 澄清范围

### L1-L5 映射

| 层级 | ID | 名称 | 类型 | 说明 |
|------|----|------|------|------|
| L1 | L1-DEVRIX | Devrix 核心域 | 核心 | 可观察性属于基础设施域 |
| L5 | L5-OBS-01 ~ 12 | 可观察层测试点 | 验收锚点 | 见 `openspec/l5-registry.md` |

### 受影响组件

| 组件 | 影响说明 |
|------|----------|
| `internal/layers/communication/` | RouteInbound 创建 root span |
| `internal/layers/llmgateway/` | 记录 LLM metrics |
| `internal/layers/multiagent/` | 记录 tool metrics |
| `internal/layers/contextengine/` | V2 集成 |
| `cmd/devrix/main.go` | 初始化可观察层 |

### 范围定义

**In Scope (V1)**:
- Trace/Span 数据模型（OTel 兼容）
- Console Exporter（JSON 输出）
- Prometheus Metrics 端点
- Structured Logging（含 trace context）
- Graceful Shutdown（刷写 pending spans）
- Health Endpoint

**Out of Scope (V1)**:
- OTLP Exporter（V2）
- Jaeger/Zipkin 集成（V2）
- Tail-based Sampling（V3）
- APM 集成（V3）

---

## 验收标准

| ID | 描述 | 优先级 |
|----|------|--------|
| L5-OBS-01 | Trace ID 在消息入口生成 | P0 |
| L5-OBS-02 | Trace ID 传播至 LLM 调用 | P0 |
| L5-OBS-03 | LLM 调用记录 latency/token metrics | P0 |
| L5-OBS-04 | 结构化日志包含 traceId | P0 |
| L5-OBS-05 | Graceful shutdown 刷写 traces | P0 |

---

## 依赖关系

| 依赖需求 | 说明 |
|----------|------|
| DM-20260607-002 (Context Engine) | 并行开发，无依赖 |
| DM-20260607-003 (LLM Gateway) | 并行开发，无依赖 |

---

## 变更历史

| 日期 | 操作 | 说明 |
|------|------|------|
| 2026-06-07 | 创建 | 初始需求登记 |
| 2026-06-07 | 澄清 | 完成 Q1-Q3 澄清 |
