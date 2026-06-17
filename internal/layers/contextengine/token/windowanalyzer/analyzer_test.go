package windowanalyzer

import (
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/shared/types"
)

// TestAnalyze_BasicBreakdown — 各 Category 独立计数。
func TestAnalyze_BasicBreakdown(t *testing.T) {
	a := NewTokenAnalyzer()
	hist := []MessageView{
		{Role: "system", Content: "x", Category: CatSystem},
		{Role: "user", Content: "hello", Category: CatMessages},
		{Role: "tool", Content: "result", Category: CatTools},
	}
	b := a.Analyze(hist)
	if b.System == 0 || b.Messages == 0 || b.Tools == 0 {
		t.Errorf("expected non-zero per-category, got %+v", b)
	}
	if b.Total == 0 {
		t.Error("Total should be > 0")
	}
	if b.System+b.Tools+b.Messages != b.Total {
		t.Errorf("Total mismatch: %+v", b)
	}
}

// TestAnalyze_EmptyHistory — 空 history → 全 0。
func TestAnalyze_EmptyHistory(t *testing.T) {
	a := NewTokenAnalyzer()
	b := a.Analyze(nil)
	if b.Total != 0 {
		t.Errorf("expected 0 total, got %d", b.Total)
	}
}

// TestAnalyze_UnknownCategory_DefaultsToMessages — 未知 category 归入 messages。
func TestAnalyze_UnknownCategory_DefaultsToMessages(t *testing.T) {
	a := NewTokenAnalyzer()
	hist := []MessageView{{Content: "abc", Category: Category("alien")}}
	b := a.Analyze(hist)
	if b.Messages == 0 {
		t.Errorf("unknown category should map to messages, got %+v", b)
	}
}

// TestAnalyzeMessages_RoleRouting — role-based 分类。
func TestAnalyzeMessages_RoleRouting(t *testing.T) {
	a := NewTokenAnalyzer()
	msgs := []types.Message{
		{Role: types.MessageRoleSystem, Content: "you are helpful"},
		{Role: types.MessageRoleUser, Content: "hi"},
		{Role: types.MessageRoleTool, Content: "tool output"},
	}
	b := a.AnalyzeMessages(msgs)
	if b.System == 0 || b.Messages == 0 || b.Tools == 0 {
		t.Errorf("expected non-zero per role, got %+v", b)
	}
}

// TestAnalyzeMessages_ThinkingMarker — content 含 <thinking> 归入 Thinking。
func TestAnalyzeMessages_ThinkingMarker(t *testing.T) {
	a := NewTokenAnalyzer()
	msgs := []types.Message{
		{Role: types.MessageRoleAssistant, Content: "<thinking>let me consider</thinking>"},
	}
	b := a.AnalyzeMessages(msgs)
	if b.Thinking == 0 {
		t.Errorf("expected thinking > 0, got %+v", b)
	}
	if b.Messages != 0 {
		t.Errorf("thinking should not bleed to messages: %+v", b)
	}
}

// TestAnalyzeMessages_ReminderMarker — content 含 <reminder> 归入 Reminders。
func TestAnalyzeMessages_ReminderMarker(t *testing.T) {
	a := NewTokenAnalyzer()
	msgs := []types.Message{
		{Role: types.MessageRoleSystem, Content: "<reminder>this is a reminder</reminder>"},
	}
	b := a.AnalyzeMessages(msgs)
	if b.Reminders == 0 {
		t.Errorf("expected reminders > 0, got %+v", b)
	}
}

// TestAnalyzeMessages_TaskNotificationMarker — <task_notifications> 归入 Reminders。
func TestAnalyzeMessages_TaskNotificationMarker(t *testing.T) {
	a := NewTokenAnalyzer()
	msgs := []types.Message{
		{Role: types.MessageRoleSystem, Content: "<task_notifications>x</task_notifications>"},
	}
	b := a.AnalyzeMessages(msgs)
	if b.Reminders == 0 {
		t.Errorf("expected reminders > 0, got %+v", b)
	}
}

// TestFormatTable_ContainsAllCategories — 渲染含全部 category + total。
func TestFormatTable_ContainsAllCategories(t *testing.T) {
	b := Breakdown{System: 100, Tools: 50, Messages: 200, Thinking: 10, Reminders: 5, Total: 365}
	out := FormatTable(b)
	for _, want := range []string{"system", "messages", "tools", "thinking", "reminders", "total", "365", "100.0%"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull:\n%s", want, out)
		}
	}
}

// TestFormatTable_EmptyBreakdown — 全零 breakdown 也能渲染。
func TestFormatTable_EmptyBreakdown(t *testing.T) {
	b := Breakdown{}
	out := FormatTable(b)
	if !strings.Contains(out, "total") {
		t.Errorf("output missing total, got:\n%s", out)
	}
}

// TestAnalyzer_InterfaceConformance — 满足 Analyzer 接口。
func TestAnalyzer_InterfaceConformance(t *testing.T) {
	var _ Analyzer = NewTokenAnalyzer()
}
