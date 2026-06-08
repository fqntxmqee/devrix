# Proposal: 可观察层运行时代码染色与 Operation 对账

**Change ID:** devrix-observability-coverage
**Demand ID:** DM-20260607-007
**Parent Spec:** observability v1.2.0
**Target Version:** 1.3.0
**Status:** S3 Planning
**Author:** Architecture
**Date:** 2026-06-07

---

## 1. Background

V1.2 完成 Jaeger Service/Operation 命名对齐，主请求路径 11 个 Operation 已埋点。Review 发现：

- **染色盲区**：LongTerm Memory、Plan/Milestone、Feishu Adapter 等模块线上运行但 trace 不可见
- **无对账机制**：无法系统化回答「哪些 Operation 在 N 天内从未命中」
- **指标分裂**：`SessionBridge` 已实现，Gateway 仍用 legacy `communication/metrics/collector.go`
- **L5 悬空**：`L5-OBS-01~12` 仍为 PLANNED，与归档 acceptance 不一致

本变更引入 **Operation 级运行时代码染色** 与 **Registry 对账**，为无效代码/闲置功能治理提供线上证据链。

---

## 2. Problem Statement

| 问题 | 影响 |
|------|------|
| 模块无 Span | 无法判断 longterm / plan / adapter 路径是否被使用 |
| 无 Operation 全集 | Jaeger 里「没出现」可能是未埋点，而非未使用 |
| 无命中计数 | 采样关闭后丢失 trace，无法统计功能切片热度 |
| Metrics 双轨 | 会话指标与 observability Bridge 不一致，对账困难 |

---

## 3. Alternatives Considered

| 方案 | 优点 | 缺点 | 决策 |
|------|------|------|------|
| A. Operation Registry + 进程内计数 | 低开销、采样无关、可测试 | 仅 Operation 粒度 | **采用** |
| B. Jaeger API 定期拉取对账 | 反映真实导出 | 依赖外部、采样偏差、运维复杂 | V1.4 |
| C. Go runtime coverage | 函数级精确 | 生产开销大、需 canary 部署 | Out of Scope |
| D. 仅补 Span 不做对账 | 实现快 | 无法批量产出「零命中」报告 | 不完整 |
| E. 不做 | 零成本 | 无效代码治理无线上证据 | 拒绝 |

---

## 4. What Changes

### 4.1 新增 L4 能力

| L4 ID | 名称 | 说明 |
|-------|------|------|
| L4-OBS-REGISTRY | Operation 注册表 | 所有 canonical operation 的静态清单与元数据 |
| L4-OBS-COVERAGE | Operation 对账 | 注册表 vs 运行时命中计数，输出 coverage 报告 |
| L4-OBS-INSTRUMENT | 扩展埋点 | 补全 P0 模块 Span |

### 4.2 新增 Operation（v1.3）

在 v1.2 的 11 个 Operation 基础上 **新增 6 个**：

| Operation | Layer | Component | 埋点位置 |
|-----------|-------|-----------|----------|
| `adapter.message.receive` | communication | adapter | `adapters/feishu.go` 入站 |
| `context.plan.generate` | context | context_engine | `pev/plan.go` PlanEngine.Generate |
| `context.milestone.run` | context | pev_engine | `pev/milestone_runner.go` Run |
| `context.longterm.recall` | context | context_engine | `memory/manager.go` EnrichWithLongTermRecall |
| `context.longterm.store` | context | context_engine | `memory/manager.go` AutoStoreLongTerm |
| `gateway.session.lifecycle` | communication | gateway | `gateway.go` create/expire（合并 create+expire） |

> v1.2 已有 Operation **不变**；`gateway.session.create` / `expire` 合并为 `gateway.session.lifecycle`，通过 `session.action` 属性区分。

### 4.3 Coverage 报告形态

```json
{
  "since": "2026-06-07T10:00:00Z",
  "operations_total": 17,
  "operations_hit": 12,
  "operations_zero_hit": [
    {"operation": "context.longterm.store", "layer": "context", "since_version": "1.3.0"}
  ],
  "coverage_ratio": 0.706
}
```

暴露方式：

- `GET /health/observability/coverage`（或并入现有 health）
- CLI：`go run ./cmd/obs-coverage-report`（开发/运维）

---

## 5. Capabilities

| Capability | L4 | L2 场景 |
|------------|-----|---------|
| OBS-REGISTRY | L4-OBS-REGISTRY | L2-OBS-COVERAGE |
| OBS-COVERAGE | L4-OBS-COVERAGE | L2-OBS-COVERAGE |
| OBS-INSTRUMENT | L4-OBS-INSTRUMENT | L2-OBS-TRACING |
| OBS-METRICS | L4-OBS-METRICS | L2-OBS-METRICS |

---

## 6. Impact

| 区域 | 变更 |
|------|------|
| `telemetry/names.go` | 新增 6 个 Op 常量 + Registry |
| `telemetry/registry.go` | **新文件** Operation 元数据 |
| `telemetry/coverage.go` | **新文件** 命中计数与报告 |
| `tracer/tracer.go` | Start 时调用 coverage.RecordHit |
| `contextengine/memory/manager.go` | longterm spans |
| `contextengine/pev/plan.go` | plan span |
| `contextengine/pev/milestone_runner.go` | milestone span |
| `communication/adapters/feishu.go` | adapter span |
| `communication/gateway/gateway.go` | session lifecycle span + SessionBridge |
| `observability.go` | HealthCheck 增加 coverage 摘要 |
| `openspec/specs/observability/spec.md` | v1.3.0 delta 合并 |
| `openspec/l5-registry.md` | L5-OBS-13~18 |

**无 Breaking Change**：YAML 配置向后兼容；新增 health 字段为 additive。

---

## 7. Scope

**In Scope**: 见 demand.md

**Out of Scope**: Go runtime coverage、pprof、Jaeger 远程对账、OTel 迁移

---

## 8. Goals (SLO)

| 指标 | 目标 |
|------|------|
| Operation Registry 覆盖率 | 100% 已埋点 Operation 登记 |
| 染色开销 | Start 路径增加 < 50ns（atomic inc） |
| P0 模块 Span | longterm / plan / milestone / feishu 4 模块有 span |
| L5 P0 | L5-OBS-13~17 全部有自动化测试 |
| 报告可用性 | 集成测试可断言 zero_hit 列表 |

---

## 9. L5 测试点（新增）

| L5 ID | 描述 | Priority | L4 |
|-------|------|----------|-----|
| L5-OBS-13 | LongTerm recall/store 触发时产生对应 Operation span | P0 | L4-OBS-INSTRUMENT |
| L5-OBS-14 | Plan 生成与 Milestone Run 产生对应 Operation span | P0 | L4-OBS-INSTRUMENT |
| L5-OBS-15 | Feishu 入站产生 `adapter.message.receive` span | P0 | L4-OBS-INSTRUMENT |
| L5-OBS-16 | Operation Registry 包含全部 canonical operations | P0 | L4-OBS-REGISTRY |
| L5-OBS-17 | Coverage 报告正确列出 zero_hit operations | P0 | L4-OBS-COVERAGE |
| L5-OBS-18 | Gateway 会话 Gauge 使用 SessionBridge | P1 | L4-OBS-METRICS |
