package renderers

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("copy stdout: %v", err)
	}
	return buf.String()
}

func testANSI() config.ANSIConfig {
	return config.ANSIConfig{
		User:      "[USER]",
		Assistant: "[ASST]",
		Error:     "[ERR]",
		Warning:   "[WARN]",
		Reset:     "[RST]",
	}
}

func TestCLIRenderer_should_render_user_and_assistant_messages(t *testing.T) {
	r := NewCLIRenderer(testANSI())

	out := captureStdout(t, func() {
		r.RenderMessage(&types.OutboundMessage{Role: types.MessageRoleUser, Content: "hello"})
		r.RenderMessage(&types.OutboundMessage{Role: types.MessageRoleAssistant, Content: "world"})
		r.RenderMessage(&types.OutboundMessage{Role: types.MessageRoleSystem, Content: "system"})
	})

	if !strings.Contains(out, "[USER]hello[RST]") {
		t.Fatalf("expected user message in output, got %q", out)
	}
	if !strings.Contains(out, "[ASST]world[RST]") {
		t.Fatalf("expected assistant message in output, got %q", out)
	}
	if !strings.Contains(out, "system\n") {
		t.Fatalf("expected plain system message, got %q", out)
	}
}

func TestCLIRenderer_should_render_streaming_status_and_error(t *testing.T) {
	r := NewCLIRenderer(testANSI())

	out := captureStdout(t, func() {
		r.RenderStreamingMessage("partial", false)
		r.RenderStreamingMessage("done", true)
		r.RenderError(errors.New("boom"))
		r.RenderStatus(types.SessionStateThinking)
		r.RenderStatus(types.SessionStateCompleted)
		r.RenderStatus(types.SessionStateFailed)
		r.RenderStatus(types.SessionStateIdle)
	})

	if !strings.Contains(out, "partial") || !strings.Contains(out, "done") {
		t.Fatalf("expected streaming output, got %q", out)
	}
	if !strings.Contains(out, "[ERR]Error: boom[RST]") {
		t.Fatalf("expected error output, got %q", out)
	}
	if !strings.Contains(out, "[Thinking...]") || !strings.Contains(out, "[Done]") || !strings.Contains(out, "[Failed]") {
		t.Fatalf("expected status output, got %q", out)
	}
}

func TestCLIRenderer_should_render_permission_tool_and_complete(t *testing.T) {
	r := NewCLIRenderer(testANSI())

	out := captureStdout(t, func() {
		r.RenderPermissionRequest(&types.PermissionRequest{
			ToolName:     "bash",
			Description:  "run command",
			InputPreview: "ls\n-la",
			RiskLevel:    types.RiskLevelCritical,
		})
		r.RenderToolCall("grep", "pattern\nline2")
		r.RenderToolResult("ok", nil)
		r.RenderToolResult("", errors.New("fail"))
		r.RenderComplete(map[string]int{"tokens": 42})
	})

	if !strings.Contains(out, "Permission Required") || !strings.Contains(out, "bash") {
		t.Fatalf("expected permission card, got %q", out)
	}
	if !strings.Contains(out, "grep") || !strings.Contains(out, "Success") || !strings.Contains(out, "fail") {
		t.Fatalf("expected tool output, got %q", out)
	}
	if !strings.Contains(out, "tokens: 42") {
		t.Fatalf("expected usage output, got %q", out)
	}
}

func TestCLIRenderer_getRiskColor_should_map_levels(t *testing.T) {
	r := NewCLIRenderer(testANSI())

	if got := r.getRiskColor(types.RiskLevelCritical); got != "[ERR]" {
		t.Fatalf("critical: got %q", got)
	}
	if got := r.getRiskColor(types.RiskLevelHigh); got != "[WARN]" {
		t.Fatalf("high: got %q", got)
	}
	if got := r.getRiskColor(types.RiskLevelLow); got != "" {
		t.Fatalf("low: got %q", got)
	}
}

func TestIndent_should_prefix_each_line(t *testing.T) {
	got := indent("a\nb", "  ")
	want := "  a\n  b"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
