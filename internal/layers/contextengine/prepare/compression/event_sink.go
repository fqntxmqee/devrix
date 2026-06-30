package compression

import "github.com/devrix/devrix/internal/shared/types"

// CompressionEventSink emits compression pipeline observability events.
//
// DSAFT: D2-S2-A03-F01 (EmitCompressionEvents)
type CompressionEventSink interface {
	EmitCompressionStep(sessionID, step string, before, after int)
	EmitAutocompact(sessionID string, meta AutocompactMeta)
	EmitAutocompactComplete(sessionID string, summary types.Message, asyncToken string)
	EmitAutocompactFailed(sessionID string, restored []types.Message, asyncToken string)
}

// NoOpCompressionEventSink discards compression observer events.
type NoOpCompressionEventSink struct{}

func (NoOpCompressionEventSink) EmitCompressionStep(string, string, int, int)          {}
func (NoOpCompressionEventSink) EmitAutocompact(string, AutocompactMeta)               {}
func (NoOpCompressionEventSink) EmitAutocompactComplete(string, types.Message, string) {}
func (NoOpCompressionEventSink) EmitAutocompactFailed(string, []types.Message, string) {}
