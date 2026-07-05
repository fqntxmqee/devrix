// Package interfaces — ResolutionContract (DM-20260704-006, S4 Phase 1).
//
// ResolutionContract is the cross-MUPS-node data shape that closes the two
// break-chains surfaced by trace c6f2d6910496e2ea63cbcf8f207b2c0a (sess_1783239758810_0):
//
//   - Break-chain A (Obs→Resolution): Plan does not declare a resolution
//     strategy for each ObsUncertainty, Execute mixes answers in prose,
//     and Verify scrapes text with regex. The contract makes the strategy
//     first-class on Plan and forces Execute to emit a structured claim
//     per ObsID.
//
//   - Break-chain B (Plan→Decide): Plan LLM emits `execution_mode:
//     "decompose"` + `child_specs[]` but Decide's SpawnDecompose decision
//     never reads ChildSpecs — it picks SpawnInline based on
//     deliverable-incomplete signals. ResolutionStrategy.SubWorktree
//     binds the decompose decision to the ObsID it resolves, and Decide
//     reads HasSubWorktree=true to force SpawnDecompose regardless of
//     deliverable state.
//
// The 5 types and their roles:
//
//   - ResolutionStrategy    (Plan → Decide): per-ObsID plan for how this
//     Execute round will resolve the uncertainty.
//   - SubWorktreeSpec       (Plan → Decide → Decompose): the optional
//     child WorkItem spec that Decide uses to force SpawnDecompose when
//     resolution requires a sibling investigation.
//   - ResolutionClaim       (Execute → Verify): per-ObsID structured
//     answer that Verify compares against ResolutionStrategy to compute
//     CoverageRatio.
//   - ResolutionReport      (Verify → Decide): CoverageRatio +
//     UnresolvedObs[] emitted at the end of Verify so Decide can
//     route the next round without re-scanning artifact prose.
//   - UnresolvedObs         (Report sub-record): one ObsID the round
//     failed to resolve, with Reason + HasSubWorktree flag Decide reads.
//
// All five types are immutable value objects — mutators (With*) return
// copies; constructors never mutate inputs. This mirrors the convention
// used by TaskSpec / TaskReport (PR-A, devrix-d7-taskcontract-unification)
// and is required so ResolutionReport can be cached, marshaled, and
// passed across goroutine boundaries without aliasing surprises.
//
// This file lives in the interfaces package (leaf, no imports of
// plan/workmodel/mups) because orchtypes already imports mups/learn,
// which transitively imports plan — adding plan→orchtypes creates the
// import cycle resolved by promoting the contract here. The pattern is
// the same one used by TaskSpec/TaskReport in PR-A.
package interfaces

import (
	"errors"
	"fmt"
	"time"

	sharederrors "github.com/devrix/devrix/internal/shared/errors"
)

// ResolutionConfidenceMin is the lower bound below which a Claim is treated
// as UnresolvedObs (Reason = "low_confidence"). 0.7 matches the Phase 4
// Verifier confidence threshold; keep them aligned so the Verify decision
// table is internally consistent.
const ResolutionConfidenceMin = 0.7

// DefaultUnresolvedStrengthThreshold is the strength above which the
// Decide layer triggers SpawnUserGate (RC-4b) when no sub_worktree is
// available. Configurable at runtime; this is the cold-start default.
const DefaultUnresolvedStrengthThreshold = 0.85

// ResolutionReason enumerates the reasons an ObsID can appear in
// ResolutionReport.UnresolvedObs. Stable wire format (snake_case) so
// downstream dashboards can filter by reason without parsing prose.
type ResolutionReason string

const (
	// ResolutionReasonNoClaim: Plan emitted a ResolutionStrategy for this
	// ObsID but Execute did not emit a matching ResolutionClaim.
	ResolutionReasonNoClaim ResolutionReason = "no_resolution_claim"

	// ResolutionReasonLowConfidence: Claim emitted but Confidence <
	// ResolutionConfidenceMin.
	ResolutionReasonLowConfidence ResolutionReason = "low_confidence"

	// ResolutionReasonEvidenceMissing: Claim emitted but SupportingEvidence
	// is empty (i.e. the LLM guessed without backing).
	ResolutionReasonEvidenceMissing ResolutionReason = "evidence_missing"

	// ResolutionReasonNoStrategy: Execute emitted a Claim but no Plan
	// ResolutionStrategy bound to this ObsID. Treated as over-reporting
	// (LLM fabricated a claim for an unsourced ObsID); downstream Decide
	// ignores the claim for spawn-decision purposes.
	ResolutionReasonNoStrategy ResolutionReason = "no_resolution_strategy"
)

