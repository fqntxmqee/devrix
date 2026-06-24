// Package agent holds embedded worker role prompts shared across domains.
package agent

import (
	_ "embed"
)

//go:embed explore.md
var ExplorePrompt string

//go:embed plan.md
var PlanPrompt string

//go:embed implement.md
var ImplementPrompt string

// SystemPromptForRole returns the worker prompt for the given role string.
// Unknown roles fall back to ImplementPrompt. Empty role also returns
// ImplementPrompt (the default worker).
func SystemPromptForRole(role string) string {
	switch role {
	case "explore", "Explore", "EXPLORE":
		return ExplorePrompt
	case "plan", "Plan", "PLAN":
		return PlanPrompt
	default:
		return ImplementPrompt
	}
}
