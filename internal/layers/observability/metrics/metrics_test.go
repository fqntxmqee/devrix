package metrics

import (
	"math"
	"testing"
)

func TestCounter(t *testing.T) {
	c := NewCounter("test", nil)

	c.Add(10)
	if c.Value() != 10 {
		t.Errorf("expected 10, got %d", c.Value())
	}

	c.Inc()
	if c.Value() != 11 {
		t.Errorf("expected 11, got %d", c.Value())
	}

	c.Add(5)
	if c.Value() != 16 {
		t.Errorf("expected 16, got %d", c.Value())
	}
}

func TestHistogram(t *testing.T) {
	bounds := []float64{0.1, 0.5, 1.0, 5.0}
	h := NewHistogram("test", nil, bounds)

	h.Observe(0.05)
	h.Observe(0.3)
	h.Observe(0.8)
	h.Observe(3.0)
	h.Observe(10.0)

	if h.Count() != 5 {
		t.Errorf("expected count 5, got %d", h.Count())
	}

	if h.Sum() != 14.15 {
		t.Errorf("expected sum 14.15, got %f", h.Sum())
	}

	avg := h.Avg()
	if math.Abs(avg-2.83) > 0.01 {
		t.Errorf("expected avg ~2.83, got %f", avg)
	}
}

func TestGauge(t *testing.T) {
	g := NewGauge("test", nil)

	g.Set(100)
	if g.Value() != 100 {
		t.Errorf("expected 100, got %f", g.Value())
	}

	g.Inc()
	if g.Value() != 101 {
		t.Errorf("expected 101, got %f", g.Value())
	}

	g.Dec()
	if g.Value() != 100 {
		t.Errorf("expected 100, got %f", g.Value())
	}

	g.Add(50)
	if g.Value() != 150 {
		t.Errorf("expected 150, got %f", g.Value())
	}

	g.Sub(30)
	if g.Value() != 120 {
		t.Errorf("expected 120, got %f", g.Value())
	}
}

func TestRegistry(t *testing.T) {
	r := NewRegistry([]string{"provider", "model"}, []string{"secret"})

	c := NewCounter("test", LabelMap{"provider": "openai"})
	err := r.RegisterCounter("test", LabelMap{"provider": "openai"}, c)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Test blocklist
	err = r.RegisterCounter("blocked", LabelMap{"secret": "xxx"}, NewCounter("blocked", nil))
	if err == nil {
		t.Error("expected error for blocked label")
	}
}

func TestLabelValidation(t *testing.T) {
	r := NewRegistry([]string{"provider"}, []string{"blocked"})

	err := r.validateLabels(LabelMap{"provider": "openai"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	err = r.validateLabels(LabelMap{"blocked": "value"})
	if err == nil {
		t.Error("expected error for blocked label")
	}

	err = r.validateLabels(LabelMap{"unknown": "value"})
	if err == nil {
		t.Error("expected error for unknown label (allowlist active)")
	}
}
