package metrics

import (
	"sync/atomic"
)

// Counter represents a cumulative counter metric
type Counter interface {
	Add(value int64)
	Inc()
	Value() int64
	Name() string
	Labels() LabelMap
	Type() MetricType
}

// counter implements Counter
type counter struct {
	name   string
	labels LabelMap
	value  int64
}

// NewCounter creates a new counter
func NewCounter(name string, labels LabelMap) Counter {
	return &counter{
		name:   name,
		labels: labels,
		value:  0,
	}
}

// Add adds a value to the counter
func (c *counter) Add(value int64) {
	atomic.AddInt64(&c.value, value)
}

// Inc increments the counter by 1
func (c *counter) Inc() {
	atomic.AddInt64(&c.value, 1)
}

// Value returns the current value
func (c *counter) Value() int64 {
	return atomic.LoadInt64(&c.value)
}

// Reset resets the counter to zero
func (c *counter) Reset() {
	atomic.StoreInt64(&c.value, 0)
}

// Name returns the counter name
func (c *counter) Name() string {
	return c.name
}

// Labels returns the counter labels
func (c *counter) Labels() LabelMap {
	return c.labels
}

// Type returns the metric type
func (c *counter) Type() MetricType {
	return MetricTypeCounter
}

// AsyncCounter is a thread-safe counter with callback
type AsyncCounter struct {
	counter *counter
	onUpdate func(value int64)
}

// NewAsyncCounter creates a new async counter
func NewAsyncCounter(name string, labels LabelMap, onUpdate func(value int64)) *AsyncCounter {
	return &AsyncCounter{
		counter: &counter{
			name:   name,
			labels: labels,
			value:  0,
		},
		onUpdate: onUpdate,
	}
}

// Add adds a value and calls the callback
func (c *AsyncCounter) Add(value int64) {
	c.counter.Add(value)
	if c.onUpdate != nil {
		c.onUpdate(c.counter.Value())
	}
}

// Inc increments and calls the callback
func (c *AsyncCounter) Inc() {
	c.counter.Inc()
	if c.onUpdate != nil {
		c.onUpdate(c.counter.Value())
	}
}

// Value returns the current value
func (c *AsyncCounter) Value() int64 {
	return c.counter.Value()
}
