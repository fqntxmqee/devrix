# Spec Delta: D7 MUPS v4.3 Phase 4 — Verify 节点升格

**Change ID:** `devrix-d7-mups-v4-phase4-verify-promotion`
**Target:** `openspec/specs/d7-orchestration/spec.md` v4.3.0 → v4.4.0
**Created:** 2026-06-23

---

## ADDED Requirements（12 ADDED）

### D7-S10-A32: VerdictKind typed enum + AggregateVerdicts（G3-1）

#### Requirement: VerdictKind 4 态 typed enum

The D7 Verify 节点 MUST expose a typed `VerdictKind` enum in `internal/shared/types/verdict.go` with 4 离散态（`VerdictPass` / `VerdictPartial` / `VerdictIndeterminate` / `VerdictFail`），实现 `String()` / `ParseVerdictKind()` / `MarshalJSON` / `UnmarshalJSON`，且 `UnmarshalJSON` 对空字符串保持零值（`VerdictPass`）向后兼容。

`orchtypes/uncertainty_coord.go` MUST 通过 type alias `type VerdictKind = types.VerdictKind` 重新导出，避免 import cycle。

**Rationale**: 替换 `UncertaintyCoord.FromVerifier` 内联 `case "pass"/"partial"/"indeterminate"/"fail"` 字符串 switch 为 typed enum switch；与 Phase 3 `SideEffectStatus` precedent 一致。

#### Requirement: AggregationStrategy 4 策略 + AggregateVerdicts 函数

`internal/layers/orchestration/workmodel/aggregate_verdicts.go` MUST 实现：

- `AggregationStrategy` 4 策略枚举：
  - `WeakConjunction`（任一 PASS 即 PASS，OR 语义）
  - `StrongConjunction`（全 PASS 才 PASS，AND 语义）
  - `Majority`（PASS > len/2 即 PASS，多数派）
  - `ThresholdByPass`（PASS ≥ 阈值即 PASS，默认阈值=1）
- `AggregateVerdicts(verdicts []Verdict, strategy AggregationStrategy) Verdict` 函数
- 边界处理：空切片 → `Verdict{Kind: VerdictIndeterminate, Reason: "empty_verdict_set"}`；单元素 → 直接返回；所有 Verdict Kind 相同 → 直接返回
- Confidence 聚合：算术平均；Reason 聚合：取最长（最具体）

### D7-S10-A33: VerdictToExitReason + 14 ExitReason（G8-1 P0-3 修复）

#### Requirement: VerdictToExitReason 4 Verdict → 14 ExitReason 映射

`internal/layers/orchestration/turn/verdict_to_exit_reason.go` MUST 实现 `VerdictToExitReason(v workmodel.Verdict, sessionID string) ExitReason` 函数，4 VerdictKind 映射：

- `VerdictPass` → `ExitReasonNatural`
- `VerdictPartial` → `ExitReasonPartialVerified`（新增）
- `VerdictIndeterminate` → `ExitReasonVerifierAbstain`（新增）
- `VerdictFail` → `ExitReasonVerifierFail`（新增）

`SystemAnomaly=true` MUST 覆盖上述映射，返回 `ExitReasonSystemAnomaly`（新增）。

#### Requirement: ExitReason 8 → 14 扩展

`internal/layers/orchestration/turn/orchestrator.go` MUST 保留既有 8 个 ExitReason enum 字符串不变（`natural` / `max_turns` / `aborted_user` / `aborted_llm` / `aborted_tool` / `repeated_tool` / `tool_failure` / `token_diminishing`），并追加 6 个新 enum：

- `ExitReasonPartialVerified = "partial_verified"`
- `ExitReasonVerifierAbstain = "verifier_abstain"`
- `ExitReasonVerifierFail = "verifier_fail"`
- `ExitReasonSystemAnomaly = "system_anomaly"`
- `ExitReasonUnresolved = "unresolved"`
- `ExitReasonAbstain = "abstain"`

#### Requirement: VerifyWithRetry parse failure → INDETERMINATE（G8-1 P0-3 修复）

