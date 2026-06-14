package compression

import (
	"context"
	"fmt"

	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/observability/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/tracer"
	"github.com/devrix/devrix/internal/shared/types"
)

type tracingStepObserver struct {
	inner     StepObserver
	obsBridge *observability.Bridge
}

// NewTracingStepObserver wraps a StepObserver with per-step tracing spans.
func NewTracingStepObserver(sessionID string, obsBridge *observability.Bridge, sink CompressionEventSink) StepObserver {
	inner := NewPipelineStepObserver(sessionID, sink)
	if inner == nil || obsBridge == nil || !obsBridge.IsEnabled() {
		return inner
	}
	return &tracingStepObserver{inner: inner, obsBridge: obsBridge}
}

func (o *tracingStepObserver) OnStep(ctx context.Context, step string, before, after int) {
	_, span := o.startSpan(ctx, telemetry.OpD2_S2_Context_Compression_Step+"."+step, tracer.SpanKindInternal,
		tracer.Attribute{Key: "compression.step", Value: step},
		tracer.Attribute{Key: "compression.tokens_before", Value: fmt.Sprintf("%d", before)},
		tracer.Attribute{Key: "compression.tokens_after", Value: fmt.Sprintf("%d", after)},
	)
	if span != nil {
		span.End()
	}
	if o.inner != nil {
		o.inner.OnStep(ctx, step, before, after)
	}
}

func (o *tracingStepObserver) OnAutocompact(meta AutocompactMeta) {
	if o.inner != nil {
		o.inner.OnAutocompact(meta)
	}
}

func (o *tracingStepObserver) OnAutocompactComplete(summaryMsg types.Message, sessionID, asyncToken string) {
	if o.inner != nil {
		o.inner.OnAutocompactComplete(summaryMsg, sessionID, asyncToken)
	}
}

func (o *tracingStepObserver) startSpan(ctx context.Context, operation string, kind tracer.SpanKind, attrs ...tracer.Attribute) (context.Context, tracer.Span) {
	if o.obsBridge == nil || o.obsBridge.Tracer() == nil {
		return ctx, nil
	}
	opts := []tracer.SpanStartOption{
		tracer.WithSpanKind(kind),
		tracer.WithSpanAttributes(telemetry.SpanAttrs(operation, attrs...)...),
	}
	if parentSC := tracer.SpanContextFromContext(ctx); parentSC != nil {
		opts = append(opts, tracer.WithParent(*parentSC))
	}
	return o.obsBridge.Tracer().Start(ctx, operation, opts...)
}
