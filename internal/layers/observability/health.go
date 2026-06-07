package observability

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// HealthHandler returns health check responses
type HealthHandler struct {
	obs *Observability
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(obs *Observability) *HealthHandler {
	return &HealthHandler{obs: obs}
}

// ServeHTTP handles health check requests
func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	status := h.obs.HealthCheck()

	w.Header().Set("Content-Type", "application/json")
	
	// Determine HTTP status code
	httpStatus := http.StatusOK
	if status["status"] == "degraded" {
		httpStatus = http.StatusServiceUnavailable
	}

	w.WriteHeader(httpStatus)
	
	if err := json.NewEncoder(w).Encode(status); err != nil {
		fmt.Fprintf(w, `{"status":"error","message":"failed to encode status"}`)
	}
}

// ReadyHandler returns readiness check responses
type ReadyHandler struct {
	obs *Observability
}

// NewReadyHandler creates a new readiness handler
func NewReadyHandler(obs *Observability) *ReadyHandler {
	return &ReadyHandler{obs: obs}
}

// ServeHTTP handles readiness check requests
func (h *ReadyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	status := h.obs.HealthCheck()

	// Ready if all components are healthy
	if status["status"] == "healthy" {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ready"}`))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusServiceUnavailable)
	json.NewEncoder(w).Encode(status)
}

// LiveHandler returns liveness check responses
type LiveHandler struct{}

// NewLiveHandler creates a new liveness handler
func NewLiveHandler() *LiveHandler {
	return &LiveHandler{}
}

// ServeHTTP handles liveness check requests
func (h *LiveHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Process is alive if this handler is called
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"alive"}`))
}
