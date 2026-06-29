# Proposal: D7 TaskContract 统一 PR-B — Pessimistic Commit + Rule-based Fallback (L3 防御运行时层)

**Change ID:** devrix-d7-taskcontract-unification-pr-b
**Demand ID:** DM-20260629-008
**Parent Demand:** DM-20260629-006 (DESIGN ONLY) + DM-20260629-007 PR-A (S7_Archived)
**Status:** s7_archived
**Priority:** P0
**Reporter:** 2026-06-29 启动 v7.0 演进第二阶段
**DSAFT Domain:** D7 Orchestration (核心域)
**DSAFT Layer:** L3 (防御运行时层)

---

## 1. Background

PR-A (DM-20260629-007) 已落地 L1 接口层 + L2 字段语义层，建立了 `internal/layers/orchestration/interfaces/` 纯类型包 + TaskSpec/TaskReport 双契约 + additive embedding 嵌入。PR-A 是 v7.0 演进第一阶段（4-Layer × 3-Phase 中 L1+L2），**L3 防御运行时层仍 0 实现**。

L3 缺失导致的核心问题（按父设计 §3.4 决策树）：
- **资源耗尽时无降级输出**（AC11 Pessimistic Commit）→ 进程挂死或 false success commit
- **VERDICT 多轮 INDETERMINATE 无强制规则**（AC12 Rule-based Fallback）→ 决策瘫痪
- **5 层 CB L1 触发后无显式退出路径**（AC11 Pessimistic action）→ 状态泄漏
- **Verifier "空证 PASS" 防御缺失**（AC15）→ silent corruption

本 PR 落地 L3 第 1 阶段：**Pessimistic Commit + Rule-based Fallback**。AC13/AC14 留 PR-C 实施（CoW VersionChain + Similarity Check）。

## 2. Approach

### 2.1 核心架构

```
Channel.Execute 结束
    ↓
[Check] 5 类触发条件（见 demand §6）
    ↓
[PessimisticCommitGuard.Evaluate()]
    ├─ ok: 正常返回 TaskReport.Result.Kind = Pass/Partial
    └─ blocked: 走 FallbackPolicy 分支
         ├─ FallbackPessimistic [default, AC11]
         │   ├─ 生成 MVPArtifact {Output, RiskWarnings, Trigger, Traceback}
         │   ├─ TaskReport.MVPArtifact = &mvp; FallbackUsed = true
         │   └─ Result.Kind = Indeterminate（强制）
         ├─ FallbackRuleBased [AC12]
         │   ├─ VERDICT 连续 ≥ 3 轮 INDETERMINATE 才触发
         │   ├─ 4 候选规则（most_tests_passed / compiled_clean / min_cost / min_uncertainty）
         │   ├─ env D7_RULE_FALLBACK_STRATEGY 切换
         │   └─ 选中规则 → Kind = Pass（强制覆盖）
         └─ FallbackAbort [v6.0.x 默认，向后兼容]
             └─ 直接 abort，Result.Kind = Failed
```

### 2.2 关键设计决策

1. **PessimisticCommitGuard interface 在 interfaces/contracts.go**（与 PR-A 一致原则：pure types，0 import D7 子包）
2. **FallbackPolicy enum** 在 `interfaces/fallback_policy.go`（值对象，不可变）
3. **ConvergenceBudget 值对象** 在 `interfaces/convergence_budget.go`（tokens 预算 + min_reserve + FallbackPolicy 字段）
4. **Feature Flag 灰度**：`D7_PESSIMISTIC_COMMIT_ENABLED` env-gated，**默认 disabled**（确保 PR-B 合并 0 行为变更）
5. **additive embedding**：Channel.Execute 出口 + EscapeEngine 触发 Pessimistic 是可选的，老路径完全不变
6. **5 层 CB 升级**：`circuit_breaker.go` 读 TaskReport.Blockage 升级（L1 → Pessimistic / L2-L5 保持原行为）

### 2.3 复用 PR-A 资产

