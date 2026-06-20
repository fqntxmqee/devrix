package turn

import (
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/prepare/persist"
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