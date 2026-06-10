package harness

import (
	"context"

	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/observability/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/tracer"
)

// startHarnessSpan creates a child span for harness operations when observability is enabled.
func startHarnessSpan(
	ctx context.Context,
	bridge *observability.Bridge,
	operation string,
	kind tracer.SpanKind,
	attrs ...tracer.Attribute,
) (context.Context, tracer.Span) {
	if bridge == nil || !bridge.IsEnabled() {
		return ctx, nil
	}
	opts := []tracer.SpanStartOption{
		tracer.WithSpanKind(kind),
		tracer.WithSpanAttributes(telemetry.SpanAttrs(operation, attrs...)...),
	}
	if parentSC := tracer.SpanContextFromContext(ctx); parentSC != nil {
		opts = append(opts, tracer.WithParent(*parentSC))
	}
	return bridge.Tracer().Start(ctx, operation, opts...)
}
