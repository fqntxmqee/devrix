package workmodel

import (
	"fmt"
	"strings"
)

// MUPS round trigger labels (pipeline.trigger). Stable identity uses round_no;
// trigger is metadata for display and Jaeger filters.
const (
	MUPSTriggerInitial = "initial"
	MUPSTriggerInline  = "inline"
	MUPSTriggerRollup  = "rollup"
	MUPSTriggerResume  = "resume"
	MUPSTriggerRefocus = "refocus"
)

// LocatorFrame is the cross-layer breadcrumb for Session → Turn → Loop → WI → MUPS.
type LocatorFrame struct {
	SessionID    string
	TurnNo       int
	LoopTick     int
	WorkItemID   string
	SemanticID   string
	Depth        int
	SiblingIndex int
	RoundNo      int
	Trigger      string
	Phase        string
	Iter         int
}

// KindSemanticSuffix maps WorkKind to the semantic_id tail segment.
func KindSemanticSuffix(kind WorkKind) string {
	switch kind {
	case WorkKindGoal:
		return "goal"
	case WorkKindImplement:
		return "impl"
	case WorkKindExplore:
		return "explore"
	case WorkKindVerify:
		return "verify"
	case WorkKindPlan:
		return "plan"
	case WorkKindShell:
		return "shell"
	case WorkKindAgent:
		return "agent"
	case WorkKindChecklist:
		return "checklist"
	default:
		if kind == "" {
			return "item"
		}
		return strings.ToLower(string(kind))
	}
}

// BuildSemanticID formats wi_d{depth}_s{sibling}_{kind}.
func BuildSemanticID(depth, sibling int, kind WorkKind) string {
	if depth < 0 {
		depth = 0
	}
	if sibling < 0 {
		sibling = 0
	}
	return fmt.Sprintf("wi_d%d_s%d_%s", depth, sibling, KindSemanticSuffix(kind))
}

// FormatMUPSRoundLabel returns stable round identity mups-r{n} (n >= 1).
func FormatMUPSRoundLabel(roundNo int) string {
	if roundNo < 1 {
		roundNo = 1
	}
	return fmt.Sprintf("mups-r%d", roundNo)
}

// FormatMUPSRoundDisplay returns human-readable mups-r{n}(trigger) when trigger set.
func FormatMUPSRoundDisplay(roundNo int, trigger string) string {
	label := FormatMUPSRoundLabel(roundNo)
	trigger = strings.TrimSpace(trigger)
	if trigger == "" {
		return label
	}
	return label + "(" + trigger + ")"
}

// MUPSRoundLabel implements stable round label on WorkItemPipelineRound.
func (r *WorkItemPipelineRound) MUPSRoundLabel() string {
	if r == nil {
		return FormatMUPSRoundLabel(1)
	}
	return FormatMUPSRoundLabel(r.RoundNo)
}

// MUPSRoundDisplay includes trigger for logs and IM footers.
func (r *WorkItemPipelineRound) MUPSRoundDisplay() string {
	if r == nil {
		return FormatMUPSRoundDisplay(1, MUPSTriggerInitial)
	}
	return FormatMUPSRoundDisplay(r.RoundNo, r.Trigger)
}

// BuildLocator assembles the URL-style locator string.
//
// Example: sess_x/turn-1/loop-3/wi_d0_s0_goal/mups-r2+inline/execute/iter-2
func BuildLocator(f LocatorFrame) string {
	parts := make([]string, 0, 8)
	if sid := strings.TrimSpace(f.SessionID); sid != "" {
		parts = append(parts, sid)
	}
	if f.TurnNo > 0 {
		parts = append(parts, fmt.Sprintf("turn-%d", f.TurnNo))
	}
	if f.LoopTick > 0 {
		parts = append(parts, fmt.Sprintf("loop-%d", f.LoopTick))
	}
	sem := strings.TrimSpace(f.SemanticID)
	if sem == "" && f.WorkItemID != "" {
		sem = f.WorkItemID
	}
	if sem != "" {
		parts = append(parts, sem)
	}
	if f.RoundNo > 0 {
		label := FormatMUPSRoundLabel(f.RoundNo)
		if tr := strings.TrimSpace(f.Trigger); tr != "" {
			parts = append(parts, label+"+"+tr)
		} else {
			parts = append(parts, label)
		}
	}
	if ph := strings.TrimSpace(f.Phase); ph != "" {
		parts = append(parts, ph)
	}
	if f.Iter > 0 {
		parts = append(parts, fmt.Sprintf("iter-%d", f.Iter))
	}
	return strings.Join(parts, "/")
}

// LocatorForRound builds a locator from round + optional phase/iter.
func LocatorForRound(f LocatorFrame, round *WorkItemPipelineRound, phase string, iter int) string {
	if round != nil {
		f.RoundNo = round.RoundNo
		f.Trigger = round.Trigger
		if f.LoopTick == 0 {
			f.LoopTick = round.LoopTick
		}
	}
	f.Phase = phase
	f.Iter = iter
	return BuildLocator(f)
}

// InferMUPSTrigger derives pipeline.trigger for the upcoming MUPS round.
func InferMUPSTrigger(item *WorkItem, isRollup bool) string {
	if isRollup {
		return MUPSTriggerRollup
	}
	if item == nil || item.LastRound == nil {
		return MUPSTriggerInitial
	}
	switch item.LastRound.SpawnPolicy {
	case SpawnInline:
		return MUPSTriggerInline
	case SpawnNone:
		if DeliverableContinuationRequired(item.LastRound) {
			return MUPSTriggerRefocus
		}
	}
	return MUPSTriggerInitial
}
