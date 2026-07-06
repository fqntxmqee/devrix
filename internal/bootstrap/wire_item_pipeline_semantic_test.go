package bootstrap

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
	"github.com/devrix/devrix/internal/layers/orchestration/sessionorchestrator"
	"github.com/devrix/devrix/internal/shared/config"
)

// TestBuildItemPipelineSemanticConfig_EnabledDefault verifies that
// passing a "production default" orchtypes.SemanticConvergenceConfig
// produces a sessionorchestrator.SemanticSimilarityConfig with
// Enabled=true (DM-20260706-006 production default).
func TestBuildItemPipelineSemanticConfig_EnabledDefault(t *testing.T) {
	in := orchtypes.DefaultSemanticConvergenceConfig()
	got := buildItemPipelineSemanticConfig(in)
	if !got.Enabled {
		t.Errorf("production default must map to Enabled=true, got false")
	}
	if got.MinSimilarityForVerify != 0.85 {
		t.Errorf("MinSimilarityForVerify: got %v want 0.85", got.MinSimilarityForVerify)
	}
	if got.MinArtifactChars <= 0 {
		t.Errorf("MinArtifactChars must be > 0, got %d", got.MinArtifactChars)
	}
}

// TestBuildItemPipelineSemanticConfig_DisabledExplicit verifies the
// semantic_convergence: {enabled: false} YAML path propagates to runner.
func TestBuildItemPipelineSemanticConfig_DisabledExplicit(t *testing.T) {
	in := orchtypes.SemanticConvergenceConfig{
		Enabled:       false,
		MinSimilarity: 0.5,
		LookbackN:     3,
		TimeoutMs:     4000,
		ModelTier:     "mini",
	}
	got := buildItemPipelineSemanticConfig(in)
	if got.Enabled {
		t.Errorf("explicit false must map to Enabled=false, got true")
	}
	if got.MinSimilarityForVerify != 0.5 {
		t.Errorf("MinSimilarityForVerify: got %v want 0.5", got.MinSimilarityForVerify)
	}
}

// TestBuildItemPipelineSemanticConfig_ZeroValueIsDisabled confirms the
// zero-value orchtypes.SemanticConvergenceConfig maps to a disabled
// SemanticSimilarityConfig (the zero value is the safe "off" default;
// callers must opt in via DefaultSemanticConvergenceConfig or BuildConfig).
func TestBuildItemPipelineSemanticConfig_ZeroValueIsDisabled(t *testing.T) {
	cfg := buildItemPipelineSemanticConfig(orchtypes.SemanticConvergenceConfig{})
	if cfg.Enabled {
		t.Errorf("zero value must map to Enabled=false, got true")
	}
}

// TestBuildSemanticConvergenceFileConfig_AllNil confirms the "absent
// in yaml" path returns nil so the orchtypes.BuildConfig keeps the
// production default.
func TestBuildSemanticConvergenceFileConfig_AllNil(t *testing.T) {
	got := buildSemanticConvergenceFileConfig(config.SemanticConvergenceFileConfig{})
	if got != nil {
		t.Errorf("expected nil when all pointer fields are nil, got %+v", got)
	}
}

// TestBuildSemanticConvergenceFileConfig_PartialOverride confirms that
// only the override fields are non-nil in the returned
// orchtypes.SemanticConvergenceFileConfig.
func TestBuildSemanticConvergenceFileConfig_PartialOverride(t *testing.T) {
	f := false
	timeout := 4000
	in := config.SemanticConvergenceFileConfig{
		Enabled:   &f,
		TimeoutMs: &timeout,
	}
	got := buildSemanticConvergenceFileConfig(in)
	if got == nil {
		t.Fatal("expected non-nil when any field is set")
	}
	if got.Enabled == nil || *got.Enabled != false {
		t.Errorf("Enabled must carry the override (false), got %+v", got.Enabled)
	}
	if got.TimeoutMs == nil || *got.TimeoutMs != 4000 {
		t.Errorf("TimeoutMs must carry the override (4000), got %+v", got.TimeoutMs)
	}
	if got.MinSimilarity != nil {
		t.Errorf("MinSimilarity must remain nil (override absent)")
	}
	if got.LookbackN != nil {
		t.Errorf("LookbackN must remain nil (override absent)")
	}
	if got.ModelTier != nil {
		t.Errorf("ModelTier must remain nil (override absent)")
	}
}

