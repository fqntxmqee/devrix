package sessionorchestrator

import (
	"context"

	"github.com/devrix/devrix/internal/layers/contextengine/materialize"
	"github.com/devrix/devrix/internal/layers/orchestration/configure"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
)

type workItemExecCtxKey struct{}

// WorkItemExecContext carries the active WorkItem for Materialize during Execute.
type WorkItemExecContext struct {
	Item  *workmodel.WorkItem
	Tasks *workmodel.TaskManager
}

// WithWorkItemExecContext attaches WorkItem exec metadata to ctx.
func WithWorkItemExecContext(ctx context.Context, ec WorkItemExecContext) context.Context {
	return context.WithValue(ctx, workItemExecCtxKey{}, ec)
}

// WorkItemExecContextFrom reads WorkItem exec metadata from ctx.
func WorkItemExecContextFrom(ctx context.Context) (WorkItemExecContext, bool) {
	if ctx == nil {
		return WorkItemExecContext{}, false
	}
	v, ok := ctx.Value(workItemExecCtxKey{}).(WorkItemExecContext)
	return v, ok
}

// ResolvePartitionForWorkItem maps a WorkItem to its primary private partition (D7-S16-A71).
func ResolvePartitionForWorkItem(sessionID string, item *workmodel.WorkItem) materialize.Partition {
	p := materialize.Partition{
		SessionID: sessionID,
		Kind:      materialize.PartitionWorkItem,
		WorkItemID: item.ID,
	}
	if item != nil && item.ParentID != "" {
		p.ParentWorkItemID = item.ParentID
	}
	return p
}

// ShouldMaterializeWorkItem reports whether Execute should use D2 Materialize instead of legacy Prepare.
func ShouldMaterializeWorkItem(ctx context.Context, sessionID, itemID string) bool {
	if !configure.FeatureLayerSubContextEnabled() {
		return false
	}
	ec, ok := WorkItemExecContextFrom(ctx)
	if !ok || ec.Item == nil || ec.Tasks == nil {
		return false
	}
	if ec.Item.NeedsRollup {
		return true
	}
	return ec.Tasks.Tree().Depth(sessionID, itemID) >= 1
}

// BuildMaterializeRequest assembles the D2 Materialize request for one WorkItem execute.
func BuildMaterializeRequest(sessionID string, item *workmodel.WorkItem, tm *workmodel.TaskManager, directive string, tokenBudget int) materialize.Request {
	partition := ResolvePartitionForWorkItem(sessionID, item)
	depth := 0
	if tm != nil {
		depth = tm.Tree().Depth(sessionID, item.ID)
	}
	policy := materialize.Policy{
		Mode:        materialize.ModeFresh,
		TokenBudget: tokenBudget,
		ToolProfile: toolProfileForItem(item),
		Depth:       depth,
	}
	signals := materialize.InboundSignals{Directive: directive}
	if item != nil && item.NeedsRollup {
		policy.Mode = materialize.ModeRollupSynth
	}
	if tm != nil {
		if dl, ok := tm.ChildDownlinkFor(sessionID, item.ID); ok {
			policy.Mode = materialize.ModeInheritCohort
			if dl.Directive != "" {
				signals.Directive = dl.Directive
			}
			signals.ScopeIn = append([]string(nil), dl.ScopeIn...)
			signals.ScopeOut = append([]string(nil), dl.ScopeOut...)
			signals.ExpectedReturn = dl.ExpectedReturn
		}
	}
	return materialize.Request{
		Partition: partition,
		Policy:    policy,
		Signals:   signals,
	}
}

func toolProfileForItem(item *workmodel.WorkItem) string {
	if item == nil {
		return "implement"
	}
	switch item.Policy {
	case workmodel.ExecPolicyReadonly:
		return "readonly"
	default:
		return "implement"
	}
}
