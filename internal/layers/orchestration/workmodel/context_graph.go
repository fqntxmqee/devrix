package workmodel

import "time"

// ContextLinkKind describes horizontal or materialized context sharing between scopes.
type ContextLinkKind string

const (
	LinkFresh        ContextLinkKind = "fresh"
	LinkUpstream     ContextLinkKind = "upstream"
	LinkShareSummary ContextLinkKind = "share_summary"
	LinkShareScope   ContextLinkKind = "share_scope"
	LinkResume       ContextLinkKind = "resume"
)

// ContextBubbleKind describes vertical child→parent context bubbling.
type ContextBubbleKind string

const (
	BubbleNone        ContextBubbleKind = "none"
	BubbleStructured  ContextBubbleKind = "structured"
	BubbleSummary     ContextBubbleKind = "summary"
	BubbleKeyMessages ContextBubbleKind = "key_messages"
	BubbleFullTail    ContextBubbleKind = "full_tail"
)

// Valid reports whether k is a known link kind.
func (k ContextLinkKind) Valid() bool {
	switch k {
	case LinkFresh, LinkUpstream, LinkShareSummary, LinkShareScope, LinkResume:
		return true
	default:
		return false
	}
}

// Valid reports whether k is a known bubble kind.
func (k ContextBubbleKind) Valid() bool {
	switch k {
	case BubbleNone, BubbleStructured, BubbleSummary, BubbleKeyMessages, BubbleFullTail:
		return true
	default:
		return false
	}
}

// ContextLinkSpec is an LLM or rule proposal for a context link (CG3).
type ContextLinkSpec struct {
	FromWorkItemID string          `json:"from_work_item_id"`
	ToWorkItemID   string          `json:"to_work_item_id"`
	Kind           ContextLinkKind `json:"kind"`
	MaxTokens      int             `json:"max_tokens,omitempty"`
	Rationale      string          `json:"rationale,omitempty"`
}

// ContextBubbleSpec is an LLM proposal for upward context bubbling.
type ContextBubbleSpec struct {
	TargetWorkItemID string            `json:"target_work_item_id"`
	Kind             ContextBubbleKind `json:"kind"`
	MaxTokens        int               `json:"max_tokens,omitempty"`
	Rationale        string            `json:"rationale,omitempty"`
}

// ContextLinkRecord is the audit trail for a materialized context link (CG2).
type ContextLinkRecord struct {
	ID           string          `json:"id"`
	FromScopeID  string          `json:"from_scope_id"`
	ToScopeID    string          `json:"to_scope_id"`
	FromWorkItemID string        `json:"from_work_item_id"`
	ToWorkItemID   string        `json:"to_work_item_id"`
	Kind         ContextLinkKind `json:"kind"`
	ProposedBy   string          `json:"proposed_by"`
	TokenCost    int             `json:"token_cost,omitempty"`
	AppliedAt    time.Time       `json:"applied_at"`
}

// ContextLinkProposedByRule prefixes rule-driven link records.
const ContextLinkProposedByRule = "rule:"

// ContextLinkProposedByLLM marks LLM-proposed accepted links.
const ContextLinkProposedByLLM = "llm"

// DefaultShareSummaryMaxTokens is the default cap for sibling share_summary (CL7).
const DefaultShareSummaryMaxTokens = 2048
