package memory

import (
	"github.com/devrix/devrix/internal/shared/errors"
)

// LongTermMemory is a V3 placeholder.
type LongTermMemory struct{}

// NewLongTermMemory creates the stub.
func NewLongTermMemory() *LongTermMemory {
	return &LongTermMemory{}
}

// Recall returns FeatureNotImplemented in V1.
func (m *LongTermMemory) Recall(query string) error {
	return errors.NewFeatureNotImplementedError("long-term memory", "v3")
}
