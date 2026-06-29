# Spec Delta: D7 v7.0 TaskContract PR-B (DM-20260629-008)

**Change ID:** devrix-d7-taskcontract-unification-pr-b
**Demand ID:** DM-20260629-008
**Phase:** PR-B (L3 防御运行时层)
**Status:** S3_Design (待 S3-Gate review)
**Created:** 2026-06-29

> **本 delta 文件遵循 OpenSpec 规范。** S4 实施时合并到 `openspec/specs/d7-orchestration/spec.md` v4.16.0 → v4.17.0 新增 3 ADDED Requirements + 12 Gherkin Scenarios。本文件仅作为 S3-Gate review 锚点 + S4 实施清单。

---

## ADDED Requirements (PR-B scope 3 NEW)

### Requirement: D7-S18-A11 Pessimistic Commit 防御运行时（L3 防御运行时层）

`PessimisticCommitGuard` MUST evaluate 5 类触发条件 (资源耗尽 / EscapeForceExit (CB L1) / VERDICT 连续 ≥ 3 轮 INDETERMINATE / Verifier 空证 PASS / 人工 abort) 并在 Feature Flag `D7_PESSIMISTIC_COMMIT_ENABLED=true` 时返回 `ok=false` 触发 `FallbackPessimistic`（生成 MVPArtifact + TaskReport.FallbackUsed=true + Result.Kind=Indeterminate）。

**Priority:** P0
**Layer:** L3 防御运行时层
**Package:** `internal/layers/orchestration/interfaces/contracts.go` + `internal/layers/orchestration/escape/fallback.go`
**Contract:** 4 ORCH_* SentinelError (code 7110-7113)
**T:** D7-S18-A11-T01 / T02 / T03 / T04 / T05

#### Scenario: 资源充足 happy path

- GIVEN `D7_PESSIMISTIC_COMMIT_ENABLED=true` AND `tokens_remaining > min_reserve` AND CB L0
- WHEN `PessimisticCommitGuard.Evaluate(spec, report, budget)` is called
- THEN returns `(ok=true, blockedReason="", err=nil)`
- AND `FallbackUsed=false`
- AND `Result.Kind` 保持原值 (Pass / Partial)

#### Scenario: 资源耗尽 → MVPArtifact

- GIVEN `tokens_remaining <= min_reserve` (resource exhausted)
- WHEN `PessimisticCommitGuard.Evaluate(spec, report, budget)` is called
- THEN returns `(ok=false, blockedReason="resource_exhausted", err=ErrORCHPessimisticTriggered)`
- AND `PessimisticCommitGuard.BuildMVPArtifact(report, "resource_exhausted")` 生成 MVPArtifact {Output, RiskWarnings, Trigger="resource_exhausted", Traceback, GeneratedAtMs}
- AND `TaskReport.WithMVPArtifact(mvp)` immutable builder 返回新副本
- AND `Result.Kind=Indeterminate` (强制)

#### Scenario: 5 层 CB L1 → Pessimistic action

- GIVEN CircuitBreaker L1 触发 (`state.cancels` 释放 + `engine.NotifyPessimistic` 调用)
- WHEN `escape/engine.go::NotifyPessimistic(report)` 接收
- THEN 读 `report.Blockage` 字段
- AND 触发 `PessimisticCommitGuard.Evaluate` → FallbackPessimistic 路径
- AND 触发 `d7.s18.pessimistic.commit.emit` Span
- AND `pessimistic_commit_trigger_count` Metric 计数 +1
- AND 5 层 CB L2-L5 行为完全不变 (复用 PR-A Blockage 字段)

#### Scenario: Feature Flag 默认 disabled (0 行为变更)

- GIVEN `D7_PESSIMISTIC_COMMIT_ENABLED=false` (default)
- WHEN `PessimisticCommitGuard.Evaluate(spec, report, budget)` is called (任意 5 类触发)
- THEN 永远返回 `(ok=true, blockedReason="", err=nil)`
- AND 老路径完全不变
- AND `FallbackUsed` 永远 false
- AND LP-1/LP-2/LP-5 集成测试 100% 兼容

#### Scenario: Verifier "空证 PASS" → Partial + Blockage

- GIVEN Verifier 提取 verdict 但缺 test/log/artifact_hash (空证)
- WHEN `extractVerdict` 判定为 `Kind=Partial` + `Blockage.Kind=RequiredExternal`
- THEN 触发 `PessimisticCommitGuard.Evaluate` → FallbackPessimistic
- AND MVPArtifact.Trigger="empty_evidence"
- AND `Result.Kind=Indeterminate` (强制)

---

### Requirement: D7-S18-A12 Rule-based Fallback 可插拔

`FallbackRuleBased` MUST 在 VERDICT 连续 ≥ 3 轮 INDETERMINATE 时按 `D7_RULE_FALLBACK_STRATEGY` env 切换 4 候选规则（most_tests_passed / compiled_clean / min_cost / min_uncertainty, default min_uncertainty），选中规则后强制 `Result.Kind=Pass` 覆盖 INDETERMINATE。

**Priority:** P0
**Layer:** L3 防御运行时层
**Package:** `internal/layers/orchestration/escape/fallback.go::FallbackRuleBased`
**T:** D7-S18-A12-T01 / T02

#### Scenario: 4 候选规则 (default min_uncertainty)

