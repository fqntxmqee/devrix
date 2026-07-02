package filter

import (
	"github.com/devrix/devrix/internal/shared/contracts"
)

// PerTaskKindFilter applies ADVISORY task_kind hints based on the
// inferred task_kind. The 5 task kinds and their hint overrides:
//
//	review   → Bounded(15)   hint  (LLM may need to read many files)
//	edit     → Bounded(10)   hint  (tighter; user wants changes)
//	test     → Bounded(12)   hint  (test runs have stable shape)
//	observe  → OpenEnded     (no convergence pressure)
//	refactor → Bounded(8)    hint  (tightest; refactor scope is bounded)
//
// DM-20260702-008 / D2-S15-A02-T12: hints are advisory only. The
// filter no longer forces Bounded(15) on Probe tools for review —
// the治本 change in T09 means the channel never hard-rejects anyway,
// so the cross-consistency rule from H9 / P1-AC-7 is relaxed: Probe
// tools keep their declared bound (OpenEnded for read_file/grep/glob
// in T11), and the task_kind hint is exposed via the spec for
// dashboards / pressure thresholds, not for rejection.
//
// DSAFT: D2-S15-A02-T03 (4 task_kind mapping) + T12 (advisory hints).
type PerTaskKindFilter struct {
	// TaskKind is the inferred user task kind.
	TaskKind string
}

// NewPerTaskKindFilter constructs a PerTaskKindFilter.
func NewPerTaskKindFilter(taskKind string) *PerTaskKindFilter {
	return &PerTaskKindFilter{TaskKind: taskKind}
}

// TaskKindBound returns the ADVISORY IterationBound hint for a task kind.
// In T12 these are hints only — ProbeToolChannel.Accept never hard-rejects
// regardless of the bound. The hints are exposed via Apply for dashboards
// and pressure thresholds.
//
// Per-task-kind hints (D2-S15-A02-T03 → T12 advisory):
//
//	review:   Bounded(15)   hint
//	edit:     Bounded(10)   hint
//	test:     Bounded(12)   hint
//	observe:  OpenEnded
//	refactor: Bounded(8)    hint
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

		// T12 (DM-20260702-008): the old cross-consistency rule
		// (review + Probe → force Bounded(15), P1-AC-7) is RELAXED.
		// ProbeToolChannel never hard-rejects (T09), so the rule's
		// purpose (prevent an OpenEnded Probe from escaping the bound)
		// is moot. We preserve the spec's declared bound; the
		// task_kind hint is exposed via TaskKindHint(s.Name) for
		// dashboards and pressure thresholds, not for spec mutation.

		// For tools that DO have a Bounded bound (Bash, Write, etc.),
		// still tighten it if the task_kind hint is tighter. OpenEnded
		// tools (read_file, grep, glob after T11; query_diagnostics;
		// lsp_*) keep their OpenEnded contract — the hint is advisory.
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

// TaskKindHint returns the advisory bound for a tool name, exposed
// for dashboards / pressure thresholds (T12). Returns (zero, false)
// when the tool has no task_kind-specific hint.
func TaskKindHint(toolName string) (contracts.IterationBound, bool) {
	switch toolName {
	case "read_file", "grep", "glob":
		return TaskKindBound("review"), true
	}
	return contracts.IterationBound{}, false
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
