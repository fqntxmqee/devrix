//go:build integration
// +build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/observability/metrics"
	"github.com/devrix/devrix/internal/layers/observability/tracer"
)

func TestObservabilityInit(t *testing.T) {
	cfg := observability.DefaultConfig()
	obs, err := observability.New(cfg)
	if err != nil {
		t.Fatalf("failed to create observability: %v", err)
	}

	if obs == nil {
		t.Fatal("observability should not be nil")
	}

	if obs.Tracer() == nil {
		t.Error("tracer should not be nil")
	}

	if obs.Meter() == nil {
		t.Error("meter should not be nil")
	}

	if obs.Logger() == nil {
		t.Error("logger should not be nil")
	}
}

func TestObservabilityNoOp(t *testing.T) {
	obs := observability.NewNoOp()
	if obs == nil {
		t.Fatal("NewNoOp should return non-nil")
	}

	if obs.IsEnabled() {
		t.Error("NewNoOp should return disabled observability")
	}

	// Should not panic
	obs.Tracer()
	obs.Meter()
	obs.Logger()
}

func TestTracerPropagation(t *testing.T) {
	cfg := observability.DefaultConfig()
	obs, err := observability.New(cfg)
	if err != nil {
		t.Fatalf("failed to create observability: %v", err)
	}

	tr := obs.Tracer()
	ctx, span := tr.Start(context.Background(), "parent.span")

	// Verify span context
	sc := span.SpanContext()
	if !sc.TraceID.IsValid() {
		t.Error("trace ID should be valid")
	}

	// Create child span
	_, childSpan := tr.Start(ctx, "child.span")
	childSc := childSpan.SpanContext()

	if childSc.TraceID != sc.TraceID {
		t.Error("child span should inherit trace ID")
	}

	childSpan.End()
	span.End()

	// Shutdown
	err = obs.Shutdown(context.Background())
	if err != nil {
		t.Errorf("shutdown failed: %v", err)
	}
}

func TestMetricsRecording(t *testing.T) {
	cfg := observability.DefaultConfig()
	obs, err := observability.New(cfg)
	if err != nil {
		t.Fatalf("failed to create observability: %v", err)
	}

	meter := obs.Meter()

	// Create counter
	counter, err := meter.Int64Counter("test_counter",
		metrics.WithLabels(metrics.LabelMap{"label": "value"}))
	if err != nil {
		t.Fatalf("failed to create counter: %v", err)
	}

	counter.Add(10)
	if counter.Value() != 10 {
		t.Errorf("expected counter value 10, got %d", counter.Value())
	}

	counter.Inc()
	if counter.Value() != 11 {
		t.Errorf("expected counter value 11, got %d", counter.Value())
	}

	// Create histogram
	histogram, err := meter.Float64Histogram("test_histogram",
		metrics.WithHistogramLabels(metrics.LabelMap{"label": "value"}))
	if err != nil {
		t.Fatalf("failed to create histogram: %v", err)
	}

	histogram.Observe(0.5)
	histogram.Observe(1.5)

	if histogram.Count() != 2 {
		t.Errorf("expected count 2, got %d", histogram.Count())
	}
}

func TestPrometheusExporter(t *testing.T) {
	cfg := observability.DefaultConfig()
	obs, err := observability.New(cfg)
	if err != nil {
		t.Fatalf("failed to create observability: %v", err)
	}

	meter := obs.Meter()
	registry := meter.Registry()

	// Create test metrics
	counter, _ := meter.Int64Counter("test_requests",
		metrics.WithLabels(metrics.LabelMap{"method": "GET"}))
	counter.Add(100)

	// Get Prometheus output
	output := registry.Output()

	if output == "" {
		t.Error("prometheus output should not be empty")
	}

	if !bytes.Contains([]byte(output), []byte("test_requests")) {
		t.Error("output should contain metric name")
	}

	if !bytes.Contains([]byte(output), []byte("method")) {
		t.Error("output should contain labels")
	}
}

