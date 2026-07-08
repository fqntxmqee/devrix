// Package testutil — d7_frame_delta_helpers.go
//
// DM-20260706-001 (devrix-d7-frame-delta-phase1-2-span-trigger) testutil-only
// helpers for FrameDelta e2e tests.
//
// Helpers in this file are TESTUTIL ONLY. They do not touch production
// code paths — they only mutate the in-memory TaskManager state held by a
// D7TestStack so the next pipeline round's Observe node sees a non-empty
// prior context (BuildObservePriorDelta then emits the Phase 2
// observe.prior_delta span with span_tag_complete=true).
//
// Production-side Phase 2 wiring is fixed by sibling change
// devrix-d7-frame-delta-phase2-production-wiring (DM-20260706-004). Until
// that lands, Phase 2 e2e span emit is gated on this testutil seed + the
// Stack registering a WorkItemExecContext-aware observer. See
// design.md §6.1 for the seed-target field rationale
// (Item.LastRound.ArtifactSummary, NOT WorkItemExecContext.ConvergenceMetric).

package testutil

import (
	"strconv"
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
)

// MaxPriorArtifactSummaryChars mirrors interfaces.MaxPriorArtifactSummaryChars
// (≤80). BuildObservePriorDelta truncates with "..." suffix when over-budget.
// Duplicated here to avoid importing the interfaces package from testutil
// (testutil is meant to be import-light).
const MaxPriorArtifactSummaryChars = 80

// SeedPriorExecContext seeds the in-memory WorkItem state held by
// stack.TaskManager so BuildObservePriorDelta returns a non-zero FrameDelta
// on the next Observe call. Mutates the targeted WorkItem's
// LastRound.ArtifactSummary (the field BuildObservePriorDelta actually
// reads at observe_frame_delta.go:52).
//
// TESTUTIL ONLY (DM-20260706-001, AC1 + AC5 cross-ref to DM-20260706-004).
// Never call from production code.
//
// Parameters:
//   - t             — *testing.T for t.Helper() + t.Cleanup.
//   - stack         — the D7TestStack returned by NewD7TestStack.
//   - sessionID     — the session ID of the work item to seed.
//   - workItemID    — the work item ID to seed.
//   - artifactSummary — the prior round's ≤80-char artifact summary. Empty
//                    string is allowed but yields zero FrameDelta (no span).
//
// Cleanup: the helper registers a t.Cleanup that resets the WorkItem's
// LastRound to whatever it was before seeding, so test order is independent.
//
// Returns the WorkItemPipelineRound that was set, so callers can inspect
// round_no, plan_id, etc. in their assertions.
func SeedPriorExecContext(
	t *testing.T,
	stack *D7TestStack,
	sessionID, workItemID, artifactSummary string,
) *workmodel.WorkItemPipelineRound {
	t.Helper()
	if stack == nil || stack.TaskManager == nil {
		t.Fatalf("SeedPriorExecContext: stack/TaskManager nil")
	}
	if sessionID == "" || workItemID == "" {
		t.Fatalf("SeedPriorExecContext: sessionID and workItemID required")
	}
	if len(artifactSummary) > MaxPriorArtifactSummaryChars {
		t.Fatalf("SeedPriorExecContext: artifactSummary len=%d > MaxPriorArtifactSummaryChars=%d (BuildObservePriorDelta truncates with '...'; pre-truncate to avoid silent loss)",
			len(artifactSummary), MaxPriorArtifactSummaryChars)
	}
	wi, ok := stack.TaskManager.GetWorkItem(sessionID, workItemID)
	if !ok || wi == nil {
		t.Fatalf("SeedPriorExecContext: work item %s/%s not found in TaskManager", sessionID, workItemID)
	}
	prevLastRound := wi.LastRound

	round := &workmodel.WorkItemPipelineRound{
		RoundNo:         1,
		WorkItemID:      workItemID,
		SessionID:       sessionID,
		ArtifactSummary: artifactSummary,
		// Stamp a non-empty plan_id so any pipeline consumer that reads
		// LastRound.PlanID sees a valid value (covers legacy fields).
		PlanID:  "testutil_seed_plan",
		VerdictID: "testutil_seed_verdict",
	}
	wi.LastRound = round
	if err := stack.TaskManager.Tree().ApplyPipelineRound(sessionID, workItemID, round, workmodel.RoundPhaseObserve); err != nil {
		t.Fatalf("SeedPriorExecContext: ApplyPipelineRound: %v", err)
	}

	t.Cleanup(func() {
		wi, ok := stack.TaskManager.GetWorkItem(sessionID, workItemID)
		if !ok || wi == nil {
			return
		}
		wi.LastRound = prevLastRound
		phase := workmodel.RoundPhaseIdle
		if prevLastRound != nil {
			phase = workmodel.RoundPhaseObserve
		}
		_ = stack.TaskManager.Tree().ApplyPipelineRound(sessionID, workItemID, prevLastRound, phase)
	})

	return round
}

// FormatConvergenceRate formats a 0..1 rate as a fixed 4-decimal string
// (e.g. 0.85 → "0.8500"). Used by tests for stable log output.
func FormatConvergenceRate(rate float64) string {
	return strconv.FormatFloat(rate, 'f', 4, 64)
}