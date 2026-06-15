// Package agent re-exports shared worker role prompts for D2 prepare assembly.
package agent

import sharedagent "github.com/devrix/devrix/internal/shared/prompts/agent"

var (
	ExplorePrompt   = sharedagent.ExplorePrompt
	PlanPrompt      = sharedagent.PlanPrompt
	ImplementPrompt = sharedagent.ImplementPrompt
)
