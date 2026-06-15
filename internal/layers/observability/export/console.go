package export

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/devrix/devrix/internal/layers/observability/instrument/tracer"
)

// ConsoleExporter exports spans to stdout
type ConsoleExporter struct {
	mu     sync.Mutex
	output *os.File
}

// NewConsoleExporter creates a new console exporter
func NewConsoleExporter() *ConsoleExporter {
	return &ConsoleExporter{
		output: os.Stdout,
	}
}

// Export exports a span to stdout
func (e *ConsoleExporter) Export(_ context.Context, s tracer.ReadableSpan) error {
	if s == nil {
		return nil
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	sc := s.SpanContext()
	if !sc.IsValid() {
		return nil
	}

	// Build span output
	output := map[string]interface{}{
		"type":        "span",
		"traceId":      sc.TraceID.String(),
		"spanId":       sc.SpanID.String(),
		"parentSpanId": "",
		"name":         s.Name(),
		"kind":         s.Kind().String(),
		"startTime":    s.StartTime().Format("2006-01-02T15:04:05.000Z"),
		"endTime":      s.EndTime().Format("2006-01-02T15:04:05.000Z"),
		"durationMs":   float64(s.Duration().Milliseconds()) / 1000.0,
		"attributes":   s.Attributes(),
		"events":       s.Events(),
		"status":       s.Status().Code.String(),
	}

	if parent := s.Parent(); parent != nil {
		output["parentSpanId"] = parent.SpanID.String()
	}

	// Add error status details
	if s.Status().Code == tracer.StatusCodeError {
		output["status.description"] = s.Status().Description
	}

	data, err := json.Marshal(output)
	if err != nil {
		return fmt.Errorf("failed to marshal span: %w", err)
	}

	_, err = e.output.Write(append(data, '\n'))
	return err
}

// SetOutput sets the output destination
func (e *ConsoleExporter) SetOutput(f *os.File) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.output = f
}

// Shutdown shuts down the exporter
func (e *ConsoleExporter) Shutdown(_ context.Context) error {
	return nil
}
