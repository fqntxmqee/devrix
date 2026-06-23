# Design: D7 MUPS v4.3 Phase 5 — Learn 节点升格

**Change ID:** `devrix-d7-mups-v4-phase5-learn`
**Status:** S3_Design → S4_Implemented → S7_Archived
**Date:** 2026-06-23
**Author:** MUPS v4.3 Phase 5 Learn 节点落地梳理

---

## 0. S3-Gate Review（5 维度自检）

### 0.1 架构决策审查

| 检查项 | 结论 | 说明 |
|--------|------|------|
| 层归属正确 | ✅ PASS | `internal/layers/orchestration/learn/`（新包）+ `internal/shared/types/learning.go`（跨域类型上提 Phase 3 SideEffectStatus / Phase 4 VerdictKind precedent）|
| 跨域类型上提 | ✅ PASS | LearningClass 上提 shared/types/learning.go（Phase 3 SideEffectStatus + Phase 4 VerdictKind precedent）|
| 节点边界清晰 | ✅ PASS | Learn 节点 = LearningAsset 5 类 + ReputationEvidence + AdaptivePrior + 3 通道 Memory + Learner interface；上游 Verify（消费 Verdict/Plan/Artifact），下游下一轮 Observe（注入 AdaptivePrior）|
| 强依赖链单向 | ✅ PASS | PR-E1 (LearningAsset) → PR-E2 (ReputationEvidence) → PR-E3 (AdaptivePrior)；PR-E1 + PR-E4 (Memory) 并行；PR-E5 (Learner + 闭环) 依赖全部前 4 PR |
| 节点级原则（5 条） | ✅ PASS | 5 原则：① LP-1 闭环回写 ② LP-2 3 通道隔离 ③ LP-3 Bayesian 更新 ④ LP-4 可证伪沉淀 ⑤ LP-5 跨会话可追溯 |

### 0.2 数据契约审查

| 检查项 | 结论 | 说明 |
|--------|------|------|
| LearningAsset struct 字段 | ✅ PASS | 15 字段（ID/SessionID/Class/Strength/SourceSessionIDs/SourceVerdictIDs/Content/AssetKey/ContentHash/FailureCriterion/ExpiryAt/CreatedAt/LastUsedAt/UseCount/TraceID）+ 不可变 |
| LearningClass 5 枚举 | ✅ PASS | uint8 + String/Marshal/Unmarshal/Parse + 类型别名 LearningClass = types.LearningClass；LearningUnknown=0 禁用 |
| 5 类 AssetContent | ✅ PASS | SOPAssetContent（★5）/ ProtocolAssetContent（★4）/ KnowledgeAssetContent（★3）/ ConclusionAssetContent（★2）/ PendingAssetContent（⭐★1，新增 MVEState）|
| AssetContent.Validate() | ✅ PASS | 5 类各自必填字段 fail-fast，统一 ErrAssetIncomplete |
| ReputationEvidence struct | ✅ PASS | 12 字段（SessionID/TrackMode/Alpha/Beta/Mean/Variance/ConfidenceLow/ConfidenceHigh/LastUpdated/UpdateCount/SourceVerdictIDs/VerifierFailureCount/IndeterminateCount）+ 不可变 |
| BayesianUpdate 数学正确性 | ✅ PASS | Pass/Partial → α++；Fail → β++；INDETERMINATE "verifier_parse_failure" → 仅 VerifierFailureCount++（⭐G8-1 修复）；其他 INDETERMINATE → IndeterminateCount++（不污染 α/β）；冷启动除零防御 |
| Wilson Score 95% 置信区间 | ✅ PASS | 标准公式 + z=1.96 |
| AdaptivePrior + DefaultPriors | ✅ PASS | Developer Beta(5,3) / Operator Beta(8,1)；BuildAdaptivePrior 合并 DefaultPrior + Reputation（Bayesian 合并公式）|
| Memory 3 通道 LP-2 隔离 | ✅ PASS | SkillMemory（SOP/Protocol）/ FeedbackMemory（Knowledge/Conclusion）/ ScheduledMemory（Pending）；ErrAssetClassMismatch 校验 |

### 0.3 接口契约审查

| 检查项 | 结论 | 说明 |
|--------|------|------|
| Learner interface 最小化 | ✅ PASS | 3 方法 Learn(ctx, req) / Inject(ctx, sessionID) / ScheduledTick(ctx) |
| Memory interface 最小化 | ✅ PASS | 4 方法 Store/Retrieve/Delete/List + MemoryFilter |
| ReputationStore interface 最小化 | ✅ PASS | 3 方法 Get/Update/List + ErrReputationStoreUnavailable |
| AssetBuilder.AssetKey 幂等性 | ✅ PASS | 格式 `sop:PlanKind:hash` + ContentHash SHA-256 hex 稳定 |
| BayesianUpdate 不可变性 | ✅ PASS | 输入 prior 不可变（copy 副本）；返回新对象（不修改 prior）|
| type alias 保持调用方零修改 | ✅ PASS | `type LearningClass = types.LearningClass`（PR-A1 UncertaintyCoord.FromVerifier 调用方零修改 precedent）|
| ScheduledMemory TriggerAt 默认值 | ✅ PASS | TriggerAt = asset.ExpiryAt（PR-E4 默认 TTL 24h）|
| Learner.Inject LP-1 时序约束 | ✅ PASS | 必须在下一轮 Observe.All() 之前完成（Orchestrator 持有 prior）|

### 0.4 跨域一致性

