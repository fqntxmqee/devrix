package bootstrap

import (
	"context"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine"
	"github.com/devrix/devrix/internal/layers/multiagent/external"
)

func TestAgentToolPlugin_Execute_should_emit_stream_events(t *testing.T) {
	script := `echo '{"type":"assistant","message":{"content":[{"type":"thinking","thinking":"plan step"},{"type":"text","text":"hello"}]}}'
echo '{"type":"result","subtype":"success","result":"hello"}'`

	agt := external.NewCLIAgentTool(external.CLIConfig{
		Name:        "claude-mock",
		DisplayName: "Claude Code",
		Command:     "bash",
		Args:        []string{"-c", script},
	})
	defer agt.Stop()

	reg := external.NewRegistry()
	if err := reg.Register(agt); err != nil {
		t.Fatalf("Register: %v", err)
	}

	plugin := newAgentToolPlugins(reg, nil)[0]
	var streamed []contextengine.ToolStreamEvent
	ctx := contextengine.WithToolStreamEmitter(context.Background(), func(ev contextengine.ToolStreamEvent) {
		streamed = append(streamed, ev)
	})
	ctx = contextengine.WithToolSessionID(ctx, "sess_stream")

	result, err := plugin.Execute(ctx, t.TempDir(), `{"task":"test"}`)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("result.Error = %q", result.Error)
	}
	if result.Output != "hello" {
		t.Fatalf("result.Output = %q, want hello", result.Output)
	}
	if len(streamed) < 2 {
		t.Fatalf("streamed = %+v, want thinking + text", streamed)
	}
	if streamed[0].Type != "thinking" || !strings.Contains(streamed[0].Content, "plan") {
		t.Fatalf("first stream event = %+v, want thinking", streamed[0])
	}
	if streamed[1].Type != "text" || streamed[1].Content != "hello" {
		t.Fatalf("second stream event = %+v, want text hello", streamed[1])
	}
}
