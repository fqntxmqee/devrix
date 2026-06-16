package contextengine

import (
	stderrors "errors"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce"
	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/errors"
	"github.com/devrix/devrix/internal/shared/types"
)

// --- Pure function tests ---

func TestStripSystemMessage(t *testing.T) {
	tests := []struct {
		name string
		in   []types.Message
		want int
	}{
		{"empty", nil, 0},
		{"no system", []types.Message{{Role: "user", Content: "hi"}}, 1},
		{"system first", []types.Message{
			{Role: types.MessageRoleSystem, Content: "sys"},
			{Role: "user", Content: "hi"},
		}, 1},
		{"system only", []types.Message{
			{Role: types.MessageRoleSystem, Content: "sys"},
		}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripSystemMessage(tt.in)
			if len(got) != tt.want {
				t.Errorf("stripSystemMessage: got %d msgs, want %d", len(got), tt.want)
			}
			if tt.want > 0 && len(tt.in) > 0 && got[0].Role == types.MessageRoleSystem {
				t.Error("first message should not be system")
			}
		})
	}
}

func TestErrorEvent(t *testing.T) {
	ev := errorEvent("s1", errors.WithCode("CODE", "msg", nil), true)
	if ev.Type != "error" {
		t.Errorf("type = %s, want error", ev.Type)
	}
	if ev.SessionID != "s1" {
		t.Errorf("sessionID = %s, want s1", ev.SessionID)
	}
	if ev.Metadata["code"] != "CODE" {
		t.Errorf("code = %s, want CODE", ev.Metadata["code"])
	}
	if ev.Metadata["recoverable"] != "true" {
		t.Errorf("recoverable = %s, want true", ev.Metadata["recoverable"])
	}
}

func TestErrorEvent_NonRecoverable(t *testing.T) {
	ev := errorEvent("s2", errors.WithCode("E2", "err", nil), false)
	if ev.Metadata["recoverable"] != "false" {
		t.Errorf("recoverable = %s, want false", ev.Metadata["recoverable"])
	}
}

func TestMapProcessError_Nil(t *testing.T) {
	if ev := mapProcessError("s1", nil); ev != nil {
		t.Errorf("expected nil for nil error, got %v", ev)
	}
}

func TestMapProcessError_SentinelError(t *testing.T) {
	se := errors.WithCode("CTX_FAIL", "something broke", nil)
	ev := mapProcessError("s1", se)
	if ev == nil || ev.Type != "error" {
		t.Fatal("expected error event")
	}
	if ev.Metadata["code"] != "CTX_FAIL" {
		t.Errorf("code = %s, want CTX_FAIL", ev.Metadata["code"])
	}
}

func TestMapProcessError_PlainError(t *testing.T) {
	plain := stderrors.New("plain error")
	ev := mapProcessError("s1", plain)
	if ev == nil || ev.Type != "error" {
		t.Fatal("expected error event")
	}
	if ev.Metadata["code"] != "CTX_PROCESS_FAILED" {
		t.Errorf("code = %s, want CTX_PROCESS_FAILED", ev.Metadata["code"])
	}
}

func TestMapProcessError_WrappedSentinelError(t *testing.T) {
	se := errors.WithCode("WRAPPED", "inner", nil)
	ev := mapProcessError("s1", se)
	if ev == nil {
		t.Fatal("expected error event")
	}
	if ev.Metadata["code"] != "WRAPPED" {
		t.Errorf("code = %s, want WRAPPED", ev.Metadata["code"])
	}
}

func TestToolDescsToVisibleTools_Nil(t *testing.T) {
	tools := toolDescsToVisibleTools(nil)
	if len(tools) != 0 {
		t.Errorf("expected empty slice for nil input, got %d", len(tools))
	}
}

func TestFilterToolsByPermissionMode_NonPlanMode(t *testing.T) {
	tools := []ToolSchema{{Name: "bash"}, {Name: "read"}}
	got := enforce.FilterToolsByPermissionMode(types.PermissionDefault, tools, "")
	if len(got) != 2 {
		t.Errorf("non-plan mode should not filter, got %d", len(got))
	}
}

func TestFilterToolsByPermissionMode_PlanMode(t *testing.T) {
	tools := []ToolSchema{
		{Name: "bash"},
		{Name: "read"},
		{Name: "write"},
	}
	got := enforce.FilterToolsByPermissionMode(types.PermissionPlan, tools, "/tmp/plan.md")
	if len(got) == len(tools) {
		t.Error("plan mode should filter tools")
	}
}

func TestToolDescsToSchemas_Nil(t *testing.T) {
	got := toolDescsToSchemas(nil)
	if len(got) != 0 {
		t.Errorf("expected empty slice for nil input, got %d", len(got))
	}
}

func TestVisibleToolsToSchemas_Nil(t *testing.T) {
	got := visibleToolsToSchemas(nil)
	if len(got) != 0 {
		t.Errorf("expected empty slice for nil state, got %d", len(got))
	}
}

func TestInfoEvent(t *testing.T) {
	ev := infoEvent("sid", "content")
	if ev.Type != "info" {
		t.Errorf("type = %s, want info", ev.Type)
	}
	if ev.Content != "content" {
		t.Errorf("content = %s, want content", ev.Content)
	}
	if ev.SessionID != "sid" {
		t.Errorf("sessionID = %s, want sid", ev.SessionID)
	}
}

func TestConfigureLLMLogging(t *testing.T) {
	ConfigureLLMLogging(observability.LLMLogSettings{
		LogContent: true,
		LogDir:     "/tmp/logs",
	})
}

func TestLastAssistantContent(t *testing.T) {
	tests := []struct {
		name string
		msgs []types.Message
		want string
	}{
		{"empty", nil, ""},
		{"no assistant", []types.Message{{Role: "user", Content: "hi"}}, ""},
		{"assistant last", []types.Message{
			{Role: "user", Content: "hi"},
			{Role: types.MessageRoleAssistant, Content: "hello"},
		}, "hello"},
		{"assistant with tool call skipped", []types.Message{
			{Role: types.MessageRoleAssistant, Content: ""},
			{Role: types.MessageRoleAssistant, Content: "hello"},
		}, "hello"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := lastAssistantContent(tt.msgs)
			if got != tt.want {
				t.Errorf("lastAssistantContent = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCurrentLLMLogSettings(t *testing.T) {
	_ = currentLLMLogSettings()
}

func TestNoOpObserver(t *testing.T) {
	var obs NoOpObserver
	obs.EmitContextCompressed(types.CompressionReport{})
	obs.EmitSnapshotRestored("s", true)
	obs.EmitErrorOccurred("s", "E", nil)
}

var _ = contracts.EngineEvent{}
