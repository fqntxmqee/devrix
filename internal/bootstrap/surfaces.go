package bootstrap

import (
	"context"
	"sort"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce/tools"
	"github.com/devrix/devrix/internal/layers/contextengine/enforce/tools/surface"
	"github.com/devrix/devrix/internal/layers/observability/diagnose/tracker"
	"github.com/devrix/devrix/internal/layers/orchestration/decisionplanning"
	"github.com/devrix/devrix/internal/shared/contracts"
)

// SurfaceBuildOpts holds the inputs needed to construct the canonical
// list of ToolSurface instances. Fields that are nil/zero are skipped
// (e.g. nil lspCfg → no LSPToolSurface).
//
// DSAFT: TOOL-SURFACE-1-A03-F04 (DM-20260617-007 devrix-tool-surface-contract)
type SurfaceBuildOpts struct {
	ToolReg   *tools.ToolRegistry  // BuiltinSurface input
	LSPConfig *tools.LSPConfig     // LSPToolSurface input (nil → omit)
	Tracker   *tracker.Tracker          // TrackerSurface input (nil → omit)
	Forker    tools.FreeForkerFunc // FreeForkSurface input (nil → omit)
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
//
// TOOL-SURFACE-1-A01-F03 (DM-20260618-001 devrix-tool-spec-enrichment):
// the returned slice is sorted by Name() so the dispatch order is
// stable across processes (required for the LLM prompt cache —
// reordering surfaces changes the tool list hash).
//
// TOOL-SURFACE-1-A02 (DM-20260618-003 devrix-surface-lazy-loading):
// ToolSearchSurface is appended LAST after the catalog of all
// non-tool_search specs has been collected. The catalog is computed by
// calling each surface's Tools(); tool_search itself is excluded.
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
	// AskUserQuestionSurface (DM-20260618-006 devrix-ask-user-question) is
	// stateless — always safe to add. Sender is wired separately by
	// bootstrap via SetAskUserQuestionSender so the surface can run
	// without a gateway (e.g. in unit tests).
	out = append(out, surface.NewAskUserQuestionSurface())
	sort.Slice(out, func(i, j int) bool { return out[i].Name() < out[j].Name() })

	// TOOL-SURFACE-1-A02: build the deferred-tool catalog for
	// tool_search. Catalog is the union of every non-tool_search spec
	// emitted by every surface. ToolSearchSurface is appended LAST so
	// findSurface() short-circuits on it ONLY when the caller explicitly
	// asks for "tool_search" (it returns "" for every other name).
	allSpecs := collectAllSpecs(out)
	out = append(out, surface.NewToolSearchSurface(allSpecs))
	sort.Slice(out, func(i, j int) bool { return out[i].Name() < out[j].Name() })
	return out
}

// collectAllSpecs returns the union of Tools() across every surface.
// ToolSearchSurface specs are excluded (it would otherwise recurse).
func collectAllSpecs(surfaces []contracts.ToolSurface) []contracts.ToolSpec {
	var out []contracts.ToolSpec
	for _, s := range surfaces {
		if s == nil {
			continue
		}
		if s.Name() == surface.ToolSearchSurfaceName {
			continue
		}
		out = append(out, s.Tools(context.Background(), "", "")...)
	}
	return out
}

// DefaultFilters returns the canonical per-mode filter chain. The
// returned slice is in FIFO order (applied left to right). For the
// main engine, callers typically pass nil (no filtering) — this is the
// chain used by per-agent engines.
//
// Current chain (from design.md §2.5 / §2.6):
//
//	decisionplanning.AsToolFilter  (per-agent: hide delegate_*, read-only workers)
//	filter.NewPerRiskFilter  (per-mode: threshold cap)
//
// PerSessionFilter is P1 and not yet implemented.
func DefaultFilters() []contracts.ToolFilter {
	return []contracts.ToolFilter{
		decisionplanning.AsToolFilter(),
	}
}
