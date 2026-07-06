package bootstrap

import (
	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
	"github.com/devrix/devrix/internal/shared/config"
)

// Pure pointer helpers for orchtypes.BuildConfig construction.
// These avoid the verbosity of inline &-of-local-variable declarations
// and keep InitOrchestration's coordinatorCfg setup readable.

func boolPtr(b bool) *bool {
	return &b
}

func intPtr(i int) *int {
	return &i
}

func strPtr(s string) *string {
	return &s
}

func float64Ptr(f float64) *float64 {
	return &f
}

// buildSemanticConvergenceFileConfig converts the shared/config
// (pointer-yaml) shape into the orchtypes FileConfig (also pointer-yaml).
// Both are pointer-based so "absent in yaml" propagates correctly; this
// adapter is a no-op on field assignment (DM-20260706-006).
func buildSemanticConvergenceFileConfig(in config.SemanticConvergenceFileConfig) *orchtypes.SemanticConvergenceFileConfig {
	if in.Enabled == nil && in.MinSimilarity == nil && in.LookbackN == nil &&
		in.TimeoutMs == nil && in.ModelTier == nil {
		return nil
	}
	return &orchtypes.SemanticConvergenceFileConfig{
		Enabled:       in.Enabled,
		MinSimilarity: in.MinSimilarity,
		LookbackN:     in.LookbackN,
		TimeoutMs:     in.TimeoutMs,
		ModelTier:     in.ModelTier,
	}
}

// mapBackgroundStatus converts a BackgroundRegistry status string to a
// coordinator TaskStatus. BackgroundRegistry uses "running" while the work
// model uses "in_progress"; all other values ("completed", "failed",
// "cancelled") match directly.
func mapBackgroundStatus(s string) orchtypes.TaskStatus {
	if s == "running" {
		return orchtypes.TaskStatusInProgress
	}
	return orchtypes.TaskStatus(s)
}