- GIVEN `D7_RULE_FALLBACK_STRATEGY` 未设置 (default) AND VERDICT 连续 ≥ 3 轮 INDETERMINATE
- WHEN `FallbackRuleBased.Select(report)` 调用
- THEN 选中 `min_uncertainty` 规则
- AND `Result.Kind=Pass` (强制覆盖)
- AND `fallback_rule_select_total{rule=min_uncertainty}++`
- AND 选中规则名写入 `TaskReport.Blockage.RuleName`

#### Scenario: env 切换规则

- GIVEN `D7_RULE_FALLBACK_STRATEGY=most_tests_passed` (显式设置)
- WHEN `FallbackRuleBased.Select(report)` 调用
- THEN 选中 `most_tests_passed` 规则
- AND `Result.Kind=Pass` (强制覆盖)
- AND `fallback_rule_select_total{rule=most_tests_passed}++`
- AND 其他 3 规则 (compiled_clean / min_cost / min_uncertainty) 不计数

---

### Requirement: D7-S18-A13 PessimisticCommitGuard 横切可观测

`d7.s18.pessimistic.commit.emit` Span + `pessimistic_commit_trigger_count` Metric + 4 ORCH_* SentinelError (code 7110-7113) MUST 构成 Pessimistic Commit 全链路可观测（Jaeger / Prometheus / sharederrors）。

**Priority:** P0
**Layer:** L3 防御运行时层（横切可观测）
**Package:** `internal/layers/orchestration/hardening/` (Metric + Span 注册) + `internal/layers/orchestration/escape/engine.go` (Emit 触发)
**T:** D7-S18-A11-T05 (Span + Metric 链路)

#### Scenario: Span + Metric + ErrorCode 三件套

- GIVEN `PessimisticCommitGuard.Evaluate` 返回 `ok=false`
- WHEN `engine.NotifyPessimistic` 触发 emit
- THEN `d7.s18.pessimistic.commit.emit` Span 落 Jaeger (key attrs: session_id, blocked_reason, fallback_policy, mvp_trigger)
- AND `pessimistic_commit_trigger_count` Metric 计数 +1 (Prometheus)
- AND `ErrORCHPessimisticTriggered` (code 7110) 通过 sharederrors 注入
- AND D5 dashboard 通过 Span + Metric 字段直接过滤无需进入子 span

---

## 7 T 矩阵 (PR-B S7 落地承诺)

| T ID | Name | 实施位置 | AC |
|------|------|----------|-----|
| D7-S18-A11-T01 | PessimisticCommitGuard.Evaluate happy path | interfaces/contracts.go | AC11 |
| D7-S18-A11-T02 | PessimisticCommitGuard.BuildMVPArtifact | escape/fallback.go::FallbackPessimistic | AC11 |
| D7-S18-A11-T03 | 5 层 CB L1 → Pessimistic action | escape/circuit_breaker.go + engine.go | AC11 |
| D7-S18-A11-T04 | Feature Flag env-gated | d7-bootstrap/wire.go | AC16 |
| D7-S18-A11-T05 | Span + Metric + ErrorCode | hardening + escape/engine.go | AC18 |
| D7-S18-A12-T01 | 4 候选规则 (default min_uncertainty) | escape/fallback.go::FallbackRuleBased | AC12 |
| D7-S18-A12-T02 | env D7_RULE_FALLBACK_STRATEGY 切换 | escape/fallback.go | AC12 |

## 6 IV 不变量 (PR-B 强化)

- **IV-1** (沿用 PR-A): interfaces/ 包 0 import D7 子包
- **IV-2** (PR-B 新增): FallbackPolicy immutable enum
- **IV-3** (PR-B 新增): ConvergenceBudget immutable value object
- **IV-4** (PR-B 新增): MVPArtifact immutable
- **IV-5** (PR-B 新增): PessimisticCommitGuard 4 错误码仅 escape + interfaces 使用
- **IV-6** (PR-B 新增): Feature Flag 默认 disabled (0 行为变更)

## 1 Span op 新增

- `d7.s18.pessimistic.commit.emit` → spec.md span-registry v4.4.0 登记为 `D7_Escape_Pessimistic_Commit_Emit` P0 span op

## Statistics

- **新增 3 ADDED Requirements** (D7-S18-A11/A12/A13)
- **新增 12 Gherkin Scenarios** (5 + 2 + 1 = 8 主体 + 4 补充)
- **新增 7 P0 T 矩阵** (D7-S18-A11-T01..T05 + D7-S18-A12-T01..T02)
- **新增 4 AC** (AC11 + AC12 + AC16 + AC18)
- **新增 6 IV** (1 沿用 + 5 新增)
- **新增 1 Span op** (d7.s18.pessimistic.commit.emit)
- **新增 5 Metric** (pessimistic_commit_trigger_count + fallback_rule_select_total{4 labels} + mvp_artifact_generated_total + pessimistic_commit_latency_us + fallback_rule_apply_total)
- **新增 4 ORCH_* SentinelError** (code 7110-7113)

## PR-B vs PR-C 边界

| 维度 | PR-B (L3 防御运行时) | PR-C (L3 CoW + 防御补全) |
|------|----------------------|--------------------------|
| 主目标 | Pessimistic + Rule-based | CoW VersionChain + Similarity Check |
| AC | AC11 + AC12 + AC16 + AC18 | AC13 + AC14 + AC15 |
| 文件 | 7 NEW + 4 MODIFIED | TBD (PR-C 单独设计) |
| Feature Flag | `D7_PESSIMISTIC_COMMIT_ENABLED` | `D7_COW_VERSIONCHAIN_ENABLED` + `D7_SIMILARITY_CHECK_ENABLED` |
| 错误码区间 | 7110-7119 (PR-B) | 7120-7129 (PR-C) |
