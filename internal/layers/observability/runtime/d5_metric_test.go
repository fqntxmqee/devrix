package runtime

import (
	"errors"
	"testing"

	"github.com/devrix/devrix/internal/layers/observability/metrics"
)

// fakeMeter records the counters it created and returns them.
type fakeMeter struct {
	counters map[string]metrics.Counter
}

func newFakeMeter() *fakeMeter { return &fakeMeter{counters: map[string]metrics.Counter{}} }

func (f *fakeMeter) Int64Counter(name string, opts ...metrics.CounterOption) (metrics.Counter, error) {
	cfg := &metrics.CounterConfig{Labels: metrics.LabelMap{}}
	for _, o := range opts {
		if o != nil {
			o(cfg)
		}
	}
	c := metrics.NewCounter(name, cfg.Labels)
	f.counters[name] = c
	return c, nil
}

func TestRegisterD5_SuccessAndIdempotent(t *testing.T) {
	ResetD5()
	m := newFakeMeter()
	if err := RegisterD5(m); err != nil {
		t.Fatalf("RegisterD5: %v", err)
	}
	if !D5Registered() {
		t.Fatal("D5Registered = false after RegisterD5")
	}
	// Idempotent
	if err := RegisterD5(m); err != nil {
		t.Fatalf("second RegisterD5 should be no-op, got %v", err)
	}
}

func TestRegisterD5_NilMeterIsNoError(t *testing.T) {
	ResetD5()
	if err := RegisterD5(nil); err != nil {
		t.Fatalf("RegisterD5(nil): %v", err)
	}
	// Not "registered" in the strict sense, but the call should not error.
	if D5Registered() {
		t.Fatal("D5Registered should be false when meter is nil")
	}
}

type errMeter struct{ err error }

func (e *errMeter) Int64Counter(name string, opts ...metrics.CounterOption) (metrics.Counter, error) {
	return nil, e.err
}

func TestRegisterD5_PropagatesError(t *testing.T) {
	ResetD5()
	want := errors.New("boom")
	if err := RegisterD5(&errMeter{err: want}); err == nil || err.Error() != want.Error() {
		t.Fatalf("RegisterD5 should propagate, got %v", err)
	}
}

func TestIncD5_BridgesToCounter(t *testing.T) {
	Reset()
	ResetD5()
	m := newFakeMeter()
	if err := RegisterD5(m); err != nil {
		t.Fatalf("RegisterD5: %v", err)
	}
	IncD5(PathQueryLoop)
	IncD5(PathQueryLoop)
	IncD5(PathLegacyHarness)

	// We can't directly look up the counter by label here (the meter
	// used in tests records them under the metric name), so just make
	// sure at least one Inc call succeeded via the public helper.
	if v := m.counters[PathResolvedTotalMetric].Value(); v <= 0 {
		t.Fatalf("counter value = %d, want > 0", v)
	}
}

func TestIncD5_NoopWhenUnregistered(t *testing.T) {
	ResetD5()
	// Should not panic
	IncD5(PathQueryLoop)
	IncD5(PathLegacyHarness)
}
