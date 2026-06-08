# Tasks: devrix-observability-coverage

**Change ID:** devrix-observability-coverage
**Demand ID:** DM-20260607-007
**Target Version:** 1.3.0
**Status:** S7 — Archived 2026-06-08

---

## Milestone M1: Operation Registry + Coverage 基础设施

| ID | 任务 | L4 | L5 | 估时 | 状态 |
|----|------|-----|-----|------|------|
| T1 | 扩展 `telemetry/names.go`：6 个新 Op 常量 + `LayerAndComponent` 分支 | L4-OBS-REGISTRY | L5-OBS-16 | 1h | completed |
| T2 | 新增 `coverage/registry.go`：`OperationMeta` + `AllOperations()` | L4-OBS-REGISTRY | L5-OBS-16 | 2h | completed |
| T3 | 新增 `coverage/registry_test.go`：常量与 registry 一致性断言 | L4-OBS-REGISTRY | L5-OBS-16 | 1h | completed |
| T4 | 新增 `coverage/coverage.go`：`RecordHit` + `Report` + `Global` | L4-OBS-COVERAGE | L5-OBS-17 | 2h | completed |
| T5 | 修改 `tracer/tracer.go`：Start 时 RecordHit + unknown op WARN | L4-OBS-COVERAGE | L5-OBS-17 | 1h | completed |
| T6 | 新增 `coverage/coverage_test.go` + `tracer_coverage_test.go` | L4-OBS-COVERAGE | L5-OBS-17 | 2h | completed |

**M1 小计**: 9h

---

## Milestone M2: P0 模块 Span 补全

| ID | 任务 | L4 | L5 | 估时 | 状态 |
|----|------|-----|-----|------|------|
| T7 | `engine.go`：longterm recall/store span | L4-OBS-INSTRUMENT | L5-OBS-13 | 2h | completed |
| T8 | `pev_engine.go`：plan generate span | L4-OBS-INSTRUMENT | L5-OBS-14 | 1h | completed |
| T9 | `pev_engine.go`：milestone run span | L4-OBS-INSTRUMENT | L5-OBS-14 | 1h | completed |
| T10 | `adapters/feishu.go`：adapter 入站 span + ctx 传播 | L4-OBS-INSTRUMENT | L5-OBS-15 | 2h | completed |
| T11 | 确保 Feishu→Gateway traceId 继承（集成测试） | L4-OBS-INSTRUMENT | L5-OBS-15 | 1h | completed |

**M2 小计**: 7h

---

## Milestone M3: 报告暴露 + Health 集成

| ID | 任务 | L4 | L5 | 估时 | 状态 |
|----|------|-----|-----|------|------|
| T12 | `observability.go`：HealthCheck 增加 coverage 摘要 | L4-OBS-COVERAGE | L5-OBS-17 | 1h | completed |
| T13 | 新增 `cmd/obs-coverage-report/main.go`（JSON 输出） | L4-OBS-COVERAGE | L5-OBS-17 | 1h | completed |
| T14 | 新增 `tests/integration/obs_coverage_test.go` | L4-OBS-COVERAGE | L5-OBS-13~17 | 3h | completed |

**M3 小计**: 5h

---

## Milestone M4: Metrics 统一（P1）

| ID | 任务 | L4 | L5 | 估时 | 状态 |
|----|------|-----|-----|------|------|
| T15 | Gateway 迁 SessionBridge；`gateway.session.lifecycle` span | L4-OBS-METRICS | L5-OBS-18 | 2h | completed |
| T16 | `collector.go` deprecate + 转发注释 | L4-OBS-METRICS | L5-OBS-18 | 0.5h | completed |
| T17 | `tests/integration/obs_session_bridge_test.go` | L4-OBS-METRICS | L5-OBS-18 | 1h | completed |
| T18 | 权限 metrics 接线（decisions/timeouts counter） | L4-OBS-METRICS | — | 2h | completed |

**M4 小计**: 5.5h

---

## Milestone M5: 文档与验收

| ID | 任务 | L4 | L5 | 估时 | 状态 |
|----|------|-----|-----|------|------|
| T19 | 更新 `openspec/l5-registry.md`：L5-OBS-13~18 → IMPLEMENTED | — | — | 0.5h | completed |
| T20 | 合并 delta 到 canonical `openspec/specs/observability/spec.md` v1.3.0（S7） | — | — | 1h | completed |
| T21 | 编写 `acceptance-report.md`（S5） | — | L5-OBS-13~18 | 1h | completed |
| T22 | 运行 `./scripts/test-unit.sh` + `./scripts/test-integration.sh` | — | — | 0.5h | completed |

**M5 小计**: 3h

---

## 汇总

| Milestone | 任务数 | 估时 | PR 建议 |
|-----------|--------|------|---------|
| M1 | T1–T6 | 9h | PR-1: Registry + Coverage 基础 |
| M2 | T7–T11 | 7h | PR-2: Span 补全 |
| M3 | T12–T14 | 5h | PR-2 或 PR-3 |
| M4 | T15–T18 | 5.5h | PR-3: Metrics 统一 |
| M5 | T19–T22 | 3h | S5/S7 |
| **合计** | **22** | **~30h** | 3 PR |

---

## PR 切分建议

1. **feat(obs): operation registry and coverage counter [DM-20260607-007]** — M1
2. **feat(obs): extend span instrumentation for v1.3 modules [DM-20260607-007]** — M2 + M3
3. **feat(obs): unify session metrics via SessionBridge [DM-20260607-007]** — M4

---

## 依赖

- V1.2 canonical spec 已合并（`observability/spec.md` v1.2.0）
- Context Engine V3 longterm/plan 已落地（DM-20260607-006）
- Feishu adapter 可集成测试
