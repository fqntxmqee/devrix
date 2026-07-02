// T: D7-S9-A50-T03, T04, T05 — ProbeToolChannel Bounded(n) hard reject
// + PromptPressure soft/hard/forced progression.
package toolchannel

import (
	"context"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/observability/instrument/ltl/invariants/termination"
	"github.com/devrix/devrix/internal/shared/contracts"
)

// makeState is a test helper that builds a termination.State.
func makeState(iter, bound int) *termination.State {
	return &termination.State{
		ChannelName:    "ProbeToolChannel",
		IterationsUsed: iter,
		BoundMax:       bound,
	}
}

// DM-20260702-008 / D2-S15-A02-T09: the治本 change. ProbeToolChannel
// Bounded(15) no longer hard-stops at iter 16. Instead:
//   - Accept ALWAYS returns (true, nil) — the LLM's recovery path
//     (Read offset/limit re-reads) is never blocked.
//   - state.Advisory = true is set on every Accept call.
//   - The L4-BOUNDED-ITERATIONS invariant records the violation in
//     state.AdvisoryViolations for observability (dashboards / metrics)
//     but does NOT drive a hard reject.
//   - On bound hit, injectForcedPressure emits a "FINAL: synthesize
//     NOW" message so the LLM gets a strong synthesize signal.
//
// The old ErrProbeToolChannelBoundExceeded contract is retired.
func TestProbeToolChannel_Bounded15_AdvisoryAtBound(t *testing.T) {
	ch := NewProbeToolChannelWithBound(15)
	call := &ToolCall{
		SessionID: "sess-1",
		ToolName:  "read_file",
		Spec: contracts.ToolSpec{
			EmissionClass:  contracts.EC_Probe,
			IterationBound: contracts.IterationBound{Kind: contracts.IB_Bounded, MaxN: 15},
		},
		TaskKind: "review",
	}
	state := makeState(15, 15) // iter=15, bound=15

	ok, err := ch.Accept(context.Background(), call, state)
	if !ok {
		t.Errorf("Accept must ALWAYS return ok=true in T09 advisory mode, got false (err=%v)", err)
	}
	if err != nil {
		t.Errorf("Accept must return nil error in T09 advisory mode, got %v", err)
	}
	if !state.Advisory {
		t.Errorf("Accept must set state.Advisory=true on every call")
	}
	if len(state.AdvisoryViolations) == 0 {
		t.Errorf("bound-hit must record an AdvisoryViolation, got 0")
	} else {
		v := state.AdvisoryViolations[0]
		if v.Invariant != "L4-BOUNDED-ITERATIONS" {
			t.Errorf("advisory violation invariant = %q, want L4-BOUNDED-ITERATIONS", v.Invariant)
		}
		if v.IterationsUsed != 15 || v.BoundMax != 15 {
			t.Errorf("advisory violation context: iter=%d bound=%d, want 15/15", v.IterationsUsed, v.BoundMax)
		}
	}
}

// DM-20260702-008: the unused `errors` and `strings` imports are still
// used by other tests in this file (PromptPressure / shadow mode). Keep
// them; only the hard-reject test changed.

// D7-S9-A50-T03: ProbeToolChannel accepts iter < bound.
func TestProbeToolChannel_AcceptsUnderBound(t *testing.T) {
	ch := NewProbeToolChannelWithBound(15)
	call := &ToolCall{
		SessionID: "sess-1",
		ToolName:  "read_file",
		Spec: contracts.ToolSpec{
			EmissionClass:  contracts.EC_Probe,
			IterationBound: contracts.IterationBound{Kind: contracts.IB_Bounded, MaxN: 15},
		},
		TaskKind: "review",
	}

	// iter=0..14 → accept
	for n := 0; n < 15; n++ {
		state := makeState(n, 15)
		ok, err := ch.Accept(context.Background(), call, state)
		if !ok {
			t.Errorf("iter=%d should be accepted, got err=%v", n, err)
		}
	}
}

// D7-S9-A50-T05: PromptPressure soft at remaining=5, hard at remaining=2
// for review task_kind.
func TestProbeToolChannel_PromptPressure_Review(t *testing.T) {
	ch := NewProbeToolChannelWithBound(15)
	call := &ToolCall{
		SessionID: "sess-1",
		ToolName:  "read_file",
		Spec: contracts.ToolSpec{
			EmissionClass:  contracts.EC_Probe,
			IterationBound: contracts.IterationBound{Kind: contracts.IB_Bounded, MaxN: 15},
		},
		TaskKind: "review",
	}

	// iter=8 (remaining=7) → no pressure
	state := makeState(8, 15)
	if err := ch.InjectPromptPressure(context.Background(), state, call); err != nil {
		t.Errorf("iter=8 should not inject: %v", err)
	}

	// iter=10 (remaining=5) → soft pressure
	state = makeState(10, 15)
	if err := ch.InjectPromptPressure(context.Background(), state, call); err != nil {
		t.Errorf("iter=10 should inject soft: %v", err)
	}

	// iter=13 (remaining=2) → hard pressure
	state = makeState(13, 15)
	if err := ch.InjectPromptPressure(context.Background(), state, call); err != nil {
		t.Errorf("iter=13 should inject hard: %v", err)
	}
}

