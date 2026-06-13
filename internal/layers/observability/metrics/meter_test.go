package metrics

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/observability/settings"
)

// T: D5-S2-A01-T05
func TestMeter_Int64UpDownCounter_should_behave_as_gauge(t *testing.T) {
	mp := NewMeterProvider(&settings.MetricsConfig{})
	m := mp.Meter("devrix")
	g, err := m.Int64UpDownCounter("active_sessions", WithLabels(LabelMap{"adapter": "cli"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if g.Type() != MetricTypeGauge {
		t.Fatalf("expected gauge type, got %v", g.Type())
	}
	g.Set(3)
	g.Sub(5)
	if got := g.Value(); got != -2 {
		t.Fatalf("expected -2, got %g", got)
	}
}
