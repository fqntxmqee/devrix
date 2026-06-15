package observability

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/observability/instrument/tracer"
)

func TestAppendLLMLogRaw_should_include_trace_id(t *testing.T) {
	dir := t.TempDir()
	span := &recordingSpan{
		sc: tracer.SpanContext{
			TraceID:    tracer.GenerateTraceID(),
			SpanID:     tracer.GenerateSpanID(),
			TraceFlags: tracer.FlagSampled,
		},
	}
	traceID, spanID := spanTraceFields(span)
	appendLLMLogRaw(dir, "sess-1", "request", 0, "test-model", traceID, spanID, json.RawMessage(`{"ok":true}`))

	path := filepath.Join(dir, "sess-1.jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	line := strings.TrimSpace(string(data))
	var record map[string]interface{}
	if err := json.Unmarshal([]byte(line), &record); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if record["trace_id"] != span.sc.TraceID.String() {
		t.Fatalf("trace_id = %v", record["trace_id"])
	}
	if record["span_id"] != span.sc.SpanID.String() {
		t.Fatalf("span_id = %v", record["span_id"])
	}
}

type recordingSpan struct {
	sc tracer.SpanContext
}

func (s *recordingSpan) End(...tracer.SpanEndOption)                          {}
func (s *recordingSpan) SetStatus(tracer.SpanStatusCode, string)                {}
func (s *recordingSpan) RecordError(error, ...tracer.RecordErrorOption)       {}
func (s *recordingSpan) SetAttributes(...tracer.Attribute)                    {}
func (s *recordingSpan) AddEvent(string, ...tracer.EventOption)               {}
func (s *recordingSpan) SpanContext() tracer.SpanContext                      { return s.sc }
func (s *recordingSpan) IsRecording() bool                                    { return true }
