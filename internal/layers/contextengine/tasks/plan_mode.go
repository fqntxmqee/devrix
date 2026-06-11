package tasks

import (
	"context"
	"fmt"
	"strings"
)

// PlanModeState represents the current plan mode state.
type PlanModeState string

const (
	PlanModeInactive PlanModeState = "inactive"
	PlanModeActive  PlanModeState = "active"
	PlanModePending PlanModeState = "pending_approval"
)

// PlanMode represents the plan mode manager.
type PlanMode struct {
	state      PlanModeState
	sessionID  string
	userGoal   string
	planResult *PlanResult
	planAgent  *PlanAgent
}

// NewPlanMode creates a new plan mode manager.
func NewPlanMode(llm LLMCompleter) *PlanMode {
	return &PlanMode{
		state:     PlanModeInactive,
		planAgent: NewPlanAgent(llm),
	}
}

// IsActive returns true if plan mode is active.
func (p *PlanMode) IsActive() bool {
	return p.state == PlanModeActive || p.state == PlanModePending
}

// Enter enters plan mode for a goal.
func (p *PlanMode) Enter(ctx context.Context, sessionID, userGoal string) error {
	if p.planAgent == nil {
		return ErrLLMNotConfigured
	}

	p.sessionID = sessionID
	p.userGoal = userGoal
	p.state = PlanModeActive

	return nil
}

// Execute runs the plan agent to explore and plan.
func (p *PlanMode) Execute(ctx context.Context, workDir string, tools []string) error {
	if !p.IsActive() {
		return nil
	}

	req := PlanRequest{
		UserGoal: p.userGoal,
		WorkDir:  workDir,
		Tools:    tools,
	}

	result := p.planAgent.Plan(ctx, req)
	if result.Err != nil {
		return result.Err
	}

	p.planResult = result
	p.state = PlanModePending

	return nil
}

// GetPlan returns the current plan result.
func (p *PlanMode) GetPlan() *PlanResult {
	return p.planResult
}

// Approve approves the plan and returns tasks.
func (p *PlanMode) Approve() []*Task {
	if p.planResult == nil {
		return nil
	}
	return p.planResult.Tasks
}

// Reject rejects the plan.
func (p *PlanMode) Reject() {
	p.state = PlanModeInactive
	p.planResult = nil
}

// Exit exits plan mode.
func (p *PlanMode) Exit() {
	p.state = PlanModeInactive
	p.planResult = nil
	p.sessionID = ""
	p.userGoal = ""
}

// GetState returns the current state.
func (p *PlanMode) GetState() PlanModeState {
	return p.state
}

// GetDisplayPlan returns a formatted plan for display.
func (p *PlanMode) GetDisplayPlan() string {
	if p.planResult == nil {
		return "No plan available"
	}

	var b strings.Builder

	b.WriteString("# Implementation Plan\n\n")
	b.WriteString("## Exploration Findings\n\n")
	b.WriteString(p.planResult.Exploration)
	b.WriteString("\n\n## Tasks\n\n")

	for i, task := range p.planResult.Tasks {
		b.WriteString(fmt.Sprintf("### %d. %s\n\n", i+1, task.Subject))
		b.WriteString(task.Description + "\n\n")
	}

	if len(p.planResult.CriticalFiles) > 0 {
		b.WriteString("## Critical Files\n\n")
		for _, f := range p.planResult.CriticalFiles {
			b.WriteString("- " + f + "\n")
		}
	}

	b.WriteString("\n---\n")
	b.WriteString("Use `/plan approve` to proceed or `/plan reject` to cancel.\n")

	return b.String()
}
