package workmodel

import (
	"fmt"
	"strings"
	"unicode/utf8"

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
		// DM-20260629-001 / T53: read BubbleKind via typed RollupReport
		// envelope instead of scattered child.LastRound.* access.
		report := NewRollupReportFromRound(child.ID, child.LastRound)
		if report == nil {
			continue
		}
		kind := report.BubbleKind
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
	if round.StructuredDeliverable != nil && len(round.StructuredDeliverable.Findings) > 0 {
		parts = append(parts, fmt.Sprintf("findings_count=%d", len(round.StructuredDeliverable.Findings)))
		f := round.StructuredDeliverable.Findings[0]
		if f.File != "" {
			parts = append(parts, fmt.Sprintf("finding_sample=%s:%d %s", f.File, f.Line, f.Severity))
		}
	} else if round.DeliverableStatus == DeliverableStatusIncomplete {
		parts = append(parts, "deliverable=incomplete")
	}
	return "structured_child_bubble: " + strings.Join(parts, "; ")
}

// IsTerminalStatus reports completed/failed/cancelled work items.
func IsTerminalStatus(s TaskStatus) bool {
	return isTerminalStatus(s)
}

// ChildChecklistBubble pairs an ephemeral checklist child for virtual rollup Observe.
type ChildChecklistBubble struct {
	ChildID string
	Item    *WorkItem
}

// CollectChecklistChildBubbles returns direct ephemeral checklist children (Path B).
func CollectChecklistChildBubbles(tm *TaskManager, sessionID, parentID string) []ChildChecklistBubble {
	if tm == nil || parentID == "" {
		return nil
	}
	var out []ChildChecklistBubble
	for _, child := range tm.Tree().ListChildren(sessionID, parentID) {
		if child == nil || child.Kind != WorkKindChecklist || !child.Ephemeral {
			continue
		}
		out = append(out, ChildChecklistBubble{ChildID: child.ID, Item: child})
	}
	return out
}

// truncateBubblePreview applies CB3 rune budget with ellipsis when truncated.
func truncateBubblePreview(s string, maxRunes int) string {
	if maxRunes <= 0 {
		maxRunes = DefaultShareSummaryMaxTokens
	}
	if s == "" || maxRunes <= 0 {
		return s
	}
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	var b strings.Builder
	count := 0
	for _, r := range s {
		if count >= maxRunes {
			b.WriteString("…")
			break
		}
		b.WriteRune(r)
		count++
	}
	return b.String()
}

// ChecklistBubbleStatement formats virtual checklist input for rollup Observe (design §5.8).
func ChecklistBubbleStatement(childID string, item *WorkItem) string {
	if item == nil {
		return ""
	}
	directive := strings.TrimSpace(item.Directive)
	if directive == "" {
		directive = strings.TrimSpace(item.Title)
	}
	directive = truncateBubblePreview(directive, DefaultShareSummaryMaxTokens)
	return fmt.Sprintf(
		"checklist_child_bubble: child=%s; status=%s; directive=%q",
		childID, item.Status, directive,
	)
}

// ChildSummaryBubble pairs a terminal child with artifact summary for parent Observe.
type ChildSummaryBubble struct {
	ChildID string
	Summary string
}

// CollectSummaryChildBubbles returns terminal children's artifact summaries (CB3 input).
func CollectSummaryChildBubbles(tm *TaskManager, sessionID, parentID string) []ChildSummaryBubble {
	if tm == nil || parentID == "" {
		return nil
	}
	var out []ChildSummaryBubble
	for _, child := range tm.Tree().ListChildren(sessionID, parentID) {
		if child == nil || child.Kind == WorkKindChecklist || child.LastRound == nil {
			continue
		}
		if !IsTerminalStatus(child.Status) {
			continue
		}
		// DM-20260629-001 / T53: read ArtifactSummary via typed
		// RollupReport envelope instead of scattered child.LastRound.* access.
		report := NewRollupReportFromRound(child.ID, child.LastRound)
		if report == nil {
			continue
		}
		summary := strings.TrimSpace(report.ArtifactSummary)
		if summary == "" {
			continue
		}
		out = append(out, ChildSummaryBubble{ChildID: child.ID, Summary: summary})
	}
	return out
}

