// Compressor adapter: D2-S15-A03 CompressContext — wraps compression.Pipeline
// with span + emit hooks + RepairToolMessageChain + CompressPerTurn skip flag.
//
// Matches prepare.ContextCompressor:
//
//	ShouldCompress(msgs []types.Message, budget types.TokenBudget) bool
//	Run(ctx context.Context, msgs []types.Message, systemPrompt string, budget types.TokenBudget) ([]types.Message, types.CompressionReport, error)
//
// Pipeline is constructed lazily via newPipeline() to keep the adapter cheap.
// Per-A04-emit hook on completion.
//
// Replaces facade/engine_prepare.go::prepareMessages (compress branch).
package adapters

import (
	"context"
	"fmt"
	"strings"

	"github.com/devrix/devrix/internal/layers/contextengine/prepare/compression"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/conversation"
	"github.com/devrix/devrix/internal/layers/observability/instrument/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/instrument/tracer"
	"github.com/devrix/devrix/internal/shared/errors"
	"github.com/devrix/devrix/internal/shared/types"
)

// PipelineFactory builds a compression Pipeline for a given session ID. Facade
// injects this so existing per-session config (counter, summarizer, observer)
// is preserved.
type PipelineFactory func(sessionID string) *compression.Pipeline

// CompressorAdapter implements prepare.ContextCompressor.
type CompressorAdapter struct {
	newPipeline PipelineFactory
	hooks       Hooks
	compressPerTurn func() bool // optional: when true, skip per-turn compression
	onComplete func(sessionID string, report types.CompressionReport, ratio func() float64) // optional: post-emit observer
}

// NewCompressorAdapter constructs a ContextCompressor wrapping a Pipeline factory.
func NewCompressorAdapter(factory PipelineFactory, opts ...HooksOption) *CompressorAdapter {
	return &CompressorAdapter{newPipeline: factory, hooks: applyHooks(opts)}
}

// WithCompressPerTurnSkip configures a predicate that, when returning true,
// causes the adapter to skip compression entirely on Run (matches the legacy
// facade behaviour where cfg.TurnRuntime.CompressPerTurn=false opts out).
func (a *CompressorAdapter) WithCompressPerTurnSkip(fn func() bool) *CompressorAdapter {
	a.compressPerTurn = fn
	return a
}

// WithCompletionCallback configures a hook invoked after a successful Run
// (mirrors facade.observer.EmitContextCompressed + SetCompressedView + emit).
func (a *CompressorAdapter) WithCompletionCallback(fn func(sessionID string, report types.CompressionReport, ratio func() float64)) *CompressorAdapter {
	a.onComplete = fn
	return a
}

// ShouldCompress returns true if compression should run. Honors
// CompressPerTurn skip flag.
func (a *CompressorAdapter) ShouldCompress(msgs []types.Message, budget types.TokenBudget) bool {
	if a.compressPerTurn != nil && !a.compressPerTurn() {
		return false
	}
	return a.newPipeline("").ShouldCompress(msgs, budget)
}

// Run executes the compression pipeline. Applies RepairToolMessageChain and
// MessagesAfterCompactBoundary on the input slice before compression, and
// stripSystemMessage on the output (matching facade behavior).
//
// Returns (msgs, report, error). On error, an error event is emitted via hooks.
func (a *CompressorAdapter) Run(
	ctx context.Context,
	msgs []types.Message,
	systemPrompt string,
	budget types.TokenBudget,
) ([]types.Message, types.CompressionReport, error) {
	sessionID := ""
	// sessionID is not part of the interface signature; the facade wires a
	// session-scoped pipeline via the factory, but per-session observer hooks
	// must be set via newPipeline(sessionID) inside the factory. The adapter
	// itself only emits the completion event with whatever sessionID the caller
	// attaches to the returned slice via onComplete.
	_ = sessionID

	repaired := conversation.RepairToolMessageChain(conversation.MessagesAfterCompactBoundary(msgs))
	compSystemPrompt := systemPrompt
	if a.compressPerTurn != nil && !a.compressPerTurn() {
		return repaired, types.CompressionReport{}, nil
	}
	if !a.ShouldCompress(repaired, budget) {
		return repaired, types.CompressionReport{}, nil
	}

	ctx, span := a.hooks.startSpan(ctx, telemetry.OpD2_S2_Context_Compression_Run, tracer.SpanKindInternal,
		tracer.Attribute{Key: "context.tokens_before", Value: fmt.Sprintf("%d", len(repaired))},
		tracer.Attribute{Key: "context.compress_per_turn", Value: boolStr(a.compressPerTurn == nil || a.compressPerTurn())},
		tracer.Attribute{Key: "context.budget", Value: fmt.Sprintf("%d", budget)},
		tracer.Attribute{Key: "context.should_compress", Value: boolStr(a.ShouldCompress(repaired, budget))},
		tracer.Attribute{Key: "context.caller", Value: "d7"},
	)
	if span != nil {
		defer span.End()
	}

	pipeline := a.newPipeline("")
	compressed, report, err := pipeline.Run(ctx, repaired, compSystemPrompt, budget)
	if err != nil {
		if span != nil {
			telemetry.RecordSpanError(span, err)
		}
		if se, ok := err.(*errors.SentinelError); ok {
			a.hooks.emit(errorEvent("", se, false))
		} else {
			a.hooks.emit(errorEvent("", errors.WithCode("CTX_COMPRESSION_FAILED", err.Error(), err), false))
		}
		return nil, report, err
	}

	if span != nil {
		ratio := report.Ratio()
		span.SetAttributes(
			tracer.Attribute{Key: "context.tokens_after", Value: fmt.Sprintf("%d", report.CompressedTokens)},
			tracer.Attribute{Key: "context.messages_before", Value: fmt.Sprintf("%d", len(repaired))},
			tracer.Attribute{Key: "context.messages_after", Value: fmt.Sprintf("%d", len(compressed))},
			tracer.Attribute{Key: "context.steps_applied", Value: strings.Join(report.StepsApplied, ",")},
			tracer.Attribute{Key: "compression.trigger_reason", Value: "token_budget_exceeded"},
			tracer.Attribute{Key: "compression.ratio", Value: fmt.Sprintf("%.4f", ratio)},
		)
	}

	if a.onComplete != nil {
		a.onComplete("", report, func() float64 { return report.Ratio() })
	}
	a.hooks.emit(infoEvent("", fmt.Sprintf("上下文已压缩 (%d→%d tokens)", report.OriginalTokens, report.CompressedTokens)))

	return stripSystemMessage(compressed), report, nil
}

// stripSystemMessage removes the first message if it is a system role message.
// Matches facade/engine_compression.go::stripSystemMessage helper.
func stripSystemMessage(msgs []types.Message) []types.Message {
	if len(msgs) > 0 && msgs[0].Role == types.MessageRoleSystem {
		return msgs[1:]
	}
	return msgs
}

// infoEvent + errorEvent + boolStr are defined in session_loader.go (same package).
// Kept local imports slim here.