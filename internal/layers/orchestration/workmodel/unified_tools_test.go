package workmodel

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce/tools"
	"github.com/devrix/devrix/internal/layers/orchestration/runregistry"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

func TestTaskWrite_ChecklistForwardsToTodoWrite(t *testing.T) {
	reg := tools.NewToolRegistry()
	_ = reg.Register(tools.NewTodoWriteRunner())
	SetUnifiedToolRegistry(reg)
	tm := NewTaskManager()
	_ = RegisterUnifiedTaskTools(reg, &config.ContextEngineConfig{Tasks: config.TasksConfig{Mode: "v2"}}, tm)

	ctx := tools.WithToolSessionContext(context.Background(), &types.SessionContext{SessionID: "s1"})
	runner := &taskWriteRunner{manager: tm}
	res, err := runner.Execute(ctx, "", `{"mode":"checklist","todos":[{"content":"a","status":"pending","activeForm":"a"}]}`)
	if err != nil {
		t.Fatal(err)
	}
	if res.Error != "" {
		t.Fatalf("unexpected error: %s", res.Error)
	}
}

func TestTaskSpawn_ForwardsDelegateExplore(t *testing.T) {
	reg := tools.NewToolRegistry()
	reg.Register(&stubForwardRunner{name: "delegate_explore", out: "ok"})
	SetUnifiedToolRegistry(reg)
	tm := NewTaskManager()
	runner := &taskSpawnRunner{manager: tm}
	res, err := runner.Execute(context.Background(), "", `{"kind":"explore","directive":"scan auth"}`)
	if err != nil {
		t.Fatal(err)
	}
	if res.Output == "" || res.Error != "" {
		t.Fatalf("res=%+v", res)
	}
}

func TestTaskAwait_RunRefTerminal(t *testing.T) {
	reg := runregistry.NewRegistry("")
	runregistry.SetGlobal(reg)
	tm := NewTaskManager()
	goal, _ := tm.EnsureGoal("s1", "g")
	item, _ := tm.CreateWorkItem("s1", CreateWorkItemInput{ParentID: goal.ID, Kind: WorkKindExplore, Title: "x", Directive: "x"})
	runID, _ := SpawnForWorkItem("s1", item.ID, "explore", tm)
	reg.SetTerminal(runID, runregistry.StatusCompleted, "done", "")

	runner := &taskAwaitRunner{manager: tm}
	ctx := tools.WithToolSessionID(context.Background(), "s1")
	res, err := runner.Execute(ctx, "", `{"task_id":"`+item.ID+`","block":false}`)
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(res.Output), &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed["status"] != runregistry.StatusCompleted {
		t.Fatalf("status=%v", parsed["status"])
	}
}

type stubForwardRunner struct {
	name string
	out  string
}

func (s *stubForwardRunner) Name() string { return s.name }
func (s *stubForwardRunner) RiskLevel() types.RiskLevel { return types.RiskLevelLow }
func (s *stubForwardRunner) Schema() tools.ToolSchema {
	return tools.ToolSchema{Name: s.name}
}
func (s *stubForwardRunner) Execute(_ context.Context, _, _ string) (*tools.ToolResult, error) {
	return &tools.ToolResult{Output: s.out}, nil
}
