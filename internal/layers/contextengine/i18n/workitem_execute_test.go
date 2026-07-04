package i18n

import (
	"strings"
	"testing"
)

func TestWorkItemExecuteIntro_ZH(t *testing.T) {
	got := WorkItemExecuteIntro(LocaleZH)
	if strings.Contains(got, "You are executing") {
		t.Fatalf("ZH intro must not be English: %q", got)
	}
	if !strings.Contains(got, "WorkItem") {
		t.Fatalf("ZH intro missing WorkItem: %q", got)
	}
}

func TestWorkItemExecuteIntro_EN(t *testing.T) {
	got := WorkItemExecuteIntro(LocaleEN)
	if !strings.Contains(got, "You are executing one WorkItem") {
		t.Fatalf("EN intro: %q", got)
	}
}

func TestWorkItemExecuteOutputHints_ZH_NoObsSelfLabelEnglishOnly(t *testing.T) {
	got := WorkItemExecuteOutputHints(LocaleZH)
	if !strings.Contains(got, "Observe 节点") {
		t.Fatalf("ZH hints missing Observe node note: %q", got)
	}
	if strings.Contains(got, "Do not label observations") {
		t.Fatal("ZH hints must not use English-only Observe note")
	}
}

func TestWorkItemExecuteOutputHints_EN_IncludesScopeContract(t *testing.T) {
	got := WorkItemExecuteOutputHints(LocaleEN)
	if !strings.Contains(got, "<scope_contract>") {
		t.Fatal("EN hints missing scope_contract")
	}
}
