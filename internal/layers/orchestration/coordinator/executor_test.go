package coordinator

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/wave"
)

func TestExecutorSelector_SelectExecutor(t *testing.T) {
	sel := NewExecutorSelector()

	tests := []struct {
		name       string
		node       wave.TaskNode
		wantExec   ExecutorType
		wantReason string
	}{
		{
			name: "cursor worker routes to D4",
			node: wave.TaskNode{
				ID:         "t1",
				WorkerType: wave.WorkerCursor,
				Directive:  "do something",
			},
			wantExec:   ExecutorD4,
			wantReason: "worker_type=cursor routes to D4",
		},
		{
			name: "claude_code worker routes to D4",
			node: wave.TaskNode{
				ID:         "t2",
				WorkerType: wave.WorkerClaudeCode,
				Directive:  "do something",
			},
			wantExec:   ExecutorD4,
			wantReason: "worker_type=claude_code routes to D4",
		},
		{
			name: "subagent worker routes to D4",
			node: wave.TaskNode{
				ID:         "t3",
				WorkerType: wave.WorkerSubAgent,
				Directive:  "do something",
			},
			wantExec:   ExecutorD4,
			wantReason: "subagent workers route to D4",
		},
		{
			name: "explore task type routes to D2",
			node: wave.TaskNode{
				ID:         "t4",
				WorkerType: wave.WorkerCursor,
				Directive:  "explore the codebase",
				Metadata:   map[string]any{"task_type": "explore"},
			},
			wantExec:   ExecutorD2,
			wantReason: "task_type=explore routes to D2 (PlanAgent)",
		},
		{
			name: "plan task type routes to D2",
			node: wave.TaskNode{
				ID:         "t5",
				WorkerType: wave.WorkerCursor,
				Directive:  "plan the implementation",
				Metadata:   map[string]any{"task_type": "plan"},
			},
			wantExec:   ExecutorD2,
			wantReason: "task_type=plan routes to D2 (PlanAgent)",
		},
		{
			name: "execute task type routes to D4",
			node: wave.TaskNode{
				ID:         "t6",
				WorkerType: wave.WorkerCursor,
				Directive:  "implement feature",
				Metadata:   map[string]any{"task_type": "execute"},
			},
			wantExec:   ExecutorD4,
			wantReason: "task_type=execute routes to D4",
		},
		{
			name: "background task type routes to D4",
			node: wave.TaskNode{
				ID:         "t7",
				WorkerType: wave.WorkerCursor,
				Directive:  "run background task",
				Metadata:   map[string]any{"task_type": "background"},
			},
			wantExec:   ExecutorD4,
			wantReason: "task_type=background routes to D4",
		},
		{
			name: "read-only task routes to D2",
			node: wave.TaskNode{
				ID:         "t8",
				WorkerType: wave.WorkerCursor,
				Directive:  "read only task",
				ReadOnly:   true,
			},
			wantExec:   ExecutorD2,
			wantReason: "read_only=true routes to D2 (PlanAgent)",
		},
		{
			name: "read-only with D4 metadata still routes to D2",
			node: wave.TaskNode{
				ID:         "t9",
				WorkerType: wave.WorkerCursor,
				Directive:  "read only task",
				ReadOnly:   true,
				Metadata:   map[string]any{"task_type": "execute"},
			},
			wantExec:   ExecutorD2,
			wantReason: "read_only=true routes to D2 (PlanAgent)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := sel.SelectExecutor(tt.node)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Executor != tt.wantExec {
				t.Errorf("Executor = %v, want %v", result.Executor, tt.wantExec)
			}
			if result.Reason != tt.wantReason {
				t.Errorf("Reason = %v, want %v", result.Reason, tt.wantReason)
			}
		})
	}
}

func TestExecutorSelector_SelectExecutor_Error(t *testing.T) {
	sel := NewExecutorSelector()

	_, err := sel.SelectExecutor(wave.TaskNode{})
	if err == nil {
		t.Error("expected error for empty node ID, got nil")
	}
}

func TestExecutorSelector_MatchExecutorByTaskType(t *testing.T) {
	sel := NewExecutorSelector()

	tests := []struct {
		taskType TaskType
		want     ExecutorType
	}{
		{TaskTypeExplore, ExecutorD2},
		{TaskTypePlan, ExecutorD2},
		{TaskTypeExecute, ExecutorD4},
		{TaskTypeBackground, ExecutorD4},
	}

	for _, tt := range tests {
		t.Run(string(tt.taskType), func(t *testing.T) {
			got := sel.MatchExecutorByTaskType(tt.taskType)
			if got != tt.want {
				t.Errorf("MatchExecutorByTaskType(%s) = %v, want %v", tt.taskType, got, tt.want)
			}
		})
	}
}

func TestExecutorSelector_CheckExecutorAvailability(t *testing.T) {
	sel := NewExecutorSelector()

	tests := []struct {
		executor ExecutorType
		wantOK   bool
	}{
		{ExecutorD2, true},
		{ExecutorD4, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.executor), func(t *testing.T) {
			available, _ := sel.CheckExecutorAvailability(tt.executor)
			if available != tt.wantOK {
				t.Errorf("CheckExecutorAvailability(%s) = %v, want %v", tt.executor, available, tt.wantOK)
			}
		})
	}
}
