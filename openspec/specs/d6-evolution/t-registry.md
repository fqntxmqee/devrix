# D6 Evolution Domain — T 层测试点注册表

**Status:** Active
**Version:** 1.0.0
**Last Updated:** 2026-06-13
**Parent:** `openspec/specs/architecture/layering.md`
**Spec Reference:** `openspec/specs/d6-evolution/spec.md`

---

## D6-S1: Version Module

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| D6-S1-A01-T01 | 版本检测与记录（PlannedVersion: v2.1.0） | Version | `internal/layers/evolution/version/version_test.go` | PLANNED | P2 |

## D6-S2: Config Module

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| D6-S2-A01-T01 | 配置热更新（PlannedVersion: v2.2.0） | Config | `internal/layers/evolution/config/hotreload_test.go` | PLANNED | P2 |

## D6-S3: Eval Module (Pilot)

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| D6-S3-A01-T01 | EvalRun 编排 | Eval | `internal/layers/evolution/eval/engine_test.go` | IMPLEMENTED | P0 |
| D6-S3-A02-T02 | LLM-as-Judge 校准与分歧 | Eval | `internal/layers/evolution/eval/judge_test.go` | IMPLEMENTED | P0 |
| D6-S3-A01-T03 | Compression Recall Probe F1 | Eval | `internal/layers/evolution/eval/compression_recall_probe_test.go` | IMPLEMENTED | P0 |
| D6-S3-A01-T04 | Delta 报告对比 | Eval | `internal/layers/evolution/eval/delta_test.go` | IMPLEMENTED | P0 |
| D6-S3-A01-T07 | eval.enabled=false 零行为 | Eval | `internal/layers/evolution/eval/engine_test.go` | IMPLEMENTED | P0 |
| D6-S3-A01-T06 | Tool 选择准确率探针 | Eval | `internal/layers/evolution/eval/tool_accuracy_probe_test.go` | IMPLEMENTED | P1 |
| D6-S3-A01-T11 | devrix eval run 子命令 | Eval | `internal/cli/eval/run_test.go` | IMPLEMENTED | P1 |
| D6-S3-A01-T12 | 调优建议生成 | Eval | `internal/layers/evolution/eval/tune_test.go` | IMPLEMENTED | P2 |
| D6-S3-A01-T13 | eval run 真实 Judge 接入 | Eval | `internal/cli/eval/judge.go` | IMPLEMENTED | P1 |
| D6-S3-A01-T09 | Provider 质量对比探针 | Eval | `internal/layers/evolution/eval/provider_quality_probe_test.go` | IMPLEMENTED | P1 |
| D6-S3-A01-T10 | Agent Fork/Join 质量探针 | Eval | `internal/layers/evolution/eval/agent_forkjoin_probe_test.go` | IMPLEMENTED | P2 |
| D6-S3-A01-T14 | Eval CI delta gate | Eval | `internal/layers/evolution/eval/gate_test.go` | IMPLEMENTED | P2 |
| D6-S3-A01-T15 | run-eval.sh CI 脚本 | Eval | `scripts/eval/run-eval.sh` | IMPLEMENTED | P2 |

---

## Statistics

| Total | IMPLEMENTED | PLANNED |
|-------|-------------|---------|
| 15 | 13 | 2 |
