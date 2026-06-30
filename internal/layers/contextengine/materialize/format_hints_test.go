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
