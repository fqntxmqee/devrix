package materialize

import (
	"testing"

	"github.com/devrix/devrix/internal/shared/types"
)

func TestPolicyFromSubTurnMode(t *testing.T) {
	tests := []struct {
		mode string
		want Mode
	}{
		{"brief", ModeFresh},
		{"", ModeFresh},
		{"fork", ModeFork},
		{"full", ModeResume},
	}
	for _, tc := range tests {
		got := PolicyFromSubTurnMode(tc.mode, 8000)
		if got.Mode != tc.want {
			t.Errorf("PolicyFromSubTurnMode(%q).Mode = %q, want %q", tc.mode, got.Mode, tc.want)
		}
		if got.TokenBudget != 8000 {
			t.Errorf("PolicyFromSubTurnMode(%q).TokenBudget = %d, want 8000", tc.mode, got.TokenBudget)
		}
	}
}

func TestComposeSubTurnMessages_Brief(t *testing.T) {
	parent := []types.Message{
		{Role: types.MessageRoleUser, Content: "old"},
		{Role: types.MessageRoleAssistant, Content: "hi"},
		{Role: types.MessageRoleUser, Content: "directive"},
	}
	pre, last := ComposeSubTurnMessages(SubTurnBrief, parent)
	if pre != nil {
		t.Fatalf("brief preloaded = %v, want nil", pre)
	}
	if last.Content != "directive" {
		t.Fatalf("brief lastUser = %q, want directive", last.Content)
	}
}

func TestMaterializeSubTurn_FullParity(t *testing.T) {
	parent := []types.Message{
		{Role: types.MessageRoleUser, Content: "u1"},
		{Role: types.MessageRoleAssistant, Content: "a1"},
		{Role: types.MessageRoleUser, Content: "u2"},
	}
	m := NewDefaultMaterializer(nil, "")
	res, err := m.Materialize(t.Context(), Request{
		Partition: Partition{SessionID: "s1", Kind: PartitionAgent, AgentID: "agent1"},
		Policy:    PolicyFromSubTurnMode(SubTurnFull, 0),
		SubTurnParent: parent,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Messages) != 3 {
		t.Fatalf("messages = %d, want 3", len(res.Messages))
	}
	if res.Messages[2].Content != "u2" {
		t.Fatalf("last message = %q, want u2", res.Messages[2].Content)
	}
}
