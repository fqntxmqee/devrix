package metrics

import (
	"sync/atomic"
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
	value  uint64 // stored as integer representation
}

// NewGauge creates a new gauge
func NewGauge(name string, labels LabelMap) Gauge {
	return &gauge{
		name:   name,
		labels: labels,
		value:  0,
	}
}

// Set sets the gauge to a value
func (g *gauge) Set(value float64) {
	atomic.StoreUint64(&g.value, float64ToUint64(value))
}

// Inc increments the gauge by 1
func (g *gauge) Inc() {
	atomic.AddUint64(&g.value, 1)
}

// Dec decrements the gauge by 1
func (g *gauge) Dec() {
	atomic.AddUint64(&g.value, ^uint64(0)) // subtract 1 using two's complement
}

// Add adds a value to the gauge
func (g *gauge) Add(value float64) {
	atomic.AddUint64(&g.value, float64ToUint64(value))
}

// Sub subtracts a value from the gauge
func (g *gauge) Sub(value float64) {
	atomic.AddUint64(&g.value, float64ToUint64(-value))
}

// Value returns the current value
func (g *gauge) Value() float64 {
	return uint64ToFloat64(atomic.LoadUint64(&g.value))
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

// float64ToUint64 converts float64 to uint64 for atomic operations
func float64ToUint64(f float64) uint64 {
	return uint64(int64(f + (1 << 63)))
}

// uint64ToFloat64 converts uint64 back to float64
func uint64ToFloat64(u uint64) float64 {
	return float64(int64(u)) - (1 << 63)
}

// AsyncGauge is a gauge with callback on update
type AsyncGauge struct {
	gauge    *gauge
	onUpdate func(value float64)
}

// NewAsyncGauge creates a new async gauge
func NewAsyncGauge(name string, labels LabelMap, onUpdate func(value float64)) *AsyncGauge {
	return &AsyncGauge{
		gauge: &gauge{
			name:   name,
			labels: labels,
			value:  0,
		},
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
