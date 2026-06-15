package coordinator

import (
	"context"
	"fmt"

	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/observability/instrument/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/instrument/tracer"
	"github.com/devrix/devrix/internal/shared/contracts"
)

func startObsSpan(
	bridge *observability.Bridge,
	ctx context.Context,
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

func (o *SessionOrchestrator) startSpan(
	ctx context.Context,
	operation string,
	kind tracer.SpanKind,
	attrs ...tracer.Attribute,
) (context.Context, tracer.Span) {
	if o == nil {
		return ctx, nil
	}
	return startObsSpan(o.obsBridge, ctx, operation, kind, attrs...)
}

func routeLabel(intent IntentClassification) string {
	switch intent.Kind {
	case IntentCommand:
		return "command"
	case IntentFast:
		return "fast"
	case IntentOrchestrate:
		return "orchestrate"
	case IntentSkip:
		return "skip"
	default:
		return string(intent.Kind)
	}
}

func intentClassifyAttrs(intent IntentClassification, source string) []tracer.Attribute {
	attrs := []tracer.Attribute{
		{Key: "orchestration.intent.kind", Value: string(intent.Kind)},
		{Key: "orchestration.intent.confidence", Value: fmt.Sprintf("%d", intent.Confidence)},
		{Key: "orchestration.classify.source", Value: source},
	}
	if intent.Reason != "" {
		attrs = append(attrs, tracer.Attribute{Key: "orchestration.intent.reason", Value: intent.Reason})
	}
	if intent.Command != "" {
		attrs = append(attrs, tracer.Attribute{Key: "orchestration.command", Value: intent.Command})
	}
	return attrs
}

func endSpan(span tracer.Span) {
	if span != nil {
		span.End()
	}
}

func endSpanWithError(span tracer.Span, err error) {
	if span == nil {
		return
	}
	if err != nil {
		span.RecordError(err)
	}
	span.End()
}

func endSpanWhenChannelClosed(ch <-chan *contracts.EngineEvent, span tracer.Span) <-chan *contracts.EngineEvent {
	if span == nil {
		return ch
	}
	out := make(chan *contracts.EngineEvent, 32)
	go func() {
		defer span.End()
		defer close(out)
		for ev := range ch {
			out <- ev
		}
	}()
	return out
}

// WithObservability wires the D5 observability bridge for Jaeger spans.
func WithObservability(bridge *observability.Bridge) OrchestratorOption {
	return func(o *SessionOrchestrator) {
		o.obsBridge = bridge
		if o.orchestratePath != nil {
			o.orchestratePath.SetObsBridge(bridge)
		}
	}
}