// ResolutionStrategy binds one ObsID to the planned resolution path for
// this MUPS round. Plan emits ResolutionStrategy[] as part of its
// artifact; Decide reads HasSubWorktree to decide SpawnDecompose vs
// SpawnInline vs SpawnUserGate.
//
// ObsID must match an Observation.ID from the upstream Observe round.
// PlannedTool is the tool name the Execute round is expected to invoke.
// SuccessCriterion is a free-text predicate the Verify layer can use as
// a fallback when the LLM Verifier abstains.
//
// SubWorktree is optional. When non-nil, it tells Decide that resolving
// this ObsID requires spawning a sibling child WorkItem — this is the
// RC-4a hook that closes break-chain B (decouple from deliverable
// incomplete signal).
type ResolutionStrategy struct {
	// ObsID is the upstream Observation.ID this strategy resolves.
	ObsID string `json:"obs_id"`

	// PlannedTool is the tool name the Execute round is expected to use.
	// Empty string means "tool-agnostic; Execute decides". Verify uses
	// this as a soft hint, not a hard check.
	PlannedTool string `json:"planned_tool,omitempty"`

	// SuccessCriterion is a free-text predicate ("file_count > 0" /
	// "exit_code == 0") that Verify can use as a fallback. Empty means
	// "LLM Verifier decides".
	SuccessCriterion string `json:"success_criterion,omitempty"`

	// SubWorktree, when non-nil, indicates this strategy requires a
	// sibling child WorkItem. Decide reads the non-nil pointer to
	// trigger SpawnDecompose (RC-4a) regardless of deliverable state.
	SubWorktree *SubWorktreeSpec `json:"sub_worktree,omitempty"`
}

// HasSubWorktree reports whether the strategy declares a SubWorktree.
// This is the single boolean Decide reads to close break-chain B.
func (r ResolutionStrategy) HasSubWorktree() bool {
	return r != (ResolutionStrategy{}) && r.SubWorktree != nil
}

// SubWorktreeSpec describes the child WorkItem to spawn for one ObsID.
// Mirrors the field set of workmodel.ChildSpec but stays in the
// interfaces leaf package so Plan → Decide wiring does not need to
// import workmodel (avoiding a cycle once DecisionPlanning→WorkModel
// direction is reversed by Phase 6 promotion).
//
// Title + DirectiveSuffix follow the Phase 2 PR-A1 PR-RF pattern: the
// directive handed to the child = base + "\n\n" + DirectiveSuffix. This
// preserves the existing `child_specs[]` directive-suffix semantics for
// callers that were relying on it.
type SubWorktreeSpec struct {
	Title           string   `json:"title"`
	DirectiveSuffix string   `json:"directive_suffix,omitempty"`
	ExpectedReturn  string   `json:"expected_return,omitempty"`
	ScopeIn         []string `json:"scope_in,omitempty"`
	PlannedTool     string   `json:"planned_tool,omitempty"`
}

// ResolutionClaim is one structured answer for one ObsID, emitted by the
// Execute node as part of its artifact. Verify compares Claims against
// Strategies to compute CoverageRatio and UnresolvedObs[].
//
// Answer is free text for now (per Open Q1 in proposal.md §10). If we
// later decide to migrate to structured JSON, the conversion happens at
// the Verify boundary — Claim stays free-text on the wire.
//
// Confidence in [0, 1]. Below ResolutionConfidenceMin the Claim is
// downgraded to UnresolvedObs with ResolutionReasonLowConfidence.
//
// SupportingEvidence is a non-empty string for Claims to count as
// "answered"; empty evidence downgrades to ResolutionReasonEvidenceMissing.
type ResolutionClaim struct {
	ObsID              string  `json:"obs_id"`
	Answer             string  `json:"answer"`
	Confidence         float64 `json:"confidence"`
	SupportingEvidence string  `json:"supporting_evidence,omitempty"`
}

