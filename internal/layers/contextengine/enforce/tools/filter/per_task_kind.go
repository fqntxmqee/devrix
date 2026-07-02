package filter

import (
	"github.com/devrix/devrix/internal/shared/contracts"
)

// PerTaskKindFilter tightens IterationBound based on the inferred
// task_kind. The 5 task kinds and their bound overrides:
//
//	review   → Bounded(15)   (LLM may need to read many files; but Bounded)
//	edit     → Bounded(10)   (tighter; user wants changes)
//	test     → Bounded(12)   (test runs have stable shape)
//	observe  → OpenEnded     (no convergence pressure)
//	refactor → Bounded(8)    (tightest; refactor scope is bounded)
//
// H9 / P1-AC-7 cross-consistency (TestPerTaskKindFilterCrossConsistency):
// when task_kind=review, Probe tools MUST NOT be downgraded to
// OpenEnded. The filter enforces this.
//
// DSAFT: D2-S15-A02-T03 (4 task_kind mapping) + T15 (cross-consistency).
type PerTaskKindFilter struct {
	// TaskKind is the inferred user task kind.
	TaskKind string
}

// NewPerTaskKindFilter constructs a PerTaskKindFilter.
func NewPerTaskKindFilter(taskKind string) *PerTaskKindFilter {
	return &PerTaskKindFilter{TaskKind: taskKind}
}

// TaskKindBound returns the canonical IterationBound for a task kind.
// Used by Apply to rewrite each spec's IterationBound.
//
// Per-task-kind defaults (D2-S15-A02-T03):
//
//	review:   Bounded(15)
//	edit:     Bounded(10)
//	test:     Bounded(12)
//	observe:  OpenEnded
//	refactor: Bounded(8)
//	"":       OpenEnded (no override; use tool default)
func TaskKindBound(taskKind string) contracts.IterationBound {
	switch taskKind {
	case "review":
		return contracts.IterationBound{Kind: contracts.IB_Bounded, MaxN: 15}
	case "edit":
		return contracts.IterationBound{Kind: contracts.IB_Bounded, MaxN: 10}
	case "test":
		return contracts.IterationBound{Kind: contracts.IB_Bounded, MaxN: 12}
	case "refactor":
		return contracts.IterationBound{Kind: contracts.IB_Bounded, MaxN: 8}
	case "observe", "":
		return contracts.IterationBound{Kind: contracts.IB_OpenEnded}
	}
	return contracts.IterationBound{Kind: contracts.IB_OpenEnded}
}

// Apply returns the specs with their IterationBound tightened per the
// task_kind override, with the cross-consistency rule applied.
//
// Cross-consistency (H9 / P1-AC-7):
//   - task_kind=review + tool.EmissionClass=EC_Probe → bound MUST be
//     Bounded (never OpenEnded). If the canonical bound is OpenEnded
//     (e.g. observe), force Bounded(15) instead.
//   - For non-Probe tools (Fact / Action / Experiment), the cross-
//     consistency rule does NOT apply. The tool's existing bound is
//     preserved (don't downgrade Fact to Bounded just because the
//     task_kind is review).
func (f *PerTaskKindFilter) Apply(specs []contracts.ToolSpec) []contracts.ToolSpec {
	out := make([]contracts.ToolSpec, len(specs))
	for i, s := range specs {
		out[i] = s

		// Cross-consistency rule (P1-AC-7): review + Probe → must be Bounded.
		// Only applies to Probe-class tools. For other classes, the
		// tool's existing bound is preserved.
		if f.TaskKind == "review" && s.EmissionClass == contracts.EC_Probe {
			// Force Bounded(15) regardless of the canonical bound.
			out[i].IterationBound = contracts.IterationBound{Kind: contracts.IB_Bounded, MaxN: 15}
			continue
		}

		// For other (Fact/Action/Experiment) tools, apply the task_kind
		// bound only if the spec already has a real Bounded bound.
		// OpenEnded tools (read-once Fact queries like query_diagnostics,
		// lsp_goto_definition) are NOT affected by the task_kind filter
		// — their OpenEnded contract is preserved.
		if s.IterationBound.Kind != contracts.IB_Bounded {
			continue
		}
		newBound := TaskKindBound(f.TaskKind)
		if isTighter(newBound, s.IterationBound) {
			out[i].IterationBound = newBound
		}
	}
	return out
}

// isTighter returns true if boundA is tighter (more restrictive) than boundB.
// OpenEnded is the loosest; Bounded(n) is tighter when n is smaller.
func isTighter(a, b contracts.IterationBound) bool {
	// OpenEnded is never tighter than anything.
	if a.Kind == contracts.IB_OpenEnded {
		return false
	}
	// If b is OpenEnded, a is always tighter.
	if b.Kind == contracts.IB_OpenEnded {
		return true
	}
	// Both Bounded: smaller MaxN is tighter.
	if a.Kind == contracts.IB_Bounded && b.Kind == contracts.IB_Bounded {
		return a.MaxN < b.MaxN
	}
	// Mixed: Bounded is always tighter than Quotient.
	if a.Kind == contracts.IB_Bounded && b.Kind == contracts.IB_Quotient {
		return true
	}
	return false
}
