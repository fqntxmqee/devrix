package contextengine

import (
	"context"

	"github.com/devrix/devrix/internal/layers/contextengine/attachments"
	"github.com/devrix/devrix/internal/layers/contextengine/compression"
	"github.com/devrix/devrix/internal/layers/contextengine/queue"
	"github.com/devrix/devrix/internal/layers/contextengine/query"
	"github.com/devrix/devrix/internal/layers/contextengine/usercontext"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// QueryLoopSupport wires QueryLoop dependencies into PEVEngine.
type QueryLoopSupport struct {
	Enabled      bool
	MaxTurns     int
	Compress     bool
	StreamingTools bool
	Attachments  *attachments.Registry
	UserContext  *usercontext.Provider
	CompressFn   func(sessionID string) query.CompressFunc
	SessionQueue *queue.SessionQueue
	Background   *query.BackgroundRegistry
}

// SetQueryLoopSupport configures QueryLoop runtime on PEVEngine (tests and wiring).
func (e *PEVEngine) SetQueryLoopSupport(s QueryLoopSupport) {
	e.queryLoop = s
}

func (e *PEVEngine) setQueryLoopSupport(s QueryLoopSupport) {
	e.SetQueryLoopSupport(s)
}

func (e *PEVEngine) queryLoopEnabled() bool {
	return e.queryLoop.Enabled
}

func newCompressFn(
	enabled bool,
	cfg *config.ContextEngineConfig,
	counter contracts.ITokenCounter,
	llm ILLMGateway,
	async *compression.AsyncAutocompacter,
	compObserver ICompressionObserver,
) func(sessionID string) query.CompressFunc {
	return func(sessionID string) query.CompressFunc {
		if !enabled || cfg == nil || !cfg.CompressionEnabled {
			return nil
		}
		max, reserved, toolResult, target := cfg.ToTokenBudget()
		return func(ctx context.Context, msgs []types.Message) ([]types.Message, error) {
			opts := []compression.Option{
				compression.WithEnabled(true),
				compression.WithCounter(counter),
				compression.WithAutocompactConfig(cfg.Compression.Autocompact),
				compression.WithSummarizer(&AutocompactSummarizer{LLM: llm, Timeout: cfg.Compression.Autocompact.Timeout}),
				compression.WithSkipAssembly(true),
				compression.WithSessionID(sessionID),
			}
			if compObserver != nil {
				opts = append(opts, compression.WithStepObserver(newPipelineStepObserver(sessionID, compObserver)))
			}
			if async != nil {
				opts = append(opts, compression.WithAsyncAutocompacter(async))
			}
			p := compression.NewPipeline(opts...)
			out, _, err := p.RunForSession(ctx, sessionID, msgs, "", types.TokenBudget{
				MaxContextTokens: max, ReservedOutput: reserved,
				ToolResultBudget: toolResult, CompressionTarget: target,
			})
			return out, err
		}
	}
}
