package enforce_test

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce"
	"github.com/devrix/devrix/internal/layers/contextengine/persist/transcript"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

type stubSubTurn struct {
	text string
	err  error
}

func (s *stubSubTurn) RunSubTurn(_ context.Context, _ contracts.SubTurnRequest) (*contracts.SubTurnResult, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &contracts.SubTurnResult{AssistantText: s.text}, nil
}

// T: D2-S10-A01-T40
func TestSubQuery_should_filter_read_only_tools_and_set_agent_id(t *testing.T) {
	parent := &types.SessionContext{SessionID: "sess_sub", Model: "test", WorkDir: t.TempDir()}

	res, err := enforce.Run(context.Background(), enforce.SubQueryDeps{
		SubTurn: &stubSubTurn{text: "explored"},
	}, enforce.SubQueryParams{
		ParentSC:       parent,
		AgentID:        "explore_test",
		AgentName:      "Explore",
		SystemPrompt:   "explore",
		PromptMessages: []types.Message{{Role: types.MessageRoleUser, Content: "scan repo"}},
		Tools: []contracts.ToolSchema{
			{Name: "read_file"}, {Name: "write_file"}, {Name: "bash"},
		},
		ReadOnlyTools: true,
		MaxTurns:      3,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.ChildSC.AgentID != "explore_test" {
		t.Fatalf("expected agent id on child sc, got %q", res.ChildSC.AgentID)
	}
	if res.ChildSC.QueryDepth != 1 {
		t.Fatalf("expected depth 1, got %d", res.ChildSC.QueryDepth)
	}
	if res.Result == nil || res.Result.AssistantText != "explored" {
		t.Fatalf("unexpected result: %+v", res.Result)
	}
}

// T: D2-S10-A01-T42
func TestSubQuery_resume_should_load_sidechain_messages(t *testing.T) {
	store, err := transcript.NewSidechainStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	_ = store.Append("sess_resume", "plan_1", types.Message{Role: types.MessageRoleUser, Content: "prior turn"})

	parent := &types.SessionContext{SessionID: "sess_resume", Model: "test"}

	res, err := enforce.Run(context.Background(), enforce.SubQueryDeps{
		SubTurn:   &stubSubTurn{text: "plan ready"},
		Sidechain: store,
	}, enforce.SubQueryParams{
		ParentSC:       parent,
		AgentID:        "plan_1",
		AgentName:      "Plan",
		SystemPrompt:   "plan",
		PromptMessages: []types.Message{{Role: types.MessageRoleUser, Content: "continue"}},
		Resume:         true,
		MaxTurns:       2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Result.AssistantText != "plan ready" {
		t.Fatalf("unexpected assistant text: %q", res.Result.AssistantText)
	}
}