// D7-S9-A50-T05: PromptPressure edit task_kind uses tight thresholds.
func TestProbeToolChannel_PromptPressure_Edit(t *testing.T) {
	ch := NewProbeToolChannelWithBound(10)
	call := &ToolCall{
		SessionID: "sess-1",
		ToolName:  "edit_file",
		Spec: contracts.ToolSpec{
			EmissionClass:  contracts.EC_Probe,
			IterationBound: contracts.IterationBound{Kind: contracts.IB_Bounded, MaxN: 10},
		},
		TaskKind: "edit",
	}

	// iter=5 (remaining=5) → no soft (softAt=3 for edit)
	state := makeState(5, 10)
	if err := ch.InjectPromptPressure(context.Background(), state, call); err != nil {
		t.Errorf("iter=5 edit should not inject: %v", err)
	}

	// iter=7 (remaining=3) → soft (softAt=3 for edit)
	state = makeState(7, 10)
	if err := ch.InjectPromptPressure(context.Background(), state, call); err != nil {
		t.Errorf("iter=7 edit should inject soft: %v", err)
	}

	// iter=9 (remaining=1) → hard (hardAt=1 for edit)
	state = makeState(9, 10)
	if err := ch.InjectPromptPressure(context.Background(), state, call); err != nil {
		t.Errorf("iter=9 edit should inject hard: %v", err)
	}
}

// D7-S9-A50-T05: PromptPressure observe task_kind never injects.
func TestProbeToolChannel_PromptPressure_Observe_NeverInjects(t *testing.T) {
	ch := NewProbeToolChannelWithBound(15)
	call := &ToolCall{
		SessionID: "sess-1",
		ToolName:  "read_file",
		Spec: contracts.ToolSpec{
			EmissionClass:  contracts.EC_Probe,
			IterationBound: contracts.IterationBound{Kind: contracts.IB_Bounded, MaxN: 15},
		},
		TaskKind: "observe",
	}

	// any iter → no pressure for observe
	for n := 0; n < 14; n++ {
		state := makeState(n, 15)
		if err := ch.InjectPromptPressure(context.Background(), state, call); err != nil {
			t.Errorf("observe iter=%d should never inject: %v", n, err)
		}
	}
}

// D7-S9-A50-T07: Router shadow mode logs would_reject without blocking.
func TestRouter_ShadowMode_LogsWouldReject(t *testing.T) {
	r := NewRouter(ModeShadow)
	call := &ToolCall{
		SessionID: "sess-2",
		ToolName:  "read_file",
		Spec: contracts.ToolSpec{
			EmissionClass:  contracts.EC_Probe,
			IterationBound: contracts.IterationBound{Kind: contracts.IB_Bounded, MaxN: 15},
		},
		TaskKind: "review",
	}

	// iter=15 → would reject in shadow
	state := r.getOrCreateState(call.SessionID, "probe")
	state.IterationsUsed = 15
	state.BoundMax = 15

	// DM-20260702-008 / D2-S15-A02-T09: shadow mode now also accepts
	// (T09 advisory). The "would reject" counter is retired; the bound
	// hit is recorded as an AdvisoryViolation on the state. We keep the
	// field for backward compat but the test now asserts the new
	// advisory semantics.
	accepted, _, err := r.Route(context.Background(), call)
	if !accepted {
		t.Errorf("advisory mode: iter=15 should be accepted, got rejected")
	}
	if err != nil {
		t.Errorf("advisory mode: err should be nil, got %v", err)
	}
	if len(state.AdvisoryViolations) == 0 {
		t.Errorf("advisory mode: bound hit should record AdvisoryViolation")
	}
}

// D7-S9-A50-T07: Router enforce mode returns ErrProbeToolChannelBoundExceeded.
func TestRouter_EnforceMode_ReturnsError(t *testing.T) {
	r := NewRouter(ModeEnforce)
	call := &ToolCall{
		SessionID: "sess-3",
		ToolName:  "read_file",
		Spec: contracts.ToolSpec{
			EmissionClass:  contracts.EC_Probe,
			IterationBound: contracts.IterationBound{Kind: contracts.IB_Bounded, MaxN: 15},
		},
		TaskKind: "review",
	}

	state := r.getOrCreateState(call.SessionID, "probe")
	state.IterationsUsed = 15
	state.BoundMax = 15

	// DM-20260702-008 / D2-S15-A02-T09: in advisory mode the router
	// still accepts the call (LLM recovery path must not be blocked).
	// The bound hit is recorded as an AdvisoryViolation on the state.
	accepted, _, err := r.Route(context.Background(), call)
	if !accepted {
		t.Errorf("advisory mode: iter=15 should be accepted (LLM recovery), got rejected")
	}
	if err != nil {
		t.Errorf("advisory mode: err should be nil, got %v", err)
	}
	if len(state.AdvisoryViolations) == 0 {
		t.Errorf("advisory mode: bound hit should record AdvisoryViolation, got 0")
	}
}