// SummaryBubbleStatement formats a child's artifact summary for parent Observe (CB3 truncate).
func SummaryBubbleStatement(childID string, artifactSummary string) string {
	summary := truncateBubblePreview(strings.TrimSpace(artifactSummary), DefaultShareSummaryMaxTokens)
	if summary == "" {
		return ""
	}
	return fmt.Sprintf("summary_child_bubble: child=%s; preview=%q", childID, summary)
}

// ChildUncertaintyBubble is the upward-bubble entry for a high-
// uncertainty child (RH-MUPS-06, DM-20260701-001 T-P2-3). When a
// child WorkItem completes with UncertaintyMean above the configured
// threshold, the parent Observe phase should see an explicit
// "this child is uncertain, do something about it" signal — not just
// the rolled-up mean. The parent's Planner / Learn can then act on
// the bubble (escalate, decompose further, or surface a question to
// the user).
type ChildUncertaintyBubble struct {
	ChildID     string
	Uncertainty float64
	VerdictKind string
	Reason      string
}

// ChildUncertaintyBubbleThreshold is the cutoff above which a child's
// terminal round emits a ChildUncertaintyBubble. Tuned to
// DefaultUncertaintyDecomposeThreshold (0.6) so the bubble fires for
// the same condition that would have triggered decompose had the
// child been a parent. Forward-compat: per-item override can live on
// the round if needed.
const ChildUncertaintyBubbleThreshold = DefaultUncertaintyDecomposeThreshold

// CollectChildUncertaintyBubbles walks the children and emits one
// bubble per child whose terminal round ended with UncertaintyMean ≥
// ChildUncertaintyBubbleThreshold. Empty when no child qualifies.
// Pure function over (tm, sessionID, parentID); safe to call inside
// the rollup / Observe phase.
func CollectChildUncertaintyBubbles(tm *TaskManager, sessionID, parentID string) []ChildUncertaintyBubble {
	if tm == nil || parentID == "" {
		return nil
	}
	var out []ChildUncertaintyBubble
	for _, child := range tm.Tree().ListChildren(sessionID, parentID) {
		if child == nil || child.Kind == WorkKindChecklist || child.LastRound == nil {
			continue
		}
		if !IsTerminalStatus(child.Status) {
			continue
		}
		u := child.LastRound.UncertaintyMean
		if u < ChildUncertaintyBubbleThreshold {
			continue
		}
		out = append(out, ChildUncertaintyBubble{
			ChildID:     child.ID,
			Uncertainty: u,
			VerdictKind: child.LastRound.VerdictKind.String(),
			Reason:      child.LastRound.ExitReason,
		})
	}
	return out
}

// ChildUncertaintyBubbleStatement renders one bubble as a
// machine-parseable string for the parent's Observe SignalLines.
// Format mirrors StructuredBubbleStatement so downstream consumers
// can apply a single parser.
func ChildUncertaintyBubbleStatement(b ChildUncertaintyBubble) string {
	parts := []string{
		fmt.Sprintf("child=%s", b.ChildID),
		fmt.Sprintf("uncertainty=%.3f", b.Uncertainty),
		fmt.Sprintf("threshold=%.3f", ChildUncertaintyBubbleThreshold),
		fmt.Sprintf("verdict=%s", b.VerdictKind),
	}
	if reason := strings.TrimSpace(b.Reason); reason != "" {
		parts = append(parts, fmt.Sprintf("reason=%q", reason))
	}
	return "child_uncertainty_bubble: " + strings.Join(parts, "; ")
}
