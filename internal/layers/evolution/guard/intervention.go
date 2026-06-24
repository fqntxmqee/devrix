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

// AgentFactory creates new agent instances during reroute.
type AgentFactory interface {
	Create(ctx context.Context, cfg multiagent.AgentConfig, session *types.Session) (multiagent.Agent, error)
}

// InterventionExecutor applies corrective actions when validation fails.
//
// DM-20260625-008: TaskController 移除。D7 5 节点管道下, 任务失败由
// Execute 节点通过 4 Channel (synchronous/async/probe/exploration) 上报,
// 不再依赖 milestone-based TaskController. Intervention.TaskFail/MilestoneFail
// 字段保留用于 D6 演化层传递意图, 但 executor 内部仅 slog.Warn 记录
// 不再触发任何 task fail 动作.
type InterventionExecutor struct {
	agents  AgentController
	factory AgentFactory

	// PR-A: H-3 silent swallow 修复（DM-20260621-011）
	// 通过 WithMetrics 注入, nil-safe（默认 nil 不计数, 不影响现有调用方）
	// PR-B: 类型从 *orchMetrics 重命名为 *guardMetrics (alias 保留兼容).
	metrics *guardMetrics
}

// NewInterventionExecutor creates an executor with the required controllers.
func NewInterventionExecutor(agents AgentController, factory AgentFactory) *InterventionExecutor {
	return &InterventionExecutor{agents: agents, factory: factory}
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

// terminateAndReroute stops the current agent, optionally logs task-fail
// intent, then creates + starts a new agent. Errors from Terminate/Wait
// are aggregated via errors.Join (DM-20260621-011 H-3, matches D7 error
// aggregation "atomic counter + slog + errors.Join" pattern).
//
// DM-20260625-008: Tasks.Fail 移除, 改为 slog.Warn 记录 D6 演化的
// task-fail 意图, 不再触发任何 milestone-based fail 动作.
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
		slog.Warn("task fail requested but no TaskController wired (D7 5-node pipeline replaces milestone-based fail)",
			"session_id", session.SessionID,
			"task_id", iv.FailReason,
			"reason", iv.Reason,
			"milestone_fail", iv.MilestoneFail,
			"task_fail", iv.TaskFail,
		)
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
		// DM-20260625-008: TaskController 移除, 不再触发 fail.
		// 保留字段用于 D6 演化层意图传递, executor 内部仅 warn.
		slog.Warn("milestone fail requested but no TaskController wired (D7 5-node pipeline replaces milestone-based fail)",
			"session_id", session.SessionID,
			"reason", iv.Reason,
		)
	}
	return nil
}
