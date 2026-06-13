package contextengine

import (
	"github.com/devrix/devrix/internal/shared/types"
)

// AutocompactMeta describes autocompact observability metadata.
type AutocompactMeta struct {
	Degraded      bool
	SummaryTokens int
	Model         string
}

// ICompressionObserver emits compression pipeline events.
//
// DSAFT: D2-S2-A03-F01 (EmitCompressionEvents)
type ICompressionObserver interface {
	EmitCompressionStep(sessionID, step string, before, after int)
	EmitAutocompact(sessionID string, meta AutocompactMeta)
	EmitAutocompactComplete(sessionID string, summary types.Message, asyncToken string)
}

// NoOpCompressionObserver discards compression observer events.
type NoOpCompressionObserver struct{}

func (NoOpCompressionObserver) EmitCompressionStep(string, string, int, int)          {}
func (NoOpCompressionObserver) EmitAutocompact(string, AutocompactMeta)               {}
func (NoOpCompressionObserver) EmitAutocompactComplete(string, types.Message, string) {}

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
