// Package learn implements the D7 Learn node — the 5th node of the MUPS 5-node
// pipeline (Observe → Plan → Execute → Verify → Learn). Learn deposits Verdict
// outputs as LearningAsset and updates ReputationEvidence via Bayesian
// Update, then injects AdaptivePrior into the next Observe.All() call (LP-1
// closed loop).
//
// Promoted from doc 46 (D7 Learn 节点详细技术方案 2026-06-22) as Phase 5 of
// devrix-d7-mups-v4-phase5-learn (DM-20260623-003).
package asset

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/devrix/devrix/internal/shared/types"
)

// LearningClass is re-exported from shared/types (Phase 3 SideEffectStatus +
// Phase 4 VerdictKind precedent).
type LearningClass = types.LearningClass

// Sentinel errors (LP-1..5 衍生). All callers MUST use errors.Is for
// comparisons.
var (
	// ErrAssetIncomplete — required fields missing or Content.Validate() failed.
	ErrAssetIncomplete = errors.New("learn: asset content validation failed")

	// ErrAssetClassMismatch — asset.Class does not match the target Memory channel
	// (LP-2 隔离).
	ErrAssetClassMismatch = errors.New("learn: asset class does not match memory channel")

	// ErrAssetBuildFailed — AssetBuilder.Build returned nil (verdict / plan / artifact
	// could not be translated into a LearningAsset).
	ErrAssetBuildFailed = errors.New("learn: failed to build asset from verdict")

	// ErrReputationStoreUnavailable moved to mups/learn/reputation/evidence.go
	// (v6.0.0 subpackage split; logically belongs to ReputationStore).

	// ErrAdaptivePriorNotReady moved to mups/learn/prior/adaptive_prior.go
	// (v6.0.0 subpackage split; logically belongs to AdaptivePrior).

	// ErrScheduledRetryExhausted — ScheduledMemory entry exceeded MaxRetries.
	ErrScheduledRetryExhausted = errors.New("learn: scheduled retry exhausted")
)

// CertaintyStrength is the ★ rating (1-5) derived from LearningClass
// (LP-4 衍生: Strength ∈ [1, 5] corresponds to Class ordinal).
type CertaintyStrength uint8

const (
	// StrengthUnknown — reserved zero value; MUST be rejected.
	StrengthUnknown CertaintyStrength = 0

	// StrengthPending — LearningPending (★1, ⭐new in Phase 5).
	StrengthPending CertaintyStrength = 1

	// StrengthConclusion — LearningConclusion (★2).
	StrengthConclusion CertaintyStrength = 2

	// StrengthKnowledge — LearningKnowledge (★3).
	StrengthKnowledge CertaintyStrength = 3

	// StrengthProtocol — LearningProtocol (★4).
	StrengthProtocol CertaintyStrength = 4

	// StrengthSOP — LearningSOP (★5).
	StrengthSOP CertaintyStrength = 5
)

// String returns the wire format name.
func (s CertaintyStrength) String() string {
	switch s {
	case StrengthPending:
		return "pending"
	case StrengthConclusion:
		return "conclusion"
	case StrengthKnowledge:
		return "knowledge"
	case StrengthProtocol:
		return "protocol"
	case StrengthSOP:
		return "sop"
	default:
		return fmt.Sprintf("CertaintyStrength(%d)", uint8(s))
	}
}

// ClassToStrength maps a LearningClass to its CertaintyStrength ★ rating.
// The LearningClass enum is declared with iota starting at LearningUnknown=0
// then LearningSOP=1, ..., LearningPending=5 — i.e. enum ordinal and ★
// rating are inverted (SOP=1 → ★5, Pending=5 → ★1). The formula
// `Strength = 6 - class` reflects that inversion. LP-4 衍生.
func ClassToStrength(class types.LearningClass) CertaintyStrength {
	if class < types.LearningSOP || class > types.LearningPending {
		return StrengthUnknown
	}
	return CertaintyStrength(6 - int(class))
}

// CurrentAssetSchemaVersion is the current AssetContent schema version. Bump
// when AssetContent fields are added/removed (LP-4 衍生: schema version gate).
const CurrentAssetSchemaVersion = "1.0.0"

// DefaultAssetTTL is the default LearningAsset TTL (LP-4 衍生).
const DefaultAssetTTL = 24 * time.Hour

// DefaultPendingMaxRetries is the default MaxRetries for PendingAssetContent
// (LP-4 衍生: matches PendingAssetContent.Validate upper bound). Owned by
// asset (PendingAssetContent semantics); memory.ScheduledMemory and
// AssetBuilder.buildPendingContent both consume this constant so the value
// stays single-sourced here.
const DefaultPendingMaxRetries = 3

