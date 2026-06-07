package tracer

import (
	"context"
	"time"
)

// ReadableSpan exposes span data for exporters.
type ReadableSpan interface {
	Span
	Name() string
	Kind() SpanKind
	StartTime() time.Time
	EndTime() time.Time
	Duration() time.Duration
	Attributes() map[string]interface{}
	Events() []Event
	Status() Status
	Parent() *SpanContext
}

// SpanExporter exports completed spans.
type SpanExporter interface {
	Export(ctx context.Context, span ReadableSpan) error
	Shutdown(ctx context.Context) error
}
