package guard

import (
	"strconv"

	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/observability/instrument/metrics"
)

// orchMetrics holds the OpenTelemetry instruments for the orchestration validator.
type orchMetrics struct {
	decisionsTotal     metrics.Counter
	validationsTotal   metrics.Counter
	interventionsTotal metrics.Counter
	judgeLatency       metrics.Histogram
	observerActive     metrics.Gauge
	decisionsByStage   metrics.Counter
}

func initOrchMetrics(obs *observability.Observability) *orchMetrics {
	m := &orchMetrics{}
	if obs == nil {
		return m
	}
	meter := obs.Meter()
	if meter == nil {
		return m
	}

	m.decisionsTotal, _ = meter.Int64Counter("orch_decisions_total",
		metrics.WithLabels(metrics.LabelMap{"category": "all", "risk_class": "all"}),
	)
	m.validationsTotal, _ = meter.Int64Counter("orch_validations_total",
		metrics.WithLabels(metrics.LabelMap{"result": "all", "from_judge": "all"}),
	)
	m.interventionsTotal, _ = meter.Int64Counter("orch_interventions_total",
		metrics.WithLabels(metrics.LabelMap{"action": "all"}),
	)
	m.judgeLatency, _ = meter.Float64Histogram("orch_judge_latency_seconds",
		metrics.WithHistogramLabels(metrics.LabelMap{"provider": "all", "model": "all"}),
	)
	m.observerActive, _ = meter.Int64UpDownCounter("orch_observer_active",
		metrics.WithLabels(metrics.LabelMap{"session_id": "all"}),
	)
	m.decisionsByStage, _ = meter.Int64Counter("orch_decisions_by_stage",
		metrics.WithLabels(metrics.LabelMap{"stage": "all"}),
	)

	// Signal that the observer metric system is initialized.
	if m.observerActive != nil {
		m.observerActive.Add(1)
	}

	return m
}

func (m *orchMetrics) recordDecision(category string, riskClass int) {
	if m == nil || m.decisionsTotal == nil {
		return
	}
	m.decisionsTotal.Labels()["category"] = category
	m.decisionsTotal.Labels()["risk_class"] = strconv.Itoa(riskClass)
	m.decisionsTotal.Add(1)
}

func (m *orchMetrics) recordValidation(valid bool, fromJudge bool) {
	if m == nil || m.validationsTotal == nil {
		return
	}
	m.validationsTotal.Labels()["result"] = strconv.FormatBool(valid)
	m.validationsTotal.Labels()["from_judge"] = strconv.FormatBool(fromJudge)
	m.validationsTotal.Add(1)
}

func (m *orchMetrics) recordIntervention(action string) {
	if m == nil || m.interventionsTotal == nil {
		return
	}
	m.interventionsTotal.Labels()["action"] = action
	m.interventionsTotal.Add(1)
}

func (m *orchMetrics) recordJudgeLatency(seconds float64) {
	if m == nil || m.judgeLatency == nil {
		return
	}
	m.judgeLatency.Observe(seconds)
}

func (m *orchMetrics) recordStage(stage string) {
	if m == nil || m.decisionsByStage == nil {
		return
	}
	m.decisionsByStage.Labels()["stage"] = stage
	m.decisionsByStage.Add(1)
}

func (m *orchMetrics) signalObserverActive() {
	if m == nil || m.observerActive == nil {
		return
	}
	m.observerActive.Add(1)
}
