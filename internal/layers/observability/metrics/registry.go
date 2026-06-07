package metrics

import (
	"fmt"
	"sync"
)

// LabelMap is a map of label key-value pairs
type LabelMap map[string]string

// Metric represents a generic metric
type Metric interface {
	Name() string
	Labels() LabelMap
	Type() MetricType
}

// MetricType represents the type of metric
type MetricType int

const (
	MetricTypeCounter MetricType = iota
	MetricTypeHistogram
	MetricTypeGauge
)

func (m MetricType) String() string {
	switch m {
	case MetricTypeCounter:
		return "Counter"
	case MetricTypeHistogram:
		return "Histogram"
	case MetricTypeGauge:
		return "Gauge"
	default:
		return fmt.Sprintf("MetricType(%d)", m)
	}
}

// Registry maintains a collection of metrics
type Registry struct {
	mu        sync.RWMutex
	metrics   map[string]Metric
	counters  map[string]Counter
	histos   map[string]Histogram
	gauges   map[string]Gauge
	
	// Label validation
	allowlist []string
	blocklist []string
}

// NewRegistry creates a new metric registry
func NewRegistry(allowlist, blocklist []string) *Registry {
	return &Registry{
		metrics:  make(map[string]Metric),
		counters: make(map[string]Counter),
		histos:  make(map[string]Histogram),
		gauges:  make(map[string]Gauge),
		allowlist: allowlist,
		blocklist: blocklist,
	}
}

// validateLabels checks if labels are allowed
func (r *Registry) validateLabels(labels LabelMap) error {
	if labels == nil {
		return nil
	}
	
	for key := range labels {
		// Check blocklist
		for _, blocked := range r.blocklist {
			if key == blocked {
				return fmt.Errorf("label %q is blocked", key)
			}
		}
		
		// Check allowlist if not empty
		if len(r.allowlist) > 0 {
			allowed := false
			for _, allowedKey := range r.allowlist {
				if key == allowedKey {
					allowed = true
					break
				}
			}
			if !allowed {
				return fmt.Errorf("label %q is not in allowlist", key)
			}
		}
	}
	
	return nil
}

// metricKey generates a unique key for a metric
func metricKey(name string, labels LabelMap) string {
	if labels == nil || len(labels) == 0 {
		return name
	}
	
	key := name
	for _, k := range sortedKeys(labels) {
		key += "{" + k + "=" + labels[k] + "}"
	}
	return key
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// Simple sort (in production use sort.Strings)
	for i := 0; i < len(keys)-1; i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[i] > keys[j] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	return keys
}

// RegisterCounter registers a counter metric
func (r *Registry) RegisterCounter(name string, labels LabelMap, counter Counter) error {
	if err := r.validateLabels(labels); err != nil {
		return err
	}
	
	key := metricKey(name, labels)
	
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if _, exists := r.metrics[key]; exists {
		return fmt.Errorf("metric %s already registered", key)
	}
	
	r.counters[key] = counter
	r.metrics[key] = counter
	
	return nil
}

// GetCounter gets a counter by name and labels
func (r *Registry) GetCounter(name string, labels LabelMap) (Counter, bool) {
	key := metricKey(name, labels)
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	c, ok := r.counters[key]
	return c, ok
}

// RegisterHistogram registers a histogram metric
func (r *Registry) RegisterHistogram(name string, labels LabelMap, histo Histogram) error {
	if err := r.validateLabels(labels); err != nil {
		return err
	}
	
	key := metricKey(name, labels)
	
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if _, exists := r.metrics[key]; exists {
		return fmt.Errorf("metric %s already registered", key)
	}
	
	r.histos[key] = histo
	r.metrics[key] = histo
	
	return nil
}

// RegisterGauge registers a gauge metric
func (r *Registry) RegisterGauge(name string, labels LabelMap, gauge Gauge) error {
	if err := r.validateLabels(labels); err != nil {
		return err
	}
	
	key := metricKey(name, labels)
	
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if _, exists := r.metrics[key]; exists {
		return fmt.Errorf("metric %s already registered", key)
	}
	
	r.gauges[key] = gauge
	r.metrics[key] = gauge
	
	return nil
}

// List returns all registered metrics
func (r *Registry) List() []Metric {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	metrics := make([]Metric, 0, len(r.metrics))
	for _, m := range r.metrics {
		metrics = append(metrics, m)
	}
	return metrics
}

// Count returns the number of registered metrics
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.metrics)
}

// Reset resets all metrics to zero
func (r *Registry) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	for _, c := range r.counters {
		if resettable, ok := c.(*counter); ok {
			resettable.Reset()
		}
	}
}
