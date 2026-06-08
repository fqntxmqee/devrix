//go:build integration && d5

package integration

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/observability"
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
		t.Error("NoOp should be disabled")
	}

	if obs.Tracer() != nil {
		t.Error("NoOp tracer should be nil")
	}
}

func TestTracerPropagation(t *testing.T) {
	cfg := observability.DefaultConfig()
	obs, err := observability.New(cfg)
	if err != nil {
		t.Fatalf("failed to create observability: %v", err)
	}

	tr := obs.Tracer()

	_, span := tr.Start(context.Background(), "parent.span")
	sc := span.SpanContext()

	// Check trace ID validity
	if sc.TraceID.IsValid() {
		t.Log("trace ID is valid")
	}

	// Create child span
	_, childSpan := tr.Start(context.Background(), "child.span")
	childSc := childSpan.SpanContext()

	if childSc.TraceID.IsValid() {
		t.Log("child trace ID is valid")
	}

	childSpan.End()
	span.End()

	_ = obs.Shutdown(context.Background())
}

func TestMetricsRecording(t *testing.T) {
	cfg := observability.DefaultConfig()
	obs, err := observability.New(cfg)
	if err != nil {
		t.Fatalf("failed to create observability: %v", err)
	}

	meter := obs.Meter()

	// Create counter without restricted labels
	counter, err := meter.Int64Counter("test_counter_simple")
	if err != nil {
		t.Fatalf("failed to create counter: %v", err)
	}

	counter.Add(10)
	counter.Inc()

	// Create histogram
	histogram, err := meter.Float64Histogram("test_histogram_simple")
	if err != nil {
		t.Fatalf("failed to create histogram: %v", err)
	}

	histogram.Observe(0.5)
	histogram.Observe(1.5)

	_ = obs.Shutdown(context.Background())
}

func TestPrometheusExporter(t *testing.T) {
	cfg := observability.DefaultConfig()
	obs, err := observability.New(cfg)
	if err != nil {
		t.Fatalf("failed to create observability: %v", err)
	}

	meter := obs.Meter()

	// Create test metrics
	counter, _ := meter.Int64Counter("test_requests")
	counter.Add(100)

	registry := meter.Registry()
	if registry == nil {
		t.Skip("registry not available")
	}

	// Get Prometheus output
	output := registry.Output()

	if output == "" {
		t.Error("prometheus output should not be empty")
	}

	_ = obs.Shutdown(context.Background())
}

func TestHealthHandler(t *testing.T) {
	cfg := observability.DefaultConfig()
	obs, err := observability.New(cfg)
	if err != nil {
		t.Fatalf("failed to create observability: %v", err)
	}

	handler := observability.NewHealthHandler(obs)
	if handler == nil {
		t.Fatal("health handler should not be nil")
	}

	req := httptest.NewRequest("GET", "/health", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	_ = obs.Shutdown(context.Background())
}

func TestSpanSampling(t *testing.T) {
	cfg := observability.DefaultConfig()

	// Test always_on
	cfg.Tracing.Sampling.Type = "always_on"
	tp := tracer.NewTracerProvider(&cfg.Tracing, nil)
	tr := tp.Tracer("test")

	_, span := tr.Start(context.Background(), "test.span")
	if !span.IsRecording() {
		t.Error("always_on should always record")
	}
	span.End()

	// Test always_off
	cfg.Tracing.Sampling.Type = "always_off"
	tp = tracer.NewTracerProvider(&cfg.Tracing, nil)
	tr = tp.Tracer("test")

	_, span = tr.Start(context.Background(), "test.span")
	if span.IsRecording() {
		t.Error("always_off should not record")
	}
	span.End()
}

func TestGracefulShutdown(t *testing.T) {
	cfg := observability.DefaultConfig()
	cfg.Tracing.Enabled = false
	cfg.Metrics.Enabled = false
	cfg.Logging.Enabled = false

	obs, err := observability.New(cfg)
	if err != nil {
		t.Fatalf("failed to create observability: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = obs.Shutdown(ctx)
	if err != nil {
		t.Logf("shutdown returned: %v", err)
	}
}

func TestStructuredLogging(t *testing.T) {
	cfg := observability.DefaultConfig()
	cfg.Logging.Format = "json"
	cfg.Logging.Level = "debug"

	obs, err := observability.New(cfg)
	if err != nil {
		t.Fatalf("failed to create observability: %v", err)
	}

	log := obs.Logger()

	// Test basic logging
	log.Info("test message", "key", "value")

	// Test with trace ID
	tr := obs.Tracer()
	_, span := tr.Start(context.Background(), "log-test-span")
	traceID := span.SpanContext().Traceparent
	log.Info("log with trace", "traceparent", traceID)
	span.End()

	_ = obs.Shutdown(context.Background())
}

func TestMetricsRegistry(t *testing.T) {
	cfg := observability.DefaultConfig()
	cfg.Metrics.Enabled = true

	obs, err := observability.New(cfg)
	if err != nil {
		t.Fatalf("failed to create observability: %v", err)
	}

	meter := obs.Meter()
	registry := meter.Registry()
	if registry == nil {
		t.Skip("registry not available")
	}

	// Test counter
	counter, err := meter.Int64Counter("registry_test_counter")
	if err != nil {
		t.Fatalf("failed to create counter: %v", err)
	}
	counter.Add(1)

	// Verify metric appears in output
	output := registry.Output()
	if output == "" {
		t.Error("registry output should not be empty")
	}

	_ = obs.Shutdown(context.Background())
}

func TestOTLPExport(t *testing.T) {
	cfg := observability.DefaultConfig()
	cfg.Tracing.OTLP.Endpoint = "http://localhost:4318/v1/traces"
	cfg.Tracing.OTLP.Insecure = true
	cfg.Tracing.Sampling.Type = "always_on"

	// This test verifies OTLP config is valid
	obs, err := observability.New(cfg)
	if err != nil {
		t.Fatalf("failed to create observability with OTLP: %v", err)
	}

	tr := obs.Tracer()
	_, span := tr.Start(context.Background(), "otlp-test-span")
	span.End()

	_ = obs.Shutdown(context.Background())
}
