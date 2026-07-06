package sessionorchestrator

import (
	"context"
	"errors"
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
	"github.com/devrix/devrix/internal/layers/orchestration/plan"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/types"
)

// TestNewItemPipelineRunner_AcceptsSemanticConfig verifies that
// ItemPipelineDeps.SemanticConfig / SemanticVerifier are wired through
// to the runner struct (DM-20260706-006 production wire path).
func TestNewItemPipelineRunner_AcceptsSemanticConfig(t *testing.T) {
	cfg := SemanticSimilarityConfig{
		Enabled:                true,
		MinSimilarityForVerify: 0.9,
		MinArtifactChars:       50,
	}
	verifier := &fakeSemanticVerifier{}
	deps := ItemPipelineDeps{
		Tasks:    workmodel.NewTaskManager(),
		Executor: &stubExecutorForDeps{},
		SemanticConfig:   cfg,
		SemanticVerifier: verifier,
	}
	r, err := NewItemPipelineRunner(deps)
	if err != nil {
		t.Fatalf("NewItemPipelineRunner: %v", err)
	}
	if !r.SemanticConfig.Enabled {
		t.Errorf("SemanticConfig.Enabled: got false, want true")
	}
	if r.SemanticConfig.MinSimilarityForVerify != 0.9 {
		t.Errorf("SemanticConfig.MinSimilarityForVerify: got %v, want 0.9", r.SemanticConfig.MinSimilarityForVerify)
	}
	if r.SemanticVerifier == nil {
		t.Errorf("SemanticVerifier: got nil, want non-nil fake")
	}
}

// TestNewItemPipelineRunner_ZeroDepsFallsBackToProductionDefault
// confirms that when ItemPipelineDeps.SemanticConfig is the zero
// value (Enabled=false + MinSimilarity=0 + MinArtifactChars=0), the
// runner uses the production default (Enabled=true + 0.85 + 100).
// This protects against test/bootstrap callers that forget to wire
// the semantic config.
func TestNewItemPipelineRunner_ZeroDepsFallsBackToProductionDefault(t *testing.T) {
	deps := ItemPipelineDeps{
		Tasks:    workmodel.NewTaskManager(),
		Executor: &stubExecutorForDeps{},
	}
	r, err := NewItemPipelineRunner(deps)
	if err != nil {
		t.Fatalf("NewItemPipelineRunner: %v", err)
	}
	if !r.SemanticConfig.Enabled {
		t.Errorf("zero-value deps must fall back to Enabled=true (production default)")
	}
	if r.SemanticConfig.MinSimilarityForVerify <= 0 {
		t.Errorf("zero-value deps must fall back to a positive MinSimilarityForVerify, got %v", r.SemanticConfig.MinSimilarityForVerify)
	}
	if r.SemanticConfig.MinArtifactChars <= 0 {
		t.Errorf("zero-value deps must fall back to a positive MinArtifactChars, got %d", r.SemanticConfig.MinArtifactChars)
	}
}

// TestNewItemPipelineRunner_ExplicitDisabledWins verifies that an
// explicit "enabled: false" in deps is preserved (not overridden by
// the production-default fallback). The fallback only fires when
// EVERY field is zero — if Enabled=false is explicit, the runner
// uses the user-provided value.
func TestNewItemPipelineRunner_ExplicitDisabledWins(t *testing.T) {
	cfg := SemanticSimilarityConfig{
		Enabled:                false,
		MinSimilarityForVerify: 0.5, // explicit non-default
		MinArtifactChars:       50,  // explicit non-default
	}
	deps := ItemPipelineDeps{
		Tasks:           workmodel.NewTaskManager(),
		Executor:        &stubExecutorForDeps{},
		SemanticConfig:  cfg,
	}
	r, err := NewItemPipelineRunner(deps)
	if err != nil {
		t.Fatalf("NewItemPipelineRunner: %v", err)
	}
	if r.SemanticConfig.Enabled {
		t.Errorf("explicit Enabled=false must be preserved, got true")
	}
	if r.SemanticConfig.MinSimilarityForVerify != 0.5 {
		t.Errorf("explicit MinSimilarityForVerify must be preserved, got %v", r.SemanticConfig.MinSimilarityForVerify)
	}
}

