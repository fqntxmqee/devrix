package memory

import (
	"context"

	"github.com/devrix/devrix/internal/shared/errors"
)

// DisabledLongTermMemory is used when longterm.enabled=false.
type DisabledLongTermMemory struct{}

// NewDisabledLongTermMemory creates a disabled long-term memory backend.
func NewDisabledLongTermMemory() *DisabledLongTermMemory {
	return &DisabledLongTermMemory{}
}

// Recall returns FeatureNotImplemented when long-term memory is disabled.
func (m *DisabledLongTermMemory) Recall(_ context.Context, _ string, _ int) ([]MemoryEntry, error) {
	return nil, errors.NewFeatureNotImplementedError("long-term memory", "v3")
}

// Store is a no-op for disabled memory.
func (m *DisabledLongTermMemory) Store(_ context.Context, _ MemoryEntry) error {
	return nil
}

// Close is a no-op.
func (m *DisabledLongTermMemory) Close() error {
	return nil
}
