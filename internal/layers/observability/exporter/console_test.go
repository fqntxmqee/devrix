package exporter

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"testing"

	"github.com/devrix/devrix/internal/layers/observability/tracer"
)

// Covers: L5-OBS-FIX-08
func TestConsoleExporter_should_implement_span_exporter(t *testing.T) {
	var exp tracer.SpanExporter = NewConsoleExporter()
	if exp == nil {
		t.Fatal("expected non-nil exporter")
	}
}

// Covers: L5-OBS-FIX-08
func TestConsoleExporter_should_export_span_json(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	defer reader.Close()

	exp := NewConsoleExporter()
	exp.SetOutput(writer)

	tp := tracer.NewTracerProvider(nil, nil)
	_, raw := tp.Tracer("test").Start(context.Background(), "test-span")
	raw.End()
	rs := raw.(tracer.ReadableSpan)

	if err := exp.Export(context.Background(), rs); err != nil {
		t.Fatalf("export: %v", err)
	}
	writer.Close()

	out, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("expected exported output")
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if payload["name"] != "test-span" {
		t.Fatalf("unexpected name: %v", payload["name"])
	}
}
