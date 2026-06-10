package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/devrix/devrix/internal/layers/observability/tracer"
)

func TestContextHandler_should_inject_trace_id_from_context(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	h := NewContextHandler(inner)

	traceID := tracer.GenerateTraceID()
	spanID := tracer.GenerateSpanID()
	sc := tracer.SpanContext{TraceID: traceID, SpanID: spanID, TraceFlags: tracer.FlagSampled}
	ctx := tracer.ContextWithSpan(context.Background(), sc)

	logger := slog.New(h)
	logger.InfoContext(ctx, "test message")

	var entry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("unmarshal log: %v", err)
	}
	if entry["traceId"] != traceID.String() {
		t.Fatalf("traceId = %v, want %s", entry["traceId"], traceID.String())
	}
	if entry["spanId"] != spanID.String() {
		t.Fatalf("spanId = %v, want %s", entry["spanId"], spanID.String())
	}
}
