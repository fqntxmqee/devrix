package surface_test

// T: D2-S15-A02-T14 — No-silent-default gate.
//
// For every registered surface, Tools() MUST return specs whose 6 v3
// control plane fields are set explicitly via ApplyV3Metadata. A spec
// that returns with the zero defaults means the surface forgot to call
// ApplyV3Metadata — that tool will be mis-routed in Phase B / Phase D,
// defeating the 治本 narrative.

import (
	"context"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce/tools/surface"
	"github.com/devrix/devrix/internal/shared/contracts"
)

type gateCase struct {
	name        string
	surf        func() contracts.ToolSurface
	mustContain []string
}

func gateCases() []gateCase {
	freefork := surface.NewFreeForkSurface(nil)
	verify := surface.NewVerifySurface()
	tracker := surface.NewTrackerSurface(nil)
	askuser := surface.NewAskUserQuestionSurface()
	// ToolSearchSurface needs no specs and no locale for the gate (the
	// spec it returns is hard-coded inside Tools()).
	toolsearch := surface.NewToolSearchSurface(nil, "")
	return []gateCase{
		{"FreeForkSurface", func() contracts.ToolSurface { return freefork }, []string{"free_fork"}},
		{"VerifySurface", func() contracts.ToolSurface { return verify }, []string{"verify_plan_execution"}},
		{"TrackerSurface", func() contracts.ToolSurface { return tracker }, []string{"query_diagnostics"}},
		{"AskUserQuestionSurface", func() contracts.ToolSurface { return askuser }, []string{"ask_user_question"}},
		{"ToolSearchSurface", func() contracts.ToolSurface { return toolsearch }, []string{"tool_search"}},
	}
}

// T: D2-S15-A02-T14 — every surface Tools() must include the expected
// tool name AND every spec must have non-zero v3 metadata.
func TestAllSurfacesHaveExplicitV3Metadata(t *testing.T) {
	for _, gc := range gateCases() {
		t.Run(gc.name, func(t *testing.T) {
			s := gc.surf()
			specs := s.Tools(context.Background(), "/tmp", "test_session")
			if len(specs) == 0 {
				t.Fatalf("%s: Tools() returned 0 specs", gc.name)
			}
			gotNames := make(map[string]bool, len(specs))
			for _, sp := range specs {
				gotNames[sp.Name] = true
				assertV3MetadataSet(t, gc.name, sp)
			}
			for _, want := range gc.mustContain {
				if !gotNames[want] {
					t.Errorf("%s: Tools() missing required tool %q (got %v)", gc.name, want, toolNames(gotNames))
				}
			}
		})
	}
}

// T: D2-S15-A02-T14 — ApplyV3Metadata leaves the zero defaults only for
// unknown tool names. Every registered tool name MUST return non-zero.
func TestNoToolNameReturnsZeroMetadata(t *testing.T) {
	for _, name := range []string{
		"read_file", "write_file", "edit_file", "bash", "grep", "glob",
		"lsp_go_to_definition", "lsp_workspace_symbol",
		"query_diagnostics", "verify_plan_execution", "ask_user_question",
		"tool_search", "free_fork",
		"delegate_explore", "task_output",
	} {
		ec, cc, ib, su, max, marker := surface.DefaultV3MetadataFor(name)
		if ec == contracts.EC_Action && cc.Kind == contracts.CC_None && ib.Kind == contracts.IB_OpenEnded &&
			su.Source == contracts.SK_Deterministic && su.Value == 0 && max == 0 && marker == "" {
			t.Errorf("DefaultV3MetadataFor(%q) returned zero defaults — should be explicit", name)
		}
	}
}

func assertV3MetadataSet(t *testing.T, surfName string, sp contracts.ToolSpec) {
	t.Helper()
	zero := sp.EmissionClass == contracts.EC_Action &&
		sp.ConvergenceContract.Kind == contracts.CC_None &&
		sp.IterationBound.Kind == contracts.IB_OpenEnded &&
		sp.SourceUncertainty.Source == contracts.SK_Deterministic &&
		sp.SourceUncertainty.Value == 0 &&
		sp.MaxResultSizeChars == 0 &&
		sp.TruncateMarkerText == ""
	if zero {
		t.Errorf("%s: spec %q has zero v3 metadata — ApplyV3Metadata was not called",
			surfName, sp.Name)
	}
	if !strings.Contains(sp.TruncateMarkerText, "complete=false") {
		t.Errorf("%s: spec %q TruncateMarkerText missing 'complete=false' marker: %q",
			surfName, sp.Name, sp.TruncateMarkerText)
	}
}

func toolNames(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
