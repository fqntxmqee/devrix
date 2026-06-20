package audit

import (
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/prepare/token"
	"github.com/devrix/devrix/internal/shared/types"
)

func TestAuditMessages_BasicCounts(t *testing.T) {
	c := token.NewCounter()
	msgs := []types.Message{
		{Role: types.MessageRoleUser, Content: "hello"},         // 5 chars → 1 tok
		{Role: types.MessageRoleAssistant, Content: "world"},     // 5 chars → 1 tok
		{Role: types.MessageRoleTool, Content: "out"},           // 3 chars → 1 tok
	}

	got := AuditMessages(c, "system", msgs, 0)

	if got.SystemTokens != 1 {
		t.Errorf("systemTokens: got %d, want 1", got.SystemTokens)
	}
	if got.MessagesTokens != 3 {
		t.Errorf("messagesTokens: got %d, want 3", got.MessagesTokens)
	}
	if got.TotalTokens != 4 {
		t.Errorf("totalTokens: got %d, want 4", got.TotalTokens)
	}
}

func TestAuditMessages_OverBudgetAndPercent(t *testing.T) {
	c := token.NewCounter()
	msgs := []types.Message{
		{Role: types.MessageRoleUser, Content: strings.Repeat("x", 400)}, // 100 tok
	}

	got := AuditMessages(c, "", msgs, 50)

	if !got.OverBudget {
		t.Error("expected OverBudget=true")
	}
	if got.BudgetPercent < 1.9 || got.BudgetPercent > 2.1 {
		t.Errorf("BudgetPercent: got %f, want ~2.0", got.BudgetPercent)
	}
	if got.LargestMsgTokens != 100 {
		t.Errorf("LargestMsgTokens: got %d, want 100", got.LargestMsgTokens)
	}
	if got.LargestMsgIdx != 0 {
		t.Errorf("LargestMsgIdx: got %d, want 0", got.LargestMsgIdx)
	}
}

func TestAuditMessages_NoBudget(t *testing.T) {
	c := token.NewCounter()
	msgs := []types.Message{{Role: types.MessageRoleUser, Content: "x"}}

	got := AuditMessages(c, "", msgs, 0)

	if got.BudgetPercent != 0 {
		t.Errorf("BudgetPercent with budget=0: got %f, want 0", got.BudgetPercent)
	}
	if got.OverBudget {
		t.Error("OverBudget with budget=0: got true, want false")
	}
}

func TestAuditMessages_NilCounter(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("nil counter should not panic, got %v", r)
		}
	}()
	got := AuditMessages(nil, "sys", nil, 100)
	if got.TotalTokens != 1 {
		t.Errorf("expected system tokens even with nil counter, got %d", got.TotalTokens)
	}
}

func TestShouldFoldProactively(t *testing.T) {
	c := token.NewCounter()

	tests := []struct {
		name              string
		msgs              []types.Message
		budget            int
		maxAssistantChars int
		proactivePercent  float64
		want              bool
	}{
		{
			name:              "over budget folds",
			msgs:              []types.Message{{Role: types.MessageRoleAssistant, Content: strings.Repeat("x", 4000)}},
			budget:            50,
			maxAssistantChars: 1000,
			want:              true,
		},
		{
			name:              "below proactive threshold",
			msgs:              []types.Message{{Role: types.MessageRoleUser, Content: strings.Repeat("x", 100)}},
			budget:            1000,
			maxAssistantChars: 1000,
			want:              false,
		},
		{
			name:              "above proactive threshold",
			msgs:              []types.Message{{Role: types.MessageRoleAssistant, Content: strings.Repeat("x", 4000)}},
			budget:            1000, // ~250 tokens → >60% of budget
			maxAssistantChars: 1000,
			proactivePercent:  0.6,
			want:              true,
		},
		{
			name:              "largest msg under cap → no fold",
			msgs:              []types.Message{{Role: types.MessageRoleAssistant, Content: strings.Repeat("x", 200)}},
			budget:            100,
			maxAssistantChars: 1000,
			want:              false,
		},
		{
			name:              "maxAssistantChars=0 disables fold",
			msgs:              []types.Message{{Role: types.MessageRoleAssistant, Content: strings.Repeat("x", 4000)}},
			budget:            100,
			maxAssistantChars: 0,
			want:              false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := AuditMessages(c, "", tt.msgs, tt.budget)
			got := ShouldFoldProactively(r, tt.maxAssistantChars, tt.proactivePercent)
			if got != tt.want {
				t.Errorf("ShouldFoldProactively: got %v, want %v", got, tt.want)
			}
		})
	}
}