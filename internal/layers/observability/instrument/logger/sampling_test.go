package logger

import (
	"io"
	"testing"
)

type captureHandler struct {
	entries []*LogEntry
}

func (h *captureHandler) Handle(entry *LogEntry) error {
	h.entries = append(h.entries, entry)
	return nil
}
func (h *captureHandler) SetLevel(_ LogLevel)   {}
func (h *captureHandler) SetOutput(_ io.Writer) {}

func TestStructuredLogger_should_sample_logs_per_span(t *testing.T) {
	h := &captureHandler{}
	l := &StructuredLogger{
		handler: h,
		sampler: newSpanLogTracker(2),
	}
	tl := l.WithTrace("trace-1", "span-abc12345")

	tl.Info("one")
	tl.Info("two")
	tl.Info("three")
	tl.Info("four")

	if len(h.entries) != 3 {
		t.Fatalf("expected 2 kept + 1 warn, got %d entries", len(h.entries))
	}
	if h.entries[2].Level != LevelWarn.String() {
		t.Fatalf("expected warn on first drop, got %s", h.entries[2].Level)
	}
}
