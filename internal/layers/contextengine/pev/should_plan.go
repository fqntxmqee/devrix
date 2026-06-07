package pev

import (
	"strings"

	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

// ShouldPlan reports whether the Plan phase should run for this message.
func ShouldPlan(cfg config.PlanConfig, state types.PEVState, message string) bool {
	if !cfg.Enabled {
		return false
	}
	if strings.HasPrefix(strings.TrimSpace(message), "/plan") {
		return true
	}
	if state.ActiveMilestoneID != "" || state.ActiveTaskID != "" {
		return false
	}
	minChars := cfg.MinCharsForPlan
	if minChars <= 0 {
		minChars = 200
	}
	return cfg.AutoDetect && len(message) >= minChars
}
