# D6 Evolution Domain — T 层测试点注册表

**Status:** Active
**Version:** 2.0.0
**Last Updated:** 2026-06-14
**Parent:** `openspec/specs/architecture/layering.md`
**Spec Reference:** `openspec/specs/d6-evolution/spec.md`

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
| 21 | 19 | 2 |

## P0 Count

| P0 |
|----|
| 5 |
