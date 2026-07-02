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
	"encoding/json"
	"fmt"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce/tools"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// BuiltinSurface exposes the file I/O + search tools registered by
// tools.NewBuiltinToolRegistry (bash, read_file, write_file, edit_file,
// glob, grep). It is the safe-default surface; per-agent filters typically
// do not reduce this set (explore/plan/worker all need at least read).
//
// The surface is constructed from an existing *tools.ToolRegistry so
// the existing test suite for builtin tools (sandbox, edit, glob, grep) does
// not need to be rewritten.
//
// TOOL-SURFACE-1-A01-F07 (DM-20260618-002): bashAST is the optional
// BashASTPolicy used by CheckPermission. A nil policy means bash
// CheckPermission returns Allow (no AST enforcement) — this is the
// graceful-degradation path for unit tests that don't wire the policy.
type BuiltinSurface struct {
	reg     *tools.ToolRegistry
	bashAST *BashASTPolicy
}

// NewBuiltinSurface constructs a surface backed by a tool registry. If reg
// is nil, the surface is empty (no tools) but still safe to call.
func NewBuiltinSurface(reg *tools.ToolRegistry) *BuiltinSurface {
	return &BuiltinSurface{reg: reg}
}

// NewBuiltinSurfaceWithBashAST wires the AST policy (DM-20260618-002).
// Pass nil to keep CheckPermission in graceful-degradation mode.
func NewBuiltinSurfaceWithBashAST(reg *tools.ToolRegistry, bashAST *BashASTPolicy) *BuiltinSurface {
	return &BuiltinSurface{reg: reg, bashAST: bashAST}
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
		rOnly, dest, openW, concSafe := OrthogonalFlagFor(sc.Name)
				spec := contracts.ToolSpec{

			Name:            sc.Name,
			Description:     sc.Description,
			Parameters:      sc.Parameters,
			Risk:            s.reg.RiskLevel(sc.Name),
			ReadOnly:        rOnly,
			Destructive:     dest,
			OpenWorld:       openW,
			ConcurrencySafe: concSafe,
			DeferLoading:    ShouldDeferByDefault(sc.Name),
		
}
		ApplyV3Metadata(&spec, sc.Name)
		out = append(out, spec)
	}
	return out
}

// InterruptBehavior implements contracts.ToolSurface. All builtin tools are
// short-run, so they all block on ctx cancellation.
func (s *BuiltinSurface) InterruptBehavior(name string) contracts.InterruptMode {
	return InterruptBehaviorFor(name)
}

// RiskLevel implements contracts.ToolSurface.
func (s *BuiltinSurface) RiskLevel(name string) types.RiskLevel {
	if s.reg == nil {
		return ""
	}
	return s.reg.RiskLevel(name)
}

// Execute implements contracts.ToolSurface. It delegates to the
// underlying tools.ToolRegistry, preserving the existing
// behaviour for all builtin tools (including sandbox policy).
func (s *BuiltinSurface) Execute(ctx context.Context, name, input, workDir string) (*contracts.ToolResult, error) {
	if s.reg == nil {
		return &contracts.ToolResult{Error: "builtin: registry not initialized"}, nil
	}
	res, err := s.reg.Execute(ctx, tools.ToolCall{
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

// CheckPermission implements contracts.ToolSurface.
//
// TOOL-SURFACE-1-A01-F07 (DM-20260618-002): bash gets AST-level
// policy; every other builtin tool (read/write/edit/grep/glob) is
// Allow. A nil bashAST short-circuits to Allow (graceful degradation
// for unit tests that don't wire the policy).
func (s *BuiltinSurface) CheckPermission(_ context.Context, spec contracts.ToolSpec, input json.RawMessage) contracts.Decision {
	if spec.Name != "bash" {
		return contracts.DecisionAllow
	}
	if s.bashAST == nil {
		return contracts.DecisionAllow
	}
	var in struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return contracts.DecisionAsk
	}
	decision, _ := s.bashAST.Check(in.Command)
	return decision
}
