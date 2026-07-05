// Package workmodel — strategy_protocol.go
//
// protocolStrategy: multi-step async protocol plan (default).
// protocolStrategy is the SAFE DEFAULT for unknown PlanKind values
// (see LookupStrategy in strategy_default.go). It returns ok=false
// for SpawnOverride in all cases — meaning all verdicts fall through
// to the existing 5-case default behavior. This guarantees 0 behavior
// change for any PlanKind that doesn't have an explicit override.

package workmodel

import (
	"github.com/devrix/devrix/internal/layers/orchestration/plan"
)

// protocolStrategy is the per-PlanKind Strategy for ProtocolPlan AND
// the safe default for any unknown PlanKind value.
type protocolStrategy struct{}

// RouteChannel returns "protocol_channel" for ProtocolPlan, empty otherwise.
func (protocolStrategy) RouteChannel(planKind plan.PlanKind) string {
	if planKind == plan.ProtocolPlan {
		return "protocol_channel"
	}
	return ""
}

// SpawnOverride returns (SpawnNone, false) for all rounds —
// protocolStrategy is the "no-op" default that lets the existing
// 5-case checkVerdictDirection logic take over. Also serves as the
// safe default for any unknown PlanKind (LookupStrategy fallback).
func (protocolStrategy) SpawnOverride(round *WorkItemPipelineRound) (SpawnPolicy, bool) {
	return SpawnNone, false
}

// ShouldDecompose reports whether protocol plans decompose.
// Protocol plans are multi-step async; decomposition allowed on failure.
func (protocolStrategy) ShouldDecompose(planKind plan.PlanKind) bool {
	_ = planKind
	return true
}

// IsReadOnly reports whether protocol plans have side effects.
// Protocol plans write to external systems (migrate, rotate).
func (protocolStrategy) IsReadOnly(planKind plan.PlanKind) bool {
	_ = planKind
	return true
}
