// Package metrics provides a legacy in-process metrics collector for the
// communication layer.
//
// Deprecated: use observability.SessionBridge and observability.Bridge metrics
// (DM-20260607-007). This package remains for reference only and is not wired
// in the production gateway path.
package metrics

import (
	"sync"
	"time"
)

// Counter represents a cumulative counter
type Counter struct {
	mu    sync.Mutex
	value uint64
}

// Inc increments the counter
func (c *Counter) Inc() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.value++
}

// Value returns the current value
func (c *Counter) Value() uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.value
}

// Add adds a value to the counter
func (c *Counter) Add(v uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.value += v
}

// Gauge represents a gauge value
type Gauge struct {
	mu    sync.Mutex
	value float64
}

// Set sets the gauge value
func (g *Gauge) Set(v float64) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.value = v
}

// Value returns the current value
func (g *Gauge) Value() float64 {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.value
}

// Inc increments the gauge value
func (g *Gauge) Inc() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.value++
}

// Dec decrements the gauge value
func (g *Gauge) Dec() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.value--
}

// Histogram represents a histogram
type Histogram struct {
	mu         sync.Mutex
	count      uint64
	sum        float64
	buckets    map[float64]uint64 // upper bound -> count
	bucketKeys []float64         // sorted bucket bounds
}

// NewHistogram creates a new histogram with the given bucket bounds
func NewHistogram(bounds []float64) *Histogram {
	h := &Histogram{
		buckets:    make(map[float64]uint64),
		bucketKeys: bounds,
	}
	for _, b := range bounds {
		h.buckets[b] = 0
	}
	return h
}

// Observe records a value
func (h *Histogram) Observe(v float64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.count++
	h.sum += v

	for _, bound := range h.bucketKeys {
		if v <= bound {
			h.buckets[bound]++
		}
	}
}

// Count returns the total count
func (h *Histogram) Count() uint64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.count
}

// Sum returns the sum of all observed values
func (h *Histogram) Sum() float64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.sum
}

// Avg returns the average value
func (h *Histogram) Avg() float64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.count == 0 {
		return 0
	}
	return h.sum / float64(h.count)
}

// Buckets returns the bucket counts
func (h *Histogram) Buckets() map[float64]uint64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	result := make(map[float64]uint64)
	for k, v := range h.buckets {
		result[k] = v
	}
	return result
}

// MetricsCollector collects all metrics
type MetricsCollector struct {
	RequestsTotal    *Counter                  // total requests by adapter and status
	ActiveSessions   *Gauge                    // current active sessions by adapter
	ResponseTime    *Histogram                 // response time by adapter
	SessionCount     *Counter                  // total sessions created
	PermissionTotal *Counter                  // permission requests by result

	adapterLabels []string
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		RequestsTotal:    &Counter{},
		ActiveSessions:    &Gauge{},
		ResponseTime:     NewHistogram([]float64{0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0}),
		SessionCount:      &Counter{},
		PermissionTotal:  &Counter{},
	}
}

// RecordRequest records a request
func (m *MetricsCollector) RecordRequest(adapter string, status string) {
	m.RequestsTotal.Inc()
}

// RecordResponseTime records a response time
func (m *MetricsCollector) RecordResponseTime(adapter string, duration time.Duration) {
	m.ResponseTime.Observe(duration.Seconds())
}

// IncrementActiveSessions increments active sessions
func (m *MetricsCollector) IncrementActiveSessions(adapter string) {
	m.ActiveSessions.Inc()
}

// DecrementActiveSessions decrements active sessions
func (m *MetricsCollector) DecrementActiveSessions(adapter string) {
	m.ActiveSessions.Dec()
}

// IncrementSessionCount increments total session count
func (m *MetricsCollector) IncrementSessionCount() {
	m.SessionCount.Inc()
}

// RecordPermission records a permission request
func (m *MetricsCollector) RecordPermission(approved bool) {
	// In a real implementation, we'd use labels
	m.PermissionTotal.Inc()
}

// GetMetrics returns all metrics in Prometheus format
func (m *MetricsCollector) GetMetrics() string {
	// Simplified Prometheus format output
	return formatPrometheusOutput(m)
}

func formatPrometheusOutput(m *MetricsCollector) string {
	// This is a simplified implementation
	// In production, use github.com/prometheus/client_golang
	output := "# HELP devrix_requests_total Total number of requests\n"
	output += "# TYPE devrix_requests_total counter\n"
	output += "devrix_requests_total " + string(rune(m.RequestsTotal.Value())) + "\n\n"

	output += "# HELP devrix_active_sessions Current number of active sessions\n"
	output += "# TYPE devrix_active_sessions gauge\n"
	output += "devrix_active_sessions " + formatFloat(m.ActiveSessions.Value()) + "\n\n"

	output += "# HELP devrix_session_count_total Total number of sessions created\n"
	output += "# TYPE devrix_session_count_total counter\n"
	output += "devrix_session_count_total " + string(rune(m.SessionCount.Value())) + "\n\n"

	output += "# HELP devrix_response_time_seconds Response time in seconds\n"
	output += "# TYPE devrix_response_time_seconds histogram\n"
	output += "devrix_response_time_seconds_count " + string(rune(m.ResponseTime.Count())) + "\n"
	output += "devrix_response_time_seconds_sum " + formatFloat(m.ResponseTime.Sum()) + "\n"

	return output
}

func formatFloat(f float64) string {
	return string(rune(int(f*1000000)/1000000)) + "." + string(rune(int(f*1000000)%1000000))
}
