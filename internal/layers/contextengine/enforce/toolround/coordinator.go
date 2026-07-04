package toolround

import (
	"context"

	"github.com/devrix/devrix/internal/layers/observability/instrument/ltl/invariants/termination"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// RoundInput carries D7 ExecuteToolRound extensions (DM-20260704-001).
type RoundInput struct {
	SessionID       string
	TaskKind        string
	RemainingBudget int
}

// RoundOutput carries channel pressure messages for the ReAct loop.
type RoundOutput struct {
	PressureMessages []types.Message
}

// Coordinator routes probe pressure through the 4-channel Router.
type Coordinator struct {
	Router *Router
}

// NewCoordinator constructs a shadow-mode coordinator.
func NewCoordinator() *Coordinator {
	return &Coordinator{Router: NewRouter(ModeShadow)}
}

// AfterToolCall updates channel state and returns advisory pressure messages.
func (c *Coordinator) AfterToolCall(ctx context.Context, in RoundInput, spec contracts.ToolSpec, toolName string) RoundOutput {
	if c == nil || c.Router == nil {
		return RoundOutput{}
	}
	call := &ToolCall{
		SessionID: in.SessionID,
		ToolName:  toolName,
		Spec:      spec,
		TaskKind:  in.TaskKind,
	}
	state := c.Router.getOrCreateState(in.SessionID, channelNameForClass(spec.EmissionClass))
	ctx = WithPromptState(ctx, state)
	accepted, _, _ := c.Router.Route(ctx, call)
	_ = accepted
	_ = c.Router.OnResult(ctx, call, &ToolResult{ToolName: toolName})
	if spec.EmissionClass == contracts.EC_Probe {
		_ = NewProbeToolChannel().InjectPromptPressure(ctx, state, call)
	}
	if msg := pressureText(state, in.RemainingBudget); msg != "" {
		return RoundOutput{PressureMessages: []types.Message{{
			SessionID: in.SessionID,
			Role:      types.MessageRoleUser,
			Content:   msg,
		}}}
	}
	return RoundOutput{}
}

func channelNameForClass(ec contracts.EmissionClass) string {
	switch ec {
	case contracts.EC_Fact:
		return "fact"
	case contracts.EC_Action:
		return "action"
	case contracts.EC_Probe:
		return "probe"
	case contracts.EC_Experiment:
		return "experiment"
	default:
		return "probe"
	}
}

func pressureText(state *termination.State, remaining int) string {
	if state == nil {
		return ""
	}
	if remaining <= 2 && remaining >= 0 {
		return "⚠️ FINAL: tool budget nearly exhausted. Synthesize NOW."
	}
	return ""
}
