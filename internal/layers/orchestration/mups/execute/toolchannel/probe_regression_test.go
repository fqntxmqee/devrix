// T: D2-S15-A02-T28 — 8K self-loop regression test.
//
// Historical context: PR #373 (channel/feishu task_incomplete) exposed a
// pathological loop where the 8K TruncateToTokens + Bounded(15) hard
// reject combination forced the LLM to keep re-issuing read_file calls
// past the bound, getting the same truncated result back, looping
// until the user killed the task. The fix (Token Design 2.0):
//   - PersistToFile writes full content to disk (T01)
//   - ProbeToolChannel.Accept is advisory (T09) — never hard-rejects
//   - read_file supports offset/limit for the recovery path (T10)
//
// This test asserts the治本 invariant: 20 consecutive read_file calls
// all get accepted (no hard reject), so the LLM can recover via
// offset/limit re-reads. Under the old design, calls 16-20 would have
// been rejected and the LLM would have been stuck.
package toolchannel

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/shared/contracts"
)

// D2-S15-A02-T28: 20 consecutive read_file calls all accepted.
// The historical 8K self-loop had the LLM call read_file 20+ times to
// recover from a truncated result; the old Bounded(15) hard reject
// would have killed the 16th call. The new advisory mode accepts all
// 20 — the LLM is free to use Read offset/limit (T10) to recover.
func TestProbeToolChannel_8K_SelfLoop_Regression_20CallsAccepted(t *testing.T) {
	ch := NewProbeToolChannelWithBound(15)
	call := &ToolCall{
		SessionID: "sess-regression",
		ToolName:  "read_file",
		Spec: contracts.ToolSpec{
			EmissionClass:  contracts.EC_Probe,
			IterationBound: contracts.IterationBound{Kind: contracts.IB_Bounded, MaxN: 15},
		},
		TaskKind: "review",
	}

	for n := 0; n < 20; n++ {
		state := makeState(n, 15)
		ok, err := ch.Accept(context.Background(), call, state)
		if !ok {
			t.Errorf("call %d/20: must be accepted in advisory mode, got rejected (err=%v)", n+1, err)
		}
		if err != nil {
			t.Errorf("call %d/20: must not return error in advisory mode, got %v", n+1, err)
		}
	}

	// Also verify the bound hits were recorded as advisory violations
	// (calls 15-19, 0-indexed, i.e. iter=15..19). That's 5 violations.
	state := makeState(19, 15)
	_, _ = ch.Accept(context.Background(), call, state)
	if len(state.AdvisoryViolations) == 0 {
		t.Errorf("20th call (iter=19) must record an AdvisoryViolation (the bound was hit at iter=15)")
	}
}

// D2-S15-A02-T28: the L4-BOUNDED-ITERATIONS invariant still fires in
// advisory mode (for observability), but the channel returns ok=true.
// This is the治本 observability contract: dashboards can still see
// "bound was hit" without the LLM being blocked.
func TestProbeToolChannel_8K_SelfLoop_ObservabilityPreserved(t *testing.T) {
	ch := NewProbeToolChannelWithBound(15)
	call := &ToolCall{
		SessionID: "sess-obs",
		ToolName:  "read_file",
		Spec: contracts.ToolSpec{
			EmissionClass:  contracts.EC_Probe,
			IterationBound: contracts.IterationBound{Kind: contracts.IB_Bounded, MaxN: 15},
		},
		TaskKind: "review",
	}

	// 15 calls under the bound: no violation
	state := makeState(14, 15)
	_, _ = ch.Accept(context.Background(), call, state)
	if len(state.AdvisoryViolations) != 0 {
		t.Errorf("under-bound calls must not record violations, got %d", len(state.AdvisoryViolations))
	}

	// 16th call (iter=15): bound hit, violation recorded
	state = makeState(15, 15)
	ok, _ := ch.Accept(context.Background(), call, state)
	if !ok {
		t.Errorf("16th call must be accepted in advisory mode")
	}
	if len(state.AdvisoryViolations) == 0 {
		t.Errorf("16th call (iter=15, at bound) must record AdvisoryViolation")
	} else {
		v := state.AdvisoryViolations[0]
		if v.Invariant != "L4-BOUNDED-ITERATIONS" {
			t.Errorf("violation.Invariant = %q, want L4-BOUNDED-ITERATIONS", v.Invariant)
		}
		if v.IterationsUsed != 15 {
			t.Errorf("violation.IterationsUsed = %d, want 15", v.IterationsUsed)
		}
	}
}
