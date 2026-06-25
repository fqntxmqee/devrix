---
demand-id: DM-20260626-003
title: D7 编排层 hardening/ 横切包迁移 — metrics.go + recovery.go → orchestration/hardening/ (v6.0.0 Step 3 落地)
priority: P1
status: S1_Proposal
dsaft_domain: architecture
created: 2026-06-26
---

# D7 hardening/ 横切包迁移

## 1. 背景

`devrix-d7-six-s-simplification` (DM-20260626-001) 在 v6.0.0 域升级中，把 D7 编排层的 14 S 博弈角色精简为 6 S + 1 横切。spec 文档层 + 代码包路径层（mups subtree）已对齐：

- 14 S → 6 S 文档 ✓ (PR #215)
- execute/ + learn/ → mups/ 子树物理迁移 ✓ (PR #216)
- 22 orchestration packages `go test -race` 100% PASS ✓

但 6 S + 1 横切中"横切 Discipline Keeper"对应的 `orchestration/hardening/` 包仍是空白（PR #215 设计文档中提及，但代码侧未落地）。当前 hardening 关注点散落在多处：

| 当前散落位置 | 内容 | 行数 | 应归 |
|------------|------|------|------|
| `sessionorchestrator/metrics.go` | InterruptMetrics 跨 canceller 失败计数器 | 61 | hardening |
| `turn/recovery.go` | LLM 错误恢复 helpers (IsContextLengthError 等) | 133 | hardening |
| `escape/circuit_breaker.go` | 5-Layer CircuitBreaker (L0-L5) | 420 | **保持 escape**（核心机制）|

## 2. 问题陈述

虽然 6 S + 1 横切文档已显式声明"横切 = Discipline Keeper → `orchestration/hardening/`"，但代码侧仍按 14 S 时期散落模式：

1. **metrics.go 散落 sessionorchestrator/**：InterruptMetrics 是跨 canceller 失败计数器，纯横切观测性质
2. **recovery.go 散落 turn/**：LLM 错误恢复 helpers（context length / 5xx / rate limit / stream tombstone）是跨 S 通用错误处理，不是 turn 专属
3. **circuit_breaker.go 留 escape/**：5-Layer CB 是 EscapeEngine 核心机制（engine.go + loop_depth_tracker.go 等深度依赖），不是横切观测基础设施

**具体后果：**
- hardening/ 包空缺，未来新增 metrics / recovery helpers 时继续散落到调用方
- bootstrap wire 14 → 6 收口无法完成（follow-up #6 依赖 hardening/ 落地）
- 跨 S 边界隐式依赖混乱

## 3. 目标

把 D7 编排层中**真正横切**的 2 类关注点收口到 `orchestration/hardening/` 新包，对齐 v6.0.0 6 S + 1 横切文档承诺：

```
orchestration/hardening/   (NEW, Discipline Keeper 横切角色)
├── metrics.go             ← sessionorchestrator/metrics.go (61 行) — InterruptMetrics + Snapshot
└── recovery.go            ← turn/recovery.go (133 行) — IsContextLengthError + IsOverloadOr5xx + compressMessagesForRecovery + NeedsMaxOutputTokenRecovery + emitStreamRecoveryTombstones + invokeStreamWithRecovery
```

**注：** `escape/circuit_breaker.go` 不动（420 行 5-Layer CB 是 EscapeEngine 核心机制，详见 §4）。

**横切包职责边界：**
- `hardening/metrics.go`：跨 canceller 失败计数器 + observability 快照（InterruptMetrics 等）
- `hardening/recovery.go`：跨 S 错误恢复 helpers（context length / 5xx / rate limit / stream tombstone）

**用户硬约束：**
- 函数签名 / 行为 / 对外接口 0 变化
- package hardening 声明全新（metrics.go + recovery.go 全部重命名为 `package hardening`）
- bootstrap wire_coordinator.go 中 14 wire 暂不动（follow-up #6 单独处理）
- EscapeEngine 5-Layer CB 逻辑 0 变化

## 4. circuit_breaker.go 不动的原因

原计划 §硬切 Discipline Keeper 列出 3 文件（metrics.go + circuit_breaker.go + error_recovery.go）。但探查发现 `escape/circuit_breaker.go` 实质是 **EscapeEngine 的核心机制**，不是横切观测：

| 证据 | 说明 |
|------|------|
| `escape/engine.go` line 40 `cbSet *CircuitBreakerSet` | EscapeEngine 字段直接持有 CBSet 引用 |
| `escape/engine.go` line 170 `(e *EscapeEngine) CircuitBreakerSet()` | EscapeEngine 暴露 CBSet 给调用方 |
| `escape/circuit_breaker.go` line 64 `CircuitBreaker interface` 含 Evaluate() | Evaluate 返回 EscapeDecision 喂给 EscapeEngine 决策链 |
| `escape/loop_depth_tracker.go` line 44 注释引用 CB | LoopDepthTracker 与 CB 协同实现 13 类失败降级矩阵 |
| 5-Layer CB (L0-L5) + CircuitBreakerSet + EvaluateAll | 整体是 EscapeEngine 的决策机制（DM-20260625-003 V5.4 已闭环）|

如果硬迁到 hardening/：
- EscapeEngine 需 import hardening（**违反依赖方向**：EscapeEngine 是核心机制，hardening 才是横切观测）
- engine.go 需重写（cbSet 字段从 `*escape.CircuitBreakerSet` → `*hardening.CircuitBreakerSet`，反向依赖）
- loop_depth_tracker.go + integration_test.go 需同步改
- 与 V5 EscapeEngine 已闭环的 doc 38 §21 + design.md §7 SoT 不符

**结论：** circuit_breaker.go **留在 escape/**，是 EscapeEngine 不可分割的一部分。本次只迁 metrics.go + recovery.go。如果未来需要"CB 状态对外 observability 暴露"（Snapshot/Counter），可以在 hardening/metrics.go 中新增 wrapper（通过 EscapeEngine 注入），而非搬 CB 本身。

## 5. 非目标

- ❌ 不动 EscapeEngine 5-Layer CB 逻辑（`escape/circuit_breaker.go` 整文件保留）
- ❌ 不动 InterruptMetrics / recovery helpers 任何函数签名
- ❌ 不改 bootstrap wire 14 → 6（follow-up #6 单独处理）
- ❌ 不动 5 个新 P0/P1 Span emit 路径（channel.route / memory.persist / system.anomaly_detect / taskgraph.synthesize / executor.select）
- ❌ 不动 LP-1 / LP-2 / LP-5 路径

## 6. 验收标准

- AC1: `orchestration/hardening/` 目录创建，metrics.go + recovery.go 2 .go 文件 git mv rename 100%
- AC2: `package hardening` 声明在 2 文件保持一致
- AC3: 全仓 `grep "orchestration/sessionorchestrator/metrics"` 0 命中
- AC4: 全仓 `grep "orchestration/turn/recovery"` 0 命中（仅 `orchestration/hardening/recovery"` 命中）
- AC5: `go build ./...` 0 错误（sessionorchestrator/interrupt.go + interrupt_test.go + metrics_test.go 引用更新；turn/orchestrator.go + recovery_test.go 引用更新）
- AC6: `go vet ./...` 0 警告
- AC7: `go test ./internal/layers/orchestration/... -race -count=1` 23/23 PASS（原 22 + 新 hardening 1 包）
- AC8: 全仓 unit tests 通过
- AC9: LP-1/LP-2/LP-5 集成测试 100% 兼容
- AC10: circuit_breaker.go 0 变化（escape 包保持原状）

## 7. 工作量预估

- 探查 + demand.md + proposal.md + design.md + spec.md: 0.3 天
- Step 1 物理目录创建 + git mv 2 文件: 0.05 天
- Step 2 import path 全仓替换: 0.1 天（中断点：metrics_test.go + interrupt.go + interrupt_test.go + recovery_test.go + orchestrator.go 全部需要更新 import + package 引用）
- Step 3 build + vet + test 回归 + LP 兼容验证: 0.1 天
- Step 4 文档同步 + 验收报告: 0.1 天
- S6 归档: 0.05 天

**总计**: ~0.7 天（参考值，实际以实施为准）

## 8. 前置依赖

- devrix-d7-six-s-simplification (DM-20260626-001) S7_Archived ✓
- devrix-d7-mups-package-migration (DM-20260626-002) S7_Archived ✓

## 9. 后续依赖

- devrix-d7-6s-bootstrap-slim (follow-up #6): 14 wire → 6 wire 收口需要 hardening/ 已落地

## 10. 风险评估

| 风险 | 等级 | 缓解 |
|------|------|------|
| metrics.go 内部 interrupt.go 引用（Metrics 字段类型）需跨包引用 hardening.InterruptMetrics | 中 | 中断点：interrupt.go + interrupt_test.go + metrics_test.go 全部加 `import "hardening"` + 字段类型 `*hardening.InterruptMetrics` |
| recovery.go 内部 orchestrator.go 引用（compressMessagesForRecovery + invokeStreamWithRecovery 是 receiver method） | 中 | receiver method 不能简单搬，需保留在 turn/ 但调用 hardening helpers；详见 design.md §3 Decision 2 |
| hardening 包从 0 文件新建，可能缺 doc.go | 低 | 在 hardening/ 中加 doc.go 说明横切职责（参考 design.md §①）|
| 中间态编译失败 | 低 | Step 1 (git mv) + Step 2 (sed) + Step 3 (test) 分离；PR 未合入不影响 master |