// IsResolved reports whether this Claim should count as resolving its
// ObsID. Confidence threshold + non-empty evidence. Mirrors the
// conditions Verify uses when populating ResolutionReport so the
// in-process call sites and the persisted report agree.
func (c ResolutionClaim) IsResolved() bool {
	if c.ObsID == "" {
		return false
	}
	if c.Confidence < ResolutionConfidenceMin {
		return false
	}
	if c.Answer == "" {
		return false
	}
	return true
}

// UnresolvedObs describes one ObsID that the Verify round could not
// resolve. Decide reads this slice to pick SpawnDecompose (HasSubWorktree)
// or SpawnUserGate (MaxUnresolvedStrength >= threshold).
type UnresolvedObs struct {
	// ObsID is the upstream Observation.ID.
	ObsID string `json:"obs_id"`

	// Strength is the upstream Observation.Strength, copied so Decide
	// can compare against the user-gate threshold without re-reading the
	// originating Observation.
	Strength float64 `json:"strength"`

	// Reason is one of ResolutionReason*. Stable wire format.
	Reason ResolutionReason `json:"reason"`

	// HasSubWorktree mirrors ResolutionStrategy.SubWorktree != nil. Set
	// by Verify during the report build so Decide does not need to
	// look up the originating ResolutionStrategy.
	HasSubWorktree bool `json:"has_sub_worktree"`

	// SubWorktree (DM-20260704-006 Phase 4) carries the originating
	// ResolutionStrategy.SubWorktree verbatim so the Decide layer can
	// build a meaningful ChildSpec (Title + DirectiveSuffix +
	// ExpectedReturn + ScopeIn) without walking back to the upstream
	// Plan. Only set when HasSubWorktree=true; nil otherwise. Wire-
	// compatible with Phase 1/2/3 consumers that ignore the field.
	SubWorktree *SubWorktreeSpec `json:"sub_worktree,omitempty"`
}

// ResolutionReport is the cross-round handoff between Verify and Decide.
// TotalStrategies + TotalClaims + CoverageRatio let downstream dashboards
// surface "3/2/0.667" without parsing UnresolvedObs.
//
// CoverageRatio = TotalClaims / TotalStrategies when TotalStrategies > 0;
// 0 when no strategies were declared (the contract's "no plan" edge case).
type ResolutionReport struct {
	// SessionID + WorkItemID + RoundNo are diagnostic — they let the
	// trace log correlate the report with its owning round without
	// requiring the consumer to walk the call stack.
	SessionID  string `json:"session_id,omitempty"`
	WorkItemID string `json:"work_item_id,omitempty"`
	RoundNo    int    `json:"round_no,omitempty"`

	// TotalStrategies = len(strategies) at build time.
	TotalStrategies int `json:"total_strategies"`

	// TotalClaims = len(claims) at build time.
	TotalClaims int `json:"total_claims"`

	// CoverageRatio in [0, 1]. 0 when TotalStrategies == 0.
	CoverageRatio float64 `json:"coverage_ratio"`

	// UnresolvedObs is the slice Decide reads for SpawnDecompose vs
	// SpawnUserGate routing. Empty when the round was fully covered.
	UnresolvedObs []UnresolvedObs `json:"unresolved_obs"`

	// ComputedAt is wall-clock when Verify finished the report.
	ComputedAt time.Time `json:"computed_at"`
}

// MaxUnresolvedStrength returns the largest Strength across the report's
// UnresolvedObs. 0 when the slice is empty. Decide uses this to decide
// SpawnUserGate (>= threshold) when no SubWorktree is available.
func (r ResolutionReport) MaxUnresolvedStrength() float64 {
	if len(r.UnresolvedObs) == 0 {
		return 0
	}
	max := 0.0
	for _, uo := range r.UnresolvedObs {
		if uo.Strength > max {
			max = uo.Strength
		}
	}
	return max
}

