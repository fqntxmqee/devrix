# Spec Delta — D7-Orchestration — Session Conclusion Completeness

**Change ID:** `devrix-session-conclusion-completeness`
**Target Spec:** `openspec/specs/d7-orchestration/spec.md`
**Target Version:** see `openspec/specs/d7-orchestration/CHANGELOG.md` (incremented on merge)
**Demand ID:** DM-20260630-011
**Created:** 2026-06-30
**Status:** S4_Implementation

---

## Why this delta

本次 Change 治本 4 个跨域 Bug 在 D7 编排层的基础设施层面。A-E 5 个 Bug 中 D7 域承担 3 个治本修复：LastTextQualityGate 分类、classifier_source 消除硬编码、BuildAdaptivePriorWithReport 把 ObsUncertainty 信号注入 Beta 先验。这些都是结构性 fix（非阈值调优，符合用户"不要调数值要重新设计"反馈）。

---

## ADDED Requirements

### D7-S2-A73: LastTextQualityGate — LLM 末次文本结构化质检

`SessionOrchestrator.finalizeLoop` SHALL apply `LastTextQualityGate` to the resolved summary BEFORE emitting `complete`. The classification result is emitted as a `D7_LastText_Quality_Gate` span and propagated as `meta["summary_quality"]` on the terminal event so D1 EmitComplete can decide whether to fallback.

#### Scenario: 4-way classification

- **Given** `resolvedSummary = "代码审查完成 ..."` (≥ 400 runes, no marker)
- **When** `ClassifyLastTextQuality` runs
- **Then** classification SHALL be `valid`

- **Given** `resolvedSummary = "结论基本合理"` (200 ≤ runes < 400, no marker)
- **When** `ClassifyLastTextQuality` runs
- **Then** classification SHALL be `thin`

- **Given** `resolvedSummary = ""` or length < 100 runes
- **When** `ClassifyLastTextQuality` runs
- **Then** classification SHALL be `too_short`

- **Given** `resolvedSummary = "<scope_contract> ..."` (≥ 100 runes, contains marker)
- **When** `ClassifyLastTextQuality` runs
- **Then** classification SHALL be `inconclusive`

#### Scenario: marker detection is case-insensitive

- **Given** `resolvedSummary` contains `<PLANNING>`, `<Planning>`, `<Scope_Contract>`, etc.
- **When** `ClassifyLastTextQuality` runs
- **Then** marker SHALL match regardless of casing

### D7-S2-A74: Orchestrator EmitMeta — summary_quality propagation

`SessionOrchestrator.emitComplete` SHALL set `meta["summary_quality"] = string(summaryQuality)` on the terminal `complete` event. The value MUST match the LastTextQualityGate output (never empty / zero value).

### D7-S2-A75: Eliminate hardcoded `learn.classifier_source="rule"`

The `processRequest` flow in `SessionOrchestrator.Run` SHALL derive `classifier_source` from the typed `intent.Source` field of `orchtypes.IntentClassification`, not from a hardcoded string. This enables the LLM classifier promotion (devrix-d7-llm-classifier-promotion) to differentiate source paths in observability without further orchestrator-side changes.

#### Scenario: rule classifier path produces source="rule"

- **Given** `intent.Source = orchtypes.SourceRule` (default zero value)
- **When** the classify span attributes are computed
- **Then** `orchestration.classify.source` SHALL equal `"rule"`
- **And** `learn.classifier_source` sessionSpan attribute SHALL equal `"rule"`

#### Scenario: default fallback when Source is unset

- **Given** an older caller passes `IntentClassification{}` without setting `Source`
- **When** the classify span attributes are computed
- **Then** `orchestration.classify.source` SHALL equal `"rule"` (default fallback)
- **Then** `learn.classifier_source` sessionSpan attribute SHALL equal `"rule"`

### D7-S5-A43: BuildAdaptivePriorWithReport — ObsUncertainty penalty injection

`orchtypes.BuildAdaptivePriorWithReport(rep, trackMode, report)` is the 3-arg overload of `prior.BuildAdaptivePrior` that CONSUMES `UncertaintyReport.Observations`. Sum of `ObsUncertainty + CatSystem + Strength ≥ 0.7` becomes a penalty that depresses the Beta prior mean (Alpha -= penalty, Beta += penalty, Alpha floored at 1).

