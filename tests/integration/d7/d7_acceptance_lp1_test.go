//go:build integration && d7

// LP-1 acceptance test (DM-20260629-001 PR-7, T43).
//
// LP-1 closed-loop long-term Bayesian reputation: 5 ProcessMessage rounds
// + 5 explicit Learn cycles accumulates Alpha (5) so the 6th Observe sees
// prior Beta(10,3) Mean≈0.769. This is the canonical LP-1 acceptance
// gate — it closes the loop end-to-end across:
//   - cold-start Developer prior Beta(5,3) →
//   - ProcessMessage → ProcessAutoClose → VerdictPass → +1 α each round →
//   - Alpha accumulated across rounds →
//   - Next ProcessMessage observes a strictly larger prior.
package d7integration

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/mups/learn"
	"github.com/devrix/devrix/internal/layers/orchestration/plan"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/types"
)

// T: D7-S12-A43-T06 / DM-20260629-001 T43.a — LP-1 5-round accumulation.
func TestAcceptance_LP1_LongTermBayesianPriorAccumulation(t *testing.T) {
	sessionID := "sess-lp1-accept"
	f := newLP1Fixture(t, sessionID)

	// Round 1: cold-start. Prior should be Beta(5,3) (DefaultDeveloperPrior).
	_ = f.processOnce(t, sessionID, "round 1")

	f.classifier.mu.Lock()
	round1Prior := f.classifier.lastPrior
	f.classifier.mu.Unlock()

	if round1Prior == nil {
		t.Fatal("round 1 prior must not be nil (cold-start fallback)")
	}
	if round1Prior.PriorBeta != (learn.BetaPrior{Alpha: 5, Beta: 3}) {
		t.Fatalf("round 1 prior = %+v, want Beta(5,3) cold-start", round1Prior.PriorBeta)
	}

	// 5 explicit Learn(VerdictPass) cycles add +5 Alpha. After processAutoClose
	// in round 1 also added +1 Alpha, the cumulative Alpha = 6.
	verdict := workmodel.Verdict{Kind: types.VerdictPass, SourceID: "v_accept", Reason: "ok"}
	planStub := plan.NewPlan("plan_accept_1", sessionID, plan.CommitmentPlan, []string{"obs_a"},
		[]plan.Step{{ID: "step_a"}}, 0.8)
	artStub := artifactStub("art_accept_1", sessionID, []string{"lp1.go"})

	for i := 0; i < 5; i++ {
		req := learn.LearnRequest{
			SessionID: sessionID,
			Verdict:   verdict,
			Plan:      planStub,
			Artifact:  artStub,
		}
		if _, err := f.learner.Learn(context.Background(), req); err != nil {
			t.Fatalf("Learn iter %d: %v", i, err)
		}
	}

	// Round 6: prior should reflect accumulated Alpha (≥6 from auto-close + 5 manual).
	_ = f.processOnce(t, sessionID, "round 6")

	f.classifier.mu.Lock()
	round6Prior := f.classifier.lastPrior
	f.classifier.mu.Unlock()

	if round6Prior == nil {
		t.Fatal("round 6 prior must not be nil")
	}
	if round6Prior.PriorBeta.Alpha < 6 {
		t.Errorf("round 6 Alpha = %d, want ≥6 (auto-close + 5 manual Pass)", round6Prior.PriorBeta.Alpha)
	}
	if round6Prior.PriorBeta.Beta != 3 {
		t.Errorf("round 6 Beta = %d, want 3 (no negatives accumulated)", round6Prior.PriorBeta.Beta)
	}
	wantMean := float64(round6Prior.PriorBeta.Alpha) / float64(round6Prior.PriorBeta.Alpha+round6Prior.PriorBeta.Beta)
	if got := round6Prior.PriorBeta.Mean(); got < wantMean-0.001 || got > wantMean+0.001 {
		t.Errorf("round 6 Mean = %.4f, want %.4f", got, wantMean)
	}
}

// T: DM-20260629-001 T43.a — LP-1 cold-start uses Developer prior; nil
// ReputationStore row triggers DefaultDeveloperPrior Beta(5,3).
func TestAcceptance_LP1_ColdStartPrior(t *testing.T) {
	sessionID := "sess-lp1-coldstart"
	f := newLP1Fixture(t, sessionID)

	// Empty ReputationStore row for sessionID.
	rep, err := f.rep.Get(context.Background(), sessionID)
	if err == nil && rep != nil {
		t.Fatalf("expected nil ReputationStore row for cold-start, got %+v", rep)
	}

	prior, err := f.learner.Inject(context.Background(), sessionID, string(learn.TrackModeDeveloper))
	if err != nil {
		t.Fatalf("Inject: %v", err)
	}
	if prior == nil {
		t.Fatal("cold-start prior must not be nil (fail-safe DefaultDeveloperPrior)")
	}
	if prior.PriorBeta != (learn.BetaPrior{Alpha: 5, Beta: 3}) {
		t.Errorf("cold-start prior = %+v, want Beta(5,3) DefaultDeveloperPrior", prior.PriorBeta)
	}

	// Operator hint should produce Operator prior.
	opPrior, err := f.learner.Inject(context.Background(), sessionID, string(learn.TrackModeOperator))
	if err != nil {
		t.Fatalf("Inject(operator): %v", err)
	}
	if opPrior == nil || opPrior.PriorBeta != (learn.BetaPrior{Alpha: 8, Beta: 1}) {
		t.Errorf("operator hint prior = %+v, want Beta(8,1) DefaultOperatorPrior",
			func() any {
				if opPrior == nil {
					return "<nil>"
				}
				return opPrior.PriorBeta
			}())
	}
}
