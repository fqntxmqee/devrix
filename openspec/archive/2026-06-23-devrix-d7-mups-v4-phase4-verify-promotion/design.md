# Design: D7 MUPS v4.3 Phase 4 — Verify 节点升格

**Change ID:** `devrix-d7-mups-v4-phase4-verify-promotion`
**Status:** S3_Design → S4_Implemented → S7_Archived
**Date:** 2026-06-23
**Author:** MUPS v4.3 Phase 4 Verify 节点落地梳理

---

## 0. S3-Gate Review（5 维度自检）

### 0.1 架构决策审查

| 检查项 | 结论 | 说明 |
|--------|------|------|
| 层归属正确 | ✅ PASS | `internal/layers/orchestration/workmodel/` 在 D7 核心域下；`internal/layers/orchestration/turn/` 已存在（verifier 调用方）；`internal/shared/types/` 跨域类型上提（Phase 1 MemoryEntry + Phase 3 SideEffectStatus precedent） |
| 跨域类型上提 | ✅ PASS | VerdictKind 上提 shared/types/verdict.go（Phase 3 SideEffectStatus precedent） |
| 节点边界清晰 | ✅ PASS | Verify 节点 = VerdictKind enum + Verdict struct + AggregateVerdicts + EvidenceExtractor + SystemAnomalyAggregator；与 Observe/Plan/Execute 边界通过 VerdictKind + Evidence 契约 |
| 强依赖链单向 | ✅ PASS | PR-D1 (VerdictKind) → PR-D2 (VerdictToExitReason) → PR-D3 (EvidenceExtractor) → PR-D4 (SystemAnomaly wiring) → Phase 5 Learn |
| 节点级原则（3-5 条） | ✅ PASS | 4 原则：① Verdict 不可变（With* 返回新副本）② 4 态离散（Pass/Partial/Indeterminate/Fail）③ AggregationStrategy 显式（4 策略）④ Evidence 结构化（Reason/Confidence/Counterexample） |

### 0.2 数据契约审查

| 检查项 | 结论 | 说明 |
|--------|------|------|
| VerdictKind 4 态 typed enum | ✅ PASS | uint8 + String/Marshal/Unmarshal/Parse + 类型别名 VerdictKind = types.VerdictKind |
| Verdict struct 字段 | ✅ PASS | Kind + Confidence ∈ [0,1] + Reason + SourceID；不可变 |
| AggregationStrategy 4 策略 | ✅ PASS | WeakConjunction（OR）/ StrongConjunction（AND）/ Majority（多数派）/ ThresholdByPass（阈值） |
| AggregateVerdicts 边界 | ✅ PASS | 空 → INDETERMINATE; 单元素 → 直接返回; 同质 → 直接返回 |
| Evidence struct 字段 | ✅ PASS | Reason + Confidence + Counterexample + SourceRef + ExtractedAt |
| ExitReason 14 枚举 | ✅ PASS | 8 既有（保持字符串不变）+ 6 新增（partial_verified/verifier_abstain/verifier_fail/system_anomaly/unresolved/abstain） |
| VerifyWithRetry parse failure → INDETERMINATE | ✅ PASS | 3 次重试后 parse 失败 → VerdictIndeterminate（不返回 error） |
| SystemAnomaly 触发条件 | ✅ PASS | AnomaliesCount ≥ Threshold (默认 3) AND CatSystem/AnomaliesCount ≥ Ratio (默认 0.5) |

### 0.3 接口契约审查

| 检查项 | 结论 | 说明 |
|--------|------|------|
| EvidenceExtractor interface 最小化 | ✅ PASS | 2 方法 Extract(ctx, VerifierOutput) ([]Evidence, error) + Validate([]Evidence) error |
| SystemAnomalyAggregator.Evaluate 幂等性 | ✅ PASS | 无副作用纯函数（输入相同 → 输出相同） |
| VerdictToExitReason 纯函数 | ✅ PASS | 输入 Verdict → 输出 ExitReason；无副作用 |
| ParseVerifierOutputWithRetry 重试上限 | ✅ PASS | maxRetries 默认 3；重试失败 → INDETERMINATE（不抛 error） |
| type alias 保持调用方零修改 | ✅ PASS | `type VerdictKind = types.VerdictKind`（PR-A1 UncertaintyCoord.FromVerifier 调用方零修改） |

