package runtime

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/observability/instrument/metrics"
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
	IncRuntimeMetric(PathD7Turn)
	IncRuntimeMetric(PathD7Turn)
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
	IncRuntimeMetric(PathD7Turn)
	IncRuntimeMetric(PathLegacyHarness)
}


// captureSlog redirects slog.Default() to a buffer at WARN level, restoring
// the previous default at test cleanup. Used to assert DEPRECATED warnings.
func captureSlog(t *testing.T) (*bytes.Buffer, func()) {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})))
	return &buf, func() { slog.SetDefault(prev) }
}

// D5 v2.1 Terminal (DM-20260619-006): IncRuntimeMetric on PathLegacyHarness
// must emit a DEPRECATED slog.Warn so on-call can spot live stragglers.
func TestIncRuntimeMetric_LegacyHarness_LogsDeprecation(t *testing.T) {
	buf, restore := captureSlog(t)
	defer restore()
	ResetRuntimeMetric()
	m := newFakeMeter()
	if err := RegisterRuntimeMetric(m); err != nil {
		t.Fatalf("RegisterRuntimeMetric: %v", err)
	}
	IncRuntimeMetric(PathLegacyHarness)
	if !strings.Contains(buf.String(), "DEPRECATED") {
		t.Fatalf("expected DEPRECATED warning, got: %s", buf.String())
	}
	if !strings.Contains(buf.String(), "legacy_harness") {
		t.Fatalf("expected legacy_harness in warning, got: %s", buf.String())
	}
}

// IncRuntimeMetric on PathD7Turn must NOT emit the deprecation warning.
func TestIncRuntimeMetric_D7Turn_NoDeprecationLog(t *testing.T) {
	buf, restore := captureSlog(t)
	defer restore()
	ResetRuntimeMetric()
	m := newFakeMeter()
	if err := RegisterRuntimeMetric(m); err != nil {
		t.Fatalf("RegisterRuntimeMetric: %v", err)
	}
	IncRuntimeMetric(PathD7Turn)
	if strings.Contains(buf.String(), "DEPRECATED") {
		t.Fatalf("PathD7Turn must not log DEPRECATED, got: %s", buf.String())
	}
}
