package telemetry_test

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/observability/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/tracer"
	"github.com/devrix/devrix/internal/shared/errors"
)

type attrSpan struct {
	tracer.Span
	attrs map[string]interface{}
	events []string
}

func (s *attrSpan) SetAttributes(kv ...tracer.Attribute) {
	if s.attrs == nil {
		s.attrs = make(map[string]interface{})
	}
	for _, a := range kv {
		s.attrs[a.Key] = a.Value
	}
}
func (s *attrSpan) RecordError(err error, _ ...tracer.RecordErrorOption) {
	if err != nil {
		s.events = append(s.events, err.Error())
	}
}
func (s *attrSpan) End(...tracer.SpanEndOption)                             {}
func (s *attrSpan) SetStatus(tracer.SpanStatusCode, string)                 {}
func (s *attrSpan) AddEvent(string, ...tracer.EventOption)                  {}
func (s *attrSpan) SpanContext() tracer.SpanContext                         { return tracer.SpanContext{} }
func (s *attrSpan) IsRecording() bool                                       { return true }

func TestRecordSpanError_should_set_error_code(t *testing.T) {
	span := &attrSpan{}
	err := errors.NewContextPermissionDeniedError("bash")
	telemetry.RecordSpanError(span, err)
	if span.attrs["error.code"] != errors.CodePermissionDenied {
		t.Fatalf("error.code = %v", span.attrs["error.code"])
	}
	if len(span.events) != 1 {
		t.Fatalf("expected recorded error event")
	}
}
