package orchestration

import (
	"context"
	"log/slog"

	"github.com/devrix/devrix/internal/layers/multiagent"
	"github.com/devrix/devrix/internal/shared/types"
)

// AgentController provides agent lifecycle operations for intervention.
type AgentController interface {
	SessionAgent(sessionID string) multiagent.Agent
	RegisterSessionAgent(sessionID string, ag multiagent.Agent)
}

// TaskController provides milestone state management.
type TaskController interface {
	Fail(id string, reason string) error
	Complete(id string) error
}

// AgentFactory creates new agent instances during reroute.
type AgentFactory interface {
	Create(ctx context.Context, cfg multiagent.AgentConfig, session *types.Session) (multiagent.Agent, error)
}

// InterventionExecutor applies corrective actions when validation fails.
type InterventionExecutor struct {
	agents  AgentController
	tasks   TaskController
	factory AgentFactory
}

// NewInterventionExecutor creates an executor with the required controllers.
func NewInterventionExecutor(agents AgentController, tasks TaskController, factory AgentFactory) *InterventionExecutor {
	return &InterventionExecutor{agents: agents, tasks: tasks, factory: factory}
}

// Execute applies the given intervention.
func (ie *InterventionExecutor) Execute(ctx context.Context, iv Intervention, session *types.Session) error {
	slog.Info("executing intervention",
		"action", iv.Action,
		"decision_id", iv.DecisionID,
		"reason", iv.Reason,
	)
	switch iv.Action {
	case "terminate":
		return ie.terminate(ctx, session)
	case "reroute":
		return ie.terminateAndReroute(ctx, session, iv)
	case "update_state":
		return ie.updateState(ctx, session, iv)
	default:
		slog.Warn("unknown intervention action", "action", iv.Action)
		return nil
	}
}

func (ie *InterventionExecutor) terminate(ctx context.Context, session *types.Session) error {
	ag := ie.agents.SessionAgent(session.SessionID)
	if ag != nil {
		return ag.Terminate(ctx)
	}
	return nil
}

func (ie *InterventionExecutor) terminateAndReroute(ctx context.Context, session *types.Session, iv Intervention) error {
	current := ie.agents.SessionAgent(session.SessionID)
	if current != nil {
		if err := current.Terminate(ctx); err != nil {
			slog.Warn("terminate current agent on reroute", "error", err)
		}
		_, _ = current.Wait(ctx)
	}

	if iv.MilestoneFail || iv.TaskFail {
		taskID := iv.FailReason
		_ = ie.tasks.Fail(taskID, iv.Reason)
	}

	cfg := iv.AgentConfig
	if cfg == nil {
		cfg = &multiagent.AgentConfig{
			SessionID: session.SessionID,
			WorkDir:   session.WorkDir,
		}
	}
	newAgent, err := ie.factory.Create(ctx, *cfg, session)
	if err != nil {
		return err
	}

	ie.agents.RegisterSessionAgent(session.SessionID, newAgent)
	go func() {
		if _, runErr := newAgent.Run(ctx); runErr != nil {
			slog.Error("reroute agent run", "error", runErr)
		}
	}()
	return nil
}

func (ie *InterventionExecutor) updateState(ctx context.Context, session *types.Session, iv Intervention) error {
	if iv.MilestoneFail {
		return ie.tasks.Fail(session.SessionID, iv.Reason)
	}
	return nil
}
