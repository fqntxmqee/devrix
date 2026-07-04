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
	report, _, err := observeWorkItem(context.Background(), "s1", goal, nil, nil, "", tm, proposer)
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

