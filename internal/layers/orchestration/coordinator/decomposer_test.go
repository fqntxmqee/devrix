package coordinator

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/wave"
)

func TestTaskDecomposer_SynthesizeTaskGraph(t *testing.T) {
	d := NewTaskDecomposer()

	tests := []struct {
		name      string
		sessionID string
		goal      string
		wantNodes int
		wantValid bool
	}{
		{
			name:      "single goal",
			sessionID: "sess_1",
			goal:      "Add user authentication",
			wantNodes: 1,
			wantValid: true,
		},
		{
			name:      "multiple goals with arrow separator",
			sessionID: "sess_2",
			goal:      "Design API → Implement backend → Write tests",
			wantNodes: 3,
			wantValid: true,
		},
		{
			name:      "empty goal",
			sessionID: "sess_3",
			goal:      "",
			wantNodes: 0,
			wantValid: false,
		},
		{
			name:      "goals with pipe separator",
			sessionID: "sess_4",
			goal:      "Refactor service | Update tests | Deploy",
			wantNodes: 3,
			wantValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := d.SynthesizeTaskGraph(context.Background(), tt.sessionID, tt.goal)
			if tt.wantValid {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if result == nil {
					t.Fatal("result is nil")
				}
				if len(result.Nodes) != tt.wantNodes {
					t.Errorf("got %d nodes, want %d", len(result.Nodes), tt.wantNodes)
				}
				if result.Validation == nil {
					t.Fatal("validation report is nil")
				}
				if result.Validation.Valid != tt.wantValid {
					t.Errorf("validation.Valid = %v, want %v", result.Validation.Valid, tt.wantValid)
				}
			} else {
				// For invalid cases, expect error or empty result
				if err != nil || (result != nil && len(result.Nodes) == 0) {
					return
				}
				t.Errorf("expected error or empty result for invalid case")
			}
		})
	}
}

func TestTaskDecomposer_validateGraph(t *testing.T) {
	d := NewTaskDecomposer()

	// Test with valid graph
	validNodes := []wave.TaskNode{
		{ID: "t1", Directive: "task 1", WorkerType: wave.WorkerCursor, ContextPolicy: wave.ContextFresh},
		{ID: "t2", Directive: "task 2", WorkerType: wave.WorkerCursor, ContextPolicy: wave.ContextFresh, DependsOn: []string{"t1"}},
	}
	report := d.validateGraph(validNodes)
	if !report.Valid {
		t.Errorf("expected valid graph, got errors: %v", report.Errors)
	}

	// Test with cycle
	cycleNodes := []wave.TaskNode{
		{ID: "t1", Directive: "task 1", WorkerType: wave.WorkerCursor, ContextPolicy: wave.ContextFresh, DependsOn: []string{"t2"}},
		{ID: "t2", Directive: "task 2", WorkerType: wave.WorkerCursor, ContextPolicy: wave.ContextFresh, DependsOn: []string{"t1"}},
	}
	report = d.validateGraph(cycleNodes)
	if report.Valid {
		t.Error("expected invalid graph due to cycle, got valid")
	}
	if len(report.Errors) == 0 {
		t.Error("expected cycle error, got none")
	}

	// Test with duplicate IDs
	dupNodes := []wave.TaskNode{
		{ID: "t1", Directive: "task 1", WorkerType: wave.WorkerCursor, ContextPolicy: wave.ContextFresh},
		{ID: "t1", Directive: "task 2", WorkerType: wave.WorkerCursor, ContextPolicy: wave.ContextFresh},
	}
	report = d.validateGraph(dupNodes)
	if report.Valid {
		t.Error("expected invalid graph due to duplicate IDs, got valid")
	}
}

func TestTaskDecomposer_decomposeGoal(t *testing.T) {
	d := NewTaskDecomposer()

	tests := []struct {
		goal string
		want int
	}{
		{"simple task", 1},
		{"step 1 → step 2 → step 3", 3},
		{"task a && task b", 2},
		{"a | b | c", 3},
	}

	for _, tt := range tests {
		t.Run(tt.goal, func(t *testing.T) {
			got := d.decomposeGoal(tt.goal)
			if len(got) != tt.want {
				t.Errorf("decomposeGoal(%q) = %v, want %d", tt.goal, got, tt.want)
			}
		})
	}
}
