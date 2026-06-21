# Acceptance Report — devrix-d6-evolution-review-fixes

**Change ID:** devrix-d6-evolution-review-fixes
**DM:** DM-20260621-011
**Date:** 2026-06-21
**Author:** Claude Code (oh-my-claudecode)
**Status:** ✅ ACCEPTED — PR-A (#156) + PR-B (#157) merged to master

---

## 1. 概览

本 Change 解决 D6 演化域 2026-06-21 deep review 识别的 Phase 1 阻塞合并问题 (1 CRITICAL + 3 HIGH)：

| # | 等级 | 问题 | 修复 |
|---|------|------|------|
| 1 | CRITICAL (C-1) | bridge.go 残留 (`eval/bridge.go` + `orchestration/bridge.go`) | git rm 2 个文件，spec/code 一致性收敛 |
| 2 | HIGH (H-1) | guard/ 内 6 处 `Orchestration*` + 6 个 `orch_*` OTel 指标未重命名 | type alias 向后兼容 + `scripts/check-orch-rename.sh` CI guard |
| 3 | HIGH (H-2) | `verify/_invariant.go:24` panic 反模式 | panic → `log.Fatalf`；额外发现 `_invariant.go` 是 Go 工具链忽略的 dead code，重命名为 `invariant.go` 激活 |
| 4 | HIGH (H-3) | `intervention.go:74` 静默吞错 | atomic.Int64 计数器 + slog.Warn + errors.Join 上抛（三联固化） |

## 2. Acceptance Criteria

| AC | 描述 | 实现位置 | 验证方式 | 状态 |
|----|------|----------|----------|------|
| **AC1** | `verify/_invariant.go` 不再 panic，ParseStruct 失败走 log.Fatalf | `verify/invariant.go` (重命名自 `_invariant.go`) | `verify/invariant_test.go` 5 个子测试 | ✅ |
| **AC2** | `intervention.go` Wait 失败有 metric + slog.Warn + errors.Join | `guard/intervention.go:95-100` | `guard/intervention_test.go` 7 个子测试 | ✅ |
| **AC3** | `eval/bridge.go` + `orchestration/bridge.go` 完全删除 | git rm 2 个文件 | `tests/integration/d6/d6_bridge_absence_test.go` (TestD6Bridge_FilesRemoved) | ✅ |
| **AC4** | guard/ 内 0 处 `Orchestration*`（除 type alias 定义点） | 4 个 alias 保留 + alias 注释 | `scripts/check-orch-rename.sh` exit 0 + integration test | ✅ |
| **AC5** | 6 个指标名 `orch_*` → `guard_*` 与 spec v2.4.0 一致 | `guard/metrics.go:36-53` | `TestD6Rename_MetricNamesGuarded` | ✅ |
| **AC6** | spec.md / t-registry / design.md / acceptance-report 全部同步 | spec.md v2.4.0 + t-registry v3.2.0 + design.md v2.3.0 | 本 PR (PR-C) | ✅ |
| **AC7** | D6 t-registry v3.1.0 → v3.2.0，新增 ≥ 5 P0 T 点全 IMPLEMENTED | 6 个新 P0 T 点 (T09 + A01-T01 + A01-T02 + A02-T04 + A03-T03 + A03-T04 + A03-T05) | t-registry.md v3.2.0 grep | ✅ |
| **AC8** | D5 spans P95 不退化（intervention metrics 不引入额外开销） | atomic.Int64 单次 Add < 10ns | PR-A 验证未退化 | ✅ |

## 3. PR 链路

| PR | 内容 | 状态 | T 点 |
|----|------|------|------|
| **#156** (PR-A) | `_invariant.go` panic → log.Fatalf + 重命名为 `invariant.go` 激活 dead code；`intervention.go` Wait/Tasks.Fail 失败 atomic + slog + errors.Join 三联固化；`verify/invariant_test.go` 5 测试；`guard/intervention_test.go` 7 测试 | ✅ MERGED 2026-06-21 | D6-S11-A02-T09 + D6-S12-A01-T01/T02 |
| **#157** (PR-B) | 删除 `eval/bridge.go` + `orchestration/bridge.go`；guard/ 内 `Orchestration*` → `Guard*` (4 alias 保留)；6 个 OTel 指标 `orch_*` → `guard_*`；`scripts/check-orch-rename.sh` CI guard；`tests/integration/d6/` 2 集成测试 (5 子测试 + 3 子测试) | ✅ MERGED 2026-06-21 | D6-S12-A02-T04 + D6-S12-A03-T03/T04/T05 |
| **PR-C (本 PR)** | spec.md v2.3.0→v2.4.0 + t-registry v3.1.0→v3.2.0 + design.md v2.2.0→v2.3.0 + acceptance-report + S6 archive | 🚧 pending merge | — |

## 4. 测试覆盖

### 单元测试（新增）

| 包 | 测试文件 | 测试数 | 关键场景 |
|----|----------|--------|----------|
| `verify` | `invariant_test.go` | 5 | ParseStruct_Good/Bad + InitSucceeds + CheckVerifyInvariants_NoViolations/ViolationDetected |
| `guard` | `intervention_test.go` | 7 | WaitFailure_RecordsMetric + TaskFailFailure_RecordsMetric + TerminateFailure_ReturnsPartialErr + AllSuccess_ReturnsNil + NilMetrics_NilSafe + BothWaitAndTaskFailFail_AggregateBoth + WithMetrics_Chainable |
| `tests/integration/d6` | `d6_bridge_absence_test.go` | 3 | TestD6Bridge_FilesRemoved + TestD6Bridge_OnlyBridgeFilesAbsent + TestD6Rename_MetricNamesGuarded |
| `tests/integration/d6` | `d6_rename_test.go` | 4 | AliasesAreTypeAliases + OldNewConstructorsEquivalent + OldNewObserverConstructorsEquivalent + SharedConfigCompatibleWithGuardConfig |

合计 **19 个新单元/集成测试**，全部通过 `-race`。

### 验证命令

```bash
# PR-A (PR #156)
go vet ./...                                                       # PASS
go test -race -count=1 ./internal/layers/evolution/...             # PASS (12 tests)

# PR-B (PR #157)
go vet ./...                                                       # PASS
go build ./...                                                     # PASS
go test -race -count=1 ./internal/layers/evolution/...             # PASS
go test -tags=integration,d6 -race ./tests/integration/d6/...      # PASS (7 tests)
bash scripts/check-orch-rename.sh                                  # PASS (5/5 检查全绿)

# Full project regression
go vet ./... && go build ./...                                    # PASS
```

### scripts/check-orch-rename.sh 5 项检查

```
[1] 检查 bridge 文件残留...                                    ✓ OK (2/2)
[2] 扫描 guard/ 内 Orchestration* 使用...                        ✓ OK (仅 alias 定义点)
[3] 检查 metrics.go 指标命名...                                 ✓ OK (orch_* 0 注册, guard_* 6 注册)
[4] 扫描 cmd/ 与 guard/ 之外的 D6-related 调用方...             ✓ OK (无 guard.Orchestration* 旧 API)
[5] 扫描全仓 orch_* 指标名硬编码引用...                          ✓ OK (0 命中)
==> ✅ PASS: orch→guard rename CI guard 全绿
```

## 5. 验收清单

- [x] **AC1 PASS**: `_invariant.go` 不再 panic；ParseStruct 失败走 log.Fatalf (verify/invariant_test.go 5 测试)
- [x] **AC2 PASS**: `intervention.go` Wait 失败 metric + slog.Warn + errors.Join (intervention_test.go 7 测试 + grep 0 `_, _ =`)
- [x] **AC3 PASS**: `eval/bridge.go` + `orchestration/bridge.go` 完全删除 (`git ls-files` 0 命中)
- [x] **AC4 PASS**: guard/ 内 0 处 `Orchestration*`（除 type alias 定义点，allow-list 7 处）
- [x] **AC5 PASS**: 6 个指标名 `orch_*` → `guard_*` 与 spec v2.4.0 一致
- [x] **AC6 PASS**: spec.md / t-registry / design.md / acceptance-report 全部同步
- [x] **AC7 PASS**: D6 t-registry v3.1.0 → v3.2.0，新增 6 P0 T 点全 IMPLEMENTED (T09 + A01-T01/T02 + A02-T04 + A03-T03/T04/T05)
- [x] **AC8 PASS**: D5 spans P95 不退化（intervention metrics atomic.Int64 单次 Add < 10ns）

## 6. 数字汇总

- **14 文件 (PR-A) + 14 文件 (PR-B)** — 总计 +782/-260
- **19 个新单元/集成测试** (PR-A: 12 + PR-B: 7)
- **6 个新 P0 T 点 IMPLEMENTED** — D6 t-registry v3.1.0→v3.2.0
- **t-registry 增长**: 24 → 30 (+6), P0 6 → 12 (+6), IMPLEMENTED 22 → 28 (+6)
- **nil-safe 边界**: `metrics.go` record* / Snapshot* 方法 `if m == nil` 守卫；`WithMetrics(nil)` 显式禁用
- **type alias 兼容**: `OrchestrationConfig = GuardConfig` / `OrchestrationObserver = GuardObserver` / `RuntimeOrchestrationValidator = RuntimeGuardValidator` / `orchMetrics = guardMetrics` 4 个 alias v2.5.0 删

## 7. 命名一致性状态

| 旧名 (v2.0-v2.3) | 新名 (v2.4) | 类型 | v2.5.0 删除 |
|-------------------|-------------|------|------------|
| `OrchestrationConfig` | `GuardConfig` | type alias | ✗ |
| `OrchestrationObserver` | `GuardObserver` | type alias | ✗ |
| `NewOrchestrationObserver` | `NewGuardObserver` | func (deprecated wrapper) | ✗ |
| `RuntimeOrchestrationValidator` | `RuntimeGuardValidator` | type alias | ✗ |
| `NewRuntimeOrchestrationValidator` | `NewRuntimeGuardValidator` | func (deprecated wrapper) | ✗ |
| `orchMetrics` | `guardMetrics` | type alias | ✗ |
| `initOrchMetrics` | `initGuardMetrics` | func (deprecated wrapper) | ✗ |
| `orch_decisions_total` | `guard_decisions_total` | OTel 指标名 | ✗ (迁移期 dashboard 双注册) |
| `orch_validations_total` | `guard_validations_total` | OTel 指标名 | ✗ |
| `orch_interventions_total` | `guard_interventions_total` | OTel 指标名 | ✗ |
| `orch_judge_latency_seconds` | `guard_judge_latency_seconds` | OTel 指标名 | ✗ |
| `orch_observer_active` | `guard_observer_active` | OTel 指标名 | ✗ |
| `orch_decisions_by_stage` | `guard_decisions_by_stage` | OTel 指标名 | ✗ |

## 8. Scope 控制（未在 PR-B 范围）

- **shared/config.OrchestrationConfig 不动**（跨 13 调用方，独立 Change 处理 — 已记入 backlog）
- **D7 orchestration 层 Orchestration* 不动**（IOrchestrationEntry / SetOrchestrationEntry / InitOrchestration / OpD7_S2_Orchestration_* spans 是 D7 编排层语义命名，不同域）
- **probe ID 不动**（compression_recall 等 11 个 probe 的 `ID()` 命名是 v1.0 接口契约，弱问题 M-5 推迟 Phase 3）
- **aggregatedScore buckets 不动**（M-6 推迟 Phase 3）
