package interfaces

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	sharederrors "github.com/devrix/devrix/internal/shared/errors"
)

// IntentSegment is the per-segment decomposition of a directive produced by
// Observe (DM-20260707-001 PR-A1 T01). One directive yields N segments; each
// segment becomes one child WorkItem.
//
// Placement: this type lives in the `interfaces` package (not in `orchtypes`)
// because plan/ plan_struct.go needs to embed the segment container, and the
// orchtypes → mups/learn/asset → plan import cycle otherwise prevents a clean
// reference. The interfaces package is intentionally dependency-free for this
// kind of cross-domain type (see interfaces/doc.go).
//
// Boundaries:
//   - Validate() — covers kind enum + priority/confidence range + non-empty Text.
//     This grammar-level file does NOT descend into DAG semantics; that lives
//     in plan/dag_validator.go (cross-package boundary explicitly noted).
//   - Default values: Priority=50, Confidence=0.5 (matches spec_delta §1).
//   - IntentSegmentKind is the per-segment task class; it is distinct from
//     orchtypes.IntentKind (which is the routing decision at the directive
//     level: fast/command/orchestrate/skip).
//
// Boundaries:
//   - Validate() — covers kind enum + priority/confidence range + non-empty Text.
//     This grammar-level file does NOT descend into DAG semantics; that lives
//     in plan/dag_validator.go (cross-package boundary explicitly noted).
//   - Default values: Priority=50, Confidence=0.5 (matches spec_delta §1).
//   - IntentSegmentKind is the per-segment task class; it is distinct from
//     orchtypes.IntentKind (which is the routing decision at the directive
//     level: fast/command/orchestrate/skip). Adding a new value requires:
//     (1) update IsKnownIntentSegmentKind, (2) extend plan.go PlanKind mapping
//     if it changes routing semantics, (3) update Phase 6 PR-F1 comments.
type IntentSegment struct {
	ID         string           `json:"id"`
	Text       string           `json:"text"`
	Kind       IntentSegmentKind `json:"kind"`
	Priority   int              `json:"priority"`   // [0, 100], default 50
	Confidence float64          `json:"confidence"` // [0, 1],  default 0.5
}

// IntentSegmentKind enumerates the per-segment task class. Deliberately a
// distinct type from orchtypes.IntentKind (routing decision at directive
// level: fast/command/orchestrate/skip) because the per-segment signal
// carries semantics — multi-intent decomposition maps to per-segment
// classification, not to a re-summarized directive-level routing choice.
//
// Phase 6 IntentClassifier (DM-20260624-001) emits one of these per segment
// at the multi-intent decomposition layer added in DM-20260707-001.
type IntentSegmentKind string

const (
	// IntentSegmentKindDeterministic — pure fact query; eligible for Observe
	// fast-path per DM-20260706-011. Examples: "1+1=几", "法国首都是哪".
	IntentSegmentKindDeterministic IntentSegmentKind = "deterministic"

	// IntentSegmentKindExplore — read-only probe; maps to ScenarioPlan
	// (Phase 2 PR-B1). Examples: "查 devrix 项目结构".
	IntentSegmentKindExplore IntentSegmentKind = "explore"

	// IntentSegmentKindCommit — single-step mutation; maps to CommitmentPlan.
	// Examples: "deploy this build".
	IntentSegmentKindCommit IntentSegmentKind = "commit"

	// IntentSegmentKindAnalyze — measurement / experiment; maps to
	// ExplorationPlan. Examples: "评估 v7 演进风险".
	IntentSegmentKindAnalyze IntentSegmentKind = "analyze"
)

// IsKnownIntentSegmentKind reports whether k belongs to the 4-value enum.
// Used by Validate() and by Phase 6 IntentClassifier wiring tests.
func IsKnownIntentSegmentKind(k IntentSegmentKind) bool {
	switch k {
	case IntentSegmentKindDeterministic, IntentSegmentKindExplore,
		IntentSegmentKindCommit, IntentSegmentKindAnalyze:
		return true
	}
	return false
}

// IntentSegmentSet is the container produced by Observe (DM-20260707-001 PR-A1
// T02). SourceDirective is preserved verbatim for audit and reverse-trace;
// DetectedAt is set at Observe time so downstream phases (Plan/Execute) can
// pin segment age.
//
// Invariant: len(Segments) >= 1 (empty set is not a valid state — Observe
// either emits ≥1 segment or falls back to the original 4-channel plan path).
type IntentSegmentSet struct {
	Segments        []IntentSegment `json:"segments"`
	SourceDirective string          `json:"source_directive"`
	DetectedAt      time.Time       `json:"detected_at"`
}

// NewIntentSegment constructs a segment with default Priority/Confidence
// pre-filled. Caller may override via field assignment or the segment is
// ready for Validate as-is.
//
// emptyIntentSegmentID and emptyIntentSegmentText are rejected by Validate;
// callers are expected to fill ID + Text before passing to downstream
// consumers.
func NewIntentSegment(id, text string, kind IntentSegmentKind) IntentSegment {
	return IntentSegment{
		ID:         id,
		Text:       text,
		Kind:       kind,
		Priority:   50,
		Confidence: 0.5,
	}
}

