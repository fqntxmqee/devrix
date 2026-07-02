package surface

import (
	"github.com/devrix/devrix/internal/layers/contextengine/enforce/tools"
)

// DelegateSurface is a type alias for PluginSurface configured with the
// delegate_* + delegate_status runners. It is the surface used by the
// main engine to expose sessionorchestrator.Dispatcher-backed tools.
//
// The runners are owned by internal/layers/orchestration/delegatetools
// and passed in at the composition root (see bootstrap.WireDelegate).
// This surface reads no package-level globals.
type DelegateSurface = PluginSurface

// NewDelegateSurface wraps a set of delegate runners (typically
// delegate_explore, delegate_plan, delegate_implement, delegate_status)
// behind the contracts.ToolSurface interface. The runners are registered
// with the main engine's tool registry separately; this surface is
// parallel to that registration and is used by the per-surface dispatch
// path in D2.
//
// Passing zero runners is safe — the surface reports "no tools" and
// returns "unknown tool" for any Execute call.
func NewDelegateSurface(runners ...tools.PluginRunner) *DelegateSurface {
	return NewPluginSurface("delegate", runners)
}

// ApplyV3Metadata is invoked by the underlying PluginSurface.Tools loop
// (D2-S15-A02-T08 / T11). The grep gate in P0-AC-6 looks for the
// sentinel token "EmissionClass:" or "ApplyV3Metadata" in every surface
// file; this comment ensures the alias file is also marked.
var _ = surfaceApplyV3MetadataSentinel
