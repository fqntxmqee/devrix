package workmodel

import (
	"time"

	"github.com/google/uuid"
)

// ContextLinkEvalContext supplies deterministic constraints for ContextLinkEvaluator.
type ContextLinkEvalContext struct {
	SessionID      string
	Parent         *WorkItem
	ParentRound    *WorkItemPipelineRound
	FromItem       *WorkItem
	ToItem         *WorkItem
	FromScope      *ContextScope
	ToScope        *ContextScope
	ExistingLinks  []ContextLinkRecord
	ShareTokenCap  int // CL7; 0 → DefaultShareSummaryMaxTokens
}

// ContextLinkDecision is the outcome of evaluating one link spec or dependency edge.
type ContextLinkDecision struct {
	Accepted   bool
	Record     *ContextLinkRecord
	RejectRule string
	Downgraded bool
}

// EvaluateContextLinkSpec applies CL0–CL8 to an LLM or caller proposal.
func EvaluateContextLinkSpec(spec ContextLinkSpec, ctx ContextLinkEvalContext) ContextLinkDecision {
	if ctx.ShareTokenCap <= 0 {
		ctx.ShareTokenCap = DefaultShareSummaryMaxTokens
	}
	if spec.FromWorkItemID == "" || spec.ToWorkItemID == "" {
		return ContextLinkDecision{RejectRule: "CL_invalid_spec"}
	}
	if ctx.FromItem == nil || ctx.ToItem == nil {
		return ContextLinkDecision{RejectRule: "CL_missing_items"}
	}
	rel := ClassifySiblingRelation(ctx.FromItem, ctx.ToItem)
	if rel.Relation == SiblingNotSibling {
		return ContextLinkDecision{RejectRule: "CL3_cross_layer"}
	}
	if rel.Relation == SiblingProjection {
		return ContextLinkDecision{RejectRule: "CL6_ephemeral"}
	}
	if rel.Relation == SiblingHumanReview {
		return ContextLinkDecision{RejectRule: "CL5_human_review"}
	}
	if rel.Relation == SiblingParallelBatch {
		return ContextLinkDecision{RejectRule: "CL4_parallel_batch"}
	}
	if rel.Relation == SiblingDependent {
		// CL0: dependency links are rule-only; LLM cannot override or close.
		if spec.Kind != LinkUpstream {
			return ContextLinkDecision{RejectRule: "CL0_dependency_forced_upstream"}
		}
	}
	if rel.Relation == SiblingIndependent {
		if spec.Kind != LinkShareSummary && spec.Kind != LinkShareScope {
			return ContextLinkDecision{RejectRule: "CL2_no_proposal"}
		}
		// CL1: share_summary requires parent decompose round.
		if spec.Kind == LinkShareSummary {
			if ctx.ParentRound == nil || ctx.ParentRound.SpawnPolicy != SpawnDecompose {
				return ContextLinkDecision{RejectRule: "CL1_requires_spawn_decompose"}
			}
			// OQ-CG-2: only completed → pending (one-way).
			if ctx.FromItem.Status != TaskStatusCompleted {
				return ContextLinkDecision{RejectRule: "CL1_from_not_completed"}
			}
			if ctx.ToItem.Status == TaskStatusCompleted {
				return ContextLinkDecision{RejectRule: "CL1_to_already_completed"}
			}
		}
	}
	kind := spec.Kind
	downgraded := false
	tokenCost := spec.MaxTokens
	if kind == LinkShareSummary {
		if tokenCost <= 0 {
			tokenCost = ctx.ShareTokenCap
		}
		if tokenCost > ctx.ShareTokenCap {
			kind = LinkShareSummary
			tokenCost = ctx.ShareTokenCap
			downgraded = true
		}
	}
	if kind == LinkShareScope && spec.MaxTokens > ctx.ShareTokenCap {
		return ContextLinkDecision{RejectRule: "CL7_token_exceeded"}
	}
	record := buildContextLinkRecord(ctx, kind, ContextLinkProposedByLLM, tokenCost)
	if wouldCreateContextCycle(ctx.ExistingLinks, record) {
		return ContextLinkDecision{RejectRule: "CL8_cycle"}
	}
	return ContextLinkDecision{Accepted: true, Record: record, Downgraded: downgraded}
}

// InferDependencyContextLink materializes CL0 / R2 BlockedBy → LinkUpstream.
func InferDependencyContextLink(upstream, dependent *WorkItem, upScope, depScope *ContextScope) *ContextLinkRecord {
	if upstream == nil || dependent == nil || upScope == nil || depScope == nil {
		return nil
	}
	if !containsID(dependent.BlockedBy, upstream.ID) {
		return nil
	}
	if isContextProjection(upstream) || isContextProjection(dependent) {
		return nil
	}
	now := time.Now().UTC()
	return &ContextLinkRecord{
		ID:             uuid.NewString(),
		FromScopeID:    upScope.ID,
		ToScopeID:      depScope.ID,
		FromWorkItemID: upstream.ID,
		ToWorkItemID:   dependent.ID,
		Kind:           LinkUpstream,
		ProposedBy:     ContextLinkProposedByRule + "R2_dependency",
		AppliedAt:      now,
	}
}

func buildContextLinkRecord(ctx ContextLinkEvalContext, kind ContextLinkKind, proposedBy string, tokenCost int) *ContextLinkRecord {
	fromScopeID := ""
	toScopeID := ""
	if ctx.FromScope != nil {
		fromScopeID = ctx.FromScope.ID
	}
	if ctx.ToScope != nil {
		toScopeID = ctx.ToScope.ID
	}
	return &ContextLinkRecord{
		ID:             uuid.NewString(),
		FromScopeID:    fromScopeID,
		ToScopeID:      toScopeID,
		FromWorkItemID: ctx.FromItem.ID,
		ToWorkItemID:   ctx.ToItem.ID,
		Kind:           kind,
		ProposedBy:     proposedBy,
		TokenCost:      tokenCost,
		AppliedAt:      time.Now().UTC(),
	}
}

func wouldCreateContextCycle(existing []ContextLinkRecord, next *ContextLinkRecord) bool {
	if next == nil {
		return false
	}
	adj := map[string][]string{}
	addEdge := func(from, to string) {
		adj[from] = append(adj[from], to)
	}
	for _, l := range existing {
		addEdge(l.FromWorkItemID, l.ToWorkItemID)
	}
	addEdge(next.FromWorkItemID, next.ToWorkItemID)
	visited := map[string]bool{}
	var dfs func(node string) bool
	dfs = func(node string) bool {
		if visited[node] {
			return true
		}
		visited[node] = true
		for _, nxt := range adj[node] {
			if dfs(nxt) {
				return true
			}
		}
		delete(visited, node)
		return false
	}
	return dfs(next.FromWorkItemID)
}