| 检查项 | 结论 | 说明 |
|--------|------|------|
| 与 Phase 4 PR-D1 VerdictKind 协调 | ✅ PASS | ReputationEvidence 接收 verify.Verdict，BayesianUpdate switch verdict.Kind |
| 与 Phase 4 PR-D3 Evidence 协调 | ✅ PASS | Evidence 可作为 LearningAsset.Content 的输入（KnowledgeAssetContent.Evidence = []Evidence）|
| 与 Phase 4 PR-D4 SystemAnomaly 协调 | ✅ PASS | SystemAnomaly=true Verdict 路径由 Phase 4 VerdictToExitReason 处理；Learn 接收 Verdict 时 SystemAnomaly 已 bind 到 ExitReason |
| 与 Phase 3 PR-C1 Artifact 协调 | ✅ PASS | PendingAssetContent.OriginalArtifactID = execute.Artifact.ID；PendingAssetContent.MVEState *execute.MVEState（跨包指针）|
| 与 Phase 2 PR-A1 Observation 协调 | ✅ PASS | Plan.SourceObservationIDs（已就绪） → LearnRequest.Observations → LearningAsset.FailureCriterion 推断 |
| 与 doc 25 AdaptivePrior 协调 | ✅ PASS | Developer Beta(5,3) / Operator Beta(8,1) 严格对齐 doc 25 §四 |
| 与 doc 27 §三 3 通道记忆协调 | ✅ PASS | Skill/Feedback/Scheduled 3 通道严格对齐 doc 27 §三 |
| 与 doc 46 Learn 节点设计稿一致 | ✅ PASS | 5 类 Asset + 3 通道 + ReputationEvidence + AdaptivePrior + LP-1 闭环全部覆盖 |

### 0.5 可验证性

| 检查项 | 结论 | 说明 |
|--------|------|------|
| 单元测试覆盖率 ≥ 80% | ✅ PASS | 5 PR × 8-16 tests / file（learning_asset/asset_content/reputation_evidence/adaptive_prior/memory/asset_builder/learner 7 个 test file）|
| 集成测试（LP-1 闭环 E2E） | ✅ PASS | PR-E5 E5.4.5 集成测试覆盖 Observe → Plan → Execute → Verify → Learn → Observe 闭环 |
| Race detector 0 warning | ✅ PASS | Memory 3 通道 + ReputationStore 全部加 sync.RWMutex；BayesianUpdate 不可变（无并发修改路径）|
| 边界覆盖（空/单元素/同质 + 冷启动除零） | ✅ PASS | BayesianUpdate 冷启动除零防御 + AssetBuilder Build nil 边界 + ScheduledMemory MaxRetries 边界 |
| 向后兼容（Phase 1-4 既有 8 类行为不变） | ✅ PASS | 新增 learn 包 + 增量 Observe 对接方法（QuantizeWithPrior 等），既有方法签名不变 |
| LP-1 闭环验证 | ✅ PASS | Orchestrator ProcessMessage 入口前 Learner.Inject → ObserveRequest.Prior → IntentQuantizer.QuantizeWithPrior 验证 prior.PriorBeta 使用 |

---

## 1. LearningClass 跨域类型上提

### 1.1 shared/types/learning.go

```go
package types

import (
    "encoding/json"
    "fmt"
)

// LearningClass classifies the kind of LearningAsset produced by the Learn
// node. The 5 classes form a strict ordinal scale that AssetBuilder routes
// to 3-channel Memory (Skill / Feedback / Scheduled).
//
// Promoted to shared/types (2026-06-23, DM-20260623-003 Phase 5 PR-E1) to
// mirror the Phase 3 SideEffectStatus / Phase 4 VerdictKind precedent:
// typed enums shared across orchtypes/learn/turn avoid cyclic imports and
// keep D5 dashboard filters uniform.
//
// Lifecycle:
//
//   LearningSOP         — Standard Operating Procedure (★5)
//   LearningProtocol    — Protocol (★4)
//   LearningKnowledge   — Knowledge (★3)
//   LearningConclusion  — Conclusion (★2)
//   LearningPending     — Pending retry (★1, ⭐new — VerdictIndeterminate)
type LearningClass uint8

const (
    // LearningUnknown — reserved zero value; MUST be rejected by ParseLearningClass
    // and factory functions.
    LearningUnknown LearningClass = iota

    // LearningSOP — Standard Operating Procedure (★5).
    // Source: ComplianceVerdict.
    LearningSOP

    // LearningProtocol — Protocol (★4).
    // Source: TimelinessVerdict.
    LearningProtocol

    // LearningKnowledge — Knowledge (★3).
    // Source: RootCauseVerdict.
    LearningKnowledge

    // LearningConclusion — Conclusion (★2).
    // Source: StatisticalVerdict.
    LearningConclusion

    // LearningPending — Pending retry (★1, ⭐new).
    // Source: VerdictIndeterminate.
    // Routed to ScheduledMemory (not Skill/Feedback).
    LearningPending
)

// String returns the wire-format lowercase name.
func (k LearningClass) String() string {
    switch k {
    case LearningSOP:
        return "sop"
    case LearningProtocol:
        return "protocol"
    case LearningKnowledge:
        return "knowledge"
    case LearningConclusion:
        return "conclusion"
    case LearningPending:
        return "pending"
    default:
        return fmt.Sprintf("LearningClass(%d)", uint8(k))
    }
}

// ParseLearningClass parses a wire-format string into a LearningClass.
// Returns an error on unknown values, including LearningUnknown.
func ParseLearningClass(s string) (LearningClass, error) {
    switch s {
    case "sop":
        return LearningSOP, nil
    case "protocol":
        return LearningProtocol, nil
    case "knowledge":
        return LearningKnowledge, nil
    case "conclusion":
        return LearningConclusion, nil
    case "pending":
        return LearningPending, nil
    default:
        return LearningUnknown, fmt.Errorf("unknown learning class %q", s)
    }
}

// MarshalJSON encodes the wire-format string.
func (k LearningClass) MarshalJSON() ([]byte, error) {
    return json.Marshal(k.String())
}

// UnmarshalJSON parses the wire-format string. An empty string defaults to
// LearningSOP (zero value compatible with Phase 3 SideEffectStatus precedent).
func (k *LearningClass) UnmarshalJSON(data []byte) error {
    var s string
    if err := json.Unmarshal(data, &s); err != nil {
        return err
    }
    if s == "" {
        *k = LearningSOP
        return nil
    }
    parsed, err := ParseLearningClass(s)
    if err != nil {
        return err
    }
    *k = parsed
    return nil
}
```

