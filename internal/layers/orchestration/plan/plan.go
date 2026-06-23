// Package plan implements MUPS v4.3 Plan 节点 (Phase 2 PR-B1).
//
// Plan is the structured output of the Plan node, consumed by the Execute node
// (Phase 3 PR-C2 4 Channel router). It carries:
//
//   - Kind (4 enum): which execution channel to route to
//   - Strength: [0, 1] capped by min(business observation strength, floor)
//   - Steps: ordered execution units
//   - FailureCriteria: PP-2 falsifiability (verified by Verifier in Phase 4)
//   - BlastRadius: PP-3 explosion radius (file/API/token/persist count)
//   - SourceObservationIDs: reverse-traceability to UncertaintyReport observations
//
// SourceObservationIDs is the key lineage field that ties Plan → Observation,
// enabling Phase 4 Verify to reverse-lookup the originating evidence.
package plan

import (
	"encoding/json"
	"fmt"
	"strings"
)

// PlanKind is the 4-class enum that drives Phase 3 Execute channel routing.
//
// Mapping (defined in Phase 3 PR-C2 channel_router.go):
//   - CommitmentPlan → CommitChannel  (synchronous, single-step, deterministic)
//   - ProtocolPlan   → ProtocolChannel (asynchronous, multi-step, idempotent)
//   - ScenarioPlan   → ScenarioChannel (read-only probe, no side-effect)
//   - ExplorationPlan → ExplorationChannel (parallel experiments, sandboxed)
type PlanKind uint8

const (
	// KindUnset (zero value) signals "plan not yet classified" — wire format
	// omits the field. Distinct from any of the 4 named kinds so the validator
	// can surface KindUnset as ErrPlanKindUnset without confusion.
	KindUnset PlanKind = iota

	// CommitmentPlan: 1 Step direct execution. Produces StateChangeCert artifact.
	// Example: "deploy this build" / "merge this PR" / "send this email".
	CommitmentPlan

	// ProtocolPlan: multi-step async execution with idempotency keys.
	// Produces ResponseRecord artifact. Tolerates partial completion.
	// Example: "migrate database schema" / "rotate API keys".
	ProtocolPlan

	// ScenarioPlan: read-only probe (no side effect). Produces ProbeReport artifact.
	// Example: "diagnose why build fails" / "explain what this code does".
	ScenarioPlan

	// ExplorationPlan: parallel experiments with sandbox isolation.
	// Produces ExperimentData artifact. Lowest priority in ChannelRouter.
	// Example: "compare 3 implementations" / "A/B test variants".
	ExplorationPlan
)

// String returns the snake_case wire format (matches the same convention as
// orchtypes.ArtifactKind so the D5 dashboard string filters are consistent).
func (k PlanKind) String() string {
	switch k {
	case CommitmentPlan:
		return "commitment_plan"
	case ProtocolPlan:
		return "protocol_plan"
	case ScenarioPlan:
		return "scenario_plan"
	case ExplorationPlan:
		return "exploration_plan"
	default:
		return fmt.Sprintf("unknown_plan_kind(%d)", uint8(k))
	}
}

// IsKnown reports whether k is one of the 4 named kinds (i.e. not KindUnset
// and not an unknown numeric value).
func (k PlanKind) IsKnown() bool {
	switch k {
	case CommitmentPlan, ProtocolPlan, ScenarioPlan, ExplorationPlan:
		return true
	default:
		return false
	}
}

// MarshalJSON outputs the snake_case string (not the numeric value).
// Zero value (KindUnset) is omitted from the JSON entirely.
func (k PlanKind) MarshalJSON() ([]byte, error) {
	if k == KindUnset {
		// omitempty: not present in JSON
		return []byte("null"), nil
	}
	if !k.IsKnown() {
		return nil, fmt.Errorf("plan: MarshalJSON: unknown PlanKind=%d", uint8(k))
	}
	return json.Marshal(k.String())
}

// UnmarshalJSON parses the snake_case wire format. Unknown strings fail loudly
// (no silent coercion to KindUnset) — matches the Phase 2 PR-RF C3 contract
// for FromVerifier (fail-fast on unknown inputs).
func (k *PlanKind) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("plan: UnmarshalJSON: %w", err)
	}
	if s == "" {
		*k = KindUnset
		return nil
	}
	switch s {
	case "commitment_plan":
		*k = CommitmentPlan
	case "protocol_plan":
		*k = ProtocolPlan
	case "scenario_plan":
		*k = ScenarioPlan
	case "exploration_plan":
		*k = ExplorationPlan
	default:
		return fmt.Errorf("plan: UnmarshalJSON: unknown PlanKind=%q (valid: commitment_plan/protocol_plan/scenario_plan/exploration_plan)", s)
	}
	return nil
}

// ParsePlanKind is the inverse of String — used by tests and CLI parsing.
// Returns ErrUnknownPlanKind for unknown strings (fail-fast).
func ParsePlanKind(s string) (PlanKind, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "commitment_plan":
		return CommitmentPlan, nil
	case "protocol_plan":
		return ProtocolPlan, nil
	case "scenario_plan":
		return ScenarioPlan, nil
	case "exploration_plan":
		return ExplorationPlan, nil
	default:
		return KindUnset, fmt.Errorf("plan: ParsePlanKind: unknown kind=%q", s)
	}
}