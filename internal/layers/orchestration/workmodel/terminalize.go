package workmodel

import (
	"github.com/devrix/devrix/internal/shared/types"
)

// SpawnNoneTerminalOpts controls ResolveSpawnNoneTaskStatus for rollup/verify paths.
type SpawnNoneTerminalOpts struct {
	IsRollup       bool
	StripDeliverableForStatus bool
}

// ResolveSpawnNoneTaskStatus maps verify outcome to TaskStatus when spawn is none.
func ResolveSpawnNoneTaskStatus(
	verdict types.VerdictKind,
	schema DeliverableSchema,
	deliverable DeliverableStatus,
	opts SpawnNoneTerminalOpts,
) TaskStatus {
	schemaForStatus := schema
	deliverableStatus := deliverable
	if opts.StripDeliverableForStatus {
		schemaForStatus = DeliverableSchemaNotApplicable
	}
	if opts.IsRollup {
		schemaForStatus = DeliverableSchemaNotApplicable
		deliverableStatus = DeliverableStatusNotApplicable
	}
	status := StatusAfterSpawnNone(verdict, schemaForStatus, deliverableStatus)
	if opts.IsRollup && verdict != types.VerdictPass {
		return TaskStatusInProgress
	}
	return status
}

// ApplyRoundTerminalization updates WorkItem status after SpawnNone when the
// resolved status is terminal (or leaves InProgress for incomplete continuation).
func ApplyRoundTerminalization(
	t *WorkTree,
	sessionID, itemID string,
	verdict types.VerdictKind,
	schema DeliverableSchema,
	deliverable DeliverableStatus,
	opts SpawnNoneTerminalOpts,
) error {
	if t == nil {
		return nil
	}
	status := ResolveSpawnNoneTaskStatus(verdict, schema, deliverable, opts)
	got, ok := t.Get(sessionID, itemID)
	if !ok || got == nil {
		return nil
	}
	if got.Status == TaskStatusPending {
		_ = t.UpdateStatus(sessionID, itemID, TaskStatusInProgress)
	}
	if status != TaskStatusInProgress {
		if err := t.UpdateStatus(sessionID, itemID, status); err != nil {
			return err
		}
		if IsTerminalStatus(status) {
			_ = t.ResetInlineRetriesAtMaxDepth(sessionID, itemID)
		}
	}
	return nil
}

// TouchInlineRetryAtMaxDepth increments the deliverable inline counter when
// policy chose SpawnInline with deliverable continuation.
func TouchInlineRetryAtMaxDepth(t *WorkTree, sessionID string, item *WorkItem, round *WorkItemPipelineRound, _ TreeEvalContext) {
	if t == nil || item == nil || round == nil {
		return
	}
	if round.SpawnPolicy != SpawnInline || !deliverableContinuationRequired(round) {
		return
	}
	if round.RollupSynthRequested {
		return
	}
	_ = t.IncrementInlineRetriesAtMaxDepth(sessionID, item.ID)
}

// IsDeliverableInlineBudgetExhausted reports whether inline retries for an
// owed deliverable hit the configured budget (any depth).
func IsDeliverableInlineBudgetExhausted(item *WorkItem) bool {
	if item == nil {
		return false
	}
	return item.InlineRetriesAtMaxDepth >= DefaultMaxInlineRetriesAtMaxDepth
}