// LearningAsset is the unified output entity of the Learn node (immutable).
// Construct via NewLearningAsset; do not mutate fields after creation (LP-1-5
// 衍生).
type LearningAsset struct {
	// ID — UUID v7 (必填).
	ID string

	// SessionID — current SessionID (必填).
	SessionID string

	// Class — 5 enum (必填).
	Class LearningClass

	// Strength — ★ 1-5 derived from Class.
	Strength CertaintyStrength

	// SourceSessionIDs — LP-5 cross-session traceability (≥1).
	SourceSessionIDs []string

	// SourceVerdictIDs — LP-5 (≥1).
	SourceVerdictIDs []string

	// SourcePlanNodeIDs (DM-20260707-001 PR-C, codex Risk A2 HIGH):
	// per-segment attribution lineage. For per-child Learn calls, this is
	// the single SegmentID of the child whose completion triggered the
	// Learn. For rollup Learn calls, this is the union of all child
	// SegmentIDs that contributed to the synthesized rollup Verdict. Empty
	// for legacy single-WorkItem Learn paths.
	SourcePlanNodeIDs []string

	// Content — 5 polymorphic classes (必填).
	Content AssetContent

	// AssetKey — idempotency key (必填, unique).
	AssetKey string

	// ContentHash — SHA-256 hex (auto-derived).
	ContentHash string

	// FailureCriterion — LP-4 falsifiability (default: "ExpiryAt < now() OR
	// UseCount > MaxUseCount").
	FailureCriterion string

	// ExpiryAt — TTL gate (LP-4). Default: now+24h.
	ExpiryAt time.Time

	// CreatedAt — auto-set (必填).
	CreatedAt time.Time

	// LastUsedAt — for LRU eviction.
	LastUsedAt time.Time

	// UseCount — for LRU eviction.
	UseCount int

	// TraceID — D5 trace correlation.
	TraceID string
}

// NewLearningAsset is the immutable factory function with fail-fast validation.
// Auto-sets CreatedAt=now, ExpiryAt=now+DefaultAssetTTL, ContentHash via
// hashContentBytes, and SourceSessionIDs=[sessionID].
//
// Returns ErrAssetIncomplete on any required field missing or
// Content.Validate() failing.
func NewLearningAsset(id, sessionID string, class LearningClass, content AssetContent, assetKey string) (*LearningAsset, error) {
	if id == "" {
		return nil, fmt.Errorf("%w: id is empty", ErrAssetIncomplete)
	}
	if sessionID == "" {
		return nil, fmt.Errorf("%w: sessionID is empty", ErrAssetIncomplete)
	}
	if class == types.LearningUnknown {
		return nil, fmt.Errorf("%w: class is LearningUnknown", ErrAssetIncomplete)
	}
	if content == nil {
		return nil, fmt.Errorf("%w: content is nil", ErrAssetIncomplete)
	}
	if assetKey == "" {
		return nil, fmt.Errorf("%w: assetKey is empty", ErrAssetIncomplete)
	}
	if err := content.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrAssetIncomplete, err)
	}
	now := time.Now()
	return &LearningAsset{
		ID:               id,
		SessionID:        sessionID,
		Class:            class,
		Strength:         ClassToStrength(class),
		Content:          content,
		AssetKey:         assetKey,
		ContentHash:      hashContentBytes(content),
		SourceSessionIDs: []string{sessionID},
		FailureCriterion: "ExpiryAt < now() OR UseCount > MaxUseCount",
		CreatedAt:        now,
		ExpiryAt:         now.Add(DefaultAssetTTL),
		LastUsedAt:       now,
	}, nil
}

// WithTraceID returns a new asset with TraceID set (immutable update).
func (a LearningAsset) WithTraceID(traceID string) LearningAsset {
	a.TraceID = traceID
	return a
}

// WithUseCount returns a new asset with incremented UseCount + LastUsedAt (immutable).
func (a LearningAsset) WithUseCount() LearningAsset {
	a.UseCount++
	a.LastUsedAt = time.Now()
	return a
}

// WithSourceVerdictIDs returns a new asset with SourceVerdictIDs extended.
func (a LearningAsset) WithSourceVerdictIDs(ids []string) LearningAsset {
	a.SourceVerdictIDs = append([]string{}, a.SourceVerdictIDs...)
	a.SourceVerdictIDs = append(a.SourceVerdictIDs, ids...)
	return a
}

// IsExpired returns true if the asset is past its ExpiryAt.
func (a LearningAsset) IsExpired() bool {
	return time.Now().After(a.ExpiryAt)
}

// NewAssetID returns a UUID-based asset ID with "asset_" prefix for
// log/trace readability.
func NewAssetID() string {
	return "asset_" + uuid.New().String()[:8]
}

// hashContentBytes returns the first 16 hex chars of the SHA-256 hash of the
// Content. Stable for identical content (LP-4 幂等).
func hashContentBytes(content AssetContent) string {
	h := sha256.New()
	h.Write([]byte(content.SchemaVersion()))
	data, err := json.Marshal(content)
	if err != nil {
		// Should not happen for our typed content, but fall back to schema version
		// so AssetKey construction never panics.
		h.Write([]byte(content.SchemaVersion()))
	} else {
		h.Write(data)
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}