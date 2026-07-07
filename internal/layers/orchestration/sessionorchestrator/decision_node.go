package sessionorchestrator

// Decision Node — D7 6-node pipeline stage 5 (DM-20260707-001 PR-D).
//
// Decision is a deterministic static-mapping stage that sits between
// Verify (stage 4) and Learn (stage 6). It maps the VerdictKind + AC
// outcomes + RoundMeta into one of 5 routing decisions. Zero LLM
// calls. The mapping table is locked in design.md §2.12 + decision-tree
// §8.6.1; do NOT add new branches without PR-D review + T-registry bump.
//
// The 11-row mapping (T47, codex-consensus verified 2026-07-07):
//
//	Row | Verdict          | Other Conditions                         | Decision
//	----+------------------+------------------------------------------+----------
//	 1  | Pass             | (default)                                | A accept
//	 2  | Partial          | Tolerance=high OR ChildBudget=0          | A accept
//	 3  | Partial          | AC decomposable AND ChildBudget>0        | C child_worker
//	 4  | Partial          | (other)                                  | A accept
//	 5  | Fail             | AttemptNo < MaxRetry                     | B retry
//	 6  | Fail             | AttemptNo >= MaxRetry                    | E human_review
//	 7  | Indeterminate    | RiskLevel=high                           | E human_review
//	 8  | Indeterminate    | RiskLevel=normal/low                     | B retry
//	 9  | Error (全 Err)   | Network/Timeout class                    | B retry
//	10  | (any)            | IsChildSegment AND all sibling decided   | D parent_rollup
//	11  | plan_error       | (Plan LLM timeout/5xx/partial — PR-F)   | E human_review
//
// Decision.next_action records the chosen path's wire-format string
// so D5 dashboards and Jaeger spans can grep on it without an enum
// reflection lookup.

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// DecisionKind is the wire-format enum for the 5 routing paths the
// Decision node can choose. The integer value MUST NOT change after
// release — D5 dashboards key on the uint value; only ADD new kinds.
type DecisionKind uint8

const (
	// DecisionAccept (path A): emit final + Learn.
	DecisionAccept DecisionKind = iota
	// DecisionRetry (path B): re-run Execute + Verify (counts toward MaxRetry).
	DecisionRetry
	// DecisionChildWorker (path C): spawn a sub-worker for the AC subset
	// that failed; parent waits for OnChildComplete.
	DecisionChildWorker
	// DecisionParentRollup (path D): all sibling child segments have
	// their own decision — trigger the parent rollup round to aggregate.
	DecisionParentRollup
	// DecisionHumanReview (path E): emit abort + flag for human review;
	// Learn still runs (β++ per design.md §2.12).
	DecisionHumanReview
)

// String returns the wire-format name. Mirrors verdict.go's contract so
// D5 dashboard filters stay uniform. Unknown kinds return a debug
// integer so logs stay grep-able (regression of the v2.6 verbatim
// decision-tree convention).
func (k DecisionKind) String() string {
	switch k {
	case DecisionAccept:
		return "accept"
	case DecisionRetry:
		return "retry"
	case DecisionChildWorker:
		return "child_worker"
	case DecisionParentRollup:
		return "parent_rollup"
	case DecisionHumanReview:
		return "human_review"
	default:
		return fmt.Sprintf("DecisionKind(%d)", uint8(k))
	}
}

// Validate returns an error for unknown DecisionKind values. Mirrors
// orchtypes verify/error pattern; non-validated Decision values are a
// construction bug, not a runtime miss.
func (k DecisionKind) Validate() error {
	switch k {
	case DecisionAccept, DecisionRetry, DecisionChildWorker,
		DecisionParentRollup, DecisionHumanReview:
		return nil
	default:
		return fmt.Errorf("decision: unknown DecisionKind %d", uint8(k))
	}
}

