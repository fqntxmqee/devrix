//go:build integration && d2

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/communication/gateway"
	"github.com/devrix/devrix/internal/layers/contextengine"
	mockctx "github.com/devrix/devrix/internal/layers/contextengine/mock"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

// Covers: L5-2-9-01, L5-2-9-03
func TestIntegration_HarnessBootstrapSmoke_disabled_zero_change(t *testing.T) {
	ctxCfg := config.DefaultContextEngineConfig()
	if ctxCfg.Harness.Enabled {
		t.Fatal("default harness must be disabled")
	}
	engine := contextengine.NewContextEngine(contextengine.EngineDeps{
		LLM:        &mockctx.LLMGateway{Response: "ok"},
		Tools:      &mockctx.ToolRunner{},
		ToolsReg:   mustBuiltinRegistry(t),
		Permission: mockctx.AllowAllPermission{},
		Config:     ctxCfg,
	})
	session := types.NewSession("sess_h_off", "cli", t.TempDir())
	ch := engine.Process(context.Background(), session, "hello harness off")
	waitProcess(t, ch)
}

// Covers: L5-2-9-01, L5-2-9-03
func TestIntegration_HarnessBootstrapSmoke_enabled_bootstrap(t *testing.T) {
	ctxCfg := config.DefaultContextEngineConfig()
	ctxCfg.Harness.Enabled = true
	ctxCfg.Harness.Prefetch.Enabled = true
	ctxCfg.LongTerm.Enabled = false

	engine := contextengine.NewContextEngine(contextengine.EngineDeps{
		LLM:        &mockctx.LLMGateway{Response: "harness ok"},
		Tools:      &mockctx.ToolRunner{},
		ToolsReg:   mustBuiltinRegistry(t),
		Permission: mockctx.AllowAllPermission{},
		Config:     ctxCfg,
	})
	session := types.NewSession("sess_h_on", "cli", t.TempDir())
	ch := engine.Process(context.Background(), session, "hello harness on")
	events := waitProcess(t, ch)
	if !session.HarnessInitialized {
		t.Fatal("expected harness initialized on session")
	}
	foundInfo := false
	for _, ev := range events {
		if ev.Type == "info" && ev.Content != "" {
			foundInfo = true
		}
	}
	if !foundInfo {
		t.Fatal("expected info events during process")
	}
}

func waitProcess(t *testing.T, ch <-chan *gateway.EngineEvent) []*gateway.EngineEvent {
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
			t.Fatal("timeout waiting for process")
		}
	}
}
