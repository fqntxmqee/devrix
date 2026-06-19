package kernel_test

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/kernel"
	"github.com/devrix/devrix/internal/shared/types"
)

// T: D2-S1-A01-T02 (NoOpObserver contract verification)
// Migrated from internal/layers/contextengine/legacy/engine_helpers_test.go
// during the 2026-06-19 D2 legacy test cleanup. The legacy file tested
// real production code (`kernel.NoOpObserver`) that belongs next to its
// source in kernel/, not in the legacy/ deprecation home.
func TestNoOpObserver(t *testing.T) {
	var obs kernel.NoOpObserver
	obs.EmitContextCompressed(types.CompressionReport{})
	obs.EmitSnapshotRestored("s", true)
	obs.EmitErrorOccurred("s", "E", nil)
}