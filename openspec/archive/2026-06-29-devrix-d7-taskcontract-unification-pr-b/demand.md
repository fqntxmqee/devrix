# Demand: D7 TaskContract 统一 PR-B — Pessimistic Commit + Rule-based Fallback (L3 防御运行时层)

**Demand ID:** DM-20260629-008
**Priority:** P0
**Reporter:** D7 v7.0 演进第二阶段
**DSAFT Domain:** D7 Orchestration (核心域)
**DSAFT Layer:** L3 (防御运行时层)
**Parent Demand:** DM-20260629-006 (DESIGN ONLY) → DM-20260629-007 PR-A 已 S7_Archived
**Sister Change:** devrix-d7-taskcontract-unification-pr-a (DM-20260629-007, S7_Archived 2026-06-29)
**Created:** 2026-06-29

---

## 1. Background

PR-A (DM-20260629-007) 已建立 `internal/layers/orchestration/interfaces/` 纯类型包 + TaskSpec/TaskReport 双契约 + additive embedding 嵌入 ChannelRequest.Spec / LearnRequest.Report。PR-A 范围是 L1 (接口层) + L2 (字段语义层)，L3 (防御运行时层) 仍 **0 实现**。

L3 的核心问题：**资源耗尽时无降级输出**（PR-A 父设计 §3.4 决策树）。当 LLM 资源耗尽、CircuitBreaker L1 触发、VERDICT 连续 INDETERMINATE 等情况下，D7 没有 fallback 路径，要么挂死要么 false success。这是 v6.0.x 留下的"机制丰富 + 防御缺失"核心痛点。

## 2. Scope（PR-B 范围严格限定）

**In scope (P0)**:
- **AC11**: PessimisticCommitGuard F 层实现 (`interfaces/contracts.go::PessimisticCommitGuard` 已登记 S22-F01)
- **AC12**: Rule-based Fallback（4 候选规则：most_tests_passed / compiled_clean / min_cost / min_uncertainty，默认 min_uncertainty）
- **Span**: `d7.s18.pessimistic.commit.emit` (1 个新 P0 span)
- **Metric**: `pessimistic_commit_trigger_count` + 4 fallback_rule_select_total
- **Error Code**: 4 ORCH_PESSIMISTIC_* / ORCH_FALLBACK_* (code range 7110-7113)
- **Feature Flag**: `D7_PESSIMISTIC_COMMIT_ENABLED` env-gated，默认 `disabled`
- **File 新增/修改**:
  - `interfaces/contracts.go` (NEW) — PessimisticCommitGuard interface
  - `interfaces/fallback_policy.go` (NEW) — FallbackPolicy enum + 4 rules
  - `interfaces/convergence_budget.go` (NEW) — ConvergenceBudget 值对象
  - `escape/fallback.go` (NEW) — 3 FallbackPolicy 实现
  - `escape/engine.go` (MODIFIED) — 5 层 CB 读 TaskReport.Blockage 升级触发 Pessimistic
  - `escape/circuit_breaker.go` (MODIFIED) — CB L1 触发 Pessimistic 通知
  - `mups/execute/channel.go` (MODIFIED) — Channel.Execute 出口 + FallbackPolicy 决策
  - `interfaces/contracts_test.go` (NEW) — 单元测试覆盖率 ≥80%
  - `escape/fallback_test.go` (NEW) — 200 LOC 单元测试

**Out of scope (PR-C 处理)**:
- AC13 CoW VersionChain（PR-C 实施）
- AC14 Similarity Check（PR-C 实施）
- AC6 ConvergenceBudget 完整版（PR-B 落地基础 enum，PR-C 完整量化）

## 3. 验收标准

| AC | Name | Layer | 优先级 | T 引用 |
|----|------|-------|--------|--------|
| **AC11** | Pessimistic Commit 防 false success | L3 | P0 | D7-S18-A11-T01/T02/T03 |
| **AC12** | Rule-based Fallback 可插拔 | L3 | P0 | D7-S18-A12-T01/T02 |
| **AC16** | Feature Flag env-gated 默认 disabled | L3 横切 | P0 | D7-S18-A11-T04 |
| **AC18** | Span + Metric + ErrorCode 全链路可观测 | L3 | P0 | D7-S18-A11-T05 |

## 4. 与 PR-A 的边界

| 维度 | PR-A (L1+L2) | PR-B (L3) |
|------|--------------|-----------|
| 接口 | TaskSpec/TaskReport (L1) | PessimisticCommitGuard interface (L3) |
| 字段 | 5+2 字段语义 (L2) | FallbackPolicy enum + 4 rules (L3) |
| 调用点 | ChannelRequest.Spec + LearnRequest.Report (additive) | Channel.Execute 出口 + EscapeEngine NotifyPessimistic (additive) |
| 新文件 | interfaces/ 7 文件 NEW | interfaces/contracts.go + escape/fallback.go + interfaces/convergence_budget.go + 3 test |
| 行为变更 | **0 行为变更** (additive embedding) | **0 行为变更** (Feature Flag 默认 disabled) |
| 0 import 原则 | interfaces 包 0 import D7 子包 | 同上（PR-B 仍守 IV-1） |

## 5. 风险与缓解

| 风险 | 概率 | 缓解 |
|------|------|------|
| Feature Flag 默认 disabled，灰度风险可控 | — | 启用前必须 24h staging 烟测 |
| 5 层 CB 升级逻辑可能影响现有 L1-L5 行为 | 中 | 复用现有 CB 状态机，Pessimistic 仅是 CB L1 触发的副作用 |
| FallbackRuleBased 4 候选规则实现复杂度 | 低 | 先实现 min_uncertainty（默认），其他 3 个 stub + TODO 留给 v7.0.1 |
| 4 个 ORCH_PESSIMISTIC_* ErrorCode 跨包影响 | 低 | 仅在 escape/ + interfaces/ 使用，其他域不感知 |

## 6. 触发场景（5 类）

1. **资源耗尽**：`tokens_remaining <= cost_budget.min_reserve`
2. **EscapeForceExit**：CircuitBreaker L1 触发
3. **VERDICT 连续 ≥ 3 轮 INDETERMINATE**：走 Rule-based Fallback
4. **空证 PASS**（无 test/log/artifact_hash）：强制降级 Kind=Partial + Blockage.RequiredExternal
5. **人工 abort**（用户 / 系统 / IM 通道关闭）：走 FallbackAbort

## 7. 实施周期

- S1-S3 设计：1 天（已含父设计复用 60%）
- S4 实现：3-4 天（新增 ~1200 LOC + 6 test 文件）
- S5 验证：1-2 天
- S6 归档：0.5 天
- **总计**：~1 周

## 8. 依赖

- PR-A (DM-20260629-007) S7_Archived 2026-06-29 ✅
- 父设计 DM-20260629-006 (`archive/2026-06-29-devrix-d7-taskcontract-unification/design.md` 648 行) ✅
- escape/ 包 v5 EscapeEngine 已 S7_Archived (PR-V5.6) ✅

## 9. 后续路径

PR-B → PR-C (CoWVersionChain `S22-F02` ⬜ PLANNED) 完成 4-Layer × 3-Phase 3/3 闭环。
