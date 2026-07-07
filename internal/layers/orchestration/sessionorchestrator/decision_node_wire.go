package sessionorchestrator

// runDecisionStage wires the Stage-5 Decision node into the
// ItemPipelineRunner Run() flow (DM-20260707-001 PR-D, T46-T51).
//
// The function is a pure adapter: it builds a DecisionContext from
// the round's existing fields, calls the static DecisionNode, and
// returns the Decision. The 11-row mapping table is the single source
// of truth for routing; this file does not contain any routing logic
// of its own.

import (
	"log/slog"

	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/types"
)

// runDecisionStage returns the Stage-5 Decision for the round. It is
// safe to call when the round has no usable Verdict (e.g. execute error
// path); in that case the Decision falls through to a deterministic
// safety-net A accept + MapRow=0 fallback so the round still completes.
//
// Why a per-runner method rather than a free function: the RoundMeta
// (AttemptNo / ChildBudgetRemaining / sibling counts) is per-WorkItem
// state, and ItemPipelineRunner is the only place that has the
// canonical view. Pulling it into a free function would force callers
// to duplicate the same source-of-truth plumbing in every test.
func (r *ItemPipelineRunner) runDecisionStage(
	sessionID string,
	item *workmodel.WorkItem,
	verdict workmodel.Verdict,
	roundNo int,
) Decision {
	if r == nil || item == nil {
		// Defensive: when the runner or item vanished (e.g. session
		// cancelled), emit a safety-net accept so the round struct
		// stays coherent. Decision-node row 0 path.
		return Decision{Kind: DecisionAccept, Reason: "decision_runner_nil", MapRow: 0}
	}
	if r.Tasks == nil {
		// Tasks unavailable (test stub). Without Tasks we can't read
		// sibling counts; collapse to row 1 (Pass) / row 6 (Fail) only.
		ctx := DecisionContext{
			RoundMeta:   buildDefaultRoundMeta(RoundMeta{AttemptNo: attemptNoFromLastRound(item, roundNo), RiskLevel: "normal"}),
			VerdictKind: uint8(verdict.Kind),
		}
		d, err := NewStaticDecisionNode().Decide(ctx)
		if err != nil {
			return Decision{Kind: DecisionAccept, Reason: "decision_nominal_" + err.Error(), MapRow: 0}
		}
		return d
	}

	// Sibling counts: count children of item.ID; siblings in our
	// tree model are the same generation (children of the parent).
	// For the common single-WorkItem path the parent has 1 child (us)
	// so SiblingTotalCount==0, which short-circuits row 10 and lets
	// the verdict-based rows fire.
	siblingDecided, siblingTotal := siblingCounts(sessionID, item, r.Tasks)

	meta := RoundMeta{
		AttemptNo:            attemptNoFromLastRound(item, roundNo),
		ChildBudgetRemaining: childBudgetRemaining(item),
		RiskLevel:            riskLevelForItem(item),
		IsChildSegment:       isChildSegment(item),
		SiblingDecidedCount:  siblingDecided,
		SiblingTotalCount:    siblingTotal,
		HasDecomposableAC:    false, // future T66-T72 wire-up
		ToleranceHint:        "normal",
		VerdictErrorClass:    verdictErrorClassFor(verdict),
		PlanErrorClass:       "", // PR-F wiring (T69) — empty in PR-D
	}

	ctx := DecisionContext{
		RoundMeta:         meta,
		VerdictKind:       uint8(verdict.Kind),
		VerdictErrorClass: meta.VerdictErrorClass,
	}
	d, err := NewStaticDecisionNode().Decide(ctx)
	if err != nil {
		// Defensive: runaway AttemptNo or negative value. log + return
		// safety-net accept so downstream ApplyPipelineDecide still
		// runs and the round doesn't lock.
		slog.Warn("item_pipeline: decision node err; falling back to A accept",
			"session_id", sessionID, "work_item_id", item.ID, "err", err)
		return Decision{Kind: DecisionAccept, Reason: "decision_error_fallback:" + err.Error(), MapRow: 0}
	}
	return d
}

// attemptNoFromLastRound converts roundNo (1-based) into AttemptNo
// (0-based retry count). The Decision table uses AttemptNo < MaxRetry
// to split row 5 (retry) from row 6 (human_review), so we keep the
// value aligned with the "how many times have we already retried"
// semantic.
func attemptNoFromLastRound(item *workmodel.WorkItem, roundNo int) int {
	if item == nil || item.LastRound == nil {
		return 0
	}
	// roundNo is the upcoming round's index; LastRound.RoundNo is the
	// one we just finished. attempts already consumed = roundNo - 1.
	if roundNo <= 0 {
		return 0
	}
	return roundNo - 1
}

