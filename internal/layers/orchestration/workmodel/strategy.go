// Package workmodel — strategy.go
//
// Strategy abstraction for per-PlanKind behavior (M3, DM-20260705-008).
// Decouples spawn policy decision from mups/execute ChannelRegistry so
// workmodel (L2) can read PlanKind-aware behavior without depending on
// mups/execute (L1) — avoids layering violation.
//
// 4 PlanKind Strategy implementations live in strategy_*.go:
//   - strategy_commitment.go   (1-step synchronous, terminal fail)
//   - strategy_protocol.go     (multi-step async, decompose on failure)
//   - strategy_scenario.go     (read-only probe, no retry)
//   - strategy_exploration.go  (parallel experiments, decompose heavily)
//
// Default registry + LookupStrategy helper live in strategy_default.go.
//
// Invariants:
//   - RouteChannel MUST return the channel name for the given PlanKind
//     (or empty string if no channel bound).
//   - SpawnOverride MAY return a custom SpawnPolicy to override the
//     checkVerdictDirection 5-case default. Return ok=false to fall through.
//   - ShouldDecompose MUST report whether the plan kind supports child
//     decomposition (protocol/exploration true; commitment/scenario false).
//   - IsReadOnly MUST report whether the plan kind has side effects
//     (commitment/protocol/exploration true; scenario false).
//
// L1 (mups/execute) does NOT depend on this interface. The bridge is
// WorkItemExecContext.Strategy (sessionorchestrator package).

package workmodel

import (
	"github.com/devrix/devrix/internal/layers/orchestration/plan"
)

// Strategy encapsulates per-PlanKind behavior (routing, channel selection,
// spawn policy override). 4 PlanKind Strategy implementations live in
// strategy_*.go (commitment/protocol/scenario/exploration).
//
// M3 design rationale: see openspec/changes/d7-mups-strategy-injection/design.md
// §④ 领域模型 and §⑥ 接口/API 设计.
type Strategy interface {
	// RouteChannel returns the channel name for the given PlanKind.
	// Empty string if no channel bound (caller should fall back to default).
	RouteChannel(planKind plan.PlanKind) string

	// SpawnOverride returns a custom SpawnPolicy to override the
	// checkVerdictDirection 5-case default. The full round is passed so the
	// Strategy can inspect DeliverableSchema/DeliverableStatus (CC-1.4
	// deliverable continuation must take precedence over the per-PlanKind
	// terminal override for CommitmentPlan + Partial). Return ok=false to
	// fall through to the 5-case default.
	SpawnOverride(round *WorkItemPipelineRound) (SpawnPolicy, bool)

	// ShouldDecompose reports whether the plan kind supports child decomposition.
	// Protocol/Exploration: true (multi-step allows decomposition).
	// Commitment/Scenario: false (1-step / read-only no decompose).
	ShouldDecompose(planKind plan.PlanKind) bool

	// IsReadOnly reports whether the plan kind has side effects.
	// Commitment/Protocol/Exploration: true (write to external systems).
	// Scenario: false (read-only probe).
	IsReadOnly(planKind plan.PlanKind) bool
}
