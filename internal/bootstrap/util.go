package bootstrap

import "github.com/devrix/devrix/internal/layers/orchestration/orchtypes"

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