// childBudgetRemaining returns the C-path budget left for this round.
// PR-D conservative: max 2 minus any explicit budget already consumed
// on prior rounds (placeholder, the live counter is per-WorkItem and
// lives on WorkItem metadata). The 2 cap matches ChildWorkItemSpec.
func childBudgetRemaining(item *workmodel.WorkItem) int {
	if item == nil {
		return 0
	}
	return 2 // full budget; per-round decrement is wired when the C-spawn
	// loop lands (post-PR-D). For now every round starts with 2 slots.
}

// riskLevelForItem is the per-item risk policy. PR-D placeholder: the
// real value comes from orchtypes.Config.RiskPolicy (future change) or
// a per-WorkItem metadata. Default "normal" so the row 7/8 split
// behaves deterministically.
func riskLevelForItem(item *workmodel.WorkItem) string {
	if item == nil {
		return "normal"
	}
	return "normal"
}

// isChildSegment checks whether the item is a child of a parent
// rollup. ItemPipelineRunner currently only sets NeedsRollup on
// rollup rounds; child segments have a non-empty ParentID. Mirrors
// the §8.6.1 row 10 trigger.
func isChildSegment(item *workmodel.WorkItem) bool {
	if item == nil {
		return false
	}
	return item.ParentID != ""
}

// siblingCounts returns (decided, total) sibling segment counts so
// the row 10 parent_rollup gate can fire when every sibling has
// decided. PR-D heuristic: count non-rollup children of the same
// parent whose LastRound is non-nil (= "has decided"). SiblingTotal
// excludes rollup-synthesised children (which the parent itself
// owns). Returns (0, 0) when the item has no parent.
func siblingCounts(sessionID string, item *workmodel.WorkItem, tm *workmodel.TaskManager) (int, int) {
	if item == nil || tm == nil || item.ParentID == "" {
		return 0, 0
	}
	children := tm.Tree().ListChildren(sessionID, item.ParentID)
	decided := 0
	for _, c := range children {
		if c == nil || c.ID == item.ID {
			continue
		}
		if c.NeedsRollup {
			continue
		}
		if c.LastRound != nil {
			decided++
		}
	}
	total := len(children) - 1 // exclude self
	if total < 0 {
		total = 0
	}
	return decided, total
}

// verdictErrorClassFor maps a Verdict into a row 9 discriminator. PR-D
// only knows Indeterminate + SystemAnomaly; other verdict kinds
// return "" so they fall through to their normal rows. Future
// extensions: distinguish parse-failure vs env-limited.
func verdictErrorClassFor(verdict workmodel.Verdict) string {
	if verdict.Kind != types.VerdictIndeterminate {
		return ""
	}
	// SystemAnomaly class is the row 9 trigger. Verdict.SourceID
	// carries the verify class; when the verify node has flagged
	// SystemAnomaly, we route to row 9 retry.
	if verdict.SourceID == "system_anomaly" {
		return "system_anomaly"
	}
	return ""
}

// exitReasonForDecision rewrites the round's ExitReason when the
// Decision node chose a non-accept path so dashboards and Escape
// detection can route correctly. The base exitReason came from
// exitReasonForVerdict (verdict → exit_reason mapping); Decision
// adds a prefix only when the path diverges from a clean accept.
//
// Asymmetric policy (Risk B2 / codex consensus): Retry SUFFIXES the
// base (base+decision_retry) because the retry path is still a normal
// round-trip — base verdict context is useful for downstream consumers.
// C/D/E FULL-REPLACE the base because the routing path diverges from
// verdict-driven exit (the parent rollup synthesizes a new artifact;
// the child worker exits its own WorkItem; human review aborts the
// round). Use the base ExitReason's prefix to disambiguate, or rely on
// round.DecisionKind for a machine-readable tag.
func exitReasonForDecision(base string, d Decision) string {
	switch d.Kind {
	case DecisionAccept:
		return base
	case DecisionRetry:
		if base != "" {
			return base + "+decision_retry"
		}
		return "decision_retry"
	case DecisionChildWorker:
		return "decision_child_worker"
	case DecisionParentRollup:
		return "decision_parent_rollup"
	case DecisionHumanReview:
		return "decision_human_review"
	default:
		return base
	}
}
