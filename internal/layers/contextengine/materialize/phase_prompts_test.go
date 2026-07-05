package materialize

import (
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/shared/contracts"
)

func TestAssembleMUPSExecuteSystemPrompt_TaskBeforeHints(t *testing.T) {
	got := AssembleMUPSExecuteSystemPrompt("BASE", "HINTS", "TASK", "")
	idxTask := strings.Index(got, "TASK")
	idxHints := strings.Index(got, "HINTS")
	idxBase := strings.Index(got, "BASE")
	if idxTask < 0 || idxHints < 0 || idxBase < 0 {
		t.Fatalf("missing segment: %q", got)
	}
	if !(idxTask < idxHints && idxHints < idxBase) {
		t.Fatalf("want TASK → HINTS → BASE, got:\n%s", got)
	}
}

func TestAssembleMUPSPhaseSystemPrompt_ExecuteUsesTaskFirst(t *testing.T) {
	got := AssembleMUPSPhaseSystemPrompt(contracts.MUPSPhaseExecute, "BASE", "HINTS", "TASK", "")
	if strings.Index(got, "TASK") > strings.Index(got, "HINTS") {
		t.Fatalf("execute phase should place TASK before HINTS:\n%s", got)
	}
}

func TestAssembleMUPSPhaseSystemPrompt_ObserveKeepsAppendixBeforeBase(t *testing.T) {
	got := AssembleMUPSPhaseSystemPrompt(contracts.MUPSPhaseObserve, "BASE", "", "", "APPENDIX")
	if strings.Index(got, "APPENDIX") > strings.Index(got, "BASE") {
		t.Fatalf("observe phase should place APPENDIX before BASE:\n%s", got)
	}
}
