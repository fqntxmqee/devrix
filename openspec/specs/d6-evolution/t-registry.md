# D6 Evolution Domain — T 层测试点注册表

**Status:** Active
**Version:** 3.2.0
**Last Updated:** 2026-06-21
**Parent:** `openspec/specs/architecture/layering.md`
**Spec Reference:** `openspec/specs/d6-evolution/spec.md`
**Change:** devrix-d6-sa-refine（DM-20260615-002 / v1.0 Canonical 重排；增 canonical_s 列 + Legacy 双轨；S4 Orchestration → S12 GuardRuntime）+ devrix-diagnostic-tools-parity (DM-20260616-003) — Verifier / devrix-diagnostic-tools-wiring (DM-20260617-002) — G4 verify_plan_execution LLM tool / devrix-tools-terminal-architecture (DM-20260618-007) — Verify W11-W12 (D6-S11-A02-T06/T07/T08) + **devrix-d6-evolution-review-fixes (DM-20260621-011) — bridge 清债 + Orchestration*→Guard* rename + panic→log.Fatal + silent swallow 修复（PR-A #156 + PR-B #157, 新增 6 P0 T 点 D6-S11-A02-T09 + D6-S12-A01-T01/T02 + D6-S12-A02-T04 + D6-S12-A03-T03/T04/T05）**

---

## D6-S11: RunEvaluation（评测执行）

### D6-S11-A01: RunEval（Engine + Probes）

| T ID | 描述 | canonical_s | Priority | Test 位置 | Status | Legacy T ID |
|------|------|-------------|----------|-----------|--------|-------------|
| D6-S11-A01-T01 | EvalRun 编排 | S11 | P0 | `internal/layers/evolution/evaluate/engine_test.go` | IMPLEMENTED | D6-S3-A01-T01 |
| D6-S11-A01-T03 | Compression Recall Probe F1 | S11 | P0 | `internal/layers/evolution/evaluate/compression_recall_probe_test.go` | IMPLEMENTED | D6-S3-A01-T03 |
| D6-S11-A01-T04 | Delta 报告对比 | S11 | P0 | `internal/layers/evolution/evaluate/delta_test.go` | IMPLEMENTED | D6-S3-A01-T04 |
| D6-S11-A01-T06 | Tool 选择准确率探针 | S11 | P1 | `internal/layers/evolution/evaluate/tool_accuracy_probe_test.go` | IMPLEMENTED | D6-S3-A01-T06 |
| D6-S11-A01-T07 | eval.enabled=false 零行为 | S11 | P0 | `internal/layers/evolution/evaluate/engine_test.go` | IMPLEMENTED | D6-S3-A01-T07 |
| D6-S11-A01-T09 | Provider 质量对比探针 | S11 | P1 | `internal/layers/evolution/evaluate/provider_quality_probe_test.go` | IMPLEMENTED | D6-S3-A01-T09 |
| D6-S11-A01-T10 | Agent Fork/Join 质量探针 | S11 | P2 | `internal/layers/evolution/evaluate/agent_forkjoin_probe_test.go` | IMPLEMENTED | D6-S3-A01-T10 |
| D6-S11-A01-T11 | devrix eval run 子命令 | S11 | P1 | `internal/cli/eval/run_test.go` | IMPLEMENTED | D6-S3-A01-T11 |
| D6-S11-A01-T13 | eval run 真实 Judge 接入 | S11 | P1 | `internal/cli/eval/judge.go` | IMPLEMENTED | D6-S3-A01-T13 |
| D6-S11-A01-T14 | Eval CI delta gate | S11 | P2 | `internal/layers/evolution/evaluate/gate_test.go` | IMPLEMENTED | D6-S3-A01-T14 |
| D6-S11-A01-T15 | run-eval.sh CI 脚本 | S11 | P2 | `scripts/eval/run-eval.sh` | IMPLEMENTED | D6-S3-A01-T15 |
| D6-S11-A01-T16 | Path Regression Probe（代码路径快照） | S11 | P1 | `internal/layers/evolution/evaluate/path_regression_probe_test.go` | IMPLEMENTED | D6-S3-A01-T16 |
| D6-S11-A01-T17 | Layer Violation Probe（分层违规扫描） | S11 | P1 | `internal/layers/evolution/evaluate/layer_violation_probe_test.go` | IMPLEMENTED | D6-S3-A01-T17 |
| D6-S11-A01-T18 | Session Isolation Probe（COW 隔离） | S11 | P1 | `internal/layers/evolution/evaluate/session_isolation_probe_test.go` | IMPLEMENTED | D6-S3-A01-T18 |
| D6-S11-A01-T19 | Probe 辅助函数（wordJaccard 等） | S11 | P2 | `internal/layers/evolution/evaluate/probe_helpers_test.go` | IMPLEMENTED | D6-S3-A01-T19 |
| D6-S11-A01-T20 | Tier Resolution Probe（D3 Tier 解析正确性 ≥ 99%） | S11 | P1 | `internal/layers/evolution/evaluate/tier_resolution_probe_test.go` | IMPLEMENTED | D6-S3-A01-T20 |
| D6-S11-A01-T21 | Breaker Anomaly Transition Probe（状态切换异常告警） | S11 | P1 | `internal/layers/evolution/evaluate/breaker_anomaly_transition_probe_test.go` | IMPLEMENTED | D6-S3-A01-T21 |
| D6-S11-A01-T22 | Safety Filter Latency Probe（P99 < 1ms） | S11 | P0 | `internal/layers/evolution/evaluate/safety_latency_probe_test.go` | IMPLEMENTED | D6-S3-A01-T22 |

