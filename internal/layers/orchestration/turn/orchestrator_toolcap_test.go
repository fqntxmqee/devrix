package turn

import (
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/prepare/persist"
	"github.com/devrix/devrix/internal/layers/llmgateway"
)

func TestOrchestrator_BuildToolResultMsgWithCap_BelowLimit(t *testing.T) {
	dir := t.TempDir()
	orch := NewOrchestrator(OrchestratorDeps{
		LLM:              &stubLLM{},
		Context:          &stubContext{},
		Tools:            &stubTools{},
		Persist:          &stubPersist{},
		ToolResultStore:  persist.NewToolResultStore(dir),
		MaxToolResultChars: 100,
	})

	msg := orch.buildToolResultMsgWithCap("s1",
		ToolResult{ToolCallID: "c1", Output: "small output"}, "read_file")

	if msg.Content != "small output" {
		t.Errorf("expected unchanged content for small output, got %q", msg.Content)
	}
	if msg.Role != "tool" {
		t.Errorf("expected role=tool, got %q", msg.Role)
	}
}

func TestOrchestrator_BuildToolResultMsgWithCap_AboveLimit_Persists(t *testing.T) {
	dir := t.TempDir()
	orch := NewOrchestrator(OrchestratorDeps{
		LLM:                 &stubLLM{},
		Context:             &stubContext{},
		Tools:               &stubTools{},
		Persist:             &stubPersist{},
		ToolResultStore:     persist.NewToolResultStore(dir),
		MaxToolResultChars:  100,
	})

	big := strings.Repeat("abcdefghij", 5000) // 50 000 bytes — > preview
	msg := orch.buildToolResultMsgWithCap("s1",
		ToolResult{ToolCallID: "c1", Output: big}, "read_file")

	if !strings.Contains(msg.Content, "<persisted-output>") {
		t.Errorf("expected persisted-output marker, got %q", msg.Content)
	}
	if !strings.Contains(msg.Content, "Output too large") {
		t.Errorf("expected size label, got %q", msg.Content)
	}
	if len(msg.Content) >= len(big) {
		t.Errorf("expected in-band content shorter than original (%d vs %d)",
			len(msg.Content), len(big))
	}
}

func TestOrchestrator_BuildToolResultMsgWithCap_NonCappedTool_PassesThrough(t *testing.T) {
	dir := t.TempDir()
	orch := NewOrchestrator(OrchestratorDeps{
		LLM:                 &stubLLM{},
		Context:             &stubContext{},
		Tools:               &stubTools{},
		Persist:             &stubPersist{},
		ToolResultStore:     persist.NewToolResultStore(dir),
		MaxToolResultChars:  100,
	})

	big := strings.Repeat("x", 1000)
	msg := orch.buildToolResultMsgWithCap("s1",
		ToolResult{ToolCallID: "c1", Output: big}, "task_create")

	if msg.Content != big {
		t.Errorf("expected unchanged content for non-capped tool, got truncated: %q", msg.Content)
	}
}

func TestOrchestrator_BuildToolResultMsgWithCap_NoStore(t *testing.T) {
	orch := NewOrchestrator(OrchestratorDeps{
		LLM:              &stubLLM{},
		Context:          &stubContext{},
		Tools:            &stubTools{},
		Persist:          &stubPersist{},
		ToolResultStore:  nil, // no store
		MaxToolResultChars: 100,
	})

	big := strings.Repeat("x", 1000)
	msg := orch.buildToolResultMsgWithCap("s1",
		ToolResult{ToolCallID: "c1", Output: big}, "read_file")

	if msg.Content != big {
		t.Errorf("expected unchanged content when no store, got %q", msg.Content)
	}
}

func TestOrchestrator_BuildToolResultMsgWithCap_ErrorSanitised(t *testing.T) {
	dir := t.TempDir()
	orch := NewOrchestrator(OrchestratorDeps{
		LLM:                 &stubLLM{},
		Context:             &stubContext{},
		Tools:               &stubTools{},
		Persist:             &stubPersist{},
		ToolResultStore:     persist.NewToolResultStore(dir),
		MaxToolResultChars:  50_000,
	})

	msg := orch.buildToolResultMsgWithCap("s1",
		ToolResult{
			ToolCallID: "c1",
			Output:     "stdout",
			Error:      "real error message",
		}, "read_file")

	if !strings.Contains(msg.Content, "real error message") {
		t.Errorf("expected error preserved, got %q", msg.Content)
	}
	if !strings.Contains(msg.Content, "stdout") {
		t.Errorf("expected stdout preserved, got %q", msg.Content)
	}
}

func TestOrchestrator_BuildAssistantToolCallMsgFolded_BelowLimit(t *testing.T) {
	dir := t.TempDir()
	orch := NewOrchestrator(OrchestratorDeps{
		LLM:               &stubLLM{},
		Context:           &stubContext{},
		Tools:             &stubTools{},
		Persist:           &stubPersist{},
		ToolResultStore:   persist.NewToolResultStore(dir),
		MaxAssistantChars: 8000,
	})

	calls := []llmgateway.ToolCall{{ID: "c1", Name: "read_file", Input: "{}"}}
	msg := orch.buildAssistantToolCallMsgFolded("s1", calls, "short reply", 1)
	if msg.Content != "short reply" {
		t.Errorf("expected unchanged short content, got %q", msg.Content)
	}
}

func TestOrchestrator_BuildAssistantToolCallMsgFolded_AboveLimit(t *testing.T) {
	dir := t.TempDir()
	orch := NewOrchestrator(OrchestratorDeps{
		LLM:               &stubLLM{},
		Context:           &stubContext{},
		Tools:             &stubTools{},
		Persist:           &stubPersist{},
		ToolResultStore:   persist.NewToolResultStore(dir),
		MaxAssistantChars: 200,
	})

	calls := []llmgateway.ToolCall{{ID: "c1", Name: "read_file", Input: "{}"}}
	long := strings.Repeat("abcdefghij", 1000) // 10000 bytes
	msg := orch.buildAssistantToolCallMsgFolded("s1", calls, long, 1)

	if !strings.Contains(msg.Content, "<prior-output-summary>") {
		t.Errorf("expected summary marker, got %q", msg.Content)
	}
	if !strings.Contains(msg.Content, "chars truncated") {
		t.Errorf("expected truncation marker, got %q", msg.Content)
	}
	if len(msg.Content) >= len(long) {
		t.Errorf("expected in-band content shorter than original (%d vs %d)",
			len(msg.Content), len(long))
	}
	// Tool call metadata must be preserved.
	if msg.Metadata["tool_calls"] == "" {
		t.Error("expected tool_calls metadata preserved")
	}
}

func TestOrchestrator_BuildAssistantToolCallMsgFolded_NoStore(t *testing.T) {
	orch := NewOrchestrator(OrchestratorDeps{
		LLM:               &stubLLM{},
		Context:           &stubContext{},
		Tools:             &stubTools{},
		Persist:           &stubPersist{},
		ToolResultStore:   nil,
		MaxAssistantChars: 100,
	})

	calls := []llmgateway.ToolCall{{ID: "c1", Name: "read_file", Input: "{}"}}
	long := strings.Repeat("x", 1000)
	msg := orch.buildAssistantToolCallMsgFolded("s1", calls, long, 1)
	if msg.Content != long {
		t.Errorf("expected unchanged content with no store, got %q", msg.Content)
	}
}