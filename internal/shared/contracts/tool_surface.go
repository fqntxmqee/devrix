package contracts

import (
	"context"

	"github.com/devrix/devrix/internal/shared/types"
)

// ToolSpec is a neutral LLM tool schema (decoupled from D3 llmgateway.ToolCall
// and D2 toolrunner.ToolSchema). All cross-layer tool exchanges use ToolSpec.
//
// DSAFT: TOOL-SURFACE-1-A01 (DM-20260617-007 devrix-tool-surface-contract)
type ToolSpec struct {
	Name        string
	Description string
	Parameters  string // JSON Schema
	Risk        types.RiskLevel
}

// ToolResult is the return type of ToolSurface.Execute.
//
// DSAFT: TOOL-SURFACE-1-A01-F04
type ToolResult struct {
	Output string
	Error  string
}

// ToolSurface is a discoverable entry point for a group of related tools.
//
// Per devrix Facet Decomposition (DM-020 D-c + architecture-design.md §1.1),
// ToolSurface is a 拆面 contract exposed to D2 (consumer) by D2 surface
// implementations. Library packages (freefork / tracker / verify / etc.) do
// not depend on this contract — the dependency direction is:
//
//	contracts ← surface (in toolrunner/surface) ← library
//
// Design principles:
//   - Accept interfaces, return structs (ToolSpec / ToolResult are structs)
//   - 4 methods, each 1-3 lines in typical implementations
//   - Does not hold ctx; Execute / Tools accept ctx
//   - Does NOT make permission decisions (IPermissionGate runs in
//     turn_adapter.ExecuteRound, BEFORE surf.Execute)
//
// DSAFT: TOOL-SURFACE-1-A01 (DM-20260617-007 devrix-tool-surface-contract)
type ToolSurface interface {
	// Name returns the surface identifier (used in devrix.yaml config,
	// log tags, and `devrix tool list` output).
	Name() string

	// Tools returns the list of tools this surface exposes for the given
	// (workDir, sessionID) context. Implementations may filter
	// conditionally (e.g. LSPToolSurface checks lsp.enabled).
	//
	// The returned slice should be deterministic for stable LLM tool
	// schema hashing (callers may cache it per session).
	Tools(ctx context.Context, workDir, sessionID string) []ToolSpec

	// RiskLevel returns the RiskLevel for a single tool name. Unknown
	// names return types.RiskLevelLow (defensive default).
	//
	// Called by turn_adapter.ExecuteRound to populate
	// IPermissionGate.Request's risk argument.
	RiskLevel(name string) types.RiskLevel

	// Execute dispatches a single tool call through the surface's
	// internal mechanism. Returns ToolResult{Output, Error}; non-empty
	// Error means the caller should not block.
	//
	// workDir and sessionID are passed explicitly (not via ctx value) so
	// surfaces do not need to know about D1/D2 ctx conventions.
	Execute(ctx context.Context, name, input, workDir string) (*ToolResult, error)
}
