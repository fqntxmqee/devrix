package tracer

import (
	"context"
	"testing"
)

func TestPropagationEnvVars_should_emit_trace_and_baggage(t *testing.T) {
	t.Parallel()

	traceID, _ := ParseTraceID("4bf92f3577b34da6a3ce929d0e0e4736")
	spanID, _ := ParseSpanID("00f067aa0ba902b7")
	ctx := ContextWithSpan(context.Background(), SpanContext{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: FlagSampled,
	})
	ctx = DefaultBaggageManager.Set(ctx, "session.id", "sess_env")

	env := PropagationEnvVars(ctx)
	if len(env) < 2 {
		t.Fatalf("expected traceparent and baggage env vars, got %v", env)
	}

	found := map[string]bool{}
	for _, e := range env {
		found[e] = true
	}
	hasTraceparent := false
	hasBaggage := false
	for k := range found {
		if len(k) > 12 && k[:12] == "TRACEPARENT=" {
			hasTraceparent = true
		}
		if len(k) > 8 && k[:8] == "BAGGAGE=" {
			hasBaggage = true
		}
	}
	if !hasTraceparent {
		t.Fatal("expected TRACEPARENT env var")
	}
	if !hasBaggage {
		t.Fatal("expected BAGGAGE env var")
	}
}
