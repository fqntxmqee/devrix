package run

import (
	"context"

	"github.com/devrix/devrix/internal/layers/multiagent"
	"github.com/devrix/devrix/internal/layers/multiagent/observability"
	"github.com/devrix/devrix/internal/layers/multiagent/isolate"
	"github.com/devrix/devrix/internal/layers/observability/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/tracer"
	sharederrors "github.com/devrix/devrix/internal/shared/errors"
	"github.com/devrix/devrix/internal/shared/types"
)

// Fork creates a child agent with an isolated message buffer and a
// dedicated SessionView (DM-20260611-005). The child inherits the
// parent's session id but writes metadata only to its own view.
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

	ctx, forkSpan := a.startSpan(ctx, telemetry.OpD4_S4_Agent_Fork, tracer.SpanKindInternal,
		tracer.Attribute{Key: "agent.id", Value: a.id},
		tracer.Attribute{Key: "child.mode", Value: string(childCfg.Mode)},
	)

	// The child's view is a fresh COW fork of the parent session.
	// We count this as the "cow" policy for D5 metrics.
	childView := isolate.Fork(a.session)
	observability.IncForkSessionView("cow")

	child, err := a.creator.Create(ctx, childCfg, a.session)
	if err != nil {
		// Failed fork; roll back the policy tag.
		observability.IncForkSessionView("snapshot")
		if forkSpan != nil {
			forkSpan.End()
		}
		return nil, err
	}
	if impl, ok := child.(*Impl); ok {
		impl.AttachSessionView(childView)
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

// Join merges a completed child agent's messages into the parent.
//
// Sorting & dedup contract (DM-20260611-005):
//   - tool_call messages that share a non-empty call_id are collapsed
//     to a single entry (the first occurrence wins; order is the order
//     in which the child appended them).
//   - non-tool messages are kept verbatim.
//
// The dedup is intentionally per-call; in v1.1 a v2.0 policy hook can
// override (e.g. shared→last-write-wins, snapshot→preserve-all).
func (a *Impl) Join(ctx context.Context, child multiagent.Agent) error {
	_, joinSpan := a.startSpan(ctx, telemetry.OpD4_S4_Agent_Join, tracer.SpanKindInternal,
		tracer.Attribute{Key: "agent.id", Value: a.id},
		tracer.Attribute{Key: "child.id", Value: child.ID()},
	)

	if err := ctx.Err(); err != nil {
		if joinSpan != nil {
			joinSpan.End()
		}
		return sharederrors.NewAgentContextCancelledError(a.id)
	}
	if child.State() != multiagent.AgentStateTerminated {
		if joinSpan != nil {
			joinSpan.End()
		}
		return sharederrors.NewAgentJoinNotCompletedError(child.ID())
	}
	result, err := child.Wait(ctx)
	if err != nil && result == nil {
		if joinSpan != nil {
			joinSpan.End()
		}
		return err
	}
	msgs := child.GetMessages()
	if result != nil && len(result.Messages) > 0 {
		msgs = append(msgs, result.Messages...)
	}
	a.appendMessages(a.dedupToolCallMessages(msgs)...)
	a.removeChild(child.ID())
	a.emit("agent.joined", map[string]any{"child_id": child.ID()})
	if joinSpan != nil {
		joinSpan.SetStatus(tracer.StatusCodeOk, "")
		joinSpan.End()
	}
	return nil
}

// dedupToolCallMessages collapses tool_call messages that share a
// non-empty tool_call_id, keeping the first occurrence globally across
// Join calls on this parent. Non-tool messages pass through unchanged.
func (a *Impl) dedupToolCallMessages(msgs []types.Message) []types.Message {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.joinedToolIDs == nil {
		a.joinedToolIDs = make(map[string]struct{}, len(msgs))
	}
	out := make([]types.Message, 0, len(msgs))
	for _, m := range msgs {
		if callID, ok := m.Metadata[multiagent.MetaToolCallID]; ok && callID != "" {
			if _, dup := a.joinedToolIDs[callID]; dup {
				continue
			}
			a.joinedToolIDs[callID] = struct{}{}
		}
		out = append(out, m)
	}
	return out
}
