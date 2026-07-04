package workmodel

import (
	"strings"
	"testing"
)

func TestBuildSemanticID(t *testing.T) {
	tests := []struct {
		depth, sibling int
		kind           WorkKind
		want           string
	}{
		{0, 0, WorkKindGoal, "wi_d0_s0_goal"},
		{1, 2, WorkKindImplement, "wi_d1_s2_impl"},
		{3, 0, WorkKindVerify, "wi_d3_s0_verify"},
		{-1, -1, WorkKindExplore, "wi_d0_s0_explore"},
	}
	for _, tc := range tests {
		got := BuildSemanticID(tc.depth, tc.sibling, tc.kind)
		if got != tc.want {
			t.Errorf("BuildSemanticID(%d,%d,%q) = %q, want %q", tc.depth, tc.sibling, tc.kind, got, tc.want)
		}
	}
}

func TestFormatMUPSRoundDisplay(t *testing.T) {
	if got := FormatMUPSRoundDisplay(2, MUPSTriggerInline); got != "mups-r2(inline)" {
		t.Fatalf("FormatMUPSRoundDisplay = %q", got)
	}
	if got := FormatMUPSRoundDisplay(0, ""); got != "mups-r1" {
		t.Fatalf("FormatMUPSRoundDisplay zero round = %q", got)
	}
}

func TestBuildLocator_fullPath(t *testing.T) {
	loc := BuildLocator(LocatorFrame{
		SessionID:  "sess_x",
		TurnNo:     1,
		LoopTick:   3,
		SemanticID: "wi_d0_s0_goal",
		RoundNo:    2,
		Trigger:    MUPSTriggerInline,
		Phase:      "execute",
		Iter:       2,
	})
	want := "sess_x/turn-1/wi_d0_s0_goal/loop-3/mups-r2+inline/execute/iter-2"
	if loc != want {
		t.Fatalf("BuildLocator = %q, want %q", loc, want)
	}
}

func TestInferMUPSTrigger(t *testing.T) {
	t.Run("initial", func(t *testing.T) {
		if got := InferMUPSTrigger(&WorkItem{}, false); got != MUPSTriggerInitial {
			t.Fatalf("got %q", got)
		}
	})
	t.Run("rollup", func(t *testing.T) {
		if got := InferMUPSTrigger(&WorkItem{LastRound: &WorkItemPipelineRound{}}, true); got != MUPSTriggerRollup {
			t.Fatalf("got %q", got)
		}
	})
	t.Run("inline", func(t *testing.T) {
		item := &WorkItem{LastRound: &WorkItemPipelineRound{SpawnPolicy: SpawnInline}}
		if got := InferMUPSTrigger(item, false); got != MUPSTriggerInline {
			t.Fatalf("got %q", got)
		}
	})
	t.Run("refocus", func(t *testing.T) {
		schema := FirstRegisteredDeliverableSchema()
		item := &WorkItem{LastRound: &WorkItemPipelineRound{
			SpawnPolicy:       SpawnNone,
			DeliverableSchema: schema,
			DeliverableStatus: DeliverableStatusIncomplete,
		}}
		if got := InferMUPSTrigger(item, false); got != MUPSTriggerRefocus {
			t.Fatalf("got %q", got)
		}
	})
}

func TestKindSemanticSuffix_unknownKind(t *testing.T) {
	got := KindSemanticSuffix(WorkKind("CUSTOM"))
	if !strings.Contains(got, "custom") {
		t.Fatalf("KindSemanticSuffix = %q", got)
	}
}
