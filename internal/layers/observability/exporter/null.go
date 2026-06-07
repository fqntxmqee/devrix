package exporter

import (
	"context"

	"github.com/devrix/devrix/internal/layers/observability/tracer"
)

// NullExporter is a no-op exporter.
type NullExporter struct{}

// NewNullExporter creates a null exporter.
func NewNullExporter() tracer.SpanExporter {
	return &NullExporter{}
}

// Export does nothing.
func (e *NullExporter) Export(_ context.Context, _ tracer.ReadableSpan) error {
	return nil
}

// ExportBatch does nothing.
func (e *NullExporter) ExportBatch(spans []tracer.ReadableSpan) error {
	return nil
}

// Shutdown does nothing.
func (e *NullExporter) Shutdown(_ context.Context) error {
	return nil
}