// RoundMeta carries per-round context that influences the decision
// beyond VerdictKind alone. All fields are inputs only — Decision
// never mutates the round, only reads.
//
// Defaults applied when the field is at zero value (see
// buildDefaultRoundMeta in item_pipeline_dispatch.go):
//   - AttemptNo: 0 (first attempt; MaxRetry=1 means one retry allowed)
//   - ChildBudgetRemaining: 0 (no child worker slots)
//   - RiskLevel: "normal"
//   - IsChildSegment: false
//   - SiblingDecidedCount / SiblingTotalCount: 0/0 (no rollup gating)
type RoundMeta struct {
	AttemptNo            int
	ChildBudgetRemaining int
	RiskLevel            string
	IsChildSegment       bool
	SiblingDecidedCount  int
	SiblingTotalCount    int
	// HasDecomposableAC is true when at least one Required-Preferred AC
	// has subtasks that can be re-dispatched via C (child worker).
	// Without this signal, row 3 (Partial → child_worker) would never
	// fire even when ChildBudgetRemaining > 0. The verify node sets
	// this from its AC decomposition analysis; nil defaults to false.
	HasDecomposableAC bool
	// ToleranceHint indicates whether the policy allows lenient passes
	// on partial. "high" makes the row 2 (Partial + Tolerance=high)
	// branch fire. Anything else (including "") collapses to "default".
	ToleranceHint string
	// VerdictErrorClass lets row 9 distinguish Network/Timeout errors
	// from other VerdictErrorIndeterminate causes. "network_timeout"
	// matches row 9; "" or other → fall through to default A accept.
	// Set by the verify node based on the error telemetry, not by
	// callers.
	VerdictErrorClass string
	// PlanErrorClass is non-empty only when the Plan node failed (T69,
	// PR-F). The PR-D codex consensus locks the row 11 wiring so
	// plan_error → E human_review even when there's no Verdict.
	PlanErrorClass string
}

// Decision is the outcome of the Decision node. Carried in
// round.Metadata["decision"] (T51) for downstream Learn + D5 + audit.
//
// NextWorkItemSpec is non-nil only when Kind == DecisionChildWorker;
// nil for A/B/D/E paths.
type Decision struct {
	Kind             DecisionKind
	Reason           string
	NextWorkItemSpec *ChildWorkItemSpec
	// MapRow records which row of the 11-row table fired. Useful for
	// D5 dashboards ("row 3 fires 12% of the time") and triage; nil
	// when the table didn't match and we fell through to A accept.
	MapRow int
}

// ChildWorkItemSpec is the per-C-spawn spec carried in Decision. The
// SubWorkerSpawner consumes this verbatim when triggering child
// execution.
//
// Validate enforces the design.md §2.12 + §2.13 invariants:
//   - ParentWorkItemID non-empty
//   - SubSegmentIDs ≥ 1 (the partial AC subset)
//   - MaxBudget ≤ 2 (per design; the parent budget allows ≤ 2 child
//     workers per round; higher values silently drain the budget)
//   - InheritACSubset non-empty when C is the routing reason
type ChildWorkItemSpec struct {
	ParentWorkItemID string
	SubSegmentIDs    []string
	InheritACSubset  []string // AC IDs the child is asked to retry
	MaxBudget        int
}

// Validate returns nil when the spec is wire-ready for spawn, otherwise
// the design.md invariant broken. Used by SubWorkerSpawner before
// allocating a WorkItem.
func (s *ChildWorkItemSpec) Validate() error {
	if s == nil {
		return fmt.Errorf("decision: ChildWorkItemSpec nil")
	}
	if strings.TrimSpace(s.ParentWorkItemID) == "" {
		return fmt.Errorf("decision: ChildWorkItemSpec.ParentWorkItemID empty")
	}
	if len(s.SubSegmentIDs) == 0 {
		return fmt.Errorf("decision: ChildWorkItemSpec.SubSegmentIDs empty (no AC subset)")
	}
	if s.MaxBudget < 0 || s.MaxBudget > 2 {
		return fmt.Errorf("decision: ChildWorkItemSpec.MaxBudget=%d out of range [0, 2]", s.MaxBudget)
	}
	if len(s.InheritACSubset) == 0 {
		return fmt.Errorf("decision: ChildWorkItemSpec.InheritACSubset empty (no AC to retry)")
	}
	return nil
}

// DecisionNode is the public stage-5 entry point. Implemented as a
// pure-function-style entry over the 11-row static table so callers
// can swap the implementation (e.g. shadow A/B with ML lookup) without
// touching item_pipeline.go.
type DecisionNode interface {
	Decide(ctx DecisionContext) (Decision, error)
}

// DecisionContext is the read-only payload DecisionNode.Decide consumes.
// Splitting it out of the round struct keeps the test surface clean and
// prevents the Decide function from accidentally mutating the round.
type DecisionContext struct {
	RoundMeta RoundMeta
	// VerdictKind is the AGGREGATE 4-state verdict from PerCriterion
	// executor; cases match shared/types.VerdictKind. We use a uint8 so
	// the Decision interface stays decoupled from the orchtypes
	// dependency (the verify package owns the canonical enum and casts
	// at the call site).
	VerdictKind uint8
	// VerdictErrorClass mirrors RoundMeta.VerdictErrorClass for callers
	// that pass it at the top level rather than via RoundMeta.
	VerdictErrorClass string
	// ACSubset is the list of AC IDs that failed (or that the verify
	// node flagged as decomposable). Empty for non-Partial verdicts.
	ACSubset []string
	// PlanErrorClass lets PR-F path row 11 fire when the Plan node
	// itself failed. Empty ⇒ row 11 never matches.
	PlanErrorClass string
}