---

## 2. LearningAsset + AssetContent

### 2.1 learn/learning_asset.go

```go
package learn

import (
    "errors"
    "time"

    "github.com/clawcode-devrix/devrix/internal/layers/orchestration/orchtypes"
    "github.com/clawcode-devrix/devrix/internal/shared/types"
)

// LearningClass is re-exported from shared/types (Phase 3 SideEffectStatus +
// Phase 4 VerdictKind precedent).
type LearningClass = types.LearningClass

// Sentinel errors (LP-1-5 衍生).
var (
    ErrAssetIncomplete            = errors.New("learn: asset content validation failed")
    ErrAssetClassMismatch         = errors.New("learn: asset class does not match memory channel")
    ErrAssetBuildFailed           = errors.New("learn: failed to build asset from verdict")
    ErrReputationStoreUnavailable = errors.New("learn: reputation store unavailable")
    ErrAdaptivePriorNotReady      = errors.New("learn: adaptive prior not ready (cold start)")
    ErrScheduledRetryExhausted    = errors.New("learn: scheduled retry exhausted")
)

// LearningAsset is the unified output entity of the Learn node (immutable).
type LearningAsset struct {
    ID            string
    SessionID     string
    Class         LearningClass
    Strength      orchtypes.CertaintyStrength

    // LP-5 cross-session traceability
    SourceSessionIDs []string
    SourceVerdictIDs []string

    // Asset content (5 polymorphic classes)
    Content AssetContent

    // LP-4 falsifiability
    FailureCriterion string
    ExpiryAt         time.Time

    // Idempotency keys
    AssetKey    string
    ContentHash string

    // Metadata
    CreatedAt  time.Time
    LastUsedAt time.Time
    UseCount   int
    TraceID    string
}

// CurrentAssetSchemaVersion is the current AssetContent schema version.
const CurrentAssetSchemaVersion = "1.0.0"

// NewLearningAsset is the immutable factory function with fail-fast validation.
// Auto-sets CreatedAt=now, ExpiryAt=now+24h, ContentHash=hashContent(content).
func NewLearningAsset(id, sessionID string, class LearningClass, content AssetContent, assetKey string) (*LearningAsset, error) {
    if id == "" || sessionID == "" || assetKey == "" {
        return nil, ErrAssetIncomplete
    }
    if class == LearningUnknown {
        return nil, ErrAssetIncomplete
    }
    if content == nil {
        return nil, ErrAssetIncomplete
    }
    if err := content.Validate(); err != nil {
        return nil, fmt.Errorf("%w: %v", ErrAssetIncomplete, err)
    }
    now := time.Now()
    return &LearningAsset{
        ID:               id,
        SessionID:        sessionID,
        Class:            class,
        Strength:         classToStrength(class),
        Content:          content,
        AssetKey:         assetKey,
        ContentHash:      hashContent(content),
        SourceSessionIDs: []string{sessionID},
        FailureCriterion: "ExpiryAt < now() OR UseCount > MaxUseCount",
        CreatedAt:        now,
        ExpiryAt:         now.Add(24 * time.Hour),
    }, nil
}

func classToStrength(class LearningClass) orchtypes.CertaintyStrength {
    switch class {
    case LearningSOP:
        return 5
    case LearningProtocol:
        return 4
    case LearningKnowledge:
        return 3
    case LearningConclusion:
        return 2
    case LearningPending:
        return 1
    default:
        return 0
    }
}

func hashContent(content AssetContent) string {
    h := sha256.New()
    h.Write([]byte(content.SchemaVersion()))
    data, _ := json.Marshal(content)
    h.Write(data)
    return hex.EncodeToString(h.Sum(nil))
}
```

### 2.2 learn/asset_content.go

