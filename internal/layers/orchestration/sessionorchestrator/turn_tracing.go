package sessionorchestrator

import (
	"context"

	"github.com/devrix/devrix/internal/layers/observability/instrument/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/instrument/tracer"
)

func (o *DefaultOrchestrator) startSpan(
	ctx context.Context,
	operation string,
	kind tracer.SpanKind,
	attrs ...tracer.Attribute,
) (context.Context, tracer.Span) {
	if o == nil || o.obsBridge == nil || !o.obsBridge.IsEnabled() {
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