// AnySubWorktreePending reports whether at least one UnresolvedObs has
// HasSubWorktree=true. Decide uses this as the RC-4a trigger (force
// SpawnDecompose regardless of deliverable state).
func (r ResolutionReport) AnySubWorktreePending() bool {
	for _, uo := range r.UnresolvedObs {
		if uo.HasSubWorktree {
			return true
		}
	}
	return false
}

// -----------------------------------------------------------------------------
// Constructors + With* methods (immutable value-object pattern)
// -----------------------------------------------------------------------------

// NewResolutionStrategy constructs a strategy without a SubWorktree.
// Use WithSubWorktree to attach one. PlannedTool and SuccessCriterion
// may be empty (tool-agnostic / LLM-Verifier-decides respectively).
func NewResolutionStrategy(obsID, plannedTool, successCriterion string) (ResolutionStrategy, error) {
	if obsID == "" {
		return ResolutionStrategy{}, ErrResolutionStrategyObsIDRequired
	}
	return ResolutionStrategy{
		ObsID:            obsID,
		PlannedTool:      plannedTool,
		SuccessCriterion: successCriterion,
	}, nil
}

// WithSubWorktree returns a copy of the strategy with the given
// SubWorktree attached. Pass nil to clear an existing SubWorktree.
// SubWorktreeSpec is validated for required fields (Title).
func (r ResolutionStrategy) WithSubWorktree(spec *SubWorktreeSpec) (ResolutionStrategy, error) {
	if spec == nil {
		r.SubWorktree = nil
		return r, nil
	}
	if err := spec.Validate(); err != nil {
		return r, err
	}
	r.SubWorktree = spec
	return r, nil
}

// Validate checks the strategy's invariants. Used by the Wire step at
// the Plan→Decide boundary and by unit tests.
func (r ResolutionStrategy) Validate() error {
	if r.ObsID == "" {
		return ErrResolutionStrategyObsIDRequired
	}
	if r.SubWorktree != nil {
		if err := r.SubWorktree.Validate(); err != nil {
			return fmt.Errorf("interfaces: ResolutionStrategy.SubWorktree: %w", err)
		}
	}
	return nil
}

// Validate checks the SubWorktreeSpec's invariants. Title is required;
// all other fields are optional (zero-value means "fall back to
// defaults at Decompose time").
func (s SubWorktreeSpec) Validate() error {
	if s.Title == "" {
		return ErrSubWorktreeSpecTitleRequired
	}
	return nil
}

// NewResolutionClaim constructs a claim. Confidence is clamped to [0,1].
// An empty Answer + non-zero Confidence is accepted on construction;
// Verify will downgrade it to UnresolvedObs with ResolutionReasonNoClaim
// (or ResolutionReasonEvidenceMissing when evidence is empty).
func NewResolutionClaim(obsID, answer, evidence string, confidence float64) (ResolutionClaim, error) {
	if obsID == "" {
		return ResolutionClaim{}, ErrResolutionClaimObsIDRequired
	}
	return ResolutionClaim{
		ObsID:              obsID,
		Answer:             answer,
		SupportingEvidence: evidence,
		Confidence:         clamp01Float(confidence, 0),
	}, nil
}

// Validate checks the claim's invariants. Confidence in [0,1]; ObsID
// required; Answer and SupportingEvidence are recommended but not
// required (Verify downgrades empty claims to UnresolvedObs).
func (c ResolutionClaim) Validate() error {
	if c.ObsID == "" {
		return ErrResolutionClaimObsIDRequired
	}
	if c.Confidence < 0 || c.Confidence > 1 {
		return NewResolutionClaimConfidenceOutOfRangeError(c.Confidence)
	}
	return nil
}

