// Package execute implements MUPS v4.3 Phase 3 PR-C2 — the Execute node's
// 4-channel router that dispatches a Plan (Phase 2 output) to one of
// CommitChannel / ProtocolChannel / ScenarioChannel / ExplorationChannel
// based on Plan.Kind, and produces an Artifact (Phase 3 PR-C1 output) for
// downstream Verify consumption.
//
// PlanChannel interface contract:
//   - Each Channel knows which PlanKind(s) it Supports().
//   - ChannelRegistry maps PlanKind → Channel; ChannelRouter dispatches.
//   - Channels use a pluggable ToolRunner so tests can inject fakes without
//     pulling in toolrunner/surface (PR-C4) or the DispatchWorker v2 (PR-C7).
//
// The PR-C2 surface is intentionally minimal: 4 Channel stubs, ChannelRouter,
// and the SentinelErrors that the PR-C7 Executor will surface. Wire-format
// invariants (PlanKind ↔ ArtifactKind 1:1 mapping) are validated by the
// ChannelRegistry tests so PR-C7 can build on stable ground.
package execute

import (
	"errors"
	"fmt"

	sharederrors "github.com/devrix/devrix/internal/shared/errors"
)

// Sentinel errors for Phase 3 PR-C2 Channel routing.
//
// Naming convention (matches D7 orchtypes + D7 plan pattern):
//   pkg/struct/specifics — pkg=execute, struct=channel / channel_registry / etc.
var (
	// ErrChannelNotFound: ChannelRegistry.Get(planKind) had no Channel
	// registered for the given PlanKind. This indicates a wiring bug
	// (someone forgot to Register a Channel) — never surfaced to users.
	ErrChannelNotFound = errors.New("execute: no channel registered for plan kind")

	// ErrChannelUnsupported: Channel.Supports(planKind) returned false.
	// Used by ChannelRouter as a defensive guard before calling Execute.
	// Distinct from ErrChannelNotFound so logs can distinguish wiring bugs
	// (not found) from semantic mismatches (Channel exists but rejects).
	ErrChannelUnsupported = errors.New("execute: channel does not support plan kind")

	// ErrChannelStepCountMismatch: CommitChannel requires exactly 1 Step;
	// ProtocolChannel requires ≥1 Step; ScenarioChannel requires ≥1 Step.
	// The mismatch is a Plan validation issue but surfaces at Execute-time
	// because some Steps can be derived dynamically (future PR).
	ErrChannelStepCountMismatch = errors.New("execute: plan step count does not match channel requirement")

	// ErrChannelPlanNil: Channel.Execute called with nil Plan. Defensive —
	// should never happen because ChannelRouter validates before dispatch.
	ErrChannelPlanNil = errors.New("execute: plan is nil")

	// ErrChannelToolRunnerNil: Channel constructed without a ToolRunner.
	// Indicates a constructor bug at wiring time.
	ErrChannelToolRunnerNil = errors.New("execute: tool runner is nil")

	// ErrChannelStepInvalid: a Step's fields are invalid for its channel
	// (e.g. CommitChannel with empty ToolName, missing IdempotencyKey on a
	// side-effecting step). Distinct from ErrChannelStepCountMismatch
	// which covers cardinality violations — these are per-step field
	// violations that warrant their own triage code.
	ErrChannelStepInvalid = errors.New("execute: plan step has invalid field")

	// ErrChannelToolCallTimedOut: the tool call hit the channel's
	// timeout. Distinct from ErrChannelStepInvalid so callers can route
	// inflight side effects to StrategyAskNow (PR-C3) without confusing
	// them with malformed-Plan errors.
	ErrChannelToolCallTimedOut = errors.New("execute: tool call timed out")

	// ErrChannelCtxCancelled: the channel's outer ctx was cancelled while
	// the channel was running. Distinct from ErrChannelToolCallTimedOut
	// (channel-internal deadline) so callers can route upstream cancellation
	// (e.g. turn cancel / parent abort) differently from inner timeouts.
	// RH-D7-09 (DM-20260630-013 T-P1-A3-8.2): previously scenario +
	// exploration would wait for every probe to finish even after the
	// outer ctx was cancelled, hiding the cancellation behind a misleading
	// "majority failed" result.
	ErrChannelCtxCancelled = errors.New("execute: ctx cancelled during channel run")
)

