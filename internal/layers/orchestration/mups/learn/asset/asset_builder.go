// Package learn: AssetBuilder — translates a (Verdict, Plan, Observation,
// Artifact) tuple into a typed LearningAsset (PR-E5 E5.2).
//
// AssetBuilder is the bridge between Verify node's output and Learn node's
// persistent memory. The Learn interface calls AssetBuilder.Build with a
// pre-classified LearningClass; AssetBuilder dispatches to one of 5 typed
// content constructors:
//
//	LearningSOP         ← ComplianceVerdict        (★5, SkillMemory)
//	LearningProtocol    ← TimelinessVerdict        (★4, SkillMemory)
//	LearningKnowledge   ← RootCauseVerdict         (★3, FeedbackMemory)
//	LearningConclusion  ← StatisticalVerdict       (★2, FeedbackMemory)
//	LearningPending     ← VerdictIndeterminate     (★1, ScheduledMemory)
//
// All constructors are pure (no side effects) so AssetBuilder is trivially
// testable. NewLearningAsset enforces fail-fast validation (LP-4 衍生).
package asset

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/interfaces"
	"github.com/devrix/devrix/internal/layers/orchestration/plan"
	"github.com/devrix/devrix/internal/layers/orchestration/wavescheduler"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/types"
)

// LearnRequest is the input contract for the Learn node. It bundles the
// upstream pipeline outputs (Verdict + Plan + Observations + Artifact) so
// AssetBuilder can compose a context-rich LearningAsset.
type LearnRequest struct {
	// Verdict — the Phase 4 Verify node's output. Required.
	Verdict workmodel.Verdict

	// Plan — the originating Plan (Phase 2 PR-B1). Optional for INDETERMINATE
	// retries where the Plan may have been evicted.
	Plan *plan.Plan

	// Observations — reverse-lookup subset of UncertaintyReport.Observations
	// (Phase 2 PR-A1). Optional for some asset kinds (e.g. PendingAsset does
	// not need observations).
	Observations []ObservationLookup

	// Artifact — the Phase 3 Execute node's output. Optional for INDETERMINATE
	// retries where the artifact may not exist yet.
	Artifact *wavescheduler.Artifact

	// SessionID — current session (必填).
	SessionID string

	// Report (devrix-d7-taskcontract-unification-pr-a, DM-20260629-007):
	// optional v7.0 unified up-link contract. When non-nil, the Learn node
	// uses Report.Dissent to populate SkillMemory.SOP entries and
	// Report.Resource for cost accounting. PR-A only sets it from new call
	// sites; legacy callers continue to pass the legacy Verdict/Plan/
	// Artifact fields. PR-B fully migrates; PR-C removes the optional
	// marker.
	Report *interfaces.TaskReport
}

// ObservationLookup mirrors plan.ObservationLookup so we don't pull the
// orchtypes package into a learn → orchtypes import cycle (LP-5 衍生).
type ObservationLookup interface {
	GetID() string
}

// Auto-Close fallback constants (Phase 7 PR-7.1, D7-S13-A47).
//
// When processAutoClose synthesizes a Verdict from a terminal EngineEvent
// (no Plan, no Artifact available), the AssetBuilder falls back to
// Verdict.SourceID as the SOP name and a synthetic step list. This keeps
// the LP-1 deposit (BayesianUpdate + ReputationStore) alive in production
// even when the Auto-Close path runs without a full Phase 2 Plan.
const (
	autoCloseSOPNamePrefix = "sop:autoclose:"
	autoCloseSOPSyntheticStep = "autoclose-completion"
)

// AssetBuilder is the (Verdict → LearningAsset) factory. It is stateless and
// safe for concurrent use.
type AssetBuilder struct{}

// NewAssetBuilder constructs an empty AssetBuilder.
func NewAssetBuilder() *AssetBuilder { return &AssetBuilder{} }

