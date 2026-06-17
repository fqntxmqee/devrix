package surface

import (
	"context"
	"fmt"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce/toolrunner"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// PluginSurface wraps a fixed set of toolrunner.PluginRunner instances and
// exposes them through contracts.ToolSurface. The dispatch table is built
// once at construction (O(N)) and Execute is O(1) by name lookup.
//
// This is the shared implementation for DelegateSurface and
// BackgroundTaskSurface — both are surface names over the same dispatch
// mechanism, just with different runner sets.
type PluginSurface struct {
	name    string
	runners map[string]toolrunner.PluginRunner
	order   []string // preserves Tools() order across calls
}

// NewPluginSurface builds a surface from a name and a list of runners.
// Duplicates (by runner.Name()) are not deduplicated — the first one wins
// and the rest are dropped. Empty name is allowed (e.g. for surfaces that
// only need a logical grouping).
func NewPluginSurface(name string, runners []toolrunner.PluginRunner) *PluginSurface {
	s := &PluginSurface{
		name:    name,
		runners: make(map[string]toolrunner.PluginRunner, len(runners)),
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
		out = append(out, contracts.ToolSpec{
			Name:        sc.Name,
			Description: sc.Description,
			Parameters:  sc.Parameters,
			Risk:        r.RiskLevel(),
		})
	}
	return out
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
