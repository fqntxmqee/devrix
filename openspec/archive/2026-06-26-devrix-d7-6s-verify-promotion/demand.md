# DM-20260626-005: exit_reason + verdict_to_exit_reason 从 sessionorchestrator/ promote 到 executionflow/verify/

**Demand ID:** DM-20260626-005
**Status:** S1_Requirement
**Priority:** P1
**Created:** 2026-06-26
**Author:** D7 编排层 Certifier 包归属梳理（v6.0.0 follow-up #4）
**Related Change:** devrix-d7-six-s-simplification (DM-20260626-001)
**Follow-up Chain:** devrix-d7-6s-package-merge (#3) → **devrix-d7-6s-verify-promotion (#4, 本次)** → devrix-d7-6s-observe-merge (#5) → devrix-d7-6s-bootstrap-slim (#6)

---

## 1. 背景

v6.0.0 域升级 follow-up 序列中，**devrix-d7-6s-package-merge (DM-20260626-004)** 已把 D7 编排层中独立存在的 `turn/` 子包（25 .go / 6467 行）物理合并到 `sessionorchestrator/`，让 S2 SessionOrchestrator (Mediator + Turn Leader + Error Recovery) 单一博弈角色恢复单包封装。

合并过程中，`exit_reason.go` (72 行) + `verdict_to_exit_reason.go` (49 行) + `verdict_to_exit_reason_test.go` (97 行) **3 个文件被临时留在了 `sessionorchestrator/` 包内**，原因是：

1. **合并当时归属不清**：S2 SessionOrchestrator 角色名义上"owns" Turn 终止原因（驱动 `exitReason` 状态机字段），但 `ExitReason` 14 枚举值里有 6 个（`partial_verified` / `verifier_abstain` / `verifier_fail` / `system_anomaly` / `unresolved` / `abstain`）是 Phase 4 Verify 升格后新增的 Verify-derived stop conditions，归属上更属于 **S4 ExecutionFlow + Verify (Costly Signaler + Certifier)** 角色。
2. **`VerdictToExitReason` 是 Certifier 角色核心能力**：它是 Verifier Verdict → orchestrator-level ExitReason 的 4 态映射（VerdictPass → Natural / VerdictPartial → PartialVerified / VerdictIndeterminate → VerifierAbstain / VerdictFail → VerifierFail），属于 S4 Certifier 角色的可验证承诺 (Costly Signaler + Certifier)，不是 S2 SessionOrchestrator 的职责。
3. **避免单次 PR scope 膨胀**：DM-20260626-004 单 PR 已经 25 文件 + 14 importer + 跨包 cycle 打破，再加 3 文件 promote 会让 review 风险翻倍（单 PR scope 应 ≤ 60 文件）；本次独立 PR 收口。

## 2. 问题陈述

3 文件临时留在 `sessionorchestrator/` 包导致：

| 问题 | 影响 |
|------|------|
| **(a) 角色与代码归属不对齐** | `ExitReason` 中 6/14 枚举值是 Verify 衍生 (`partial_verified` / `verifier_abstain` / `verifier_fail` / `system_anomaly` / `unresolved` / `abstain`)，但代码全在 `sessionorchestrator/`；S4 Certifier 角色的可验证承诺 (14 ExitReason) 在 spec 层与代码层脱钩 |
| **(b) `VerdictToExitReason` 跨角色调用断裂** | 真正的 verifier 节点（executionflow/verify/anomaly.go）在调用此函数时需要 `import "internal/layers/orchestration/sessionorchestrator"`，跨级调用方向反了（Certifier 是 S4，SessionOrchestrator 是 S2，S4 不应该依赖 S2 实现细节）|
| **(c) 测试点归属错位** | `verdict_to_exit_reason_test.go` 5 个 test function 测试的是 Certifier 行为，但放在 `sessionorchestrator/` 包下，与 A 层 `D7-S10-A33 VerdictToExitReason` 注册到 S4 (ExecutionFlow + Verify) 不一致 |
| **(d) 为后续 follow-up #6 (6s-bootstrap-slim) 制造前置障碍** | `wire_coordinator.go` 中需要分别 wire sessionorchestrator 包内的 ExitReason 常量 + verify 包的 anomaly aggregator，未来 6 S 全部落地后还需要再做一次 wire 收敛 |

## 3. 目标

**3 文件物理 promote**：`sessionorchestrator/{exit_reason.go, verdict_to_exit_reason.go, verdict_to_exit_reason_test.go}` → `executionflow/verify/`，让 S4 Certifier 角色的可验证承诺 (14 ExitReason + VerdictToExitReason 4 态映射) 在 spec 层与代码层完全对齐。

**0 函数签名变化**（pure physical migration）：保持 `func VerdictToExitReason(v workmodel.Verdict, sessionID string) ExitReason` 签名不变，保持 `ExitReason` 类型定义不变，保持 14 枚举值定义不变。

**0 行为变化**：所有 5 个 test function 测试矩阵（4 Kinds / SystemAnomaly Override / Empty VerdictKind / Unknown Kind / Nil Confidence / SessionID Accepted）保持 100% PASS。

## 4. 验收标准

| AC | 描述 | 验收方式 |
|----|------|----------|
| AC1 | `sessionorchestrator/{exit_reason.go, verdict_to_exit_reason.go, verdict_to_exit_reason_test.go}` 3 文件 `git mv` 到 `executionflow/verify/` | `git log --follow` 显示 100% rename detection |
| AC2 | 3 文件 `package sessionorchestrator` → `package verify` 改名一致 | `head -1` 三文件一致 |
| AC3 | `sessionorchestrator/turn_orchestrator.go` 中 8 处 `ExitReason*` 引用（state 字段类型 + 6 个常量 + resolveFinalText 函数签名）改为 `verify.ExitReason*` | grep 0 残留 `ExitReason[^a-zA-Z]` 在 sessionorchestrator/ 包内 |
| AC4 | `executionflow/verify/exit_reason.go` 仅依赖 `workmodel` (无 sessionorchestrator 循环引用) | `go list -deps ./internal/layers/orchestration/executionflow/verify` 不含 sessionorchestrator |
| AC5 | `executionflow/verify/verdict_to_exit_reason.go` 仅依赖 `workmodel` + `orchtypes` (无 sessionorchestrator 循环引用) | 同上 |
| AC6 | 5 个 test function (`TestVerdictToExitReason_4Kinds` / `_SystemAnomalyOverrides` / `_EmptyVerdictKind_DefaultsToAbstain` / `_UnknownKind_DefaultsToAbstain` / `_NilConfidence_HandledGracefully` / `_SessionIDAccepted`) 全 PASS | `go test -race ./internal/layers/orchestration/executionflow/verify/... -count=1` PASS |
| AC7 | `go build ./...` 0 错误 | terminal output |
| AC8 | `go vet ./...` 0 警告 | terminal output |
| AC9 | 22/22 orchestration packages `go test -race -count=1` 全 PASS | terminal output |
| AC10 | LP-1 (Bayesian reputation) / LP-2 (Memory 3 通道) / LP-5 (Cross-session traceability) 集成测试 100% 兼容 | 现有 d7 integration test 套件 PASS |
| AC11 | `sessionorchestrator/autoclose.go` + `hardening/` + `escape/circuit_breaker.go` git diff 0 变化 (本次 promote 不影响这些 baseline-stability 文件) | `git diff` 0 hits |
| AC12 | spec 同步：`openspec/specs/d7-orchestration/d7-domain.md` v2.2.0 → v2.3.0 + `design.md` v4.2.0 → v4.3.0 + `t-registry.md` v4.4.0 → v4.5.0 + 根 `t-registry.md` v5.4.0 → v5.5.0 | version bump + changelog row |
| AC13 | `verify-archive.sh devrix-d7-6s-verify-promotion` 12/12 PASS | terminal output |

## 5. 风险与约束

| 风险 | 等级 | 缓解 |
|------|------|------|
| 跨包 import cycle (sessionorchestrator ↔ executionflow/verify) | Low | executionflow/verify/ 已存在且 0 sessionorchestrator dep；promote 后单向依赖 `sessionorchestrator → verify` (DAG 方向合法：SessionOrchestrator 在 Turn 终止时引用 Verify 输出的 ExitReason) |
| `ExitReason*` 常量在 sessionorchestrator 内大量引用 (turn_orchestrator.go 内 8 处) | Medium | sed 批量替换 `ExitReason` → `verify.ExitReason` (精确 8 处), 加 import `"internal/layers/orchestration/executionflow/verify"` |
| LP-1/LP-2/LP-5 路径行为变化 | Low | 0 函数签名变化 + 0 行为变化 (pure physical migration)；所有 14 ExitReason 枚举值 string 值不变 |
| 测试点 `D7-S2-A50-T04` (turn/ 0 残留) 与本次 promote 冲突 | Low | DM-20260626-004 的 T04 只验证 `orchestration/turn/` 目录 0 残留 + `hardening/` 0 变化 + `escape/circuit_breaker.go` 0 变化 + `sessionorchestrator/autoclose.go` 0 变化；本次 promote 让 `sessionorchestrator/exit_reason.go` 等 3 文件**移到** verify/，但 `sessionorchestrator/` 包内**只剩 0 个 ExitReason 引用** (turn_orchestrator.go 8 处全替换为 `verify.ExitReason*`)，所以 T04 的"0 残留"实际语义升级为"sessionorchestrator/ 包内 0 ExitReason 残留 + exit_reason.go 等 3 文件已迁 verify/" |
| 4 P0 T 编号占用 (D7-S2-A50 已被 DM-20260626-004 占用) | Low | 本次 T 编号使用 `D7-S4-A50` (S4 Certifier 角色 A50 = 本次 promote)，与 S2-A50 区分 |

## 6. 范围

**In Scope:**
- 3 文件 `git mv` (exit_reason.go + verdict_to_exit_reason.go + verdict_to_exit_reason_test.go)
- 3 文件 `package sessionorchestrator` → `package verify` (sed)
- sessionorchestrator/turn_orchestrator.go 中 8 处 `ExitReason*` → `verify.ExitReason*` (sed + 加 import)
- sessionorchestrator/turn_orchestrator_test.go 中 2 处 `ExitReasonNatural` → `verify.ExitReasonNatural` (test 引用)
- spec 同步 (d7-domain.md v2.3.0 + design.md v4.3.0 + t-registry v4.5.0 + 根 v5.5.0)
- D7-S4-A50 4 新 P0 T (T01-T04) IMPLEMENTED

**Out of Scope:**
- `ExitReason` 类型重设计 / 14 枚举值字符串重命名 (本次不动语义, 只搬位置)
- `VerdictToExitReason` 函数签名变化 (本次保持 `(v workmodel.Verdict, sessionID string) ExitReason` 不变)
- S4 ExecutionFlow + Verify 其他文件 promote (本次只 promote 3 文件; anomaly.go + 其他 verify 文件已在 verify/)
- D7-S10-A33 `MapVerdictToExitReason` 单独 spec 入口 (那是后续 v6.0.1 follow-up; 本次 promote 是 pure physical migration, 不重命名函数)
- D7 14 S → 6 S 文档语义保持不变 (本次不影响 S 层归类)
- 5 个新 P0/P1 Span emit 路径 0 变化
- multiagent/ 域不动
- harden/escape/autoclose 0 变化

## 7. 依赖

| 上游依赖 | 状态 | 说明 |
|----------|------|------|
| `devrix-d7-6s-package-merge` (DM-20260626-004) | ✅ S7_Archived (2026-06-26) | turn/ 整包已合并到 sessionorchestrator/ |
| `workmodel.Verdict` 类型 | ✅ 存在 | verdict_to_exit_reason.go 依赖 |
| `orchtypes.VerdictKind` 4 态枚举 | ✅ 存在 | verdict_to_exit_reason.go 依赖 |
| `executionflow/verify/anomaly.go` | ✅ 存在 | verify/ 包已存在, 本次 promote 是把 3 文件加入 |

| 下游消费 | 影响 |
|----------|------|
| `devrix-d7-6s-observe-merge` (#5) | observe/orchtypes/ → decisionplanning/; 与本次 verify-promotion 并行, 无依赖 |
| `devrix-d7-6s-bootstrap-slim` (#6) | wire 14 → 6; 依赖 #4 + #5 完成后才能收口 |