func TestHealthEndpoint(t *testing.T) {
	cfg := observability.DefaultConfig()
	obs, err := observability.New(cfg)
	if err != nil {
		t.Fatalf("failed to create observability: %v", err)
	}

	// Create health handler
	handler := observability.NewHealthHandler(obs)

	// Create test request
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response["status"] != "healthy" {
		t.Errorf("expected status healthy, got %v", response["status"])
	}
}

func TestStructuredLogging(t *testing.T) {
	cfg := observability.DefaultConfig()
	obs, err := observability.New(cfg)
	if err != nil {
		t.Fatalf("failed to create observability: %v", err)
	}

	logger := obs.Logger()

	var buf bytes.Buffer
	logger.SetOutput(&buf)

	// Test basic logging
	logger.Info("test message", "key", "value")

	output := buf.String()
	if output == "" {
		t.Error("output should not be empty")
	}

	// Verify JSON structure
	var logEntry map[string]interface{}
	if err := json.Unmarshal([]byte(output), &logEntry); err != nil {
		t.Fatalf("output should be valid JSON: %v", err)
	}

	if logEntry["message"] != "test message" {
		t.Errorf("expected message 'test message', got %v", logEntry["message"])
	}

	if logEntry["level"] != "INFO" {
		t.Errorf("expected level INFO, got %v", logEntry["level"])
	}
}

func TestBaggagePropagation(t *testing.T) {
	cfg := observability.DefaultConfig()
	obs, err := observability.New(cfg)
	if err != nil {
		t.Fatalf("failed to create observability: %v", err)
	}

	tr := obs.Tracer()
	ctx, span := tr.Start(context.Background(), "parent.span")

	// Set baggage
	bg := observability.NewBaggageManager(32)
	ctx = bg.Set(ctx, "tenant_id", "tenant-123")
	ctx = bg.Set(ctx, "user_id", "user-456")

	// Verify baggage
	if val, ok := bg.Get(ctx, "tenant_id"); !ok || val != "tenant-123" {
		t.Errorf("expected tenant_id=tenant-123, got %v, ok=%v", val, ok)
	}

	// Inject to header
	header := bg.InjectToHeader(ctx)
	if header == "" {
		t.Error("header should not be empty")
	}

	span.End()

	// Shutdown
	obs.Shutdown(context.Background())
}

func TestConfigValidation(t *testing.T) {
	cfg := observability.DefaultConfig()

	// Valid config
	if err := cfg.Validate(); err != nil {
		t.Errorf("default config should be valid: %v", err)
	}

	// Invalid tracing exporter
	cfg.Tracing.Exporter = "invalid"
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for invalid tracing exporter")
	}

	// Invalid sampling rate
	cfg = observability.DefaultConfig()
	cfg.Tracing.Sampling.Type = "trace_id_ratio"
	cfg.Tracing.Sampling.Rate = 1.5 // Invalid > 1.0
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for invalid sampling rate")
	}

	// Invalid logging level
	cfg = observability.DefaultConfig()
	cfg.Logging.Level = "verbose" // Invalid
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for invalid logging level")
	}
}

func TestSpanSampling(t *testing.T) {
	cfg := observability.DefaultConfig()

	// Test always_on
	cfg.Tracing.Sampling.Type = "always_on"
	tp := tracer.NewTracerProvider(&cfg.Tracing)
	tr := tp.Tracer("test")

	_, span := tr.Start(context.Background(), "test.span")
	if !span.IsRecording() {
		t.Error("always_on should always record")
	}
	span.End()

	// Test always_off
	cfg.Tracing.Sampling.Type = "always_off"
	tp = tracer.NewTracerProvider(&cfg.Tracing)
	tr = tp.Tracer("test")

	_, span = tr.Start(context.Background(), "test.span")
	if span.IsRecording() {
		t.Error("always_off should not record")
	}
	span.End()
}

func TestGracefulShutdown(t *testing.T) {
	cfg := observability.DefaultConfig()
	obs, err := observability.New(cfg)
	if err != nil {
		t.Fatalf("failed to create observability: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err = obs.Shutdown(ctx)
	if err != nil {
		t.Errorf("shutdown should succeed: %v", err)
	}
}
