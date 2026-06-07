package pev_test

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/pev"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

type stubPlanner struct {
	batch []*types.Milestone
}

func (s *stubPlanner) CreateBatch(_ string, milestones []*types.Milestone) error {
	s.batch = append([]*types.Milestone(nil), milestones...)
	return nil
}
func (s *stubPlanner) GetExecutionOrder(_ string) ([]*types.Milestone, error) { return nil, nil }
func (s *stubPlanner) UpdateProgress(string, float64) error                   { return nil }
func (s *stubPlanner) Complete(string) error                                    { return nil }
func (s *stubPlanner) Fail(string, string) error                                { return nil }

type planLLM struct {
	response string
}

func (p *planLLM) Complete(_ context.Context, _ pev.PlanLLMRequest) (string, error) {
	return p.response, nil
}

func TestValidatePlanDocument_should_accept_valid_dag(t *testing.T) {
	doc := &pev.PlanDocument{
		TaskID: "task_abc",
		Milestones: []pev.PlanMilestoneSpec{
			{ID: "ms_1", Name: "analyze", Description: "read code"},
			{ID: "ms_2", Name: "fix", Description: "apply fix", Dependencies: []string{"ms_1"}},
		},
	}
	milestones, err := pev.ValidatePlanDocument(doc, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(milestones) != 2 {
		t.Fatalf("expected 2 milestones, got %d", len(milestones))
	}
	order, err := pev.TopologicalSort(milestones)
	if err != nil {
		t.Fatalf("topo sort: %v", err)
	}
	if order[0].ID != "ms_1" || order[1].ID != "ms_2" {
		t.Fatalf("unexpected order: %v, %v", order[0].ID, order[1].ID)
	}
}

func TestValidatePlanDocument_should_reject_cycle(t *testing.T) {
	doc := &pev.PlanDocument{
		TaskID: "task_cycle",
		Milestones: []pev.PlanMilestoneSpec{
			{ID: "ms_a", Name: "a", Dependencies: []string{"ms_b"}},
			{ID: "ms_b", Name: "b", Dependencies: []string{"ms_a"}},
		},
	}
	_, err := pev.ValidatePlanDocument(doc, 10)
	if err == nil {
		t.Fatal("expected cycle error")
	}
}

func TestValidatePlanDocument_should_reject_over_max_milestones(t *testing.T) {
	doc := &pev.PlanDocument{
		TaskID: "task_many",
		Milestones: []pev.PlanMilestoneSpec{
			{ID: "ms_1", Name: "one"},
			{ID: "ms_2", Name: "two"},
		},
	}
	_, err := pev.ValidatePlanDocument(doc, 1)
	if err == nil {
		t.Fatal("expected max milestones error")
	}
}

func TestPlanEngine_should_persist_valid_plan(t *testing.T) {
	planner := &stubPlanner{}
	engine := pev.NewPlanEngine(&planLLM{response: `{
		"task_id": "task_ok",
		"milestones": [
			{"id": "ms_1", "name": "step1", "description": "d1", "dependencies": []}
		]
	}`}, planner, config.DefaultPlanConfig())

	result, err := engine.Plan(context.Background(), "implement feature")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Degraded {
		t.Fatalf("expected non-degraded plan, err=%v", result.Err)
	}
	if len(planner.batch) != 1 {
		t.Fatalf("expected 1 persisted milestone, got %d", len(planner.batch))
	}
}

func TestPlanEngine_should_degrade_on_invalid_json(t *testing.T) {
	engine := pev.NewPlanEngine(&planLLM{response: "not json"}, &stubPlanner{}, config.DefaultPlanConfig())
	result, err := engine.Plan(context.Background(), "goal")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Degraded {
		t.Fatal("expected degraded result")
	}
}

var _ contracts.IMilestonePlanner = (*stubPlanner)(nil)
