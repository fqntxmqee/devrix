package runtime

import (
	"sync"

	"github.com/devrix/devrix/internal/layers/observability/metrics"
)

// Metric names exposed via D5 (observability).
//
//	runtime_path_resolved_total{path="query_loop|legacy_harness"}
//
// This metric is the D5 counterpart of the in-process PathCounters. The
// Probe layer (PathRegressionProbe) reads the in-process counters directly
// for assertions, but operators / dashboards consume the Prometheus-side
// counter via this name.
const (
	PathResolvedTotalMetric = "runtime_path_resolved_total"
	PathLabelKey            = "path"
	PathLabelQueryLoop      = "query_loop"
	PathLabelLegacyHarness  = "legacy_harness"
)

// metricRegistrar is the subset of metrics.Meter that we need to register
// the D5 counter. Defined as an interface so tests can supply a fake.
type metricRegistrar interface {
	Int64Counter(name string, opts ...metrics.CounterOption) (metrics.Counter, error)
}

// D5 handles registration of the runtime path_resolved counter with the
// process observability meter, and a thread-safe bridge from
// `Record(PathKind)` to the D5 Counter. We keep two separate
// `Int64Counter` instruments (one per `path` label value) because the
// in-process Meter uses LabelMap keyed by string and would otherwise
// overwrite on hot-reload.
type D5 struct {
	mu               sync.Mutex
	meter            metricRegistrar
	queryLoopCtr     metrics.Counter
	legacyHarnessCtr metrics.Counter
	registered       bool
}

var d5Singleton D5

// RegisterD5 attaches the D5 counter to the given meter. Idempotent: a
// second call with the same meter is a no-op. Returns an error if the
// meter fails to allocate the counters.
func RegisterD5(m metricRegistrar) error {
	d5Singleton.mu.Lock()
	defer d5Singleton.mu.Unlock()
	if d5Singleton.registered {
		// Already wired (likely by a second call in a test or by hot-reload).
		return nil
	}
	if m == nil {
		d5Singleton.registered = true // mark to avoid retry storms
		return nil
	}
	ql, err := m.Int64Counter(PathResolvedTotalMetric, metrics.WithLabels(metrics.LabelMap{
		PathLabelKey: PathLabelQueryLoop,
	}))
	if err != nil {
		return err
	}
	lh, err := m.Int64Counter(PathResolvedTotalMetric, metrics.WithLabels(metrics.LabelMap{
		PathLabelKey: PathLabelLegacyHarness,
	}))
	if err != nil {
		return err
	}
	d5Singleton.meter = m
	d5Singleton.queryLoopCtr = ql
	d5Singleton.legacyHarnessCtr = lh
	d5Singleton.registered = true
	return nil
}

// ResetD5 clears registration so tests can re-register with a different
// meter. Does not touch the in-process PathCounters (call Reset for
// that).
func ResetD5() {
	d5Singleton.mu.Lock()
	defer d5Singleton.mu.Unlock()
	d5Singleton.meter = nil
	d5Singleton.queryLoopCtr = nil
	d5Singleton.legacyHarnessCtr = nil
	d5Singleton.registered = false
}

// D5Registered reports whether RegisterD5 has succeeded. Useful for
// tests and for the integration in ContextEngine to avoid a noop
// registration on every Process() call.
func D5Registered() bool {
	d5Singleton.mu.Lock()
	defer d5Singleton.mu.Unlock()
	return d5Singleton.registered && d5Singleton.queryLoopCtr != nil
}

// IncD5 is the D5 bridge for `Record`. If the metric has not been
// registered (e.g. observability disabled), this is a no-op.
//
// We do NOT fail-soft at the call site: callers stay simple
// (`runtime.Record(path)`) and rely on this bridge to keep the D5
// counter in sync.
func IncD5(p PathKind) {
	d5Singleton.mu.Lock()
	ql, lh := d5Singleton.queryLoopCtr, d5Singleton.legacyHarnessCtr
	d5Singleton.mu.Unlock()
	if ql == nil || lh == nil {
		return
	}
	switch p {
	case PathQueryLoop:
		ql.Inc()
	case PathLegacyHarness:
		lh.Inc()
	}
}
