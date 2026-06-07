package tracer

import (
	"context"
	"sync"
	"testing"
)

type recordingExporter struct {
	mu      sync.Mutex
	exports int
}

func (e *recordingExporter) Export(_ context.Context, _ ReadableSpan) error {
	e.mu.Lock()
	e.exports++
	e.mu.Unlock()
	return nil
}

func (e *recordingExporter) Shutdown(_ context.Context) error { return nil }

func (e *recordingExporter) count() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.exports
}

// Covers: L5-OBS-FIX-03
func TestTracerProvider_should_flush_pending_spans_on_shutdown(t *testing.T) {
	exp := &recordingExporter{}
	tp := NewTracerProvider(nil, exp)
	tr := tp.Tracer("test")
	_, span := tr.Start(context.Background(), "pending-span")
	if tr.ActiveSpanCount() != 1 {
		t.Fatalf("expected 1 active span, got %d", tr.ActiveSpanCount())
	}
	_ = span
	if err := tp.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	if got := exp.count(); got != 1 {
		t.Fatalf("expected 1 exported span, got %d", got)
	}
	if tr.ActiveSpanCount() != 0 {
		t.Fatalf("expected active spans cleared, got %d", tr.ActiveSpanCount())
	}
}

func TestTraceIDGeneration(t *testing.T) {
	tid1 := GenerateTraceID()
	tid2 := GenerateTraceID()
	if tid1 == tid2 {
		t.Error("trace IDs should be unique")
	}
}

func TestSpanIDGeneration(t *testing.T) {
	sid1 := GenerateSpanID()
	sid2 := GenerateSpanID()
	if sid1 == sid2 {
		t.Error("span IDs should be unique")
	}
}
