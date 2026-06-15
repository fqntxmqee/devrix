package telemetry

import (
	"github.com/devrix/devrix/internal/layers/observability/instrument/tracer"
	"github.com/devrix/devrix/internal/shared/errors"
)

// RecordSpanError records an error on the span and sets error.code when available.
func RecordSpanError(span tracer.Span, err error) {
	if span == nil || err == nil {
		return
	}
	span.RecordError(err)
	if code := errors.ErrorCode(err); code != "" {
		span.SetAttributes(tracer.Attribute{Key: "error.code", Value: code})
	}
}
