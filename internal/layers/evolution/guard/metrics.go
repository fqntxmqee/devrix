package guard

import (
	"strconv"
	"sync/atomic"

	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/observability/instrument/metrics"
)

// guardMetrics holds the OpenTelemetry instruments for the guard validator.
//
// PR-B (DM-20260621-011): renamed from orchMetrics to align with guard/ domain
// naming. All 6 OTel metric names also renamed from orch_* to guard_*. The
// underlying config.MetricProvider keeps backward compatibility shims so
// dashboards see both names during the v2.4 → v2.5 migration window.
type guardMetrics struct {
	decisionsTotal     metrics.Counter
	validationsTotal   metrics.Counter
	interventionsTotal metrics.Counter
	judgeLatency       metrics.Histogram
	observerActive     metrics.Gauge
	decisionsByStage   metrics.Counter

	// PR-A: H-3 silent swallow 修复（DM-20260621-011）
	// intervention.go 中 Wait/Tasks.Fail 失败时累加, 用于 SLO 报警
	waitFailed     atomic.Int64
	taskFailFailed atomic.Int64
}

func initGuardMetrics(obs *observability.Observability) *guardMetrics {
	m := &guardMetrics{}
	if obs == nil {
		return m
	}
	meter := obs.Meter()
	if meter == nil {
		return m
	}

	m.decisionsTotal, _ = meter.Int64Counter("guard_decisions_total",
		metrics.WithLabels(metrics.LabelMap{"category": "all", "risk_class": "all"}),
	)
	m.validationsTotal, _ = meter.Int64Counter("guard_validations_total",
		metrics.WithLabels(metrics.LabelMap{"result": "all", "from_judge": "all"}),
	)
	m.interventionsTotal, _ = meter.Int64Counter("guard_interventions_total",
		metrics.WithLabels(metrics.LabelMap{"action": "all"}),
	)
	m.judgeLatency, _ = meter.Float64Histogram("guard_judge_latency_seconds",
		metrics.WithHistogramLabels(metrics.LabelMap{"provider": "all", "model": "all"}),
	)
	m.observerActive, _ = meter.Int64UpDownCounter("guard_observer_active",
		metrics.WithLabels(metrics.LabelMap{"session_id": "all"}),
	)
	m.decisionsByStage, _ = meter.Int64Counter("guard_decisions_by_stage",
		metrics.WithLabels(metrics.LabelMap{"stage": "all"}),
	)

	// Signal that the observer metric system is initialized.
	if m.observerActive != nil {
		m.observerActive.Add(1)
	}

	return m
}

func (m *guardMetrics) recordDecision(category string, riskClass int) {
	if m == nil || m.decisionsTotal == nil {
		return
	}
	m.decisionsTotal.Labels()["category"] = category
	m.decisionsTotal.Labels()["risk_class"] = strconv.Itoa(riskClass)
	m.decisionsTotal.Add(1)
}

func (m *guardMetrics) recordValidation(valid bool, fromJudge bool) {
	if m == nil || m.validationsTotal == nil {
		return
	}
	m.validationsTotal.Labels()["result"] = strconv.FormatBool(valid)
	m.validationsTotal.Labels()["from_judge"] = strconv.FormatBool(fromJudge)
	m.validationsTotal.Add(1)
}

func (m *guardMetrics) recordIntervention(action string) {
	if m == nil || m.interventionsTotal == nil {
		return
	}
	m.interventionsTotal.Labels()["action"] = action
	m.interventionsTotal.Add(1)
}

func (m *guardMetrics) recordJudgeLatency(seconds float64) {
	if m == nil || m.judgeLatency == nil {
		return
	}
	m.judgeLatency.Observe(seconds)
}

func (m *guardMetrics) recordStage(stage string) {
	if m == nil || m.decisionsByStage == nil {
		return
	}
	m.decisionsByStage.Labels()["stage"] = stage
	m.decisionsByStage.Add(1)
}

func (m *guardMetrics) signalObserverActive() {
	if m == nil || m.observerActive == nil {
		return
	}
	m.observerActive.Add(1)
}

// recordWaitFailed increments the wait failure counter (nil-safe).
func (m *guardMetrics) recordWaitFailed() {
	if m == nil {
		return
	}
	m.waitFailed.Add(1)
}

// recordTaskFailFailed increments the task fail failure counter (nil-safe).
func (m *guardMetrics) recordTaskFailFailed() {
	if m == nil {
		return
	}
	m.taskFailFailed.Add(1)
}

// SnapshotWaitFailed returns the current wait failure count (nil-safe).
func (m *guardMetrics) SnapshotWaitFailed() int64 {
	if m == nil {
		return 0
	}
	return m.waitFailed.Load()
}

// SnapshotTaskFailFailed returns the current task fail failure count (nil-safe).
func (m *guardMetrics) SnapshotTaskFailFailed() int64 {
	if m == nil {
		return 0
	}
	return m.taskFailFailed.Load()
}

// orchMetrics is an alias for guardMetrics kept for backward compatibility.
//
// Deprecated: use guardMetrics. This alias will be removed in v2.5.0 (DM-20260621-011).
//go:deprecated
type orchMetrics = guardMetrics

// initOrchMetrics is the deprecated constructor for guardMetrics.
//
// Deprecated: use initGuardMetrics. This alias will be removed in v2.5.0 (DM-20260621-011).
//go:deprecated
func initOrchMetrics(obs *observability.Observability) *guardMetrics {
	return initGuardMetrics(obs)
}