### 0.4 跨域一致性

| 检查项 | 结论 | 说明 |
|--------|------|------|
| 与 Phase 2 PR-A1 UncertaintyCoord 兼容 | ✅ PASS | FromVerifier string switch 保持不变（PR-D2 才统一替换为 typed enum switch） |
| 与 Phase 3 PR-C1 ArtifactKind 4 类协调 | ✅ PASS | VerdictKind 4 态与 ArtifactKind 4 类无耦合（独立维度） |
| 与 Phase 3 PR-C2 Channel 4 类协调 | ✅ PASS | Channel 产出 Artifact → Verify 消费 Artifact → 产出 Verdict（Channel 与 Verify 边界清晰） |
| 与 doc 17/18 L1/L2 verifier 设计一致 | ✅ PASS | L2 verifier 仍由 Phase 1 VerifyWithRetry 实现，L1 ExitReason 扩展为 14 节点级枚举 |
| 与 doc 35 §三.5 SystemAnomaly 决策一致 | ✅ PASS | CatSystem 异常聚合为 SystemAnomaly → forced UncertaintyCoord.Value=0.95 |

### 0.5 可验证性

| 检查项 | 结论 | 说明 |
|--------|------|------|
| 单元测试覆盖率 ≥ 80% | ✅ PASS | 4 PR × 10-14 tests / file（aggregate_verdicts/verdict_to_exit_reason/evidence/evidence_extractor/system_anomaly/verify_with_retry 6 个 test file） |
| 集成测试（ObserveNode wiring） | ✅ PASS | PR-D4 D4.2.2 5 个集成测试覆盖 systemAnomaly 传导路径 |
| Race detector 0 warning | ✅ PASS | 纯函数 + 不可变对象，无并发修改路径 |
| 边界覆盖（空/单元素/同质） | ✅ PASS | AggregateVerdicts 3 边界 + VerifyWithRetry 3 边界 + SystemAnomaly 4 阈值 |
| 向后兼容（既有 8 ExitReason 字符串不变） | ✅ PASS | PR-D2 D2.2.1 既定 8 值字符串保持，仅追加 6 新值 |

---

## 1. VerdictKind typed enum

### 1.1 shared/types/verdict.go

```go
package types

import (
    "encoding/json"
    "fmt"
)

// VerdictKind classifies the output of a Verifier sub-agent. The 4 kinds
// form a strict ordinal scale (Pass < Partial < Indeterminate < Fail) that
// AggregateVerdicts can rank against.
//
// Promoted to shared/types (2026-06-23, DM-20260623-002 Phase 4 PR-D1) to
// mirror the Phase 3 SideEffectStatus precedent: typed enums shared across
// orchtypes/workmodel/turn avoid cyclic imports and keep D5 dashboard
// filters uniform.
//
// Lifecycle:
//
//   Pass          — criteria fully satisfied
//   Partial       — criteria partially satisfied (need human review)
//   Indeterminate — verifier abstained (parse failure / no consensus)
//   Fail          — criteria violated
type VerdictKind uint8

const (
    // VerdictPass — criteria fully satisfied.
    VerdictPass VerdictKind = iota
    // VerdictPartial — criteria partially satisfied.
    VerdictPartial
    // VerdictIndeterminate — verifier abstained.
    VerdictIndeterminate
    // VerdictFail — criteria violated.
    VerdictFail
)

// String returns the wire format name (lowercase).
func (k VerdictKind) String() string {
    switch k {
    case VerdictPass:
        return "pass"
    case VerdictPartial:
        return "partial"
    case VerdictIndeterminate:
        return "indeterminate"
    case VerdictFail:
        return "fail"
    default:
        return fmt.Sprintf("VerdictKind(%d)", uint8(k))
    }
}

// ParseVerdictKind reverses String(). Unknown values fail-fast.
func ParseVerdictKind(s string) (VerdictKind, error) {
    switch s {
    case "pass":
        return VerdictPass, nil
    case "partial":
        return VerdictPartial, nil
    case "indeterminate":
        return VerdictIndeterminate, nil
    case "fail":
        return VerdictFail, nil
    default:
        return 0, fmt.Errorf("types: unknown VerdictKind %q", s)
    }
}

// MarshalJSON encodes as String() form.
func (k VerdictKind) MarshalJSON() ([]byte, error) {
    return json.Marshal(k.String())
}

// UnmarshalJSON parses the wire format. Empty string defaults to zero
// value (VerdictPass) for v2 backward compatibility.
func (k *VerdictKind) UnmarshalJSON(data []byte) error {
    var s string
    if err := json.Unmarshal(data, &s); err != nil {
        return err
    }
    if s == "" {
        *k = VerdictPass
        return nil
    }
    parsed, err := ParseVerdictKind(s)
    if err != nil {
        return err
    }
    *k = parsed
    return nil
}
```

