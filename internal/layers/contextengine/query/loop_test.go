package query_test

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/attachments"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/conversation"
	"github.com/devrix/devrix/internal/layers/contextengine/query"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

// T: D2-S10-A01-T34
func TestQueryLoop_should_continue_until_no_tool_use(t *testing.T) {
	llm := &query.SequentialLLM{Responses: []query.LLMScript{
		{ToolCalls: []query.ToolCall{{ID: "c1", Name: "bash", Input: "ls"}}},
		{ToolCalls: []query.ToolCall{{ID: "c2", Name: "bash", Input: "pwd"}}},
		{ToolCalls: []query.ToolCall{{ID: "c3", Name: "bash", Input: "whoami"}}},
		{Content: "done"},
	}}
	tools := &query.RecordingToolExecutor{}
	loop := &query.Loop{LLM: llm, Tools: tools, Permission: query.AllowPermission{}}

	sc := &types.SessionContext{SessionID: "s1", Model: "test"}
	res, err := loop.Run(context.Background(), sc, query.Params{
		SystemPrompt: "sys",
		Messages:     []types.Message{{Role: types.MessageRoleUser, Content: "go"}},
		MaxTurns:     0,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if llm.Calls != 4 {
		t.Fatalf("expected 4 LLM calls, got %d", llm.Calls)
	}
	if len(tools.Calls) != 3 {
		t.Fatalf("expected 3 tool execs, got %d", len(tools.Calls))
	}
	if res.TurnCount != 3 {
		t.Fatalf("expected turnCount 3, got %d", res.TurnCount)
	}
	if len(res.ToolCallHistory) != 3 {
		t.Fatalf("expected 3 tool records, got %d", len(res.ToolCallHistory))
	}
}

// T: D2-S10-A01-T35
func TestUserContextPrepend_should_not_persist_in_snapshot_messages(t *testing.T) {
	msgs := []types.Message{{Role: types.MessageRoleUser, Content: "hi"}}
	prepend := conversation.PrependMetaUser(msgs, "<system-reminder>agents</system-reminder>")
	if len(prepend) != 2 {
		t.Fatalf("expected 2 api messages, got %d", len(prepend))
	}
	if conversation.HasMetaUserContext(msgs) {
		t.Fatal("original snapshot messages must not contain prepend block")
	}
	if !conversation.HasMetaUserContext(prepend) {
		t.Fatal("api messages should contain prepend block")
	}
}

// T: D2-S10-A01-T36
func TestPlanModeAttachment_should_alternate_full_and_sparse(t *testing.T) {
	reg := attachments.NewRegistry(config.AttachmentsConfig{Enabled: true, PlanModeFullEvery: 5})
	sc := &types.SessionContext{SessionID: "s1", PermissionMode: types.PermissionPlan, PlanFilePath: "/tmp/plan.md"}

	full := reg.Collect(context.Background(), sc, nil, 0)
	if len(full) != 1 {
		t.Fatalf("expected 1 attachment at turn 0")
	}
	data := full[0].Data.(attachments.PlanModeData)
	if data.ReminderType != "full" {
		t.Fatalf("turn 0 want full, got %s", data.ReminderType)
	}
	sparse := reg.Collect(context.Background(), sc, nil, 1)
	data2 := sparse[0].Data.(attachments.PlanModeData)
	if data2.ReminderType != "sparse" {
		t.Fatalf("turn 1 want sparse, got %s", data2.ReminderType)
	}
}
