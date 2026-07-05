package sessionorchestrator

import (
	"context"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/i18n"
	"github.com/devrix/devrix/internal/layers/orchestration/wavescheduler"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/prompttags"
	"github.com/devrix/devrix/internal/shared/types"
)

// T: D7-S5-A98-T01 — L5-MUPS-REJ-03 StrategicPlanReject → next Plan user frame.
func TestRunItemPipeline_StrategicPlanRejectFeedsPlanUserFrame(t *testing.T) {
	runner, tm, _ := newItemPipelineTestRunner(t)
	runner.StrategicPlanProposer = rejectingStrategicPlanProposer{}
	runner.Verify = func(_ *wavescheduler.Artifact) workmodel.Verdict {
		return workmodel.Verdict{Kind: types.VerdictPass, SourceID: "v_ok", Confidence: 0.9}
	}

	sessionID := "sess-plan-reject-frame"
	goal, err := tm.EnsureGoal(sessionID, "review architecture")
	if err != nil {
		t.Fatalf("EnsureGoal: %v", err)
	}

	first, err := runner.Run(context.Background(), sessionID, goal, "", ItemPipelineRunOpts{})
	if err != nil {
		t.Fatalf("first Run: %v", err)
	}
	if first.PlanParseReject == "" {
		t.Fatalf("first round PlanParseReject empty")
	}

	goal, _ = tm.GetWorkItem(sessionID, goal.ID)
	rec := prompttags.NewPlanParseReject(prompttags.RejectBudgetCap, "children", "", 5, 2)
	got := buildStrategicPlanUserPrompt(StrategicPlanInput{
		WorkItemID:       goal.ID,
		Directive:        "review architecture",
		PriorParseReject: goal.LastRound.PlanParseReject,
	}, i18n.LocaleZH)
	if !strings.Contains(got, "prior_parse_reject:") {
		t.Fatalf("plan user frame missing prior_parse_reject:\n%s", got)
	}
	if !strings.Contains(got, goal.LastRound.PlanParseReject) {
		t.Fatalf("plan user frame missing reject payload:\n%s", got)
	}
	_ = rec
}

// T: D7-S5-A98-T02 — L5-MUPS-REJ-01 Observe parse fail → next Observe user frame.
func TestObserveWorkItem_ParseRejectFeedsNextObserveFrame(t *testing.T) {
	tm := workmodel.NewTaskManager()
	goal, _ := tm.EnsureGoal("s1", "observe targets")
	goal.LastRound = &workmodel.WorkItemPipelineRound{
		ObserveParseReject: prompttags.NewObserveParseReject(prompttags.RejectParseFail, "bad json", "{").CompactJSON(),
	}
	proposer := StaticObservationProposer{}
	in := buildObserveSignalInput("s1", goal, tm)
	if in.PriorParseReject == "" {
		t.Fatal("PriorParseReject not loaded from LastRound")
	}
	got := buildLLMObservationUserPrompt(in, i18n.LocaleEN)
	if !strings.Contains(got, "prior_parse_reject:") {
		t.Fatalf("observe user frame missing prior_parse_reject:\n%s", got)
	}
	_ = proposer
}