`internal/layers/orchestration/workmodel/verify_with_retry.go` MUST 实现 `ParseVerifierOutputWithRetry(raw string, maxRetries int) VerifierOutput` 函数：

- 默认 `maxRetries = 3`
- 重试 3 次后 parse 仍失败 → 返回 `VerifierOutput{ParsedKind: VerdictIndeterminate, Confidence: 0, RetryCount: 3}`，**不返回 error**
- 重试中任一次成功 → 立即返回成功结果

**Rationale**: G8-1 P0-3 bug 修复。原实现 parser 失败直接 fail-fast 并触发 `ErrUncertaintyReportInvalid` 拒绝 retry，导致 verifier 暂时性网络抖动被误判为 FAIL。

### D7-S10-A34: Evidence struct + EvidenceExtractor

#### Requirement: Evidence struct 5 字段

`internal/layers/orchestration/workmodel/evidence.go` MUST 定义 `Evidence` struct：

- `Reason string` — 判定依据，自然语言描述（必填）
- `Confidence float64` — 置信度 ∈ [0,1]（必填，clamp 到 [0,1]）
- `Counterexample string` — 反例（可选）
- `SourceRef string` — 来源 Verifier ID / Plan ID / Observation ID（必填）
- `ExtractedAt time.Time` — 提取时间

#### Requirement: EvidenceExtractor interface 2 方法

`internal/layers/orchestration/workmodel/evidence_extractor.go` MUST 实现：

- `EvidenceExtractor` interface：
  - `Extract(ctx context.Context, verifierOutput VerifierOutput) ([]Evidence, error)` — 从 Verifier LLM 输出提取 Evidence 列表
  - `Validate(evidence []Evidence) error` — 验证 Evidence 列表合法性
- `LLMEvidenceExtractor` 实现（基于 JSON 解析 + 正则 fallback）
- `StubEvidenceExtractor` 实现（返回固定 Evidence，便于测试）

### D7-S10-A35: SystemAnomaly 异常聚合 + ObserveNode wiring

#### Requirement: SystemAnomalyAggregator 阈值触发

`internal/layers/orchestration/workmodel/system_anomaly.go` MUST 实现：

- `SystemAnomalyConfig` 配置：
  - `AnomalyThreshold int`（默认 3）
  - `MinCatSystemRatio float64`（默认 0.5）
- `SystemAnomalyAggregator.Evaluate(report UncertaintyReport) bool` 函数
- 触发条件：`len(report.Anomalies) ≥ AnomalyThreshold AND CatSystem/AnomaliesCount ≥ MinCatSystemRatio`

#### Requirement: ObserveNode wiring SystemAnomaly 传 FromVerifier

`internal/layers/orchestration/workmodel/uncertainty.go` MUST 实现：

- `EvaluateSystemAnomaly(report UncertaintyReport) bool` — 封装 `SystemAnomalyAggregator.Evaluate`
- `BuildUncertaintyCoordFromReport(report UncertaintyReport, verifier Verdict) (UncertaintyCoord, error)` — 统一 ObserveNode → Verify → UncertaintyCoord 转换

`SystemAnomaly=true` MUST 强制 `UncertaintyCoord.Value = 0.95`（Phase 2 PR-A1 FromVerifier 已实现），与 doc 35 §三.5 SystemAnomaly 决策保持一致。

---

## MODIFIED Requirements

无 MODIFIED Requirements（本 PR 不修改既有 Requirement，仅新增 12 ADDED Requirement）。

## REMOVED Requirements

无 REMOVED Requirements。

## 关联

- 前置：Phase 2 PR-A1（UncertaintyCoord + FromVerifier） + Phase 3 PR-C1（ArtifactKind 4 类 + SideEffectStatus 5 态）
- 后续：Phase 5 Learn 节点（LearningAsset 4 类 + ReputationEvidence 强依赖本 PR 的 Verdict + Evidence 数据契约）
- 设计稿：doc 45 (D7 Verify 节点详细技术方案) + doc 17 (L2 verifier) + doc 18 (L1 ExitReason)