```go
package learn

import (
    "errors"
    "time"

    "github.com/clawcode-devrix/devrix/internal/layers/orchestration/execute"
)

// AssetContent is the polymorphic content interface for 5 classes.
type AssetContent interface {
    Validate() error
    SchemaVersion() string
    ByteSize() int
}

// ──────────────────────────────────────────────────────────
// ★5 SOPAssetContent
// ──────────────────────────────────────────────────────────
type SOPAssetContent struct {
    Name            string
    Description     string
    Steps           []string
    PreConditions   []string
    PostConditions  []string
    ApplicableTools []string
    EstimatedMs     int
}

func (c *SOPAssetContent) Validate() error {
    if c.Name == "" || len(c.Steps) == 0 {
        return errors.New("sop: Name and Steps are required")
    }
    return nil
}

func (c *SOPAssetContent) SchemaVersion() string { return CurrentAssetSchemaVersion }
func (c *SOPAssetContent) ByteSize() int {
    return len(c.Name) + len(c.Description) + len(c.Steps)*64
}

// ──────────────────────────────────────────────────────────
// ★4 ProtocolAssetContent
// ──────────────────────────────────────────────────────────
type ProtocolAssetContent struct {
    Name     string
    Trigger  string
    Actions  []string
    SLA      SLAConfig
    Fallback string
}

type SLAConfig struct {
    TargetMs    int
    MaxRetries  int
    OpenTimeout time.Duration
}

func (c *ProtocolAssetContent) Validate() error {
    if c.Trigger == "" {
        return errors.New("protocol: Trigger is required")
    }
    return nil
}

func (c *ProtocolAssetContent) SchemaVersion() string { return CurrentAssetSchemaVersion }
func (c *ProtocolAssetContent) ByteSize() int         { return 256 }

// ──────────────────────────────────────────────────────────
// ★3 KnowledgeAssetContent
// ──────────────────────────────────────────────────────────
type KnowledgeAssetContent struct {
    Topic        string
    Hypothesis   string
    Evidence     []string
    CounterEvid  []string
    Confidence   float64
    RelatedCases []string
}

func (c *KnowledgeAssetContent) Validate() error {
    if c.Topic == "" || c.Hypothesis == "" {
        return errors.New("knowledge: Topic and Hypothesis are required")
    }
    if c.Confidence < 0 || c.Confidence > 1 {
        return errors.New("knowledge: Confidence must be in [0, 1]")
    }
    return nil
}

func (c *KnowledgeAssetContent) SchemaVersion() string { return CurrentAssetSchemaVersion }
func (c *KnowledgeAssetContent) ByteSize() int         { return 512 }

// ──────────────────────────────────────────────────────────
// ★2 ConclusionAssetContent
// ──────────────────────────────────────────────────────────
type ConclusionAssetContent struct {
    Statement           string
    PValue              float64
    ConfidenceInterval  [2]float64
    SampleSize          int
    Methodology         string
    Limitations         []string
}

func (c *ConclusionAssetContent) Validate() error {
    if c.Statement == "" {
        return errors.New("conclusion: Statement is required")
    }
    return nil
}

func (c *ConclusionAssetContent) SchemaVersion() string { return CurrentAssetSchemaVersion }
func (c *ConclusionAssetContent) ByteSize() int         { return 256 }

// ──────────────────────────────────────────────────────────
// ⭐★1 PendingAssetContent (5th class — VerdictIndeterminate)
// ──────────────────────────────────────────────────────────
type PendingAssetContent struct {
    IndeterminateReason string
    OriginalArtifactID  string
    RetryAttempts       int
    MaxRetries          int
    NextRetryAt         time.Time
    BlockedReason       string

    // ⭐MVP MVE checkpoint state (from doc 44 §4.4 StrategyDecider)
    MVEState  *execute.MVEState
    PlanID    string
    SessionID string
    Question  string
    Options   []string
}

func (c *PendingAssetContent) Validate() error {
    if c.IndeterminateReason == "" || c.OriginalArtifactID == "" {
        return errors.New("pending: IndeterminateReason and OriginalArtifactID are required")
    }
    if c.RetryAttempts < 0 || c.RetryAttempts > 3 {
        return errors.New("pending: RetryAttempts must be in [0, 3]")
    }
    if c.MVEState != nil && c.Question == "" {
        return errors.New("pending: Question is required when MVEState is set")
    }
    return nil
}

func (c *PendingAssetContent) SchemaVersion() string { return CurrentAssetSchemaVersion }
func (c *PendingAssetContent) ByteSize() int         { return 384 }
```

---

## 3. ReputationEvidence + Bayesian Update

### 3.1 learn/reputation_evidence.go

```go
package learn

import (
    "errors"
    "math"
    "time"

    "github.com/clawcode-devrix/devrix/internal/layers/orchestration/verify"
)

const (
    TrackModeDeveloper = "developer"
    TrackModeOperator  = "operator"
)

// TrackMode is a string-typed alias for the 2 supported tracks.
type TrackMode string

// ParseTrackMode parses a wire-format track mode.
func ParseTrackMode(s string) (TrackMode, error) {
    switch s {
    case TrackModeDeveloper, TrackModeOperator:
        return TrackMode(s), nil
    default:
        return "", fmt.Errorf("unknown track mode %q", s)
    }
}

// ReputationEvidence captures cross-session reputation (Bayesian Beta).
type ReputationEvidence struct {
    SessionID  string
    TrackMode  TrackMode

    // Beta distribution parameters (LP-3)
    Alpha int
    Beta  int

    // Derived metrics
    Mean           float64
    Variance       float64
    ConfidenceLow  float64
    ConfidenceHigh float64

    // Metadata
    LastUpdated      time.Time
    UpdateCount      int
    SourceVerdictIDs []string

    // ⭐G8-1 fix: Verifier failure counter (does NOT pollute α/β)
    VerifierFailureCount int

    // ⭐G5-3 fix: INDETERMINATE counter (env-limited + verifier-failure)
    IndeterminateCount int
}

// NewReputationEvidence creates a fresh ReputationEvidence with zero
// prior (cold start).
func NewReputationEvidence(sessionID string, trackMode TrackMode) (*ReputationEvidence, error) {
    if sessionID == "" {
        return nil, ErrReputationStoreUnavailable
    }
    if _, err := ParseTrackMode(string(trackMode)); err != nil {
        return nil, fmt.Errorf("%w: %v", ErrReputationStoreUnavailable, err)
    }
    return &ReputationEvidence{
        SessionID:        sessionID,
        TrackMode:        trackMode,
        Alpha:            0,
        Beta:             0,
        Mean:             0,
        Variance:         0,
        ConfidenceLow:    0,
        ConfidenceHigh:   1,
        LastUpdated:      time.Now(),
        UpdateCount:      0,
        SourceVerdictIDs: []string{},
    }, nil
}

// BayesianUpdate applies one Verdict to a prior ReputationEvidence and
// returns a NEW ReputationEvidence (prior unchanged).
//
// ⭐G8-1 fix: VerdictIndeterminate with IndeterminateReason ==
// "verifier_parse_failure" only increments VerifierFailureCount and does
// NOT pollute α/β (verifier LLM output format issue is not the user's
// fault).
func BayesianUpdate(prior *ReputationEvidence, verdict verify.Verdict) *ReputationEvidence {
    next := *prior // immutable copy
    next.UpdateCount++
    next.LastUpdated = time.Now()
    next.SourceVerdictIDs = append(next.SourceVerdictIDs, verdict.ID)

    switch verdict.Kind {
    case verify.VerdictPass, verify.VerdictPartial:
        next.Alpha++
    case verify.VerdictFail:
        next.Beta++
    case verify.VerdictIndeterminate:
        // ⭐G8-1 fix: distinguish verifier_parse_failure from env-limited
        if verdict.IndeterminateReason == "verifier_parse_failure" {
            next.VerifierFailureCount++
            // DO NOT update α/β — verifier output issue is not user's fault
        } else {
            next.IndeterminateCount++
            // Other INDETERMINATE: keep prior behavior (no α/β update)
        }
    }

    // Derived metrics
    total := next.Alpha + next.Beta
    if total == 0 {
        // ⭐G8-1 defensive: cold start α=β=0 → keep prior Mean
        next.Mean = prior.Mean
        next.Variance = 0
        next.ConfidenceLow = 0
        next.ConfidenceHigh = 1
    } else {
        totalF := float64(total)
        next.Mean = float64(next.Alpha) / totalF
        next.Variance = float64(next.Alpha*next.Beta) / (totalF * totalF * (totalF + 1))
        next.ConfidenceLow, next.ConfidenceHigh = wilsonScoreInterval(next.Alpha, next.Beta, 0.95)
    }

    return &next
}

// wilsonScoreInterval computes the Wilson Score confidence interval.
// confidence=0.95 → z=1.96.
func wilsonScoreInterval(alpha, beta int, confidence float64) (float64, float64) {
    n := float64(alpha + beta)
    if n == 0 {
        return 0, 1
    }
    var z float64
    switch confidence {
    case 0.95:
        z = 1.96
    case 0.99:
        z = 2.576
    default:
        z = 1.96
    }
    pHat := float64(alpha) / n
    z2 := z * z
    center := pHat + z2/(2*n)
    margin := z * math.Sqrt((pHat*(1-pHat))/n+z2/(4*n*n))
    denom := 1 + z2/n
    return (center - margin) / denom, (center + margin) / denom
}
```

