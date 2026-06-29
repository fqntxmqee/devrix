// Package kernel defines the D2 Context Engine domain kernel contracts.
// Root contracts.go re-exports these types for backward compatibility.
package kernel

import (
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/compression"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// --- Observer types ---

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

// --- Cross-layer contracts (aliased from shared/) ---

// IEngine is the cross-layer context processing contract (L1 ↔ L2 ↔ L4).
//
// DSAFT: D2-S1-A01-F01
type IEngine = contracts.IEngine

// EngineEvent is emitted by the context engine during Process.
//
// DSAFT: D2-S1-A01-F02
type EngineEvent = contracts.EngineEvent

// ContextEngineConfig is the D2 domain configuration root.
//
// DSAFT: D2-S1-A01 (Config)
type ContextEngineConfig = config.ContextEngineConfig

// --- ContextEngine (formerly legacy.ContextEngine) ---
//
// DM-20260629-002 (devrix-d2-dsaft-restructuring): legacy/ package retired
// (P5 deprecation window closed). The D2 ContextEngine implementation
// moved to internal/layers/contextengine/kernel/ alongside the observer
// contracts. Tests and bootstrap now import it via the kernel package
// directly rather than via the legacy package.

// ContextEngine re-exports the D2 domain entry point (S15→S17 glue via D7 PreparedTurn).
//
// DSAFT: D2-S15-A01 (LoadSession) / D2-S17-A01 (PersistSessionState)
// (Already declared in context_engine_types.go — no alias needed; this
// header documents the DM-20260629-002 legacy→kernel migration.)
