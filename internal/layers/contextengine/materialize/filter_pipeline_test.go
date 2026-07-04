package materialize

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce/tools/filter"
	"github.com/devrix/devrix/internal/layers/contextengine/i18n"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// T: D2-S15-A91-T01 — explore agent → Fact+Probe only.
func TestFilterPipeline_ExploreAgent_FactProbeOnly(t *testing.T) {
	deps := testFilterDeps(t)
	in := FilterPipelineInput{
		Phase:        contracts.MUPSPhaseExecute,
		ToolProfile:  "implement",
		AgentProfile: "explore",
		WorkDir:      "/tmp",
	}
	tools := RunFilterPipeline(context.Background(), deps, in)
	allowed := map[string]bool{"read_file": true, "grep": true, "glob": true, "list_dir": true}
	for _, tool := range tools {
		if !allowed[tool.Name] {
			t.Fatalf("explore agent tool %q not in readonly explore set", tool.Name)
		}
	}
	if len(tools) == 0 {
		t.Fatal("expected at least one tool for explore agent")
	}
}

// T: D2-S15-A91-T02 — review task_kind applies bounded hint metadata path.
func TestFilterPipeline_ReviewTaskKind_BoundedHint(t *testing.T) {
	bound := filter.TaskKindBound("review")
	if bound.Kind != contracts.IB_Bounded || bound.MaxN != 15 {
		t.Fatalf("review bound = %+v, want Bounded(15)", bound)
	}
}

// T: D2-S15-A91-T03 — readonly profile removes ask_user_question.
func TestFilterPipeline_ReadonlyProfile_RemovesInteractive(t *testing.T) {
	deps := testFilterDeps(t)
	in := FilterPipelineInput{
		Phase:       contracts.MUPSPhaseExecute,
		ToolProfile: "readonly",
		WorkDir:     "/tmp",
	}
	tools := RunFilterPipeline(context.Background(), deps, in)
	for _, tool := range tools {
		if tool.Name == "ask_user_question" {
			t.Fatal("readonly profile must not include ask_user_question")
		}
		if tool.Name == "bash" || tool.Name == "edit_file" {
			t.Fatalf("readonly profile must not include %q", tool.Name)
		}
	}
}

// T: D2-S15-A91-T04 — permission runs before emission class.
func TestFilterPipeline_OrderInvariant_PermissionBeforeEmission(t *testing.T) {
	deps := testFilterDeps(t)
	in := FilterPipelineInput{
		Phase:       contracts.MUPSPhaseExecute,
		ToolProfile: "implement",
		WorkDir:     "/tmp",
	}
	trace := TraceFilterPipelineSteps(context.Background(), deps, in)
	if len(trace) < 2 {
		t.Fatalf("trace = %+v", trace)
	}
	permIdx, ecIdx := -1, -1
	for i, step := range trace {
		switch step.Name {
		case "permission":
			permIdx = i
		case "emission_class":
			ecIdx = i
		}
	}
	if permIdx < 0 || ecIdx < 0 || permIdx >= ecIdx {
		t.Fatalf("pipeline order wrong: trace=%+v permIdx=%d ecIdx=%d", trace, permIdx, ecIdx)
	}
}

func testFilterDeps(t *testing.T) FilterPipelineDeps {
	t.Helper()
	return FilterPipelineDeps{
		Surfaces: []contracts.ToolSurface{&stubFilterSurface{}},
		Locale:   i18n.LocaleEN,
	}
}

type stubFilterSurface struct{}

func (s *stubFilterSurface) Name() string { return "stub" }

func (s *stubFilterSurface) Tools(_ context.Context, _, _ string) []contracts.ToolSpec {
	return []contracts.ToolSpec{
		{Name: "read_file", Description: "read", EmissionClass: contracts.EC_Probe},
		{Name: "grep", Description: "grep", EmissionClass: contracts.EC_Probe},
		{Name: "edit_file", Description: "edit", EmissionClass: contracts.EC_Action},
		{Name: "bash", Description: "bash", EmissionClass: contracts.EC_Action},
		{Name: "ask_user_question", Description: "ask", EmissionClass: contracts.EC_Action},
	}
}

func (s *stubFilterSurface) IsConcurrencySafe(json.RawMessage) bool { return false }

func (s *stubFilterSurface) ToAutoClassifierInput(json.RawMessage) string { return "" }

func (s *stubFilterSurface) Execute(context.Context, string, string, string) (*contracts.ToolResult, error) {
	return &contracts.ToolResult{}, nil
}

func (s *stubFilterSurface) InterruptBehavior(string) contracts.InterruptMode {
	return contracts.InterruptBlock
}

func (s *stubFilterSurface) CheckPermission(context.Context, contracts.ToolSpec, json.RawMessage) contracts.Decision {
	return contracts.DecisionAllow
}

func (s *stubFilterSurface) RiskLevel(name string) types.RiskLevel {
	if name != "" {
		return types.RiskLevelLow
	}
	return ""
}

func TestProfileFilter_ReadonlyBlocksWriteTools(t *testing.T) {
	specs := []contracts.ToolSpec{
		{Name: "read_file", EmissionClass: contracts.EC_Probe},
		{Name: "edit_file", EmissionClass: contracts.EC_Action},
	}
	got := profileFilter("readonly", specs)
	if len(got) != 1 || got[0].Name != "read_file" {
		t.Fatalf("readonly filter = %v", got)
	}
}

func TestAllowedEmissionClasses_ExecuteExplore(t *testing.T) {
	classes := allowedEmissionClasses(contracts.MUPSPhaseExecute, "explore")
	if len(classes) != 2 {
		t.Fatalf("classes = %v", classes)
	}
}

func TestObservePlanSkipTools(t *testing.T) {
	deps := testFilterDeps(t)
	for _, phase := range []contracts.MUPSPhase{contracts.MUPSPhaseObserve, contracts.MUPSPhasePlan} {
		tools := RunFilterPipeline(context.Background(), deps, FilterPipelineInput{Phase: phase, WorkDir: "/tmp"})
		if len(tools) != 0 {
			t.Fatalf("phase %s tools = %v, want empty", phase, tools)
		}
	}
}

func TestBuildPhaseAppendix_ObserveContainsSchema(t *testing.T) {
	got := BuildPhaseAppendix(contracts.MUPSPhaseObserve, i18n.LocaleEN, nil, "", "")
	if !strings.Contains(got, "obs_fact") {
		t.Fatalf("appendix = %q", got)
	}
}
