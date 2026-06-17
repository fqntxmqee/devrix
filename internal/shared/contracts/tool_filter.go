package contracts

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/devrix/devrix/internal/shared/types"
)

// FilterCtx is the input to a ToolFilter decision. It is a neutral struct
// (no D1/D2 internal types) so filters can compose across domains.
//
// DSAFT: TOOL-SURFACE-1-A02 (DM-20260617-007 devrix-tool-surface-contract)
type FilterCtx struct {
	SessionID     string
	AgentType     string // "main" | "explore" | "plan" | "fix" | "delegate" | "worker" | ...
	Mode          string // "plan_mode" | "yolo" | "loop_first" | "rule_orchestrate"
	RiskThreshold types.RiskLevel
}

// ToolFilter is the minimal unit of tool visibility policy. Implementations
// should be pure (no I/O, no side effects) so they can be composed and
// tested in isolation.
//
// DSAFT: TOOL-SURFACE-1-A02-F01
type ToolFilter interface {
	// Apply returns a subset of specs. Order is preserved. If specs is
	// nil or empty, the filter returns the same value (idempotent on
	// nil). Filters MUST NOT mutate the input slice.
	Apply(specs []ToolSpec, ctx FilterCtx) []ToolSpec
}

// Composite chains multiple filters in FIFO order. Each filter sees the
// output of the previous one. Order is significant: per-agent first,
// per-risk second, per-session third (a typical composition).
func Composite(filters ...ToolFilter) ToolFilter {
	return &compositeFilter{filters: filters}
}

type compositeFilter struct{ filters []ToolFilter }

func (c *compositeFilter) Apply(specs []ToolSpec, ctx FilterCtx) []ToolSpec {
	for _, f := range c.filters {
		if f == nil {
			continue
		}
		specs = f.Apply(specs, ctx)
	}
	return specs
}

// DeferDecision is the optional hook a filter can register to mark tools
// for defer-loading (NOT removal — DeferLoading=true keeps the tool
// discoverable via tool_search but omits the schema from the default
// system prompt).
//
// DSAFT: TOOL-SURFACE-1-A01-F08 (DM-20260618-003).
type DeferDecision interface {
	// ShouldDefer returns true when the spec's schema should be omitted
	// from the default system prompt. Runtime filters use this to defer
	// tools conditionally (e.g. plan_mode → defer all open-world tools).
	// tool_search itself MUST always return false.
	ShouldDefer(ctx context.Context, spec ToolSpec) bool
}

// DeferChain composes multiple DeferDecision hooks in FIFO order.
// Returns true if ANY hook returns true (logical OR).
//
// DSAFT: TOOL-SURFACE-1-A01-F08.
func DeferChain(decisions ...DeferDecision) DeferDecision {
	return &deferChain{decisions: decisions}
}

type deferChain struct{ decisions []DeferDecision }

func (d *deferChain) ShouldDefer(ctx context.Context, spec ToolSpec) bool {
	for _, dec := range d.decisions {
		if dec == nil {
			continue
		}
		if dec.ShouldDefer(ctx, spec) {
			return true
		}
	}
	return false
}

// AlwaysDefer returns a DeferDecision that defers tools matching the
// names list (exact match).
func AlwaysDefer(names ...string) DeferDecision {
	set := make(map[string]bool, len(names))
	for _, n := range names {
		set[n] = true
	}
	return &alwaysDefer{set: set}
}

type alwaysDefer struct{ set map[string]bool }

func (a *alwaysDefer) ShouldDefer(_ context.Context, spec ToolSpec) bool {
	return a.set[spec.Name]
}

// NeverDefer is a no-op DeferDecision (always false). Useful as a
// placeholder and in tests.
type NeverDefer struct{}

func (NeverDefer) ShouldDefer(_ context.Context, _ ToolSpec) bool { return false }

// Allow returns a filter that keeps only tools whose Name is in the
// allowlist. An empty allowlist keeps nothing.
func Allow(names ...string) ToolFilter {
	set := make(map[string]bool, len(names))
	for _, n := range names {
		set[n] = true
	}
	return &allowFilter{set: set}
}

