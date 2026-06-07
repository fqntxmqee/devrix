package metrics

import (
	"sync"
)

// Gauge represents a gauge metric (瞬时值)
type Gauge interface {
	Set(value float64)
	Inc()
	Dec()
	Add(value float64)
	Sub(value float64)
	Value() float64
	Name() string
	Labels() LabelMap
	Type() MetricType
}

// gauge implements Gauge
type gauge struct {
	name   string
	labels LabelMap
	mu     sync.Mutex
	value  float64
}

// NewGauge creates a new gauge
func NewGauge(name string, labels LabelMap) Gauge {
	return &gauge{
		name:   name,
		labels: labels,
	}
}

// Set sets the gauge to a value
func (g *gauge) Set(value float64) {
	g.mu.Lock()
	g.value = value
	g.mu.Unlock()
}

// Inc increments the gauge by 1
func (g *gauge) Inc() {
	g.Add(1)
}

// Dec decrements the gauge by 1
func (g *gauge) Dec() {
	g.Sub(1)
}

// Add adds a value to the gauge
func (g *gauge) Add(value float64) {
	g.mu.Lock()
	g.value += value
	g.mu.Unlock()
}

// Sub subtracts a value from the gauge
func (g *gauge) Sub(value float64) {
	g.mu.Lock()
	g.value -= value
	g.mu.Unlock()
}

// Value returns the current value
func (g *gauge) Value() float64 {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.value
}

// Name returns the gauge name
func (g *gauge) Name() string {
	return g.name
}

// Labels returns the gauge labels
func (g *gauge) Labels() LabelMap {
	return g.labels
}

// Type returns the metric type
func (g *gauge) Type() MetricType {
	return MetricTypeGauge
}

// AsyncGauge is a gauge with callback on update
type AsyncGauge struct {
	gauge    Gauge
	onUpdate func(value float64)
}

// NewAsyncGauge creates a new async gauge
func NewAsyncGauge(name string, labels LabelMap, onUpdate func(value float64)) *AsyncGauge {
	return &AsyncGauge{
		gauge:    NewGauge(name, labels),
		onUpdate: onUpdate,
	}
}

// Set sets and notifies
func (g *AsyncGauge) Set(value float64) {
	g.gauge.Set(value)
	if g.onUpdate != nil {
		g.onUpdate(g.gauge.Value())
	}
}

// Inc increments and notifies
func (g *AsyncGauge) Inc() {
	g.gauge.Inc()
	if g.onUpdate != nil {
		g.onUpdate(g.gauge.Value())
	}
}

// Dec decrements and notifies
func (g *AsyncGauge) Dec() {
	g.gauge.Dec()
	if g.onUpdate != nil {
		g.onUpdate(g.gauge.Value())
	}
}

// Add adds and notifies
func (g *AsyncGauge) Add(value float64) {
	g.gauge.Add(value)
	if g.onUpdate != nil {
		g.onUpdate(g.gauge.Value())
	}
}

// Sub subtracts and notifies
func (g *AsyncGauge) Sub(value float64) {
	g.gauge.Sub(value)
	if g.onUpdate != nil {
		g.onUpdate(g.gauge.Value())
	}
}

// Value returns the current value
func (g *AsyncGauge) Value() float64 {
	return g.gauge.Value()
}
