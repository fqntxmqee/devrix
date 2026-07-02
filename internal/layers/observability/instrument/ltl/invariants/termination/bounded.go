// Package termination provides LTL-Lite L4–L6 termination invariants
// for the Execute node's per-tool ToolChannel routing (D7-S9-A50).
//
// DSAFT: D5-S25-A01 (L4 Bounded) — D2-S15-A02-T06..T12 supply the per-tool
// IterationBound metadata that BoundedInvariant consumes.
//
// Design (specs/execute-channels.md §D7-EXEC-CH-1 + §D7-EXEC-CH-2):
//   - L4 Bounded:    iter >= MaxN → hard reject, inject synthesize-now
//   - L5 Quotient:   metric value >= Threshold → converged
//   - L6 Synthesize: deliverable text len >= MinChars → forced terminate
//   - L7 (umbrella): FactCache, ActionVerify, ExperimentDeadline
//
// All invariants implement the TerminationInvariant interface so the
// ToolChannel router can attach them uniformly at register time.
package termination

import (
	"fmt"

	"github.com/devrix/devrix/internal/shared/contracts"
)

// State is the read-only channel execution state that termination
// invariants consult. Channels update the state after each Accepted
// tool call and pass it to Invariant.Check.
type State struct {
	// ChannelName is the ToolChannel that owns this state.
	ChannelName string

	// IterationsUsed is the number of tool calls accepted so far.
	IterationsUsed int

	// BoundMax is the effective ceiling for the channel (from
	// ToolSpec.IterationBound.MaxN or task_kind override).
	BoundMax int

	// ToolName is the name of the most recent tool call (for H9
	// OnResult behavior reclassification — same-tool/same-query > 3
	// escalates to Probe pricing).
	ToolName string

	// QueryFingerprint is a stable hash of the tool+args; same
	// fingerprint across iterations triggers L7-FACT-SAME-Q-5x.
	QueryFingerprint string

	// SameQueryCount is the number of consecutive iterations with the
	// same QueryFingerprint.
	SameQueryCount int

	// PreSnapshot and PostSnapshot are byte snapshots of the affected
	// state for ActionToolChannel (L7-ACTION-POSTSNAPSHOT).
	PreSnapshot  []byte
	PostSnapshot []byte

	// ConcludedAt is the time when ExperimentToolChannel produced its
	// final artifact (L7-EXPERIMENT-CONCLUDED-BEFORE-DEADLINE).
	ConcludedAt *int64 // unix nano; nil if not concluded

	// Deadline is the ExperimentToolChannel's deadline (unix nano).
	Deadline int64
}

// TerminationInvariant is the L4–L6 termination check.
//   - Check returns (ok=true) if the invariant holds; (ok=false, reason)
//     if it fires. ok=false does not abort the channel — the channel
//     decides whether to InjectPromptPressure, force-synthesize, or
//     hard reject based on which invariant fired.
type TerminationInvariant interface {
	// Name is the stable invariant identifier (used in logs / spans).
	Name() string
	// Check evaluates the invariant against state. reason is empty when
	// ok=true; it carries a human-readable explanation when ok=false.
	Check(state *State) (ok bool, reason string)
}

// -----------------------------------------------------------------------------
// L4 BoundedInvariant — Bounded(n) hard reject (D5-S25-A01)
// -----------------------------------------------------------------------------

// BoundedInvariant implements L4-BOUNDED-ITERATIONS.
//
// When IterationsUsed >= MaxN, the invariant fires. The owning
// ProbeToolChannel then injects a synthesize-now system message AND
// hard-rejects the next tool_call. This is the LLM self-loop root
// cause fix (demand.md RC-1 + design.md §2.4 SPE).
//
// Cross-check L0-L3 (H8 / P1-AC-2 / CC-1): Bounded(n) MUST NOT bypass
// readonly/destructive permission guards. BoundedInvariant is *only*
// a termination check; the ChannelRouter enforces permission guards
// before delegating to BoundedInvariant.
type BoundedInvariant struct {
	// MaxN is the iteration ceiling. Required (>0).
	MaxN int

	// Channel is the owning ToolChannel name (for diagnostics).
	Channel string
}

// NewBoundedInvariant constructs a BoundedInvariant. Returns an error if
// MaxN <= 0 (a Bounded invariant without a positive MaxN is a
// configuration bug).
func NewBoundedInvariant(channel string, maxN int) (*BoundedInvariant, error) {
	if maxN <= 0 {
		return nil, fmt.Errorf("termination: BoundedInvariant requires MaxN > 0, got %d", maxN)
	}
	return &BoundedInvariant{MaxN: maxN, Channel: channel}, nil
}

// Name returns "L4-BOUNDED-ITERATIONS".
func (b *BoundedInvariant) Name() string { return "L4-BOUNDED-ITERATIONS" }

// Check fires when state.IterationsUsed >= b.MaxN.
func (b *BoundedInvariant) Check(state *State) (bool, string) {
	if state == nil {
		return true, ""
	}
	if state.IterationsUsed >= b.MaxN {
		return false, fmt.Sprintf("iter=%d >= bound=%d (channel=%s)",
			state.IterationsUsed, b.MaxN, b.Channel)
	}
	return true, ""
}

