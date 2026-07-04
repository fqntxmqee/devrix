package kernel

import (
	"context"

	"github.com/devrix/devrix/internal/layers/contextengine/i18n"
	"github.com/devrix/devrix/internal/layers/contextengine/persist"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/compression"
	"github.com/devrix/devrix/internal/shared/textutil"
	"github.com/devrix/devrix/internal/layers/observability/instrument/metrics"
	"github.com/devrix/devrix/internal/layers/observability/instrument/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/instrument/tracer"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

func (e *ContextEngine) shouldCompress(msgs []types.Message, budget types.TokenBudget) bool {
	return e.compressionPipeline("").ShouldCompress(msgs, budget)
}

func (e *ContextEngine) compressionPipeline(sessionID string) *compression.Pipeline {
	opts := []compression.Option{
		compression.WithEnabled(e.cfg.CompressionEnabled),
		compression.WithCounter(e.counter),
		compression.WithAutocompactConfig(e.cfg.Compression.Autocompact),
		compression.WithCompressionConfig(e.cfg.Compression),
		compression.WithSummarizer(e.summarizer),
		compression.WithLocale(i18n.ParseLanguage(e.cfg.Workspace.Language)),
	}
	if sessionID != "" {
		opts = append(opts,
			compression.WithStepObserver(compression.NewTracingStepObserver(sessionID, e.obsBridge, e.compObserver)),
			compression.WithSessionID(sessionID),
		)
		if base := e.sessionsBaseDir(); base != "" {
			opts = append(opts, compression.WithProjectDir(base))
			state := e.compressionState(sessionID)
			opts = append(opts, compression.WithPerMessageBudget(&compression.PerMessageBudget{
				ProjectDir: base,
				SessionID:  sessionID,
				State:      state,
			}))
		}
	}
	if e.asyncCompact != nil {
		opts = append(opts, compression.WithAsyncAutocompacter(e.asyncCompact))
	}
	return compression.NewPipeline(opts...)
}

func (e *ContextEngine) sessionsBaseDir() string {
	if e == nil || e.cfg == nil {
		return ""
	}
	base := e.cfg.MainTranscript.BaseDir
	if base == "" {
		base = config.DefaultMainTranscriptConfig().BaseDir
	}
	return textutil.ExpandPath(base)
}

func (e *ContextEngine) compressionState(sessionID string) *persist.ContentReplacementState {
	if e == nil || sessionID == "" {
		return persist.NewContentReplacementState()
	}
	if v, ok := e.compressionStates.Load(sessionID); ok {
		if s, ok := v.(*persist.ContentReplacementState); ok && s != nil {
			return s
		}
	}
	s := persist.NewContentReplacementState()
	actual, _ := e.compressionStates.LoadOrStore(sessionID, s)
	if st, ok := actual.(*persist.ContentReplacementState); ok && st != nil {
		return st
	}
	return s
}

func (e *ContextEngine) initMetrics() {
	e.metricsOnce.Do(func() {
		if e.obsBridge == nil || e.obsBridge.Meter() == nil {
			return
		}
		e.compressionRatio, _ = e.obsBridge.Meter().Float64Histogram("compression_ratio",
			metrics.WithBounds(metrics.CompressionRatioBounds()))
	})
}

func (e *ContextEngine) startSpan(ctx context.Context, operation string, kind tracer.SpanKind, attrs ...tracer.Attribute) (context.Context, tracer.Span) {
	if e.obsBridge == nil || e.obsBridge.Tracer() == nil {
		return ctx, nil
	}
	opts := []tracer.SpanStartOption{
		tracer.WithSpanKind(kind),
		tracer.WithSpanAttributes(telemetry.SpanAttrs(operation, attrs...)...),
	}
	if parentSC := tracer.SpanContextFromContext(ctx); parentSC != nil {
		opts = append(opts, tracer.WithParent(*parentSC))
	}
	return e.obsBridge.Tracer().Start(ctx, operation, opts...)
}
