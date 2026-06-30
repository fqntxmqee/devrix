package workmodel

import (
	"testing"
)

func TestParseScopeContractBlock_JSON(t *testing.T) {
	content := `analysis done
<scope_contract>
{"goal_statement":"build auth","in_scope":["internal/auth"],"open_questions":["OAuth or JWT?"]}
</scope_contract>`
	sc, ok := ParseScopeContractBlock(content)
	if !ok || sc == nil {
		t.Fatal("expected parse ok")
	}
	if !sc.HasOpenQuestions() {
		t.Fatal("expected open questions")
	}
	if sc.GoalStatement != "build auth" {
		t.Fatalf("goal = %q", sc.GoalStatement)
	}
}

func TestResolveGoalScopeContract_RuleInferred(t *testing.T) {
	item := &WorkItem{Kind: WorkKindGoal}
	sc := ResolveGoalScopeContract(item, "fix internal/pkg/handler.go bug", "")
	if sc == nil || len(sc.InScope) == 0 {
		t.Fatalf("expected inferred scope, got %+v", sc)
	}
	if sc.HasOpenQuestions() {
		t.Fatal("specific file should not open questions")
	}
}

func TestResolveGoalScopeContract_OpenQuestionsBlock(t *testing.T) {
	item := &WorkItem{Kind: WorkKindGoal}
	content := "still thinking\n<open_questions>\nWhich database?\n</open_questions>"
	sc := ResolveGoalScopeContract(item, "migrate data", content)
	if sc == nil || !sc.HasOpenQuestions() {
		t.Fatalf("expected open questions, got %+v", sc)
	}
}

func TestSetScopeContract_Persists(t *testing.T) {
	tm := NewTaskManager()
	goal, _ := tm.EnsureGoal("s1", "g")
	sc := &ScopeContract{GoalStatement: "g", InScope: []string{"a.go"}}
	if err := tm.SetScopeContract("s1", goal.ID, sc); err != nil {
		t.Fatalf("SetScopeContract: %v", err)
	}
	got, _ := tm.GetWorkItem("s1", goal.ID)
	if got.ScopeContract == nil || got.ScopeContract.GoalStatement != "g" {
		t.Fatalf("scope not persisted: %+v", got.ScopeContract)
	}
}
