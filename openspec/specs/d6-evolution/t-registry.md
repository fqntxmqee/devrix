# D6 Evolution Domain — T 层测试点注册表

**Status:** Active
**Version:** 2.2.1
**Last Updated:** 2026-06-14
**Parent:** `openspec/specs/architecture/layering.md`
**Spec Reference:** `openspec/specs/d6-evolution/spec.md`
**Change:** devrix-d3-sa-refine-v1.1（DM-20260614-017 / D6 探针 #1 / #2 / #4 落地；T20/T21/T22 新增；S5 验收后 v1.1 PLANNED→IMPLEMENTED）

---

## D6-S1: Version Module

| T ID | 描述 | 优先级 | S 映射 | Test 位置 | Status |
|-------|------|--------|--------|-----------|--------|
| D6-S1-A01-T01 | 版本检测与记录 | P2 | Version | `internal/layers/evolution/version/version_test.go` | PLANNED |

## D6-S2: Config Module

| T ID | 描述 | 优先级 | S 映射 | Test 位置 | Status |
|-------|------|--------|--------|-----------|--------|
| D6-S2-A01-T01 | 配置热更新 | P2 | Config | `internal/layers/evolution/config/hotreload_test.go` | PLANNED |

## D6-S3-A01: RunEval (Engine + Probes)

| T ID | 描述 | 优先级 | S 映射 | Test 位置 | Status |
|-------|------|--------|--------|-----------|--------|
| D6-S3-A01-T01 | EvalRun 编排 | P0 | Eval | `internal/layers/evolution/eval/engine_test.go` | IMPLEMENTED |
| D6-S3-A01-T03 | Compression Recall Probe F1 | P0 | Eval | `internal/layers/evolution/eval/compression_recall_probe_test.go` | IMPLEMENTED |
| D6-S3-A01-T04 | Delta 报告对比 | P0 | Eval | `internal/layers/evolution/eval/delta_test.go` | IMPLEMENTED |
| D6-S3-A01-T06 | Tool 选择准确率探针 | P1 | Eval | `internal/layers/evolution/eval/tool_accuracy_probe_test.go` | IMPLEMENTED |
| D6-S3-A01-T07 | eval.enabled=false 零行为 | P0 | Eval | `internal/layers/evolution/eval/engine_test.go` | IMPLEMENTED |
| D6-S3-A01-T09 | Provider 质量对比探针 | P1 | Eval | `internal/layers/evolution/eval/provider_quality_probe_test.go` | IMPLEMENTED |
| D6-S3-A01-T10 | Agent Fork/Join 质量探针 | P2 | Eval | `internal/layers/evolution/eval/agent_forkjoin_probe_test.go` | IMPLEMENTED |
| D6-S3-A01-T11 | devrix eval run 子命令 | P1 | Eval | `internal/cli/eval/run_test.go` | IMPLEMENTED |
| D6-S3-A01-T12 | 调优建议生成 | P2 | Eval | `internal/layers/evolution/eval/tune_test.go` | IMPLEMENTED |
| D6-S3-A01-T13 | eval run 真实 Judge 接入 | P1 | Eval | `internal/cli/eval/judge.go` | IMPLEMENTED |
| D6-S3-A01-T14 | Eval CI delta gate | P2 | Eval | `internal/layers/evolution/eval/gate_test.go` | IMPLEMENTED |
| D6-S3-A01-T15 | run-eval.sh CI 脚本 | P2 | Eval | `scripts/eval/run-eval.sh` | IMPLEMENTED |
| D6-S3-A01-T16 | Path Regression Probe (代码路径快照) | P1 | Eval | `internal/layers/evolution/eval/path_regression_probe_test.go` | IMPLEMENTED |
| D6-S3-A01-T17 | Layer Violation Probe (分层违规扫描) | P1 | Eval | `internal/layers/evolution/eval/layer_violation_probe_test.go` | IMPLEMENTED |
| D6-S3-A01-T18 | Session Isolation Probe (COW 隔离) | P1 | Eval | `internal/layers/evolution/eval/session_isolation_probe_test.go` | IMPLEMENTED |
| D6-S3-A01-T19 | Probe 辅助函数 (wordJaccard 等) | P2 | Eval | `internal/layers/evolution/eval/probe_helpers_test.go` | IMPLEMENTED |
| D6-S3-A01-T20 | Tier Resolution Probe (D3 Tier 解析正确性 ≥ 99%) | P1 | Eval | `internal/layers/evolution/eval/tier_resolution_probe_test.go` | **IMPLEMENTED**（v1.1 S5 验收通过，hit≥99% / <99% yellow / error red 4 例） |
| D6-S3-A01-T21 | Breaker Anomaly Transition Probe (状态切换异常告警) | P1 | Eval | `internal/layers/evolution/eval/breaker_anomaly_transition_probe_test.go` | **IMPLEMENTED**（v1.1 S5 验收通过，frequent-flip / rapid-alternate / half_open→open streak 5 例） |
| D6-S3-A01-T22 | Safety Filter Latency Probe (P99 < 1ms) | P0 | Eval | `internal/layers/evolution/eval/safety_latency_probe_test.go` | **IMPLEMENTED**（v1.1 S5 验收通过，<1ms pass / [1,2) yellow / ≥2ms red / 5 例） |

## D6-S3-A02: JudgeResult

| T ID | 描述 | 优先级 | S 映射 | Test 位置 | Status |
|-------|------|--------|--------|-----------|--------|
| D6-S3-A02-T02 | LLM-as-Judge 校准与分歧 (Cohen's kappa) | P0 | Eval | `internal/layers/evolution/eval/judge_test.go` | IMPLEMENTED |

## D6-S3-A05: ManageDataset

| T ID | 描述 | 优先级 | S 映射 | Test 位置 | Status |
|-------|------|--------|--------|-----------|--------|
| D6-S3-A05-T01 | Dataset 加载、抽样与校验 | P1 | Eval | `internal/layers/evolution/eval/dataset_test.go` | IMPLEMENTED |

## D6-S4-A01: ValidateDecision

| T ID | 描述 | 优先级 | S 映射 | Test 位置 | Status |
|-------|------|--------|--------|-----------|--------|
| D6-S4-A01-T01 | Runtime Orchestration Validator (preFilter + Judge + Intervention) | P1 | Orchestration | `internal/layers/evolution/orchestration/validator_test.go` | IMPLEMENTED |

---

## Statistics

| Total | IMPLEMENTED | PLANNED |
|-------|-------------|---------|
| 24 | 19 | 5 |

## P0 Count

| P0 |
|----|
| 6 |

---

## Revision History

| 版本 | 日期 | 变更 |
|------|------|------|
| 2.0.0 | 2026-06-14 | 初版：19 IMPLEMENTED + 2 PLANNED（T01 S1/S2 + 余 17） |
| 2.1.0 | 2026-06-14 | Path Regression / Layer Violation / Session Isolation 3 探针（T16/T17/T18，IMPLEMENTED） |
| 2.2.0 | 2026-06-14 | v1.1 落地：D6 探针 #1 / #2 / #4 → T20/T21/T22（PLANNED v1.1 实施）；T19 = Probe 辅助函数 保持；Total 21 → 24（P0 5 → 6，PLANNED 2 → 5）；probe #3 Token 预算触发率 推迟至 v1.2 |
| **2.2.1** | 2026-06-14 | **v1.1 S5 验收后**：T20/T21/T22 PLANNED→IMPLEMENTED；Total IMPLEMENTED 19 → 22，PLANNED 5 → 2（仅 T01 S1 + T01 S2） |