// buildDefaultRoundMeta returns a RoundMeta with safe defaults applied.
// ItemPipelineRunner should call this whenever it constructs RoundMeta
// without the producer (verify / child_workitem) setting each field —
// without defaults, row 5/6 (Fail + MaxRetry) would always treat
// AttemptNo as zero, row 9 would never match because VerdictErrorClass
// is "", and row 11 (PR-F plan_error) would fire whenever PlanErrorClass
// is non-empty (correct today, but defensive).
func buildDefaultRoundMeta(in RoundMeta) RoundMeta {
	if in.RiskLevel == "" {
		in.RiskLevel = "normal"
	}
	if in.ChildBudgetRemaining < 0 {
		in.ChildBudgetRemaining = 0
	}
	if in.AttemptNo < 0 {
		in.AttemptNo = 0
	}
	if in.SiblingDecidedCount < 0 {
		in.SiblingDecidedCount = 0
	}
	if in.SiblingTotalCount < 0 {
		in.SiblingTotalCount = 0
	}
	return in
}

// defaultMaxRetry is the production ceiling for the row 5 vs row 6 split
// (Fail → retry vs human_review). design.md §2.12 "Fail → AttemptNo <
// MaxRetry=1 → retry; otherwise human_review". ItemPipelineRunner
// exposes this via orchtypes.Config.DAGExecutor.MaxRetryOnPartialFail
// for partial fails; the FAIL-specific retry cap is hardcoded to 1
// here because the Fail semantic is stricter (no silent tail-of-budget
// retries that confuse VerdictPass+A paths).
const defaultMaxRetry = 1

// maxAttemptNo is the runaway-loop guard (Q5 / Risk E). An AttemptNo above
// this bound is treated as corruption: Decide returns an error so the
// caller falls back to safety-net A accept instead of an infinite retry.
// Bound of 100 matches the design.md §2.12 budget assumption (a fresh
// round shouldn't exceed 100 retries before human review).
const maxAttemptNo = 100

// staticDecisionNode is the production implementation of DecisionNode.
// Pure function over the 11-row table — no clock, no IO, no LLM, so it
// always returns in < 1ms. Tests can construct a fresh instance per
// scenario; there is no shared state to protect.
type staticDecisionNode struct {
	// maxRetry overrides defaultMaxRetry when > 0. Tests inject
	// different ceilings to cover the row 5/6 boundary.
	maxRetry int
}

// NewStaticDecisionNode constructs the production DecisionNode. The
// returned value is safe for concurrent use across goroutines — the
// underlying Decide is a pure function.
func NewStaticDecisionNode() DecisionNode {
	return &staticDecisionNode{maxRetry: defaultMaxRetry}
}

// NewStaticDecisionNodeWithMaxRetry is the test seam: lets a unit test
// dial MaxRetry down to 0 to force every Fail → human_review without
// touching the orchtypes config.
func NewStaticDecisionNodeWithMaxRetry(maxRetry int) DecisionNode {
	if maxRetry < 0 {
		maxRetry = 0
	}
	return &staticDecisionNode{maxRetry: maxRetry}
}

