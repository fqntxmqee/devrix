package logger

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/devrix/devrix/internal/layers/observability/tracer"
)

// StructuredLogger provides trace-aware structured logging
type StructuredLogger struct {
	handler    Handler
	redactor   *Redactor
	attrs      []any
	component  string
	service    string
	version    string
	mu         sync.RWMutex
}

// NewStructuredLogger creates a new structured logger
func NewStructuredLogger(cfg *LoggerConfig) *StructuredLogger {
	if cfg == nil {
		cfg = DefaultLoggerConfig()
	}

	var handler Handler
	switch cfg.Format {
	case "text":
		handler = NewTextHandler()
	default:
		handler = NewJSONHandler()
	}

	handler.SetLevel(ParseLogLevel(cfg.Level))

	l := &StructuredLogger{
		handler:   handler,
		component: cfg.Component,
		service:   cfg.Service,
		version:   cfg.Version,
	}

	if cfg.Redactor.Enabled && len(cfg.Redactor.Patterns) > 0 {
		l.redactor = NewRedactor(cfg.Redactor.Patterns)
	}

	return l
}

// With returns a new logger with additional attributes
func (l *StructuredLogger) With(args ...any) *StructuredLogger {
	newLogger := &StructuredLogger{
		handler:   l.handler,
		redactor:  l.redactor,
		attrs:     append(l.attrs, args...),
		component: l.component,
		service:   l.service,
		version:   l.version,
	}
	return newLogger
}

// WithComponent returns a new logger with a component name
func (l *StructuredLogger) WithComponent(component string) *StructuredLogger {
	return l.With("component", component)
}

// WithTrace returns a new logger with trace context
func (l *StructuredLogger) WithTrace(sc tracer.SpanContext) *StructuredLogger {
	return l.With(
		"traceId", sc.TraceID.String(),
		"spanId", sc.SpanID.String(),
	)
}

// Debug logs a debug message
func (l *StructuredLogger) Debug(msg string, args ...any) {
	l.log(LevelDebug, msg, args...)
}

// Info logs an info message
func (l *StructuredLogger) Info(msg string, args ...any) {
	l.log(LevelInfo, msg, args...)
}

// Warn logs a warning message
func (l *StructuredLogger) Warn(msg string, args ...any) {
	l.log(LevelWarn, msg, args...)
}

// Error logs an error message
func (l *StructuredLogger) Error(msg string, args ...any) {
	l.log(LevelError, msg, args...)
}

// log creates and handles a log entry
func (l *StructuredLogger) log(level LogLevel, msg string, args ...any) {
	entry := l.buildEntry(level, msg, args...)
	l.handler.Handle(entry)
}

// buildEntry creates a log entry
func (l *StructuredLogger) buildEntry(level LogLevel, msg string, args ...any) *LogEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()

	entry := &LogEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Level:     level.String(),
		Message:   msg,
		Component: l.component,
		Service:   l.service,
		Version:   l.version,
	}

	// Merge attrs with args
	fields := make(map[string]interface{})

	for i := 0; i < len(l.attrs); i += 2 {
		if i+1 < len(l.attrs) {
			key := toString(l.attrs[i])
			if key != "" {
				fields[key] = l.attrs[i+1]
			}
		}
	}

	// Parse args (key-value pairs)
	for i := 0; i < len(args); i += 2 {
		if i+1 < len(args) {
			key := toString(args[i])
			value := args[i+1]
			if key == "" {
				continue
			}

			// Handle error type
			if err, ok := value.(error); ok {
				fields[key] = err.Error()
				if key == "error" {
					fields["error_type"] = fmt.Sprintf("%T", err)
				}
			} else {
				fields[key] = value
			}
		}
	}

	// Extract trace context
	if traceID, ok := fields["traceId"].(string); ok {
		entry.TraceID = traceID
		delete(fields, "traceId")
	}
	if spanID, ok := fields["spanId"].(string); ok {
		entry.SpanID = spanID
		delete(fields, "spanId")
	}

	// Extract component override
	if comp, ok := fields["component"].(string); ok {
		entry.Component = comp
		delete(fields, "component")
	}

	// Extract error
	if errMsg, ok := fields["error"].(string); ok {
		entry.Error = errMsg
		delete(fields, "error")
	}

	// Apply redaction
	if l.redactor != nil {
		entry.Fields = l.redactor.RedactMap(fields)
	} else {
		entry.Fields = fields
	}

	return entry
}

// toString converts a value to string
func toString(v any) string {
	switch val := v.(type) {
	case string:
		return val
	default:
		return fmt.Sprintf("%v", val)
	}
}

// LoggerConfig holds logger configuration
type LoggerConfig struct {
	Level        string          `yaml:"level"`
	Format       string          `yaml:"format"`
	Component   string          `yaml:"component"`
	Service     string          `yaml:"service"`
	Version     string          `yaml:"version"`
	IncludeTrace bool            `yaml:"include_trace_id"`
	Redactor    RedactorConfig `yaml:"redactor"`
}

// RedactorConfig holds redactor configuration
type RedactorConfig struct {
	Enabled  bool     `yaml:"enabled"`
	Patterns []string `yaml:"patterns"`
}

// DefaultLoggerConfig returns default logger configuration
func DefaultLoggerConfig() *LoggerConfig {
	return &LoggerConfig{
		Level:        "info",
		Format:       "json",
		Component:    "devrix",
		Service:      "devrix",
		Version:      "1.0.0",
		IncludeTrace: true,
		Redactor: RedactorConfig{
			Enabled: true,
			Patterns: []string{
				"password", "token", "api_key", "secret",
				"authorization", "private_key",
			},
		},
	}
}

// SetOutput sets the output writer for all handlers
func (l *StructuredLogger) SetOutput(w *os.File) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if h, ok := l.handler.(*JSONHandler); ok {
		h.SetOutput(w)
	} else if h, ok := l.handler.(*TextHandler); ok {
		h.SetOutput(w)
	}
}

// SetLevel sets the minimum log level
func (l *StructuredLogger) SetLevel(level string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	logLevel := ParseLogLevel(level)
	if h, ok := l.handler.(*JSONHandler); ok {
		h.SetLevel(logLevel)
	} else if h, ok := l.handler.(*TextHandler); ok {
		h.SetLevel(logLevel)
	}
}

// Handler returns the underlying handler
func (l *StructuredLogger) Handler() Handler {
	return l.handler
}
