// Package sessionorchestrator — convergence_metric.go
//
// DM-20260705-010 (devrix-d7-mups-frame-delta-closure) Phase 3 T12:
// ComputeConvergenceMetric — deterministic per-sub-turn convergence
// measurement bridging Execute → next-round Observe (Markov 链闭环).
//
// 设计契约 (design.md §4.1 + §6.1):
//   - ConvergenceMetric 是纯值对象 (3 字段, 无 method mutation), 由
//     ComputeConvergenceMetric 纯函数返回.
//   - Deterministic only — 0 LLM invocation. 走工具结果 diff + claim 数 +
//     obs_uncertainty 残量 (subTurns 记录首/末轮 gap 计数).
//   - subTurns 为空 → 返回零值 ConvergenceMetric{} + slog.Warn, 不阻塞 sub-turn
//     (design.md §3.3 fallback table AC4 T01).
//
// SLA: < 1ms / sub-turn (design.md §5.1 节点 SLA).
package sessionorchestrator

import (
	"log/slog"
	"strings"

	"github.com/devrix/devrix/internal/layers/orchestration/interfaces"
)

// SubTurnRecord is the per-sub-turn trace record used as input to
// ComputeConvergenceMetric. Built by item_pipeline.go after each Execute
// sub-turn from the tool-result diff + ResolutionClaim count + residual
// obs_uncertainty count (all deterministic, 0 LLM).
//
// DM-20260705-010 Phase 3 T12.
type SubTurnRecord struct {
	// InitialObsGaps is the number of open observation gaps (obs_uncertainty
	// 残量) at the START of this sub-turn.
	InitialObsGaps int
	// ResidualObsGaps is the number of open observation gaps remaining at the
	// END of this sub-turn (after tool_result + claim resolution).
	ResidualObsGaps int
	// PromptContainsPlanFrameDelta is true iff this sub-turn's Execute
	// system_prompt actually carried the injected Plan frame delta (not the
	// baseline fallback). Feeds ConvergenceMetric.FrameDeltaConsumed.
	PromptContainsPlanFrameDelta bool
}

// ConvergenceMetric is the deterministic per-round convergence measurement.
// Immutable value object — ComputeConvergenceMetric is the only constructor.
//
// Field mapping (design.md §4.4 span attributes):
//   - UncertaintyReductionRate → span tag uncertainty_reduction_rate (AC4, AC7)
//   - ObservedGapsClosedCount  → span tag observed_gaps_closed_count (AC4)
//   - FrameDeltaConsumed       → span tag frame_delta_consumed (AC4)
type ConvergenceMetric struct {
	// UncertaintyReductionRate is (initialGaps - residualGaps) / initialGaps,
	// clamped to [0, 1]. 0 when initialGaps == 0 (nothing to reduce).
	UncertaintyReductionRate float64 `json:"uncertainty_reduction_rate"`
	// ObservedGapsClosedCount is initialGaps - residualGaps (absolute count of
	// gaps closed across the sub-turn sequence). Never negative.
	ObservedGapsClosedCount int `json:"observed_gaps_closed_count"`
	// FrameDeltaConsumed is true iff the last sub-turn's prompt carried the
	// injected Plan frame delta (evidence the closure loop is live).
	FrameDeltaConsumed bool `json:"frame_delta_consumed"`
}

// ComputeConvergenceMetric is a deterministic pure function. Deterministic
// only — 0 LLM invocation (design.md §2.5 P0 约束). Same subTurns input
// always yields the same ConvergenceMetric.
//
// Algorithm (design.md §6.1):
//   - initialGaps  = subTurns[0].InitialObsGaps    (round entry gap count)
//   - residualGaps = subTurns[last].ResidualObsGaps (round exit gap count)
//   - rate         = (initialGaps - residualGaps) / initialGaps, clamped [0,1]
//   - closedCount  = max(0, initialGaps - residualGaps)
//   - consumed     = subTurns[last].PromptContainsPlanFrameDelta
//
// lastMetric is reserved for future EWMA smoothing across rounds (v1.1); v1.0
// ignores it and computes fresh each round. Empty subTurns → zero-value
// ConvergenceMetric{} + slog.Warn (fallback, does not block the sub-turn).
func ComputeConvergenceMetric(subTurns []SubTurnRecord, lastMetric *ConvergenceMetric) ConvergenceMetric {
	if len(subTurns) == 0 {
		slog.Warn("ComputeConvergenceMetric: empty subTurns, returning zero metric",
			"phase", "DM-20260705-010")
		return ConvergenceMetric{}
	}
	_ = lastMetric // reserved for v1.1 cross-round smoothing

	initialGaps := subTurns[0].InitialObsGaps
	residualGaps := subTurns[len(subTurns)-1].ResidualObsGaps

	closedCount := initialGaps - residualGaps
	if closedCount < 0 {
		closedCount = 0
	}

	rate := 0.0
	if initialGaps > 0 {
		rate = float64(closedCount) / float64(initialGaps)
	}
	if rate > 1.0 {
		rate = 1.0
	}

	return ConvergenceMetric{
		UncertaintyReductionRate: rate,
		ObservedGapsClosedCount:  closedCount,
		FrameDeltaConsumed:       subTurns[len(subTurns)-1].PromptContainsPlanFrameDelta,
	}
}

// BuildRoundSubTurnRecord derives a deterministic SubTurnRecord for one
// Execute round from its observation IDs, ResolutionClaims, ResolutionReport
// and whether the Plan frame delta was injected into the prompt. 0 LLM.
//
// Gap-count sources (design.md §5.2 "工具结果 diff + claim 数 + obs_uncertainty
// 残量"):
//   - When ResolutionReport is present (Plan carried ResolutionStrategies):
//     initialGaps = report.TotalStrategies, residualGaps = len(UnresolvedObs).
//   - Legacy rounds (nil report): initialGaps = len(obsIDs), residualGaps =
//     len(obsIDs) − (# distinct obsIDs answered by a ResolutionClaim).
func BuildRoundSubTurnRecord(
	obsIDs []string,
	claims []interfaces.ResolutionClaim,
	report *interfaces.ResolutionReport,
	frameDeltaConsumed bool,
) SubTurnRecord {
	initialGaps := len(obsIDs)
	residualGaps := initialGaps
	if report != nil {
		initialGaps = report.TotalStrategies
		residualGaps = len(report.UnresolvedObs)
	} else {
		residualGaps = initialGaps - countClosedObsGaps(obsIDs, claims)
	}
	if residualGaps < 0 {
		residualGaps = 0
	}
	return SubTurnRecord{
		InitialObsGaps:               initialGaps,
		ResidualObsGaps:              residualGaps,
		PromptContainsPlanFrameDelta: frameDeltaConsumed,
	}
}

// countClosedObsGaps counts distinct obsIDs that have at least one
// ResolutionClaim with a non-empty Answer. Deterministic set intersection.
func countClosedObsGaps(obsIDs []string, claims []interfaces.ResolutionClaim) int {
	if len(obsIDs) == 0 || len(claims) == 0 {
		return 0
	}
	answered := make(map[string]bool, len(claims))
	for _, c := range claims {
		if strings.TrimSpace(c.Answer) != "" {
			answered[c.ObsID] = true
		}
	}
	closed := 0
	for _, id := range obsIDs {
		if answered[id] {
			closed++
		}
	}
	return closed
}
