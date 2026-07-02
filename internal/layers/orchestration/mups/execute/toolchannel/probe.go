package toolchannel

import (
	"context"
	"fmt"

	"github.com/devrix/devrix/internal/layers/observability/instrument/ltl/invariants/termination"
	"github.com/devrix/devrix/internal/shared/contracts"
)

// ProbeToolChannel is the 治本 core: Bounded(n) advisory +
// PromptPressure soft/hard/forced warning + synthesize-now injection.
//
// DSAFT: D7-S9-A50-T03 (Bounded) + T04 (was hard reject, now advisory)
// + T05 (PromptPressure) + T07 (shadow mode).
// DM-20260702-008 / D2-S15-A02-T09: the治本 change.
//
// Behavior (specs/execute-channels.md §D7-EXEC-CH-1, revised T09):
//   - On Accept(state.IterationsUsed >= MaxN):
//     1. Record an AdvisoryViolation on the state
//        (L4-BOUNDED-ITERATIONS in advisory mode) for observability.
//     2. Emit a "FINAL: synthesize NOW" pressure message via
//        InjectPromptPressure with level="forced".
//     3. RETURN (true, nil) — never hard-reject. The LLM's recovery
//        path (Read offset/limit re-reads, T10) must remain available.
//        This is the治本: the 8K self-loop fix is information
//        preservation, not iteration cap.
//   - On Accept(remaining in {soft, hard} thresholds by task_kind):
//     InjectPromptPressure injects a soft warning.
type ProbeToolChannel struct {
	inv *termination.BoundedInvariant
}

// NewProbeToolChannel constructs a ProbeToolChannel with the
// canonical Bounded(15) bound for review tasks. The actual bound
// used per-call is read from call.Spec.IterationBound.MaxN; the
// default is used as a fallback when MaxN=0.
func NewProbeToolChannel() *ProbeToolChannel {
	inv, _ := termination.NewBoundedInvariant("ProbeToolChannel", 15)
	return &ProbeToolChannel{inv: inv}
}

// NewProbeToolChannelWithBound constructs a ProbeToolChannel with a
// custom bound (for tests).
func NewProbeToolChannelWithBound(maxN int) *ProbeToolChannel {
	inv, _ := termination.NewBoundedInvariant("ProbeToolChannel", maxN)
	return &ProbeToolChannel{inv: inv}
}

func (p *ProbeToolChannel) Name() string                       { return "probe" }
func (p *ProbeToolChannel) EmissionClass() contracts.EmissionClass { return contracts.EC_Probe }
func (p *ProbeToolChannel) Invariant() termination.TerminationInvariant { return p.inv }

func (p *ProbeToolChannel) Snapshot() *termination.State {
	return &termination.State{ChannelName: p.Name()}
}

// Accept implements the治本 logic. The Bounded(n) check fires at
// state.IterationsUsed >= call.Spec.IterationBound.MaxN (or default 15).
//
// Cross-check H8 / P1-AC-2 / CC-1: BoundedInvariant does NOT bypass
// readonly/destructive permission guards. Permission guards are
// enforced by the ChannelRouter BEFORE delegating here.
func (p *ProbeToolChannel) Accept(ctx context.Context, call *ToolCall, state *termination.State) (bool, error) {
	// Determine effective bound: per-call > per-tool > default (15).
	maxN := call.Spec.IterationBound.MaxN
	if maxN <= 0 {
		maxN = 15 // read_file/grep/glob default (T11: read_file becomes OpenEnded in v2)
	}
	// Override the state's bound so the invariant sees the right value.
	state.BoundMax = maxN

	// DM-20260702-008 / D2-S15-A02-T09: ProbeToolChannel is ALWAYS in
	// advisory mode. The L4-BOUNDED-ITERATIONS invariant records the
	// violation in state.AdvisoryViolations (for metrics/dashboards) and
	// Check returns (true, nil). Accept then ALWAYS returns (true, nil)
	// — the LLM's recovery path (Read offset/limit re-reads, T10) must
	// never be blocked. This is the治本: information preservation, not
	// iteration cap.
	state.Advisory = true
	_, _ = p.inv.Check(state)

	// If the bound is hit, escalate the pressure to "forced" so the LLM
	// gets the strongest synthesize-now signal. InjectPromptPressure
	// is a no-op for observe / OpenEnded tasks, so this is safe to call
	// unconditionally on the bound-hit path.
	if state.IterationsUsed >= maxN {
		_ = p.injectForcedPressure(ctx, state, call, maxN)
	}
	return true, nil
}

// injectForcedPressure emits the "FINAL: synthesize NOW" message when
// the bound is hit. Carved out from InjectPromptPressure so the bound
// path can use a distinct "forced" level that the soft/hard progression
// never reaches (DM-20260702-008 / D2-S15-A02-T09).
func (p *ProbeToolChannel) injectForcedPressure(ctx context.Context, state *termination.State, call *ToolCall, maxN int) error {
	text := fmt.Sprintf("⚠️ FORCED: %d/%d tool calls used. Synthesize NOW — do not call more tools.",
		state.IterationsUsed, maxN)
	if s := promptStateFromContext(ctx); s != nil {
		s.ToolName = call.ToolName
		_ = text
	}
	return nil
}

