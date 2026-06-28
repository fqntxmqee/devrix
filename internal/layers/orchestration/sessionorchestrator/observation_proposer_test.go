package sessionorchestrator

import (
	"context"
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
		t.Fatalf("expected llm_observation_proposer uncertainty in report: %+v", report.Observations)
	}
}

func TestParseObservationProposalsJSON(t *testing.T) {
	raw := `[{"kind":"obs_fact","strength":0.7,"statement":"ok","evidence":["wi_1"]}]`
	got, err := parseObservationProposalsJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Kind != orchtypes.ObsFact {
		t.Fatalf("got = %+v", got)
	}
}
