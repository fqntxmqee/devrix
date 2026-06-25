# Proposal: D7 verify-promotion

**Change ID:** devrix-d7-6s-verify-promotion
**Status:** S2_Proposal
**Priority:** P1
**Created:** 2026-06-26
**DM:** DM-20260626-005
**Related:** devrix-d7-six-s-simplification (DM-20260626-001) · devrix-d7-6s-package-merge (DM-20260626-004)

---

## 1. 概述

v6.0.0 域升级 follow-up #4 — 把 `sessionorchestrator/` 包内临时留存的 `exit_reason.go` + `verdict_to_exit_reason.go` + `verdict_to_exit_reason_test.go` 3 文件物理 promote 到 `executionflow/verify/`，让 S4 ExecutionFlow + Verify (Costly Signaler + Certifier) 角色的可验证承诺 (14 ExitReason + VerdictToExitReason 4 态映射) 在 spec 层与代码层完全对齐。

这是 DM-20260626-004 (`devrix-d7-6s-package-merge`) 的"前置欠账"收口：当时合并 turn/ → sessionorchestrator/ 时为避免单 PR scope 膨胀 (25 + 14 importer + 跨包 cycle 已经 60 文件)，3 个 Verify-衍生文件**临时留在了 sessionorchestrator/**；本次独立 PR 收口，0 函数签名变化。

## 2. 动机

| 信号 | 当前状态 | 期望状态 |
|------|----------|----------|
| `ExitReason` 14 枚举值归属 | 6/14 (Verify 衍生) + 8/14 (deterministic) 混在 `sessionorchestrator/` | 6/14 移到 `executionflow/verify/`；8/14 deterministic 仍引用 `verify.ExitReason*` (SessionOrchestrator 在 Turn 终止时消费 Verify 输出) |
| `VerdictToExitReason` 函数归属 | `sessionorchestrator/` (S2 角色代码) | `executionflow/verify/` (S4 角色代码, Certifier 核心能力) |
| Spec 层 A 注册表 | `D7-S10-A33 VerdictToExitReason` 注册到 S4 ExecutionFlow+Verify | 一致: `verify/verdict_to_exit_reason.go::VerdictToExitReason` (函数名保持不变, 仅搬位置) |
| 跨包调用方向 | `executionflow/verify/anomaly.go` (Certifier) 未来调用 `sessionorchestrator.VerdictToExitReason` → 反向依赖 (S4 → S2) | `sessionorchestrator/turn_orchestrator.go` → `verify.ExitReason*` (正向: S2 在 Turn 终止时消费 S4 输出的 ExitReason) |

## 3. 方案

### 3.1 物理迁移（pure physical migration）

```
sessionorchestrator/exit_reason.go               (72 行)  ─┐
sessionorchestrator/verdict_to_exit_reason.go    (49 行)   ├── git mv ──▶  executionflow/verify/
sessionorchestrator/verdict_to_exit_reason_test.go (97 行) ┘
```

**0 函数签名变化**：
- `ExitReason` 类型保持 string alias
- 14 枚举值字符串保持 ("natural" / "max_turns" / ... / "abstain")
- `VerdictToExitReason(v workmodel.Verdict, sessionID string) ExitReason` 签名保持

### 3.2 跨包引用更新（sed 批量替换）

`sessionorchestrator/turn_orchestrator.go` 内 8 处 `ExitReason*` 引用改为 `verify.ExitReason*`：

| # | 行号 | 原 | 改 |
|---|------|----|----|
| 1 | 222 | `exitReason ExitReason` (state 字段类型) | `exitReason verify.ExitReason` |
| 2 | 270 | `st.exitReason = ExitReasonMaxTurns` | `st.exitReason = verify.ExitReasonMaxTurns` |
| 3 | 513 | `exitReason: ExitReasonNatural,` | `exitReason: verify.ExitReasonNatural,` |
| 4 | 743 | `st.exitReason = ExitReasonNatural` | `st.exitReason = verify.ExitReasonNatural` |
| 5 | 753 | `st.exitReason = ExitReasonRepeatedTool` | `st.exitReason = verify.ExitReasonRepeatedTool` |
| 6 | 786 | `st.exitReason = ExitReasonToolFailure` | `st.exitReason = verify.ExitReasonToolFailure` |
| 7 | 803 | `st.exitReason = ExitReasonTokenDiminishing` | `st.exitReason = verify.ExitReasonTokenDiminishing` |
| 8 | 864 | `func resolveFinalText(... exitReason ExitReason, maxTurns int)` | `func resolveFinalText(... exitReason verify.ExitReason, maxTurns int)` |
| 9 | 870 | `if exitReason == ExitReasonMaxTurns && maxTurns > 0` | `if exitReason == verify.ExitReasonMaxTurns && maxTurns > 0` |
| 10 | 910 | `exitReason ExitReason,` (makeCompletionMessage 参数) | `exitReason verify.ExitReason,` |
| 11 | 921 | `metadataKeyExitReason: string(exitReason),` | 不变 (string conversion 无类型引用) |

并加 import:
```go
"github.com/devrix/devrix/internal/layers/orchestration/executionflow/verify"
```

`sessionorchestrator/turn_orchestrator_test.go` 内 2 处 `ExitReasonNatural` → `verify.ExitReasonNatural` (lines 1386 + 1393, test 字段引用)。

### 3.3 跨包 import cycle 验证

Promote 后的依赖图：
```
sessionorchestrator ──▶ executionflow/verify ──▶ workmodel + orchtypes
                                  ▲
                                  │
                            (无反向依赖)
```

`executionflow/verify/` 内已存在 `anomaly.go` (impl DetectSystemAnomaly), 仅依赖 `workmodel` + `orchtypes` + `d7spans`, 不引用 sessionorchestrator。本次 promote 后 `exit_reason.go` + `verdict_to_exit_reason.go` 加入 verify 包, 同样仅依赖 `workmodel` + `orchtypes`, 无 cycle 风险。

### 3.4 0 函数签名变化 = pure physical migration 黄金标准

与 DM-20260626-004 (`devrix-d7-6s-package-merge`) 同样的"pure physical migration"安全网策略：
- 14 ExitReason 枚举值 string 全部不变 ("natural" / "max_turns" / "aborted_user" / "aborted_llm" / "aborted_tool" / "repeated_tool" / "tool_failure" / "token_diminishing" / "partial_verified" / "verifier_abstain" / "verifier_fail" / "system_anomaly" / "unresolved" / "abstain")
- `VerdictToExitReason` 签名 `(v workmodel.Verdict, sessionID string) ExitReason` 不变
- 5 个 test function (TestVerdictToExitReason_4Kinds / _SystemAnomalyOverrides / _EmptyVerdictKind_DefaultsToAbstain / _UnknownKind_DefaultsToAbstain / _NilConfidence_HandledGracefully / _SessionIDAccepted) 测试矩阵不变
- LP-1/LP-2/LP-5 集成测试零修改零风险

## 4. 备选方案

### 方案 A (推荐): 物理 promote (本次采用)

3 文件 git mv + sed package + 8 处 `ExitReason*` 跨包引用替换。0 函数签名变化。

**优点**: 最简单, 0 风险, LP-1/LP-2/LP-5 100% 兼容, 14 ExitReason 字符串值不变, 6 个测试函数零修改
**缺点**: sessionorchestrator/turn_orchestrator.go 需要 import verify/ 包 (新依赖方向)

### 方案 B (拒绝): 14 ExitReason 拆分为 deterministic (8) + verify-derived (6)

拆 `ExitReason` 为 `DeterministicExitReason` (8 值) + `VerifyExitReason` (6 值), 分别归属 sessionorchestrator + verify。

**优点**: 类型更精确
**缺点**:
- (a) **打破 LP-1/LP-2/LP-5 集成测试**: emitComplete 拿到的 `Metadata["exit_reason"]` 字符串值不变, 但类型从单一 `ExitReason` 变成两个类型 union, 需要 union type 或 wrapper interface, 大幅扩大 scope
- (b) 0 函数签名变化破坏: 任何函数签名变化都会触发集成测试 100% 重跑验证
- (c) 与 DM-20260626-004 的 pure physical migration 安全网策略冲突

### 方案 C (拒绝): 把 8 deterministic ExitReason 也搬到 verify/

让 verify/ 拥有全部 14 ExitReason, sessionorchestrator 不持有任何 ExitReason 概念。

**优点**: 单包统一
**缺点**:
- (a) 违反 S2 SessionOrchestrator 角色边界: SessionOrchestrator 必须**拥有** Turn 终止状态机, ExitReason 是 state 字段类型, 属于 S2 角色 (虽然 6 个 verify-derived 是从 Verifier 输入消费)
- (b) sessionorchestrator/turn_orchestrator.go 内的 `exitReason ExitReason` 字段类型变成 `verify.ExitReason`, 跨包 state 字段, 违反 S2 单包封装

**结论**: 方案 A 是唯一合规且 0 风险的方案。

## 5. 实施路径 (4 步, 0.5 天)

### 第 1 步: Spec 层 (0.5 h)

- d7-domain.md v2.2.0 → v2.3.0 (新增 v2.3.0 changelog row + verify/ 包角色描述更新)
- design.md v4.2.0 → v4.3.0 (verify/ 包结构描述更新 + VerdictToExitReason 实现位置更新)
- t-registry.md v4.4.0 → v4.5.0 (D7-S4-A50 4 新 P0 T IMPLEMENTED, 域 t-registry 218 → 222 IMPLEMENTED)
- 根 t-registry.md v5.4.0 → v5.5.0 (DM-20260626-005 增量条目, 总 P0 350 → 354)

### 第 2 步: 物理迁移 (1 h)

```bash
cd internal/layers/orchestration/
git mv sessionorchestrator/exit_reason.go executionflow/verify/exit_reason.go
git mv sessionorchestrator/verdict_to_exit_reason.go executionflow/verify/verdict_to_exit_reason.go
git mv sessionorchestrator/verdict_to_exit_reason_test.go executionflow/verify/verdict_to_exit_reason_test.go
```

3 文件 `package sessionorchestrator` → `package verify` (sed)

### 第 3 步: 跨包引用更新 (1 h)

```bash
# turn_orchestrator.go 加 import
sed -i '' 's|"github.com/devrix/devrix/internal/layers/orchestration/executionflow/bridge"|&\n\t"github.com/devrix/devrix/internal/layers/orchestration/executionflow/verify"|' \
  internal/layers/orchestration/sessionorchestrator/turn_orchestrator.go

# 8 处 ExitReason* → verify.ExitReason* (精确 sed, 不依赖变量名)
sed -i '' -E 's|\bExitReason\b|verify.ExitReason|g; s|\bExitReason([A-Z][a-zA-Z]*)\b|verify.ExitReason\1|g' \
  internal/layers/orchestration/sessionorchestrator/turn_orchestrator.go \
  internal/layers/orchestration/sessionorchestrator/turn_orchestrator_test.go
```

### 第 4 步: 验证 + 归档 (1.5 h)

- `go build ./...` 0 错误
- `go vet ./...` 0 警告
- `go test -race -count=1 ./internal/layers/orchestration/...` 22/22 PASS
- LP-1 (Bayesian reputation TestAutoClose_FullLP1Loop) / LP-2 (5 节点 TestIntegration_5NodePipeline_End2End) / LP-5 (Cross-session traceability) 100% 兼容
- `sessionorchestrator/autoclose.go` + `hardening/` + `escape/circuit_breaker.go` git diff 0 变化
- S4-Gate: PR + auto-merge (squash)
- S5: acceptance-report.md (13 AC × 9 sections)
- S6: archive 目录 + PR + verify-archive.sh 12/12 PASS

## 6. 关键文件

### 修改文件

- `internal/layers/orchestration/sessionorchestrator/turn_orchestrator.go` (8 处 ExitReason* → verify.ExitReason*)
- `internal/layers/orchestration/sessionorchestrator/turn_orchestrator_test.go` (2 处 ExitReasonNatural → verify.ExitReasonNatural)
- `openspec/specs/d7-orchestration/d7-domain.md` v2.2.0 → v2.3.0
- `openspec/specs/d7-orchestration/design.md` v4.2.0 → v4.3.0
- `openspec/specs/d7-orchestration/t-registry.md` v4.4.0 → v4.5.0
- `openspec/t-registry.md` (root) v5.4.0 → v5.5.0

### 迁移文件 (git mv)

- `internal/layers/orchestration/sessionorchestrator/exit_reason.go` → `internal/layers/orchestration/executionflow/verify/exit_reason.go`
- `internal/layers/orchestration/sessionorchestrator/verdict_to_exit_reason.go` → `internal/layers/orchestration/executionflow/verify/verdict_to_exit_reason.go`
- `internal/layers/orchestration/sessionorchestrator/verdict_to_exit_reason_test.go` → `internal/layers/orchestration/executionflow/verify/verdict_to_exit_reason_test.go`

## 7. T 层 (4 新 P0)

| T 点 | 名称 | 状态 |
|------|------|------|
| **D7-S4-A50-T01** | `sessionorchestrator/{exit_reason,verdict_to_exit_reason,verdict_to_exit_reason_test}.go` 3 文件 `git mv` 到 `executionflow/verify/` | PLANNED |
| **D7-S4-A50-T02** | 3 文件 `package sessionorchestrator` → `package verify` + 8 处 `ExitReason*` 跨包引用替换为 `verify.ExitReason*` | PLANNED |
| **D7-S4-A50-T03** | executionflow/verify/ 包 0 sessionorchestrator 反向依赖 + 跨包 import cycle 0 风险 (单向: sessionorchestrator → verify) | PLANNED |
| **D7-S4-A50-T04** | `go build/vet/test -race` 22/22 orchestration packages 全绿 + LP-1/2/5 集成测试 100% 兼容 + `hardening/` + `escape/circuit_breaker.go` + `sessionorchestrator/autoclose.go` git diff 0 变化 | PLANNED |

A50 编号选择: D7-S4 是 v6.0.0 S4 ExecutionFlow + Verify (Costly Signaler + Certifier); A50 是 S4 下的下一个空位 (S4-A47 system.anomaly_detect + S4-A48 S4-A49 待用).

## 8. 后续 follow-up

| # | Change ID | 范围 |
|---|-----------|------|
| #5 | devrix-d7-6s-observe-merge | observe/orchtypes/ → decisionplanning/ |
| #6 | devrix-d7-6s-bootstrap-slim | wire 14 → 6 (依赖 #4 + #5 完成) |