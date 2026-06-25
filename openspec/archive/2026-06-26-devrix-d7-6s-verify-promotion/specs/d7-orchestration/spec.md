# D7 Orchestration Spec (verify-promotion Delta)

**Change ID:** devrix-d7-6s-verify-promotion
**Status:** S3_Design
**Priority:** P1
**Created:** 2026-06-26
**DM:** DM-20260626-005

---

## Delta 概述

本次 delta 仅做物理包归属迁移 (`exit_reason.go` + `verdict_to_exit_reason.go` + `verdict_to_exit_reason_test.go` 从 `sessionorchestrator/` 迁移到 `executionflow/verify/`), 不修改 `openspec/specs/d7-orchestration/spec.md` v2.2.0 的语义内容, 仅追加 v2.3.0 changelog row。

## Gherkin 验收规格 (不变)

完整的 Gherkin 验收规格见 `openspec/specs/d7-orchestration/spec.md` v2.2.0。本次 delta **0 Gherkin scenario 变化**, 0 spec.md scenario 文件变更。

## 变更摘要

| 项目 | 数值 |
|------|------|
| 物理迁移文件 | 3 .go (218 行) |
| package 改名 | 3 (`sessionorchestrator` → `verify`) |
| 跨包引用更新 | sessionorchestrator/turn_orchestrator.go 11 处 + turn_orchestrator_test.go 2 处 |
| 函数签名变化 | 0 (pure physical migration) |
| 行为变化 | 0 (14 ExitReason 字符串值不变 + 5 测试函数测试矩阵不变) |
| spec 同步 | d7-domain v2.3.0 + design v4.3.0 + t-registry v4.5.0 + 根 v5.5.0 |
| 新 P0 T | 4 (D7-S4-A50-T01..T04) |

## 新 P0 T 注册

### D7-S4-A50-T01: 3 文件 git mv + rename 100%

**GIVEN** `internal/layers/orchestration/sessionorchestrator/` 包内临时留存 `exit_reason.go` + `verdict_to_exit_reason.go` + `verdict_to_exit_reason_test.go` 3 文件 (来自 DM-20260626-004 turn-merge 临时策略)
**WHEN** `git mv` 3 文件到 `internal/layers/orchestration/executionflow/verify/`
**THEN** `git log --follow` 显示 100% rename detection
**AND** `ls sessionorchestrator/ | grep -E "exit_reason|verdict_to"` 输出空 (3 文件已移出)
**AND** `ls executionflow/verify/` 显示新 3 文件 + 原 anomaly.go + anomaly_test.go

### D7-S4-A50-T02: package 改名 + 13 处 ExitReason* 跨包引用替换

**GIVEN** 3 文件已 git mv 到 executionflow/verify/
**WHEN** `sed -i '' '1s|package sessionorchestrator|package verify|'` 应用到 3 文件
**AND** sessionorchestrator/turn_orchestrator.go 加 `"internal/layers/orchestration/executionflow/verify"` import
**AND** sed `\bExitReason\b` + `\bExitReason([A-Z][a-zA-Z]*)\b` 替换为 `verify.ExitReason*` (精确 11 处)
**AND** sessionorchestrator/turn_orchestrator_test.go 替换 `ExitReasonNatural` → `verify.ExitReasonNatural` (精确 2 处)
**THEN** 3 文件 `head -1` 全部输出 `package verify`
**AND** `grep -rn "ExitReason[^a-zA-Z]" sessionorchestrator/*.go | grep -v "verify\."` 输出空 (13 处全替换)
**AND** `grep -rn "sessionorchestrator\." executionflow/verify/{exit_reason,verdict_to_exit_reason,verdict_to_exit_reason_test}.go` 输出空 (无反向依赖)

### D7-S4-A50-T03: cross-package import cycle 0 风险

**GIVEN** promote 后 sessionorchestrator → verify 跨包引用方向 (DAG 边)
**WHEN** `go list -deps ./internal/layers/orchestration/executionflow/verify | grep sessionorchestrator` 检查反向依赖
**THEN** 输出空 (单向 DAG: sessionorchestrator → verify, 无反向, 无 import cycle)
**AND** `go list -deps ./internal/layers/orchestration/sessionorchestrator | grep executionflow/verify` 命中 (确认正向引用)