// Build dispatches on class to one of 5 typed content constructors and
// returns a NEW LearningAsset (immutable). Returns (nil, nil) when the
// content cannot be constructed from req — callers translate that to
// ErrAssetBuildFailed.
//
// The returned AssetKey format is "{class}:{verdictSource}:{contentHash}"
// for idempotency: re-Learning the same Verdict re-builds with the same key,
// so Memory.Store replaces rather than duplicates (LP-4 衍生).
func (b *AssetBuilder) Build(ctx context.Context, req LearnRequest, class LearningClass) (*LearningAsset, error) {
	if req.SessionID == "" {
		return nil, fmt.Errorf("%w: SessionID is empty", ErrAssetIncomplete)
	}

	content, contentHash, err := buildContent(req, class)
	if err != nil || content == nil {
		return nil, err // already wrapped
	}

	assetKey := buildAssetKey(class, req.Verdict.SourceID, contentHash)
	id := NewAssetID()
	asset, err := NewLearningAsset(id, req.SessionID, class, content, assetKey)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrAssetBuildFailed, err)
	}

	// Wire lineage metadata.
	if req.Plan != nil {
		asset.SourceSessionIDs = append(asset.SourceSessionIDs, req.Plan.SessionID)
	}
	if req.Verdict.SourceID != "" {
		asset.SourceVerdictIDs = append(asset.SourceVerdictIDs, req.Verdict.SourceID)
	}

	return asset, nil
}

// buildContent dispatches to one of 5 typed constructors. Returns
// (nil, "", nil) when the constructor cannot produce content from the
// request — caller translates to ErrAssetBuildFailed.
func buildContent(req LearnRequest, class LearningClass) (AssetContent, string, error) {
	switch class {
	case LearningClass(types.LearningSOP):
		content, hash, err := buildSOPContent(req)
		if content == nil {
			return nil, "", err
		}
		return content, hash, err
	case LearningClass(types.LearningProtocol):
		content, hash, err := buildProtocolContent(req)
		if content == nil {
			return nil, "", err
		}
		return content, hash, err
	case LearningClass(types.LearningKnowledge):
		content, hash, err := buildKnowledgeContent(req)
		if content == nil {
			return nil, "", err
		}
		return content, hash, err
	case LearningClass(types.LearningConclusion):
		content, hash, err := buildConclusionContent(req)
		if content == nil {
			return nil, "", err
		}
		return content, hash, err
	case LearningClass(types.LearningPending):
		content, hash, err := buildPendingContent(req)
		if content == nil {
			return nil, "", err
		}
		return content, hash, err
	default:
		return nil, "", fmt.Errorf("%w: unknown class %s", ErrAssetBuildFailed, class)
	}
}

// ─────────────────────────────────────────────────────────────────────────
// 5 typed content constructors
// ─────────────────────────────────────────────────────────────────────────

func buildSOPContent(req LearnRequest) (*SOPAssetContent, string, error) {
	name := extractSOPName(req)
	steps := extractStepsFromPlan(req.Plan)
	var tools []string
	if req.Artifact != nil {
		tools = req.Artifact.FilesChanged
	}
	if name == "" || len(steps) == 0 {
		return nil, "", nil // signals ErrAssetBuildFailed to caller
	}
	content := &SOPAssetContent{
		Name:            name,
		Description:     req.Verdict.Reason,
		Steps:           steps,
		ApplicableTools: tools,
	}
	return content, hashContentJSON(content), nil
}

func buildProtocolContent(req LearnRequest) (*ProtocolAssetContent, string, error) {
	name := extractProtocolName(req)
	trigger := extractTriggerFromVerdict(req.Verdict)
	var actions []string
	if req.Artifact != nil && req.Artifact.WorkerType != "" {
		actions = []string{string(req.Artifact.WorkerType)}
	}
	if trigger == "" {
		return nil, "", nil
	}
	content := &ProtocolAssetContent{
		Name:     name,
		Trigger:  trigger,
		Actions:  actions,
		SLA:      SLAConfig{TargetMs: 1000, MaxRetries: 3, OpenTimeout: 30 * time.Second},
		Fallback: req.Verdict.Reason,
	}
	return content, hashContentJSON(content), nil
}

