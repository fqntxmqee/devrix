package contextengine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/shared/types"
)

func TestFormatMessages_should_include_roles_and_truncate(t *testing.T) {
	msgs := formatMessages([]types.Message{
		{Role: types.MessageRoleUser, Content: strings.Repeat("a", 600)},
		{
			Role:     types.MessageRoleAssistant,
			Metadata: map[string]string{"tool_calls": `[{"id":"tc1"}]`},
		},
	}, 500)
	if len(msgs) != 2 {
		t.Fatalf("len = %d", len(msgs))
	}
	if msgs[0].Role != "user" || !strings.HasSuffix(msgs[0].Content, "...") {
		t.Fatalf("user msg: %+v", msgs[0])
	}
	if !strings.Contains(msgs[1].Content, "tool_calls") {
		t.Fatalf("assistant msg: %+v", msgs[1])
	}
}

func TestFormatToolCalls_should_include_errors(t *testing.T) {
	info := formatToolCalls(
		[]ToolCall{{Name: "call_cursor", Input: `{"task":"hi"}`}},
		[]ToolResult{{Error: "chdir failed"}},
		defaultToolTruncate,
	)
	if len(info) != 1 || info[0].Error != "chdir failed" {
		t.Fatalf("info: %+v", info)
	}
}

func TestConfigureLLMLogging_should_expand_home_dir(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home dir")
	}
	observability.ConfigureLLMLogging(observability.LLMLogSettings{LogDir: "~/custom-llm-logs"})
	settings := observability.CurrentLLMLogSettings()
	want := filepath.Join(home, "custom-llm-logs")
	if settings.LogDir != want {
		t.Fatalf("LogDir = %q, want %q", settings.LogDir, want)
	}
}

func TestAppendLLMLogFile_should_write_full_jsonl(t *testing.T) {
	dir := t.TempDir()
	observability.ConfigureLLMLogging(observability.LLMLogSettings{LogContent: true, LogDir: dir})

	info := LLMCallInfo{
		Iteration:    0,
		Model:        "test-model",
		SystemPrompt: strings.Repeat("s", 1200),
		Messages:     []MsgInfo{{Role: "user", Content: strings.Repeat("u", 1200)}},
	}
	observability.AppendLLMLogFile(dir, "sess/1", "request", 0, "test-model", info)

	path := filepath.Join(dir, "sess_1.jsonl")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if !strings.Contains(string(raw), strings.Repeat("s", 1200)) {
		t.Fatal("expected full system prompt in log file")
	}
	if !strings.Contains(string(raw), strings.Repeat("u", 1200)) {
		t.Fatal("expected full user message in log file")
	}
}

func TestFormatMessages_should_not_truncate_when_limit_zero(t *testing.T) {
	long := strings.Repeat("x", 800)
	msgs := formatMessages([]types.Message{{Role: types.MessageRoleUser, Content: long}}, 0)
	if msgs[0].Content != long {
		t.Fatalf("content len = %d", len(msgs[0].Content))
	}
}
