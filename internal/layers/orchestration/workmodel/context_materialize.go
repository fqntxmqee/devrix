package workmodel

import (
	"github.com/devrix/devrix/internal/layers/orchestration/wavescheduler"
)

// WaveContextPolicyForItem maps WorkItem context state to Wave ContextPolicy (F3).
func WaveContextPolicyForItem(item *WorkItem) wavescheduler.ContextPolicy {
	if item == nil {
		return wavescheduler.ContextFresh
	}
	if item.ContextPolicy != "" && item.ContextPolicy.Valid() {
		return item.ContextPolicy.WaveContextPolicy()
	}
	if len(item.BlockedBy) > 0 {
		return wavescheduler.ContextUpstream
	}
	return wavescheduler.ContextFresh
}

// PrimaryUpstreamWorkItemID returns the first BlockedBy dependency for upstream projection.
func PrimaryUpstreamWorkItemID(item *WorkItem) string {
	if item == nil || len(item.BlockedBy) == 0 {
		return ""
	}
	return item.BlockedBy[0]
}

// WaveContextPolicy maps ContextLinkKind to Wave scheduler policy.
func (k ContextLinkKind) WaveContextPolicy() wavescheduler.ContextPolicy {
	switch k {
	case LinkUpstream, LinkShareSummary:
		return wavescheduler.ContextUpstream
	case LinkResume:
		return wavescheduler.ContextResume
	default:
		return wavescheduler.ContextFresh
	}
}

// ProjectWaveTaskNode builds a dispatch-time TaskNode with ContextGraph policy (F3).
func ProjectWaveTaskNode(item *WorkItem) wavescheduler.TaskNode {
	if item == nil {
		return wavescheduler.TaskNode{}
	}
	policy := WaveContextPolicyForItem(item)
	node := wavescheduler.TaskNode{
		ID:            item.ID,
		Title:         item.Title,
		Directive:     item.Directive,
		WorkerType:    wavescheduler.WorkerSubAgent,
		ContextPolicy: policy,
		DependsOn:     append([]string(nil), item.BlockedBy...),
		Metadata:      map[string]any{"work_item_id": item.ID},
	}
	if policy == wavescheduler.ContextUpstream {
		node.UpstreamTaskID = PrimaryUpstreamWorkItemID(item)
	}
	if item.ContextScopeID != "" {
		if node.Metadata == nil {
			node.Metadata = map[string]any{}
		}
		node.Metadata["context_scope_id"] = item.ContextScopeID
		node.SidechainAgentID = ContextScopeSidechainKey(item.ID)
	}
	return node
}

// InferWorkItemContextPolicy sets ContextPolicy from BlockedBy.
func InferWorkItemContextPolicy(item *WorkItem) ContextLinkKind {
	if item == nil {
		return LinkFresh
	}
	if len(item.BlockedBy) > 0 {
		return LinkUpstream
	}
	if item.ContextPolicy != "" {
		return item.ContextPolicy
	}
	return LinkFresh
}
