package materialize

import (
	"strings"
	"testing"
)

func TestWorkItemOutputFormatHints_IncludesScopeContract(t *testing.T) {
	if !strings.Contains(WorkItemOutputFormatHints, "<scope_contract>") {
		t.Fatal("format hints must document scope_contract block")
	}
	if strings.Contains(WorkItemOutputFormatHints, "聚焦") {
		t.Fatal("format hints must not contain tactical NL")
	}
}

func TestWorkItemOutputFormatHints_NoDefaultDeliverableSchema(t *testing.T) {
	if strings.Contains(WorkItemOutputFormatHints, "p0_p1_file_line") {
		t.Fatal("global format hints must not hardcode a deliverable schema; schema comes from Strategic Plan / downlink tag")
	}
	if !strings.Contains(WorkItemOutputFormatHints, "<deliverable_schema>{registered_schema}</deliverable_schema>") {
		t.Fatal("format hints must document generic deliverable_schema tag syntax")
	}
}