type allowFilter struct{ set map[string]bool }

func (f *allowFilter) Apply(specs []ToolSpec, _ FilterCtx) []ToolSpec {
	if len(f.set) == 0 {
		return nil
	}
	out := make([]ToolSpec, 0, len(specs))
	for _, s := range specs {
		if f.set[s.Name] {
			out = append(out, s)
		}
	}
	return out
}

// Deny returns a filter that removes tools whose Name is in the
// blocklist. An empty blocklist passes everything through.
func Deny(names ...string) ToolFilter {
	set := make(map[string]bool, len(names))
	for _, n := range names {
		set[n] = true
	}
	return &denyFilter{set: set}
}

type denyFilter struct{ set map[string]bool }

func (f *denyFilter) Apply(specs []ToolSpec, _ FilterCtx) []ToolSpec {
	if len(f.set) == 0 {
		return specs
	}
	out := make([]ToolSpec, 0, len(specs))
	for _, s := range specs {
		if !f.set[s.Name] {
			out = append(out, s)
		}
	}
	return out
}

// ApplyFilters runs the filter chain against every surface's visible
// specs and returns a slice of *filteredSurface wrappers. The wrappers
// preserve the surface's Name/RiskLevel/Execute and gate Tools/Execute
// on the filtered list.
//
// WorkDir and sessionID for the inner surface.Tools() call are empty
// (surfaces typically only use those for per-dir file resolution; the
// filter chain only cares about spec identity).
func ApplyFilters(surfaces []ToolSurface, filters []ToolFilter, ctx FilterCtx) []ToolSurface {
	if len(filters) == 0 || len(surfaces) == 0 {
		return surfaces
	}
	out := make([]ToolSurface, len(surfaces))
	for i, s := range surfaces {
		specs := s.Tools(context.Background(), "", "")
		for _, f := range filters {
			if f == nil {
				continue
			}
			specs = f.Apply(specs, ctx)
		}
		out[i] = &filteredSurface{parent: s, visible: specs}
	}
	return out
}

// filteredSurface wraps a ToolSurface with a pre-computed visible spec
// list. Execute is gated: if name is not in visible, returns
// "tool not visible in current context" without calling the parent.
type filteredSurface struct {
	parent  ToolSurface
	visible []ToolSpec
}

func (f *filteredSurface) Name() string { return f.parent.Name() }

func (f *filteredSurface) Tools(_ context.Context, _, _ string) []ToolSpec {
	return f.visible
}

func (f *filteredSurface) RiskLevel(name string) types.RiskLevel {
	return f.parent.RiskLevel(name)
}

func (f *filteredSurface) Execute(ctx context.Context, name, input, workDir string) (*ToolResult, error) {
	for _, v := range f.visible {
		if v.Name == name {
			return f.parent.Execute(ctx, name, input, workDir)
		}
	}
	return &ToolResult{
		Error: fmt.Sprintf("tool %q not visible in current context (agent=%q, mode=%q)", name, ctxForLog(ctx), ""),
	}, nil
}

// InterruptBehavior delegates to the parent surface. The filter doesn't
// change the interrupt mode — only the visible set of tools.
func (f *filteredSurface) InterruptBehavior(name string) InterruptMode {
	return f.parent.InterruptBehavior(name)
}

// CheckPermission delegates to the parent surface. Visibility filtering
// is a different concern from permission: a hidden tool never reaches
// CheckPermission (turn_adapter filters on Tools() first), so the
// permission check is identical to the parent's.
func (f *filteredSurface) CheckPermission(ctx context.Context, spec ToolSpec, input json.RawMessage) Decision {
	return f.parent.CheckPermission(ctx, spec, input)
}

// ctxForLog extracts a session/agent hint from ctx for error messages.
// Returns "" if no usable value is found; this is intentionally cheap
// and best-effort.
func ctxForLog(_ context.Context) string { return "" }
