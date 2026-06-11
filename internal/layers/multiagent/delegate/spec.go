package delegate

import (
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

const exploreSystemPrompt = `You are the Explore worker agent. Investigate read-only.
Use read_file, glob, grep, list_dir, and bash (read-only) to gather facts.
Return concise findings; do not modify files.`

const planSystemPrompt = `You are the Plan worker agent. Produce an implementation plan.
Use read-only tools only. Output structured steps; do not edit source files.`

const implementSystemPrompt = `You are the Implement worker agent. Execute the assigned task.
Use available tools to implement the directive safely.`
