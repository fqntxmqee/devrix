package sessionorchestrator

import (
	"context"
	"fmt"

	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/observability/instrument/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/instrument/tracer"
	"github.com/devrix/devrix/internal/layers/orchestration/mups/learn"
	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
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

func routeLabel(intent orchtypes.IntentClassification) string {
	if intent.Reason == "loop_first_default" {
		return "turn"
	}
	// v6.1.0: IntentFast and IntentOrchestrate both route through
	// OrchestratePath (5-node MUPS pipeline). Collapse the label so trace
	// dashboards reflect the single execution surface.
	switch intent.Kind {
	case orchtypes.IntentCommand:
		return "command"
	case orchtypes.IntentFast, orchtypes.IntentOrchestrate:
		return "turn_loop"
	case orchtypes.IntentSkip:
		return "skip"
	default:
		return string(intent.Kind)
	}
}

// intentClassifyAttrs reads the classifier source from intent.Source
// (DM-20260630-011). The legacy `source` parameter was retired in
// favour of the typed ClassifierSource on IntentClassification so
// dashboards / D6 Evolution can distinguish rule / llm / hybrid paths
// without a parallel hardcoded string.
func intentClassifyAttrs(intent orchtypes.IntentClassification) []tracer.Attribute {
	source := string(intent.Source)
	if source == "" {
		source = string(orchtypes.SourceRule)
	}
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

// priorSessionSpanAttrs returns the 5 prior-related attributes that
// ProcessMessage writes onto the sessionSpan for D5 observability
// (Phase 7 PR-7.3, D7-S13-A49-T06). The 6th attribute
// (learn.classifier_source) is set after the classify path resolves and
// is not part of this helper because it depends on whether the shadow
// classifier is wired.
//
//	alpha         — string (e.g. "8")
//	beta          — string (e.g. "1")
//	mean          — string formatted to 3 decimals (e.g. "0.889")
//	track_mode    — "developer" / "operator"
//	injected_at   — "phase6_lp1" (real injection) or
//	                "cold_start_failsafe" (no prior was injected)
//
// TrackMode resolution (3-tier policy):
//  1. prior.Reputation != nil && prior.Reputation.TrackMode != "" → use rep
//  2. req.TrackMode != ""                                         → use hint
//  3. else                                                        → "developer"
func priorSessionSpanAttrs(prior *learn.AdaptivePrior, observeReq orchtypes.ObserveRequest, req orchtypes.ProcessRequest) []tracer.Attribute {
	if prior == nil {
		return nil
	}
	priorInjectedAt := "cold_start_failsafe"
	if observeReq.Prior != nil {
		priorInjectedAt = "phase6_lp1"
	}
	priorTrackMode := string(learn.TrackModeDeveloper)
	if prior.Reputation != nil && prior.Reputation.TrackMode != "" {
		priorTrackMode = string(prior.Reputation.TrackMode)
	} else if req.TrackMode != "" {
		priorTrackMode = req.TrackMode
	}
	return []tracer.Attribute{
		{Key: "learn.prior.alpha", Value: fmt.Sprintf("%d", prior.PriorBeta.Alpha)},
		{Key: "learn.prior.beta", Value: fmt.Sprintf("%d", prior.PriorBeta.Beta)},
		{Key: "learn.prior.mean", Value: fmt.Sprintf("%.3f", prior.PriorBeta.Mean())},
		{Key: "learn.prior.track_mode", Value: priorTrackMode},
		{Key: "learn.prior.injected_at", Value: priorInjectedAt},
	}
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
	}
}
