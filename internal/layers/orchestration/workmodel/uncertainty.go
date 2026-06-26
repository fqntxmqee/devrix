package workmodel

import "math"

// UncertaintyWeights for anchor composition (AC27).
var UncertaintyWeights = struct {
	Historical, Structural, LLMClaim, Evidence float64
}{
	Historical: 0.4,
	Structural: 0.3,
	LLMClaim:   0.2,
	Evidence:   0.1,
}

// ChildOutcomeStats summarizes child terminal states for parent re-evaluation.
type ChildOutcomeStats struct {
	Total     int
	Completed int
	Failed    int
	Running   int
}

// ComputeUncertainty blends historical, structural, LLM claim, and evidence signals.
// When evidenceCount is zero and llmClaim is zero, falls back to historical+structural only.
func ComputeUncertainty(item *WorkItem, stats ChildOutcomeStats, llmClaim float64, evidenceCount int) float64 {
	if item == nil {
		return 0
	}
	historical := historicalUncertainty(stats)
	structural := structuralUncertainty(item, stats)
	evidence := evidenceScore(evidenceCount)

	if evidenceCount == 0 && llmClaim == 0 {
		return clamp01(historical + structural*0.5)
	}
	u := UncertaintyWeights.Historical*historical +
		UncertaintyWeights.Structural*structural +
		UncertaintyWeights.LLMClaim*clamp01(llmClaim) +
		UncertaintyWeights.Evidence*evidence
	return clamp01(u)
}

func historicalUncertainty(stats ChildOutcomeStats) float64 {
	if stats.Total == 0 {
		return 0.5
	}
	failRate := float64(stats.Failed) / float64(stats.Total)
	if stats.Running > 0 {
		return clamp01(0.3 + failRate*0.4)
	}
	return clamp01(failRate)
}

func structuralUncertainty(item *WorkItem, stats ChildOutcomeStats) float64 {
	base := float64(len(item.BlockedBy)) * 0.08
	if stats.Total > 0 && stats.Running > 0 {
		base += 0.15
	}
	return clamp01(base)
}

func evidenceScore(count int) float64 {
	if count <= 0 {
		return 0
	}
	return clamp01(1.0 - 1.0/float64(count+1))
}

// UnifiedUncertaintyInput feeds the G2 unified uncertainty formula (design §2.3).
type UnifiedUncertaintyInput struct {
	WilsonLower       float64
	ChildStats        ChildOutcomeStats
	VerdictConfidence float64
	EvidenceCount     int
}

// ComputeUnifiedUncertainty merges MUPS reputation and WorkTree structural signals.
func ComputeUnifiedUncertainty(in UnifiedUncertaintyInput) float64 {
	historical := historicalUncertainty(in.ChildStats)
	evidence := evidenceScore(in.EvidenceCount)
	wilson := clamp01(1 - in.WilsonLower)
	conf := clamp01(1 - in.VerdictConfidence)
	u := 0.35*wilson +
		0.25*historical +
		0.25*conf +
		0.15*evidence
	return clamp01(u)
}

func clamp01(v float64) float64 {
	return math.Max(0, math.Min(1, v))
}

// AdaptiveThreshold holds self-evolving uncertainty threshold state (Phase 8 baseline).
type AdaptiveThreshold struct {
	GlobalDefault float64
	PerUser       map[string]float64
}

// ThresholdFor returns the effective threshold with cold-start fallback (AC51).
func (a *AdaptiveThreshold) ThresholdFor(userID string) float64 {
	if a == nil {
		return 0.6
	}
	if v, ok := a.PerUser[userID]; ok && v > 0 {
		return v
	}
	if a.GlobalDefault > 0 {
		return a.GlobalDefault
	}
	return 0.6
}

// UpdateWithHysteresis adjusts threshold only when delta > 0.1 for 3 consecutive sessions (AC52).
func (a *AdaptiveThreshold) UpdateWithHysteresis(userID string, observed float64, consecutiveSameDirection int) {
	if a == nil {
		return
	}
	if a.PerUser == nil {
		a.PerUser = make(map[string]float64)
	}
	cur := a.ThresholdFor(userID)
	delta := observed - cur
	if math.Abs(delta) <= 0.1 || consecutiveSameDirection < 3 {
		return
	}
	a.PerUser[userID] = clamp01(cur + delta*0.1)
}
