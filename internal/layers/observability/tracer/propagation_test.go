package tracer

import (
	"context"
	"testing"
)

func TestPropagator_should_inject_and_extract_traceparent(t *testing.T) {
	traceID, err := ParseTraceID("4bf92f3577b34da6a3ce929d0e0e4736")
	if err != nil {
		t.Fatalf("trace id: %v", err)
	}
	spanID, err := ParseSpanID("00f067aa0ba902b7")
	if err != nil {
		t.Fatalf("span id: %v", err)
	}

	sc := SpanContext{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: FlagSampled,
		TraceState: "vendor=value",
	}
	ctx := ContextWithSpan(context.Background(), sc)

	carrier := MapCarrier{}
	prop := NewPropagator()
	if err := prop.Inject(ctx, carrier); err != nil {
		t.Fatalf("inject: %v", err)
	}

	if carrier.Get("traceparent") == "" {
		t.Fatal("expected traceparent header")
	}
	if carrier.Get("tracestate") != "vendor=value" {
		t.Fatalf("expected tracestate, got %q", carrier.Get("tracestate"))
	}

	extracted, err := prop.Extract(context.Background(), carrier)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if !extracted.IsValid() || !extracted.Remote {
		t.Fatal("expected valid remote span context")
	}
	if extracted.TraceState != "vendor=value" {
		t.Fatalf("expected tracestate vendor=value, got %q", extracted.TraceState)
	}
}

func TestPropagator_inject_should_skip_invalid_context(t *testing.T) {
	carrier := MapCarrier{}
	prop := NewPropagator()
	if err := prop.Inject(context.Background(), carrier); err != nil {
		t.Fatalf("inject: %v", err)
	}
	if len(carrier) != 0 {
		t.Fatalf("expected empty carrier, got %v", carrier)
	}
}

func TestPropagator_extract_should_fail_without_traceparent(t *testing.T) {
	prop := NewPropagator()
	_, err := prop.Extract(context.Background(), MapCarrier{})
	if err == nil {
		t.Fatal("expected error for missing traceparent")
	}
}

func TestParseTraceparent_should_validate_format(t *testing.T) {
	_, err := parseTraceparent("bad-header")
	if err == nil {
		t.Fatal("expected error for invalid header")
	}

	_, err = parseTraceparent("00-short-00f067aa0ba902b7-01")
	if err == nil {
		t.Fatal("expected error for invalid trace id length")
	}
}

func TestMapCarrier_should_get_set_and_list_keys(t *testing.T) {
	c := MapCarrier{"a": "1", "b": "2"}
	if c.Get("a") != "1" {
		t.Fatalf("get: %q", c.Get("a"))
	}
	c.Set("c", "3")
	if len(c.Keys()) != 3 {
		t.Fatalf("expected 3 keys, got %d", len(c.Keys()))
	}
}

func TestHTTPHeaderCarrier_should_store_headers(t *testing.T) {
	h := NewHTTPHeaderCarrier()
	h.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")

	if got := h.Get("traceparent"); got == "" {
		t.Fatal("expected traceparent header")
	}
	if vals := h.GetAll("traceparent"); len(vals) != 1 {
		t.Fatalf("expected 1 value, got %d", len(vals))
	}
	if len(h.Keys()) != 1 {
		t.Fatalf("expected 1 key, got %d", len(h.Keys()))
	}
}

func TestSpanContext_traceparent_helpers(t *testing.T) {
	traceID, _ := ParseTraceID("4bf92f3577b34da6a3ce929d0e0e4736")
	spanID, _ := ParseSpanID("00f067aa0ba902b7")
	sc := SpanContext{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: FlagSampled,
		TraceState: "k=v",
	}

	if sc.Traceparent() != sc.String() {
		t.Fatalf("traceparent mismatch: %q vs %q", sc.Traceparent(), sc.String())
	}
	if sc.Tracestate() != "k=v" {
		t.Fatalf("tracestate: %q", sc.Tracestate())
	}
}
