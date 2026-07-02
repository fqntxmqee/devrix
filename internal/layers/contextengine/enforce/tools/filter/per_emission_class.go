// Package filter v2 provides the D2-S15-A02-T02..T05 Filter v2 that filters
// tool specs by EmissionClass + task_kind + agent. Phase D of the
// devrix-mups-tool-classification-and-channel-autonomy change.
//
// Phase D design (specs/execute-channels.md §D7-EXEC-CH-4 + design.md §2.5):
//   - PerEmissionClassFilter: keep only specs matching the agent's
//     allowed emission classes (e.g. explore agent → only Fact + Probe).
//   - PerTaskKindFilter: tighten IterationBound based on task_kind
//     (review=Bounded(15), edit=Bounded(10), test=Bounded(12),
//     observe=OpenEnded, refactor=Bounded(8)).
//   - cross-consistency: when task_kind=review, Probe tools MUST NOT
//     be downgraded to OpenEnded (H9 / P1-AC-7).
//
// Filter v2 is intentionally 3-dimensional (agent + emission_class +
// task_kind). The 4th dimension (workspace) is OOS-10 per the
// game-theory consensus.
package filter

import (
	"github.com/devrix/devrix/internal/shared/contracts"
)

// PerEmissionClassFilter filters tool specs by EmissionClass.
//
// The agent's allowed classes are passed in (default: all). Tools whose
// EmissionClass is not in the allow list are removed.
type PerEmissionClassFilter struct {
	// AllowedClasses is the set of emission classes the agent can use.
	// nil = allow all.
	AllowedClasses map[contracts.EmissionClass]bool
}

// NewPerEmissionClassFilter constructs a filter with the given allow list.
// nil = allow all classes.
func NewPerEmissionClassFilter(allowed []contracts.EmissionClass) *PerEmissionClassFilter {
	if allowed == nil {
		return &PerEmissionClassFilter{AllowedClasses: nil}
	}
	m := make(map[contracts.EmissionClass]bool, len(allowed))
	for _, c := range allowed {
		m[c] = true
	}
	return &PerEmissionClassFilter{AllowedClasses: m}
}

// Apply returns the subset of specs whose EmissionClass is allowed.
func (f *PerEmissionClassFilter) Apply(specs []contracts.ToolSpec) []contracts.ToolSpec {
	if f.AllowedClasses == nil {
		return specs
	}
	out := make([]contracts.ToolSpec, 0, len(specs))
	for _, s := range specs {
		if f.AllowedClasses[s.EmissionClass] {
			out = append(out, s)
		}
	}
	return out
}

// AllowedEmissionClassesForAgent returns the canonical emission class
// allow list for a given agent type (D2-S15-A02-T04). The 6 agent
// types and their rules:
//
//	explore  → Fact + Probe (read-only)
//	worker   → Fact + Action + Probe
//	delegate → Probe (delegate_*) + Action (write back)
//	planner  → all
//	verifier → Fact + Probe
//	reviewer → Fact + Probe
func AllowedEmissionClassesForAgent(agentType string) []contracts.EmissionClass {
	switch agentType {
	case "explore":
		return []contracts.EmissionClass{contracts.EC_Fact, contracts.EC_Probe}
	case "worker":
		return []contracts.EmissionClass{contracts.EC_Fact, contracts.EC_Action, contracts.EC_Probe}
	case "delegate":
		return []contracts.EmissionClass{contracts.EC_Probe, contracts.EC_Action}
	case "verifier":
		return []contracts.EmissionClass{contracts.EC_Fact, contracts.EC_Probe}
	case "reviewer":
		return []contracts.EmissionClass{contracts.EC_Fact, contracts.EC_Probe}
	case "planner", "":
		return nil // allow all
	}
	return nil
}
