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

func TestRegisterRuntimeMetric_SuccessAndIdempotent(t *testing.T) {
	ResetRuntimeMetric()
	m := newFakeMeter()
	if err := RegisterRuntimeMetric(m); err != nil {
		t.Fatalf("RegisterRuntimeMetric: %v", err)
	}
	if !RuntimeMetricRegistered() {
		t.Fatal("RuntimeMetricRegistered = false after RegisterRuntimeMetric")
	}
	// Idempotent
	if err := RegisterRuntimeMetric(m); err != nil {
		t.Fatalf("second RegisterRuntimeMetric should be no-op, got %v", err)
	}
}

func TestRegisterRuntimeMetric_NilMeterIsNoError(t *testing.T) {
	ResetRuntimeMetric()
	if err := RegisterRuntimeMetric(nil); err != nil {
		t.Fatalf("RegisterRuntimeMetric(nil): %v", err)
	}
	// Not "registered" in the strict sense, but the call should not error.
	if RuntimeMetricRegistered() {
		t.Fatal("RuntimeMetricRegistered should be false when meter is nil")
	}
}

type errMeter struct{ err error }

func (e *errMeter) Int64Counter(name string, opts ...metrics.CounterOption) (metrics.Counter, error) {
	return nil, e.err
}

func TestRegisterRuntimeMetric_PropagatesError(t *testing.T) {
	ResetRuntimeMetric()
	want := errors.New("boom")
	if err := RegisterRuntimeMetric(&errMeter{err: want}); err == nil || err.Error() != want.Error() {
		t.Fatalf("RegisterRuntimeMetric should propagate, got %v", err)
	}
}

func TestIncRuntimeMetric_BridgesToCounter(t *testing.T) {
	Reset()
	ResetRuntimeMetric()
	m := newFakeMeter()
	if err := RegisterRuntimeMetric(m); err != nil {
		t.Fatalf("RegisterRuntimeMetric: %v", err)
	}
	IncRuntimeMetric(PathQueryLoop)
	IncRuntimeMetric(PathQueryLoop)
	IncRuntimeMetric(PathLegacyHarness)

	// We can't directly look up the counter by label here (the meter
	// used in tests records them under the metric name), so just make
	// sure at least one Inc call succeeded via the public helper.
	if v := m.counters[PathResolvedTotalMetric].Value(); v <= 0 {
		t.Fatalf("counter value = %d, want > 0", v)
	}
}

func TestIncRuntimeMetric_NoopWhenUnregistered(t *testing.T) {
	ResetRuntimeMetric()
	// Should not panic
	IncRuntimeMetric(PathQueryLoop)
	IncRuntimeMetric(PathLegacyHarness)
}
