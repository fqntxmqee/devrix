package export

import (
	"context"

	"github.com/devrix/devrix/internal/layers/observability/instrument/tracer"
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

// Shutdown does nothing.
func (e *NullExporter) Shutdown(_ context.Context) error {
	return nil
}
