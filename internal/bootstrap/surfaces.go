package bootstrap

import (
	"github.com/devrix/devrix/internal/layers/contextengine/enforce/toolrunner"
	"github.com/devrix/devrix/internal/layers/contextengine/enforce/toolrunner/surface"
	"github.com/devrix/devrix/internal/layers/observability/diagnose/tracker"
	"github.com/devrix/devrix/internal/layers/orchestration/toolpolicy"
	"github.com/devrix/devrix/internal/shared/contracts"
)

// SurfaceBuildOpts holds the inputs needed to construct the canonical
// list of ToolSurface instances. Fields that are nil/zero are skipped
// (e.g. nil lspCfg → no LSPToolSurface).
//
// DSAFT: TOOL-SURFACE-1-A03-F04 (DM-20260617-007 devrix-tool-surface-contract)
type SurfaceBuildOpts struct {
	ToolReg   *toolrunner.ToolRegistry  // BuiltinSurface input
	LSPConfig *toolrunner.LSPConfig     // LSPToolSurface input (nil → omit)
	Tracker   *tracker.Tracker          // TrackerSurface input (nil → omit)
	Forker    toolrunner.FreeForkerFunc // FreeForkSurface input (nil → omit)
	WorkDir   string                    // for VerifySurface fallback (optional)
}

// BuildSurfaces assembles the canonical surface list for a main/per-agent
// engine. The order is significant: surfaces are tried in the order they
// appear in the list during dispatch (W9: turn_adapter.findSurface
// short-circuits on first match).
//
// Surfaces that are unconfigured (nil deps) are silently dropped from
// the list — callers should check the length if they need to assert
// "all 7 surfaces present".
func BuildSurfaces(opts SurfaceBuildOpts) []contracts.ToolSurface {
	var out []contracts.ToolSurface
	if opts.ToolReg != nil {
		out = append(out, surface.NewBuiltinSurface(opts.ToolReg))
	}
	// W11 phase 2c: LSP surface is added unconditionally so the lsp tool
	// schema is in the LLM tool list even when LSP is disabled. The
	// surface itself reports the disabled state at Execute time.
	out = append(out, surface.NewLSPToolSurface(opts.LSPConfig))
	if opts.Tracker != nil {
		out = append(out, surface.NewTrackerSurface(opts.Tracker))
	}
	if opts.Forker != nil {
		out = append(out, surface.NewFreeForkSurface(opts.Forker))
	}
	// VerifySurface is stateless — always safe to add.
	out = append(out, surface.NewVerifySurface())
	return out
}

// DefaultFilters returns the canonical per-mode filter chain. The
// returned slice is in FIFO order (applied left to right). For the
// main engine, callers typically pass nil (no filtering) — this is the
// chain used by per-agent engines.
//
// Current chain (from design.md §2.5 / §2.6):
//
//	toolpolicy.AsToolFilter  (per-agent: hide delegate_*, read-only workers)
//	filter.NewPerRiskFilter  (per-mode: threshold cap)
//
// PerSessionFilter is P1 and not yet implemented.
func DefaultFilters() []contracts.ToolFilter {
	return []contracts.ToolFilter{
		toolpolicy.AsToolFilter(),
	}
}