#### Scenario: cold-start with nil report equals 2-arg baseline

- **Given** `report = nil`
- **When** `BuildAdaptivePriorWithReport` runs
- **Then** returned `AdaptivePrior.PriorBeta` SHALL equal `prior.BuildAdaptivePrior(rep, trackMode).PriorBeta`

#### Scenario: 2 high-strength signals (each 0.85) shift Beta prior down by 2

- **Given** `report.Observations` contains 2 `ObsUncertainty` × `CatSystem` × `Strength=0.85`
- **When** `BuildAdaptivePriorWithReport` runs
- **Then** the returned `AdaptivePrior.PriorBeta.Alpha` SHALL equal `baseline.Alpha - 2`
- **And** `Beta` SHALL equal `baseline.Beta + 2`
- **And** `Mean() < baseline.Mean()`

#### Scenario: extreme penalty floors Alpha at 1

- **Given** 20 stacked `ObsUncertainty` × `CatSystem` × `Strength=1.0`
- **When** `BuildAdaptivePriorWithReport` runs with developer track (default `Alpha=5`)
- **Then** `Alpha` SHALL be floored at 1 (not 5-20=-15)

---

## Test Point Mapping

| T Point             | Description                                    | File                                                                              |
| ------------------- | ---------------------------------------------- | --------------------------------------------------------------------------------- |
| D7-S2-T01 (AC1)     | LastTextQualityGate 4-way classification        | sessionorchestrator/last_text_quality_gate_test.go                                |
| D7-S2-T02 (AC1)     | Case-insensitive marker detection              | sessionorchestrator/last_text_quality_gate_test.go:TestLastTextQualityGate_MarkerCaseInsensitive |
| D7-S2-T03 (AC5)     | Default-zero Source fallback to "rule"         | sessionorchestrator/tracing_test.go:TestIntentClassifyAttrs_should_default_to_rule_when_source_unset |
| D7-S5-T04 (AC4)     | BuildAdaptivePriorWithReport cold-start/nil/penalty/floor | orchtypes/adaptive_prior_overload_test.go                              |

---

## Files Modified

- `internal/layers/orchestration/orchtypes/intent.go` — 新增 `ClassifierSource` 类型 + `IntentClassification.Source` 字段 + `WithSource` immutable setter
- `internal/layers/orchestration/orchtypes/uncertainty_report.go` — `Partition()` 新增 `ObsUncertainty + CatSystem + strength≥0.7` → `Anomalies` 投递路径
- `internal/layers/orchestration/decisionplanning/classifier.go` — `ClassifyWithPrior` 设置 `Source=SourceRule`；新增 `ClassifyWithReport` 3-arg overload
- `internal/layers/orchestration/sessionorchestrator/last_text_quality_gate.go` (NEW) — `SummaryQualityKind` 4 类 + `ClassifyLastTextQuality` + `EmitLastTextQuality`
- `internal/layers/orchestration/sessionorchestrator/turn_recovery.go` — `finalizeLoop` 调用 `EmitLastTextQuality`；`emitComplete` 增加 `summaryQuality` 参数并写入 `meta["summary_quality"]`
- `internal/layers/orchestration/sessionorchestrator/orchestrator.go` — 消除硬编码 `"rule"`，改读 `intent.Source.String()`
- `internal/layers/orchestration/sessionorchestrator/tracing.go` — `intentClassifyAttrs` 从 `intent.Source` 派生 source
- `internal/layers/orchestration/orchtypes/adaptive_prior_overload.go` (NEW) — 跨 cycle-aware 重定位至 orchtypes 包（避免与 anomaly_detector.go 闭环冲突）；实现 `BuildAdaptivePriorWithReport`
- `internal/layers/orchestration/executionflow/verify/anomaly.go` — 新增 `AnomalyKindTaskIncomplete` + `AnomalyKindEmptyConclusion` 常量
- `internal/layers/orchestration/executionflow/verify/anomaly_kind_incomplete.go` (NEW) — `DetectTaskIncomplete` + `DetectEmptyConclusion` + `nonTriggered` helper
- `internal/layers/orchestration/hardening/emitter.go` — 新增 `EmitMaterializeEmptyYield` + `EmitLastTextQualityGate` + `EmitEmitCompleteFallback`
- `internal/layers/observability/instrument/telemetry/names.go` — 新增 3 个 telemetry ops
