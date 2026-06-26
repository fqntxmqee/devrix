package workmodel

import (
	"github.com/devrix/devrix/internal/layers/orchestration/plan"
	"github.com/devrix/devrix/internal/shared/types"
)

// ContextBubbleEvalContext supplies constraints for ContextBubbleEvaluator (CB0–CB6).
type ContextBubbleEvalContext struct {
	Child          *WorkItem
	Target         *WorkItem // parent or ancestor
	PersistScope   plan.PersistScope
	Depth          int
	MaxDepth       int
	TokenBudget    int
	EscapeExhausted bool
	Round          *WorkItemPipelineRound
}

// ContextBubbleDecision is the rule output for upward bubbling.
type ContextBubbleDecision struct {
	Kind       ContextBubbleKind
	Downgraded bool
	RejectRule string
}

// ContextBubbleEvaluator applies CB0–CB6. Structured bubble is always enforced separately (CB0).
func ContextBubbleEvaluator(spec *ContextBubbleSpec, ctx ContextBubbleEvalContext) ContextBubbleDecision {
	if ctx.MaxDepth <= 0 {
		ctx.MaxDepth = DefaultMaxDecomposeDepth
	}
	if ctx.TokenBudget <= 0 {
		ctx.TokenBudget = DefaultShareSummaryMaxTokens
	}
	// CB5: human review gate — no narrative bubble.
	if ctx.Child != nil && IsHumanReviewItem(ctx.Child) {
		return ContextBubbleDecision{Kind: BubbleNone, RejectRule: "CB5_human_review"}
	}
	if ctx.Target != nil && IsHumanReviewItem(ctx.Target) && ctx.Child != nil && ctx.Child.Status == TaskStatusPending {
		return ContextBubbleDecision{Kind: BubbleNone, RejectRule: "CB5_human_review_pending"}
	}
	// CB6: escape budget exhausted — structured only.
	if ctx.EscapeExhausted {
		return ContextBubbleDecision{Kind: BubbleStructured}
	}
	proposed := BubbleStructured
	if spec != nil && spec.Kind.Valid() {
		proposed = spec.Kind
	}
	// CB0 default: at minimum structured; LLM cannot propose below structured via None.
	if proposed == BubbleNone {
		proposed = BubbleStructured
	}
	// CB1: transient scope caps narrative bubble.
	if ctx.PersistScope == plan.PersistTransient {
		if proposed == BubbleFullTail || proposed == BubbleKeyMessages {
			return ContextBubbleDecision{Kind: BubbleStructured, Downgraded: true, RejectRule: "CB1_transient_cap"}
		}
	}
	// CB2: max depth caps at summary.
	if ctx.Depth >= ctx.MaxDepth {
		if rankBubble(proposed) > rankBubble(BubbleSummary) {
			return ContextBubbleDecision{Kind: BubbleSummary, Downgraded: true, RejectRule: "CB2_max_depth"}
		}
	}
	kind := proposed
	downgraded := false
	maxTokens := 0
	if spec != nil {
		maxTokens = spec.MaxTokens
	}
	// CB3: token budget downgrade chain full_tail → summary → structured.
	if maxTokens > ctx.TokenBudget && kind == BubbleFullTail {
		kind = BubbleSummary
		downgraded = true
	}
	if maxTokens > ctx.TokenBudget && kind == BubbleSummary {
		kind = BubbleStructured
		downgraded = true
	}
	if maxTokens > ctx.TokenBudget && kind == BubbleKeyMessages {
		kind = BubbleStructured
		downgraded = true
	}
	// CB4: VerdictFail + exploratory plan allows key_messages when within budget.
	if ctx.Round != nil && ctx.Round.VerdictKind == types.VerdictFail && IsExploratoryPlanKind(ctx.Round.PlanKind) {
		if spec != nil && spec.Kind == BubbleKeyMessages && maxTokens <= ctx.TokenBudget {
			return ContextBubbleDecision{Kind: BubbleKeyMessages}
		}
	}
	if rankBubble(kind) < rankBubble(BubbleStructured) {
		kind = BubbleStructured
	}
	return ContextBubbleDecision{Kind: kind, Downgraded: downgraded}
}

func rankBubble(k ContextBubbleKind) int {
	switch k {
	case BubbleNone:
		return 0
	case BubbleStructured:
		return 1
	case BubbleSummary:
		return 2
	case BubbleKeyMessages:
		return 3
	case BubbleFullTail:
		return 4
	default:
		return 1
	}
}
