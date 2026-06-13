package exporter

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/observability/tracer"
)

func TestSpanToOTLP_should_include_event_attributes(t *testing.T) {
	traceID, err := tracer.ParseTraceID("4bf92f3577b34da6a3ce929d0e0e4736")
	if err != nil {
		t.Fatal(err)
	}

	tp := tracer.NewTracerProvider(nil, nil)
	_, sp := tp.Tracer("test").Start(context.Background(), "llm.stream",
		tracer.WithTraceID(traceID),
	)
	sp.AddEvent("llm.request", tracer.WithEventAttributes(
		tracer.Attribute{Key: "llm.request_json", Value: `{"model":"test"}`},
	))
	sp.AddEvent("exception", tracer.WithEventAttributes(
		tracer.Attribute{Key: "exception.message", Value: "provider unavailable"},
	))

	exp := &OTLPExporter{serviceName: "devrix-test"}
	got := exp.spanToOTLP(sp.(tracer.ReadableSpan))
	if len(got.Events) != 2 {
		t.Fatalf("events = %d, want 2", len(got.Events))
	}
	if got.Events[0].Name != "llm.request" {
		t.Fatalf("first event = %q", got.Events[0].Name)
	}
	if len(got.Events[0].Attributes) != 1 || got.Events[0].Attributes[0].Key != "llm.request_json" {
		t.Fatalf("llm.request attrs = %+v", got.Events[0].Attributes)
	}
	if got.Events[1].Attributes[0].Value.StringValue != "provider unavailable" {
		t.Fatalf("exception message = %q", got.Events[1].Attributes[0].Value.StringValue)
	}
}

func TestOtlpEventAttributes_should_return_nil_for_empty(t *testing.T) {
	if otlpEventAttributes(nil) != nil {
		t.Fatal("expected nil")
	}
}