// NewIntentSegmentSet wraps an Observe-emitted segment list into the
// container. len(segments) must be ≥ 1; if empty, Validate() returns
// ErrIntentSegmentSetEmpty.
func NewIntentSegmentSet(sourceDirective string, detectedAt time.Time, segments []IntentSegment) IntentSegmentSet {
	return IntentSegmentSet{
		Segments:        segments,
		SourceDirective: sourceDirective,
		DetectedAt:      detectedAt,
	}
}

// Validate enforces the per-segment invariants declared in spec_delta §1.
// It is intentionally a grammar check only — semantic checks (e.g.,
// "segment references a plan node id") live in the consumer.
//
// Boundary: Validate does NOT cross into any DAG validation. The DAG
// validator at plan/dag_validator.go owns PlanDAG semantics. A Validate
// pass here does not imply the DAG is well-formed.
func (s *IntentSegment) Validate() error {
	if s.ID == "" {
		return NewIntentSegmentInvalidIDError()
	}
	if s.Text == "" {
		return NewIntentSegmentInvalidTextError()
	}
	if !IsKnownIntentSegmentKind(s.Kind) {
		return NewIntentSegmentInvalidKindError(string(s.Kind))
	}
	if s.Priority < 0 || s.Priority > 100 {
		return NewIntentSegmentInvalidPriorityError(s.Priority)
	}
	if s.Confidence < 0 || s.Confidence > 1 {
		return NewIntentSegmentInvalidConfidenceError(s.Confidence)
	}
	return nil
}

// Validate enforces IntentSegmentSet invariants: ≥1 segment AND every
// segment passes Validate(). SourceDirective empty policy (per codex
// consensus 2026-07-07): logs slog.Warn but does NOT return an error — the
// field is observability, not a correctness gate at this layer. A future
// PR-B (DAG executor) hook may escalate to error if down-stream consumers
// require the field.
func (s *IntentSegmentSet) Validate() error {
	if len(s.Segments) == 0 {
		return NewIntentSegmentSetEmptyError()
	}
	if s.SourceDirective == "" {
		slog.Warn(
			"intent_segment_set_empty_source_directive",
			"segment_count", len(s.Segments),
			"detected_at", s.DetectedAt,
		)
	}
	for i := range s.Segments {
		if err := s.Segments[i].Validate(); err != nil {
			return fmt.Errorf("intent_segment_set: segment[%d]: %w", i, err)
		}
	}
	return nil
}

// Sentinel errors for IntentSegment / IntentSegmentSet. They follow the
// existing orchtypes sentinel pattern (orchtypes/errors.go): inner errors
// returned unwrapped, wrap helpers expose canonical ORCH_*_7xxx codes for
// upstream consumers.
//
// Code range notes (devrix convention, see orchtypes/errors.go):
//   - 70xx was reserved for the original 4 sentinels (7001-7004 already used).
//   - 7114-7119 reserved for PR-A1 IntentSegment sentinels.
//   - 7120-7122 reserved for PR-A2 IntentSegmenter sentinels.
//   - 72xx reserved for PR-A1 PlanDAG sentinels (see plan/dag_validator.go).
var (
	ErrIntentSegmentInvalidID        = errors.New("orchtypes: IntentSegment.ID is empty")
	ErrIntentSegmentInvalidText      = errors.New("orchtypes: IntentSegment.Text is empty")
	ErrIntentSegmentInvalidKind      = errors.New("orchtypes: IntentSegment.IntentKind is not in the 4-value enum")
	ErrIntentSegmentInvalidPriority  = errors.New("orchtypes: IntentSegment.Priority out of [0, 100]")
	ErrIntentSegmentInvalidConfidence = errors.New("orchtypes: IntentSegment.Confidence out of [0, 1]")

	ErrIntentSegmentSetEmpty = errors.New("orchtypes: IntentSegmentSet requires len(Segments) >= 1")

	// PR-A2 sentinels (7120-7122). These surface IntentSegmenter-specific
	// failures (LLM timeout, malformed response, empty production). They are
	// NOT returned by IntentSegment.Validate() — that grammar check is owned
	// by 7114-7119. These are wired through SegmenterDispatcher.Segment().

	// ErrIntentSegmenterLLMTimeout — LLM call exceeded the configured budget
	// (default 800ms). Dispatcher falls back to RuleBased. Never bubbles up
	// to the caller unless RuleBased also fails.
	ErrIntentSegmenterLLMTimeout = errors.New("orchtypes: IntentSegmenter LLM call exceeded timeout budget")

	// ErrIntentSegmenterLLMInvalidResponse — LLM returned text that failed
	// JSON parsing (no JSON object/array detected, or ParseWholeBody
	// rejected). Dispatcher falls back to RuleBased.
	ErrIntentSegmenterLLMInvalidResponse = errors.New("orchtypes: IntentSegmenter LLM returned unparseable response")

	// ErrIntentSegmenterNoSegment — Segmenter exhausted LLM and RuleBased
	// paths without producing ≥1 segment. Should not happen in practice
	// (RuleBased always returns ≥1 lazy fallback), but guards against
	// future implementations that might return empty sets.
	ErrIntentSegmenterNoSegment = errors.New("orchtypes: IntentSegmenter produced 0 segments (invariant violation)")
)

