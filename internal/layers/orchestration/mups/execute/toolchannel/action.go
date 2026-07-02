package toolchannel

import (
	"context"

	"github.com/devrix/devrix/internal/layers/observability/instrument/ltl/invariants/termination"
	"github.com/devrix/devrix/internal/shared/contracts"
)

// ActionToolChannel handles EC_Action tools (state-changing:
// write_file, edit_file, bash, verify_plan_execution, ask_user_question,
// task_*).
//
// Per ConvergenceContract=StateChangeRequired, the PostSnapshot MUST
// differ from PreSnapshot (L7-ACTION-POSTSNAPSHOT). The channel
// captures snapshots in OnResult.
//
// DSAFT: D7-S9-A50-T06.
type ActionToolChannel struct {
	inv *termination.ActionPostSnapshotInvariant
}

func NewActionToolChannel() *ActionToolChannel {
	return &ActionToolChannel{inv: termination.NewActionPostSnapshotInvariant()}
}

func (a *ActionToolChannel) Name() string                          { return "action" }
func (a *ActionToolChannel) EmissionClass() contracts.EmissionClass { return contracts.EC_Action }
func (a *ActionToolChannel) Invariant() termination.TerminationInvariant {
	return a.inv
}
func (a *ActionToolChannel) Snapshot() *termination.State {
	return &termination.State{ChannelName: a.Name()}
}

func (a *ActionToolChannel) Accept(ctx context.Context, call *ToolCall, state *termination.State) (bool, error) {
	// Action channels accept all calls; the snapshot check is in OnResult.
	return true, nil
}

func (a *ActionToolChannel) OnResult(ctx context.Context, call *ToolCall, result *ToolResult, state *termination.State) error {
	// In a full implementation, the channel would compare the tool's
	// effect on disk (or DB) before vs. after. For the skeleton, we
	// invoke the invariant as a no-op when snapshots aren't populated.
	_, reason := a.inv.Check(state)
	if reason != "" {
		state.ToolName = "STATE_CHANGE_MISSING: " + reason
	}
	return nil
}

func (a *ActionToolChannel) InjectPromptPressure(ctx context.Context, state *termination.State, call *ToolCall) error {
	// Action channels do not inject pressure (state-changing is bounded by
	// user intent, not iteration count).
	return nil
}

func (a *ActionToolChannel) Finalize(ctx context.Context, state *termination.State) (*ChannelOutcome, error) {
	return &ChannelOutcome{
		ChannelName:    a.Name(),
		PrimaryClass:   a.EmissionClass(),
		IterationsUsed: state.IterationsUsed,
		BoundMax:       callMaxIfSet(state),
		Forced:         false,
	}, nil
}

func callMaxIfSet(state *termination.State) int {
	if state.BoundMax > 0 {
		return state.BoundMax
	}
	return 0
}