// NewResolutionReport builds a report from the strategies + claims
// emitted this round. The constructor is the single place that computes
// CoverageRatio + UnresolvedObs so downstream callers can read the
// derived fields without re-implementing the logic.
//
// Algorithm (matches design.md §4 + spec RC-3):
//
//	1. For each strategy:
//	   - If no matching claim → UnresolvedObs{Reason: no_resolution_claim}
//	   - If claim.Confidence < threshold → UnresolvedObs{Reason: low_confidence}
//	   - If claim.SupportingEvidence empty → UnresolvedObs{Reason: evidence_missing}
//	   - Otherwise: counts as resolved.
//	2. For each claim with no matching strategy → UnresolvedObs{Reason: no_resolution_strategy}
//	3. CoverageRatio = resolved / len(strategies) when len(strategies) > 0, else 0.
//	4. HasSubWorktree is set from the strategy's SubWorktree field.
func NewResolutionReport(sessionID, workItemID string, roundNo int, strategies []ResolutionStrategy, claims []ResolutionClaim) (ResolutionReport, error) {
	if sessionID == "" {
		return ResolutionReport{}, ErrResolutionReportSessionIDRequired
	}
	if workItemID == "" {
		return ResolutionReport{}, ErrResolutionReportWorkItemIDRequired
	}
	if roundNo < 0 {
		return ResolutionReport{}, ErrResolutionReportRoundNoNegative
	}
	// Validate inputs before computing so callers learn about malformed
	// strategies/claims immediately rather than getting a report that
	// silently drops them.
	for i := range strategies {
		if err := strategies[i].Validate(); err != nil {
			return ResolutionReport{}, fmt.Errorf("interfaces: strategies[%d]: %w", i, err)
		}
	}
	for i := range claims {
		if err := claims[i].Validate(); err != nil {
			return ResolutionReport{}, fmt.Errorf("interfaces: claims[%d]: %w", i, err)
		}
	}

	var unresolved []UnresolvedObs
	resolved := 0
	for _, s := range strategies {
		c, hasClaim := lookupClaim(claims, s.ObsID)
		if !hasClaim {
			unresolved = append(unresolved, UnresolvedObs{
				ObsID:          s.ObsID,
				Strength:       extractObsStrength(s.ObsID),
				Reason:         ResolutionReasonNoClaim,
				HasSubWorktree: s.HasSubWorktree(),
				SubWorktree:    s.SubWorktree,
			})
			continue
		}
		// Check order matches ResolutionClaim.IsResolved() so the report
		// build is the single source of truth for "is this obs resolved".
		// A Claim with empty Answer + high Confidence + evidence would
		// otherwise count as resolved here while IsResolved() returns
		// false — that drift was caught in S4 Phase 2 review.
		if c.Answer == "" {
			unresolved = append(unresolved, UnresolvedObs{
				ObsID:          s.ObsID,
				Strength:       extractObsStrength(s.ObsID),
				Reason:         ResolutionReasonNoClaim,
				HasSubWorktree: s.HasSubWorktree(),
				SubWorktree:    s.SubWorktree,
			})
			continue
		}
		if c.Confidence < ResolutionConfidenceMin {
			unresolved = append(unresolved, UnresolvedObs{
				ObsID:          s.ObsID,
				Strength:       extractObsStrength(s.ObsID),
				Reason:         ResolutionReasonLowConfidence,
				HasSubWorktree: s.HasSubWorktree(),
				SubWorktree:    s.SubWorktree,
			})
			continue
		}
		if c.SupportingEvidence == "" {
			unresolved = append(unresolved, UnresolvedObs{
				ObsID:          s.ObsID,
				Strength:       extractObsStrength(s.ObsID),
				Reason:         ResolutionReasonEvidenceMissing,
				HasSubWorktree: s.HasSubWorktree(),
				SubWorktree:    s.SubWorktree,
			})
			continue
		}
		resolved++
	}

	// Over-reported claims (no matching strategy) → UnresolvedObs.
	seen := make(map[string]struct{}, len(strategies))
	for _, s := range strategies {
		seen[s.ObsID] = struct{}{}
	}
	for _, c := range claims {
		if _, ok := seen[c.ObsID]; ok {
			continue
		}
		unresolved = append(unresolved, UnresolvedObs{
			ObsID:    c.ObsID,
			Strength: extractObsStrength(c.ObsID),
			Reason:   ResolutionReasonNoStrategy,
		})
	}

	ratio := 0.0
	if len(strategies) > 0 {
		ratio = float64(resolved) / float64(len(strategies))
	}

	return ResolutionReport{
		SessionID:       sessionID,
		WorkItemID:      workItemID,
		RoundNo:         roundNo,
		TotalStrategies: len(strategies),
		TotalClaims:     len(claims),
		CoverageRatio:   ratio,
		UnresolvedObs:   unresolved,
		ComputedAt:      time.Now(),
	}, nil
}

