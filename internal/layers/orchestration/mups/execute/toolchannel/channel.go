// Package toolchannel implements the D7-S9-A50 ToolChannel abstraction
// for the Execute node. It is the per-tool-EmissionClass termination
// routing layer that complements the per-PlanKind execution strategy
// in the parent mups/execute package.
//
// Design (specs/execute-channels.md §D7-EXEC-CH-1):
//   - One ToolChannel per EmissionClass (4 channels).
//   - Each ToolChannel attaches a primary LTL-Lite termination invariant
//     at register time.
//   - ProbeToolChannel is the治本 core: Bounded(n) hard reject +
//     PromptPressure soft warning + synthesize-now injection.
//
// Naming clarification (demand §3.1 + H7/H8 consensus):
//   - ToolChannel = this package (per-tool termination, by EmissionClass)
//   - PlanChannel = parent mups/execute (per-PlanKind strategy)
//
// DSAFT: D7-S9-A50-T01 (interface) + T02 (Router) + T03..T06 (4 channels)
// + T07 (shadow mode) + T08 (L0-L3 cross-check).
package toolchannel

import (
	"context"
	"fmt"
	"time"

	"github.com/devrix/devrix/internal/layers/observability/instrument/ltl/invariants/termination"
	"github.com/devrix/devrix/internal/shared/contracts"
)

// ToolCall is the per-tool invocation input to a ToolChannel.
// Distinct from mups/execute.ToolRequest: ToolCall carries EmissionClass
// metadata so the channel can route by class without a separate lookup.
type ToolCall struct {
	// SessionID is the owning session.
	SessionID string
	// ToolName is the name of the tool to invoke.
	ToolName string
	// Spec is the per-tool ToolSpec (v3 metadata).
	Spec contracts.ToolSpec
	// Args is the tool-specific input.
	Args map[string]any
	// QueryFingerprint is a stable hash of (ToolName, normalized Args).
	// Used by L7-FACT-SAME-Q-5x for OnResult reclassification (H9).
	QueryFingerprint string
	// TaskKind is the inferred user task kind (review/edit/test/observe/refactor).
	// Drives task_kind-bound overrides on ProbeToolChannel.
	TaskKind string
	// Timestamp is the call time.
	Timestamp time.Time
}

// ToolResult is the per-tool invocation output. Distinct from
// mups/execute.ToolResult: this struct carries EmissionClass + IterUsed
// so the channel can update its State.
type ToolResult struct {
	// ToolName echoes the input.
	ToolName string
	// Output is the tool's textual output.
	Output string
	// ExitCode is 0 for success.
	ExitCode int
	// Err is the tool's error (nil on success).
	Err error
	// Duration is the wall-clock invocation time.
	Duration time.Duration
	// Complete indicates whether the result was the full intended output
	// (false when the LLM/D2 truncated it; the LLM may need to REREAD).
	Complete bool
	// TruncatedAt is the chars position when Complete=false (0 when full).
	TruncatedAt int
}

// ChannelOutcome is the final result returned by a ToolChannel.Finalize.
// Distinct from wavescheduler.Artifact: this struct carries the per-channel
// termination summary for the VerifyContract's 4-tuple audit.
type ChannelOutcome struct {
	// ChannelName is the canonical name (e.g. "probe", "fact", "action", "experiment").
	ChannelName string
	// PrimaryClass is the channel's EmissionClass.
	PrimaryClass contracts.EmissionClass
	// DeliverableText is the LLM's final text after synthesize (or "" if forced).
	DeliverableText string
	// Evidence lists the tool call IDs (used by VerifyContract MinEvidence).
	Evidence []string
	// IterationsUsed is the final iteration count.
	IterationsUsed int
	// BoundMax is the bound that was enforced (0 = no bound).
	BoundMax int
	// Forced indicates the channel forced terminate (vs. natural LLM completion).
	Forced bool
	// ForcedReason is the reason when Forced=true (e.g. "bound_exceeded", "synthesize_min_chars").
	ForcedReason string
	// InvariantViolated is the L* invariant that fired (empty if all held).
	InvariantViolated string
}