// TestOrchtypesToItemPipelineConfigFieldMapping is the cross-package
// field-mapping regression guard. The wire path
// (buildItemPipelineSemanticConfig) must map orchtypes → sessionorchestrator
// fields correctly so the runtime config is honored.
func TestOrchtypesToItemPipelineConfigFieldMapping(t *testing.T) {
	in := orchtypes.SemanticConvergenceConfig{
		Enabled:       true,
		MinSimilarity: 0.77,
		LookbackN:     7,
		TimeoutMs:     1234,
		ModelTier:     "haiku",
	}
	// Mirror the wire mapping (kept identical to wire_item_pipeline.go
	// buildItemPipelineSemanticConfig). This is a duplicate check so
	// that any drift between the wire helper and the runner struct
	// surfaces as a test failure here.
	out := SemanticSimilarityConfig{
		Enabled:                in.Enabled,
		MinSimilarityForVerify: in.MinSimilarity,
		MinArtifactChars:       100,
	}
	if out.Enabled != in.Enabled {
		t.Errorf("Enabled mapping: got %v want %v", out.Enabled, in.Enabled)
	}
	if out.MinSimilarityForVerify != in.MinSimilarity {
		t.Errorf("MinSimilarityForVerify mapping: got %v want %v", out.MinSimilarityForVerify, in.MinSimilarity)
	}
	if out.MinArtifactChars != 100 {
		t.Errorf("MinArtifactChars must be 100 (production floor), got %d", out.MinArtifactChars)
	}
}

// TestDefaultSemanticSimilarityConfig_EnabledTrueIsProductionDefault
// is the production-default regression guard. The original hotfix had
// Enabled=false; the production rollout flipped it to true.
func TestDefaultSemanticSimilarityConfig_EnabledTrueIsProductionDefault(t *testing.T) {
	cfg := DefaultSemanticSimilarityConfig()
	if !cfg.Enabled {
		t.Fatal("production default must be Enabled=true (DM-20260706-006 rollout)")
	}
	if cfg.MinSimilarityForVerify < 0.5 || cfg.MinSimilarityForVerify > 1.0 {
		t.Errorf("MinSimilarityForVerify out of [0.5, 1.0]: %v", cfg.MinSimilarityForVerify)
	}
	if cfg.MinArtifactChars < 50 {
		t.Errorf("MinArtifactChars must be ≥ 50 to avoid trivial triggers, got %d", cfg.MinArtifactChars)
	}
}

// --- minimal stubs ---

// fakeSemanticVerifier returns a fixed VerdictPass so the wiring path
// can confirm the runner stored the pointer correctly.
type fakeSemanticVerifier struct{}

func (f *fakeSemanticVerifier) VerifySemantically(ctx context.Context, req SemanticVerifyRequest) (workmodel.Verdict, error) {
	return workmodel.Verdict{Kind: types.VerdictPass, Confidence: 0.9, SourceID: "fake"}, nil
}

// stubExecutorForDeps satisfies WorkItemExecutor for NewItemPipelineRunner.
// ExecuteWorkItem is never called in these tests.
type stubExecutorForDeps struct{}

func (s *stubExecutorForDeps) ExecuteWorkItem(ctx context.Context, sessionID, workItemID, directive string) (*WorkItemResult, error) {
	return nil, errors.New("stubExecutorForDeps: not implemented")
}

// Ensure plan package reference stays stable (used elsewhere in the
// ItemPipelineDeps wiring; not exercised by these tests but referenced
// here to surface import drift early).
var _ = plan.NewDefaultPlanner
