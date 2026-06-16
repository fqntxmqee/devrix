package fallback

import (
	"regexp"
	"strings"

	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

var (
	promptInjectionPattern = regexp.MustCompile(`(?i)(ignore (all )?previous instructions|system prompt override|jailbreak)`)
	sensitiveKeywordPattern = regexp.MustCompile(`(?i)(password|api[_-]?key|secret|token)`)
)

// PreflightEvaluator performs rule-based pre-LLM context evaluation.
type PreflightEvaluator struct {
	cfg config.PreflightConfig
}

// NewPreflightEvaluator creates a preflight evaluator.
func NewPreflightEvaluator(cfg config.PreflightConfig, _ IToolPoolFilter) *PreflightEvaluator {
	return &PreflightEvaluator{cfg: cfg}
}

// Evaluate scores assembled context and optionally filters irrelevant tools.
func (e *PreflightEvaluator) Evaluate(
	_ *types.SessionContext,
	userMessage string,
	visibleTools []ToolDesc,
	assembledContext string,
) types.PreflightResult {
	result := types.PreflightResult{Mode: e.cfg.Mode}
	if e.cfg.Mode == "" {
		result.Mode = config.PreflightModeWarnOnly
	}

	msg := strings.TrimSpace(userMessage)
	if msg == "" {
		result.Scores.Completeness = 0
		result.Warnings = append(result.Warnings, "empty user message")
	} else {
		result.Scores.Completeness = 100
	}

	result.Scores.Relevance = 80
	if len(msg) < 5 {
		result.Scores.Relevance = 40
		result.Warnings = append(result.Warnings, "message too short for reliable routing")
	}

	result.Scores.Safety = 100
	if promptInjectionPattern.MatchString(msg) {
		result.Scores.Safety = 20
		result.Warnings = append(result.Warnings, "possible prompt injection pattern detected")
	}
	if sensitiveKeywordPattern.MatchString(msg) {
		result.Scores.Safety = minInt(result.Scores.Safety, 60)
		result.Warnings = append(result.Warnings, "message contains sensitive keywords")
	}

	estimatedTokens := len(assembledContext) / 4
	if e.cfg.TokenBudget > 0 {
		ratio := float64(estimatedTokens) / float64(e.cfg.TokenBudget)
		if ratio >= e.cfg.WarnRatio {
			result.Scores.TokenBudget = int((1 - ratio) * 100)
			if result.Scores.TokenBudget < 0 {
				result.Scores.TokenBudget = 0
			}
			result.Warnings = append(result.Warnings, "assembled context approaching token budget")
		} else {
			result.Scores.TokenBudget = 100
		}
	}

	if e.cfg.ToolFilter.Enabled && e.cfg.ToolFilter.Mode == config.PreflightToolFilterAutoRepair {
		_, decision := filterRelevantTools(msg, visibleTools)
		result.ToolFilter = decision
	}
	return result
}

// FilterVisibleTools applies preflight tool filter to visible tools when enabled.
func (e *PreflightEvaluator) FilterVisibleTools(
	userMessage string,
	visibleTools []ToolDesc,
) ([]ToolDesc, types.ToolFilterDecision) {
	if e == nil || !e.cfg.Enabled || !e.cfg.ToolFilter.Enabled ||
		e.cfg.ToolFilter.Mode != config.PreflightToolFilterAutoRepair {
		return visibleTools, types.ToolFilterDecision{KeptTools: toolNames(visibleTools)}
	}
	return filterRelevantTools(userMessage, visibleTools)
}

func filterRelevantTools(message string, tools []ToolDesc) ([]ToolDesc, types.ToolFilterDecision) {
	tokens := tokenize(message)
	if len(tokens) == 0 || len(tools) == 0 {
		return tools, types.ToolFilterDecision{KeptTools: toolNames(tools)}
	}

	kept := make([]ToolDesc, 0, len(tools))
	removed := make([]string, 0)
	for _, tool := range tools {
		if _, ok := simpleModeTools[tool.Name]; ok {
			kept = append(kept, tool)
			continue
		}
		score := scoreTokens(tokens, tool.Name+" "+tool.Description)
		if score > 0 {
			kept = append(kept, tool)
		} else {
			removed = append(removed, tool.Name)
		}
	}
	if len(kept) == 0 {
		kept = tools
		removed = nil
	}
	return kept, types.ToolFilterDecision{
		Applied:      len(removed) > 0,
		RemovedTools: removed,
		KeptTools:    toolNames(kept),
	}
}

func toolNames(tools []ToolDesc) []string {
	out := make([]string, 0, len(tools))
	for _, t := range tools {
		out = append(out, t.Name)
	}
	return out
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