func buildKnowledgeContent(req LearnRequest) (*KnowledgeAssetContent, string, error) {
	topic := extractTopic(req.Verdict)
	hypothesis := extractHypothesis(req.Verdict)
	var evidence []string
	if req.Artifact != nil && req.Artifact.Summary != "" {
		evidence = []string{req.Artifact.Summary}
	}
	if topic == "" || hypothesis == "" {
		return nil, "", nil
	}
	content := &KnowledgeAssetContent{
		Topic:      topic,
		Hypothesis: hypothesis,
		Evidence:   evidence,
		Confidence: clampConfidence(req.Verdict.Confidence),
	}
	return content, hashContentJSON(content), nil
}

func buildConclusionContent(req LearnRequest) (*ConclusionAssetContent, string, error) {
	statement := extractConclusion(req.Verdict)
	if statement == "" {
		return nil, "", nil
	}
	content := &ConclusionAssetContent{
		Statement:   statement,
		PValue:      0.05,
		SampleSize:  1,
		Methodology: "single_verdict",
	}
	return content, hashContentJSON(content), nil
}

func buildPendingContent(req LearnRequest) (*PendingAssetContent, string, error) {
	reason := req.Verdict.IndeterminateReason
	if reason == "" {
		reason = "env_limited" // default for INDETERMINATE w/o explicit reason
	}
	originalArtifactID := ""
	if req.Artifact != nil {
		originalArtifactID = req.Artifact.TaskID
	}
	planID := ""
	if req.Plan != nil {
		planID = req.Plan.ID
	}
	content := &PendingAssetContent{
		IndeterminateReason: reason,
		OriginalArtifactID:  originalArtifactID,
		RetryAttempts:       0,
		MaxRetries:          DefaultPendingMaxRetries,
		NextRetryAt:         time.Now().Add(5 * time.Minute),
		PlanID:              planID,
		SessionID:           req.SessionID,
	}
	return content, hashContentJSON(content), nil
}

// ─────────────────────────────────────────────────────────────────────────
// Field extractors (pure helpers)
// ─────────────────────────────────────────────────────────────────────────

func extractSOPName(req LearnRequest) string {
	if req.Plan != nil {
		// Prefer Plan ID as a stable, traceable name.
		if req.Plan.ID != "" {
			return "sop:plan:" + req.Plan.ID
		}
	}
	if req.Artifact != nil && req.Artifact.Summary != "" {
		return "sop:summary:" + truncate(req.Artifact.Summary, 64)
	}
	// Auto-Close fallback (Phase 7 PR-7.1, D7-S13-A47-T02): when neither
	// Plan nor Artifact is available (processAutoClose synthesized the
	// Verdict from a terminal EngineEvent), key the SOP on Verdict.SourceID
	// so the Learn deposit still produces a storable asset.
	if req.Verdict.SourceID != "" {
		return autoCloseSOPNamePrefix + req.Verdict.SourceID
	}
	return ""
}

func extractStepsFromPlan(p *plan.Plan) []string {
	if p == nil {
		// Auto-Close fallback (Phase 7 PR-7.1, D7-S13-A47-T02): synthesize a
		// single placeholder step so the SOPAssetContent has Steps > 0 and
		// the AssetBuilder.Build path succeeds end-to-end.
		return []string{autoCloseSOPSyntheticStep}
	}
	steps := make([]string, 0, len(p.Steps))
	for _, s := range p.Steps {
		if s.ID != "" {
			steps = append(steps, s.ID)
		}
	}
	return steps
}

func extractProtocolName(req LearnRequest) string {
	if req.Plan != nil && req.Plan.ID != "" {
		return "proto:plan:" + req.Plan.ID
	}
	return "proto:" + truncate(req.Verdict.SourceID, 32)
}