// Decide applies the 11-row static-mapping table. The pre-order
// matters: row 10 (parent_rollup) fires when ALL sibling segments
// have decided, regardless of Verdict. Rows 11 (plan_error) and 9
// (Error Network/Timeout) depend on RoundMeta signals and short-circuit
// the verdict ordering. Returns (Decision{accept, "fallback"}, nil)
// when no row matches — the safety-net gate that prevents lockup on
// an unmapped combination.
//
// Row 9 is reachable only when VerdictKind falls outside the canonical
// 4-state enum {Pass, Partial, Indeterminate, Fail}, e.g. when a 5xx-class
// translator casts VerdictKind=99 + VerdictErrorClass=network_timeout.
// Production never sees this path; row 9 exists for defense-in-depth.
func (n *staticDecisionNode) Decide(ctx DecisionContext) (Decision, error) {
	if err := ctx.RoundMeta.AttemptNoValidated(); err != nil {
		return Decision{}, err
	}
	meta := buildDefaultRoundMeta(ctx.RoundMeta)

	// Mirror top-level VerdictErrorClass into RoundMeta so the row 9
	// path is reachable from callers that pass either shape.
	if meta.VerdictErrorClass == "" && ctx.VerdictErrorClass != "" {
		meta.VerdictErrorClass = ctx.VerdictErrorClass
	}
	if meta.PlanErrorClass == "" && ctx.PlanErrorClass != "" {
		meta.PlanErrorClass = ctx.PlanErrorClass
	}

	// Row 11 — plan_error → human_review (PR-F, codex consensus 2026-07-07).
	// This short-circuits every other row: when the Plan LLM call
	// itself fails, no Verdict is available and the routing is
	// unconditional human_review. Place this BEFORE row 10 because plan
	// errors should not be hidden by a parent_rollup that just
	// happens to be ready.
	if meta.PlanErrorClass != "" {
		return Decision{
			Kind:   DecisionHumanReview,
			Reason: fmt.Sprintf("plan_error:%s", meta.PlanErrorClass),
			MapRow: 11,
		}, nil
	}

	// Row 10 — (any) IsChildSegment + all sibling decided → parent_rollup.
	// Fires even when VerdictKind == VerdictPass because parent
	// aggregation may still be needed to synthesize the rollup summary.
	if meta.IsChildSegment && meta.SiblingTotalCount > 0 &&
		meta.SiblingDecidedCount >= meta.SiblingTotalCount {
		return Decision{
			Kind:   DecisionParentRollup,
			Reason: fmt.Sprintf("all_siblings_decided (%d/%d)", meta.SiblingDecidedCount, meta.SiblingTotalCount),
			MapRow: 10,
		}, nil
	}

	vk := ctx.VerdictKind
	switch vk {
	case 0: // types.VerdictPass
		// Row 1 — Pass + (default) → accept.
		return Decision{
			Kind:   DecisionAccept,
			Reason: "verdict_pass",
			MapRow: 1,
		}, nil

	case 1: // types.VerdictPartial
		// Row 2 — Partial + Tolerance=high OR ChildBudget=0 → accept.
		if strings.EqualFold(meta.ToleranceHint, "high") || meta.ChildBudgetRemaining <= 0 {
			tolOrBudget := "tolerance_high"
			if meta.ChildBudgetRemaining <= 0 && !strings.EqualFold(meta.ToleranceHint, "high") {
				tolOrBudget = "child_budget_zero"
			}
			return Decision{
				Kind:   DecisionAccept,
				Reason: fmt.Sprintf("verdict_partial+%s", tolOrBudget),
				MapRow: 2,
			}, nil
		}
		// Row 3 — Partial + decomposable AC + ChildBudget>0 → child_worker.
		if meta.HasDecomposableAC && meta.ChildBudgetRemaining > 0 && len(ctx.ACSubset) > 0 {
			spec := &ChildWorkItemSpec{
				ParentWorkItemID: "", // filled by caller via round.WorkItemID
				SubSegmentIDs:    append([]string(nil), ctx.ACSubset...),
				InheritACSubset:  append([]string(nil), ctx.ACSubset...),
				MaxBudget:        meta.ChildBudgetRemaining,
			}
			_ = spec // caller fills ParentWorkItemID; Validate happens at spawn
			return Decision{
				Kind:             DecisionChildWorker,
				Reason:           "verdict_partial+ac_decomposable",
				NextWorkItemSpec: spec,
				MapRow:           3,
			}, nil
		}
		// Row 4 — Partial + (other) → accept.
		return Decision{
			Kind:   DecisionAccept,
			Reason: "verdict_partial+fallback",
			MapRow: 4,
		}, nil

	case 2: // types.VerdictIndeterminate
		// Row 7 — Indeterminate + RiskLevel=high → human_review.
		if strings.EqualFold(meta.RiskLevel, "high") {
			return Decision{
				Kind:   DecisionHumanReview,
				Reason: "verdict_indeterminate+risk_high",
				MapRow: 7,
			}, nil
		}
		// Row 8 — Indeterminate + RiskLevel=normal/low → retry.
		return Decision{
			Kind:   DecisionRetry,
			Reason: fmt.Sprintf("verdict_indeterminate+risk_%s", strings.ToLower(meta.RiskLevel)),
			MapRow: 8,
		}, nil

	case 3: // types.VerdictFail
		maxRetry := n.maxRetry
		if maxRetry < 0 {
			maxRetry = 0
		}
		// Row 5 — Fail + AttemptNo < MaxRetry → retry.
		if meta.AttemptNo < maxRetry {
			return Decision{
				Kind:   DecisionRetry,
				Reason: fmt.Sprintf("verdict_fail+attempt_%d<max_%d", meta.AttemptNo, maxRetry),
				MapRow: 5,
			}, nil
		}
		// Row 6 — Fail + AttemptNo >= MaxRetry → human_review.
		return Decision{
			Kind:   DecisionHumanReview,
			Reason: fmt.Sprintf("verdict_fail+attempt_%d>=max_%d", meta.AttemptNo, maxRetry),
			MapRow: 6,
		}, nil

	default:
		// Row 9 — VerdictError (not in {Pass, Partial, Indeterminate,
		// Fail}) + Network/Timeout class → retry. VerdictError in our
		// 4-state enum collapses to the "Indeterminate" branch above,
		// so row 9 here is reached only when VerdictKind is outside the
		// 0..3 range AND VerdictErrorClass marks Network/Timeout.
		// Mirrors decision-tree §8.6.1.
		if meta.VerdictErrorClass != "" {
			return Decision{
				Kind:   DecisionRetry,
				Reason: fmt.Sprintf("verdict_error+%s", meta.VerdictErrorClass),
				MapRow: 9,
			}, nil
		}
		// Safety-net fallback: no row matched (e.g. unknown VerdictKind
		// without VerdictErrorClass). Return A accept + slog.Warn in
		// the caller so a future-mapped kind doesn't lock the pipeline.
		return Decision{
			Kind:   DecisionAccept,
			Reason: "decision_map_miss_fallback",
			MapRow: 0,
		}, nil
	}
}

