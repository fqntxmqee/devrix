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
	"strings"

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

// IsConcurrencySafe implements contracts.ToolSurface v4.
//
// DSAFT: D2-S15-A02-T17. The 4 builtin tools that need per-input logic:
//   - bash       → IsReadOnlyBashCommand (per-input command string check)
//   - read_file  → 恒 true (AC18 8K 回归锁 removed; size-agnostic read-only)
//   - write_file → 恒 false (write op, must serialize)
//   - edit_file  → 恒 false (per design.md 同 target 路径 false; batch-level
//                  path-merge is T18 partitionToolCalls responsibility, PR-B)
//
// grep / glob fall back to the v2 static bool via IsConcurrencySafeForBuiltinTool
// (which delegates to DefaultIsConcurrencySafeFor). All other builtin tools
// (none in current registry) would also fall back to default.
func (s *BuiltinSurface) IsConcurrencySafe(input json.RawMessage) bool {
	// Get the current tool name from the latest tool call. BuiltinSurface
	// holds multiple tools (bash, read_file, write_file, edit_file, grep,
	// glob), so we need to know which one we're deciding for. The interface
	// only passes input — the call site (partitionToolCalls T18) provides
	// the tool name via surface.Tools() lookup.
	//
	// For the per-input decision, we use a different path: look at the
	// input's "command" / "file_path" / "path" shape to dispatch.
	//
	// However, the cleanest per-tool dispatch needs the tool name. The
	// v4 interface signature (input only) is a known limitation; we
	// address it by inspecting the input's first JSON key.
	return isConcurrencySafeFromInputShape(input)
}

// isConcurrencySafeFromInputShape dispatches to the right override based
// on the input's JSON shape. This is a workaround for the v4 interface
// signature (input only, no tool name) — partitionToolCalls (T18) will
// provide the tool name explicitly via a side helper; for now we infer
// from the input shape (each tool has a distinctive first field).
//
// Shape detection:
//   - bash       → {"command": "..."}
//   - read_file  → {"file_path": "..."} or {"path": "..."}
//   - write_file → {"file_path": "..."} or {"path": "..."}
//   - edit_file  → {"file_path": "..."} or {"path": "..."}
//   - grep/glob  → {"pattern": "..."}
func isConcurrencySafeFromInputShape(input json.RawMessage) bool {
	// Quick first-byte sniff to avoid unmarshal cost.
	if len(input) == 0 {
		return false // empty input → sequential (conservative)
	}
	// Detect bash by looking for "command" key.
	if hasJSONKey(input, "command") {
		return IsConcurrencySafeForBuiltinTool("bash", input)
	}
	// Detect read_file vs write_file vs edit_file by tool name from input
	// shape; they all use file_path/path. We can't disambiguate by shape
	// alone, so we use a marker — but the caller (T18) will pass tool
	// name explicitly. For PR-A we conservatively return false for
	// file-path inputs (write/edit default safe).
	//
	// TODO(D2-S15-A02-T18): replace this heuristic with explicit tool
	// name from partitionToolCalls call site.
	if hasJSONKey(input, "file_path") || hasJSONKey(input, "path") {
		// For PR-A the safe default is: read_file is safe, write/edit are
		// not. We can't tell from shape alone, so we return false (serial)
		// to be safe. The T18 partitionToolCalls will provide explicit
		// tool name and use the per-tool dispatch.
		return false
	}
	// grep / glob
	return IsConcurrencySafeForBuiltinTool("grep", input)
}

// hasJSONKey returns true if the JSON object contains the given key.
// Lightweight: scans raw bytes for `"key"` or `"key":` to avoid unmarshal cost.
func hasJSONKey(input json.RawMessage, key string) bool {
	needle := `"` + key + `"`
	return strings.Contains(string(input), needle)
}

// ToAutoClassifierInput implements contracts.ToolSurface v4.
//
// DSAFT: D2-S15-A02-T17. Projects the tool call into the compact string
// the auto-mode classifier consumes. See ToAutoClassifierInputForBuiltinTool
// for the per-tool projection.
func (s *BuiltinSurface) ToAutoClassifierInput(input json.RawMessage) string {
	if hasJSONKey(input, "command") {
		return ToAutoClassifierInputForBuiltinTool("bash", input)
	}
	if hasJSONKey(input, "file_path") || hasJSONKey(input, "path") {
		// Same shape-dispatch limitation as IsConcurrencySafe above.
		// T18 partitionToolCalls will provide explicit tool name; for
		// now return the file_path/path as the projection (read_file
		// and edit_file both project to the file path).
		var in struct {
			FilePath string `json:"file_path"`
			Path     string `json:"path"`
		}
		if err := json.Unmarshal(input, &in); err != nil {
			return string(input)
		}
		if in.FilePath != "" {
			return in.FilePath
		}
		return in.Path
	}
	return ToAutoClassifierInputForBuiltinTool("grep", input)
}
