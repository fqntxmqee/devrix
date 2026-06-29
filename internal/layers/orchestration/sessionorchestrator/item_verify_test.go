package sessionorchestrator

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/plan"
	"github.com/devrix/devrix/internal/layers/orchestration/wavescheduler"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/types"
)

func TestVerifyArtifact_MaxItersWithToolsIsPartial(t *testing.T) {
	art := &wavescheduler.Artifact{
		TaskID:   "wi_1",
		ExitCode: 1,
		Metadata: map[string]any{
			"stop_reason": "max_iters",
			"tool_calls":  3,
		},
	}
	v := verifyArtifact(art)
	if v.Kind != types.VerdictPartial {
		t.Fatalf("verdict = %v, want partial", v.Kind)
	}
}

func TestVerifyArtifact_MaxItersNoToolsIsFail(t *testing.T) {
	art := &wavescheduler.Artifact{
		TaskID:   "wi_1",
		ExitCode: 1,
		Metadata: map[string]any{
			"stop_reason": "max_iters",
			"tool_calls":  0,
		},
	}
	v := verifyArtifact(art)
	if v.Kind != types.VerdictFail {
		t.Fatalf("verdict = %v, want fail", v.Kind)
	}
}

func TestVerifyArtifactForWorkItem_UserGateIsPartial(t *testing.T) {
	art := &wavescheduler.Artifact{
		TaskID:   "wi_goal",
		Summary:  "I've sent the scope questions. Awaiting your selection.",
		ExitCode: 0,
	}
	item := &workmodel.WorkItem{Kind: workmodel.WorkKindGoal}
	pl := &plan.Plan{Kind: plan.ExplorationPlan}
	v := verifyArtifactForWorkItem(art, item, pl)
	if v.Kind != types.VerdictPartial {
		t.Fatalf("verdict = %v, want partial for user gate", v.Kind)
	}
}

func TestVerifyArtifactForWorkItem_ScopeOnlyIsPartial(t *testing.T) {
	art := &wavescheduler.Artifact{
		TaskID:   "wi_goal",
		Summary:  "done\n<scope_contract>{\"open_questions\":[\"depth?\"]}</scope_contract>",
		ExitCode: 0,
	}
	item := &workmodel.WorkItem{
		Kind: workmodel.WorkKindGoal,
		ScopeContract: &workmodel.ScopeContract{
			OpenQuestions: []string{"depth?"},
		},
	}
	pl := &plan.Plan{Kind: plan.ExplorationPlan}
	v := verifyArtifactForWorkItem(art, item, pl)
	if v.Kind != types.VerdictPartial {
		t.Fatalf("verdict = %v, want partial for scope-only", v.Kind)
	}
}

func TestFilterPipelineTools_RemovesAskUser(t *testing.T) {
	in := []ToolSchema{
		{Name: "read_file"},
		{Name: "ask_user_question"},
		{Name: "grep"},
	}
	out := filterPipelineTools(in)
	if len(out) != 2 {
		t.Fatalf("len = %d, want 2", len(out))
	}
	for _, tool := range out {
		if tool.Name == "ask_user_question" {
			t.Fatal("ask_user_question should be filtered")
		}
	}
}