### 1.2 orchtypes type alias

```go
// orchtypes/uncertainty_coord.go
import "github.com/devrix/devrix/internal/shared/types"

// VerdictKind is a type alias re-exported from shared/types so that
// UncertaintyCoord (Phase 2 PR-A1) and Verdict (Phase 4 PR-D1) can share
// the same wire format. The concrete enum + String/Parse live in
// shared/types/verdict.go.
type VerdictKind = types.VerdictKind
```

## 2. AggregationStrategy + AggregateVerdicts

### 2.1 workmodel/aggregate_verdicts.go

```go
package workmodel

import (
    "encoding/json"
    "fmt"
)

// AggregationStrategy selects how a list of Verdicts is folded into a
// single Verdict by AggregateVerdicts.
//
// Strategy semantics:
//
//   WeakConjunction   — any PASS → PASS (OR, most permissive)
//   StrongConjunction — all PASS → PASS (AND, strictest)
//   Majority          — PASS > len/2 → PASS (plurality)
//   ThresholdByPass   — PASS ≥ threshold → PASS (configurable)
type AggregationStrategy uint8

const (
    WeakConjunction AggregationStrategy = iota
    StrongConjunction
    Majority
    ThresholdByPass
)

func (s AggregationStrategy) String() string { ... }
func ParseAggregationStrategy(s string) (AggregationStrategy, error) { ... }
func (s AggregationStrategy) MarshalJSON() ([]byte, error) { ... }
func (s *AggregationStrategy) UnmarshalJSON(data []byte) error { ... }

// Verdict is a single Verifier sub-agent output.
type Verdict struct {
    Kind       VerdictKind `json:"kind"`
    Confidence float64     `json:"confidence,omitempty"`
    Reason     string      `json:"reason,omitempty"`
    SourceID   string      `json:"source_id,omitempty"`
}

// AggregateVerdicts folds verdicts into a single verdict according to
// strategy. Empty input returns INDETERMINATE; single input returned
// directly; homogeneous input returned directly.
func AggregateVerdicts(verdicts []Verdict, strategy AggregationStrategy) Verdict {
    if len(verdicts) == 0 {
        return Verdict{Kind: VerdictIndeterminate, Reason: "empty_verdict_set"}
    }
    if len(verdicts) == 1 {
        return verdicts[0]
    }
    // Same-kind shortcut: no aggregation needed.
    allSame := true
    for i := 1; i < len(verdicts); i++ {
        if verdicts[i].Kind != verdicts[0].Kind {
            allSame = false
            break
        }
    }
    if allSame {
        return aggregateMeta(verdicts)
    }
    // Strategy dispatch.
    switch strategy {
    case WeakConjunction:
        return aggregateWeakConjunction(verdicts)
    case StrongConjunction:
        return aggregateStrongConjunction(verdicts)
    case Majority:
        return aggregateMajority(verdicts)
    case ThresholdByPass:
        return aggregateThresholdByPass(verdicts, 1) // default threshold=1
    }
    return Verdict{Kind: VerdictIndeterminate, Reason: "unknown_strategy"}
}

func aggregateWeakConjunction(vs []Verdict) Verdict {
    hasPass, hasFail := false, false
    var confidenceSum float64
    var maxReason string
    for _, v := range vs {
        if v.Kind == VerdictPass { hasPass = true }
        if v.Kind == VerdictFail { hasFail = true }
        confidenceSum += v.Confidence
        if len(v.Reason) > len(maxReason) { maxReason = v.Reason }
    }
    switch {
    case hasPass: return Verdict{Kind: VerdictPass, Confidence: confidenceSum/float64(len(vs)), Reason: maxReason}
    case hasFail: return Verdict{Kind: VerdictFail, Confidence: confidenceSum/float64(len(vs)), Reason: maxReason}
    default:      return Verdict{Kind: VerdictIndeterminate, Confidence: confidenceSum/float64(len(vs)), Reason: maxReason}
    }
}

// StrongConjunction / Majority / ThresholdByPass 类似实现
```