| 资产 | 复用方式 |
|------|----------|
| `interfaces.TaskSpec` | PessimisticCommitGuard 接收 `*TaskSpec` 引用 |
| `interfaces.TaskReport` | 出口返回 `*TaskReport`，WithBlockage / WithResource 已被 PR-A 启用 |
| `interfaces.Dissent` | MVPArtifact 风险警告用 Dissent 字段 |
| `sharederrors.WithCode` 模式 | 4 ORCH_PESSIMISTIC_* / ORCH_FALLBACK_* (7110-7113) |
| `interfaces/ 0 import` 原则 | PR-B contracts.go / fallback_policy.go / convergence_budget.go 仍守 IV-1 |

## 3. 4 AC + 7 T 矩阵

| AC | Activity | T | 实施位置 | 状态 |
|----|----------|---|----------|------|
| AC11 | D7-S18-A11 Pessimistic Commit | T01 happy path (资源充足) | interfaces/contracts.go | PLANNED |
| AC11 | D7-S18-A11 | T02 资源耗尽 → MVPArtifact | escape/fallback.go::FallbackPessimistic | PLANNED |
| AC11 | D7-S18-A11 | T03 5 类触发条件 (含 CB L1) | escape/engine.go + circuit_breaker.go | PLANNED |
| AC12 | D7-S18-A12 Rule-based Fallback | T01 4 候选规则 (default min_uncertainty) | escape/fallback.go::FallbackRuleBased | PLANNED |
| AC12 | D7-S18-A12 | T02 env D7_RULE_FALLBACK_STRATEGY 切换 | escape/fallback.go | PLANNED |
| AC16 | D7-S18 横切 Feature Flag | T01 D7_PESSIMISTIC_COMMIT_ENABLED 灰度 | d7-bootstrap/wire.go | PLANNED |
| AC18 | D7-S18 可观测 | T01 d7.s18.pessimistic.commit.emit Span + pessimistic_commit_trigger_count Metric | hardening/span + metric | PLANNED |

## 4. Span / Metric / ErrorCode 清单

### 4.1 Span (1 个 P0 新增)

| Op | Kind | Component | 触发点 | AC |
|----|------|-----------|--------|----|
| `d7.s18.pessimistic.commit.emit` | INTERNAL | escape | `Evaluate()` 返回 MVP | AC11 |

PR-A 已登记的 5 个 Span (`D7_Interfaces_Task_Spec_Created` / `D7_Interfaces_Task_Report_Created` / `D7_TaskReport_Dissent_Recorded` / `D7_TaskReport_Blockage_Recorded` / `D7_TaskReport_Resource_Recorded`) 仍工作，PR-B 仅新增 1 个。

### 4.2 Metric (5 个新增)

| Name | Unit | 触发点 | AC |
|------|------|--------|----|
| `pessimistic_commit_trigger_count` | count | `Evaluate()` 返回 blocked | AC11 |
| `fallback_rule_select_total{rule=most_tests_passed}` | count | FallbackRuleBased 选中 | AC12 |
| `fallback_rule_select_total{rule=compiled_clean}` | count | 同上 | AC12 |
| `fallback_rule_select_total{rule=min_cost}` | count | 同上 | AC12 |
| `fallback_rule_select_total{rule=min_uncertainty}` | count | 同上（默认） | AC12 |

### 4.3 ErrorCode (4 个新增)

| Code | Name | 范围 |
|------|------|------|
| `ORCH_PESSIMISTIC_TRIGGERED` | 资源耗尽触发 Pessimistic | 7110 |
| `ORCH_PESSIMISTIC_MVP_EMPTY` | MVPArtifact 输出为空 | 7111 |
| `ORCH_FALLBACK_RULE_INVALID` | FallbackRuleBased 规则未识别 | 7112 |
| `ORCH_FALLBACK_ABORT_TIMEOUT` | FallbackAbort 超时 | 7113 |

## 5. 文件清单 (7 NEW + 3 MODIFIED)

### NEW (7)
- `internal/layers/orchestration/interfaces/contracts.go` (PessimisticCommitGuard interface)
- `internal/layers/orchestration/interfaces/fallback_policy.go` (FallbackPolicy enum)
- `internal/layers/orchestration/interfaces/convergence_budget.go` (ConvergenceBudget 值对象)
- `internal/layers/orchestration/escape/fallback.go` (3 FallbackPolicy 实现)
- `internal/layers/orchestration/interfaces/contracts_test.go` (≥80% 覆盖)
- `internal/layers/orchestration/escape/fallback_test.go` (200 LOC)
- `internal/layers/orchestration/escape/circuit_breaker_test.go` (CB L1 Pessimistic 升级测试)

