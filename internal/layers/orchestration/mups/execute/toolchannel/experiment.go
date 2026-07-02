package toolchannel

import (
	"context"

	"github.com/devrix/devrix/internal/layers/observability/instrument/ltl/invariants/termination"
	"github.com/devrix/devrix/internal/shared/contracts"
)

// ExperimentToolChannel handles EC_Experiment tools (free_fork, sub-process,
// worktree). The channel enforces L7-EXPERIMENT-CONCLUDED-BEFORE-DEADLINE.
//
// DSAFT: D7-S9-A50-T06.
type ExperimentToolChannel struct {
	inv *termination.ExperimentDeadlineInvariant
}

func NewExperimentToolChannel() *ExperimentToolChannel {
	return &ExperimentToolChannel{inv: termination.NewExperimentDeadlineInvariant()}
}

func (e *ExperimentToolChannel) Name() string                          { return "experiment" }
func (e *ExperimentToolChannel) EmissionClass() contracts.EmissionClass { return contracts.EC_Experiment }
func (e *ExperimentToolChannel) Invariant() termination.TerminationInvariant {
	return e.inv
}
func (e *ExperimentToolChannel) Snapshot() *termination.State {
	return &termination.State{ChannelName: e.Name()}
}

func (e *ExperimentToolChannel) Accept(ctx context.Context, call *ToolCall, state *termination.State) (bool, error) {
	// Experiment channels accept all calls; the deadline check is in Finalize.
	return true, nil
}

func (e *ExperimentToolChannel) OnResult(ctx context.Context, call *ToolCall, result *ToolResult, state *termination.State) error {
	return nil
}

func (e *ExperimentToolChannel) InjectPromptPressure(ctx context.Context, state *termination.State, call *ToolCall) error {
	return nil
}

func (e *ExperimentToolChannel) Finalize(ctx context.Context, state *termination.State) (*ChannelOutcome, error) {
	_, reason := e.inv.Check(state)
	return &ChannelOutcome{
		ChannelName:       e.Name(),
		PrimaryClass:      e.EmissionClass(),
		IterationsUsed:    state.IterationsUsed,
		BoundMax:          state.BoundMax,
		Forced:            reason != "",
		ForcedReason:      reason,
		InvariantViolated: e.inv.Name(),
	}, nil
}