### D6-S11-A02: JudgeResult

| T ID | 描述 | canonical_s | Priority | Test 位置 | Status | Legacy T ID |
|------|------|-------------|----------|-----------|--------|-------------|
| D6-S11-A02-T02 | LLM-as-Judge 校准与分歧（Cohen's kappa） | S11 | P0 | `internal/layers/evolution/evaluate/judge_test.go` | IMPLEMENTED | D6-S3-A02-T02 |
| D6-S11-A02-T03 | Verifier 全部 plan item 通过 | S11 | P0 | `internal/layers/evolution/verify/plan_test.go` | IMPLEMENTED | — |
| D6-S11-A02-T04 | Verifier 缺失 evidence → unverified | S11 | P0 | `internal/layers/evolution/verify/plan_test.go` | IMPLEMENTED | — |
| D6-S11-A02-T05 | Verifier `_test.go` 缺 `func TestXxx(` → fail | S11 | P0 | `internal/layers/evolution/verify/plan_test.go` | IMPLEMENTED | — |
| **D6-S11-A02-T06** | **tasks.md parser 兼容 \| W{N}.{M} \| 表格 + done/pending (Verify W11)** | **S11** | **P0** | **`internal/layers/evolution/verify/plan_test.go` (TestW11_12_VerifyStack_T_CrossRef) + tests/integration/tools_terminal_test.go (TestVerify_AllPass)** | **IMPLEMENTED** | **—** |
| **D6-S11-A02-T07** | **evidence kind → checker 路由 5 kind (file/test/cmd/api/...) (Verify W11)** | **S11** | **P0** | **`internal/layers/evolution/verify/plan_test.go` + `surface/verify_surface.go`** | **IMPLEMENTED** | **—** |
| **D6-S11-A02-T08** | **aggregator (verified/unverified/skipped/summary) + report JSON (Verify W12)** | **S11** | **P0** | **`internal/layers/evolution/verify/plan_test.go` (TestW11_12_VerifyStack_T_CrossRef)** | **IMPLEMENTED** | **—** |
| **D6-S11-A02-T09** | **verify invariant ParseStruct 失败 fail-safe (panic → log.Fatalf + `_invariant.go` → `invariant.go` 重命名激活 dead code)** | **S11** | **P0** | **`internal/layers/evolution/verify/invariant_test.go` (TestParseVerifyInvariants_GoodStruct_Succeeds / TestParseVerifyInvariants_BadStruct_ReturnsError / TestVerifyInvariants_InitSucceeds / TestCheckVerifyInvariants_NoViolations / TestCheckVerifyInvariants_ViolationDetected)** | **IMPLEMENTED** | **—** |

### D6-S11-A04: GenerateTune

| T ID | 描述 | canonical_s | Priority | Test 位置 | Status | Legacy T ID |
|------|------|-------------|----------|-----------|--------|-------------|
| D6-S11-A04-T01 | 调优建议生成 | S11 | P2 | `internal/layers/evolution/evaluate/tune_test.go` | IMPLEMENTED | D6-S3-A01-T12 |

### D6-S11-A05: ManageDataset

| T ID | 描述 | canonical_s | Priority | Test 位置 | Status | Legacy T ID |
|------|------|-------------|----------|-----------|--------|-------------|
| D6-S11-A05-T01 | Dataset 加载、抽样与校验 | S11 | P1 | `internal/layers/evolution/evaluate/dataset_test.go` | IMPLEMENTED | D6-S3-A05-T01 |

## D6-S12: GuardRuntime（运行时守护）

