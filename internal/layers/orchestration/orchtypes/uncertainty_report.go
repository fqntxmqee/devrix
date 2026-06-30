package orchtypes

import (
	"fmt"
	"sort"
	"time"
)

// QuantizedIntent is a compact summary of intent classification produced
// upstream. It is embedded in UncertaintyReport for downstream Plan nodes.
// Phase 2 PR-A2 will produce this via IntentQuantizer; here we keep a small
// stub struct so Phase 2 PR-A1 compiles in isolation.
//
// Kind is typed as IntentKind (not plain string) so PR-A2's IntentQuantizer
// can attach the result without a string↔IntentKind translation layer. The
// JSON wire format is preserved because IntentKind is string-aliased —
// "kind": "fast" still round-trips identically.
type QuantizedIntent struct {
	Kind       IntentKind `json:"kind"`
	Confidence float64    `json:"confidence"`
	Reason     string     `json:"reason,omitempty"`
	Rounds     int        `json:"rounds"`
	Source     string     `json:"source,omitempty"`
}

// AdaptivePrior is a Bayesian prior over a target's reputation/score. The
// real type is defined in Phase 5 PR-E2 (learn/adaptive_prior.go); for
// Phase 2 we keep a placeholder so that UncertaintyReport.Prior can be
// nil-safe. The real type replaces this in Phase 5 without changing the
// UncertaintyReport field type.
//
// NOTE: do not confuse with workmodel.AdaptiveThreshold (Phase 1) — that
// one is a stateless threshold map, not a Bayesian prior.
type AdaptivePrior struct {
	Score      float64   `json:"score"`
	Confidence float64   `json:"confidence"`
	Source     string    `json:"source"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// UncertaintyReport aggregates Observations emitted by ObserveNode. It is
// partitioned by Category (business vs system) and tagged with an Overall
// strength computed over business observations only.
type UncertaintyReport struct {
	SessionID            string           `json:"session_id"`
	Observations         []Observation    `json:"observations"`
	BusinessObservations []Observation    `json:"business_observations"`
	SystemObservations   []Observation    `json:"system_observations"`
	Anomalies            []Observation    `json:"anomalies"`
	Overall              float64          `json:"overall"`
	QuantizedIntent      *QuantizedIntent `json:"quantized_intent,omitempty"`
	Prior                *AdaptivePrior   `json:"prior,omitempty"`
	CreatedAt            time.Time        `json:"created_at"`
}

// NewUncertaintyReport partitions observations by Category and computes the
// business-only Overall strength. Anomalies are those ObsDeviation with
// CatSystem; this keeps the business path clean.
func NewUncertaintyReport(sessionID string, observations []Observation) (UncertaintyReport, error) {
	if sessionID == "" {
		return UncertaintyReport{}, ErrUncertaintyReportSessionIDRequired
	}
	r := UncertaintyReport{
		SessionID:    sessionID,
		Observations: append([]Observation(nil), observations...),
		CreatedAt:    time.Now(),
	}
	if err := r.Partition(); err != nil {
		return UncertaintyReport{}, err
	}
	return r, nil
}

// obsUncertaintyAnomalyThreshold — DM-20260630-011.
//
// ObsUncertainty observations with strength ≥ this threshold (and
// Category=CatSystem) are surfaced as anomalies so downstream
// DetectTaskIncomplete / DetectEmptyConclusion detectors can fire.
// Set to 0.7 (not a magic threshold; corresponds to the LLM classifier
// confidence band documented in design.md §① AC4).
const obsUncertaintyAnomalyThreshold = 0.7

// Partition splits Observations into BusinessObservations and
// SystemObservations based on Category. It also extracts Anomalies
// (CatSystem + ObsDeviation, OR CatSystem + ObsUncertainty with strength
// ≥ obsUncertaintyAnomalyThreshold) into a separate slice. Returns an
// error if the partition invariant is violated (defensive — should never
// fire with well-formed inputs).
//
// DM-20260630-011 (devrix-session-conclusion-completeness) — ObsUncertainty
// 兜底: prior implementation only surfaced ObsDeviation as anomalies.
// LLM-driven uncertainty signals (e.g. "task may not be a real review"
// or "no concrete findings produced") were silently dropped, blocking
// the DetectTaskIncomplete detector (executionflow/verify/anomaly.go).
// The 0.7 strength threshold matches the rule-based detector at the
// same layer so downstream consumers see a consistent noise floor.
func (r *UncertaintyReport) Partition() error {
	r.BusinessObservations = r.BusinessObservations[:0]
	r.SystemObservations = r.SystemObservations[:0]
	r.Anomalies = r.Anomalies[:0]
	for _, o := range r.Observations {
		switch o.Category {
		case CatSystem:
			r.SystemObservations = append(r.SystemObservations, o)
			if o.Kind == ObsDeviation {
				r.Anomalies = append(r.Anomalies, o)
			} else if o.Kind == ObsUncertainty && o.Strength >= obsUncertaintyAnomalyThreshold {
				// DM-20260630-011 AC4: surface high-strength system
				// uncertainty signals as anomalies. Defensive guard
				// prevents non-CatSystem obs from entering via the
				// wrong branch (shouldn't happen given the case label).
				r.Anomalies = append(r.Anomalies, o)
			}
		case CatBusiness:
			r.BusinessObservations = append(r.BusinessObservations, o)
		default:
			return fmt.Errorf("orchtypes: %w: %d", ErrObservationUnknownCategory, o.Category)
		}
	}
	if len(r.BusinessObservations)+len(r.SystemObservations) != len(r.Observations) {
		return NewUncertaintyReportPartitionInvariantError()
	}
	// Keep Overall in lockstep with the partition so callers that mutate
	// Observations directly (e.g. via AddObservation) don't see stale data.
	// NaN fallback to 0.5 (cold-start neutral) matches UncertaintyCoord.Value's
	// cold-start default — keeps semantics consistent downstream.
	r.Overall = clamp01Float(r.ComputeOverallStrength(), 0.5)
	return nil
}

// ComputeOverallStrength returns the mean of business observation strengths.
// System observations (CatSystem) are explicitly excluded to avoid polluting
// the business path with infrastructure noise. Returns 0.5 (cold-start
// neutral) when there are no business observations.
func (r *UncertaintyReport) ComputeOverallStrength() float64 {
	if len(r.BusinessObservations) == 0 {
		return 0.5
	}
	sum := 0.0
	for _, o := range r.BusinessObservations {
		sum += o.Strength
	}
	return sum / float64(len(r.BusinessObservations))
}

// FilterByKind returns all observations matching the given Kind, scanning
// the full set (NOT just BusinessObservations). This is intentional:
// callers may want to know about a system-level ObsUncertainty even when
// they are mostly operating on the business path.
func (r *UncertaintyReport) FilterByKind(k ObservationKind) []Observation {
	var out []Observation
	for _, o := range r.Observations {
		if o.Kind == k {
			out = append(out, o)
		}
	}
	return out
}

// FilterByCategory returns all observations matching the given Category.
func (r *UncertaintyReport) FilterByCategory(c Category) []Observation {
	switch c {
	case CatSystem:
		return append([]Observation(nil), r.SystemObservations...)
	default:
		return append([]Observation(nil), r.BusinessObservations...)
	}
}

// AddObservation returns a NEW UncertaintyReport containing the added
// observation. The receiver is not mutated. Partition and Overall are
// re-computed.
func (r UncertaintyReport) AddObservation(o Observation) (UncertaintyReport, error) {
	r.Observations = append(r.Observations, o)
	if err := r.Partition(); err != nil {
		return r, err
	}
	return r, nil
}

// SetQuantizedIntent returns a NEW report with the intent attached.
func (r UncertaintyReport) SetQuantizedIntent(qi *QuantizedIntent) UncertaintyReport {
	r.QuantizedIntent = qi
	return r
}

// SetPrior returns a NEW report with the prior attached.
func (r UncertaintyReport) SetPrior(p *AdaptivePrior) UncertaintyReport {
	r.Prior = p
	return r
}

// Validate ensures basic invariants. Useful as a guard before passing the
// report downstream to Plan.
func (r UncertaintyReport) Validate() error {
	if r.SessionID == "" {
		return ErrUncertaintyReportSessionIDRequired
	}
	for i, o := range r.Observations {
		if err := o.Validate(); err != nil {
			return fmt.Errorf("orchtypes: Observations[%d]: %w", i, err)
		}
	}
	return nil
}

// SortedObservationsByStrength returns observations sorted by Strength DESC.
// Useful for the Plan node when picking top-K strongest signals.
func (r *UncertaintyReport) SortedObservationsByStrength() []Observation {
	out := append([]Observation(nil), r.Observations...)
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Strength > out[j].Strength
	})
	return out
}
