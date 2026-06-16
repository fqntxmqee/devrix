package workmodel

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/observability/instrument/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/instrument/tracer"
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
	obsBridge  *observability.Bridge
}

// NewPlanMode creates a new plan mode manager.
func NewPlanMode(llm LLMCompleter, obsBridge *observability.Bridge) *PlanMode {
	return &PlanMode{
		state:     PlanModeInactive,
		planAgent: NewPlanAgent(llm, obsBridge),
		obsBridge: obsBridge,
	}
}

// startSpan creates a child span for PlanMode operations.
func (p *PlanMode) startSpan(ctx context.Context, operation string, kind tracer.SpanKind, attrs ...tracer.Attribute) (context.Context, tracer.Span) {
	if p.obsBridge == nil || !p.obsBridge.IsEnabled() {
		return ctx, nil
	}
	opts := []tracer.SpanStartOption{
		tracer.WithSpanKind(kind),
		tracer.WithSpanAttributes(telemetry.SpanAttrs(operation, attrs...)...),
	}
	if parentSC := tracer.SpanContextFromContext(ctx); parentSC != nil {
		opts = append(opts, tracer.WithParent(*parentSC))
	}
	return p.obsBridge.Tracer().Start(ctx, operation, opts...)
}

// IsActive returns true if plan mode is active.
func (p *PlanMode) IsActive() bool {
	return p.state == PlanModeActive || p.state == PlanModePending
}

// Enter enters plan mode for a goal.
func (p *PlanMode) Enter(ctx context.Context, sessionID, userGoal string) error {
	_, span := p.startSpan(ctx, telemetry.OpD2_S8_Task_PlanMode_Enter, tracer.SpanKindInternal,
		tracer.Attribute{Key: "plan_mode.state", Value: string(PlanModeActive)},
	)

	if p.planAgent == nil || !p.planAgent.HasLLM() {
		if span != nil {
			span.End()
		}
		return ErrLLMNotConfigured
	}

	p.sessionID = sessionID
	p.userGoal = userGoal
	p.state = PlanModeActive

	if span != nil {
		span.End()
	}
	return nil
}

// Execute runs the plan agent to explore and plan.
func (p *PlanMode) Execute(ctx context.Context, workDir string, tools []string) error {
	start := time.Now()
	_, span := p.startSpan(ctx, telemetry.OpD2_S8_Task_PlanMode_Execute, tracer.SpanKindInternal,
		tracer.Attribute{Key: "plan_mode.state", Value: string(p.state)},
		tracer.Attribute{Key: "plan_mode.tool_count", Value: fmt.Sprintf("%d", len(tools))},
	)

	if !p.IsActive() {
		if span != nil {
			span.End()
		}
		return nil
	}

	req := PlanRequest{
		UserGoal: p.userGoal,
		WorkDir:  workDir,
		Tools:    tools,
	}

	result := p.planAgent.Plan(ctx, req)
	if result.Err != nil {
		if span != nil {
			span.RecordError(result.Err)
			span.End()
		}
		return result.Err
	}

	p.planResult = result
	p.state = PlanModePending

	if span != nil {
		span.SetAttributes(
			tracer.Attribute{Key: "plan_mode.result_tasks", Value: fmt.Sprintf("%d", len(result.Tasks))},
			tracer.Attribute{Key: "plan_mode.duration_ms", Value: fmt.Sprintf("%d", time.Since(start).Milliseconds())},
		)
		span.End()
	}
	return nil
}

// GetPlan returns the current plan result.
func (p *PlanMode) GetPlan() *PlanResult {
	return p.planResult
}

// Approve approves the plan and returns tasks.
func (p *PlanMode) Approve() []*Task {
	_, span := p.startSpan(context.Background(), telemetry.OpD2_S8_Task_PlanMode_Approve, tracer.SpanKindInternal,
		tracer.Attribute{Key: "plan_mode.task_count", Value: fmt.Sprintf("%d", len(p.planResult.Tasks))},
	)
	if p.planResult == nil {
		if span != nil {
			span.End()
		}
		return nil
	}
	tasks := p.planResult.Tasks
	if span != nil {
		span.End()
	}
	return tasks
}

// Reject rejects the plan.
func (p *PlanMode) Reject() {
	_, span := p.startSpan(context.Background(), telemetry.OpD2_S8_Task_PlanMode_Reject, tracer.SpanKindInternal)
	p.state = PlanModeInactive
	p.planResult = nil
	if span != nil {
		span.End()
	}
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