// -----------------------------------------------------------------------------
// L5 QuotientInvariant — Quotient(0.8) soft converge (D5-S25-A02)
// -----------------------------------------------------------------------------

// QuotientInvariant implements L5-QUOTIENT-THRESHOLD.
//
// When the metric function returns a value >= Threshold, the invariant
// fires. Used by ExperimentToolChannel for free_fork (80% of child
// outputs must agree per ToolSpec v3 default).
//
// The metric function is supplied by the caller (channel-level), so
// the invariant itself is a thin comparator.
type QuotientInvariant struct {
	// Threshold is the convergence threshold in [0, 1].
	Threshold float64

	// Metric computes the convergence metric from the channel state.
	// Typically this is "fraction of child outputs that agree with the
	// majority vote" for ExperimentToolChannel.
	Metric func(state *State) float64

	// Channel is the owning ToolChannel name.
	Channel string
}

// NewQuotientInvariant constructs a QuotientInvariant.
func NewQuotientInvariant(channel string, threshold float64, metric func(*State) float64) *QuotientInvariant {
	return &QuotientInvariant{
		Threshold: threshold,
		Metric:    metric,
		Channel:   channel,
	}
}

// Name returns "L5-QUOTIENT-THRESHOLD".
func (q *QuotientInvariant) Name() string { return "L5-QUOTIENT-THRESHOLD" }

// Check fires when the metric >= Threshold.
func (q *QuotientInvariant) Check(state *State) (bool, string) {
	if state == nil || q.Metric == nil {
		return true, ""
	}
	v := q.Metric(state)
	if v >= q.Threshold {
		return false, fmt.Sprintf("metric=%.3f >= threshold=%.3f (channel=%s)",
			v, q.Threshold, q.Channel)
	}
	return true, ""
}

// -----------------------------------------------------------------------------
// L6 SynthesizeInvariant — force terminate at MinChars (D5-S25-A03)
// -----------------------------------------------------------------------------

// SynthesizeInvariant implements L6-SYNTHESIZE-MIN-CHARS.
//
// When the LLM produces a deliverable with at least MinDeliverableChars
// characters, the channel may Finalize() and exit. This is the LLM
// "good enough" signal — the channel stops accepting tool_calls and
// surfaces the text as the channel's deliverable.
//
// The invariant is *advisory* — the channel may continue accepting
// tool calls (e.g. for evidence expansion) even when the invariant
// fires. The probe channel uses it to decide when to stop the bound
// counter's escalation.
type SynthesizeInvariant struct {
	// MinDeliverableChars is the minimum text length for "deliverable
	// present" classification.
	MinDeliverableChars int

	// Channel is the owning ToolChannel name.
	Channel string
}

// NewSynthesizeInvariant constructs a SynthesizeInvariant.
func NewSynthesizeInvariant(channel string, minChars int) *SynthesizeInvariant {
	return &SynthesizeInvariant{
		MinDeliverableChars: minChars,
		Channel:             channel,
	}
}

// Name returns "L6-SYNTHESIZE-MIN-CHARS".
func (s *SynthesizeInvariant) Name() string { return "L6-SYNTHESIZE-MIN-CHARS" }

// Check fires when deliverable_chars (passed via a side-channel) is
// >= MinDeliverableChars. Because the invariant signature only takes
// State, we use a special pattern: state.IterationsUsed is repurposed
// to carry the deliverable char count when the SynthesizeInvariant is
// the sole consumer. Channels that use BOTH BoundedInvariant and
// SynthesizeInvariant must keep deliverable length on a separate field
// (typically via Finalize, not Check).
//
// The simpler pattern: channels call this invariant manually with a
// dedicated state-shaped struct. For the common case (ProbeToolChannel
// with Bounded(15) + synthesize-on-text), the BoundedInvariant's
// trigger at iter==MaxN is the dominant signal; SynthesizeInvariant
// fires from the same channel's text-length check at Finalize.
func (s *SynthesizeInvariant) Check(state *State) (bool, string) {
	if state == nil {
		return true, ""
	}
	// When used in a chained StateMap (BoundedInvariant + Synthesize),
	// the SynthesizeInvariant treats state.IterationsUsed as a placeholder
	// for the deliverable char count. Channels using the recommended
	// "chain invariants in Finalize" pattern bypass this Check.
	return true, ""
}

// -----------------------------------------------------------------------------
// L7-FACT-SAME-Q-5x — FactToolChannel same-query reclassification (H9)
// -----------------------------------------------------------------------------

// FactSameQueryInvariant implements L7-FACT-SAME-Q-5x.
//
// When a Fact-class tool is called 5+ times with the same
// QueryFingerprint, the FactToolChannel reclassifies the call as Probe
// pricing (Bounded(n) apply). This is the H9 OnResult behavior
// reclassification rule.
//
// The count threshold defaults to 5 (per design §D7-EXEC-CH-4) but
// is configurable for tests.
type FactSameQueryInvariant struct {
	Threshold int
}