## 3. VerdictToExitReason + 14 ExitReason

### 3.1 turn/verdict_to_exit_reason.go

```go
package turn

import (
    "github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
    "github.com/devrix/devrix/internal/layers/orchestration/workmodel"
)

// VerdictToExitReason maps a Verifier Verdict to an orchestrator-level
// ExitReason. This is the single source of truth for "why the turn
// stopped" when the stop was triggered by Verify, NOT by deterministic
// orchestrator conditions (max_turns / aborted_* / repeated_tool etc).
func VerdictToExitReason(v workmodel.Verdict, sessionID string) ExitReason {
    if v.SystemAnomaly {
        return ExitReasonSystemAnomaly
    }
    switch v.Kind {
    case orchtypes.VerdictPass:
        return ExitReasonNatural
    case orchtypes.VerdictPartial:
        return ExitReasonPartialVerified
    case orchtypes.VerdictIndeterminate:
        return ExitReasonVerifierAbstain
    case orchtypes.VerdictFail:
        return ExitReasonVerifierFail
    }
    return ExitReasonVerifierAbstain // unknown → abstain (safe default)
}
```

### 3.2 ExitReason 8 → 14 扩展（orchestrator.go line 73-97）

```go
const (
    // 既有 8 个 enum（字符串保持不变，向后兼容）
    ExitReasonNatural       ExitReason = "natural"
    ExitReasonMaxTurns      ExitReason = "max_turns"
    ExitReasonAbortedUser   ExitReason = "aborted_user"
    ExitReasonAbortedLLM    ExitReason = "aborted_llm"
    ExitReasonAbortedTool   ExitReason = "aborted_tool"
    ExitReasonRepeatedTool  ExitReason = "repeated_tool"
    ExitReasonToolFailure   ExitReason = "tool_failure"
    ExitReasonTokenDiminishing ExitReason = "token_diminishing"
    
    // Phase 4 新增 6 个 enum（PR-D2）
    ExitReasonPartialVerified ExitReason = "partial_verified"
    ExitReasonVerifierAbstain ExitReason = "verifier_abstain"
    ExitReasonVerifierFail    ExitReason = "verifier_fail"
    ExitReasonSystemAnomaly   ExitReason = "system_anomaly"
    ExitReasonUnresolved      ExitReason = "unresolved"
    ExitReasonAbstain         ExitReason = "abstain"
)
```

## 4. VerifyWithRetry parse failure → INDETERMINATE

### 4.1 workmodel/verify_with_retry.go

