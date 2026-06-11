package bootstrap_test

import (
	"testing"

	"github.com/devrix/devrix/internal/bootstrap"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

type noopHandler struct{}

func (noopHandler) OnMessage(*types.OutboundMessage)                       {}
func (noopHandler) OnPermissionRequest(*types.PermissionRequest) bool     { return true }
func (noopHandler) OnError(error, string)                                {}
func (noopHandler) OnStatus(string, types.SessionState)                  {}

// Covers: L5-4-10-09 (single session — progress stays in same handler, no second chat)
func TestCLIProgressHandler_should_render_worker_progress_only(t *testing.T) {
	ansi := config.DefaultConfig().CLI.ANSI
	h := bootstrap.NewCLIProgressHandler(noopHandler{}, ansi)
	// RenderWorkerProgress writes to stdout; smoke call only.
	h.OnMessage(&types.OutboundMessage{
		Metadata: map[string]string{"event_type": "worker_progress", "kind": "started"},
		Content:  "explore running",
	})
	// Normal messages still reach base — use a counting handler instead for a smoke test.
	base := &countingHandler{}
	h2 := bootstrap.NewCLIProgressHandler(base, ansi)
	h2.OnMessage(&types.OutboundMessage{Content: "hello"})
	if base.messages != 1 {
		t.Fatalf("base messages = %d", base.messages)
	}
	h2.OnMessage(&types.OutboundMessage{
		Metadata: map[string]string{"event_type": "worker_progress"},
		Content:  "progress",
	})
	if base.messages != 1 {
		t.Fatalf("worker_progress should not increment base, got %d", base.messages)
	}
}

type countingHandler struct {
	messages int
}

func (c *countingHandler) OnMessage(*types.OutboundMessage) {
	c.messages++
}
func (c *countingHandler) OnPermissionRequest(*types.PermissionRequest) bool { return true }
func (c *countingHandler) OnError(error, string)                             {}
func (c *countingHandler) OnStatus(string, types.SessionState)               {}
