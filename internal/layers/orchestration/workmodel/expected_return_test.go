package workmodel

import "testing"

func TestDeliverableSchemaTag(t *testing.T) {
	got := DeliverableSchemaTag(DeliverableSchemaP0P1FileLine)
	want := "<deliverable_schema>p0_p1_file_line</deliverable_schema>"
	if got != want {
		t.Fatalf("tag = %q, want %q", got, want)
	}
}

func TestDefaultChildExpectedReturn_ReviewDirective(t *testing.T) {
	item := &WorkItem{Kind: WorkKindGoal}
	got := DefaultChildExpectedReturn(item, "review internal/kernel")
	if got == "" {
		t.Fatal("expected schema tag for review directive")
	}
}

func TestParseDeliverableSchemaTag(t *testing.T) {
	got := ParseDeliverableSchemaTag("<deliverable_schema>p0_p1_file_line</deliverable_schema>")
	if got != DeliverableSchemaP0P1FileLine {
		t.Fatalf("parsed = %q", got)
	}
}
