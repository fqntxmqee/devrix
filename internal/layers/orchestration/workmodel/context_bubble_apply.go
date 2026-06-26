package workmodel

import (
	"fmt"
	"strings"

	"github.com/devrix/devrix/internal/layers/orchestration/plan"
)

// ApplyContextBubbleDecision writes the evaluated bubble kind onto the pipeline round (F2).
func ApplyContextBubbleDecision(round *WorkItemPipelineRound, spec *ContextBubbleSpec, ctx ContextBubbleEvalContext) ContextBubbleDecision {
	dec := ContextBubbleEvaluator(spec, ctx)
	if round != nil {
		round.ContextBubbleKind = dec.Kind
		if round.ContextBubbleKind == "" {
			round.ContextBubbleKind = BubbleStructured
		}
	}
	return dec
}

// DefaultContextBubbleEvalContext builds bubble evaluation input for a completing child.
func DefaultContextBubbleEvalContext(child, parent *WorkItem, round *WorkItemPipelineRound, tm *TaskManager, sessionID string) ContextBubbleEvalContext {
	ctx := ContextBubbleEvalContext{
		Child:  child,
		Target: parent,
		Round:  round,
	}
	if child != nil && tm != nil && sessionID != "" {
		ctx.Depth = tm.Tree().Depth(sessionID, child.ID)
	}
	if tm != nil && tm.Tree().MaxDecomposeDepth() > 0 {
		ctx.MaxDepth = tm.Tree().MaxDecomposeDepth()
	} else {
		ctx.MaxDepth = DefaultMaxDecomposeDepth
	}
	if round != nil && round.PlanID != "" {
		ctx.PersistScope = plan.PersistSession
	}
	return ctx
}

// ChildStructuredBubble pairs a terminal child with its structured pipeline round.
type ChildStructuredBubble struct {
	ChildID string
	Round   *WorkItemPipelineRound
}

// CollectStructuredChildBubbles returns terminal children's rounds eligible for parent Observe (CB0).
func CollectStructuredChildBubbles(tm *TaskManager, sessionID, parentID string) []ChildStructuredBubble {
	if tm == nil || parentID == "" {
		return nil
	}
	children := tm.Tree().ListChildren(sessionID, parentID)
	var out []ChildStructuredBubble
	for _, child := range children {
		if child == nil || child.LastRound == nil {
			continue
		}
		if child.Kind == WorkKindChecklist {
			continue
		}
		if !IsTerminalStatus(child.Status) {
			continue
		}
		kind := child.LastRound.ContextBubbleKind
		if kind == BubbleNone {
			continue
		}
		if kind == "" {
			kind = BubbleStructured
		}
		if rankBubble(kind) < rankBubble(BubbleStructured) {
			continue
		}
		out = append(out, ChildStructuredBubble{ChildID: child.ID, Round: child.LastRound})
	}
	return out
}

// StructuredBubbleStatement formats LP-5 fields for Observe injection (design §6.1).
func StructuredBubbleStatement(childID string, round *WorkItemPipelineRound) string {
	if round == nil {
		return ""
	}
	parts := []string{
		fmt.Sprintf("child=%s", childID),
		fmt.Sprintf("verdict=%s", round.VerdictKind),
		fmt.Sprintf("plan=%s", round.PlanID),
		fmt.Sprintf("verdict_id=%s", round.VerdictID),
		fmt.Sprintf("uncertainty=%.3f", round.UncertaintyMean),
	}
	if round.SpawnPolicy != "" {
		parts = append(parts, fmt.Sprintf("spawn=%s", round.SpawnPolicy))
	}
	if len(round.ObservationIDs) > 0 {
		parts = append(parts, fmt.Sprintf("observations=%s", strings.Join(round.ObservationIDs, ",")))
	}
	return "structured_child_bubble: " + strings.Join(parts, "; ")
}

// IsTerminalStatus reports completed/failed/cancelled work items.
func IsTerminalStatus(s TaskStatus) bool {
	return isTerminalStatus(s)
}
