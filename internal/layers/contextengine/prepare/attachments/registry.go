package attachments

import (
	"context"
	"fmt"
	"strings"

	"github.com/devrix/devrix/internal/layers/contextengine/prepare/conversation"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

// Payload is a typed attachment before rendering to meta user messages.
type Payload struct {
	Type string
	Data any
}

// Provider collects attachments for one loop iteration.
type Provider interface {
	Collect(ctx context.Context, sc *types.SessionContext, msgs []types.Message, turnCount int) []Payload
}

// Registry composes attachment providers.
type Registry struct {
	cfg       config.AttachmentsConfig
	providers []Provider
}

// NewRegistry builds a registry with built-in providers.
func NewRegistry(cfg config.AttachmentsConfig) *Registry {
	if cfg.PlanModeFullEvery <= 0 {
		cfg.PlanModeFullEvery = 5
	}
	return &Registry{
		cfg: cfg,
		providers: []Provider{
			&PlanModeProvider{cfg: cfg},
		},
	}
}

// Collect aggregates payloads from all providers.
func (r *Registry) Collect(ctx context.Context, sc *types.SessionContext, msgs []types.Message, turnCount int) []Payload {
	if !r.cfg.Enabled || sc == nil {
		return nil
	}
	var out []Payload
	for _, p := range r.providers {
		out = append(out, p.Collect(ctx, sc, msgs, turnCount)...)
	}
	return out
}

// Render converts payloads to meta user messages appended to conversation state.
func Render(payloads []Payload) []types.Message {
	var out []types.Message
	for _, p := range payloads {
		switch p.Type {
		case "plan_mode":
			data, _ := p.Data.(PlanModeData)
			content := PlanModeInstructions(data)
			if content == "" {
				continue
			}
			out = append(out, metaMessage(content))
		case "plan_mode_exit":
			content := PlanModeExitInstructions()
			out = append(out, metaMessage(content))
		}
	}
	return out
}

func metaMessage(content string) types.Message {
	return conversation.PrependMetaUser(nil, content)[0]
}

// PlanModeData carries plan mode attachment fields.
type PlanModeData struct {
	ReminderType string
	PlanFilePath string
	PlanExists   bool
	IsSubAgent   bool
}

// PlanModeProvider injects plan mode workflow reminders.
type PlanModeProvider struct {
	cfg config.AttachmentsConfig
}

func (p *PlanModeProvider) Collect(_ context.Context, sc *types.SessionContext, _ []types.Message, turnCount int) []Payload {
	if sc == nil || !sc.PermissionMode.IsPlanMode() || sc.AgentID != "" {
		return nil
	}
	reminder := "sparse"
	if turnCount == 0 || turnCount%p.cfg.PlanModeFullEvery == 0 {
		reminder = "full"
	}
	return []Payload{{
		Type: "plan_mode",
		Data: PlanModeData{
			ReminderType: reminder,
			PlanFilePath: sc.PlanFilePath,
			PlanExists:   sc.PlanFilePath != "",
			IsSubAgent:   sc.AgentID != "",
		},
	}}
}

// PlanModeInstructions renders plan mode workflow text (CC getPlanModeV2 aligned, condensed).
func PlanModeInstructions(d PlanModeData) string {
	if d.IsSubAgent {
		return planModeSubAgentInstructions(d)
	}
	if d.ReminderType == "sparse" {
		return fmt.Sprintf(`Plan mode is active. Continue researching and update the plan file at %s when ready. Do not make edits outside the plan file.`, d.PlanFilePath)
	}
	planFileHint := fmt.Sprintf("Create or edit your plan at %s using write_file.", d.PlanFilePath)
	if d.PlanExists {
		planFileHint = fmt.Sprintf("A plan file exists at %s — read and edit it incrementally.", d.PlanFilePath)
	}
	return strings.TrimSpace(fmt.Sprintf(`Plan mode is active. You MUST NOT execute non-readonly changes until plan approval.

## Plan File
%s

## Workflow
1. Explore — use read-only tools and Explore sub-agents to understand the codebase.
2. Design — use Plan sub-agents for implementation strategy.
3. Review — read critical files and clarify with the user if needed.
4. Final Plan — write concise plan to the plan file only (paths, reuse points, verification command).
5. Exit — call exit_plan_mode to request approval before implementation.`, planFileHint))
}

func planModeSubAgentInstructions(d PlanModeData) string {
	return fmt.Sprintf("Plan mode is active in the parent session. You are a sub-agent: stay read-only. Plan file: %s", d.PlanFilePath)
}

// PlanModeExitInstructions renders one-shot exit notice.
func PlanModeExitInstructions() string {
	return "## Exited Plan Mode\n\nYou have exited plan mode. You may now make edits and run tools."
}