### D7-S4-A50-T04: 22/22 orchestration packages go test -race + LP-1/2/5 兼容 + baseline stability

**GIVEN** promote + 跨包引用替换已完成
**WHEN** `go build ./...` + `go vet ./...` + `go test -race -count=1 ./internal/layers/orchestration/...` 全跑
**THEN** `go build` 0 错误
**AND** `go vet` 0 警告
**AND** 22/22 orchestration packages 全 PASS, 0 race detector warnings
**AND** LP-1 (Bayesian reputation TestAutoClose_FullLP1Loop) PASS
**AND** LP-2 (5 节点 TestIntegration_5NodePipeline_End2End) PASS
**AND** LP-5 (Cross-session traceability) PASS
**AND** `git diff sessionorchestrator/autoclose.go hardening/ escape/circuit_breaker.go` 输出空 (baseline stability)

## 兼容性保证

### 0 函数签名变化

| 函数/类型 | 签名 | 变化 |
|----------|------|------|
| `type ExitReason string` | `string` alias | 不变 |
| `const ExitReasonNatural ExitReason = "natural"` | string 值 "natural" | 不变 |
| `const ExitReasonMaxTurns ExitReason = "max_turns"` | string 值 "max_turns" | 不变 |
| `const ExitReasonAbortedUser ExitReason = "aborted_user"` | string 值 "aborted_user" | 不变 |
| `const ExitReasonAbortedLLM ExitReason = "aborted_llm"` | string 值 "aborted_llm" | 不变 |
| `const ExitReasonAbortedTool ExitReason = "aborted_tool"` | string 值 "aborted_tool" | 不变 |
| `const ExitReasonRepeatedTool ExitReason = "repeated_tool"` | string 值 "repeated_tool" | 不变 |
| `const ExitReasonToolFailure ExitReason = "tool_failure"` | string 值 "tool_failure" | 不变 |
| `const ExitReasonTokenDiminishing ExitReason = "token_diminishing"` | string 值 "token_diminishing" | 不变 |
| `const ExitReasonPartialVerified ExitReason = "partial_verified"` | string 值 "partial_verified" | 不变 |
| `const ExitReasonVerifierAbstain ExitReason = "verifier_abstain"` | string 值 "verifier_abstain" | 不变 |
| `const ExitReasonVerifierFail ExitReason = "verifier_fail"` | string 值 "verifier_fail" | 不变 |
| `const ExitReasonSystemAnomaly ExitReason = "system_anomaly"` | string 值 "system_anomaly" | 不变 |
| `const ExitReasonUnresolved ExitReason = "unresolved"` | string 值 "unresolved" | 不变 |
| `const ExitReasonAbstain ExitReason = "abstain"` | string 值 "abstain" | 不变 |
| `func VerdictToExitReason(v workmodel.Verdict, sessionID string) ExitReason` | `(Verdict, string) → ExitReason` | 不变 |

### 0 行为变化 (5 测试函数测试矩阵不变)

| Test | 验证矩阵 | 期望结果 |
|------|----------|----------|
| TestVerdictToExitReason_4Kinds | Pass/Partial/Indeterminate/Fail → Natural/PartialVerified/VerifierAbstain/VerifierFail | 不变 |
| TestVerdictToExitReason_SystemAnomalyOverrides | 4 Kinds × SystemAnomaly=true → ExitReasonSystemAnomaly | 不变 |
| TestVerdictToExitReason_EmptyVerdictKind_DefaultsToAbstain | zero VerdictKind → Natural (零值 VerdictPass) | 不变 |
| TestVerdictToExitReason_UnknownKind_DefaultsToAbstain | Kind=99 → VerifierAbstain | 不变 |
| TestVerdictToExitReason_NilConfidence_HandledGracefully | Confidence=0 → 映射正常 | 不变 |
| TestVerdictToExitReason_SessionIDAccepted | 4 种 sessionID 字符串 → 接受无 panic | 不变 |