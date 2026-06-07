package metrics

import (
	"testing"
)

func TestCounter(t *testing.T) {
	c := NewCounter("test", nil)
	c.Add(10)
	if c.Value() != 10 {
		t.Errorf("expected 10, got %d", c.Value())
	}
}

func TestHistogram(t *testing.T) {
	h := NewHistogram("test", nil, []float64{0.1, 0.5, 1.0})
	h.Observe(0.05)
	h.Observe(0.3)
	h.Observe(0.8)

	if h.Count() != 3 {
		t.Errorf("expected count 3, got %d", h.Count())
	}
}

func TestRegistry(t *testing.T) {
	r := NewRegistry(nil, nil)
	c := NewCounter("test", LabelMap{"provider": "openai"})
	if err := r.RegisterCounter("test", LabelMap{"provider": "openai"}, c); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