func extractTriggerFromVerdict(v workmodel.Verdict) string {
	if v.Kind == types.VerdictPartial || v.Kind == types.VerdictPass {
		return "on:" + v.SourceID
	}
	return ""
}

func extractTopic(v workmodel.Verdict) string {
	if v.SourceID != "" {
		return "topic:" + v.SourceID
	}
	return "topic:" + truncate(v.Reason, 32)
}

func extractHypothesis(v workmodel.Verdict) string {
	if v.Reason != "" {
		return v.Reason
	}
	return "hypothesis:" + string(v.Kind)
}

func extractConclusion(v workmodel.Verdict) string {
	if v.Reason == "" {
		return "conclusion:" + string(v.Kind)
	}
	return v.Reason
}

// ─────────────────────────────────────────────────────────────────────────
// Hashing & key formatting
// ─────────────────────────────────────────────────────────────────────────

// hashContentJSON returns the first 16 hex chars of SHA-256(content JSON).
// Stable for identical content (LP-4 幂等).
func hashContentJSON(content AssetContent) string {
	h := sha256.New()
	h.Write([]byte(content.SchemaVersion()))
	data, err := json.Marshal(content)
	if err != nil {
		h.Write([]byte(content.SchemaVersion()))
	} else {
		h.Write(data)
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// buildAssetKey formats the idempotency key as "{class}:{source}:{hash}".
// Re-Learning the same Verdict re-builds with the same key so Memory.Store
// replaces rather than duplicates.
func buildAssetKey(class LearningClass, sourceID, contentHash string) string {
	return strings.Join([]string{class.String(), sourceID, contentHash}, ":")
}

// ─────────────────────────────────────────────────────────────────────────
// Class routing helpers
// ─────────────────────────────────────────────────────────────────────────

// classFromVerdictKind maps a VerdictKind to its corresponding LearningClass
// for AssetBuilder routing. Matches the doc 45 §4.6 source-verdict mapping.
//
// VerdictKind → LearningClass mapping:
//   VerdictPass         → LearningSOP          (★5, deterministic)
//   VerdictPartial      → LearningProtocol     (★4, idempotent multi-step)
//   VerdictFail         → LearningKnowledge    (★3, hypothesis+evidence)
//   VerdictIndeterminate → LearningPending     (★1, deferred)
//
// Note: StatisticalVerdict (→LearningConclusion) is not a single VerdictKind;
// in production it is one of the four sub-aspects under a ComplianceVerdict
// wrapper. When Kind == VerdictFail and Reason contains "statistical", route
// to LearningConclusion instead. This is a pragmatic shortcut for PR-E5;
// full multi-aspect aggregation is owned by Phase 4 PR-D1.
func classFromVerdictKind(kind types.VerdictKind, reason string) LearningClass {
	switch kind {
	case types.VerdictPass:
		return LearningClass(types.LearningSOP)
	case types.VerdictPartial:
		return LearningClass(types.LearningProtocol)
	case types.VerdictFail:
		if strings.Contains(strings.ToLower(reason), "statistical") {
			return LearningClass(types.LearningConclusion)
		}
		return LearningClass(types.LearningKnowledge)
	case types.VerdictIndeterminate:
		return LearningClass(types.LearningPending)
	default:
		return LearningClass(types.LearningUnknown)
	}
}

// ClassFromVerdictKind is the public version of classFromVerdictKind. The
// learn.DefaultLearner needs to pre-compute the routing class before calling
// AssetBuilder.Build (to short-circuit unknown kinds early). Keeping the
// public version here avoids duplicating the switch in two packages.
func ClassFromVerdictKind(kind types.VerdictKind, reason string) LearningClass {
	return classFromVerdictKind(kind, reason)
}

// ─────────────────────────────────────────────────────────────────────────
// Utility helpers
// ─────────────────────────────────────────────────────────────────────────

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func clampConfidence(c float64) float64 {
	if c < 0 {
		return 0
	}
	if c > 1 {
		return 1
	}
	return c
}