package workmodel

import (
	"strings"
)

// DetectPlanningMeta classifies planning/transition meta (category: planning_meta).
// Uses structural template markers only — no NL phrase enumeration.
var structuralPlanningMetaMarkers = []string{
	"<scope_contract>",
	"<directive_template>",
	"<task_recap>",
	"<planning>",
	"<open_questions>",
	// MiniMax phantom tool markup leaked into synthesis text when tools are disabled.
	"<tool_call>",
	"<invoke",
}

// DetectPlanningMeta reports whether summary matches the planning_meta reject category.
func DetectPlanningMeta(summary string) bool {
	lower := strings.ToLower(strings.TrimSpace(summary))
	if lower == "" {
		return false
	}
	for _, m := range structuralPlanningMetaMarkers {
		if strings.Contains(lower, strings.ToLower(m)) {
			return true
		}
	}
	return isTransitionMetaPrefix(lower)
}

func isTransitionMetaPrefix(lower string) bool {
	for _, p := range []string{
		"let me continue",
		"let me read",
		"let me explore",
		"let me first",
		"let me start",
		"i'll examine",
		"i will examine",
		"the user wants me to",
		"我将要",
		"parallel explore",
		"继续探索",
		"继续查看",
		"继续阅读",
	} {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}