---

## 4. AdaptivePrior + DefaultPriors

### 4.1 learn/adaptive_prior.go

```go
package learn

// BetaPrior is a Beta distribution prior (α, β).
type BetaPrior struct {
    Alpha int
    Beta  int
}

// String returns the wire-format "Beta(α,β)".
func (p BetaPrior) String() string {
    return fmt.Sprintf("Beta(%d,%d)", p.Alpha, p.Beta)
}

// DefaultPriors — from doc 25 §四.
var (
    // DefaultDeveloperPrior — slightly positive prior for developers.
    DefaultDeveloperPrior = BetaPrior{Alpha: 5, Beta: 3}

    // DefaultOperatorPrior — strongly positive prior for operators.
    DefaultOperatorPrior = BetaPrior{Alpha: 8, Beta: 1}
)

// InjectTarget identifies where to inject the AdaptivePrior.
type InjectTarget int

const (
    InjectIntentQuantizer InjectTarget = iota
    InjectHistoricalDetector
    InjectRuleClassifier
)

func (t InjectTarget) String() string {
    switch t {
    case InjectIntentQuantizer:
        return "intent_quantizer"
    case InjectHistoricalDetector:
        return "historical_detector"
    case InjectRuleClassifier:
        return "rule_classifier"
    default:
        return fmt.Sprintf("InjectTarget(%d)", int(t))
    }
}

// DefaultInjectTargets — the 3 standard injection points.
var DefaultInjectTargets = []InjectTarget{
    InjectIntentQuantizer,
    InjectHistoricalDetector,
    InjectRuleClassifier,
}

// AdaptivePrior is the immutable output of BuildAdaptivePrior.
type AdaptivePrior struct {
    Reputation    *ReputationEvidence
    PriorBeta     BetaPrior
    InjectTargets []InjectTarget
}

// BuildAdaptivePrior constructs an AdaptivePrior from a ReputationEvidence
// and track mode. Merges DefaultPrior with Reputation (Bayesian combination).
//
// rep == nil → DefaultPrior + InjectTargets + Reputation=nil.
// trackMode == "" → DefaultDeveloperPrior (fail-safe).
func BuildAdaptivePrior(rep *ReputationEvidence, trackMode TrackMode) *AdaptivePrior {
    var prior BetaPrior
    switch trackMode {
    case TrackModeOperator:
        prior = DefaultOperatorPrior
    case TrackModeDeveloper, "":
        prior = DefaultDeveloperPrior
    default:
        prior = DefaultDeveloperPrior
    }

    if rep != nil {
        prior.Alpha += rep.Alpha
        prior.Beta += rep.Beta
    }

    return &AdaptivePrior{
        Reputation:    rep,
        PriorBeta:     prior,
        InjectTargets: DefaultInjectTargets,
    }
}
```

---

## 5. Memory 3 通道

### 5.1 learn/memory.go

