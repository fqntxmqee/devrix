//go:build integration && d5

package integration

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/observability/instrument/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/instrument/tracer"
)

// T: D5-S4-A01-T03 (adapter → gateway trace inheritance)
func TestAdapterToGateway_should_share_trace_id(t *testing.T) {
	cfg := observability.DefaultConfig()
	obs, err := observability.New(cfg)
	if err != nil {
		t.Fatalf("new observability: %v", err)
	}

	tr := obs.Tracer()
	ctx := context.Background()

	ctx, adapterSpan := tr.Start(ctx, telemetry.OpD1_S17_Adapter_Message_Receive,
		tracer.WithSpanKind(tracer.SpanKindServer),
		tracer.WithSpanAttributes(telemetry.SpanAttrs(telemetry.OpD1_S17_Adapter_Message_Receive,
			tracer.Attribute{Key: "adapter", Value: "feishu"},
		)...),
	)
	parentTrace := adapterSpan.SpanContext().TraceID
	adapterSpan.End()

	ctx, gatewaySpan := tr.Start(ctx, telemetry.OpD1_S13_Capture_Message_Receive,
		tracer.WithSpanKind(tracer.SpanKindServer),
		tracer.WithSpanAttributes(telemetry.SpanAttrs(telemetry.OpD1_S13_Capture_Message_Receive)...),
	)
	childTrace := gatewaySpan.SpanContext().TraceID
	gatewaySpan.End()

	if !parentTrace.IsValid() || !childTrace.IsValid() {
		t.Fatalf("invalid trace ids parent=%v child=%v", parentTrace, childTrace)
	}
	if parentTrace != childTrace {
		t.Fatalf("trace id mismatch: adapter=%s gateway=%s", parentTrace, childTrace)
	}
}
