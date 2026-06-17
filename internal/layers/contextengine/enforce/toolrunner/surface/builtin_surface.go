// Package surface — D2 ToolSurface implementations (DM-20260617-007).
//
// Each surface wraps a logical group of tools behind the contracts.ToolSurface
// interface. Library packages (freefork, tracker, verify, etc.) do not depend
// on this package; the dependency direction is:
//
//	contracts (shared/contracts) ← surface (here) ← library
//
// DSAFT: TOOL-SURFACE-1-A03 (DM-20260617-007 devrix-tool-surface-contract)
package surface

import (
	"context"
	"fmt"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce/toolrunner"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// BuiltinSurface exposes the file I/O + search tools registered by
// toolrunner.NewBuiltinToolRegistry (bash, read_file, write_file, edit_file,
// glob, grep). It is the safe-default surface; per-agent filters typically
// do not reduce this set (explore/plan/worker all need at least read).
//
// The surface is constructed from an existing *toolrunner.ToolRegistry so
// the existing test suite for builtin tools (sandbox, edit, glob, grep) does
// not need to be rewritten.
type BuiltinSurface struct {
	reg *toolrunner.ToolRegistry
}

// NewBuiltinSurface constructs a surface backed by a tool registry. If reg
// is nil, the surface is empty (no tools) but still safe to call.
func NewBuiltinSurface(reg *toolrunner.ToolRegistry) *BuiltinSurface {
	return &BuiltinSurface{reg: reg}
}

// Name implements contracts.ToolSurface.
func (s *BuiltinSurface) Name() string { return "builtin" }

// Tools implements contracts.ToolSurface. WorkDir/sessionID are unused for
// builtins (they are looked up from ctx at execute time).
func (s *BuiltinSurface) Tools(_ context.Context, _, _ string) []contracts.ToolSpec {
	if s.reg == nil {
		return nil
	}
	schemas, err := s.reg.ListTools(context.Background(), "")
	if err != nil {
		return nil
	}
	out := make([]contracts.ToolSpec, 0, len(schemas))
	for _, sc := range schemas {
		out = append(out, contracts.ToolSpec{
			Name:        sc.Name,
			Description: sc.Description,
			Parameters:  sc.Parameters,
			Risk:        s.reg.RiskLevel(sc.Name),
		})
	}
	return out
}

// RiskLevel implements contracts.ToolSurface.
func (s *BuiltinSurface) RiskLevel(name string) types.RiskLevel {
	if s.reg == nil {
		return types.RiskLevelLow
	}
	return s.reg.RiskLevel(name)
}

// Execute implements contracts.ToolSurface. It delegates to the
// underlying toolrunner.ToolRegistry, preserving the existing
// behaviour for all builtin tools (including sandbox policy).
func (s *BuiltinSurface) Execute(ctx context.Context, name, input, workDir string) (*contracts.ToolResult, error) {
	if s.reg == nil {
		return &contracts.ToolResult{Error: "builtin: registry not initialized"}, nil
	}
	res, err := s.reg.Execute(ctx, toolrunner.ToolCall{
		ID:        "",
		Name:      name,
		Input:     input,
		RiskLevel: s.reg.RiskLevel(name),
	})
	if err != nil {
		return nil, fmt.Errorf("builtin: execute %s: %w", name, err)
	}
	if res == nil {
		return &contracts.ToolResult{Error: "builtin: nil result"}, nil
	}
	return &contracts.ToolResult{Output: res.Output, Error: res.Error}, nil
}
