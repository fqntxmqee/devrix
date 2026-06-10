package harness_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/harness"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

type stubToolLister struct {
	tools []harness.ToolDesc
}

func (s stubToolLister) ListTools(_ context.Context, _ string) ([]harness.ToolDesc, error) {
	return s.tools, nil
}

// Covers: L5-2-9-01, L5-2-9-03, L5-2-9-04
func TestBootstrap_should_run_stages_in_order(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0o644); err != nil {
		t.Fatal(err)
	}

	lister := stubToolLister{tools: []harness.ToolDesc{
		{Name: "bash"},
		{Name: "read_file"},
		{Name: "write_file"},
		{Name: "mcp_search"},
	}}

	var stages []types.BootstrapStage
	boot := harness.NewBootstrap(harness.BootstrapDeps{
		Config: config.HarnessConfig{
			Enabled: true,
			Trusted: true,
			Prefetch: config.HarnessPrefetchConfig{Enabled: true, MaxWalkDepth: 2},
			DeferredInit: config.DeferredInitConfig{Enabled: true},
			ToolPool: config.ToolPoolConfig{IncludeMCP: false},
		},
		ToolsReg: lister,
		EmitStage: func(stage types.BootstrapStage, _ map[string]string) {
			stages = append(stages, stage)
		},
	})

	session := types.NewSession("sess_test", "cli", dir)
	state, err := boot.Run(context.Background(), session)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !state.Initialized {
		t.Fatal("expected initialized")
	}
	wantStages := []types.BootstrapStage{
		types.BootstrapStagePrefetch,
		types.BootstrapStageGuards,
		types.BootstrapStageSetup,
		types.BootstrapStageDeferredInit,
		types.BootstrapStageToolPool,
	}
	if len(state.Report.StagesApplied) != len(wantStages) {
		t.Fatalf("stages: got %v want %v", state.Report.StagesApplied, wantStages)
	}
	for i, want := range wantStages {
		if state.Report.StagesApplied[i] != want {
			t.Fatalf("stage[%d]: got %q want %q", i, state.Report.StagesApplied[i], want)
		}
	}
	if state.Report.DeferredInit.PluginInit != true {
		t.Fatal("expected deferred init flags when trusted")
	}
	if state.Report.VisibleTools != 3 {
		t.Fatalf("visible tools: got %d want 3 (mcp excluded)", state.Report.VisibleTools)
	}
}

// Covers: L5-2-9-04
func TestBootstrap_should_skip_deferred_flags_when_untrusted(t *testing.T) {
	dir := t.TempDir()
	boot := harness.NewBootstrap(harness.BootstrapDeps{
		Config: config.HarnessConfig{
			Enabled: true,
			Trusted: false,
			Prefetch: config.HarnessPrefetchConfig{Enabled: false},
			DeferredInit: config.DeferredInitConfig{Enabled: true},
		},
		ToolsReg: stubToolLister{tools: []harness.ToolDesc{{Name: "bash"}}},
	})
	state, err := boot.Run(context.Background(), types.NewSession("sess_u", "cli", dir))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if state.Report.DeferredInit.PluginInit {
		t.Fatal("expected deferred init false when untrusted")
	}
}
