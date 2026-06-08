//go:build integration

package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/contextengine"
	mockctx "github.com/devrix/devrix/internal/layers/contextengine/mock"
	"github.com/devrix/devrix/internal/layers/contextengine/registry"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

func writeExitScript(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "exit42.py")
	if err := os.WriteFile(path, []byte("import sys\nsys.exit(42)\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

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

// Covers: L5-CTX-26
func TestIntegration_VerifyCommandTimeoutFailsVerify(t *testing.T) {
	dir := t.TempDir()
	cfg := config.DefaultContextEngineConfig()
	cfg.PEV.VerifyMode = config.VerifyModeCommands
	cfg.PEV.VerifyPolicy = config.VerifyPolicyAllPass
	cfg.PEV.VerifyCommands = []config.VerifyCommandConfig{
		{Name: "slow", Executable: "sleep", Args: []string{"10"}, Timeout: 100 * time.Millisecond},
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

	sc := &types.SessionContext{SessionID: "s-timeout", WorkDir: dir, Model: "test"}
	vr := engine.VerifyPEV(context.Background(), sc, []contextengine.ToolResult{{Output: "ok"}})
	if vr.Passed {
		t.Fatalf("expected verify failure on timeout, got %+v", vr)
	}
	if vr.Deviation <= 0 {
		t.Fatalf("expected positive deviation, got %f", vr.Deviation)
	}
}

// Covers: L5-CTX-26
func TestIntegration_VerifyCommandNonZeroExitFailsVerify(t *testing.T) {
	dir := t.TempDir()
	cfg := config.DefaultContextEngineConfig()
	cfg.PEV.VerifyMode = config.VerifyModeCommands
	cfg.PEV.VerifyPolicy = config.VerifyPolicyAllPass
	cfg.PEV.VerifyCommands = []config.VerifyCommandConfig{
		{Name: "exit-42", Executable: "python3", Args: []string{writeExitScript(t, dir)}},
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

	sc := &types.SessionContext{SessionID: "s-exit", WorkDir: dir, Model: "test"}
	vr := engine.VerifyPEV(context.Background(), sc, []contextengine.ToolResult{{Output: "ok"}})
	if vr.Passed {
		t.Fatal("expected verify failure on non-zero exit")
	}
}
