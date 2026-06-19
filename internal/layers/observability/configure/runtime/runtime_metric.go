package runtime

import (
	"sync"

	"github.com/devrix/devrix/internal/layers/observability/instrument/metrics"
)

const (
	PathResolvedTotalMetric = "runtime_path_resolved_total"
	PathLabelKey            = "path"
	PathLabelD7Turn         = "d7_turn"
	PathLabelLegacyHarness  = "legacy_harness"
)

type metricRegistrar interface {
	Int64Counter(name string, opts ...metrics.CounterOption) (metrics.Counter, error)
}

type RuntimeMetric struct {
	mu               sync.Mutex
	meter            metricRegistrar
	d7TurnCtr        metrics.Counter
	legacyHarnessCtr metrics.Counter
	registered       bool
}

var runtimeMetricSingleton RuntimeMetric

func RegisterRuntimeMetric(m metricRegistrar) error {
	runtimeMetricSingleton.mu.Lock()
	defer runtimeMetricSingleton.mu.Unlock()
	if runtimeMetricSingleton.registered {
		return nil
	}
	if m == nil {
		runtimeMetricSingleton.registered = true
		return nil
	}
	d7, err := m.Int64Counter(PathResolvedTotalMetric, metrics.WithLabels(metrics.LabelMap{
		PathLabelKey: PathLabelD7Turn,
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
	runtimeMetricSingleton.d7TurnCtr = d7
	runtimeMetricSingleton.legacyHarnessCtr = lh
	runtimeMetricSingleton.registered = true
	return nil
}

func ResetRuntimeMetric() {
	runtimeMetricSingleton.mu.Lock()
	defer runtimeMetricSingleton.mu.Unlock()
	runtimeMetricSingleton.meter = nil
	runtimeMetricSingleton.d7TurnCtr = nil
	runtimeMetricSingleton.legacyHarnessCtr = nil
	runtimeMetricSingleton.registered = false
}

func RuntimeMetricRegistered() bool {
	runtimeMetricSingleton.mu.Lock()
	defer runtimeMetricSingleton.mu.Unlock()
	return runtimeMetricSingleton.registered && runtimeMetricSingleton.d7TurnCtr != nil
}

func IncRuntimeMetric(p PathKind) {
	runtimeMetricSingleton.mu.Lock()
	d7, lh := runtimeMetricSingleton.d7TurnCtr, runtimeMetricSingleton.legacyHarnessCtr
	runtimeMetricSingleton.mu.Unlock()
	if d7 == nil || lh == nil {
		return
	}
	switch p {
	case PathD7Turn:
		d7.Inc()
	case PathLegacyHarness:
		lh.Inc()
	}
}
