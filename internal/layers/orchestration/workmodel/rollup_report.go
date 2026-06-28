package workmodel

import (
	"time"

	"github.com/devrix/devrix/internal/shared/types"
)

// RollupReport is the typed rollup signal bundle a parent reads from a
// child's last pipeline round. Replaces the previous "child.LastRound.*"
// scattered-field reads with a single typed envelope so the rollup
// pipeline (ApplyPipelineDecide, ReevaluateParentAfterChild, Path A/B
// gates, ExtractSessionDeliverable, StructuredBubbleStatement) all
// pass through one stable contract.
//
// DM-20260629-001 / PR-3-extended / T52: RollupReport struct replaces
// the implicit child.LastRound envelope with 5 data fields + 2 metadata.
//
// 5 data fields (the rollup signal):
//   - VerdictKind     — child run verdict (pass/fail/abstain/indeterminate)
//   - ArtifactSummary — execute-phase artifact Summary text
//   - UncertaintyMean — child run uncertainty after learning
//   - SpawnPolicy     — final spawn policy (decompose/await/inline/...)
//   - BubbleKind      — child → parent bubble kind (none/structured/...)
//
// 2 metadata fields:
//   - ChildID         — owning child WorkItem ID
//   - GeneratedAt     — when this report was constructed (UTC)
//
// Construct via NewRollupReportFromRound; the constructor is a pure
// aggregation read (no business logic). All five data fields are zero
// when the child has not yet run (round == nil) — callers must check
// the embedded round if a "ran at least once" signal is required.
type RollupReport struct {
	// Data fields (5)
	VerdictKind     types.VerdictKind
	ArtifactSummary string
	UncertaintyMean float64
	SpawnPolicy     SpawnPolicy
	BubbleKind      ContextBubbleKind

	// Metadata fields (2)
	ChildID     string
	GeneratedAt time.Time
}

// NewRollupReportFromRound aggregates the rollup-relevant fields from a
// child WorkItem's last pipeline round into a typed envelope. Returns
// nil when round is nil so callers can early-out without dereferencing.
//
// This constructor is the single entry point for the 5
// `child.LastRound.*` / `parent.LastRound.*` / `root.LastRound.*` reads
// called out in DM-20260629-001 / T53 (context_bubble_apply.go:67/188
// + rollup_gate.go:36/135/154). Adding a new rollup field requires
// updating both this struct and the constructor — no other call site
// needs to change.
func NewRollupReportFromRound(childID string, round *WorkItemPipelineRound) *RollupReport {
	if round == nil {
		return nil
	}
	return &RollupReport{
		VerdictKind:     round.VerdictKind,
		ArtifactSummary: round.ArtifactSummary,
		UncertaintyMean: round.UncertaintyMean,
		SpawnPolicy:     round.SpawnPolicy,
		BubbleKind:      round.ContextBubbleKind,
		ChildID:         childID,
		GeneratedAt:     time.Now().UTC(),
	}
}