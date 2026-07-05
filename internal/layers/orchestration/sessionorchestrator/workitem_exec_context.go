package sessionorchestrator

import (
	"context"
	"fmt"
	"strings"

	"github.com/devrix/devrix/internal/layers/contextengine/materialize"
	"github.com/devrix/devrix/internal/layers/orchestration/plan"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/contracts"
)

type workItemExecCtxKey struct{}
type suppressExecuteTextEmitKey struct{}

// WithSuppressExecuteTextEmit hides LLM text chunks from IM streaming (e.g.
// findings_json synthesis turn) while still accumulating artifact content.
func WithSuppressExecuteTextEmit(ctx context.Context) context.Context {
	return context.WithValue(ctx, suppressExecuteTextEmitKey{}, true)
}

// SuppressExecuteTextEmit reports whether Execute should omit text EngineEvents.
func SuppressExecuteTextEmit(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	v, _ := ctx.Value(suppressExecuteTextEmitKey{}).(bool)
	return v
}

// WorkItemExecContext carries the active WorkItem for Materialize during Execute.
type WorkItemExecContext struct {
	Item  *workmodel.WorkItem
	Tasks *workmodel.TaskManager
	// MaxItersOverride when >0 caps the ReAct loop for this execute (DM-20260630-012).
	MaxItersOverride int
	// DeliverableContract selects Verify deliverable gate for this round.
	DeliverableContract workmodel.DeliverableContract
	// DeliverableSchema is deprecated; kept for legacy round serialization.
	DeliverableSchema workmodel.DeliverableSchema
	// PriorVerifyReason carries the previous round's verify verdict reason
	// (RH-MUPS-10 / D7-S9-A91, DM-20260701-001). When the executor is being
	// invoked for a retry (SpawnInline after a non-Pass verdict), the
	// LLM producer must see WHY the prior round failed so it can self-
	// correct instead of re-producing the same artifact. Empty when this
	// is the first round or the prior round passed.
	PriorVerifyReason string
	// Emit is scoped to this ExecuteWorkItem invocation. It must never live on
	// the shared DefaultWorkItemExecutor because the executor is singleton-ish
	// in production wiring and can serve multiple sessions concurrently.
	Emit func(*contracts.EngineEvent)
	// Strategy is the per-PlanKind behavior abstraction (M3, DM-20260705-008).
	// WorkItemExecContext consumers call ec.Strategy.SpawnOverride to gate
	// per-PlanKind verdict→spawn overrides (commitment terminal, scenario
	// read-only, exploration parallel-pipeline continuation). nil-guard is
	// enforced by WithWorkItemExecContext; consumers can rely on a non-nil
	// Strategy (default = protocolStrategy for unknown PlanKinds).
	Strategy workmodel.Strategy
}

// WithWorkItemExecContext attaches WorkItem exec metadata to ctx.
// M3 (DM-20260705-008): nil-guards ec.Strategy by binding protocolStrategy
// (the safe default for unknown PlanKinds) so downstream consumers can rely
// on a non-nil Strategy without a per-call guard.
func WithWorkItemExecContext(ctx context.Context, ec WorkItemExecContext) context.Context {
	if ec.Strategy == nil {
		ec.Strategy = workmodel.LookupStrategy(plan.KindUnset)
	}
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
		SessionID:  sessionID,
		Kind:       materialize.PartitionWorkItem,
		WorkItemID: item.ID,
	}
	if item != nil && item.ParentID != "" {
		p.ParentWorkItemID = item.ParentID
	}
	return p
}

// ShouldMaterializeWorkItem reports whether Execute should use D2 Materialize instead of legacy Prepare.
// L0 Goal uses legacy Prepare; depth≥1 and rollup WorkItems always Materialize (DM-20260627-003).
func ShouldMaterializeWorkItem(ctx context.Context, sessionID, itemID string) bool {
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
		ToolProfile: toolProfileForItemWithTasks(sessionID, item, tm),
		Depth:       depth,
	}
	signals := materialize.InboundSignals{Directive: directive}
	if item != nil && tm != nil && workmodel.IsParentRollupSynth(tm, sessionID, item) {
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
		if len(item.BlockedBy) > 0 {
			policy.Mode = materialize.ModeUpstream
			signals.SignalLines = upstreamSignalLines(sessionID, item, tm)
		}
		if item.ParentID != "" {
			if peers := tm.PeerStatusSignalsForCohort(sessionID, item.ParentID); len(peers) > 0 {
				signals.SignalLines = append(signals.SignalLines, workmodel.PeerStatusLines(peers)...)
			}
		}
	}
	if item != nil && item.NeedsRollup && tm != nil {
		for _, c := range tm.Tree().ListChildren(sessionID, item.ID) {
			if c == nil || c.LastRound == nil || c.LastRound.StructuredDeliverable == nil {
				continue
			}
			for _, f := range c.LastRound.StructuredDeliverable.Findings {
				line := fmt.Sprintf("child_finding: %s %s:%d", f.Severity, f.File, f.Line)
				if msg := strings.TrimSpace(f.Message); msg != "" {
					line += " " + workmodel.TruncateArtifactSummary(msg, 120)
				}
				signals.SignalLines = append(signals.SignalLines, line)
			}
		}
	}
	return materialize.Request{
		Partition: partition,
		Policy:    policy,
		Signals:   signals,
	}
}

func upstreamSignalLines(sessionID string, item *workmodel.WorkItem, tm *workmodel.TaskManager) []string {
	if item == nil || tm == nil {
		return nil
	}
	var lines []string
	for _, blockerID := range item.BlockedBy {
		upstream, ok := tm.GetWorkItem(sessionID, blockerID)
		if !ok || upstream == nil {
			continue
		}
		if upstream.LastRound != nil {
			if stmt := workmodel.StructuredBubbleStatement(blockerID, upstream.LastRound); stmt != "" {
				lines = append(lines, stmt)
			}
			if s := strings.TrimSpace(upstream.LastRound.ArtifactSummary); s != "" {
				lines = append(lines, "upstream_artifact_summary: "+workmodel.TruncateArtifactSummary(s, 240))
			}
		}
	}
	return lines
}

func toolProfileForItem(item *workmodel.WorkItem) string {
	return toolProfileForItemWithTasks("", item, nil)
}

func toolProfileForItemWithTasks(sessionID string, item *workmodel.WorkItem, tm *workmodel.TaskManager) string {
	if item == nil {
		return "implement"
	}
	if item.NeedsRollup && (tm == nil || workmodel.IsParentRollupSynth(tm, sessionID, item)) {
		return "rollup_synth"
	}
	switch item.Policy {
	case workmodel.ExecPolicyReadonly:
		return "readonly"
	default:
		return "implement"
	}
}
