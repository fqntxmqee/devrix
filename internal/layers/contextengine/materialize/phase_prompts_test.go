package materialize

import (
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/i18n"
	"github.com/devrix/devrix/internal/shared/contracts"
)

// T: D2-S15-A92-T01 — observe/plan zh/en parity.
func TestPhaseAppendix_ZhEnParity(t *testing.T) {
	obsZH := BuildPhaseAppendix(contracts.MUPSPhaseObserve, i18n.LocaleZH, nil, "", "")
	obsEN := BuildPhaseAppendix(contracts.MUPSPhaseObserve, i18n.LocaleEN, nil, "", "")
	if obsZH == obsEN {
		t.Fatal("observe appendix should differ by locale")
	}
	if !strings.Contains(obsZH, "Observe 节点") || !strings.Contains(obsEN, "Observe node") {
		t.Fatalf("zh=%q en=%q", obsZH, obsEN)
	}
	planZH := BuildPhaseAppendix(contracts.MUPSPhasePlan, i18n.LocaleZH, nil, "", `{"citation":["file_line"]}`)
	planEN := BuildPhaseAppendix(contracts.MUPSPhasePlan, i18n.LocaleEN, nil, "", `{"citation":["file_line"]}`)
	if planZH == planEN {
		t.Fatal("plan appendix should differ by locale")
	}
}

// T: D2-S15-A92-T02 — execute output hints contain deliverable_schema.
func TestBuildExecuteOutputHints_DeliverableSchema(t *testing.T) {
	wi := &contracts.MUPSWorkItemSnapshot{
		DeliverableSchema: "findings_json",
	}
	got := BuildExecuteOutputHints(wi)
	if !strings.Contains(got, "<deliverable_schema>findings_json</deliverable_schema>") {
		t.Fatalf("hints = %q", got)
	}
}

func TestAssembleMUPSSystemPrompt_Layers(t *testing.T) {
	got := AssembleMUPSSystemPrompt("base", "hints", "appendix")
	for _, want := range []string{"base", "hints", "appendix"} {
		if !strings.Contains(got, want) {
			t.Fatalf("assembled = %q, missing %q", got, want)
		}
	}
}

func TestBuildPhaseAppendix_RollupSynth(t *testing.T) {
	got := BuildPhaseAppendix(contracts.MUPSPhaseExecute, i18n.LocaleEN, nil, "rollup_synth", "")
	if !strings.Contains(got, "synthesize") {
		t.Fatalf("rollup appendix = %q", got)
	}
}
