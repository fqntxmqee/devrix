package execute

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/hardening"
	"github.com/devrix/devrix/internal/layers/orchestration/plan"
	"github.com/devrix/devrix/internal/layers/orchestration/wavescheduler"
)

// -----------------------------------------------------------------------------
// Channel interface + supporting types (PR-C2 surface)
// -----------------------------------------------------------------------------

// ToolResult is the minimal projection a Channel needs from a ToolRunner.
// PR-C7 will replace this with the full surface.ToolResult; PR-C2 keeps
// the shape minimal so tests can construct deterministic fakes.
//
// ExitCode is the tool's return code (0 = success). For tools without a
// numeric exit (e.g. some HTTP APIs), callers should map the response status
// into ExitCode (e.g. 200 → 0, 5xx → non-zero).
type ToolResult struct {
	ToolName    string
	ExitCode    int
	Output      string
	StartedAt   time.Time
	CompletedAt time.Time
	RetryCount  int
}

// ToolRunner is the pluggable execution surface Channels depend on. It is
// defined here (not imported from toolrunner/surface) to avoid a cross-PR
// hard dependency — PR-C2 lands before PR-C4 ToolSpec v3. PR-C7 will
// reconcile this with the full surface.Surface contract.
type ToolRunner interface {
	Invoke(ctx context.Context, req ToolRequest) (ToolResult, error)
}

// ToolRequest is the input to ToolRunner.Invoke. StepIdempotencyKey is
// required for any tool with side effects (PR-C2 AC: IdempotencyKey_Required
// is enforced at the Channel level for CommitChannel; full enforcement is
// PR-C7 via the IdempotencyCache).
type ToolRequest struct {
	SessionID       string
	ToolName        string
	Args            map[string]any
	IdempotencyKey  string
	StepID          string
}

// Channel is the per-PlanKind execution unit. The Execute method runs the
// Plan through the channel's strategy (synchronous / async / parallel /
// free-fork) and returns an Artifact consumable by Phase 4 Verify.
//
// Implementations:
//   - CommitChannel      (channel_commit.go): synchronous 1-Step direct tool call
//   - ProtocolChannel    (channel_protocol.go): sequential multi-Step with rollback
//   - ScenarioChannel    (channel_scenario.go): parallel probes with majority vote
//   - ExplorationChannel (channel_exploration.go): multi-agent free-fork
//
// All channels return *wavescheduler.Artifact (the PR-C1 4-class output)
// plus a non-nil error if anything failed. Partial completion is reported
// via the Artifact.SideEffectStatus field (SideEffectInflight /
// SideEffectRolledBack).
type Channel interface {
	// Name returns a short stable identifier (e.g. "commit", "protocol").
	// Used in error messages and metrics labels.
	Name() string

	// Supports reports whether the channel handles the given PlanKind.
	// The ChannelRegistry uses this to validate at Register-time.
	Supports(planKind plan.PlanKind) bool

	// Execute runs the Plan and returns the resulting Artifact. Errors here
	// are routed to the StrategyDecider (PR-C3) — Channels themselves never
	// decide whether to retry; they surface the failure.
	Execute(ctx context.Context, p *plan.Plan, req ChannelRequest) (*wavescheduler.Artifact, error)
}

// ChannelRequest is the contextual input handed to Channel.Execute. The
// SessionID is required.
//
// PriorVerdictKinds is intentionally typed as []string (not a Phase 4 enum)
// for PR-C2 because the Phase 4 VerdictKind enum has not landed yet. PR-C2
// only carries the values through to Channel implementations that need them;
// when Phase 4 lands, the type will be tightened via a type alias.
type ChannelRequest struct {
	SessionID        string
	PriorVerdictKinds []string
}

// ChannelRegistry maps PlanKind → Channel. Constructed once at wiring time
// (typically in the Executor in PR-C7), then frozen — Channels are
// effectively immutable for the lifetime of the registry.
//
// Convention: each PlanKind maps to exactly one Channel. The Registry
// validates the 1:1 invariant at Register-time and refuses to overwrite.
type ChannelRegistry struct {
	channels map[plan.PlanKind]Channel
}

// NewChannelRegistry constructs an empty registry.
func NewChannelRegistry() *ChannelRegistry {
	return &ChannelRegistry{
		channels: make(map[plan.PlanKind]Channel, 4),
	}
}

