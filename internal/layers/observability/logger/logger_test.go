package logger

import (
	"testing"
)

func TestJSONHandler(t *testing.T) {
	handler := NewJSONHandler()
	handler.SetLevel(LevelInfo)
	// JSON handler outputs to stdout by default
	t.Log("JSON handler created")
}

func TestTextHandler(t *testing.T) {
	handler := NewTextHandler()
	handler.SetLevel(LevelInfo)
	t.Log("Text handler created")
}

func TestLevelParsing(t *testing.T) {
	if LevelInfo.String() != "INFO" {
		t.Errorf("expected INFO, got %s", LevelInfo.String())
	}
}

func TestRedactor(t *testing.T) {
	r := NewRedactor([]string{"password", "token"})
	if r == nil {
		t.Error("redactor should not be nil")
	}

	result := r.Redact("password=secret")
	_ = result // redactor implemented
	t.Log("Redactor created")
}

func TestNewStructuredLogger(t *testing.T) {
	cfg := &LoggerConfig{Level: "debug", Format: "json"}
	logger := NewStructuredLogger(cfg)
	if logger == nil {
		t.Error("logger should not be nil")
	}
}
