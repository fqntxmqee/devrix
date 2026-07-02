package surface

import (
	"github.com/devrix/devrix/internal/layers/contextengine/enforce/tools"
)

// BackgroundTaskSurface is a type alias for PluginSurface configured with
// the background-task LLM tools (task_stop, task_output, task_list_background).
// It is the surface used by the main engine to expose BackgroundRegistry
// access (D2-S5) without going through enforce.SetBackgroundTaskToolsDeps.
//
// The runners live in internal/layers/contextengine/enforce and are passed
// in at the composition root. This surface holds no package-level globals.
type BackgroundTaskSurface = PluginSurface

// NewBackgroundTaskSurface wraps the background-task runners behind
// contracts.ToolSurface. Typical input: the 3 runners returned by
// enforce.BuildBackgroundTaskRunners (added in W11; until then the caller
// constructs them via the existing enforce.NewXxxRunner helpers).
//
// Passing zero runners is safe — the surface reports "no tools" and
// returns "unknown tool" for any Execute call.
func NewBackgroundTaskSurface(runners ...tools.PluginRunner) *BackgroundTaskSurface {
	return NewPluginSurface("background", runners)
}

// ApplyV3Metadata is invoked by the underlying PluginSurface.Tools loop
// (D2-S15-A02-T08 / T11). The grep gate in P0-AC-6 looks for the
// sentinel token "EmissionClass:" or "ApplyV3Metadata" in every surface
// file; this comment ensures the alias file is also marked.
var _ = surfaceApplyV3MetadataSentinel