// AttemptNoValidated returns an error when AttemptNo is absurd (out of
// any plausible round-trip bound). Defense-in-depth so a corrupted
// RoundMeta cannot lock the Decision node into an infinite retry loop.
func (m RoundMeta) AttemptNoValidated() error {
	if m.AttemptNo < 0 {
		return fmt.Errorf("decision: RoundMeta.AttemptNo=%d must be >= 0", m.AttemptNo)
	}
	if m.AttemptNo > maxAttemptNo {
		return fmt.Errorf("decision: RoundMeta.AttemptNo=%d exceeds %d (runaway loop sentinel)", m.AttemptNo, maxAttemptNo)
	}
	return nil
}

// DecisionJSON is the wire format persisted to round.Metadata["decision"]
// (T51). It mirrors Decision with JSON tags so D5 dashboards can decode
// the metadata blob without importing the orchestration package.
type DecisionJSON struct {
	Kind             string             `json:"kind"`
	Reason           string             `json:"reason"`
	MapRow           int                `json:"map_row"`
	NextWorkItemSpec *ChildSpecJSON     `json:"next_spec,omitempty"`
	DecidedAt        time.Time          `json:"decided_at"`
}

// ChildSpecJSON mirrors ChildWorkItemSpec with JSON tags for
// round.Metadata persistence. SubSegmentIDs + InheritACSubset are
// carried as []string; richer AC descriptor payloads are intentionally
// omitted (Learn re-reads from DB if needed).
type ChildSpecJSON struct {
	ParentWorkItemID string   `json:"parent_work_item_id,omitempty"`
	SubSegmentIDs    []string `json:"sub_segment_ids,omitempty"`
	InheritACSubset  []string `json:"inherit_ac_subset,omitempty"`
	MaxBudget        int      `json:"max_budget,omitempty"`
}

// MarshalDecisionJSON returns the canonical JSON encoding of the
// Decision for round.Metadata["decision"]. Stable across PR-D's
// versioning: the schema MUST NOT break without a T-registry bump.
func MarshalDecisionJSON(d Decision) (string, error) {
	payload := DecisionJSON{
		Kind:      d.Kind.String(),
		Reason:    d.Reason,
		MapRow:    d.MapRow,
		DecidedAt: time.Now().UTC(),
	}
	if d.NextWorkItemSpec != nil {
		payload.NextWorkItemSpec = &ChildSpecJSON{
			ParentWorkItemID: d.NextWorkItemSpec.ParentWorkItemID,
			SubSegmentIDs:    append([]string(nil), d.NextWorkItemSpec.SubSegmentIDs...),
			InheritACSubset:  append([]string(nil), d.NextWorkItemSpec.InheritACSubset...),
			MaxBudget:        d.NextWorkItemSpec.MaxBudget,
		}
	}
	b, err := json.Marshal(&payload)
	if err != nil {
		return "", fmt.Errorf("decision: marshal: %w", err)
	}
	return string(b), nil
}
