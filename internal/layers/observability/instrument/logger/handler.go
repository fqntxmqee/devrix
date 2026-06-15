package logger

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
)

// LogLevel represents the severity of a log entry
type LogLevel int

const (
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarn
	LevelError
)

func (l LogLevel) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return fmt.Sprintf("LEVEL(%d)", l)
	}
}

// ParseLogLevel parses a string to LogLevel
func ParseLogLevel(s string) LogLevel {
	switch strings.ToLower(s) {
	case "debug":
		return LevelDebug
	case "info":
		return LevelInfo
	case "warn", "warning":
		return LevelWarn
	case "error":
		return LevelError
	default:
		return LevelInfo
	}
}

// LogEntry represents a structured log entry
type LogEntry struct {
	Timestamp string                 `json:"timestamp"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	TraceID   string                 `json:"traceId,omitempty"`
	SpanID    string                 `json:"spanId,omitempty"`
	Component string                 `json:"component,omitempty"`
	Service   string                 `json:"service,omitempty"`
	Version   string                 `json:"version,omitempty"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
	Error     string                 `json:"error,omitempty"`
}

// Handler is an interface for log handlers
type Handler interface {
	Handle(entry *LogEntry) error
	SetLevel(level LogLevel)
	SetOutput(w io.Writer)
}

// JSONHandler writes logs in JSON format
type JSONHandler struct {
	mu     sync.Mutex
	writer io.Writer
	level  LogLevel
}

// NewJSONHandler creates a new JSON handler
func NewJSONHandler() *JSONHandler {
	return &JSONHandler{
		writer: os.Stdout,
		level:  LevelInfo,
	}
}

// Handle writes a log entry in JSON format
func (h *JSONHandler) Handle(entry *LogEntry) error {
	if entry.Level != "" {
		entryLevel := ParseLogLevel(entry.Level)
		if entryLevel < h.level {
			return nil
		}
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal log entry: %w", err)
	}

	_, err = h.writer.Write(append(data, '\n'))
	return err
}

// SetLevel sets the minimum log level
func (h *JSONHandler) SetLevel(level LogLevel) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.level = level
}

// SetOutput sets the output writer
func (h *JSONHandler) SetOutput(w io.Writer) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.writer = w
}

// TextHandler writes logs in human-readable text format
type TextHandler struct {
	mu     sync.Mutex
	writer io.Writer
	level  LogLevel
}

// NewTextHandler creates a new text handler
func NewTextHandler() *TextHandler {
	return &TextHandler{
		writer: os.Stdout,
		level:  LevelInfo,
	}
}

// Handle writes a log entry in text format
func (h *TextHandler) Handle(entry *LogEntry) error {
	if entry.Level != "" {
		entryLevel := ParseLogLevel(entry.Level)
		if entryLevel < h.level {
			return nil
		}
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	var sb strings.Builder
	sb.WriteString(entry.Timestamp)
	sb.WriteString(" ")
	sb.WriteString("[")
	sb.WriteString(entry.Level)
	sb.WriteString("]")
	
	if entry.TraceID != "" {
		sb.WriteString(" [")
		sb.WriteString(entry.TraceID[:8])
		sb.WriteString("]")
	}
	
	sb.WriteString(" ")
	sb.WriteString(entry.Message)

	for k, v := range entry.Fields {
		sb.WriteString(" ")
		sb.WriteString(k)
		sb.WriteString("=")
		sb.WriteString(fmt.Sprintf("%v", v))
	}

	if entry.Error != "" {
		sb.WriteString(" error=")
		sb.WriteString(entry.Error)
	}

	sb.WriteString("\n")

	_, err := h.writer.Write([]byte(sb.String()))
	return err
}

// SetLevel sets the minimum log level
func (h *TextHandler) SetLevel(level LogLevel) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.level = level
}

// SetOutput sets the output writer
func (h *TextHandler) SetOutput(w io.Writer) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.writer = w
}

// NewHandler creates a handler based on format
func NewHandler(format string) Handler {
	switch strings.ToLower(format) {
	case "json":
		return NewJSONHandler()
	case "text":
		return NewTextHandler()
	default:
		return NewJSONHandler()
	}
}
