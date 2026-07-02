package surface

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce/tools"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// PluginSurface wraps a fixed set of tools.PluginRunner instances and
// exposes them through contracts.ToolSurface. The dispatch table is built
// once at construction (O(N)) and Execute is O(1) by name lookup.
//
// This is the shared implementation for DelegateSurface and
// BackgroundTaskSurface — both are surface names over the same dispatch
// mechanism, just with different runner sets.
type PluginSurface struct {
	name    string
	runners map[string]tools.PluginRunner
	order   []string // preserves Tools() order across calls
}

// NewPluginSurface builds a surface from a name and a list of runners.
// Duplicates (by runner.Name()) are not deduplicated — the first one wins
// and the rest are dropped. Empty name is allowed (e.g. for surfaces that
// only need a logical grouping).
func NewPluginSurface(name string, runners []tools.PluginRunner) *PluginSurface {
	s := &PluginSurface{
		name:    name,
		runners: make(map[string]tools.PluginRunner, len(runners)),
		order:   make([]string, 0, len(runners)),
	}
	for _, r := range runners {
		if r == nil {
			continue
		}
		n := r.Name()
		if _, dup := s.runners[n]; dup {
			continue
		}
		s.runners[n] = r
		s.order = append(s.order, n)
	}
	return s
}

// Name implements contracts.ToolSurface.
func (s *PluginSurface) Name() string { return s.name }

// Tools implements contracts.ToolSurface. Returns specs in the order the
// runners were passed to NewPluginSurface (so tests can assert a stable
// shape rather than a map iteration).
func (s *PluginSurface) Tools(_ context.Context, _, _ string) []contracts.ToolSpec {
	if len(s.runners) == 0 {
		return nil
	}
	out := make([]contracts.ToolSpec, 0, len(s.order))
	for _, n := range s.order {
		r := s.runners[n]
		sc := r.Schema()
		rOnly, dest, openW, concSafe := OrthogonalFlagFor(sc.Name)
				spec := contracts.ToolSpec{

			Name:            sc.Name,
			Description:     sc.Description,
			Parameters:      sc.Parameters,
			Risk:            r.RiskLevel(),
			ReadOnly:        rOnly,
			Destructive:     dest,
			OpenWorld:       openW,
			ConcurrencySafe: concSafe,
		
}
		ApplyV3Metadata(&spec, sc.Name)
		out = append(out, spec)
	}
	return out
}

// InterruptBehavior implements contracts.ToolSurface. delegate_* tools are
// long-run (they spawn Dispatcher child agents), so they opt into
// InterruptCancel; everything else blocks.
func (s *PluginSurface) InterruptBehavior(name string) contracts.InterruptMode {
	return InterruptBehaviorFor(name)
}

// CheckPermission implements contracts.ToolSurface. The default for
// plugin-backed tools (delegate_*, task_*) is Allow; per-runner
// overrides can be added by extending PluginRunner with a CheckPermission
// hook (P2 follow-up — see DM-002 §3.3).
func (s *PluginSurface) CheckPermission(_ context.Context, _ contracts.ToolSpec, _ json.RawMessage) contracts.Decision {
	return contracts.DecisionAllow
}

// RiskLevel implements contracts.ToolSurface. For known names, returns the
// runner's reported level; for unknown names returns the defensive LOW
// default (consistent with the other surfaces).
func (s *PluginSurface) RiskLevel(name string) types.RiskLevel {
	if r, ok := s.runners[name]; ok {
		return r.RiskLevel()
	}
	return types.RiskLevelLow
}

// IsConcurrencySafe implements contracts.ToolSurface v4.
//
// DSAFT: D2-S15-A02-T17. PluginSurface is dynamic — runners are passed
// in at composition time and the v4 signature has no tool-name argument.
// We can't dispatch by input shape (delegate_*/task_* inputs differ per
// runner and the surface doesn't own a name->input map). The
// conservative default is `false` (sequential):
//
//   - delegate_* tools are never concurrency-safe (v2 truth table) —
//     false is correct.
//   - task_output IS concurrency-safe in theory (v2 truth table = true)
//     but treating it as sequential is safe; T18 partitionToolCalls
//     may later add a runner-level hook for finer dispatch.
//
// Surfacing the conservative default matches the v4 contract's
// "conservative on ambiguity" guidance and lets the call site assume
// the lower-concurrency option when in doubt.
func (s *PluginSurface) IsConcurrencySafe(_ json.RawMessage) bool {
	return false
}

// ToAutoClassifierInput implements contracts.ToolSurface v4. P2 stub
// default — returns "" to skip in classifier transcript. The dynamic
// tool catalog means we can't apply a per-tool projection here; T18
// partitionToolCalls will provide explicit tool name and the per-tool
// helper if the auto-mode classifier needs richer input.
func (s *PluginSurface) ToAutoClassifierInput(_ json.RawMessage) string {
	return ""
}

// Execute implements contracts.ToolSurface. Dispatches by tool name to
// the matching runner. Returns a ToolResult error envelope on unknown name
// or nil runner set — never a Go error — to match the contract used by
// every other surface in this package.
func (s *PluginSurface) Execute(ctx context.Context, name, input, workDir string) (*contracts.ToolResult, error) {
	if s.runners == nil {
		return &contracts.ToolResult{Error: fmt.Sprintf("%s: surface not initialized", s.name)}, nil
	}
	r, ok := s.runners[name]
	if !ok {
		return &contracts.ToolResult{Error: fmt.Sprintf("%s: unknown tool %q", s.name, name)}, nil
	}
	res, err := r.Execute(ctx, workDir, input)
	if err != nil {
		return nil, fmt.Errorf("%s: %s: %w", s.name, name, err)
	}
	if res == nil {
		return &contracts.ToolResult{Error: fmt.Sprintf("%s: %s: nil result", s.name, name)}, nil
	}
	return &contracts.ToolResult{Output: res.Output, Error: res.Error}, nil
}

// surfaceApplyV3MetadataSentinel is a grep-gate sentinel referenced by
// PluginSurface.ApplyV3Metadata. See background_task_surface.go and
// delegate_surface.go for the alias chain.
var surfaceApplyV3MetadataSentinel = struct{}{}
