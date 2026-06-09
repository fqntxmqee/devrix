package exporter

import (
	"context"
	"sync"

	"github.com/devrix/devrix/internal/layers/observability/tracer"
)

// MemoryExporter collects exported spans in memory for test inspection.
type MemoryExporter struct {
	mu    sync.Mutex
	spans []tracer.ReadableSpan
}

// NewMemoryExporter creates a MemoryExporter.
func NewMemoryExporter() *MemoryExporter {
	return &MemoryExporter{}
}

// Export appends the span to the in-memory slice.
func (e *MemoryExporter) Export(_ context.Context, s tracer.ReadableSpan) error {
	if s == nil {
		return nil
	}
	e.mu.Lock()
	e.spans = append(e.spans, s)
	e.mu.Unlock()
	return nil
}

// Spans returns a copy of all collected spans.
func (e *MemoryExporter) Spans() []tracer.ReadableSpan {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]tracer.ReadableSpan, len(e.spans))
	copy(out, e.spans)
	return out
}

// Reset clears all collected spans.
func (e *MemoryExporter) Reset() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.spans = nil
}

// Shutdown is a no-op.
func (e *MemoryExporter) Shutdown(_ context.Context) error {
	return nil
}
