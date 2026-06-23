package orchtypes

import (
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
)

// observationAdapter implements workmodel.AnomalyCategory by exposing
// the Category enum as a uint8. Defined here (not in workmodel) so the
// adapter can import orchtypes without creating a cycle: orchtypes →
// workmodel (for TaskStatus) is the existing one-way dependency, and the
// adapter satisfies the workmodel interface from the orchtypes side.
type observationAdapter struct {
	cat uint8
}

func (a observationAdapter) GetCategory() uint8 { return a.cat }

// AsWorkmodelAnomalyCategories converts an Observation slice into the
// workmodel.AnomalyCategory interface slice. This is the bridge that
// lets Phase 4 PR-D4's workmodel.SystemAnomalyAggregator evaluate
// orchtypes.UncertaintyReport.Anomalies without forcing workmodel to
// import orchtypes at the type level.
func AsWorkmodelAnomalyCategories(observations []Observation) []workmodel.AnomalyCategory {
	out := make([]workmodel.AnomalyCategory, 0, len(observations))
	for _, o := range observations {
		var cat uint8
		switch o.Category {
		case CatBusiness:
			cat = workmodel.CatBusinessValue
		case CatSystem:
			cat = workmodel.CatSystemValue
		default:
			cat = workmodel.CatBusinessValue // safe default
		}
		out = append(out, observationAdapter{cat: cat})
	}
	return out
}

// EvaluateSystemAnomaly is the canonical Phase 4 PR-D4 wiring entry
// point: given an UncertaintyReport, returns true iff CatSystem anomalies
// exceed the SystemAnomaly thresholds (default: AnomaliesCount ≥ 3 AND
// CatSystem/AnomaliesCount ≥ 0.5).
//
// This replaces the missing Phase 2 PR-A1 wiring where the FromVerifier
// `systemAnomaly` parameter was always false at the ObserveNode call site.
// The PR-D4 wiring propagates the SystemAnomaly decision end-to-end so
// the orchestrator surfaces distrust via UncertaintyCoord.Value=0.95.
func EvaluateSystemAnomaly(report UncertaintyReport) bool {
	return workmodel.EvaluateSystemAnomalyFromCategories(
		AsWorkmodelAnomalyCategories(report.Anomalies),
	)
}

// BuildUncertaintyCoordFromReport is the unified Observe → Verify → Coord
// wiring entry point. Phase 4 PR-D4 introduces this so the ObserveNode can
// produce a coord with SystemAnomaly correctly propagated to
// FromVerifierTyped (which forces Value=0.95 on SystemAnomaly=true).
//
// Parameters:
//
//	report   — UncertaintyReport from ObserveNode
//	verifier — workmodel.Verdict from the Verify sub-agent
//
// Returns UncertaintyCoord or error if the verifier Kind is unknown
// (forwarded from FromVerifierTyped fail-fast).
func BuildUncertaintyCoordFromReport(report UncertaintyReport, verifier workmodel.Verdict) (UncertaintyCoord, error) {
	systemAnomaly := EvaluateSystemAnomaly(report)
	return FromVerifierTyped(verifier.Kind, verifier.Confidence, verifier.Reason, systemAnomaly)
}
