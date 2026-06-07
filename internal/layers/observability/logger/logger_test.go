package logger

import (
	"bytes"
	"strings"
	"testing"
)

func TestJSONHandler(t *testing.T) {
	var buf bytes.Buffer
	handler := NewJSONHandler()
	handler.SetOutput(&buf)

	entry := &LogEntry{
		Timestamp: "2026-01-01T00:00:00Z",
		Level:     "INFO",
		Message:   "test message",
		Component: "test",
	}

	err := handler.Handle(entry)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "test message") {
		t.Errorf("expected output to contain message, got: %s", output)
	}
	if !strings.Contains(output, `"level":"INFO"`) {
		t.Errorf("expected output to contain level, got: %s", output)
	}
}

func TestTextHandler(t *testing.T) {
	var buf bytes.Buffer
	handler := NewTextHandler()
	handler.SetOutput(&buf)

	entry := &LogEntry{
		Timestamp: "2026-01-01T00:00:00Z",
		Level:     "INFO",
		Message:   "test message",
	}

	err := handler.Handle(entry)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "test message") {
		t.Errorf("expected output to contain message, got: %s", output)
	}
}

func TestLevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	handler := NewJSONHandler()
	handler.SetLevel(LevelWarn)
	handler.SetOutput(&buf)

	handler.Handle(&LogEntry{
		Level:   "DEBUG",
		Message: "debug message",
	})

	if buf.Len() > 0 {
		t.Error("debug message should be filtered")
	}

	handler.Handle(&LogEntry{
		Level:   "ERROR",
		Message: "error message",
	})

	if !strings.Contains(buf.String(), "error message") {
		t.Error("error message should not be filtered")
	}
}

func TestRedactor(t *testing.T) {
	r := NewRedactor([]string{"password", "token"})

	tests := []struct {
		input    string
		expected string
	}{
		{"password=secret", "password=[REDACTED]"},
		{"token=abc123", "token=[REDACTED]"},
		{"username=user", "username=user"},
	}

	for _, tc := range tests {
		result := r.Redact(tc.input)
		if result != tc.expected {
			t.Errorf("expected %q, got %q", tc.expected, result)
		}
	}
}

func TestRedactorMap(t *testing.T) {
	r := NewRedactor([]string{"secret"})

	m := map[string]interface{}{
		"username": "john",
		"password": "secret123",
	}

	result := r.RedactMap(m)

	if result["username"] != "john" {
		t.Errorf("expected username to be john, got %v", result["username"])
	}
	if result["password"] != "[REDACTED]" {
		t.Errorf("expected password to be [REDACTED], got %v", result["password"])
	}
}

func TestStructuredLogger(t *testing.T) {
	var buf bytes.Buffer
	cfg := &LoggerConfig{
		Level:  "debug",
		Format: "json",
		Redactor: RedactorConfig{
			Enabled:  true,
			Patterns: []string{"secret"},
		},
	}

	l := NewStructuredLogger(cfg)
	l.SetOutput(&buf)

	l.Info("test message", "key", "value")

	output := buf.String()
	if !strings.Contains(output, "test message") {
		t.Errorf("expected output to contain message, got: %s", output)
	}
	if !strings.Contains(output, "key") {
		t.Errorf("expected output to contain key, got: %s", output)
	}
}
