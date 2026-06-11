package pev

import (
	"strings"

	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

// PlanSignals are phrases that indicate planning is beneficial.
var PlanSignals = []string{
	"添加新功能",
	"实现",
	"重构",
	"添加功能",
	"开发",
	"设计",
	"架构",
	"添加模块",
	"实现功能",
	"新增",
	"构建",
	"搭建",
	"创建",
	"add feature",
	"implement",
	"refactor",
	"design",
	"architecture",
	"new module",
	"complex",
	"multiple",
}

// ShouldPlan reports whether the Plan phase should run for this message.
func ShouldPlan(cfg config.PlanConfig, state types.PEVState, message string) bool {
	// Disabled check
	if !cfg.Enabled {
		return false
	}

	// Active milestone check - don't replan mid-execution
	if state.ActiveMilestoneID != "" || state.ActiveTaskID != "" {
		return false
	}

	// Explicit command trigger
	if strings.HasPrefix(strings.TrimSpace(message), "/plan") {
		return true
	}

	// Auto-detect trigger
	if cfg.AutoDetect {
		// Check message length threshold
		minChars := cfg.MinCharsForPlan
		if minChars <= 0 {
			minChars = 200
		}
		if len(message) >= minChars {
			return true
		}

		// Check for planning signals
		if hasPlanSignal(message) {
			return true
		}
	}

	return false
}

// hasPlanSignal checks if message contains planning signals.
func hasPlanSignal(message string) bool {
	msgLower := strings.ToLower(message)
	for _, signal := range PlanSignals {
		if strings.Contains(msgLower, strings.ToLower(signal)) {
			return true
		}
	}
	return false
}

// ShouldPlanDetailed returns detailed analysis of why planning should/shouldn't run.
func ShouldPlanDetailed(cfg config.PlanConfig, state types.PEVState, message string) PlanDecision {
	decision := PlanDecision{
		Message: message,
	}

	// Check each condition
	if !cfg.Enabled {
		decision.Reasons = append(decision.Reasons, "Plan mode is disabled")
		return decision
	}

	if state.ActiveMilestoneID != "" {
		decision.Reasons = append(decision.Reasons, "Already executing milestone: "+state.ActiveMilestoneID)
		return decision
	}

	if strings.HasPrefix(strings.TrimSpace(message), "/plan") {
		decision.ShouldPlan = true
		decision.Reasons = append(decision.Reasons, "Explicit /plan command")
		return decision
	}

	if !cfg.AutoDetect {
		decision.Reasons = append(decision.Reasons, "Auto-detect is disabled")
		return decision
	}

	minChars := cfg.MinCharsForPlan
	if minChars <= 0 {
		minChars = 200
	}
	if len(message) >= minChars {
		decision.ShouldPlan = true
		decision.Reasons = append(decision.Reasons, "Message length: "+itoa(len(message))+" >= "+itoa(minChars))
	}

	if hasPlanSignal(message) {
		decision.ShouldPlan = true
		decision.Reasons = append(decision.Reasons, "Contains planning signal")
	}

	return decision
}

// PlanDecision holds detailed plan decision analysis.
type PlanDecision struct {
	ShouldPlan bool
	Message    string
	Reasons    []string
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	negative := false
	if i < 0 {
		negative = true
		i = -i
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	if negative {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}