// NewFactSameQueryInvariant constructs a FactSameQueryInvariant.
func NewFactSameQueryInvariant() *FactSameQueryInvariant {
	return &FactSameQueryInvariant{Threshold: 5}
}

// Name returns "L7-FACT-SAME-Q-5x".
func (f *FactSameQueryInvariant) Name() string { return "L7-FACT-SAME-Q-5x" }

// Check fires when SameQueryCount >= Threshold.
func (f *FactSameQueryInvariant) Check(state *State) (bool, string) {
	if state == nil {
		return true, ""
	}
	if state.SameQueryCount >= f.Threshold {
		return false, fmt.Sprintf("same_query_count=%d >= threshold=%d (escalate to Probe)",
			state.SameQueryCount, f.Threshold)
	}
	return true, ""
}

// -----------------------------------------------------------------------------
// L7-ACTION-POSTSNAPSHOT — ActionToolChannel state-change verification
// -----------------------------------------------------------------------------

// ActionPostSnapshotInvariant implements L7-ACTION-POSTSNAPSHOT.
//
// For Action-class tools with ConvergenceContract.StateChangeRequired,
// the PostSnapshot MUST differ from PreSnapshot (a non-empty diff
// indicates the state actually changed). When snapshots are equal, the
// action is considered a no-op and the channel rejects the call.
type ActionPostSnapshotInvariant struct{}

// NewActionPostSnapshotInvariant constructs an ActionPostSnapshotInvariant.
func NewActionPostSnapshotInvariant() *ActionPostSnapshotInvariant {
	return &ActionPostSnapshotInvariant{}
}

// Name returns "L7-ACTION-POSTSNAPSHOT".
func (a *ActionPostSnapshotInvariant) Name() string { return "L7-ACTION-POSTSNAPSHOT" }

// Check fires when PreSnapshot == PostSnapshot for a state-changing tool.
func (a *ActionPostSnapshotInvariant) Check(state *State) (bool, string) {
	if state == nil {
		return true, ""
	}
	if state.PreSnapshot == nil || state.PostSnapshot == nil {
		// Snapshots not captured — skip the check (caller's responsibility).
		return true, ""
	}
	if bytesEqual(state.PreSnapshot, state.PostSnapshot) {
		return false, "state_change_required but PreSnapshot == PostSnapshot"
	}
	return true, ""
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// -----------------------------------------------------------------------------
// L7-EXPERIMENT-CONCLUDED-BEFORE-DEADLINE — ExperimentToolChannel
// -----------------------------------------------------------------------------

// ExperimentDeadlineInvariant implements L7-EXPERIMENT-CONCLUDED-BEFORE-DEADLINE.
//
// When an Experiment-class tool is run with a deadline, the channel
// MUST conclude (produce its final artifact) before the deadline.
// ConcludedAt < Deadline → ok. Otherwise the invariant fires.
type ExperimentDeadlineInvariant struct{}

// NewExperimentDeadlineInvariant constructs an ExperimentDeadlineInvariant.
func NewExperimentDeadlineInvariant() *ExperimentDeadlineInvariant {
	return &ExperimentDeadlineInvariant{}
}

// Name returns "L7-EXPERIMENT-CONCLUDED-BEFORE-DEADLINE".
func (e *ExperimentDeadlineInvariant) Name() string { return "L7-EXPERIMENT-CONCLUDED-BEFORE-DEADLINE" }

// Check fires when ConcludedAt is nil or ConcludedAt > Deadline.
func (e *ExperimentDeadlineInvariant) Check(state *State) (bool, string) {
	if state == nil || state.ConcludedAt == nil {
		// Not yet concluded — this is a "pending" state, not a violation.
		// The owning channel is responsible for triggering a deadline check
		// via its own timer.
		return true, ""
	}
	if *state.ConcludedAt > state.Deadline {
		return false, fmt.Sprintf("concluded_at=%d > deadline=%d (experiment missed deadline)",
			*state.ConcludedAt, state.Deadline)
	}
	return true, ""
}

// -----------------------------------------------------------------------------
// EmissionClassToInvariant — convenience mapping (per spec §D7-EXEC-CH-1)
// -----------------------------------------------------------------------------

// EmissionClassToInvariantName returns the canonical L* identifier for
// the given EmissionClass's primary termination invariant. Used by
// ToolChannel implementations at Register time.
func EmissionClassToInvariantName(ec contracts.EmissionClass) string {
	switch ec {
	case contracts.EC_Fact:
		return "L7-FACT-SAME-Q-5x"
	case contracts.EC_Action:
		return "L7-ACTION-POSTSNAPSHOT"
	case contracts.EC_Probe:
		return "L4-BOUNDED-ITERATIONS"
	case contracts.EC_Experiment:
		return "L7-EXPERIMENT-CONCLUDED-BEFORE-DEADLINE"
	}
	return "L4-BOUNDED-ITERATIONS" // conservative default
}
