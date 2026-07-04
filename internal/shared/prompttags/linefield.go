package prompttags

import (
	"fmt"
	"strings"
)

// User-frame tag names for Observe/Plan LLM prompts (key: value lines, not envelope tags).
const (
	TagWorkItemID         TagName = "work_item_id"
	TagDirective          TagName = "directive"
	TagPriorMean          TagName = "prior_mean"
	TagScopeGoal          TagName = "scope_goal"
	TagScopeOpenQuestion  TagName = "scope_open_question"
	TagSignal             TagName = "signal"
	TagObservationIDs     TagName = "observation_ids"
	TagObservationSummary TagName = "observation_summary"
	TagDepth              TagName = "depth"
	TagMaxDepth           TagName = "max_depth"
	TagExistingChildren   TagName = "existing_children"
	TagRemainingChildren  TagName = "remaining_children"
	TagMaxChildren        TagName = "max_children"
	TagDecomposeUsedToday TagName = "decompose_used_today"
	TagRemainingDaily     TagName = "remaining_daily"
	TagMaxDaily           TagName = "max_daily"
	TagMaxIters           TagName = "max_iters"
	TagParentScopeIn      TagName = "parent_scope_in"
	TagUncertaintyMean    TagName = "uncertainty_mean"
)

// FrameSpec defines fixed-order key: value lines for a MUPS user prompt frame.
type FrameSpec struct {
	Fields []TagName
}

// ObserveUserFrame is the field order for Observe-phase user prompts.
var ObserveUserFrame = FrameSpec{
	Fields: []TagName{
		TagWorkItemID,
		TagDirective,
		TagPriorMean,
		TagScopeGoal,
		TagScopeOpenQuestion,
		TagSignal,
	},
}

// PlanUserFrame is the field order for Plan-phase user prompts (RH-MUPS-07 T-P1-4).
var PlanUserFrame = FrameSpec{
	Fields: []TagName{
		TagWorkItemID,
		TagDirective,
		TagObservationIDs,
		TagObservationSummary,
		TagDepth,
		TagMaxDepth,
		TagExistingChildren,
		TagRemainingChildren,
		TagMaxChildren,
		TagDecomposeUsedToday,
		TagRemainingDaily,
		TagMaxDaily,
		TagMaxIters,
		TagParentScopeIn,
		TagUncertaintyMean,
	},
}

// BuildLineFrame serializes fields into newline-separated key: value lines per spec order.
// Omitted map entries are skipped. Repeated-line tags emit one line per slice element.
func BuildLineFrame(spec FrameSpec, fields map[TagName]any) string {
	var b strings.Builder
	for _, name := range spec.Fields {
		v, ok := fields[name]
		if !ok {
			continue
		}
		writeLineField(&b, name, v)
	}
	return b.String()
}

func writeLineField(b *strings.Builder, name TagName, v any) {
	switch name {
	case TagScopeOpenQuestion, TagSignal:
		lines, ok := v.([]string)
		if !ok {
			return
		}
		for _, line := range lines {
			if name == TagScopeOpenQuestion && strings.TrimSpace(line) == "" {
				continue
			}
			fmt.Fprintf(b, "%s: %s\n", name, line)
		}
	case TagObservationIDs, TagParentScopeIn:
		lines, ok := v.([]string)
		if !ok || len(lines) == 0 {
			return
		}
		fmt.Fprintf(b, "%s: %s\n", name, strings.Join(lines, ","))
	case TagPriorMean:
		f, ok := v.(float64)
		if !ok || f <= 0 {
			return
		}
		fmt.Fprintf(b, "%s: %.3f\n", name, f)
	case TagUncertaintyMean:
		f, ok := v.(float64)
		if !ok || f <= 0 {
			return
		}
		fmt.Fprintf(b, "%s: %.3f\n", name, f)
	default:
		switch val := v.(type) {
		case string:
			fmt.Fprintf(b, "%s: %s\n", name, val)
		case int:
			fmt.Fprintf(b, "%s: %d\n", name, val)
		case int64:
			fmt.Fprintf(b, "%s: %d\n", name, val)
		}
	}
}
