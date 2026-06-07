package metrics

import (
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/observability/settings"
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

func TestGauge_should_preserve_float_precision(t *testing.T) {
	g := NewGauge("sessions", nil)
	g.Set(1.5)
	g.Add(0.25)
	if got := g.Value(); got != 1.75 {
		t.Fatalf("expected 1.75, got %g", got)
	}
	g.Sub(0.75)
	if got := g.Value(); got != 1.0 {
		t.Fatalf("expected 1.0, got %g", got)
	}
}

func TestHistogram_should_not_double_accumulate_buckets(t *testing.T) {
	h := NewHistogram("latency", nil, []float64{0.1, 0.5, 1.0})
	h.Observe(0.05)
	h.Observe(0.3)

	buckets := h.Buckets()
	if buckets[0.1] != 1 {
		t.Fatalf("expected bucket 0.1=1, got %d", buckets[0.1])
	}
	if buckets[0.5] != 1 {
		t.Fatalf("expected bucket 0.5=1, got %d", buckets[0.5])
	}

	r := NewRegistry(nil, nil)
	_ = r.RegisterHistogram("latency", nil, h)
	out := r.Output()
	if strings.Count(out, `le="0.1"`) != 1 || strings.Count(out, `le="0.5"`) != 1 {
		t.Fatalf("unexpected prometheus bucket output: %s", out)
	}
	if !strings.Contains(out, `le="0.1"} 1`) {
		t.Fatalf("expected cumulative le=0.1 count 1, got: %s", out)
	}
	if !strings.Contains(out, `le="0.5"} 2`) {
		t.Fatalf("expected cumulative le=0.5 count 2, got: %s", out)
	}
	if !strings.Contains(out, `le="+Inf"} 2`) {
		t.Fatalf("expected +Inf count 2, got: %s", out)
	}
}

func TestInt64UpDownCounter_should_return_gauge(t *testing.T) {
	mp := NewMeterProvider(&settings.MetricsConfig{})
	m := mp.Meter("devrix")
	g, err := m.Int64UpDownCounter("active_sessions", WithLabels(LabelMap{"adapter": "cli"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	g.Set(2)
	g.Dec()
	if got := g.Value(); got != 1 {
		t.Fatalf("expected gauge value 1, got %g", got)
	}
}

func TestRegistry(t *testing.T) {
	r := NewRegistry(nil, nil)
	c := NewCounter("test", LabelMap{"provider": "openai"})
	if err := r.RegisterCounter("test", LabelMap{"provider": "openai"}, c); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
