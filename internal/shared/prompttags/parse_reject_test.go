package prompttags

import (
	"strings"
	"testing"
)

func TestParseRejectRecord_CompactJSONRoundtrip(t *testing.T) {
	rec := NewPlanParseReject(RejectBudgetCap, "child_specs", "too many children", 5, 2)
	line := rec.CompactJSON()
	if !strings.Contains(line, `"phase":"plan"`) {
		t.Fatalf("line = %q", line)
	}
	got, ok := ParseRejectRecordFromJSON(line)
	if !ok {
		t.Fatal("parse failed")
	}
	if got.Field != "child_specs" || got.Requested != 5 || got.MaxAllowed != 2 {
		t.Fatalf("got = %+v", got)
	}
}

func TestNewObserveParseReject_TruncatesSnippet(t *testing.T) {
	long := strings.Repeat("x", 300)
	rec := NewObserveParseReject(RejectParseFail, "bad json", long)
	if len([]rune(rec.Snippet)) > 121 {
		t.Fatalf("snippet not truncated: len=%d", len(rec.Snippet))
	}
}
