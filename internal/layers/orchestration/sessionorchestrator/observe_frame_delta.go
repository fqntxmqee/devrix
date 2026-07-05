// Package sessionorchestrator — observe_frame_delta.go
//
// DM-20260705-010 (devrix-d7-mups-frame-delta-closure) Phase 2 T7:
// BuildObservePriorDelta — build the Observe-side FrameDelta from the
// previous round's WorkItemExecContext (5 fields → 2 fields surfaced).
//
// 设计契约 (design.md §3.2):
//   - 首轮 (prevExecCtx == nil OR Item == nil OR Item.LastRound == nil):
//     返回 FrameDelta{} 零值 (no signal, observe.prior_delta_empty span emit)
//   - 非首轮: 从 prevExecCtx.Item.LastRound.ArtifactSummary 提摘要
//     (≤ MaxPriorArtifactSummaryChars = 80 字符, 截断加 "...")
//   - known_gaps: Phase 2 stub 空数组, Phase 3+ 通过 WorkItemExecContext
//     新增字段 LastPlanScopeIn + ObservedResolved 计算差集
//
// Span emit (AC5):
//   - OpD7_S5_Observe_PriorDelta_Inject (emit prior_artifact_summary_length
//     + known_gaps_count + span_tag_complete)
package sessionorchestrator

import (
	"context"
	"strings"

	"github.com/devrix/devrix/internal/layers/orchestration/hardening"
	"github.com/devrix/devrix/internal/layers/orchestration/interfaces"
)

// BuildObservePriorDelta constructs an Observe-side FrameDelta from the
// previous round's WorkItemExecContext. Returns FrameDelta{} zero-value when:
//   - prevExecCtx is nil (first round, fresh session)
//   - prevExecCtx.Item is nil (no WorkItem context — defensive)
//   - prevExecCtx.Item.LastRound is nil (no prior round data)
//
// Idempotency: same prevExecCtx → same FrameDelta. Pure function with
// side-effect: telemetry span emit (nil-bridge safe via hardening).
//
// Field mapping (design.md §3.2 fallback table):
//   - prior_artifact_summary ← prevExecCtx.Item.LastRound.ArtifactSummary (≤80)
//   - known_gaps             ← empty slice (Phase 2 stub, Phase 3+ from
//                              WorkItemExecContext.LastPlanScopeIn - ObservedResolved)
//
// v1.1 evolution: when LastConvergenceMetric (Phase 3) lands, the
// prior_artifact_summary will switch from raw ArtifactSummary to the
// convergence-aware format ("Round N: X% converged, gaps closed: 3/5").
func BuildObservePriorDelta(ctx context.Context, sessionID string, prevExecCtx *WorkItemExecContext) interfaces.FrameDelta {
	if prevExecCtx == nil || prevExecCtx.Item == nil || prevExecCtx.Item.LastRound == nil {
		// 首轮 / fresh session → no signal, no-op FrameDelta
		hardening.EmitObservePriorDelta(ctx, sessionID, "", 0, false)
		return interfaces.FrameDelta{}
	}
	summary := strings.TrimSpace(prevExecCtx.Item.LastRound.ArtifactSummary)
	if summary == "" {
		hardening.EmitObservePriorDelta(ctx, sessionID, "", 0, false)
		return interfaces.FrameDelta{}
	}
	if len(summary) > interfaces.MaxPriorArtifactSummaryChars {
		summary = summary[:interfaces.MaxPriorArtifactSummaryChars-3] + "..."
	}
	fd := interfaces.FrameDelta{
		PriorArtifactSummary: summary,
		// Phase 2 stub: known_gaps 留空。
		// Phase 3+ 扩展: 从 prevExecCtx.LastPlanScopeIn - prevExecCtx.ObservedResolved
		// 计算 machine-readable gap ID 数组 (每项 ≤ MaxKnownGapsItemChars=60)。
		KnownGaps: nil,
	}
	hardening.EmitObservePriorDelta(ctx, sessionID, fd.PriorArtifactSummary, len(fd.KnownGaps), true)
	return fd
}