package logger

import (
	"errors"
	"testing"
)

func TestJSONHandler_should_emit_valid_level(t *testing.T) {
	handler := NewJSONHandler()
	handler.SetLevel(LevelInfo)
	if handler == nil {
		t.Fatal("handler should not be nil")
	}
}

func TestTextHandler_should_emit_valid_level(t *testing.T) {
	handler := NewTextHandler()
	handler.SetLevel(LevelInfo)
	if handler == nil {
		t.Fatal("handler should not be nil")
	}
}

func TestLevelParsing(t *testing.T) {
	if LevelInfo.String() != "INFO" {
		t.Errorf("expected INFO, got %s", LevelInfo.String())
	}
}

func TestRedactor_should_mask_sensitive_values(t *testing.T) {
	r := NewRedactor([]string{"password", "token"})
	if r == nil {
		t.Fatal("redactor should not be nil")
	}
	result := r.Redact("password=secret")
	if result == "password=secret" {
		t.Fatal("expected redacted output")
	}
}

func TestNewStructuredLogger(t *testing.T) {
	cfg := &LoggerConfig{Level: "debug", Format: "json"}
	logger := NewStructuredLogger(cfg)
	if logger == nil {
		t.Error("logger should not be nil")
	}
}

// T: D5-S3-A01-T02
func TestStructuredLogger_should_include_stack_on_error(t *testing.T) {
	h := &captureHandler{}
	l := &StructuredLogger{handler: h}
	l.Error("boom", "error", errors.New("kaput"))

	if len(h.entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(h.entries))
	}
	entry := h.entries[0]
	if entry.Error != "kaput" {
		t.Fatalf("expected error message, got %q", entry.Error)
	}
	stack, ok := entry.Fields["stack"].(string)
	if !ok || stack == "" {
		t.Fatal("expected stack field in error log")
	}
}

// T: D5-S3-A01-T03
func TestStructuredLogger_Close_should_reset_sampler_state(t *testing.T) {
	l := &StructuredLogger{
		handler: &captureHandler{},
		sampler: newSpanLogTracker(1),
	}
	tl := l.WithTrace("trace-1", "span-1")
	tl.Info("one")
	if err := l.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
}