// ToolChannel is the per-EmissionClass termination abstraction.
// Implementations: ProbeToolChannel, FactToolChannel, ActionToolChannel,
// ExperimentToolChannel.
type ToolChannel interface {
	// Name returns the canonical channel identifier.
	Name() string
	// EmissionClass returns the EmissionClass this channel handles.
	EmissionClass() contracts.EmissionClass
	// Invariant returns the primary LTL-Lite termination invariant.
	Invariant() termination.TerminationInvariant
	// Accept decides whether to accept a tool call. Returns:
	//   - (true, nil)  → call proceeds
	//   - (false, nil) → call rejected silently (only in shadow mode)
	//   - (false, err) → call rejected with reason (enforce mode)
	// The bound check + permission cross-check (H8) happen here.
	Accept(ctx context.Context, call *ToolCall, state *termination.State) (bool, error)
	// OnResult processes the tool result. ProbeToolChannel uses this
	// for H9 OnResult behavior reclassification (same query > 3 → Probe).
	OnResult(ctx context.Context, call *ToolCall, result *ToolResult, state *termination.State) error
	// InjectPromptPressure is the soft-warning hook. ProbeToolChannel
	// injects a Schelling-focal-point prompt at remaining=5/3/2 (by task_kind).
	InjectPromptPressure(ctx context.Context, state *termination.State, call *ToolCall) error
	// Finalize closes the channel and returns the outcome.
	Finalize(ctx context.Context, state *termination.State) (*ChannelOutcome, error)
	// Snapshot returns a copy of the channel's mutable state. The caller
	// (Router) maintains the per-session state map and passes it back
	// on each Accept/OnResult.
	Snapshot() *termination.State
}

// ---- Shadow mode configuration ----

// Mode selects between shadow (log-only) and enforce (hard reject).
type Mode int

const (
	// ModeShadow logs would_reject without blocking. Used during the
	// 1-week rollout (H11 / P1-AC-5) before hard enforce.
	ModeShadow Mode = iota
	// ModeEnforce blocks rejected calls. The default after the shadow
	// gate (FP<5%) is cleared.
	ModeEnforce
)

// String returns the symbolic name.
func (m Mode) String() string {
	switch m {
	case ModeShadow:
		return "shadow"
	case ModeEnforce:
		return "enforce"
	}
	return "unknown"
}

// ---- ErrProbeToolChannelBoundExceeded ----

// ErrProbeToolChannelBoundExceeded is returned by ProbeToolChannel.Accept
// when the L4-BOUNDED-ITERATIONS invariant fires. The ChannelRouter
// surfaces this to the LLM as a "synthesize now" signal.
var ErrProbeToolChannelBoundExceeded = fmt.Errorf("toolchannel: probe bound exceeded")

// ---- Router ----

// Router routes a ToolCall to the appropriate ToolChannel based on
// ToolSpec.EmissionClass. It maintains the per-session state map.
//
// DSAFT: D7-S9-A50-T02.
type Router struct {
	channels map[contracts.EmissionClass]ToolChannel
	mode     Mode
	// perSession maps sessionID → channel name → State (mutable).
	perSession map[string]map[string]*termination.State
	// wouldRejectCount tracks shadow-mode rejections (H11 metric).
	wouldRejectCount int64
}

// NewRouter constructs a Router with the 4 canonical channels.
func NewRouter(mode Mode) *Router {
	return &Router{
		channels: map[contracts.EmissionClass]ToolChannel{
			contracts.EC_Fact:       NewFactToolChannel(),
			contracts.EC_Action:     NewActionToolChannel(),
			contracts.EC_Probe:      NewProbeToolChannel(),
			contracts.EC_Experiment: NewExperimentToolChannel(),
		},
		mode:       mode,
		perSession: make(map[string]map[string]*termination.State),
	}
}

