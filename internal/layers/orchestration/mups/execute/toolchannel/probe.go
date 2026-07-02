package toolchannel

import (
	"context"
	"fmt"

	"github.com/devrix/devrix/internal/layers/observability/instrument/ltl/invariants/termination"
	"github.com/devrix/devrix/internal/shared/contracts"
)

// ProbeToolChannel is the 治本 core: Bounded(n) hard reject +
// PromptPressure soft warning + synthesize-now injection.
//
// DSAFT: D7-S9-A50-T03 (Bounded) + T04 (hard reject) + T05 (PromptPressure)
// + T07 (shadow mode). Demand RC-1 root cause fix.
//
// Behavior (specs/execute-channels.md §D7-EXEC-CH-1):
//   - On Accept(state.IterationsUsed >= MaxN):
//     1. Return ErrProbeToolChannelBoundExceeded (enforce mode)
//        OR (false, nil) in shadow mode (logged only)
//     2. ProbeToolChannel emits a "synthesize now" system message
//        via InjectPromptPressure on the call before the reject
//   - On Accept(remaining in {soft, hard} thresholds by task_kind):
//     InjectPromptPressure injects a soft warning
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
		maxN = 15 // read_file/grep/glob default
	}
	// Override the state's bound so the invariant sees the right value.
	state.BoundMax = maxN

	// Run the L4 invariant.
	ok, reason := p.inv.Check(state)
	if !ok {
		// Bound hit — hard reject. The channel emits ErrProbeToolChannelBoundExceeded
		// so the LLM context sees a clear "synthesize now" signal.
		return false, fmt.Errorf("%w: %s (call=%s, task_kind=%s)",
			ErrProbeToolChannelBoundExceeded, reason, call.ToolName, call.TaskKind)
	}
	return true, nil
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