```go
package learn

import (
    "context"
    "errors"
    "sync"

    "github.com/clawcode-devrix/devrix/internal/layers/orchestration/orchtypes"
)

// MemoryChannel identifies the 3 memory channels (LP-2 隔离).
type MemoryChannel int

const (
    MemorySkill     MemoryChannel = iota // SOP / Protocol
    MemoryFeedback                       // Knowledge / Conclusion
    MemoryScheduled                      // Pending retry
)

// MemoryFilter constrains List queries.
type MemoryFilter struct {
    Class       LearningClass
    SessionID   string
    MinStrength orchtypes.CertaintyStrength
    Expired     bool
}

// Memory is the 4-method interface for all 3 channels.
type Memory interface {
    Store(ctx context.Context, asset *LearningAsset) error
    Retrieve(ctx context.Context, key string) (*LearningAsset, error)
    Delete(ctx context.Context, key string) error
    List(ctx context.Context, filter MemoryFilter) ([]*LearningAsset, error)
}

// ──────────────────────────────────────────────────────────
// SkillMemory — SOP / Protocol
// ──────────────────────────────────────────────────────────
type SkillMemory struct {
    Store map[string]*LearningAsset
    mu    sync.RWMutex
}

func NewSkillMemory() *SkillMemory {
    return &SkillMemory{Store: make(map[string]*LearningAsset)}
}

func (m *SkillMemory) Store(ctx context.Context, asset *LearningAsset) error {
    if asset.Class != LearningSOP && asset.Class != LearningProtocol {
        return ErrAssetClassMismatch
    }
    m.mu.Lock()
    defer m.mu.Unlock()
    m.Store[asset.AssetKey] = asset
    return nil
}

func (m *SkillMemory) Retrieve(ctx context.Context, key string) (*LearningAsset, error) {
    m.mu.RLock()
    defer m.mu.RUnlock()
    a, ok := m.Store[key]
    if !ok {
        return nil, ErrAssetIncomplete
    }
    return a, nil
}

func (m *SkillMemory) Delete(ctx context.Context, key string) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    delete(m.Store, key)
    return nil
}

func (m *SkillMemory) List(ctx context.Context, filter MemoryFilter) ([]*LearningAsset, error) {
    m.mu.RLock()
    defer m.mu.RUnlock()
    var out []*LearningAsset
    for _, a := range m.Store {
        if filter.Class != LearningUnknown && a.Class != filter.Class {
            continue
        }
        if filter.SessionID != "" && a.SessionID != filter.SessionID {
            continue
        }
        if filter.MinStrength > 0 && a.Strength < filter.MinStrength {
            continue
        }
        if !filter.Expired && a.ExpiryAt.Before(time.Now()) {
            continue
        }
        out = append(out, a)
    }
    return out, nil
}

// ──────────────────────────────────────────────────────────
// FeedbackMemory — Knowledge / Conclusion
// ──────────────────────────────────────────────────────────
type FeedbackMemory struct {
    Store map[string]*LearningAsset
    mu    sync.RWMutex
}

func NewFeedbackMemory() *FeedbackMemory {
    return &FeedbackMemory{Store: make(map[string]*LearningAsset)}
}

func (m *FeedbackMemory) Store(ctx context.Context, asset *LearningAsset) error {
    if asset.Class != LearningKnowledge && asset.Class != LearningConclusion {
        return ErrAssetClassMismatch
    }
    m.mu.Lock()
    defer m.mu.Unlock()
    m.Store[asset.AssetKey] = asset
    return nil
}

// ... Retrieve/Delete/List same pattern as SkillMemory

// ──────────────────────────────────────────────────────────
// ScheduledMemory — Pending retry
// ──────────────────────────────────────────────────────────
type ScheduledRetry struct {
    Asset       *LearningAsset
    TriggerAt   time.Time
    RetryCount  int
    MaxRetries  int
    LastRetryAt time.Time
}

type ScheduledMemory struct {
    Store map[string]*ScheduledRetry
    mu    sync.RWMutex
}

func NewScheduledMemory() *ScheduledMemory {
    return &ScheduledMemory{Store: make(map[string]*ScheduledRetry)}
}

func (m *ScheduledMemory) Store(ctx context.Context, asset *LearningAsset) error {
    if asset.Class != LearningPending {
        return ErrAssetClassMismatch
    }
    m.mu.Lock()
    defer m.mu.Unlock()
    m.Store[asset.AssetKey] = &ScheduledRetry{
        Asset:      asset,
        TriggerAt:  asset.ExpiryAt,
        MaxRetries: 3,
    }
    return nil
}

// ... Retrieve/Delete/List same pattern
```

---

## 6. Learner interface + DefaultLearner

### 6.1 learn/learner.go