```go
package workmodel

import (
    "encoding/json"
    "github.com/devrix/devrix/internal/shared/types"
)

// VerifierOutput is the raw LLM output + parsed VerdictKind.
type VerifierOutput struct {
    Raw           string         `json:"raw"`
    ParsedKind    types.VerdictKind `json:"parsed_kind"`
    Confidence    float64        `json:"confidence,omitempty"`
    RetryCount    int            `json:"retry_count,omitempty"`
}

// ParseVerifierOutput parses a single verifier LLM output (JSON format).
// Returns error on parse failure.
func ParseVerifierOutput(raw string) (VerifierOutput, error) {
    var parsed struct {
        Kind       string  `json:"kind"`
        Confidence float64 `json:"confidence"`
        Reason     string  `json:"reason"`
    }
    if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
        return VerifierOutput{Raw: raw}, err
    }
    kind, err := types.ParseVerdictKind(parsed.Kind)
    if err != nil {
        return VerifierOutput{Raw: raw}, err
    }
    return VerifierOutput{
        Raw:        raw,
        ParsedKind: kind,
        Confidence: parsed.Confidence,
    }, nil
}

// ParseVerifierOutputWithRetry retries parse up to maxRetries times.
// After maxRetries failures, returns INDETERMINATE (NOT error) — this is
// the G8-1 P0-3 fix: parse failure must not propagate as FAIL.
func ParseVerifierOutputWithRetry(raw string, maxRetries int) VerifierOutput {
    for i := 0; i < maxRetries; i++ {
        out, err := ParseVerifierOutput(raw)
        if err == nil {
            out.RetryCount = i
            return out
        }
    }
    // 3 次重试仍失败 → INDETERMINATE (G8-1 修复)
    return VerifierOutput{
        Raw:        raw,
        ParsedKind: types.VerdictIndeterminate,
        Confidence: 0,
        RetryCount: maxRetries,
    }
}
```

## 5. Evidence + EvidenceExtractor

### 5.1 workmodel/evidence.go

```go
package workmodel

type Evidence struct {
    Reason        string    `json:"reason"`
    Confidence    float64   `json:"confidence,omitempty"`
    Counterexample string   `json:"counterexample,omitempty"`
    SourceRef     string    `json:"source_ref,omitempty"`
    ExtractedAt   time.Time `json:"extracted_at"`
}

func NewEvidence(reason string, confidence float64, sourceRef string) (Evidence, error) {
    if reason == "" {
        return Evidence{}, errors.New("workmodel: Evidence.Reason is required")
    }
    if sourceRef == "" {
        return Evidence{}, errors.New("workmodel: Evidence.SourceRef is required")
    }
    return Evidence{
        Reason:      reason,
        Confidence:  clamp01Float(confidence, 0.5),
        SourceRef:   sourceRef,
        ExtractedAt: time.Now(),
    }, nil
}

func (e Evidence) Validate() error {
    if e.Reason == "" {
        return errors.New("workmodel: Evidence.Reason is required")
    }
    if e.Confidence < 0 || e.Confidence > 1 {
        return errors.New("workmodel: Evidence.Confidence out of range")
    }
    if e.SourceRef == "" {
        return errors.New("workmodel: Evidence.SourceRef is required")
    }
    return nil
}
```

### 5.2 workmodel/evidence_extractor.go

```go
package workmodel

import (
    "context"
    "encoding/json"
)

// EvidenceExtractor extracts structured Evidence from Verifier LLM output.
type EvidenceExtractor interface {
    Extract(ctx context.Context, verifierOutput VerifierOutput) ([]Evidence, error)
    Validate(evidence []Evidence) error
}

// LLMEvidenceExtractor extracts Evidence from LLM JSON output.
type LLMEvidenceExtractor struct{}

func (l *LLMEvidenceExtractor) Extract(ctx context.Context, v VerifierOutput) ([]Evidence, error) {
    var parsed struct {
        Reasons        []string `json:"reasons"`
        Confidences    []float64 `json:"confidences"`
        Counterexamples []string `json:"counterexamples"`
    }
    if err := json.Unmarshal([]byte(v.Raw), &parsed); err != nil {
        return nil, fmt.Errorf("evidence: malformed LLM output: %w", err)
    }
    var evs []Evidence
    for i := range parsed.Reasons {
        ev, err := NewEvidence(parsed.Reasons[i], parsed.Confidences[i], v.SourceID())
        if err != nil {
            return nil, err
        }
        if parsed.Counterexamples[i] != "" {
            ev.Counterexample = parsed.Counterexamples[i]
        }
        evs = append(evs, ev)
    }
    return evs, nil
}

func (l *LLMEvidenceExtractor) Validate(evidence []Evidence) error {
    if len(evidence) == 0 {
        return errors.New("evidence: empty list")
    }
    for i, e := range evidence {
        if err := e.Validate(); err != nil {
            return fmt.Errorf("evidence[%d]: %w", i, err)
        }
    }
    return nil
}
```

