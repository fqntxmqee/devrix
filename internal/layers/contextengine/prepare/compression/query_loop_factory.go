package compression

import (
	"context"

	"github.com/devrix/devrix/internal/layers/contextengine/query"
	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// NewQueryLoopCompressFactory builds a per-session CompressFactory for QueryLoop.
//
// DM-020: summarizer is the D2→D3 拆面 contract; production injects
// D7 turn.CompressionSummarizer, but the deprecated LLMSummarizer fallback
// remains valid for internal tests.
func NewQueryLoopCompressFactory(
	enabled bool,
	cfg *config.ContextEngineConfig,
	counter contracts.ITokenCounter,
	summarizer contracts.Summarizer,
	async *AsyncAutocompacter,
	sink CompressionEventSink,
	obsBridge *observability.Bridge,
) func(sessionID string) query.CompressFunc {
	return func(sessionID string) query.CompressFunc {
		if !enabled || cfg == nil || !cfg.CompressionEnabled {
			return nil
		}
		max, reserved, toolResult, target, snipTarget := cfg.ToTokenBudget()
		return func(ctx context.Context, msgs []types.Message) ([]types.Message, error) {
			opts := []Option{
				WithEnabled(true),
				WithCounter(counter),
				WithAutocompactConfig(cfg.Compression.Autocompact),
				WithCompressionConfig(cfg.Compression),
				WithSummarizer(summarizer),
				WithSkipAssembly(true),
				WithSessionID(sessionID),
			}
			if sink != nil {
				opts = append(opts, WithStepObserver(
					NewTracingStepObserver(sessionID, obsBridge, sink),
				))
			}
			if async != nil {
				opts = append(opts, WithAsyncAutocompacter(async))
			}
			p := NewPipeline(opts...)
			out, _, err := p.RunForSession(ctx, sessionID, msgs, "", types.TokenBudget{
				MaxContextTokens:  max,
				ReservedOutput:    reserved,
				ToolResultBudget:  toolResult,
				CompressionTarget: target,
				SnipTarget:        snipTarget,
			})
			return out, err
		}
	}
}