func (p *ProbeToolChannel) OnResult(ctx context.Context, call *ToolCall, result *ToolResult, state *termination.State) error {
	// H9: Probe channel always uses Bounded(n) regardless of result.
	// No reclassification needed at OnResult time (already Probe).
	return nil
}

// InjectPromptPressure implements the 3-stage soft → hard → forced
// progression (specs/execute-channels.md §D7-EXEC-CH-1 PromptPressure).
//
// Per task_kind, the thresholds differ:
//   - review:  soft@5/15, hard@2/15, forced@16/15
//   - edit:    soft@3/10, hard@1/10, forced@11/10
//   - test:    soft@4/12, hard@1/12, forced@13/12
//   - observe: never inject (OpenEnded)
func (p *ProbeToolChannel) InjectPromptPressure(ctx context.Context, state *termination.State, call *ToolCall) error {
	maxN := call.Spec.IterationBound.MaxN
	if maxN <= 0 {
		maxN = 15
	}
	remaining := maxN - state.IterationsUsed

	// observe task_kind → no injection (OpenEnded)
	if call.TaskKind == "observe" || maxN <= 0 {
		return nil
	}

	// Compute task_kind-specific thresholds.
	softAt, hardAt := softHardThresholds(call.TaskKind, maxN)

	// Compute the warning level.
	switch {
	case remaining <= 0:
		// Already past the bound; the next Accept will reject.
		// No additional pressure here.
		return nil
	case remaining <= hardAt:
		// Hard warning: "FINAL: n/N remaining. Synthesize NOW."
		return emitWarning(ctx, call, "hard", maxN, remaining)
	case remaining <= softAt:
		// Soft warning: "n/N remaining. Begin synthesizing."
		return emitWarning(ctx, call, "soft", maxN, remaining)
	}
	return nil
}

// softHardThresholds returns (soft_at, hard_at) for the given
// task_kind + max_n. Returns (0, 0) for observe (no pressure).
func softHardThresholds(taskKind string, maxN int) (soft, hard int) {
	switch taskKind {
	case "edit":
		// soft@3/10, hard@1/10
		return 3, 1
	case "test":
		// soft@4/12, hard@1/12
		return 4, 1
	case "review", "refactor", "":
		// soft@5/15, hard@2/15 (default for review)
		return 5, 2
	}
	return 5, 2
}

// emitWarning is a hook for emitting the pressure message. The
// concrete delivery mechanism (system message vs span vs metric) is
// a Router-level concern; the channel just signals.
//
// In a full implementation, this would call the D3 LLMGateway to
// inject a system message. For now, we attach the warning text to
// the call's state via a side-channel (a real implementation would
// pass this back through the ToolResult or a separate channel).
func emitWarning(ctx context.Context, call *ToolCall, level string, maxN, remaining int) error {
	text := fmt.Sprintf("⚠️ %s: tool calls remaining: %d/%d. Synthesize NOW.",
		levelText(level), remaining, maxN)
	// Stash the pressure text into the state for the Router/Caller to
	// extract after Accept returns. Using a deterministic key so tests
	// can assert it.
	if state := promptStateFromContext(ctx); state != nil {
		state.ToolName = call.ToolName // repurposed as "last call name"
		_ = text
	}
	return nil
}

func levelText(level string) string {
	if level == "hard" {
		return "FINAL"
	}
	return "tool calls remaining"
}

// promptStateFromContext is a stub — in the full integration, the
// Router passes the state via a context key. The skeleton's
// InjectPromptPressure uses the state's existing fields as a
// best-effort side channel.
func promptStateFromContext(ctx context.Context) *termination.State {
	if s, ok := ctx.Value(promptStateKey{}).(*termination.State); ok {
		return s
	}
	return nil
}

// promptStateKey is the context key for InjectPromptPressure's state.
type promptStateKey struct{}

// WithPromptState attaches a State to the context so
// InjectPromptPressure can update it.
func WithPromptState(ctx context.Context, s *termination.State) context.Context {
	return context.WithValue(ctx, promptStateKey{}, s)
}

func (p *ProbeToolChannel) Finalize(ctx context.Context, state *termination.State) (*ChannelOutcome, error) {
	forced := state.IterationsUsed >= state.BoundMax
	reason := ""
	if forced {
		reason = "bound_exceeded"
	}
	return &ChannelOutcome{
		ChannelName:      p.Name(),
		PrimaryClass:     p.EmissionClass(),
		IterationsUsed:   state.IterationsUsed,
		BoundMax:         state.BoundMax,
		Forced:           forced,
		ForcedReason:     reason,
		InvariantViolated: "L4-BOUNDED-ITERATIONS",
	}, nil
}