// Validate checks the report's invariants. Used after JSON unmarshal
// to catch malformed persisted reports.
func (r ResolutionReport) Validate() error {
	if r.SessionID == "" {
		return ErrResolutionReportSessionIDRequired
	}
	if r.WorkItemID == "" {
		return ErrResolutionReportWorkItemIDRequired
	}
	if r.RoundNo < 0 {
		return ErrResolutionReportRoundNoNegative
	}
	if r.CoverageRatio < 0 || r.CoverageRatio > 1 {
		return NewResolutionReportCoverageRatioOutOfRangeError(r.CoverageRatio)
	}
	return nil
}

// lookupClaim is a small linear-scan helper. Claims are typically a
// short list (per-ObsID answers), so the O(n) cost is negligible
// compared to the surrounding Verify round.
func lookupClaim(claims []ResolutionClaim, obsID string) (ResolutionClaim, bool) {
	for _, c := range claims {
		if c.ObsID == obsID {
			return c, true
		}
	}
	return ResolutionClaim{}, false
}

// extractObsStrength is a placeholder for Phase 5 wiring that will
// look up the originating Observation by ObsID to copy its Strength
// into UnresolvedObs. In Phase 1 we cannot reach into the Observe
// layer without a circular import, so we default to 0.0 and rely on
// the caller (Verify layer) to populate the field from its own
// Observation cache before calling NewResolutionReport.
//
// Phase 5 wiring note: when this is wired, the signature should
// change to accept an `obsByID map[string]Observation` parameter so
// the function is pure. Marking here so the eventual caller knows.
func extractObsStrength(_ string) float64 {
	return 0.0
}

// clamp01Float clamps v to [0,1]. When v is NaN it returns onNaN so
// the caller can choose a sensible cold-start value (e.g. 0.5 for
// coord-style "neutral uncertainty", 0 for hard thresholds like
// Confidence). Mirrors orchtypes.clamp01Float (intentionally not
// exported there to keep orchtypes as the package boundary).
func clamp01Float(v float64, onNaN float64) float64 {
	if v != v { // NaN check without importing math
		return onNaN
	}
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// -----------------------------------------------------------------------------
// Sentinels (DM-20260704-006 codes 7010-7019 reserved for ResolutionContract)
// -----------------------------------------------------------------------------

var (
	ErrResolutionStrategyObsIDRequired         = errors.New("interfaces: ResolutionStrategy ObsID required")
	ErrResolutionClaimObsIDRequired            = errors.New("interfaces: ResolutionClaim ObsID required")
	ErrResolutionClaimConfidenceOutOfRange     = errors.New("interfaces: ResolutionClaim confidence out of [0,1]")
	ErrSubWorktreeSpecTitleRequired            = errors.New("interfaces: SubWorktreeSpec Title required")
	ErrResolutionReportSessionIDRequired       = errors.New("interfaces: ResolutionReport SessionID required")
	ErrResolutionReportWorkItemIDRequired      = errors.New("interfaces: ResolutionReport WorkItemID required")
	ErrResolutionReportRoundNoNegative         = errors.New("interfaces: ResolutionReport RoundNo negative")
	ErrResolutionReportCoverageRatioOutOfRange = errors.New("interfaces: ResolutionReport CoverageRatio out of [0,1]")
)

func NewResolutionClaimConfidenceOutOfRangeError(c float64) *sharederrors.SentinelError {
	return sharederrors.WithCode(
		"ORCH_RES_CLAIM_CONF_7010",
		fmt.Sprintf("resolution claim confidence %.3f out of [0,1]", c),
		ErrResolutionClaimConfidenceOutOfRange,
	)
}

func NewResolutionReportCoverageRatioOutOfRangeError(r float64) *sharederrors.SentinelError {
	return sharederrors.WithCode(
		"ORCH_RES_REPORT_RATIO_7011",
		fmt.Sprintf("resolution report coverage ratio %.3f out of [0,1]", r),
		ErrResolutionReportCoverageRatioOutOfRange,
	)
}