// TestBuildCoordinatorConfig_SemanticConvergenceDefault exercises the
// shared/config layer end-to-end: BuildCoordinatorConfig must produce
// Enabled=true (production default) when the file does not override
// semantic_convergence.
func TestBuildCoordinatorConfig_SemanticConvergenceDefault(t *testing.T) {
	got := config.BuildCoordinatorConfig(nil)
	if got.SemanticConvergence.Enabled == nil {
		t.Fatal("default SemanticConvergence.Enabled must be non-nil")
	}
	if !*got.SemanticConvergence.Enabled {
		t.Errorf("production default must be Enabled=true, got false")
	}
	if got.SemanticConvergence.MinSimilarity == nil || *got.SemanticConvergence.MinSimilarity != 0.85 {
		t.Errorf("MinSimilarity default mismatch: %+v", got.SemanticConvergence.MinSimilarity)
	}
	if got.SemanticConvergence.TimeoutMs == nil || *got.SemanticConvergence.TimeoutMs != 8000 {
		t.Errorf("TimeoutMs default mismatch: %+v", got.SemanticConvergence.TimeoutMs)
	}
}

// TestBuildCoordinatorConfig_SemanticConvergenceOverride confirms the
// yaml override path: enabled=false + timeout=4000 propagates through
// BuildCoordinatorConfig and then BuildSemanticConvergenceConfig.
func TestBuildCoordinatorConfig_SemanticConvergenceOverride(t *testing.T) {
	f := false
	timeout := 4000
	file := &config.CoordinatorFileConfig{
		SemanticConvergence: &config.SemanticConvergenceFileConfig{
			Enabled:   &f,
			TimeoutMs: &timeout,
		},
	}
	coord := config.BuildCoordinatorConfig(file)
	if coord.SemanticConvergence.Enabled == nil || *coord.SemanticConvergence.Enabled {
		t.Errorf("override Enabled=false must propagate, got %+v", coord.SemanticConvergence.Enabled)
	}
	if coord.SemanticConvergence.TimeoutMs == nil || *coord.SemanticConvergence.TimeoutMs != 4000 {
		t.Errorf("override TimeoutMs=4000 must propagate, got %+v", coord.SemanticConvergence.TimeoutMs)
	}
}

// TestBuildConfig_SemanticConvergenceEndToEnd is the full yaml → runner
// config pipeline. A nil file produces the production default
// (Enabled=true); an explicit override propagates to the runner config.
func TestBuildConfig_SemanticConvergenceEndToEnd(t *testing.T) {
	// Path 1: production default.
	cfg := orchtypes.BuildConfig(nil)
	if !cfg.SemanticConvergence.Enabled {
		t.Errorf("orchtypes.BuildConfig(nil).SemanticConvergence.Enabled must be true, got false")
	}
	// Path 2: explicit override (file.SemanticConvergence.Enabled = false).
	f := false
	override := &orchtypes.SemanticConvergenceFileConfig{Enabled: &f}
	file := &orchtypes.FileConfig{SemanticConvergence: override}
	cfg2 := orchtypes.BuildConfig(file)
	if cfg2.SemanticConvergence.Enabled {
		t.Errorf("explicit file override must propagate, got Enabled=true")
	}
}

// Compile-time check that sessionorchestrator.SemanticSimilarityConfig
// is reachable from this file (ensures the import path stays stable).
var _ = sessionorchestrator.DefaultSemanticSimilarityConfig
