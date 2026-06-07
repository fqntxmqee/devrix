//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine"
	mockctx "github.com/devrix/devrix/internal/layers/contextengine/mock"
	"github.com/devrix/devrix/internal/layers/contextengine/registry"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

// Covers: L5-CTX-14, L5-CTX-15
func TestIntegration_VerifyCommandsMode(t *testing.T) {
	dir := t.TempDir()
	cfg := config.DefaultContextEngineConfig()
	cfg.PEV.VerifyMode = config.VerifyModeCommands
	cfg.PEV.VerifyPolicy = config.VerifyPolicyAllPass
	cfg.PEV.VerifyCommands = []config.VerifyCommandConfig{
		{Name: "go-version", Executable: "go", Args: []string{"version"}},
	}

	engine := contextengine.NewPEVEngine(
		&mockctx.LLMGateway{Response: "done"},
		&mockctx.ToolRunner{Output: "ok"},
		registry.NewBuiltinRegistry(),
		mockctx.AllowAllPermission{},
		contextengine.NoOpObserver{},
		&cfg.PEV,
		nil,
		contextengine.NewBuiltinVerifyRunner(dir),
		contextengine.NoOpPEVObserver{},
		nil,
		config.DefaultPlanConfig(),
	)

	sc := &types.SessionContext{
		SessionID: "s1",
		WorkDir:   dir,
		Model:     "test",
	}
	vr := engine.VerifyPEV(context.Background(), sc, []contextengine.ToolResult{{Output: "ok"}})
	if !vr.Passed {
		t.Fatalf("expected verify pass, got %+v", vr)
	}
}