// Register binds a Channel to every PlanKind it Supports(). Returns
// ErrChannelUnsupported if the Channel supports no known PlanKind (a
// construction bug), or if a Channel was already registered for one of
// those PlanKinds (wiring conflict — distinct Channels claim the same kind).
func (r *ChannelRegistry) Register(c Channel) error {
	if c == nil {
		return fmt.Errorf("%w: nil channel", ErrChannelUnsupported)
	}
	for _, k := range allPlanKinds() {
		if !c.Supports(k) {
			continue
		}
		if existing, ok := r.channels[k]; ok {
			return fmt.Errorf("%w: planKind=%s already bound to channel=%s",
				ErrChannelUnsupported, k.String(), existing.Name())
		}
		r.channels[k] = c
	}
	return nil
}

// Get returns the Channel registered for the given PlanKind, or
// ErrChannelNotFound (wrapped via NewChannelNotFoundError) if none was
// registered. The router calls this and surfaces the error to the caller.
func (r *ChannelRegistry) Get(k plan.PlanKind) (Channel, error) {
	if !k.IsKnown() {
		return nil, NewChannelUnsupportedError("<router>", k.String())
	}
	c, ok := r.channels[k]
	if !ok {
		return nil, NewChannelNotFoundError(k.String())
	}
	return c, nil
}

// BoundKinds returns the PlanKinds currently registered. Used by tests and
// the PR-C7 wiring diagnostics.
func (r *ChannelRegistry) BoundKinds() []plan.PlanKind {
	out := make([]plan.PlanKind, 0, len(r.channels))
	for k := range r.channels {
		out = append(out, k)
	}
	return out
}

// Len returns the number of registered channels. Useful for tests asserting
// that all 4 PlanKinds were registered.
func (r *ChannelRegistry) Len() int {
	return len(r.channels)
}

// allPlanKinds returns the 4 named PlanKind values in canonical order.
// Iteration order is deterministic (sorted by uint8 value) so tests can
// assert Register-time bindings without map-iteration flakiness.
func allPlanKinds() []plan.PlanKind {
	return []plan.PlanKind{
		plan.CommitmentPlan,
		plan.ProtocolPlan,
		plan.ScenarioPlan,
		plan.ExplorationPlan,
	}
}

// -----------------------------------------------------------------------------
// ChannelRouter — the public Phase 3 PR-C2 surface
// -----------------------------------------------------------------------------

// ChannelRouter dispatches a Plan to the appropriate Channel based on
// Plan.Kind. It is the boundary between the Orchestrator (Phase 2 caller)
// and the 4 Channels (Phase 3 internals).
//
// The router is intentionally stateless: it delegates to ChannelRegistry
// for lookup and to Channel.Execute for execution. The StrategyDecider
// (PR-C3) will be inserted between Router.Get and Channel.Execute in
// a later PR.
type ChannelRouter struct {
	registry *ChannelRegistry
}

// NewChannelRouter constructs a router backed by the given registry. The
// caller owns the registry and may Register additional Channels before
// passing it in.
func NewChannelRouter(reg *ChannelRegistry) *ChannelRouter {
	return &ChannelRouter{registry: reg}
}

// Route looks up the Channel for Plan.Kind and invokes Execute. The
// returned Artifact is Phase 4 Verify input; the returned error (if any)
// is routed to the Orchestrator's error handler.
//
// Defensive checks before delegation:
//  1. Plan must not be nil.
//  2. Plan.Kind must be one of the 4 named kinds (IsKnown).
//  3. ChannelRegistry must have a Channel for Plan.Kind.
func (r *ChannelRouter) Route(ctx context.Context, p *plan.Plan, req ChannelRequest) (*wavescheduler.Artifact, error) {
	if r == nil || r.registry == nil {
		return nil, NewChannelNotFoundError("<nil-router>")
	}
	if p == nil {
		return nil, fmt.Errorf("%w", ErrChannelPlanNil)
	}
	// v6.0.0 S6-A48 P0: emit channel.route Span before channel lookup so Jaeger
	// records both successful routes and the rare Kind-mismatch / nil-plan paths.
	// Plan.Strength is float64 in [0,1]; surface as 3-decimal string for the Jaeger UI.
	score := strconv.FormatFloat(p.Strength, 'f', 3, 64)
	end := hardening.EmitChannelRoute(ctx, p.SessionID, p.Kind.String(), "", score, "false")
	if !p.Kind.IsKnown() {
		err := NewChannelUnsupportedError("<router>", p.Kind.String())
		end(err)
		return nil, err
	}
	ch, err := r.registry.Get(p.Kind)
	if err != nil {
		end(err)
		return nil, err
	}
	art, err := ch.Execute(ctx, p, req)
	// success path: close span (err may still be non-nil from ch.Execute)
	end(err)
	return art, err
}
