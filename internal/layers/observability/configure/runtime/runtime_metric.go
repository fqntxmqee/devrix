package runtime

import (
	"sync"

	"github.com/devrix/devrix/internal/layers/observability/instrument/metrics"
)

// Metric names exposed via the observability (D5) layer.
//
//	runtime_path_resolved_total{path="query_loop|legacy_harness"}
//
// This metric is the observability counterpart of the in-process
// PathCounters. The Probe layer (PathRegressionProbe) reads the
// in-process counters directly for assertions, but operators / dashboards
// consume the Prometheus-side counter via this name.
const (
	PathResolvedTotalMetric = "runtime_path_resolved_total"
	PathLabelKey            = "path"
	PathLabelQueryLoop      = "query_loop"
	PathLabelLegacyHarness  = "legacy_harness"
)

// metricRegistrar is the subset of metrics.Meter that we need to register
// the runtime path counter. Defined as an interface so tests can supply
// a fake.
type metricRegistrar interface {
	Int64Counter(name string, opts ...metrics.CounterOption) (metrics.Counter, error)
}

// RuntimeMetric handles registration of the runtime path_resolved counter
// with the process observability meter, and a thread-safe bridge from
// `Record(PathKind)` to the Counter. We keep two separate `Int64Counter`
// instruments (one per `path` label value) because the in-process Meter
// uses LabelMap keyed by string and would otherwise overwrite on
// hot-reload.
type RuntimeMetric struct {
	mu               sync.Mutex
	meter            metricRegistrar
	queryLoopCtr     metrics.Counter
	legacyHarnessCtr metrics.Counter
	registered       bool
}

var runtimeMetricSingleton RuntimeMetric

// RegisterRuntimeMetric attaches the runtime path counter to the given
// meter. Idempotent: a second call with the same meter is a no-op.
// Returns an error if the meter fails to allocate the counters.
func RegisterRuntimeMetric(m metricRegistrar) error {
	runtimeMetricSingleton.mu.Lock()
	defer runtimeMetricSingleton.mu.Unlock()
	if runtimeMetricSingleton.registered {
		// Already wired (likely by a second call in a test or by hot-reload).
		return nil
	}
	if m == nil {
		runtimeMetricSingleton.registered = true // mark to avoid retry storms
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
	runtimeMetricSingleton.meter = m
	runtimeMetricSingleton.queryLoopCtr = ql
	runtimeMetricSingleton.legacyHarnessCtr = lh
	runtimeMetricSingleton.registered = true
	return nil
}

// ResetRuntimeMetric clears registration so tests can re-register with a
// different meter. Does not touch the in-process PathCounters (call
// Reset for that).
func ResetRuntimeMetric() {
	runtimeMetricSingleton.mu.Lock()
	defer runtimeMetricSingleton.mu.Unlock()
	runtimeMetricSingleton.meter = nil
	runtimeMetricSingleton.queryLoopCtr = nil
	runtimeMetricSingleton.legacyHarnessCtr = nil
	runtimeMetricSingleton.registered = false
}

// RuntimeMetricRegistered reports whether RegisterRuntimeMetric has
// succeeded. Useful for tests and for the integration in ContextEngine
// to avoid a noop registration on every Process() call.
func RuntimeMetricRegistered() bool {
	runtimeMetricSingleton.mu.Lock()
	defer runtimeMetricSingleton.mu.Unlock()
	return runtimeMetricSingleton.registered && runtimeMetricSingleton.queryLoopCtr != nil
}

// IncRuntimeMetric is the bridge for `Record`. If the metric has not
// been registered (e.g. observability disabled), this is a no-op.
//
// We do NOT fail-soft at the call site: callers stay simple
// (`runtime.Record(path)`) and rely on this bridge to keep the
// observability counter in sync.
func IncRuntimeMetric(p PathKind) {
	runtimeMetricSingleton.mu.Lock()
	ql, lh := runtimeMetricSingleton.queryLoopCtr, runtimeMetricSingleton.legacyHarnessCtr
	runtimeMetricSingleton.mu.Unlock()
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
