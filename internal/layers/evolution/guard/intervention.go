package guard

import (
	"context"
	"errors"
	"fmt"
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

	// PR-A: H-3 silent swallow 修复（DM-20260621-011）
	// 通过 WithMetrics 注入, nil-safe（默认 nil 不计数, 不影响现有调用方）
	// PR-B: 类型从 *orchMetrics 重命名为 *guardMetrics (alias 保留兼容).
	metrics *guardMetrics
}

// NewInterventionExecutor creates an executor with the required controllers.
func NewInterventionExecutor(agents AgentController, tasks TaskController, factory AgentFactory) *InterventionExecutor {
	return &InterventionExecutor{agents: agents, tasks: tasks, factory: factory}
}

// WithMetrics injects the guardMetrics for error aggregation observability.
// Returns the executor for chaining. Nil-safe: nil disables recording.
func (ie *InterventionExecutor) WithMetrics(m *guardMetrics) *InterventionExecutor {
	ie.metrics = m
	return ie
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

// terminateAndReroute stops the current agent, optionally fails the task,
// then creates + starts a new agent. Errors from Terminate/Wait/Tasks.Fail
// are aggregated via errors.Join (DM-20260621-011 H-3, matches D7 error
// aggregation "atomic counter + slog + errors.Join" pattern).
func (ie *InterventionExecutor) terminateAndReroute(ctx context.Context, session *types.Session, iv Intervention) error {
	var errs []error

	current := ie.agents.SessionAgent(session.SessionID)
	if current != nil {
		if termErr := current.Terminate(ctx); termErr != nil {
			slog.Warn("terminate current agent on reroute",
				"session_id", session.SessionID, "error", termErr)
			errs = append(errs, fmt.Errorf("terminate: %w", termErr))
		}
		if _, waitErr := current.Wait(ctx); waitErr != nil {
			ie.metrics.recordWaitFailed()
			slog.Warn("wait current agent failed",
				"session_id", session.SessionID, "error", waitErr)
			errs = append(errs, fmt.Errorf("wait: %w", waitErr))
		}
	}

	if iv.MilestoneFail || iv.TaskFail {
		taskID := iv.FailReason
		if failErr := ie.tasks.Fail(taskID, iv.Reason); failErr != nil {
			ie.metrics.recordTaskFailFailed()
			slog.Warn("task fail failed",
				"task_id", taskID, "reason", iv.Reason, "error", failErr)
			errs = append(errs, fmt.Errorf("task_fail: %w", failErr))
		}
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
		errs = append(errs, fmt.Errorf("create: %w", err))
		return errors.Join(errs...)
	}

	ie.agents.RegisterSessionAgent(session.SessionID, newAgent)
	go func() {
		if _, runErr := newAgent.Run(ctx); runErr != nil {
			slog.Error("reroute agent run", "error", runErr)
		}
	}()
	return errors.Join(errs...)
}

func (ie *InterventionExecutor) updateState(ctx context.Context, session *types.Session, iv Intervention) error {
	if iv.MilestoneFail {
		return ie.tasks.Fail(session.SessionID, iv.Reason)
	}
	return nil
}
