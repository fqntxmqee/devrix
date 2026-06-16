// Package kernel defines the D2 Context Engine domain kernel contracts.
// Root contracts.go re-exports these types for backward compatibility.
package kernel

import (
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/compression"
	"github.com/devrix/devrix/internal/shared/types"
)

// AutocompactMeta describes autocompact observability metadata.
type AutocompactMeta = compression.AutocompactMeta

// ICompressionObserver emits compression pipeline events.
//
// DSAFT: D2-S2-A03-F01 (EmitCompressionEvents)
type ICompressionObserver = compression.CompressionEventSink

// NoOpCompressionObserver discards compression observer events.
type NoOpCompressionObserver = compression.NoOpCompressionEventSink

// IObserver emits context engine observability events.
//
// DSAFT: D2-S1-A01-F02 (EmitEngineEvents)
type IObserver interface {
	EmitContextCompressed(report types.CompressionReport)
	EmitSnapshotRestored(sessionID string, fromBackup bool)
	EmitErrorOccurred(sessionID string, code string, err error)
}

// NoOpObserver discards observer events.
type NoOpObserver struct{}

func (NoOpObserver) EmitContextCompressed(types.CompressionReport) {}
func (NoOpObserver) EmitSnapshotRestored(string, bool)           {}
func (NoOpObserver) EmitErrorOccurred(string, string, error)     {}

// TokenUsage reports token consumption for observability helpers.
type TokenUsage struct {
	PromptTokens     int
	CompletionTokens int
	CacheReadTokens  int
	ReasoningTokens  int
}
