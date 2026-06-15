package metrics

import (
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"
	"sync"
)

// PrometheusExporter exports metrics in Prometheus format
type PrometheusExporter struct {
	registry *Registry
	mu      sync.RWMutex
}

// NewPrometheusExporter creates a new Prometheus exporter
func NewPrometheusExporter(registry *Registry) *PrometheusExporter {
	return &PrometheusExporter{
		registry: registry,
	}
}

// Handler returns an HTTP handler for the /metrics endpoint
func (e *PrometheusExporter) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		e.mu.RLock()
		defer e.mu.RUnlock()
		
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		
		output := e.registry.Output()
		w.Write([]byte(output))
	})
}

// Output returns all metrics in Prometheus exposition format
func (r *Registry) Output() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	var sb strings.Builder
	
	// Output counters
	for key, c := range r.counters {
		sb.WriteString("# HELP ")
		sb.WriteString(key)
		sb.WriteString(" ")
		sb.WriteString(key)
		sb.WriteString("\n# TYPE ")
		sb.WriteString(key)
		sb.WriteString(" counter\n")
		sb.WriteString(key)
		sb.WriteString(labelsToPrometheus(r.counters[key].Labels()))
		sb.WriteString(" ")
		sb.WriteString(fmt.Sprintf("%d\n", c.Value()))
	}
	
	// Output histograms
	for key, h := range r.histos {
		sb.WriteString("# HELP ")
		sb.WriteString(key)
		sb.WriteString(" ")
		sb.WriteString(key)
		sb.WriteString("\n# TYPE ")
		sb.WriteString(key)
		sb.WriteString(" histogram\n")
		
		// Buckets
		buckets := h.Buckets()
		sortedBounds := make([]float64, 0, len(buckets))
		for bound := range buckets {
			sortedBounds = append(sortedBounds, bound)
		}
		sort.Float64s(sortedBounds)
		
		labels := labelsToPrometheus(r.histos[key].Labels())
		
		// Cumulative buckets (Observe stores per-bucket counts, not cumulative)
		var cumulative uint64
		for _, bound := range sortedBounds {
			if math.IsInf(bound, 1) {
				continue
			}
			cumulative += buckets[bound]
			sb.WriteString(key)
			sb.WriteString("_bucket{le=\"")
			sb.WriteString(fmt.Sprintf("%g", bound))
			sb.WriteString("\"")
			sb.WriteString(labels)
			sb.WriteString("} ")
			sb.WriteString(fmt.Sprintf("%d\n", cumulative))
		}
		sb.WriteString(key)
		sb.WriteString("_bucket{le=\"+Inf\"")
		sb.WriteString(labels)
		sb.WriteString("} ")
		sb.WriteString(fmt.Sprintf("%d\n", h.Count()))
		
		// Sum and count
		sb.WriteString(key)
		sb.WriteString("_sum")
		sb.WriteString(labels)
		sb.WriteString(" ")
		sb.WriteString(fmt.Sprintf("%g\n", h.Sum()))
		
		sb.WriteString(key)
		sb.WriteString("_count")
		sb.WriteString(labels)
		sb.WriteString(" ")
		sb.WriteString(fmt.Sprintf("%d\n", h.Count()))
	}
	
	// Output gauges
	for key, g := range r.gauges {
		sb.WriteString("# HELP ")
		sb.WriteString(key)
		sb.WriteString(" ")
		sb.WriteString(key)
		sb.WriteString("\n# TYPE ")
		sb.WriteString(key)
		sb.WriteString(" gauge\n")
		sb.WriteString(key)
		sb.WriteString(labelsToPrometheus(r.gauges[key].Labels()))
		sb.WriteString(" ")
		sb.WriteString(fmt.Sprintf("%g\n", g.Value()))
	}
	
	return sb.String()
}

// labelsToPrometheus converts labels to Prometheus format
func labelsToPrometheus(labels LabelMap) string {
	if labels == nil || len(labels) == 0 {
		return ""
	}
	
	var sb strings.Builder
	sb.WriteString("{")
	
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	
	first := true
	for _, k := range keys {
		if !first {
			sb.WriteString(",")
		}
		first = false
		sb.WriteString(k)
		sb.WriteString("=\"")
		sb.WriteString(escapePrometheusValue(labels[k]))
		sb.WriteString("\"")
	}
	
	sb.WriteString("}")
	return sb.String()
}

// escapePrometheusValue escapes special characters in label values
func escapePrometheusValue(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	return s
}

// CollectAndExport collects all metrics and exports them
func (e *PrometheusExporter) CollectAndExport() []byte {
	return []byte(e.registry.Output())
}