// Register overrides the default channel for an EmissionClass. Used
// for testing (e.g. injecting a tighter Bound for unit tests).
func (r *Router) Register(ec contracts.EmissionClass, ch ToolChannel) {
	r.channels[ec] = ch
}

// Mode returns the current mode (shadow or enforce).
func (r *Router) Mode() Mode { return r.mode }

// SetMode switches between shadow and enforce at runtime.
func (r *Router) SetMode(m Mode) { r.mode = m }

// Route dispatches call to the channel for call.Spec.EmissionClass.
// Returns the Accept result and (in shadow mode) an indicator of
// whether the call would have been rejected in enforce mode.
func (r *Router) Route(ctx context.Context, call *ToolCall) (bool, *ToolResult, error) {
	ch, ok := r.channels[call.Spec.EmissionClass]
	if !ok {
		return false, nil, fmt.Errorf("toolchannel: no channel for EmissionClass=%v", call.Spec.EmissionClass)
	}

	state := r.getOrCreateState(call.SessionID, ch.Name())
	accepted, err := ch.Accept(ctx, call, state)

	// Shadow mode: any rejection (with or without error) is logged as
	// would_reject and suppressed. The hard-enforce behavior is the
	// future state; the shadow period only observes what *would* have
	// been rejected. (H11 / P1-AC-5)
	if !accepted && r.mode == ModeShadow {
		r.wouldRejectCount++
		return false, nil, nil
	}
	if !accepted {
		return false, nil, err
	}

	// In a full implementation, the Router would call the actual tool
	// runner here and return a real ToolResult. For this skeleton, the
	// caller is expected to invoke the tool and call OnResult/Finalize
	// explicitly. The Accept above is the治理 core.
	return true, nil, nil
}

// OnResult propagates a tool result to the appropriate channel.
func (r *Router) OnResult(ctx context.Context, call *ToolCall, result *ToolResult) error {
	ch, ok := r.channels[call.Spec.EmissionClass]
	if !ok {
		return fmt.Errorf("toolchannel: no channel for EmissionClass=%v", call.Spec.EmissionClass)
	}
	state := r.getOrCreateState(call.SessionID, ch.Name())
	state.IterationsUsed++
	if call.QueryFingerprint != "" && call.QueryFingerprint == state.QueryFingerprint {
		state.SameQueryCount++
	} else {
		state.QueryFingerprint = call.QueryFingerprint
		state.SameQueryCount = 1
	}
	return ch.OnResult(ctx, call, result, state)
}

// Finalize closes the channel for a session and returns the outcome.
func (r *Router) Finalize(ctx context.Context, sessionID string, ec contracts.EmissionClass) (*ChannelOutcome, error) {
	ch, ok := r.channels[ec]
	if !ok {
		return nil, fmt.Errorf("toolchannel: no channel for EmissionClass=%v", ec)
	}
	state := r.getOrCreateState(sessionID, ch.Name())
	return ch.Finalize(ctx, state)
}

// WouldRejectCount returns the number of calls that would have been
// rejected in shadow mode (H11 / P1-AC-5 metric).
func (r *Router) WouldRejectCount() int64 { return r.wouldRejectCount }

func (r *Router) getOrCreateState(sessionID, channelName string) *termination.State {
	byCh, ok := r.perSession[sessionID]
	if !ok {
		byCh = make(map[string]*termination.State)
		r.perSession[sessionID] = byCh
	}
	state, ok := byCh[channelName]
	if !ok {
		state = &termination.State{ChannelName: channelName}
		byCh[channelName] = state
	}
	return state
}

// ResetSession clears the per-session state map (call at session end).
func (r *Router) ResetSession(sessionID string) {
	delete(r.perSession, sessionID)
}
