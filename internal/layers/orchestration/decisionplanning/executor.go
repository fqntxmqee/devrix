package decisionplanning

import (
	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
	"fmt"

	"github.com/devrix/devrix/internal/layers/orchestration/wavescheduler"
)

// ExecutorType represents the target execution domain.
type ExecutorType string

const (
	// ExecutorD2 routes to contextengine (PlanAgent for explore/plan tasks).
	ExecutorD2 ExecutorType = "d2"
	// ExecutorD4 routes to multi-agent (worker pool for execute/background tasks).
	ExecutorD4 ExecutorType = "d4"
)

// ExecutorSelector implements D7-S5-A03 SelectExecutor.
// It routes TaskNodes to either D2 (PlanAgent) or D4 (multi-agent workers)
// based on task type and worker configuration.
//
// Deprecated: WaveScheduler dispatches directly through the WorkerRunner
// interface; ExecutorSelector is not used in the current dispatch path.
// Code and tests are kept for future reference.
type ExecutorSelector struct{}

// NewExecutorSelector creates a new ExecutorSelector.
func NewExecutorSelector() *ExecutorSelector {
	return &ExecutorSelector{}
}

// SelectResult contains the result of executor selection.
type SelectResult struct {
	Executor   ExecutorType
	Reason     string
	WorkerType wavescheduler.WorkerType
}

// SelectExecutor implements D7-S5-A03. It determines which executor
// should handle a given TaskNode based on task type and configuration.
func (s *ExecutorSelector) SelectExecutor(node wavescheduler.TaskNode) (*SelectResult, error) {
	if node.ID == "" {
		return nil, fmt.Errorf("SelectExecutor: node ID is required")
	}

	result := &SelectResult{
		WorkerType: node.WorkerType,
	}

	// Determine executor based on WorkerType and task metadata
	switch node.WorkerType {
	case wavescheduler.WorkerCursor, wavescheduler.WorkerClaudeCode:
		// Cursor and Claude Code workers route to D4 (main execution)
		result.Executor = ExecutorD4
		result.Reason = fmt.Sprintf("worker_type=%s routes to D4", node.WorkerType)
	case wavescheduler.WorkerSubAgent:
		// Sub-agents route to D4 (delegated multi-agent execution)
		result.Executor = ExecutorD4
		result.Reason = "subagent workers route to D4"
	default:
		// Unknown worker type - route to D4 as fallback
		result.Executor = ExecutorD4
		result.Reason = fmt.Sprintf("unknown worker_type=%s, fallback to D4", node.WorkerType)
	}

	// Check task type override in metadata
	if taskType, ok := node.Metadata["task_type"].(string); ok {
		switch orchtypes.TaskType(taskType) {
		case orchtypes.TaskTypeExplore, orchtypes.TaskTypePlan:
			// Explore and plan tasks route to D2 (PlanAgent)
			result.Executor = ExecutorD2
			result.Reason = fmt.Sprintf("task_type=%s routes to D2 (PlanAgent)", taskType)
		case orchtypes.TaskTypeExecute, orchtypes.TaskTypeBackground:
			// Execute and background tasks route to D4
			result.Executor = ExecutorD4
			result.Reason = fmt.Sprintf("task_type=%s routes to D4", taskType)
		}
	}

	// Read-only tasks always route to D2
	if node.ReadOnly {
		result.Executor = ExecutorD2
		result.Reason = "read_only=true routes to D2 (PlanAgent)"
	}

	return result, nil
}

// MatchExecutorByTaskType implements D7-S5-A03-F01.
func (s *ExecutorSelector) MatchExecutorByTaskType(taskType orchtypes.TaskType) ExecutorType {
	switch taskType {
	case orchtypes.TaskTypeExplore, orchtypes.TaskTypePlan:
		return ExecutorD2
	case orchtypes.TaskTypeExecute, orchtypes.TaskTypeBackground:
		return ExecutorD4
	default:
		return ExecutorD4 // Default fallback
	}
}

// CheckExecutorAvailability implements D7-S5-A03-F02.
// v1.1 returns availability based on hardcoded pool size.
// Future versions will query actual pool capacity.
func (s *ExecutorSelector) CheckExecutorAvailability(executor ExecutorType) (available bool, reason string) {
	switch executor {
	case ExecutorD2:
		// D2 PlanAgent is available if not at capacity
		return true, "D2 PlanAgent available"
	case ExecutorD4:
		// D4 worker pool availability check
		// v1.1: assume available, real impl would check WaveScheduler slots
		return true, "D4 worker pool available"
	default:
		return false, fmt.Sprintf("unknown executor type: %s", executor)
	}
}
