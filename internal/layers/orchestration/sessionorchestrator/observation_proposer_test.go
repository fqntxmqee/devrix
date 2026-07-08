package sessionorchestrator

import (
	"context"
	"fmt"
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
)

func TestValidateObservationProposals_CapsObsFactStrength(t *testing.T) {
	proposals := []ObservationProposal{{
		Kind:      orchtypes.ObsFact,
		Category:  orchtypes.CatBusiness,
		Strength:  0.99,
		Statement: "scope looks complete",
		Evidence:  []string{"goal_1"},
	}}
	obs, err := ValidateObservationProposals(proposals, "s1", "goal_1")
	if err != nil {
		t.Fatal(err)
	}
	if len(obs) != 1 {
		t.Fatalf("obs = %d", len(obs))
	}
	if obs[0].Strength > maxLLMObsFactStrength {
		t.Fatalf("strength = %v, want <= %v", obs[0].Strength, maxLLMObsFactStrength)
	}
	if obs[0].Source != llmObsProposerSource {
		t.Fatalf("source = %q", obs[0].Source)
	}
}

// T: D7-S16-A96-T01 (DM-20260704-005) ValidateObservationProposals caps at 3 valid proposals.
func TestValidateObservationProposals_CapsAtThree(t *testing.T) {
	proposals := make([]ObservationProposal, 5)
	for i := range proposals {
		proposals[i] = ObservationProposal{
			Kind:      orchtypes.ObsUncertainty,
			Category:  orchtypes.CatBusiness,
			Strength:  0.5,
			Question:  fmt.Sprintf("question %d", i+1),
			Evidence:  []string{"goal_1"},
		}
	}
	obs, err := ValidateObservationProposals(proposals, "s1", "goal_1")
	if err != nil {
		t.Fatal(err)
	}
	if len(obs) != maxObservationProposals {
		t.Fatalf("obs = %d, want %d", len(obs), maxObservationProposals)
	}
}

func TestValidateObservationProposals_SkipsInvalidBeforeCap(t *testing.T) {
	proposals := []ObservationProposal{
		{Kind: orchtypes.ObsFact, Category: orchtypes.CatBusiness, Strength: 0.5, Statement: ""}, // invalid
		{Kind: orchtypes.ObsUncertainty, Category: orchtypes.CatBusiness, Strength: 0.5, Question: "q1", Evidence: []string{"goal_1"}},
		{Kind: orchtypes.ObsUncertainty, Category: orchtypes.CatBusiness, Strength: 0.5, Question: "q2", Evidence: []string{"goal_1"}},
		{Kind: orchtypes.ObsUncertainty, Category: orchtypes.CatBusiness, Strength: 0.5, Question: "q3", Evidence: []string{"goal_1"}},
		{Kind: orchtypes.ObsUncertainty, Category: orchtypes.CatBusiness, Strength: 0.5, Question: "q4", Evidence: []string{"goal_1"}},
	}
	obs, err := ValidateObservationProposals(proposals, "s1", "goal_1")
	if err != nil {
		t.Fatal(err)
	}
	if len(obs) != 3 {
		t.Fatalf("obs = %d, want 3 (invalid skipped, then cap)", len(obs))
	}
	if p, ok := obs[0].Payload.(orchtypes.UncertaintyPayload); !ok || p.Question != "q1" {
		t.Fatalf("first valid = %+v", obs[0])
	}
}

