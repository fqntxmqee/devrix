package plan

import (
	"encoding/json"
	"fmt"
	"time"

	ifaces "github.com/devrix/devrix/internal/layers/orchestration/interfaces"
	sharederrors "github.com/devrix/devrix/internal/shared/errors"
)

// Plan is the structured output of the Plan node (MUPS v4.3 Phase 2 PR-B1),
// extended for multi-intent decomposition (DM-20260707-001 PR-A1 T03).
//
// One Plan per user message; consumed by Phase 3 Execute node which routes
// to one of 4 channels based on Plan.Kind.
//
// Constructed via DefaultPlanner.Plan() or via direct literal in tests —
// Validate() enforces PP-1/2/3 before the Plan can be dispatched.
//
// Multi-intent extension (PR-A1, DM-20260707-001 方案 β):
//   - IntentSegmentSet *ifaces.IntentSegmentSet      → multi-segment directive
//   - DAG             *PlanDAG                       → segment-level parallel execution
//
// These two optional fields carry the multi-intent semantics; both nil
// preserves backward compatibility with the Phase 2 PR-B1 4-channel path.
//
// Boundary note: Plan.Validate() does NOT descend into IntentSegmentSet / DAG
// semantics. The DAG validator at plan/dag_validator.go owns PlanDAG checks;
// IntentSegment.Validate() owns its own grammar. Validate here is strictly
// the PP-1/2/3 + structural checks inherited from Phase 2 PR-B1.
type Plan struct {
	ID                   string             `json:"id"`
	SessionID            string             `json:"session_id,omitempty"`
	Kind                 PlanKind           `json:"kind"`
	Strength             float64            `json:"strength"`
	Steps                []Step             `json:"steps"`
	FailureCriteria      []FailureCriterion `json:"failure_criteria"`
	BlastRadius          BlastRadius        `json:"blast_radius"`
	SourceObservationIDs []string           `json:"source_observation_ids"`
	AnomaliesCount       int                `json:"anomalies_count,omitempty"`
	// ResolutionStrategies (DM-20260704-006) — the per-ObsID resolution
	// contract that closes break-chain A (Obs→Resolution). Optional; when
	// non-empty, the LLM Verifier cross-checks ResolutionClaim[] from
	// Execute against these strategies to compute CoverageRatio. Any
	// strategy whose SubWorktree is non-nil triggers SpawnDecompose via
	// RC-4a (break-chain B closure).
	ResolutionStrategies []ifaces.ResolutionStrategy `json:"resolution_strategies,omitempty"`

	// IntentSegmentSet (DM-20260707-001 PR-A1 T03) — produced by Observe when
	// the directive yields ≥2 segments. Triggers the multi-intent path. The
	// Plan is then consumed by the DAG executor (PR-B) which routes each
	// segment to RunPlanDAG. SpawnPolicy remains the existing 3-value enum
	// (workmodel/pipeline_round.go:27-34) — this field carries its semantics.
	IntentSegmentSet *ifaces.IntentSegmentSet `json:"intent_segment_set,omitempty"`

	// DAG (DM-20260707-001 PR-A1 T03) — the per-segment execution graph. nil
	// when Plan falls back to the 4-channel Phase 2 PR-B1 path. Validity is
	// enforced by plan/dag_validator.go; Plan.Validate() does not check it.
	DAG *PlanDAG `json:"dag,omitempty"`

	// Metadata (DM-20260707-001 PR-F T71) — generic string→string bag for
	// routing hints that don't fit any of the typed fields above. The
	// canonical key is "force_plan" (set via InjectForcePlanHint) which the
	// next Execute round reads via ShouldForcePlanFromPlan to bypass the
	// observational fast-path. Plan.Validate() does not check Metadata —
	// callers are responsible for not colliding with reserved keys.
	Metadata map[string]string `json:"metadata,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	// FailureCriteriaOpWhitelist and ObservableFailureCriterionFields are
	// package-level; Validate() reads them.
}

// NewPlan constructs a Plan with the given required fields. Use this rather
// than struct literals in production code so that CreatedAt is always set and
// Kind defaults to KindUnset (forcing the caller to classify before dispatch).
//
// Validate() must be called separately — NewPlan does NOT validate, allowing
// builders to construct incremental Plans.
func NewPlan(id, sessionID string, kind PlanKind, sourceObservationIDs []string, steps []Step, strength float64) *Plan {
	return &Plan{
		ID:                   id,
		SessionID:            sessionID,
		Kind:                 kind,
		Steps:                steps,
		Strength:             strength,
		SourceObservationIDs: append([]string(nil), sourceObservationIDs...),
		FailureCriteria:      []FailureCriterion{},
		BlastRadius:          BlastRadius{},
		CreatedAt:            time.Now(),
	}
}

// WithFailureCriteria returns a copy with the new FailureCriteria slice.
func (p Plan) WithFailureCriteria(fc []FailureCriterion) Plan {
	p.FailureCriteria = append([]FailureCriterion(nil), fc...)
	return p
}

// WithBlastRadius returns a copy with the new BlastRadius.
func (p Plan) WithBlastRadius(br BlastRadius) Plan {
	p.BlastRadius = br
	return p
}

// WithAnomaliesCount returns a copy with the new AnomaliesCount (from Phase 2
// UncertaintyReport.Anomalies; the Plan carries it forward so Phase 4 Verify
// can correlate observation anomalies with verification outcomes).
func (p Plan) WithAnomaliesCount(n int) Plan {
	p.AnomaliesCount = n
	return p
}

// WithResolutionStrategies returns a copy with the new ResolutionStrategies
// slice (DM-20260704-006). The slice is copied so callers can mutate the
// input without affecting the receiver (immutable value-object pattern).
func (p Plan) WithResolutionStrategies(strategies []ifaces.ResolutionStrategy) Plan {
	p.ResolutionStrategies = append([]ifaces.ResolutionStrategy(nil), strategies...)
	return p
}

// WithIntentSegmentSet returns a copy with the IntentSegmentSet set
// (DM-20260707-001 PR-A1 T03). Triggers the multi-intent path on the next
// Execute round; SpawnPolicy itself is untouched.
//
// Pass nil to clear the field (immutable builder semantics). The caller
// retains ownership of the underlying *ifaces.IntentSegmentSet.
func (p Plan) WithIntentSegmentSet(s *ifaces.IntentSegmentSet) Plan {
	if s == nil {
		p.IntentSegmentSet = nil
	} else {
		copy := *s
		p.IntentSegmentSet = &copy
	}
	return p
}

// WithDAG returns a copy with the DAG set (DM-20260707-001 PR-A1 T03).
// Triggers the multi-intent path on the next Execute round alongside
// IntentSegmentSet — both fields must be non-nil for the DAG executor to
// fire; the asymmetry is caught by PR-B at run time, not here.
//
// Deep-copies the slice/map fields (Nodes, Edges, Priorities) so callers
// can mutate the input PlanDAG without affecting the receiver (immutable
// value-object pattern). Pass nil to clear.
//
// Validity of the DAG is owned by plan/dag_validator.go; this builder just
// stores the pointer.
func (p Plan) WithDAG(d *PlanDAG) Plan {
	if d == nil {
		p.DAG = nil
		return p
	}
	cp := PlanDAG{
		Nodes:          append([]PlanNode(nil), d.Nodes...),
		Edges:          append([]DataEdge(nil), d.Edges...),
		Priorities:     make(map[string]int, len(d.Priorities)),
		MaxParallelism: d.MaxParallelism,
	}
	for k, v := range d.Priorities {
		cp.Priorities[k] = v
	}
	p.DAG = &cp
	return p
}

// Validate enforces PP-1 (strength match), PP-2 (falsifiability),
// PP-3 (blast radius), and structural checks. Returns nil if the Plan is
// dispatch-ready; otherwise wraps the first violation in a SentinelError.
//
// Validation order (cheapest first):
//  1. Structural (Kind, Steps, SourceObservationIDs, PersistScope, Strength range)
//  2. PP-2 (FailureCriteria non-empty + Op whitelist + Field observability)
//  3. PP-3 (BlastRadius limits — fail-fast, returns the first offending axis)
//
// PP-1 (Strength vs min BusinessObs.Strength) is enforced by DefaultPlanner
// before Plan construction — at construction time we don't have access to the
// originating UncertaintyReport. PP-1 here is just the [0, 1] range check.
func (p *Plan) Validate() error {
	return p.ValidateWithOpts(ValidateOpts{})
}

// ValidateWithOpts is Validate with caller-provided thresholds (mostly for
// tests; production code uses the package defaults).
func (p *Plan) ValidateWithOpts(opts ValidateOpts) error {
	// 1. Structural checks
	if !p.Kind.IsKnown() {
		return NewPlanKindUnsetError()
	}
	if len(p.Steps) == 0 {
		return sharederrors.WithCode(
			"PLAN_STEPS_8010",
			"plan: at least one Step required for dispatch",
			ErrPlanStepsEmpty,
		)
	}
	if len(p.SourceObservationIDs) == 0 {
		return NewPlanSourceObservationIDsRequiredError()
	}
	if p.Strength < 0 || p.Strength > 1 {
		return sharederrors.WithCode(
			"PLAN_STRENGTH_8011",
			fmt.Sprintf("plan: Strength=%g out of [0, 1] range", p.Strength),
			ErrPlanStrengthOutOfRange,
		)
	}
	if !p.BlastRadius.PersistScope.Valid() {
		return sharederrors.WithCode(
			"PLAN_PERSIST_8012",
			fmt.Sprintf("plan: PersistScope=%q is not one of transient/session/permanent", p.BlastRadius.PersistScope),
			ErrPlanPersistScopeInvalid,
		)
	}

	// 2. PP-2: FailureCriteria non-empty + Op whitelist + Field observability
	if len(p.FailureCriteria) == 0 {
		return sharederrors.WithCode(
			"PLAN_PP2_EMPTY_8020",
			"plan: FailureCriteria must be non-empty (PP-2 falsifiability)",
			ErrPlanFailureCriteriaEmpty,
		)
	}
	for _, fc := range p.FailureCriteria {
		if !isOpAllowed(fc.Op) {
			return sharederrors.WithCode(
				"PLAN_PP2_OP_8021",
				fmt.Sprintf("plan: FailureCriteria.Op=%q not in whitelist (eq/ne/gt/lt/in/contains)", fc.Op),
				ErrPlanFailureCriteriaInvalidOp,
			)
		}
		if !isFieldObservable(fc.Field) {
			return sharederrors.WithCode(
				"PLAN_PP2_FIELD_8022",
				fmt.Sprintf("plan: FailureCriteria.Field=%q not in observable set (exit_code/diff_hash/api_status/duration_ms/output_match)", fc.Field),
				ErrPlanFailureCriteriaInvalidField,
			)
		}
	}

	// 3. PP-3: BlastRadius limits — first offending axis wins (deterministic).
	if p.BlastRadius.FileCount > opts.fileLimit() {
		return NewPlanBlastRadiusExceededError("FileCount", p.BlastRadius.FileCount, opts.fileLimit())
	}
	if p.BlastRadius.APICallCount > opts.apiLimit() {
		return NewPlanBlastRadiusExceededError("APICallCount", p.BlastRadius.APICallCount, opts.apiLimit())
	}
	if p.BlastRadius.TokenCost > opts.tokenLimit() {
		return NewPlanBlastRadiusExceededError("TokenCost", p.BlastRadius.TokenCost, opts.tokenLimit())
	}

	// 4. DAG (DM-20260707-001 PR-A1 T13) — when present, validate grammar
	// (no cycles, ≤10 nodes, ≤8 fan-out, no duplicates, no dangling edges).
	// nil DAG is the PR-B1 4-channel path and is valid as-is.
	if p.DAG != nil {
		if err := validateDAG(p.DAG, opts.dagOpts()); err != nil {
			return err
		}
	}

	return nil
}

// ReverseLookupObservations takes a slice of Observations and returns the subset
// whose IDs are present in Plan.SourceObservationIDs. This is the Phase 4
// Verify reverse-lookup primitive — the Verifier can pass the originating
// Observations to the LLM Verifier so the verdict can cite evidence.
//
// The function tolerates a nil input and returns an empty slice.
func (p *Plan) ReverseLookupObservations(observations []ObservationLookup) []ObservationLookup {
	if p == nil || len(p.SourceObservationIDs) == 0 || len(observations) == 0 {
		return nil
	}
	idx := make(map[string]struct{}, len(p.SourceObservationIDs))
	for _, id := range p.SourceObservationIDs {
		idx[id] = struct{}{}
	}
	out := make([]ObservationLookup, 0, len(idx))
	for _, obs := range observations {
		if _, ok := idx[obs.GetID()]; ok {
			out = append(out, obs)
		}
	}
	return out
}

// ObservationLookup is the minimal projection the Plan needs to perform
// reverse-lookup. Defined as an interface here so the Plan package does not
// need to import orchtypes (no import cycle, matches Phase 1 MemoryEntry precedent).
type ObservationLookup interface {
	GetID() string
}

// isOpAllowed checks whether the Op string is in the FailureCriterion whitelist.
func isOpAllowed(op string) bool {
	for _, allowed := range FailureCriterionOpWhitelist {
		if op == allowed {
			return true
		}
	}
	return false
}

// isFieldObservable checks whether the Field path is extractable from
// ExecutionEvidence (per PR-C5 EvidenceExtractor).
func isFieldObservable(field string) bool {
	for _, allowed := range ObservableFailureCriterionFields {
		if field == allowed {
			return true
		}
	}
	return false
}

// MarshalJSON ensures the JSON wire format is consistent with the omitempty
// convention. PlanKind is delegated to its own MarshalJSON which already
// omits KindUnset.
func (p Plan) MarshalJSON() ([]byte, error) {
	type alias Plan
	return json.Marshal(alias(p))
}