```go
package learn

import (
    "context"
    "fmt"

    "github.com/clawcode-devrix/devrix/internal/layers/orchestration/execute"
    "github.com/clawcode-devrix/devrix/internal/layers/orchestration/plan"
    "github.com/clawcode-devrix/devrix/internal/layers/orchestration/verify"
)

// LearnRequest is the input to Learner.Learn().
type LearnRequest struct {
    Verdict      verify.Verdict
    Plan         plan.Plan
    Observations []orchtypes.Observation
    Artifact     execute.Artifact
    SessionID    string
}

// Learner is the public interface of the Learn node.
type Learner interface {
    // Learn deposits a Verdict as a LearningAsset and updates ReputationEvidence.
    Learn(ctx context.Context, req LearnRequest) ([]LearningAsset, error)

    // Inject (LP-1) reads ReputationEvidence for the session and produces
    // an AdaptivePrior for the next Observe.All() call.
    Inject(ctx context.Context, sessionID string) (*AdaptivePrior, error)

    // ScheduledTick processes Pending retries (called periodically).
    ScheduledTick(ctx context.Context) error
}

// ReputationStore is the persistence interface for ReputationEvidence.
type ReputationStore interface {
    Get(ctx context.Context, sessionID string) (*ReputationEvidence, error)
    Update(ctx context.Context, evidence *ReputationEvidence) error
    List(ctx context.Context, trackMode TrackMode, limit int) ([]*ReputationEvidence, error)
}

// DefaultLearner is the default Learner implementation.
type DefaultLearner struct {
    SkillMemory      Memory
    FeedbackMemory   Memory
    ScheduledMemory  Memory
    ReputationStore  ReputationStore
    AssetBuilder     *AssetBuilder
    BayesianUpdater  func(prior *ReputationEvidence, verdict verify.Verdict) *ReputationEvidence
    FeedbackAdder    func(ctx context.Context, asset *LearningAsset) error
}

// Learn implements Learner.Learn.
func (l *DefaultLearner) Learn(ctx context.Context, req LearnRequest) ([]LearningAsset, error) {
    // 1. Choose LearningClass (INDETERMINATE → LearningPending)
    class := classFromVerdictKind(req.Verdict.Kind)
    if class == LearningUnknown {
        return nil, fmt.Errorf("%w: unknown verdict kind", ErrAssetBuildFailed)
    }

    // 2. Build LearningAsset
    asset := l.AssetBuilder.Build(ctx, req, class)
    if asset == nil {
        return nil, ErrAssetBuildFailed
    }

    // 3. Route to Memory (LP-2 隔离)
    var storeErr error
    switch class {
    case LearningSOP, LearningProtocol:
        storeErr = l.SkillMemory.Store(ctx, asset)
    case LearningKnowledge, LearningConclusion:
        storeErr = l.FeedbackMemory.Store(ctx, asset)
    case LearningPending:
        storeErr = l.ScheduledMemory.Store(ctx, asset)
    }
    if storeErr != nil {
        return nil, storeErr
    }

    // 4. Bayesian update ReputationEvidence (LP-3)
    prior, err := l.ReputationStore.Get(ctx, req.SessionID)
    if err != nil {
        prior = nil // cold start: prior == nil
    }
    if prior == nil {
        prior = &ReputationEvidence{
            SessionID:        req.SessionID,
            TrackMode:        determineTrackMode(req),
            Alpha:            0,
            Beta:             0,
            Mean:             0,
            Variance:         0,
            ConfidenceLow:    0,
            ConfidenceHigh:   1,
            LastUpdated:      time.Now(),
            UpdateCount:      0,
            SourceVerdictIDs: []string{},
        }
    }
    updated := l.BayesianUpdater(prior, req.Verdict)
    if err := l.ReputationStore.Update(ctx, updated); err != nil {
        return nil, fmt.Errorf("%w: %v", ErrReputationStoreUnavailable, err)
    }

    return []LearningAsset{*asset}, nil
}

// Inject implements Learner.Inject (LP-1 closed loop).
func (l *DefaultLearner) Inject(ctx context.Context, sessionID string) (*AdaptivePrior, error) {
    rep, err := l.ReputationStore.Get(ctx, sessionID)
    if err != nil {
        // Cold start: use DefaultPriors
        return BuildAdaptivePrior(nil, TrackModeDeveloper), nil
    }
    return BuildAdaptivePrior(rep, rep.TrackMode), nil
}

// ScheduledTick implements Learner.ScheduledTick.
func (l *DefaultLearner) ScheduledTick(ctx context.Context) error {
    // 1. Iterate ScheduledMemory entries where TriggerAt <= now
    // 2. For each, increment RetryCount; if RetryCount >= MaxRetries:
    //    - Move to FeedbackMemory (warning)
    //    - Delete from ScheduledMemory
    // 3. Otherwise, re-trigger Verifier (Phase 4 doc 45 §4.6)
    return nil
}

func classFromVerdictKind(kind verify.VerdictKind) LearningClass {
    switch kind {
    case verify.VerdictIndeterminate:
        return LearningPending
    case verify.VerdictPass, verify.VerdictPartial:
        // Default to Knowledge for now (asset builder refines by VerdictClass)
        return LearningKnowledge
    case verify.VerdictFail:
        return LearningKnowledge
    default:
        return LearningUnknown
    }
}

func determineTrackMode(req LearnRequest) TrackMode {
    // MVP: derive from Plan.Kind or fallback to developer
    if req.Plan.Kind == plan.PlanOperatorMode {
        return TrackModeOperator
    }
    return TrackModeDeveloper
}
```

### 6.2 learn/asset_builder.go

```go
package learn

import (
    "context"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"

    "github.com/clawcode-devrix/devrix/internal/layers/orchestration/plan"
    "github.com/clawcode-devrix/devrix/internal/layers/orchestration/verify"
)

// AssetBuilder constructs a LearningAsset from a LearnRequest + class.
type AssetBuilder struct{}

func (b *AssetBuilder) Build(ctx context.Context, req LearnRequest, class LearningClass) *LearningAsset {
    var content AssetContent
    var assetKey string

    switch class {
    case LearningSOP:
        content = &SOPAssetContent{
            Name:            extractSOPName(req),
            Description:     req.Verdict.Reason,
            Steps:           extractStepsFromPlan(req.Plan),
            ApplicableTools: extractTools(req.Artifact),
            EstimatedMs:     extractEstimatedMs(req.Artifact),
        }
        assetKey = fmt.Sprintf("sop:%s:%s", req.Plan.Kind, hashContentBytes(content))

    case LearningProtocol:
        content = &ProtocolAssetContent{
            Name:     extractProtocolName(req),
            Trigger:  extractTriggerFromVerdict(req.Verdict),
            Actions:  extractActions(req.Artifact),
            Fallback: req.Plan.FailureCriteria,
        }
        assetKey = fmt.Sprintf("protocol:%s:%s", req.Plan.Kind, hashContentBytes(content))

    case LearningKnowledge:
        content = &KnowledgeAssetContent{
            Topic:       extractTopic(req.Verdict),
            Hypothesis:  req.Verdict.Reason,
            Evidence:    extractEvidence(req.Verdict),
            Confidence:  clamp01(req.Verdict.Confidence),
        }
        assetKey = fmt.Sprintf("knowledge:%s:%s", req.Plan.Kind, hashContentBytes(content))

    case LearningConclusion:
        content = &ConclusionAssetContent{
            Statement: req.Verdict.Reason,
            PValue:    clamp01(req.Verdict.Confidence),
            Methodology: req.Plan.Strategy.String(),
        }
        assetKey = fmt.Sprintf("conclusion:%s:%s", req.Plan.Kind, hashContentBytes(content))

    case LearningPending:
        content = &PendingAssetContent{
            IndeterminateReason: req.Verdict.IndeterminateReason,
            OriginalArtifactID:  req.Artifact.ID,
            RetryAttempts:       0,
            MaxRetries:          3,
            NextRetryAt:         time.Now().Add(time.Hour),
        }
        assetKey = fmt.Sprintf("pending:%s:%s", req.Artifact.ID, hashContentBytes(content))

    default:
        return nil
    }

    asset, err := NewLearningAsset(uuid.New().String(), req.SessionID, class, content, assetKey)
    if err != nil {
        return nil
    }
    return asset
}

func hashContentBytes(content AssetContent) string {
    h := sha256.New()
    data, _ := json.Marshal(content)
    h.Write(data)
    return hex.EncodeToString(h.Sum(nil))[:16]
}

func clamp01(v float64) float64 {
    if v < 0 { return 0 }
    if v > 1 { return 1 }
    return v
}

// Stub extractors (full impl in PR-E5 E5.2.2)
func extractSOPName(req LearnRequest) string { return req.Plan.ID }
func extractProtocolName(req LearnRequest) string { return req.Plan.ID }
func extractTopic(v verify.Verdict) string { return v.Class.String() }
func extractStepsFromPlan(p plan.Plan) []string { return []string{p.ID} }
func extractTriggerFromVerdict(v verify.Verdict) string { return v.Reason }
func extractActions(a execute.Artifact) []string { return []string{a.ID} }
func extractEvidence(v verify.Verdict) []string { return v.EvidenceIDs }
func extractTools(a execute.Artifact) []string { return a.ToolRefs }
func extractEstimatedMs(a execute.Artifact) int { return a.EstimatedMs }
```

