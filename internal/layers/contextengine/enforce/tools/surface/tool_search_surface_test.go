package surface_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce/tools/surface"
	"github.com/devrix/devrix/internal/layers/contextengine/i18n"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// T: TOOL-SURFACE-1-T28 — ToolSearchSurface.Tools() returns exactly one
// spec for `tool_search` with DeferLoading=false (must always be in-pack
// or the LLM can't escape the deferred set).
func TestToolSearchSurface_Tools(t *testing.T) {
	s := newSearchSurface([]contracts.ToolSpec{
		{Name: "delegate_research", DeferLoading: true},
	})
	tools := s.Tools(context.Background(), "", "")
	if len(tools) != 1 {
		t.Fatalf("Tools() = %d entries, want 1", len(tools))
	}
	if tools[0].Name != "tool_search" {
		t.Errorf("Tools[0].Name = %q, want tool_search", tools[0].Name)
	}
	if tools[0].DeferLoading {
		t.Error("tool_search must have DeferLoading=false")
	}
}

// T: TOOL-SURFACE-1-T28 — search() returns up to 5 deferred tools
// matching the query (exact > glob > substring).
func TestToolSearchSurface_Search(t *testing.T) {
	s := newSearchSurface([]contracts.ToolSpec{
		{Name: "delegate_research", DeferLoading: true, Description: "research agent"},
		{Name: "delegate_explore", DeferLoading: true, Description: "explore agent"},
		{Name: "delegate_status", DeferLoading: true, Description: "status check"},
		{Name: "task_output_background", DeferLoading: true, Description: "background task output"},
		{Name: "bash", DeferLoading: false, Description: "should not be searchable"},
	})
	// Exact name match (highest priority).
	res := executeSearch(t, s, `{"query":"delegate_research"}`)
	if len(res) != 1 || res[0].Name != "delegate_research" {
		t.Errorf("exact match: got %v, want [delegate_research]", names(res))
	}
	// Glob match (case-insensitive).
	res = executeSearch(t, s, `{"query":"delegate_*"}`)
	if len(res) < 3 {
		t.Errorf("glob: got %d matches, want >=3 (delegate_*)", len(res))
	}
	// Substring match.
	res = executeSearch(t, s, `{"query":"task"}`)
	if len(res) == 0 || res[0].Name != "task_output_background" {
		t.Errorf("substring: got %v, want task_output_background", names(res))
	}
	// No defer-loading tool is excluded (bash).
	res = executeSearch(t, s, `{"query":"bash"}`)
	if len(res) != 0 {
		t.Errorf("bash (non-defer) should be hidden from search, got %v", names(res))
	}
}

// T: TOOL-SURFACE-1-T28 — category filter narrows results to a name prefix.
func TestToolSearchSurface_Search_Category(t *testing.T) {
	s := newSearchSurface([]contracts.ToolSpec{
		{Name: "delegate_research", DeferLoading: true},
		{Name: "delegate_explore", DeferLoading: true},
		{Name: "task_output_background", DeferLoading: true},
	})
	res := executeSearch(t, s, `{"query":"","category":"delegate"}`)
	for _, r := range res {
		if !strings.HasPrefix(r.Name, "delegate") {
			t.Errorf("category filter leaked: %q", r.Name)
		}
	}
	if len(res) != 2 {
		t.Errorf("got %d, want 2", len(res))
	}
}

// T: TOOL-SURFACE-1-T28 — top-5 cap: more than 5 matches → only first 5 returned.
func TestToolSearchSurface_Search_Top5(t *testing.T) {
	var specs []contracts.ToolSpec
	for i := 0; i < 10; i++ {
		specs = append(specs, contracts.ToolSpec{
			Name:         "delegate_x" + string(rune('0'+i)),
			DeferLoading: true,
		})
	}
	s := newSearchSurface(specs)
	res := executeSearch(t, s, `{"query":"delegate_*"}`)
	if len(res) != 5 {
		t.Errorf("top-5 cap: got %d, want 5", len(res))
	}
}

// T: TOOL-SURFACE-1-T28 — Execute() round-trip: input JSON → matching
// ToolSearchResult JSON.
func TestToolSearchSurface_Execute(t *testing.T) {
	s := newSearchSurface([]contracts.ToolSpec{
		{Name: "delegate_research", DeferLoading: true, Description: "research", Risk: types.RiskLevelHigh},
	})
	out, err := s.Execute(context.Background(), "tool_search", `{"query":"delegate_research"}`, "")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var got []surface.ToolSearchResult
	if err := json.Unmarshal([]byte(out.Output), &got); err != nil {
		t.Fatalf("unmarshal: %v (output=%q)", err, out.Output)
	}
	if len(got) != 1 || got[0].Name != "delegate_research" {
		t.Errorf("got %v, want [delegate_research]", got)
	}
}

// T: TOOL-SURFACE-1-T28 — Execute() with unknown tool name returns Error envelope.
func TestToolSearchSurface_Execute_UnknownName(t *testing.T) {
	s := newSearchSurface(nil)
	out, err := s.Execute(context.Background(), "not_tool_search", `{}`, "")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out.Error == "" {
		t.Error("Error empty, want unknown-tool envelope")
	}
}

// T: TOOL-SURFACE-1-T26 — ShouldDeferByDefault returns true for the 6
// hardcoded candidates (delegate_* + task_output_background) and false
// for everything else (including tool_search, which must NOT be deferred).
func TestShouldDeferByDefault(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"delegate_explore", true},
		{"delegate_status", true},
		{"delegate_status_all", true},
		{"delegate_plan", true},
		{"delegate_research", true},
		{"task_output_background", true},
		{"bash", false},
		{"read_file", false},
		{"grep", false},
		{"tool_search", false}, // forced non-defer (deadlock otherwise)
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := surface.ShouldDeferByDefault(c.name); got != c.want {
				t.Errorf("ShouldDeferByDefault(%q) = %v, want %v", c.name, got, c.want)
			}
		})
	}
}

func newSearchSurface(specs []contracts.ToolSpec) *surface.ToolSearchSurface {
	return surface.NewToolSearchSurface(specs, i18n.LocaleEN)
}

func names(rs []surface.ToolSearchResult) []string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = r.Name
	}
	return out
}

// executeSearch calls Execute on tool_search with the given JSON input
// and returns the decoded result list. Used by search-path tests so we
// exercise the public surface rather than the internal search method.
func executeSearch(t *testing.T, s *surface.ToolSearchSurface, input string) []surface.ToolSearchResult {
	t.Helper()
	out, err := s.Execute(context.Background(), "tool_search", input, "")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out.Error != "" {
		t.Fatalf("Execute returned Error: %s", out.Error)
	}
	var got []surface.ToolSearchResult
	if err := json.Unmarshal([]byte(out.Output), &got); err != nil {
		t.Fatalf("unmarshal: %v (output=%q)", err, out.Output)
	}
	return got
}
