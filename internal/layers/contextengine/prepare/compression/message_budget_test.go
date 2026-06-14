package compression

import (
	"context"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

func TestPipeline_should_apply_message_budget_when_over_max_messages(t *testing.T) {
	p := NewPipeline(
		WithEnabled(true),
		WithMessageBudget(6, 3, 1),
	)
	msgs := []types.Message{
		{Role: types.MessageRoleUser, Content: "task"},
	}
	for i := range 8 {
		id := string(rune('a' + i))
		msgs = append(msgs,
			types.Message{Role: types.MessageRoleAssistant, Metadata: map[string]string{"tool_calls": `[{"id":"` + id + `"}]`}},
			types.Message{Role: types.MessageRoleTool, Content: strings.Repeat("x", 20), Metadata: map[string]string{"tool_call_id": id}},
		)
	}

	out, report, err := p.RunForSession(context.Background(), "sess", msgs, "", types.DefaultTokenBudget())
	if err != nil {
		t.Fatalf("RunForSession: %v", err)
	}
	if len(out) > 6 {
		t.Fatalf("len = %d, want <= 6", len(out))
	}
	if out[0].Content != "task" {
		t.Fatalf("head user lost: %+v", out[0])
	}
	found := false
	for _, step := range report.StepsApplied {
		if step == stepMessageBudget {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected message_budget step, got %v", report.StepsApplied)
	}
}

func TestPipeline_should_clear_stale_tool_results_each_run(t *testing.T) {
	p := NewPipeline(
		WithEnabled(true),
		WithMicrocompactConfig(config.MicrocompactConfig{KeepRecentToolResults: 1}),
	)
	msgs := []types.Message{
		{Role: types.MessageRoleAssistant, Metadata: map[string]string{"tool_calls": `[{"id":"a"}]`}},
		{Role: types.MessageRoleTool, Content: "old", Metadata: map[string]string{"tool_call_id": "a"}},
		{Role: types.MessageRoleAssistant, Metadata: map[string]string{"tool_calls": `[{"id":"b"}]`}},
		{Role: types.MessageRoleTool, Content: "new", Metadata: map[string]string{"tool_call_id": "b"}},
	}

	out, report, err := p.RunForSession(context.Background(), "sess", msgs, "", types.DefaultTokenBudget())
	if err != nil {
		t.Fatalf("RunForSession: %v", err)
	}
	if out[1].Content != clearedToolResultContent {
		t.Fatalf("first tool = %q, want cleared", out[1].Content)
	}
	if out[3].Content != "new" {
		t.Fatalf("recent tool = %q, want preserved", out[3].Content)
	}
	found := false
	for _, step := range report.StepsApplied {
		if step == stepClearToolResults {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected clear_tool_results step, got %v", report.StepsApplied)
	}
}