| T ID | 描述 | canonical_s | Priority | Test 位置 | Status | Legacy T ID |
|------|------|-------------|----------|-----------|--------|-------------|
| D6-S12-A01-T01 | Runtime Guard Validator（preFilter + Judge + Intervention，v2.4.0 由 `RuntimeOrchestrationValidator` 重命名） | S12 | P1 | `internal/layers/evolution/guard/validator_test.go` | IMPLEMENTED | D6-S4-A01-T01 |
| **D6-S12-A01-T01** | **InterventionExecutor Wait 失败 metric + slog.Warn + errors.Join（silent swallow → 三联固化）** | **S12** | **P0** | **`internal/layers/evolution/guard/intervention_test.go` (TestInterventionExecutor_WaitFailure_RecordsMetric / TestInterventionExecutor_TerminateFailure_ReturnsPartialErr / TestInterventionExecutor_AllSuccess_ReturnsNil / TestInterventionExecutor_NilMetrics_NilSafe / TestInterventionExecutor_BothWaitAndTaskFailFail_AggregateBoth / TestInterventionExecutor_WithMetrics_Chainable)** | **IMPLEMENTED** | **—** |
| **D6-S12-A01-T02** | **InterventionExecutor tasks.Fail 失败 metric + slog.Warn（silent swallow → 三联固化）** | **S12** | **P0** | **`internal/layers/evolution/guard/intervention_test.go` (TestInterventionExecutor_TaskFailFailure_RecordsMetric)** | **IMPLEMENTED** | **—** |
| **D6-S12-A02-T04** | **bridge.go 完全删除（`eval/bridge.go` + `orchestration/bridge.go` git ls-files 0 命中）** | **S12** | **P0** | **`tests/integration/d6/d6_bridge_absence_test.go` (TestD6Bridge_FilesRemoved / TestD6Bridge_OnlyBridgeFilesAbsent)** | **IMPLEMENTED** | **—** |
| **D6-S12-A03-T03** | **guard/ 内 0 处 `Orchestration*`（除 type alias 定义点）+ 全仓 rename CI guard PASS** | **S12** | **P0** | **`scripts/check-orch-rename.sh` + `tests/integration/d6/d6_bridge_absence_test.go` (TestD6Rename_NoGuardUsageBeyondAliases)** | **IMPLEMENTED** | **—** |
| **D6-S12-A03-T04** | **6 个 OTel 指标 `orch_*` → `guard_*` 与 spec v2.4.0 一致** | **S12** | **P0** | **`tests/integration/d6/d6_bridge_absence_test.go` (TestD6Rename_MetricNamesGuarded)** | **IMPLEMENTED** | **—** |
| **D6-S12-A03-T05** | **type alias 向后兼容 (RuntimeOrchestrationValidator = RuntimeGuardValidator / OrchestrationObserver = GuardObserver / OrchestrationConfig = GuardConfig / orchMetrics = guardMetrics)** | **S12** | **P0** | **`tests/integration/d6/d6_rename_test.go` (TestD6Rename_AliasesAreTypeAliases / TestD6Rename_OldNewConstructorsEquivalent / TestD6Rename_OldNewObserverConstructorsEquivalent / TestD6Rename_SharedConfigCompatibleWithGuardConfig)** | **IMPLEMENTED** | **—** |

## D6-S13: TrackVersion（PLANNED）

| T ID | 描述 | canonical_s | Priority | Test 位置 | Status | Legacy T ID |
|------|------|-------------|----------|-----------|--------|-------------|
| D6-S13-A01-T01 | 版本检测与记录 | S13 | P2 | `internal/layers/evolution/version/version_test.go` | PLANNED | D6-S1-A01-T01 |

## D6-S14: ReloadConfig（PLANNED）

| T ID | 描述 | canonical_s | Priority | Test 位置 | Status | Legacy T ID |
|------|------|-------------|----------|-----------|--------|-------------|
| D6-S14-A01-T01 | 配置热更新 | S14 | P2 | `internal/layers/evolution/config/hotreload_test.go` | PLANNED | D6-S2-A01-T01 |

---

## Legacy Module Index（旧 T 编号→新 Canonical）

| Legacy S | T 数 | Canonical S | Scenario |
|----------|------|-------------|----------|
| D6-S1 Version | 1（PLANNED） | S13 | TrackVersion |
| D6-S2 Config | 1（PLANNED） | S14 | ReloadConfig |
| D6-S3 Eval | 21（全 IMPLEMENTED） | S11 | RunEvaluation |
| D6-S4 Orchestration | 1（IMPL） | S12 | GuardRuntime |

---

## Statistics

| Total | IMPLEMENTED | PLANNED |
|-------|-------------|---------|
| 30 | 28 | 2 |

## P0 Count

| P0 |
|----|
| 12 |

---

## Revision History

| 版本 | 日期 | 变更 |
|------|------|------|
| 2.2.1 | 2026-06-14 | v1.1 S5 验收后：T20/T21/T22 PLANNED→IMPLEMENTED |
| 3.0.0 | 2026-06-15 | **SA Refine v1.0**：Canonical S11–S14 重排；增 canonical_s + Legacy T ID 列；S4 Orchestration → S12 GuardRuntime |
| 3.1.0 | 2026-06-18 | t-registry 表头 Last Updated 更新；T06/T07/T08 (Verify W11/W12) IMPLEMENTED |
| **3.2.0** | **2026-06-21** | **devrix-d6-evolution-review-fixes (DM-20260621-011)**：新增 6 P0 T 点 — T09 (verify invariant fail-safe + `_invariant.go` 重命名) + A01-T01 (intervention Wait 三联固化) + A01-T02 (tasks.Fail 三联固化) + A02-T04 (bridge 清债) + A03-T03 (rename CI guard) + A03-T04 (orch_* → guard_* 指标) + A03-T05 (type alias 向后兼容)；P0 6→12，IMPLEMENTED 22→28 |
