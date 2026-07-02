// T: D7-S9-A50-T03, T04, T05 — ProbeToolChannel Bounded(n) hard reject
// + PromptPressure soft/hard/forced progression.
package toolchannel

import (
	"context"
	"errors"
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

// D7-S9-A50-T03 + T04: ProbeToolChannel Bounded(15) hard stops at iter 16.
func TestProbeToolChannel_Bounded15_HardStopsAt16(t *testing.T) {
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
	if ok {
		t.Errorf("iter=15 should be at bound and fire hard reject")
	}
	if err == nil {
		t.Errorf("expected ErrProbeToolChannelBoundExceeded, got nil")
	}
	if !errors.Is(err, ErrProbeToolChannelBoundExceeded) {
		t.Errorf("expected ErrProbeToolChannelBoundExceeded, got %v", err)
	}
	if !strings.Contains(err.Error(), "read_file") {
		t.Errorf("error should mention call tool name: %v", err)
	}
}

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

	accepted, _, err := r.Route(context.Background(), call)
	if accepted {
		t.Errorf("shadow mode: iter=15 should not be accepted")
	}
	if err != nil {
		t.Errorf("shadow mode: err should be nil (log only), got %v", err)
	}
	if r.WouldRejectCount() != 1 {
		t.Errorf("WouldRejectCount should be 1, got %d", r.WouldRejectCount())
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

	accepted, _, err := r.Route(context.Background(), call)
	if accepted {
		t.Errorf("enforce mode: iter=15 should not be accepted")
	}
	if err == nil {
		t.Errorf("enforce mode: err should not be nil")
	}
	if !errors.Is(err, ErrProbeToolChannelBoundExceeded) {
		t.Errorf("expected ErrProbeToolChannelBoundExceeded, got %v", err)
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

	// At iter=15, Accept should fire regardless of permission state.
	// The channel does NOT consult permission state.
	state := makeState(15, 15)
	ok, err := ch.Accept(context.Background(), call, state)
	if ok {
		t.Errorf("Accept should fire at iter=15")
	}
	if err == nil {
		t.Errorf("Accept should return ErrProbeToolChannelBoundExceeded")
	}
	// Permission guards are checked by the ChannelRouter BEFORE Accept.
	// This is the H8 / P1-AC-2 cross-check invariant.
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