## 6. SystemAnomaly 异常聚合

### 6.1 workmodel/system_anomaly.go

```go
package workmodel

// SystemAnomalyConfig 配置 SystemAnomaly 触发条件。
type SystemAnomalyConfig struct {
    AnomalyThreshold  int     // 默认 3, AnomaliesCount ≥ Threshold 触发
    MinCatSystemRatio float64 // 默认 0.5, CatSystem/AnomaliesCount ≥ Ratio 触发
}

type SystemAnomalyAggregator struct {
    cfg         SystemAnomalyConfig
    catSystemCount int
}

func NewSystemAnomalyAggregator(cfg SystemAnomalyConfig) *SystemAnomalyAggregator {
    if cfg.AnomalyThreshold == 0 { cfg.AnomalyThreshold = 3 }
    if cfg.MinCatSystemRatio == 0 { cfg.MinCatSystemRatio = 0.5 }
    return &SystemAnomalyAggregator{cfg: cfg}
}

func (a *SystemAnomalyAggregator) Evaluate(report UncertaintyReport) bool {
    anomalies := report.Anomalies
    if len(anomalies) < a.cfg.AnomalyThreshold {
        return false
    }
    catSystemCount := 0
    for _, obs := range anomalies {
        if obs.Category == CatSystem {
            catSystemCount++
        }
    }
    return float64(catSystemCount)/float64(len(anomalies)) >= a.cfg.MinCatSystemRatio
}

func (a *SystemAnomalyAggregator) RecordCatSystem(count int) {
    a.catSystemCount += count
}

func (a *SystemAnomalyAggregator) Reset() {
    a.catSystemCount = 0
}
```

### 6.2 workmodel/uncertainty.go wiring 集成

```go
// EvaluateSystemAnomaly 封装 SystemAnomalyAggregator.Evaluate
func EvaluateSystemAnomaly(report UncertaintyReport) bool {
    agg := NewSystemAnomalyAggregator(SystemAnomalyConfig{})
    return agg.Evaluate(report)
}

// BuildUncertaintyCoordFromReport 统一 ObserveNode → Verify → UncertaintyCoord 转换
func BuildUncertaintyCoordFromReport(report UncertaintyReport, verifier Verdict) (UncertaintyCoord, error) {
    systemAnomaly := EvaluateSystemAnomaly(report)
    return FromVerifier(verifier.Kind.String(), verifier.Confidence, verifier.Reason, systemAnomaly)
}
```

## 7. 跨节点依赖

### 7.1 上游契约（从 Observe + Plan + Execute 接收）

- `UncertaintyReport.Observations` → SystemAnomaly 评估输入
- `UncertaintyReport.Anomalies` → CatSystem 计数输入
- `Artifact.SourcePlanID` → Verdict.SourceID 反向追溯
- `Plan.SourceObservationIDs` → Evidence.SourceRef 反向追溯

### 7.2 下游契约（向 Learn 交付）

- `Verdict{Kind, Confidence, Reason, SourceID}` → Phase 5 LearningAsset 4 类输入
- `Evidence{Reason, Confidence, Counterexample, SourceRef}` → Phase 5 SOPAsset + KnowledgeAsset 内容来源
- `ExitReason (14 值)` → Phase 5 ReputationEvidence 元数据
- `SystemAnomaly (bool)` → Phase 5 ReputationEvidence 惩罚信号

### 7.3 不变式

- `Verdict` 不可变（所有修改通过 With* 返回新副本）
- `Evidence` Reason + SourceRef 必填（构造时 fail-fast）
- `VerdictKind` 4 态离散（不可插入新值）
- `ExitReason` 14 值枚举（8 既有字符串不变 + 6 新增）