// -----------------------------------------------------------------------------
// SentinelError helpers (sharederrors.WithCode)
// -----------------------------------------------------------------------------

// NewChannelNotFoundError returns a SentinelError wrapping ErrChannelNotFound
// with the offending PlanKind in the message so logs are self-triageable.
func NewChannelNotFoundError(planKind string) *sharederrors.SentinelError {
	return sharederrors.WithCode(
		"EXEC_CHANNEL_9001",
		fmt.Sprintf("execute: no channel registered for plan kind=%s", planKind),
		fmt.Errorf("%w: planKind=%s", ErrChannelNotFound, planKind),
	)
}

// NewChannelUnsupportedError returns a SentinelError wrapping
// ErrChannelUnsupported with both channel name and plan kind for triage.
func NewChannelUnsupportedError(channelName, planKind string) *sharederrors.SentinelError {
	return sharederrors.WithCode(
		"EXEC_CHANNEL_9002",
		fmt.Sprintf("execute: channel=%s does not support planKind=%s", channelName, planKind),
		fmt.Errorf("%w: channel=%s planKind=%s", ErrChannelUnsupported, channelName, planKind),
	)
}

// NewChannelStepCountMismatchError returns a SentinelError with the channel
// name, observed step count, and required range.
func NewChannelStepCountMismatchError(channelName string, observed, wantMin, wantMax int) *sharederrors.SentinelError {
	return sharederrors.WithCode(
		"EXEC_CHANNEL_9003",
		fmt.Sprintf("execute: channel=%s step count=%d, want [%d, %d]", channelName, observed, wantMin, wantMax),
		fmt.Errorf("%w: channel=%s observed=%d want=[%d,%d]", ErrChannelStepCountMismatch, channelName, observed, wantMin, wantMax),
	)
}

// NewChannelToolRunnerNilError returns a SentinelError for constructor bugs.
func NewChannelToolRunnerNilError(channelName string) *sharederrors.SentinelError {
	return sharederrors.WithCode(
		"EXEC_CHANNEL_9004",
		fmt.Sprintf("execute: channel=%s constructed without a ToolRunner", channelName),
		fmt.Errorf("%w: channel=%s", ErrChannelToolRunnerNil, channelName),
	)
}

// NewChannelStepInvalidError returns a SentinelError for per-step field
// violations (empty ToolName, missing IdempotencyKey, etc.).
func NewChannelStepInvalidError(channelName, toolName, reason string) *sharederrors.SentinelError {
	return sharederrors.WithCode(
		"EXEC_CHANNEL_9005",
		fmt.Sprintf("execute: channel=%s step (tool=%s) invalid: %s", channelName, toolName, reason),
		fmt.Errorf("%w: channel=%s tool=%s: %s", ErrChannelStepInvalid, channelName, toolName, reason),
	)
}

// NewChannelToolCallTimedOutError returns a SentinelError when the tool
// call hit the channel's timeout. The StrategyDecider (PR-C3) routes
// these to StrategyAskNow because the side effect may have committed.
func NewChannelToolCallTimedOutError(channelName, toolName string) *sharederrors.SentinelError {
	return sharederrors.WithCode(
		"EXEC_CHANNEL_9006",
		fmt.Sprintf("execute: channel=%s tool=%s call timed out (side-effect status uncertain)", channelName, toolName),
		fmt.Errorf("%w: channel=%s tool=%s", ErrChannelToolCallTimedOut, channelName, toolName),
	)
}

// NewChannelCtxCancelledError returns a SentinelError when the channel's
// outer ctx was cancelled mid-run. Callers can route this to
// StrategyCancel (turn abort) rather than StrategyAskNow (inflight
// side-effect query). ctxErr is the underlying context error.
func NewChannelCtxCancelledError(channelName string, ctxErr error) *sharederrors.SentinelError {
	return sharederrors.WithCode(
		"EXEC_CHANNEL_9007",
		fmt.Sprintf("execute: channel=%s ctx cancelled mid-run: %v", channelName, ctxErr),
		fmt.Errorf("%w: channel=%s: %w", ErrChannelCtxCancelled, channelName, ctxErr),
	)
}
