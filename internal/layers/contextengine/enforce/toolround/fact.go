package toolround

import (
	"context"

	"github.com/devrix/devrix/internal/layers/observability/instrument/ltl/invariants/termination"
	"github.com/devrix/devrix/internal/shared/contracts"
)

// FactToolChannel handles EC_Fact tools (deterministic read-only:
// read_file, grep, glob, query_diagnostics, lsp_goto_definition, etc.).
//
// Per H9: when the same tool+query is called 5+ times consecutively, the
// channel reclassifies the call as Probe (Bounded(n) apply). This is
// the H9 OnResult behavior reclassification (specs/execute-channels.md
// §D7-EXEC-CH-4 + §D7-EXEC-CH-1 L7-FACT-SAME-Q-5x).
//
// DSAFT: D7-S9-A50-T06.
type FactToolChannel struct {
	inv *termination.FactSameQueryInvariant
}

func NewFactToolChannel() *FactToolChannel {
	return &FactToolChannel{inv: termination.NewFactSameQueryInvariant()}
}

func (f *FactToolChannel) Name() string                          { return "fact" }
func (f *FactToolChannel) EmissionClass() contracts.EmissionClass { return contracts.EC_Fact }
func (f *FactToolChannel) Invariant() termination.TerminationInvariant {
	return f.inv
}
func (f *FactToolChannel) Snapshot() *termination.State {
	return &termination.State{ChannelName: f.Name()}
}

// Accept: Fact channels accept all calls; the H9 reclassification is
// applied at OnResult (the channel does not reject — it just re-prices
// future calls as Probe via the Router's escalation rule).
func (f *FactToolChannel) Accept(ctx context.Context, call *ToolCall, state *termination.State) (bool, error) {
	return true, nil
}

// OnResult: update same-query counter; if SameQueryCount >= 5, the
// Router should escalate the next call to ProbeToolChannel (H9).
// The router reads state.SameQueryCount and re-dispatches accordingly.
func (f *FactToolChannel) OnResult(ctx context.Context, call *ToolCall, result *ToolResult, state *termination.State) error {
	// The Router updates SameQueryCount before calling OnResult.
	// Here we just check the invariant for observability.
	ok, reason := f.inv.Check(state)
	if !ok {
		// Escalation signal — log via the call's ToolName field as a
		// side channel (a real implementation would emit a span).
		state.ToolName = "ESCALATE_TO_PROBE: " + reason
	}
	return nil
}

func (f *FactToolChannel) InjectPromptPressure(ctx context.Context, state *termination.State, call *ToolCall) error {
	// Fact channels do not inject pressure (OpenEnded by default).
	return nil
}

func (f *FactToolChannel) Finalize(ctx context.Context, state *termination.State) (*ChannelOutcome, error) {
	return &ChannelOutcome{
		ChannelName:    f.Name(),
		PrimaryClass:   f.EmissionClass(),
		IterationsUsed: state.IterationsUsed,
		BoundMax:       0, // OpenEnded for Fact
		Forced:         false,
	}, nil
}
