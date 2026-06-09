package observability

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestShutdownManager_should_shutdown_with_context(t *testing.T) {
	obs := NewNoOp()
	mgr := NewShutdownManager(2 * time.Second)

	if err := mgr.ShutdownWithContext(context.Background(), obs); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
}

func TestShutdownManager_should_use_default_timeout(t *testing.T) {
	mgr := NewShutdownManager(0)
	if mgr.timeout != 5*time.Second {
		t.Fatalf("expected default 5s timeout, got %v", mgr.timeout)
	}
}

func TestShutdownManager_should_batch_shutdown_instances(t *testing.T) {
	instances := []*Observability{NewNoOp(), NewNoOp()}
	mgr := NewShutdownManager(time.Second)

	if err := mgr.BatchShutdown(context.Background(), instances); err != nil {
		t.Fatalf("batch shutdown: %v", err)
	}
}

func TestHealthHandler_should_return_healthy_status(t *testing.T) {
	obs := NewNoOp()
	handler := NewHealthHandler(obs)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["status"] != "healthy" {
		t.Fatalf("expected healthy, got %v", body["status"])
	}
}

func TestHealthHandler_should_return_degraded_status(t *testing.T) {
	obs := NewNoOp()
	obs.mu.Lock()
	obs.status.Metrics = "unhealthy"
	obs.mu.Unlock()

	handler := NewHealthHandler(obs)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestReadyHandler_should_report_ready_when_healthy(t *testing.T) {
	handler := NewReadyHandler(NewNoOp())
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ready", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Body.String() != `{"status":"ready"}` {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestReadyHandler_should_report_not_ready_when_degraded(t *testing.T) {
	obs := NewNoOp()
	obs.mu.Lock()
	obs.status.Logging = "unhealthy"
	obs.mu.Unlock()

	handler := NewReadyHandler(obs)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ready", nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestLiveHandler_should_report_alive(t *testing.T) {
	handler := NewLiveHandler()
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/live", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Body.String() != `{"status":"alive"}` {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestObservability_should_shutdown_and_health_check(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Tracing.Exporter = "console"
	cfg.Metrics.Exporter = "null"

	obs, err := New(cfg)
	if err != nil {
		t.Fatalf("new: %v", err)
	}

	status := obs.HealthCheck()
	if status["status"] != "healthy" {
		t.Fatalf("expected healthy, got %v", status["status"])
	}

	if err := obs.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
}
