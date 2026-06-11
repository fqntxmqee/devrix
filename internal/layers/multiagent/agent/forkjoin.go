package agent

import (
	"context"

	"github.com/devrix/devrix/internal/layers/multiagent"
	"github.com/devrix/devrix/internal/layers/observability/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/tracer"
	sharederrors "github.com/devrix/devrix/internal/shared/errors"
)

// Fork creates a child agent with an isolated message buffer.
func (a *Impl) Fork(ctx context.Context, cfg multiagent.AgentConfig) (multiagent.Agent, error) {
	if a.State() == multiagent.AgentStateTerminated {
		return nil, sharedTerminated(a.id)
	}
	if a.cfg.ParentID != "" {
		return nil, sharederrors.NewAgentInvalidConfigError("worker agents cannot fork")
	}
	if a.activeChildCount() >= a.cfg.MaxChildren {
		return nil, sharederrors.NewAgentMaxChildrenError(a.activeChildCount(), a.cfg.MaxChildren)
	}

	childCfg := cfg
	childCfg.SessionID = a.cfg.SessionID
	childCfg.ParentID = a.id
	childCfg.MaxChildren = 0
	if childCfg.WorkDir == "" {
		childCfg.WorkDir = a.cfg.WorkDir
	}
	if childCfg.PermissionTimeout <= 0 {
		childCfg.PermissionTimeout = a.cfg.PermissionTimeout
	}

	ctx, forkSpan := a.startSpan(ctx, telemetry.OpAgentFork, tracer.SpanKindInternal,
		tracer.Attribute{Key: "agent.id", Value: a.id},
		tracer.Attribute{Key: "child.mode", Value: string(childCfg.Mode)},
	)

	child, err := a.creator.Create(ctx, childCfg, a.session)
	if err != nil {
		if forkSpan != nil { forkSpan.End() }
		return nil, err
	}
	a.addChild(child)
	a.emit("agent.forked", map[string]any{"child_id": child.ID()})
	if forkSpan != nil {
		forkSpan.SetAttributes(tracer.Attribute{Key: "child.id", Value: child.ID()})
		forkSpan.SetStatus(tracer.StatusCodeOk, "")
		forkSpan.End()
	}
	return child, nil
}

// Join merges a completed child message buffer into the parent.
func (a *Impl) Join(ctx context.Context, child multiagent.Agent) error {
	_, joinSpan := a.startSpan(ctx, telemetry.OpAgentJoin, tracer.SpanKindInternal,
		tracer.Attribute{Key: "agent.id", Value: a.id},
		tracer.Attribute{Key: "child.id", Value: child.ID()},
	)

	if err := ctx.Err(); err != nil {
		if joinSpan != nil { joinSpan.End() }
		return sharederrors.NewAgentContextCancelledError(a.id)
	}
	if child.State() != multiagent.AgentStateTerminated {
		if joinSpan != nil { joinSpan.End() }
		return sharederrors.NewAgentJoinNotCompletedError(child.ID())
	}
	result, err := child.Wait(ctx)
	if err != nil && result == nil {
		if joinSpan != nil { joinSpan.End() }
		return err
	}
	a.appendMessages(child.GetMessages()...)
	if result != nil && len(result.Messages) > 0 {
		a.appendMessages(result.Messages...)
	}
	a.removeChild(child.ID())
	a.emit("agent.joined", map[string]any{"child_id": child.ID()})
	if joinSpan != nil {
		joinSpan.SetStatus(tracer.StatusCodeOk, "")
		joinSpan.End()
	}
	return nil
}