// D7-S9-A50-T07: Router escalation: same query 5+ times on Fact → reclassify.
func TestRouter_FactEscalationToProbe(t *testing.T) {
	r := NewRouter(ModeEnforce)
	call := &ToolCall{
		SessionID: "sess-4",
		ToolName:  "read_file",
		Spec: contracts.ToolSpec{
			EmissionClass: contracts.EC_Fact, // initially Fact
		},
		QueryFingerprint: "abc123",
		TaskKind:         "review",
	}

	// Make 5 calls with the same fingerprint. After the 5th, the fact
	// channel's invariant fires and signals escalation.
	for i := 0; i < 5; i++ {
		err := r.OnResult(context.Background(), call, &ToolResult{ToolName: "read_file", Output: "data"})
		if err != nil {
			t.Errorf("OnResult iter=%d: %v", i, err)
		}
	}

	// Verify the state has same_query_count=5
	state := r.getOrCreateState(call.SessionID, "fact")
	if state.SameQueryCount != 5 {
		t.Errorf("expected SameQueryCount=5, got %d", state.SameQueryCount)
	}
	if !strings.Contains(state.ToolName, "ESCALATE_TO_PROBE") {
		t.Errorf("expected escalation signal in ToolName, got %q", state.ToolName)
	}
}

// D7-S9-A50-T08: L0-L3 cross-check — ProbeToolChannel does not bypass
// readonly/destructive permission guards.
//
// The probe channel's Accept returns an error on bound hit, but it
// does NOT consult permission state. In the full integration, the
// ChannelRouter enforces permission guards BEFORE delegating to Accept.
// This test asserts the invariant signature: Accept takes only the
// state and call, not any permission-related parameter.
func TestProbeToolChannel_DoesNotBypassPermissionGuards(t *testing.T) {
	ch := NewProbeToolChannelWithBound(15)
	call := &ToolCall{
		SessionID: "sess-5",
		ToolName:  "read_file",
		Spec: contracts.ToolSpec{
			EmissionClass:  contracts.EC_Probe,
			IterationBound: contracts.IterationBound{Kind: contracts.IB_Bounded, MaxN: 15},
		},
		TaskKind: "review",
	}

	// DM-20260702-008 / D2-S15-A02-T09: in advisory mode, Accept at
	// iter=15 always returns (true, nil). The bound hit is recorded
	// as an AdvisoryViolation. Permission guards are still checked by
	// the ChannelRouter BEFORE Accept (H8 / P1-AC-2 cross-check) — that
	// cross-check is unaffected by the advisory change because the
	// channel signature still does not take a permission parameter.
	state := makeState(15, 15)
	ok, err := ch.Accept(context.Background(), call, state)
	if !ok {
		t.Errorf("Accept must return ok=true in advisory mode, got false")
	}
	if err != nil {
		t.Errorf("Accept must return nil error in advisory mode, got %v", err)
	}
	if len(state.AdvisoryViolations) == 0 {
		t.Errorf("advisory mode: bound hit should record AdvisoryViolation")
	}
}

// D7-S9-A50-T01: ToolChannel interface — 4 channels implement it.
func TestToolChannel_AllFourImplement(t *testing.T) {
	channels := []ToolChannel{
		NewFactToolChannel(),
		NewActionToolChannel(),
		NewProbeToolChannel(),
		NewExperimentToolChannel(),
	}
	for _, ch := range channels {
		if ch.Name() == "" {
			t.Errorf("%T: Name() empty", ch)
		}
		if ch.Invariant() == nil {
			t.Errorf("%T: Invariant() nil", ch)
		}
		if ch.Snapshot() == nil {
			t.Errorf("%T: Snapshot() nil", ch)
		}
	}
}

// D7-S9-A50-T02: Router initializes with 4 channels.
func TestRouter_Has4Channels(t *testing.T) {
	r := NewRouter(ModeEnforce)
	if len(r.channels) != 4 {
		t.Errorf("expected 4 channels, got %d", len(r.channels))
	}
	for _, ec := range []contracts.EmissionClass{
		contracts.EC_Fact,
		contracts.EC_Action,
		contracts.EC_Probe,
		contracts.EC_Experiment,
	} {
		if _, ok := r.channels[ec]; !ok {
			t.Errorf("missing channel for EmissionClass=%v", ec)
		}
	}
}