### MODIFIED (3)
- `internal/layers/orchestration/escape/engine.go` (Pessimistic action 集成)
- `internal/layers/orchestration/escape/circuit_breaker.go` (CB L1 触发 Pessimistic 通知)
- `internal/layers/orchestration/mups/execute/channel.go` (Channel.Execute 出口 + FallbackPolicy 决策)
- `internal/layers/orchestration/d7-bootstrap/wire.go` (Feature Flag 注入)

### SPEC DOCS (6 同步)
- `openspec/specs/d7-orchestration/spec.md` (新增 3 ADDED Requirements for D7-S18)
- `openspec/specs/d7-orchestration/d7-domain.md` (§8 Layer 4-Layer × 3-Phase PR-B 落地)
- `openspec/specs/d7-orchestration/a-registry.md` (D7-S18-A11/A12 2 A entries)
- `openspec/specs/d7-orchestration/f-registry.md` (D7-S22-F01 PessimisticCommitGuard 状态 PLANNED → IMPLEMENTED)
- `openspec/specs/d7-orchestration/span-registry.md` (1 个新 P0 span ops)
- `openspec/specs/d7-orchestration/t-registry.md` (7 个 P0 T 登记)

## 6. 行为变更范围

**0 行为变更**（PR-B 与 PR-A 同样原则）：
- `D7_PESSIMISTIC_COMMIT_ENABLED=false` (default) 时 PessimisticCommitGuard 永远返回 ok（不触发 fallback）
- 5 层 CB 行为完全不变
- Channel.Execute 出口语义不变
- 仅在 Feature Flag 显式 enabled 时，5 类触发条件才走 fallback 路径

## 7. 验证策略

- `go test -race -count=1 ./internal/layers/orchestration/...` → 24+ packages PASS
- `go test -cover ./internal/layers/orchestration/interfaces/` → ≥ 80% (含 contracts.go)
- `go test -cover ./internal/layers/orchestration/escape/` → ≥ 80% (含 fallback.go)
- LP-1/LP-2/LP-5 集成测试 100% 兼容（Feature Flag 默认 disabled）
- 灰度验证：`D7_PESSIMISTIC_COMMIT_ENABLED=true` staging 烟测 24h 后再 prod

## 8. 风险

| 风险 | 概率 | 缓解 |
|------|------|------|
| Feature Flag 灰度期间 false negative | 中 | 灰度分桶 1% → 10% → 50% → 100% 渐进 |
| 4 候选规则实现复杂度（Rule-based） | 低 | 先实现 min_uncertainty，其他 3 个 stub + TODO 留给 v7.0.1 |
| CB L1 Pessimistic action 状态机复杂度 | 中 | 复用 PR-A TaskReport.Blockage 字段，L1 仅是 action 之一 |
| 4 个 ORCH_* 错误码与 PR-A 5 个共占 7100-7113 区间 | 0 | 区间划分：7100-7109 PR-A / 7110-7119 PR-B / 7120-7129 PR-C |

## 9. 时间线

| 阶段 | 日期 | 输出 |
|------|------|------|
| S1-S3 设计 (本 Change) | 2026-06-29 | proposal + design + tasks + review-design + specs/ |
| S3-Gate | 2026-06-30 | review-design.md PASS |
| S4 实现 | 2026-06-30 ~ 2026-07-02 | 7 NEW + 3 MODIFIED + spec sync |
| S5 验证 | 2026-07-03 | 24+ packages -race PASS + 覆盖率 ≥ 80% |
| S6 归档 | 2026-07-03 | archive/ + PR auto-merge |
| **总计** | **~4 天** | PR-B S7_Archived |

## 10. 后续路径

PR-B S7_Archived → PR-C (CoWVersionChain `S22-F02` ⬜ PLANNED) 实施 4-Layer × 3-Phase 3/3 闭环 → v7.0.0 minor release。
