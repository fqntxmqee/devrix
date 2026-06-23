package workmodel

import (
	"sync"
)

// Default SystemAnomaly thresholds. These are conservative defaults that
// surface systemic issues (CatSystem anomalies) without false positives
// from isolated business deviations.
const (
	DefaultAnomalyThreshold  = 3
	DefaultMinCatSystemRatio = 0.5
)

// AnomalyCategory is a minimal interface satisfied by any observation type
// that has a Category field. We use this (instead of importing
// orchtypes.Observation directly) to avoid an import cycle — orchtypes
// already imports workmodel for TaskStatus.
type AnomalyCategory interface {
	GetCategory() uint8
}

// CatBusinessValue and CatSystemValue mirror orchtypes.Category constants
// without importing them. They are uint8 to keep the dependency one-way.
const (
	CatBusinessValue uint8 = 0
	CatSystemValue   uint8 = 1
)

// SystemAnomalyConfig configures the SystemAnomalyAggregator trigger
// thresholds. The aggregator fires (returns true) when ALL conditions are
// met:
//
//   - len(anomalies) >= AnomalyThreshold
//   - CatSystemCount / AnomaliesCount >= MinCatSystemRatio
//
// Phase 4 PR-D4 (DM-20260623-002) introduces this so the ObserveNode can
// produce a SystemAnomaly bool flag that the Verify node propagates into
// UncertaintyCoord (forced Value=0.95 per Phase 2 PR-A1 FromVerifier).
type SystemAnomalyConfig struct {
	// AnomalyThreshold is the minimum number of anomalies (CatBusiness
	// + CatSystem) required for the aggregator to consider firing.
	// Default: 3.
	AnomalyThreshold int

	// MinCatSystemRatio is the minimum fraction of CatSystem anomalies
	// (vs total anomalies) required for the aggregator to fire. This
	// guards against false positives when CatBusiness dominates.
	// Default: 0.5.
	MinCatSystemRatio float64
}

// SystemAnomalyAggregator aggregates CatSystem anomalies into a single
// SystemAnomaly bool flag. The aggregator is stateless w.r.t. its inputs
// (each Evaluate call is independent), but it accumulates the count of
// CatSystem observations via RecordCatSystem for callers that want to
// track running totals across many reports.
type SystemAnomalyAggregator struct {
	cfg            SystemAnomalyConfig
	mu             sync.Mutex
	catSystemCount int
}

// NewSystemAnomalyAggregator creates a new aggregator. Zero-value config
// fields are replaced with defaults.
func NewSystemAnomalyAggregator(cfg SystemAnomalyConfig) *SystemAnomalyAggregator {
	if cfg.AnomalyThreshold == 0 {
		cfg.AnomalyThreshold = DefaultAnomalyThreshold
	}
	if cfg.MinCatSystemRatio == 0 {
		cfg.MinCatSystemRatio = DefaultMinCatSystemRatio
	}
	return &SystemAnomalyAggregator{cfg: cfg}
}

// Evaluate inspects a list of anomalies and returns true iff the system
// anomaly thresholds are met. Pure function w.r.t. the input (no side
// effects on the aggregator's internal counters).
//
// Accepts the AnomalyCategory interface so callers can pass orchtypes
// observations without creating a workmodel → orchtypes import cycle.
func (a *SystemAnomalyAggregator) Evaluate(anomalies []AnomalyCategory) bool {
	if len(anomalies) < a.cfg.AnomalyThreshold {
		return false
	}
	catSystemCount := 0
	for _, obs := range anomalies {
		if obs.GetCategory() == CatSystemValue {
			catSystemCount++
		}
	}
	return float64(catSystemCount)/float64(len(anomalies)) >= a.cfg.MinCatSystemRatio
}

// RecordCatSystem increments the aggregator's running CatSystem count.
// Useful for callers that want to track cumulative CatSystem signals
// across multiple reports (e.g. session-level dashboards).
func (a *SystemAnomalyAggregator) RecordCatSystem(count int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.catSystemCount += count
}

// CatSystemCount returns the cumulative CatSystem count.
func (a *SystemAnomalyAggregator) CatSystemCount() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.catSystemCount
}

// Reset clears the cumulative counters. Configuration is preserved.
func (a *SystemAnomalyAggregator) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.catSystemCount = 0
}

// EvaluateSystemAnomalyFromCategories is a stateless convenience wrapper
// around SystemAnomalyAggregator using the default thresholds. Suitable
// for one-shot evaluations where running counters are not needed.
func EvaluateSystemAnomalyFromCategories(anomalies []AnomalyCategory) bool {
	return NewSystemAnomalyAggregator(SystemAnomalyConfig{}).Evaluate(anomalies)
}
