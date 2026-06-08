package agent

import (
	"context"

	"github.com/devrix/devrix/internal/layers/multiagent"
	sharederrors "github.com/devrix/devrix/internal/shared/errors"
)

// Fork creates a child agent with an isolated message buffer.
func (a *Impl) Fork(ctx context.Context, cfg multiagent.AgentConfig) (multiagent.Agent, error) {
	if a.State() == multiagent.AgentStateTerminated {
		return nil, sharedTerminated(a.id)
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

	child, err := a.creator.Create(ctx, childCfg, a.session)
	if err != nil {
		return nil, err
	}
	a.addChild(child)
	a.emit("agent.forked", map[string]any{"child_id": child.ID()})
	return child, nil
}

// Join merges a completed child message buffer into the parent.
func (a *Impl) Join(ctx context.Context, child multiagent.Agent) error {
	if err := ctx.Err(); err != nil {
		return sharederrors.NewAgentContextCancelledError(a.id)
	}
	if child.State() != multiagent.AgentStateTerminated {
		return sharederrors.NewAgentJoinNotCompletedError(child.ID())
	}
	result, err := child.Wait(ctx)
	if err != nil && result == nil {
		return err
	}
	a.appendMessages(child.GetMessages()...)
	if result != nil && len(result.Messages) > 0 {
		a.appendMessages(result.Messages...)
	}
	a.removeChild(child.ID())
	a.emit("agent.joined", map[string]any{"child_id": child.ID()})
	return nil
}