func TestObserveWorkItem_MergesValidatedProposals(t *testing.T) {
	tm := workmodel.NewTaskManager()
	goal, _ := tm.EnsureGoal("s1", "build feature")
	proposer := StaticObservationProposer{Proposals: []ObservationProposal{{
		Kind:      orchtypes.ObsUncertainty,
		Category:  orchtypes.CatBusiness,
		Strength:  0.6,
		Question:  "Need API version?",
		Evidence:  []string{"goal_1"},
	}}}
	report, _, _, err := observeWorkItem(context.Background(), "s1", goal, nil, nil, "", tm, nil, proposer)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, o := range report.Observations {
		if o.Kind == orchtypes.ObsUncertainty && o.Source == llmObsProposerSource {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected observation_proposer uncertainty in report: %+v", report.Observations)
	}
}

// T: D7-S17-A98-T03 (DM-20260706-004 devrix-d7-frame-delta-phase2-production-wiring)
// AC2 unit gate: when observeWorkItem receives a non-nil prevExecCtx
// whose Item.LastRound.ArtifactSummary is non-empty, the buildObserveSignalInput
// call inside mergeProposedObservations must populate
// in.PriorArtifactSummary. This guards the Phase 2 wiring fix
// (item_pipeline.go constructs prevExecCtx above the Observe call).
//
// Before the fix: nil was hardcoded at observation_proposer.go:257, so
// PriorArtifactSummary stayed empty regardless of the prior round's
// ArtifactSummary. After the fix: prevExecCtx flows through, and the LLM
// user frame's prior_artifact_summary tag becomes populated on Round 2+.
func TestObserveWorkItem_NonFirstRoundPopulatesPriorArtifactSummary(t *testing.T) {
	tm := workmodel.NewTaskManager()
	goal, _ := tm.EnsureGoal("s1", "review plan directory")
	// Simulate a prior Execute round by attaching a LastRound with a
	// representative ArtifactSummary. BuildObservePriorDelta reads this
	// field to construct FrameDelta.PriorArtifactSummary.
	goal.LastRound = &workmodel.WorkItemPipelineRound{
		RoundNo:         1,
		WorkItemID:      goal.ID,
		ArtifactSummary: "Round 1: 4 files reviewed; 1 file needs deeper analysis",
	}
	prevExecCtx := &WorkItemExecContext{
		Item:  goal,
		Tasks: tm,
	}
	var captured ObserveSignalInput
	captureProposer := captureProposerFunc(func(in ObserveSignalInput) {
		captured = in
	})
	report, _, _, err := observeWorkItem(
		context.Background(),
		"s1",
		goal,
		nil,
		nil,
		"",
		tm,
		prevExecCtx,
		captureProposer,
	)
	if err != nil {
		t.Fatalf("observeWorkItem: %v", err)
	}
	_ = report
	if captured.PriorArtifactSummary == "" {
		t.Fatalf("PriorArtifactSummary expected to be populated from prev round, got empty: in=%+v", captured)
	}
	if captured.PriorArtifactSummary != goal.LastRound.ArtifactSummary {
		// Truncation to MaxPriorArtifactSummaryChars (80) is allowed.
		trimmed := goal.LastRound.ArtifactSummary
		if len(trimmed) > 80 {
			trimmed = trimmed[:80-3] + "..."
		}
		if captured.PriorArtifactSummary != trimmed {
			t.Errorf("PriorArtifactSummary=%q want %q (raw or 80-truncated)",
				captured.PriorArtifactSummary, trimmed)
		}
	}
}

// T: D7-S17-A98-T04 (DM-20260706-004) AC2 first-round invariant: when
// prevExecCtx is nil OR prevExecCtx.Item.LastRound is nil,
// buildObserveSignalInput must NOT set PriorArtifactSummary — the
// FrameDelta stays zero-value and the prior_delta_empty span fires
// (matches testutil AC2 baseline in d7_frame_delta_e2e_test.go).
func TestObserveWorkItem_FirstRoundLeavesPriorArtifactSummaryEmpty(t *testing.T) {
	tm := workmodel.NewTaskManager()
	goal, _ := tm.EnsureGoal("s1", "fresh directive")
	var captured ObserveSignalInput
	captureProposer := captureProposerFunc(func(in ObserveSignalInput) {
		captured = in
	})
	// Case A: nil prevExecCtx (very first round of a fresh WorkItem).
	if _, _, _, err := observeWorkItem(
		context.Background(), "s1", goal, nil, nil, "", tm, nil, captureProposer,
	); err != nil {
		t.Fatalf("observeWorkItem nil prevExecCtx: %v", err)
	}
	if captured.PriorArtifactSummary != "" {
		t.Errorf("nil prevExecCtx → PriorArtifactSummary=%q want empty",
			captured.PriorArtifactSummary)
	}
	// Case B: prevExecCtx with LastRound=nil (new WorkItem whose prior
	// round slot hasn't been initialised).
	var capturedB ObserveSignalInput
	captureProposerB := captureProposerFunc(func(in ObserveSignalInput) {
		capturedB = in
	})
	if _, _, _, err := observeWorkItem(
		context.Background(), "s1", goal, nil, nil, "", tm,
		&WorkItemExecContext{Item: goal, Tasks: tm},
		captureProposerB,
	); err != nil {
		t.Fatalf("observeWorkItem LastRound=nil: %v", err)
	}
	if capturedB.PriorArtifactSummary != "" {
		t.Errorf("LastRound=nil → PriorArtifactSummary=%q want empty",
			capturedB.PriorArtifactSummary)
	}
}

// captureProposerFunc adapts a closure to the ObservationProposer interface
// so the test can record the ObserveSignalInput fed to the LLM.
type captureProposerFunc func(in ObserveSignalInput)

func (f captureProposerFunc) ProposeObservations(_ context.Context, in ObserveSignalInput) ([]ObservationProposal, error) {
	f(in)
	return nil, nil
}

