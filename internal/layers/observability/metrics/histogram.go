package metrics

import (
	"math"
	"sync"
)

// Histogram represents a histogram metric
type Histogram interface {
	Observe(value float64)
	Count() uint64
	Sum() float64
	Avg() float64
	Buckets() map[float64]uint64
	Name() string
	Labels() LabelMap
	Type() MetricType
}

// histogram implements Histogram
type histogram struct {
	name    string
	labels  LabelMap
	bounds  []float64
	buckets map[float64]uint64
	count   uint64
	sum     float64
	mu      sync.Mutex
}

// NewHistogram creates a new histogram with the given bucket bounds
func NewHistogram(name string, labels LabelMap, bounds []float64) Histogram {
	buckets := make(map[float64]uint64)
	for _, b := range bounds {
		buckets[b] = 0
	}
	// Add +Inf bucket
	buckets[math.Inf(1)] = 0
	
	return &histogram{
		name:    name,
		labels:  labels,
		bounds:  bounds,
		buckets: buckets,
	}
}

// Observe records a value
func (h *histogram) Observe(value float64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.count++
	h.sum += value

	for _, bound := range h.bounds {
		if value <= bound {
			h.buckets[bound]++
		}
	}
}

// Count returns the total count
func (h *histogram) Count() uint64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.count
}

// Sum returns the sum
func (h *histogram) Sum() float64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.sum
}

// Avg returns the average
func (h *histogram) Avg() float64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.count == 0 {
		return 0
	}
	return h.sum / float64(h.count)
}

// Buckets returns a copy of bucket counts
func (h *histogram) Buckets() map[float64]uint64 {
	h.mu.Lock()
	defer h.mu.Unlock()

	result := make(map[float64]uint64, len(h.buckets))
	for k, v := range h.buckets {
		result[k] = v
	}
	return result
}

// Name returns the histogram name
func (h *histogram) Name() string {
	return h.name
}

// Labels returns the histogram labels
func (h *histogram) Labels() LabelMap {
	return h.labels
}

// Type returns the metric type
func (h *histogram) Type() MetricType {
	return MetricTypeHistogram
}

// DefaultHistogramBounds returns OTel recommended bucket bounds
func DefaultHistogramBounds() []float64 {
	return []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}
}

// LLMHistogramBounds returns bucket bounds suitable for LLM latency
func LLMHistogramBounds() []float64 {
	return []float64{0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0, 30.0, 60.0, 120.0}
}
