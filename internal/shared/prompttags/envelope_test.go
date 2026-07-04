package prompttags

import (
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/shared/contracts"
)

type deliverableContractFixture struct {
	Citation  string   `json:"citation,omitempty"`
	Severity  string   `json:"severity,omitempty"`
	Structure string   `json:"structure,omitempty"`
	MinRunes  int      `json:"min_runes,omitempty"`
	Reject    []string `json:"reject,omitempty"`
}

func TestWrapExtractOne_ScopeContract_Golden(t *testing.T) {
	sc := contracts.MUPSScopeContract{
		GoalStatement: "build auth",
		InScope:       []string{"internal/auth"},
		OutOfScope:    []string{"frontend"},
		OpenQuestions: []string{"OAuth or JWT?"},
	}
	wrapped := Wrap(TagScopeContract, sc)
	wantSubstrings := []string{
		"<scope_contract>",
		`"goal_statement":"build auth"`,
		`"in_scope":["internal/auth"]`,
		`"out_of_scope":["frontend"]`,
		`"open_questions":["OAuth or JWT?"]`,
		"</scope_contract>",
	}
	for _, want := range wantSubstrings {
		if !strings.Contains(wrapped, want) {
			t.Fatalf("wrapped = %q, missing %q", wrapped, want)
		}
	}
	got, ok := ExtractOne[contracts.MUPSScopeContract](TagScopeContract, wrapped)
	if !ok {
		t.Fatal("extract failed")
	}
	if got.GoalStatement != sc.GoalStatement || len(got.InScope) != 1 || got.InScope[0] != "internal/auth" {
		t.Fatalf("got = %+v", got)
	}
	if len(got.OutOfScope) != 1 || got.OutOfScope[0] != "frontend" {
		t.Fatalf("out_of_scope = %+v", got.OutOfScope)
	}
}

func TestWrapExtractOne_DeliverableContract_Golden(t *testing.T) {
	c := deliverableContractFixture{
		Citation: "file_line",
		Severity: "p0_p1",
		Reject:   []string{"planning_meta"},
	}
	wrapped := Wrap(TagDeliverableContract, c)
	got, ok := ExtractOne[deliverableContractFixture](TagDeliverableContract, wrapped)
	if !ok || got.Citation != "file_line" || got.Severity != "p0_p1" {
		t.Fatalf("got = %+v ok=%v wrapped=%q", got, ok, wrapped)
	}
}

func TestWrapExtractOne_DeliverableSchema_Golden(t *testing.T) {
	wrapped := Wrap(TagDeliverableSchema, "p0_p1_file_line")
	want := "<deliverable_schema>p0_p1_file_line</deliverable_schema>"
	if wrapped != want {
		t.Fatalf("wrapped = %q want %q", wrapped, want)
	}
	got, ok := ExtractOne[string](TagDeliverableSchema, wrapped)
	if !ok || got != "p0_p1_file_line" {
		t.Fatalf("got = %q ok=%v", got, ok)
	}
}

func TestWrapExtractOne_PriorVerifyReason_Golden(t *testing.T) {
	wrapped := Wrap(TagPriorVerifyReason, "missing file:line citations")
	got, ok := ExtractOne[string](TagPriorVerifyReason, wrapped)
	if !ok || got != "missing file:line citations" {
		t.Fatalf("got = %q ok=%v", got, ok)
	}
}

func TestWrapExtractOne_OpenQuestions_Lines(t *testing.T) {
	lines := []string{"Which database?", "  ", "Cloud or on-prem?"}
	wrapped := Wrap(TagOpenQuestions, lines)
	got, ok := ExtractOne[[]string](TagOpenQuestions, wrapped)
	if !ok || len(got) != 2 || got[0] != "Which database?" || got[1] != "Cloud or on-prem?" {
		t.Fatalf("got = %+v ok=%v wrapped=%q", got, ok, wrapped)
	}
}

func TestWrap_EmptyScalarReturnsEmpty(t *testing.T) {
	if got := Wrap(TagDeliverableSchema, ""); got != "" {
		t.Fatalf("empty schema wrap = %q", got)
	}
}

func TestExtractAll_FindsMultipleTags(t *testing.T) {
	content := Wrap(TagDeliverableSchema, "findings_json") + "\n" + Wrap(TagPriorVerifyReason, "retry")
	all := ExtractAll(content, "execute")
	if all[TagDeliverableSchema] != "findings_json" {
		t.Fatalf("schema = %q", all[TagDeliverableSchema])
	}
	if all[TagPriorVerifyReason] != "retry" {
		t.Fatalf("reason = %q", all[TagPriorVerifyReason])
	}
}

func TestExtractAll_PhaseFilter_ExecuteOnly(t *testing.T) {
	content := Wrap(TagDeliverableSchema, "findings_json") + "\n" + Wrap(TagPriorVerifyReason, "retry")
	all := ExtractAll(content, "observe")
	if len(all) != 0 {
		t.Fatalf("observe phase should extract no envelope tags, got %v", all)
	}
	all = ExtractAll(content, "")
	if len(all) != 2 {
		t.Fatalf("empty phase should extract all tags, got %v", all)
	}
}

func TestExtractAll_PhaseFilter_VerifyPriorReason(t *testing.T) {
	content := Wrap(TagPriorVerifyReason, "missing citations") + "\n" + Wrap(TagDeliverableSchema, "findings_json")
	all := ExtractAll(content, "verify")
	if all[TagPriorVerifyReason] != "missing citations" {
		t.Fatalf("verify prior reason = %q", all[TagPriorVerifyReason])
	}
	if _, ok := all[TagDeliverableSchema]; ok {
		t.Fatal("deliverable_schema should not apply to verify phase")
	}
}

func TestExtractOne_MultilineEnvelope(t *testing.T) {
	content := "done\n<scope_contract>\n{\"goal_statement\":\"g\"}\n</scope_contract>"
	got, ok := ExtractOne[contracts.MUPSScopeContract](TagScopeContract, content)
	if !ok || got.GoalStatement != "g" {
		t.Fatalf("got = %+v ok=%v", got, ok)
	}
}
