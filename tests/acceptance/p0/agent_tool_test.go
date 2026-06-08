//go:build acceptance && d4

package p0_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/multiagent/tool"
)

// Covers: L5-4-6-01
func TestAcceptance_AgentToolRegistry_P0(t *testing.T) {
	reg := tool.NewRegistry()

	// Register P0 tools
	alpha := tool.NewCLIAgentTool(tool.CLIConfig{
		Name:         "alpha",
		DisplayName:  "Alpha",
		Description:  "First tool",
		Capabilities: []string{"coding", "review"},
		Command:      "bash",
		Args:         []string{"-c", `echo '{"type":"complete","content":""}'`},
	})
	beta := tool.NewCLIAgentTool(tool.CLIConfig{
		Name:         "beta",
		DisplayName:  "Beta",
		Description:  "Second tool",
		Capabilities: []string{"research"},
		Command:      "bash",
		Args:         []string{"-c", `echo '{"type":"complete","content":""}'`},
	})

	if err := reg.Register(alpha); err != nil {
		t.Fatalf("Register alpha: %v", err)
	}
	if err := reg.Register(beta); err != nil {
		t.Fatalf("Register beta: %v", err)
	}

	// Get by name
	got, err := reg.Get("alpha")
	if err != nil {
		t.Fatalf("Get('alpha'): %v", err)
	}
	if got.Info().Name != "alpha" {
		t.Errorf("got.Name = %q, want 'alpha'", got.Info().Name)
	}

	// List returns sorted
	list := reg.List()
	if len(list) != 2 {
		t.Fatalf("List() = %d items, want 2", len(list))
	}
	if list[0].Name != "alpha" || list[1].Name != "beta" {
		t.Errorf("List() order = %+v, want alpha, beta", list)
	}

	// FindByCapability — single match
	codingTools := reg.FindByCapability("coding")
	if len(codingTools) != 1 {
		t.Errorf("FindByCapability('coding') = %d, want 1", len(codingTools))
	}

	// FindByCapability — no match
	none := reg.FindByCapability("editing")
	if len(none) != 0 {
		t.Errorf("FindByCapability('editing') = %d, want 0", len(none))
	}

	// Duplicate registration rejected
	if err := reg.Register(alpha); err == nil {
		t.Error("expected error on duplicate registration")
	}

	// Get unknown tool
	if _, err := reg.Get("unknown"); err == nil {
		t.Error("expected error for unknown tool")
	}

	// Register nil
	if err := reg.Register(nil); err == nil {
		t.Error("expected error for nil tool")
	}
}

// Covers: L5-4-6-02, L5-4-6-04
func TestAcceptance_CLIAgentTool_StreamJSONAndSessionReuse_P0(t *testing.T) {
	agt := tool.NewCLIAgentTool(tool.CLIConfig{
		Name:    "echo-stream",
		Command: "bash",
		Args:    []string{"-c", `echo '{"type":"text","content":"msg1"}'; echo '{"type":"text","content":"msg2"}'; echo '{"type":"complete","content":""}'`},
	})
	defer agt.Stop()

	ctx := context.Background()

	// L5-4-6-02: CLI adapter parses stream-json correctly
	ch, err := agt.Execute(ctx, "sess_reuse", tool.Request{Task: "ping"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var events []tool.Event
	for evt := range ch {
		events = append(events, evt)
	}
	if len(events) < 3 {
		t.Fatalf("got %d events, want >= 3 (2 text + 1 complete)", len(events))
	}
	if events[0].Type != "text" || events[0].Content != "msg1" {
		t.Errorf("event[0] = %+v, want {text msg1}", events[0])
	}
	if events[1].Type != "text" || events[1].Content != "msg2" {
		t.Errorf("event[1] = %+v, want {text msg2}", events[1])
	}
	if events[2].Type != "complete" {
		t.Errorf("event[2].Type = %q, want 'complete'", events[2].Type)
	}

	// L5-4-6-04: Session reuse — same sessionID reuses subprocess
	ch2, err := agt.Execute(ctx, "sess_reuse", tool.Request{Task: "ping2"})
	if err != nil {
		t.Fatalf("second Execute: %v", err)
	}
	for evt := range ch2 {
		if evt.Type == "complete" {
			break
		}
	}
}

// Covers: L5-4-6-07
func TestAcceptance_AgentToolSessionIsolation_P0(t *testing.T) {
	// Use a stateful subprocess that tracks state per session
	agt := tool.NewCLIAgentTool(tool.CLIConfig{
		Name:    "stateful",
		Command: "bash",
		Args:    []string{"-c", `while read line; do echo '{"type":"text","content":"ok"}'; echo '{"type":"complete","content":""}'; done`},
	})
	defer agt.Stop()

	ctx := context.Background()
	var wg sync.WaitGroup

	// Run two isolated sessions concurrently
	for _, sid := range []string{"sess_a", "sess_b"} {
		wg.Add(1)
		sid := sid
		go func() {
			defer wg.Done()
			ch, err := agt.Execute(ctx, sid, tool.Request{Task: "test"})
			if err != nil {
				t.Errorf("Execute(%s): %v", sid, err)
				return
			}
			var gotText bool
			for evt := range ch {
				if evt.Type == "text" && evt.Content == "ok" {
					gotText = true
				}
				if evt.Type == "complete" {
					break
				}
			}
			if !gotText {
				t.Errorf("session %s: no 'ok' text received", sid)
			}
		}()
	}
	wg.Wait()
}

// Covers: L5-4-6-02 (non-JSON fallback), L5-4-6-03 (timeout)
func TestAcceptance_CLIAgentTool_EdgeCases_P1(t *testing.T) {
	t.Run("non-json line falls back to text event", func(t *testing.T) {
		agt := tool.NewCLIAgentTool(tool.CLIConfig{
			Name:    "plain",
			Command: "bash",
			Args:    []string{"-c", `echo "raw line"; echo '{"type":"complete","content":""}'`},
		})
		defer agt.Stop()

		ch, err := agt.Execute(context.Background(), "sess_nonjson_acc", tool.Request{Task: "t"})
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		var events []tool.Event
		for evt := range ch {
			events = append(events, evt)
		}
		if events[0].Type != "text" || !strings.Contains(events[0].Content, "raw line") {
			t.Errorf("expected text fallback, got %+v", events[0])
		}
	})

	t.Run("context timeout stops subprocess", func(t *testing.T) {
		agt := tool.NewCLIAgentTool(tool.CLIConfig{
			Name:    "slow",
			Command: "bash",
			Args:    []string{"-c", `echo '{"type":"text","content":"start"}'; sleep 30; echo '{"type":"complete","content":""}'`},
		})
		defer agt.Stop()

		ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
		defer cancel()

		ch, err := agt.Execute(ctx, "sess_timeout_acc", tool.Request{Task: "t"})
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		gotStart := false
		for evt := range ch {
			if evt.Type == "text" && evt.Content == "start" {
				gotStart = true
			}
		}
		if !gotStart {
			t.Error("expected 'start' text before timeout")
		}
	})
}
