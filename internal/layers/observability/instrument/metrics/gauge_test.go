package metrics

import "testing"

// T: D5-S2-A01-T03
func TestGauge_should_set_inc_dec_precisely(t *testing.T) {
	g := NewGauge("sessions", nil)
	g.Set(10)
	g.Inc()
	g.Dec()
	g.Add(2.5)
	g.Sub(1.5)
	if got := g.Value(); got != 11.0 {
		t.Fatalf("expected 11.0, got %g", got)
	}
}

// T: D5-S2-A01-T03
func TestGauge_should_handle_negative_values(t *testing.T) {
	g := NewGauge("balance", nil)
	g.Set(-3)
	g.Sub(2)
	if got := g.Value(); got != -5 {
		t.Fatalf("expected -5, got %g", got)
	}
}
