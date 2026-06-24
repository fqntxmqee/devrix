package sessionorchestrator

import (
	"context"
	"fmt"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/google/uuid"
)

// sessionIDKey is the context key for session ID extraction.
type sessionIDKey struct{}

// WithSessionID attaches sessionID to ctx for WorkModel method calls.
func WithSessionID(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, sessionIDKey{}, sessionID)
}

// SessionIDFromCtx extracts sessionID from ctx. Returns "" if not found.
func SessionIDFromCtx(ctx context.Context) string {
	if v, ok := ctx.Value(sessionIDKey{}).(string); ok {
		return v
	}
	return ""
}

// WorkModel is the D7-S1 unified facade. Storage lives in
// orchestration/workmodel (v2.0 Slice-2 migration from D2 contextengine/tasks).
//
// See R1 Q2 三模型 + 统一查询入口 decision and R2 命题 B.
type WorkModel interface {
	// CreateTask inserts a new task in pending status. Returns the
	// generated task ID. Equivalent to D2 TaskManager.Create.
	CreateTask(ctx context.Context, spec orchtypes.TaskSpec) (string, error)
	// UpdateStatus transitions a task to a new status. v1.0 does not
	// validate transitions; v1.1 introduces a state-machine guard.
	UpdateStatus(ctx context.Context, taskID string, status orchtypes.TaskStatus) error
	// QueryWorkPlan returns a unified snapshot of PlanTask +
	// WaveTaskNode + BackgroundRun for a session. v1.0 reads the D2
	// TaskManager in-memory map and joins with the Wave Scheduler and
	// the Background registry.
	QueryWorkPlan(ctx context.Context, sessionID string) (WorkPlanSnapshot, error)
}

// WorkPlanSnapshot is the unified read-model returned by QueryWorkPlan.
type WorkPlanSnapshot struct {
	SessionID  string
	Tasks      []orchtypes.TaskSpec
	Flows      []FlowLite
	Background []BackgroundLite
}

// FlowLite is a minimal D7-S4 projection; the full ExecutionFlowSnapshot
// lives in internal/shared/contracts.
type FlowLite struct {
	FlowID   string
	WorkerID string
	TaskID   string
	Status   string
}

// BackgroundLite is a minimal D7-S1 BackgroundRun projection. v1.0 sources
// from D2 query/background.go (per R2 保留项 4.3 resolution C — v1.0 does
// not migrate).
type BackgroundLite struct {
	RunID  string
	Status orchtypes.TaskStatus
	Output string
}

// LocalWorkModel is the v1.1 implementation: it uses the coordinator's
// TaskManager directly.
type LocalWorkModel struct {
	tasks   *workmodel.TaskManager
	flowHub interface {
		Snapshot(sessionID string) interface{}
	}
	listBackground func(sessionID string) []BackgroundLite
}

// NewLocalWorkModel returns a WorkModel that uses the coordinator's TaskManager.
func NewLocalWorkModel(tasks *workmodel.TaskManager) *LocalWorkModel {
	return &LocalWorkModel{tasks: tasks}
}

// SetFlowHub wires the flow hub for QueryWorkPlan.
func (m *LocalWorkModel) SetFlowHub(hub interface {
	Snapshot(sessionID string) interface{}
}) {
	m.flowHub = hub
}

// SetBackgroundProvider wires the background task source for QueryWorkPlan.
// The function must return a slice of BackgroundLite for the given session,
// or nil/empty if no background tasks exist.
func (m *LocalWorkModel) SetBackgroundProvider(fn func(sessionID string) []BackgroundLite) {
	m.listBackground = fn
}

// CreateTask creates a task using the local TaskManager.
func (m *LocalWorkModel) CreateTask(ctx context.Context, spec orchtypes.TaskSpec) (string, error) {
	sessionID := SessionIDFromCtx(ctx)
	if sessionID == "" {
		return "", fmt.Errorf("LocalWorkModel: sessionID not found in context")
	}
	task, err := m.tasks.Create(sessionID, spec.Subject, spec.Goal)
	if err != nil {
		return "", fmt.Errorf("LocalWorkModel.CreateTask: %w", err)
	}
	if task == nil {
		return "", fmt.Errorf("LocalWorkModel.CreateTask: nil task without error")
	}
	return task.ID, nil
}

// UpdateStatus updates task status using the local TaskManager.
func (m *LocalWorkModel) UpdateStatus(ctx context.Context, taskID string, status orchtypes.TaskStatus) error {
	sessionID := SessionIDFromCtx(ctx)
	if sessionID == "" {
		return fmt.Errorf("LocalWorkModel: sessionID not found in context")
	}
	return m.tasks.UpdateStatus(sessionID, taskID, status)
}

// QueryWorkPlan returns a snapshot combining local tasks, flow state, and
// background runs.
func (m *LocalWorkModel) QueryWorkPlan(ctx context.Context, sessionID string) (WorkPlanSnapshot, error) {
	snapshot := WorkPlanSnapshot{SessionID: sessionID}

	// Tasks
	tasks := m.tasks.List(sessionID)
	for _, t := range tasks {
		spec := orchtypes.TaskSpec{
			ID:      t.ID,
			Subject: t.Subject,
			Goal:    t.Description,
		}
		snapshot.Tasks = append(snapshot.Tasks, spec)
	}

	// Background runs
	if m.listBackground != nil {
		snapshot.Background = m.listBackground(sessionID)
	}

	// Execution flows
	if m.flowHub != nil {
		if raw := m.flowHub.Snapshot(sessionID); raw != nil {
			if flows, ok := raw.([]FlowLite); ok {
				snapshot.Flows = flows
			}
		}
	}

	return snapshot, nil
}

// CreateWorkPlan implements D7-S1-A01. v1.1 creates a single Task from the goal.
// Future versions will call SynthesizeTaskGraph (D7-S5-A02) to decompose
// the goal into a DAG of tasks.
func (m *LocalWorkModel) CreateWorkPlan(ctx context.Context, sessionID, goal string) (*orchtypes.Plan, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("CreateWorkPlan: sessionID is required")
	}
	if goal == "" {
		return nil, fmt.Errorf("CreateWorkPlan: goal is required")
	}

	plan := &orchtypes.Plan{
		ID:        "plan_" + uuid.New().String()[:8],
		SessionID: sessionID,
		Tasks:     []orchtypes.TaskSpec{},
		CreatedAt: time.Now(),
	}

	// v1.1: single task from goal (SynthesizeTaskGraph deferred to future)
	task, err := m.tasks.Create(sessionID, goal, goal)
	if err != nil {
		return nil, fmt.Errorf("CreateWorkPlan: failed to create task: %w", err)
	}
	if task == nil {
		return nil, fmt.Errorf("CreateWorkPlan: failed to create task (nil task without error)")
	}
	taskID := task.ID

	// Build orchtypes.TaskSpec for the created task
	tasks := m.tasks.List(sessionID)
	for _, t := range tasks {
		if t.ID == taskID {
			plan.Tasks = append(plan.Tasks, orchtypes.TaskSpec{
				ID:      t.ID,
				Subject: t.Subject,
				Goal:    t.Description,
			})
			break
		}
	}

	return plan, nil
}
