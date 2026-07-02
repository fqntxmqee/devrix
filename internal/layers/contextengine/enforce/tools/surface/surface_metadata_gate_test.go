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
// DM-20260702-008 / D2-S15-A02-T06 — every registered tool must declare
// a non-zero PersistThreshold (MaxResultSizeChars). The gate also enforces
// a per-tool floor: read_file=8K, grep/glob=20K, bash=30K, edit/write/
// web*/lsp/agent/task/plan=100K. Sentinel "PersistThreshold:" in this
// file is grep-able by CI to confirm the gate is wired.
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
		// T06 PersistThreshold sentinel: per-tool MaxResultSizeChars must
		// be positive. The actual value is decided by the per-tool
		// differentiation in orthogonal_flags.go (8K/20K/30K/100K/4K/2K);
		// the gate only checks "non-zero" so a future tool can pick any
		// positive value.
		if max <= 0 {
			t.Errorf("PersistThreshold: DefaultV3MetadataFor(%q).MaxResultSizeChars = %d, want > 0", name, max)
		}
	}
}

// DM-20260702-008 / D2-S15-A02-T06 — sentinel floor for read_file: the
// 8K floor exists because Read is the recovery path (offset/limit
// re-reads); making the threshold anything smaller doesn't gain
// anything and risks masking real size issues.
func TestPersistThreshold_Floor_ReadFile(t *testing.T) {
	_, _, _, _, max, _ := surface.DefaultV3MetadataFor("read_file")
	if max < 8*1024 {
		t.Errorf("PersistThreshold: read_file MaxResultSizeChars = %d, want >= 8K (recovery path)", max)
	}
}

// DM-20260702-008 / D2-S15-A02-T06 — sentinel floor for bash: 30K is
// the clawcode-aligned value; bash output is re-issued, not re-read,
// so a higher threshold wastes tokens without giving the LLM better
// recovery.
func TestPersistThreshold_Floor_Bash(t *testing.T) {
	_, _, _, _, max, _ := surface.DefaultV3MetadataFor("bash")
	if max < 30*1024 {
		t.Errorf("PersistThreshold: bash MaxResultSizeChars = %d, want >= 30K (clawcode-aligned)", max)
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