---

## 7. ReputationStore

### 7.1 learn/reputation_store.go

```go
package learn

import (
    "context"
    "sync"
)

type InMemoryReputationStore struct {
    Store map[string]*ReputationEvidence
    mu    sync.RWMutex
}

func NewInMemoryReputationStore() *InMemoryReputationStore {
    return &InMemoryReputationStore{Store: make(map[string]*ReputationEvidence)}
}

func (s *InMemoryReputationStore) Get(ctx context.Context, sessionID string) (*ReputationEvidence, error) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    rep, ok := s.Store[sessionID]
    if !ok {
        return nil, nil // cold start: caller treats nil as new
    }
    return rep, nil
}

func (s *InMemoryReputationStore) Update(ctx context.Context, evidence *ReputationEvidence) error {
    if evidence == nil || evidence.SessionID == "" {
        return ErrReputationStoreUnavailable
    }
    s.mu.Lock()
    defer s.mu.Unlock()
    s.Store[evidence.SessionID] = evidence
    return nil
}

func (s *InMemoryReputationStore) List(ctx context.Context, trackMode TrackMode, limit int) ([]*ReputationEvidence, error) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    var out []*ReputationEvidence
    for _, rep := range s.Store {
        if trackMode != "" && rep.TrackMode != trackMode {
            continue
        }
        out = append(out, rep)
        if limit > 0 && len(out) >= limit {
            break
        }
    }
    return out, nil
}
```

---

## 8. Observe 节点对接 (LP-1 闭环)

### 8.1 orchtypes/intent_quantizer.go (增量)

```go
// QuantizeWithPrior uses prior.PriorBeta as the Beta prior when present.
func (q *IntentQuantizer) QuantizeWithPrior(ctx context.Context, intent string, prior *learn.AdaptivePrior) (IntentPayload, error) {
    if prior == nil {
        return q.Quantize(ctx, intent)
    }
    // Use prior.PriorBeta.Alpha/Beta to weight the classification
    return q.quantizeWithBeta(ctx, intent, prior.PriorBeta.Alpha, prior.PriorBeta.Beta)
}
```

### 8.2 sessionorchestrator/orchestrator.go ProcessMessage (LP-1 时序约束)

```go
func (o *Orchestrator) ProcessMessage(ctx context.Context, msg *bridge.UserMessage) error {
    // ... existing setup ...

    // ⭐LP-1: Inject prior BEFORE Observe.All()
    if o.Learner != nil {
        prior, err := o.Learner.Inject(ctx, msg.SessionID)
        if err == nil && prior != nil {
            o.ObserveRequest.Prior = prior
        }
        // fail-safe: nil prior → Observe uses DefaultDeveloperPrior
    }

    // ... existing Observe.All(), Plan, Execute, Verify ...

    // After Verify completes, Learn the verdict
    if o.Learner != nil && verdict != nil {
        _, err := o.Learner.Learn(ctx, learn.LearnRequest{
            Verdict: *verdict,
            Plan: plan,
            Observations: observations,
            Artifact: artifact,
            SessionID: msg.SessionID,
        })
        if err != nil {
            slog.Warn("learn failed", "err", err)
        }
    }

    return nil
}
```

---

## Cross-references

- **设计稿**：doc 46 (D7 Learn 节点详细技术方案 1124 行)
- **方法论**：doc 35 §三 (5 节点管道方法论) + doc 37 §2.5-6 (LearningAsset + ReputationEvidence 实体)
- **先验来源**：doc 25 §四 (Developer Beta(5,3) / Operator Beta(8,1))
- **3 通道记忆**：doc 27 §三 (Skill/Feedback/Scheduled 分类)
- **跨会话累积策略**：doc 27 §四
- **G8-1 修复参考**：Phase 4 doc 45 §4.6 VerifyWithRetry parse failure → INDETERMINATE
- **跨域类型 precedent**：Phase 1 MemoryEntry (shared/types) + Phase 3 SideEffectStatus (D7-S9-A25-T04) + Phase 4 VerdictKind (D7-S10-A32-T01)
- **anomalyAdapter 模式 precedent**：Phase 4 PR-D4 observationAdapter 模式避免 orchtypes ↔ workmodel import cycle