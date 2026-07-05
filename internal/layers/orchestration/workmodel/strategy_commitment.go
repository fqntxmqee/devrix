// Package workmodel — strategy_commitment.go
//
// commitmentStrategy: 1-step synchronous commitment plan.
// M3 行为增量:
//   - VerdictFail + CommitmentPlan    → SpawnNone (terminal, 1-step commitment 不重试)
//   - VerdictPartial + CommitmentPlan → SpawnNone (terminal partial acceptance)
//
// Other PlanKinds: SpawnOverride returns (SpawnNone, false) → fall through to default.

package workmodel

import (
	"github.com/devrix/devrix/internal/layers/orchestration/plan"
	"github.com/devrix/devrix/internal/shared/types"
)

// commitmentStrategy is the per-PlanKind Strategy for CommitmentPlan.
type commitmentStrategy struct{}

// RouteChannel returns "commit_channel" for CommitmentPlan, empty otherwise.
func (commitmentStrategy) RouteChannel(planKind plan.PlanKind) string {
	if planKind == plan.CommitmentPlan {
		return "commit_channel"
	}
	return ""
}

// SpawnOverride enforces terminal behavior for 1-step commitment plans.
// Returns ok=false for non-CommitmentPlan, for verdicts that should fall
// through to the default 5-case logic, OR when CC-1.4 deliverable
// continuation is required (deliverable must be completed before
// terminating the plan).
func (commitmentStrategy) SpawnOverride(round *WorkItemPipelineRound) (SpawnPolicy, bool) {
	if round == nil || round.PlanKind != plan.CommitmentPlan {
		return SpawnNone, false
	}
	switch round.VerdictKind {
	case types.VerdictFail, types.VerdictPartial:
		// CC-1.4 precedence: deliverable continuation must take precedence
		// over the terminal override. Fall through to 5-case default which
		// handles spawnForDeliverableContinuation.
		if deliverableContinuationRequired(round) {
			return SpawnNone, false
		}
		// M3 行为增量: 1-step commitment plans are terminal on Fail/Partial
		// when no deliverable is owed (no retry, no decompose). Matches
		// Phase 3 PR-C2 CommitChannel "1-Step 同步 + IdempotencyKey 强制"
		// semantics.
		return SpawnNone, true
	default:
		return SpawnNone, false
	}
}

// ShouldDecompose reports whether commitment plans decompose.
// Commitment plans are 1-step synchronous; no decomposition.
func (commitmentStrategy) ShouldDecompose(planKind plan.PlanKind) bool {
	_ = planKind // strategy is per-PlanKind; parameter is for interface conformance
	return false
}

// IsReadOnly reports whether commitment plans have side effects.
// Commitment plans write to external systems (deploy, merge, send).
func (commitmentStrategy) IsReadOnly(planKind plan.PlanKind) bool {
	_ = planKind
	return true
}
