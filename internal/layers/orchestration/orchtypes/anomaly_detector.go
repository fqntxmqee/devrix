package orchtypes

import (
	"context"
	"fmt"

	"github.com/devrix/devrix/internal/layers/orchestration/mups/learn"
)

// AnomalyCategory classifies the kind of anomaly detected by
// AnomalyDetector. Phase 6 PR-F1 (D7-S12-A41-T03) introduces 3 categories
// aligned with doc 35 §三.1 "anomaly detection" 4×4 quadrant.
type AnomalyCategory string

const (
	// AnomalyCategoryRate — frequency-based anomaly (e.g. "100 messages in 1s").
	AnomalyCategoryRate AnomalyCategory = "rate"

	// AnomalyCategoryPattern — pattern-based anomaly (e.g. repeated identical message).
	AnomalyCategoryPattern AnomalyCategory = "pattern"

	// AnomalyCategoryContent — content-based anomaly (e.g. hostile / off-policy).
	AnomalyCategoryContent AnomalyCategory = "content"
)

// Anomaly is an atomic anomaly record.
type Anomaly struct {
	Category AnomalyCategory
	Severity float64 // 0-1; higher = more severe
	Evidence string
}

// AnomalyReport aggregates anomaly detection results.
type AnomalyReport struct {
	// Anomalies — list of detected anomalies.
	Anomalies []Anomaly

	// TriggeredSystemAnomaly — true if any anomaly's Severity >= effective threshold.
	TriggeredSystemAnomaly bool

	// Severity — max severity across all anomalies (0 if none).
	Severity float64

	// Threshold — the effective threshold used (baseline or prior-adjusted).
	Threshold float64
}

// AnomalyDetector detects anomalies from a list of candidate anomalies.
// Phase 6 PR-F1 (D7-S12-A41-T03) introduces the baseline Detect and the
// prior-aware HistoricalDetector.DetectWithPrior.
//
// Concurrency-safe: no internal state mutated during Detect /
// DetectWithPrior.
type AnomalyDetector struct {
	baselineThreshold float64
}

// NewAnomalyDetector creates an AnomalyDetector with the default
// baseline threshold of 0.5.
func NewAnomalyDetector() *AnomalyDetector {
	return &AnomalyDetector{baselineThreshold: 0.5}
}

// HistoricalDetector is the history-aware detector. Phase 6 PR-F1
// reuses AnomalyDetector itself; HistoricalDetector is a thin alias for
// future history-aware extensions.
func (d *AnomalyDetector) HistoricalDetector() *AnomalyDetector {
	return d
}

// Detect is the baseline anomaly detection: any anomaly with
// Severity >= threshold triggers SystemAnomaly.
func (d *AnomalyDetector) Detect(_ context.Context, anomalies []Anomaly) (AnomalyReport, error) {
	return d.detectWithThreshold(anomalies, d.baselineThreshold, "baseline")
}

// DetectWithPrior applies AdaptivePrior.PriorBeta.Mean() to the threshold.
//
// Effective threshold = 0.5 * Mean (when prior != nil and Mean > 0).
//
// prior.Mean() higher (more trusted user) → threshold higher
// (more tolerant of mild anomalies).
// prior.Mean() lower (less trusted user) → threshold lower
// (more sensitive to anomalies).
// prior == nil or Mean == 0 (cold start) → baseline 0.5.
func (d *AnomalyDetector) DetectWithPrior(_ context.Context, anomalies []Anomaly, prior *learn.AdaptivePrior) (AnomalyReport, error) {
	threshold := d.baselineThreshold
	source := "baseline"
	if prior != nil {
		mean := prior.PriorBeta.Mean()
		if mean > 0 {
			threshold = 0.5 * mean
			source = fmt.Sprintf("prior.Mean=%.3f", mean)
		}
	}
	return d.detectWithThreshold(anomalies, threshold, source)
}

func (d *AnomalyDetector) detectWithThreshold(anomalies []Anomaly, threshold float64, source string) (AnomalyReport, error) {
	report := AnomalyReport{
		Anomalies: []Anomaly{},
		Threshold: threshold,
	}
	maxSev := 0.0
	triggered := false
	for _, a := range anomalies {
		if a.Severity < 0 || a.Severity > 1 {
			return AnomalyReport{}, fmt.Errorf("anomaly_detector: anomaly severity %.3f out of [0,1]", a.Severity)
		}
		report.Anomalies = append(report.Anomalies, a)
		if a.Severity > maxSev {
			maxSev = a.Severity
		}
		if a.Severity >= threshold {
			triggered = true
		}
	}
	report.Severity = maxSev
	report.TriggeredSystemAnomaly = triggered
	_ = source // reserved for future telemetry
	return report, nil
}
