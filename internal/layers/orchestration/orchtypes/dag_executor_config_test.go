package orchtypes

import (
	"testing"
)

// TestDefaultDAGExecutorConfig_OFF locks the production default: the
// flag is OFF until ops explicitly flips it. Mirrors the rollout plan
// in proposal.md §6 ("first 5% → 100% across 2 weeks"). If this test
// breaks, someone changed the default and the multi-intent DAG path
// is now firing for every directive in prod — re-run the canary checks
// before merging.
func TestDefaultDAGExecutorConfig_OFF(t *testing.T) {
	cfg := DefaultDAGExecutorConfig()
	if cfg.Enabled {
		t.Fatal("DefaultDAGExecutorConfig().Enabled must be false (PR-D staging rollout default)")
	}
	if cfg.MaxFanOut != 8 {
		t.Fatalf("MaxFanOut default = %d, want 8", cfg.MaxFanOut)
	}
	if cfg.MaxRetryOnPartialFail != 1 {
		t.Fatalf("MaxRetryOnPartialFail default = %d, want 1", cfg.MaxRetryOnPartialFail)
	}
}

// TestBuildDAGExecutorConfig_NilFileKeepsDefaults verifies the legacy
// pre-PR-D path: a nil FileConfig returns the default and zero side
// effects. Existing bootstrap callers that don't yet wire the new
// sub-config rely on this.
func TestBuildDAGExecutorConfig_NilFileKeepsDefaults(t *testing.T) {
	cfg := BuildDAGExecutorConfig(nil)
	def := DefaultDAGExecutorConfig()
	if cfg != def {
		t.Fatalf("BuildDAGExecutorConfig(nil) = %+v, want defaults %+v", cfg, def)
	}
}

// TestBuildDAGExecutorConfig_ExplicitFlipOn verifies ops can flip the
// flag ON with `dag_executor.enabled: true` and the value flows through.
// Without flipping, the multi-intent path never runs (default OFF above).
func TestBuildDAGExecutorConfig_ExplicitFlipOn(t *testing.T) {
	enabled := true
	maxFan := 4
	maxRetry := 0
	cfg := BuildDAGExecutorConfig(&DAGExecutorFileConfig{
		Enabled:               &enabled,
		MaxFanOut:             &maxFan,
		MaxRetryOnPartialFail: &maxRetry,
	})
	if !cfg.Enabled {
		t.Fatal("Enabled = false, want true after explicit flip")
	}
	if cfg.MaxFanOut != 4 {
		t.Fatalf("MaxFanOut = %d, want 4", cfg.MaxFanOut)
	}
	if cfg.MaxRetryOnPartialFail != 0 {
		t.Fatalf("MaxRetryOnPartialFail = %d, want 0", cfg.MaxRetryOnPartialFail)
	}
}

// TestBuildDAGExecutorConfig_PartialOverrideKeepsDefault verifies that
// specifying only one field in FileConfig preserves the others from
// defaults. The YAML deserialization rule per coding.md §4.2.
func TestBuildDAGExecutorConfig_PartialOverrideKeepsDefault(t *testing.T) {
	enabled := true
	cfg := BuildDAGExecutorConfig(&DAGExecutorFileConfig{Enabled: &enabled})
	def := DefaultDAGExecutorConfig()
	if !cfg.Enabled {
		t.Fatal("Enabled = false after explicit flip, want true")
	}
	if cfg.MaxFanOut != def.MaxFanOut {
		t.Fatalf("MaxFanOut = %d, want default %d", cfg.MaxFanOut, def.MaxFanOut)
	}
	if cfg.MaxRetryOnPartialFail != def.MaxRetryOnPartialFail {
		t.Fatalf("MaxRetryOnPartialFail = %d, want default %d",
			cfg.MaxRetryOnPartialFail, def.MaxRetryOnPartialFail)
	}
}

// TestBuildConfig_DAGExecutorSubConfigWiring verifies that the parent
// BuildConfig carries the DAGExecutor sub-config through. This is the
// end-to-end YAML → runtime flag path; if it's broken, the multi-intent
// fork at item_pipeline.go never gets flipped on, no matter what yaml
// says.
func TestBuildConfig_DAGExecutorSubConfigWiring(t *testing.T) {
	enabled := true
	cfg := BuildConfig(&FileConfig{
		DAGExecutor: &DAGExecutorFileConfig{Enabled: &enabled},
	})
	if !cfg.DAGExecutor.Enabled {
		t.Fatal("BuildConfig did not propagate DAGExecutor.Enabled = true")
	}
}
