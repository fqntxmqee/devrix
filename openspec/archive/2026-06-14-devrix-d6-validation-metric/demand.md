---
demand-id: DM-20260614-002
title: D6 校验指标可观测性 — 4 counter + timeout 告警
source: devrix-d7-orchestration-domain R2 决议 P1 #6
priority: P1
status: S1_Requirement
dsaft_domain: D7, D5
created: 2026-06-14
last-updated: 2026-06-14
---

# D6 校验指标可观测性 — 4 counter + timeout 告警

## 1. 原始描述

`devrix-d7-orchestration-domain` R2 决议 §5 P1 第 6 项明确指出：

> D6 advisory "50ms 超时 = pass" 是沉默同意反模式。超时计为 pass 时，校验层故障会被静默掩盖。

**现状**：

- `internal/layers/d7/orchestrator.go:106-110` 调用 `D6Validator.ValidateOrchestration()`，结果被 `_ =` 丢弃。
- 调用未计时，无法区分 pass / fail / timeout / error。
- D5 `observability` 层无 `orchestration.d6.validation.*` counter。
- 运维侧无 `timeout_rate > 5%` 告警规则。

**目标**：让 D6 advisory 校验从"沉默同意"变为"可观测可告警"。具体：

1. 4 个 counter 注入 D5：
   - `orchestration.d6.validation.pass`
   - `orchestration.d6.validation.fail`
   - `orchestration.d6.validation.timeout`
   - `orchestration.d6.validation.error`
2. 调用方计时：超 50ms 视为 timeout，超 100ms 视为 error（兜底阈值）。
3. In-process timeout_rate 计算（滑窗 5min）+ > 5% 触发 `WARN` log 与可订阅告警事件。
4. 全链路在 D6 无 validator 时降级为 no-op（保持向后兼容）。

## 2. 范围

| 域 | 改动 |
|------|------|
| D7 (`internal/layers/d7/`) | 计时 + 分流 + 4 counter 注入 |
| D5 (`internal/layers/observability/`) | 新增 `D6ValidationMetrics` 结构 + 滑窗 rate |
| 跨域 | `IOrchestrationEntry` 扩展 `OnValidationResult` hook（D5 侧订阅） |

## 3. 不在范围

- 真实 D6 validator 实现（仍是接口，调用方注入 fake/no-op）
- Prometheus AlertManager rules yaml 推送（D5 exporter 已支持 counter 输出，运维侧接入不在 v1.0 P1 范围）
- 历史数据回溯（counter 启动清零）

## 4. 验收

- T 点（待 S3 拆）：D7-D6-T01 已存在但 PLANNED，本 change 落为 IMPLEMENTED
- 新增 D7-D6-T03 / D7-D6-T04（counter 注入 + 告警）
- 单元测试覆盖：pass/fail/timeout/error 四路径 + rate 计算 + 滑窗失效
- d7 包覆盖率维持 ≥ 80%
- 4 counter 在 Prometheus 导出器中可见（MemoryExporter / PrometheusExporter 路径）
