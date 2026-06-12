package delegate

import (
	"github.com/devrix/devrix/internal/layers/contextengine/prompt/agent"
	"github.com/devrix/devrix/internal/shared/types"
)

// WorkerRole identifies the delegated worker specialization.
type WorkerRole string

const (
	WorkerRoleExplore   WorkerRole = "explore"
	WorkerRolePlan      WorkerRole = "plan"
	WorkerRoleImplement WorkerRole = "implement"
)

// WorkerSpec configures a D4 delegated worker run.
type WorkerSpec struct {
	Role         WorkerRole
	Directive    string
	TaskID       string
	WorktreeSlug string
	MaxTurns     int
	Async        bool
}

// DelegateResult is returned after a synchronous delegate completes.
type DelegateResult struct {
	WorkerID string
	Role     WorkerRole
	Summary  string
	Messages []types.Message
	Error    error
}

// SystemPromptForRole returns the worker system prompt for a role.
func SystemPromptForRole(role WorkerRole) string {
	switch role {
	case WorkerRoleExplore:
		return exploreSystemPrompt
	case WorkerRolePlan:
		return planSystemPrompt
	default:
		return implementSystemPrompt
	}
}

// exploreSystemPrompt is the structured prompt for Explore worker agents,
// loaded from the embedded prompts/explore.md.
var exploreSystemPrompt = agent.ExplorePrompt

// planSystemPrompt is the structured prompt for Plan worker agents,
// loaded from the embedded prompts/plan.md.
var planSystemPrompt = agent.PlanPrompt

// implementSystemPrompt is the structured prompt for Implement worker agents,
// loaded from the embedded prompts/implement.md.
var implementSystemPrompt = agent.ImplementPrompt