// Wrap helpers — emit *sharederrors.SentinelError with stable ORCH_*_71xx
// codes so upstream audit logs and metrics (D7.<scenario>.<event>) can grep
// without parse-fishing on inner error strings.
//
// Code allocation (PR-A1, see reviews/pr-a1-consensus-packet.md):
//   ORCH_INTENT_SEGMENT_ID_7114
//   ORCH_INTENT_SEGMENT_TEXT_7115
//   ORCH_INTENT_SEGMENT_KIND_7116
//   ORCH_INTENT_SEGMENT_PRIORITY_7117
//   ORCH_INTENT_SEGMENT_CONFIDENCE_7118
//   ORCH_INTENT_SET_EMPTY_7119
//
// PR-A2 (see reviews/pr-a2-codex-consensus-2026-07-07.md):
//   ORCH_INTENT_SEGMENTER_LLM_TIMEOUT_7120
//   ORCH_INTENT_SEGMENTER_LLM_INVALID_7121
//   ORCH_INTENT_SEGMENTER_NO_SEGMENT_7122
func NewIntentSegmentInvalidIDError() *sharederrors.SentinelError {
	return sharederrors.WithCode(
		"ORCH_INTENT_SEGMENT_ID_7114",
		"IntentSegment.ID cannot be empty",
		ErrIntentSegmentInvalidID,
	)
}

func NewIntentSegmentInvalidTextError() *sharederrors.SentinelError {
	return sharederrors.WithCode(
		"ORCH_INTENT_SEGMENT_TEXT_7115",
		"IntentSegment.Text cannot be empty",
		ErrIntentSegmentInvalidText,
	)
}

func NewIntentSegmentInvalidKindError(kind string) *sharederrors.SentinelError {
	return sharederrors.WithCode(
		"ORCH_INTENT_SEGMENT_KIND_7116",
		fmt.Sprintf("IntentSegment.IntentKind %q is not in {deterministic, explore, commit, analyze}", kind),
		ErrIntentSegmentInvalidKind,
	)
}

func NewIntentSegmentInvalidPriorityError(p int) *sharederrors.SentinelError {
	return sharederrors.WithCode(
		"ORCH_INTENT_SEGMENT_PRIORITY_7117",
		fmt.Sprintf("IntentSegment.Priority=%d out of [0, 100]", p),
		ErrIntentSegmentInvalidPriority,
	)
}

func NewIntentSegmentInvalidConfidenceError(c float64) *sharederrors.SentinelError {
	return sharederrors.WithCode(
		"ORCH_INTENT_SEGMENT_CONFIDENCE_7118",
		fmt.Sprintf("IntentSegment.Confidence=%.3f out of [0, 1]", c),
		ErrIntentSegmentInvalidConfidence,
	)
}

func NewIntentSegmentSetEmptyError() *sharederrors.SentinelError {
	return sharederrors.WithCode(
		"ORCH_INTENT_SET_EMPTY_7119",
		"IntentSegmentSet requires at least one segment",
		ErrIntentSegmentSetEmpty,
	)
}

// PR-A2 Segmenter sentinels (7120-7122). Wrap helpers live here, next to
// the inner errors, so all 9 IntentSegment-related sentinels share one
// audit-trail package.
func NewIntentSegmenterLLMTimeoutError(elapsedMs int64, budgetMs int64) *sharederrors.SentinelError {
	return sharederrors.WithCode(
		"ORCH_INTENT_SEGMENTER_LLM_TIMEOUT_7120",
		fmt.Sprintf("IntentSegmenter LLM call timed out: elapsed=%dms budget=%dms", elapsedMs, budgetMs),
		fmt.Errorf("%w: elapsed=%dms budget=%dms", ErrIntentSegmenterLLMTimeout, elapsedMs, budgetMs),
	)
}

func NewIntentSegmenterLLMInvalidResponseError(snippet string) *sharederrors.SentinelError {
	return sharederrors.WithCode(
		"ORCH_INTENT_SEGMENTER_LLM_INVALID_7121",
		fmt.Sprintf("IntentSegmenter LLM returned unparseable response (snippet=%q)", snippet),
		fmt.Errorf("%w: snippet=%q", ErrIntentSegmenterLLMInvalidResponse, snippet),
	)
}

func NewIntentSegmenterNoSegmentError() *sharederrors.SentinelError {
	return sharederrors.WithCode(
		"ORCH_INTENT_SEGMENTER_NO_SEGMENT_7122",
		"IntentSegmenter produced 0 segments (invariant violation)",
		ErrIntentSegmenterNoSegment,
	)
}

// IsIntentSegmenterNoSegmentError reports whether err is (or wraps) the
// 7122 no-segment sentinel. Used by SegmenterDispatcher for log/audit
// dispatching (PR-A2 Q5: classify LLM error reason for slog/metric).
func IsIntentSegmenterNoSegmentError(err error) bool {
	return err != nil && (errors.Is(err, ErrIntentSegmenterNoSegment))
}
