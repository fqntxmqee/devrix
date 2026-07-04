package toolround

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/shared/contracts"
)

// T: D2-S18-A90-T02 — Router dispatches 4 emission-class channels.
func TestRouter_FourChannelDispatch(t *testing.T) {
	r := NewRouter(ModeShadow)
	for _, ec := range []contracts.EmissionClass{
		contracts.EC_Fact, contracts.EC_Action, contracts.EC_Probe, contracts.EC_Experiment,
	} {
		ch, ok := r.channels[ec]
		if !ok || ch == nil {
			t.Fatalf("missing channel for %v", ec)
		}
		if ch.EmissionClass() != ec {
			t.Fatalf("channel class = %v, want %v", ch.EmissionClass(), ec)
		}
	}
}

// T: D2-S18-A90-T01 — probe iter at bound injects pressure path.
func TestCoordinator_ProbePressureAtBound(t *testing.T) {
	c := NewCoordinator()
	spec := contracts.ToolSpec{
		Name:           "grep",
		EmissionClass:  contracts.EC_Probe,
		IterationBound: contracts.IterationBound{Kind: contracts.IB_Bounded, MaxN: 15},
	}
	out := c.AfterToolCall(context.Background(), RoundInput{
		SessionID:       "s1",
		TaskKind:        "review",
		RemainingBudget: 1,
	}, spec, "grep")
	if len(out.PressureMessages) == 0 {
		t.Fatal("expected pressure message when remaining budget low")
	}
}
