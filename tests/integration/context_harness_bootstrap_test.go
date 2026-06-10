//go:build integration && d2

package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/communication/gateway"
	"github.com/devrix/devrix/internal/layers/contextengine"
	mockctx "github.com/devrix/devrix/internal/layers/contextengine/mock"
	"github.com/devrix/devrix/internal/layers/contextengine/registry"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

// Covers: L5-2-9-08, L5-2-9-12
func TestIntegration_HarnessBootstrap_disabled_v4_regression(t *testing.T) {
	ctxCfg := config.DefaultContextEngineConfig()
	if ctxCfg.Harness.Enabled {
		t.Fatal("default harness must be disabled")
	}
	ctxCfg.LongTerm.Enabled = false

	engine := contextengine.NewContextEngine(contextengine.EngineDeps{
		LLM:        &mockctx.LLMGateway{Response: "legacy ok"},
		Tools:      &mockctx.ToolRunner{},
		ToolsReg:   registry.NewBuiltinRegistry(),
		Permission: mockctx.AllowAllPermission{},
		Config:     ctxCfg,
	})

	session := types.NewSession("sess_v4", "cli", t.TempDir())
	ch := engine.Process(context.Background(), session, "hello")
	events := drainHarnessEvents(t, ch)

	if session.HarnessInitialized {
		t.Fatal("harness must not initialize when disabled")
	}
	sc, ok := engine.SessionContext(session.SessionID)
	if !ok {
		t.Fatal("session context missing")
	}
	if strings.Contains(sc.SystemPrompt, "<loaded_context>") {
		t.Fatal("disabled path must not emit harness XML system prompt")
	}
	for _, ev := range events {
		if ev.Type == "info" && strings.Contains(ev.Content, "Harness bootstrap") {
			t.Fatal("disabled path must not emit harness bootstrap info")
		}
	}
}

// Covers: L5-2-9-01, L5-2-9-03, L5-2-9-08, L5-2-9-10, L5-2-9-13
func TestIntegration_HarnessBootstrap_enabled_flow(t *testing.T) {
	ctxCfg := config.DefaultContextEngineConfig()
	ctxCfg.Harness.Enabled = true
	ctxCfg.Harness.Prefetch.Enabled = true
	ctxCfg.LongTerm.Enabled = false

	engine := contextengine.NewContextEngine(contextengine.EngineDeps{
		LLM:        &mockctx.LLMGateway{Response: "harness ok"},
		Tools:      &mockctx.ToolRunner{},
		ToolsReg:   registry.NewBuiltinRegistry(),
		Permission: mockctx.AllowAllPermission{},
		Config:     ctxCfg,
	})

	session := types.NewSession("sess_h_on", "cli", t.TempDir())

	ch1 := engine.Process(context.Background(), session, "first message")
	events1 := drainHarnessEvents(t, ch1)
	if !session.HarnessInitialized {
		t.Fatal("expected harness initialized after first process")
	}
	if !hasHarnessBootstrapInfo(events1) {
		t.Fatal("expected harness bootstrap info event on first process")
	}

	sc, ok := engine.SessionContext(session.SessionID)
	if !ok {
		t.Fatal("session context missing")
	}
	if sc.Harness == nil || !sc.Harness.Initialized {
		t.Fatal("expected harness state on session context")
	}
	if !strings.Contains(sc.SystemPrompt, "<loaded_context>") {
		t.Fatal("expected XML loaded_context in system prompt")
	}
	if len(sc.CompressedView) == 0 || sc.CompressedView[0].Role != types.MessageRoleSystem {
		t.Fatal("compressed view must lead with assembled system prompt")
	}
	if sc.CompressedView[0].Content != sc.SystemPrompt {
		t.Fatal("compressed view system must equal Build output")
	}

	ch2 := engine.Process(context.Background(), session, "second message")
	events2 := drainHarnessEvents(t, ch2)
	if hasHarnessBootstrapInfo(events2) {
		t.Fatal("bootstrap info must not repeat on second process")
	}
}

func drainHarnessEvents(t *testing.T, ch <-chan *gateway.EngineEvent) []*gateway.EngineEvent {
	t.Helper()
	var events []*gateway.EngineEvent
	deadline := time.After(5 * time.Second)
	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				return events
			}
			events = append(events, ev)
		case <-deadline:
			t.Fatal("timeout waiting for process events")
		}
	}
}

func hasHarnessBootstrapInfo(events []*gateway.EngineEvent) bool {
	for _, ev := range events {
		if ev.Type == "info" && strings.Contains(ev.Content, "Harness bootstrap") {
			return true
		}
	}
	return false
}
