package milestone

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/devrix/devrix/internal/shared/types"
)

// ITaskFlowService defines the interface for task flow operations
type ITaskFlowService interface {
	// Create creates a new task flow
	Create(name string, dag *types.MilestoneDAG) (*types.TaskFlow, error)
	// Get retrieves a task flow by ID
	Get(id string) (*types.TaskFlow, error)
	// Start starts the task flow execution
	Start(id string) error
	// UpdateMilestoneProgress updates a milestone's progress
	UpdateMilestoneProgress(id string, milestoneID string, progress float64) error
	// CompleteMilestone marks a milestone as completed and advances
	CompleteMilestone(id string, milestoneID string) error
	// FailMilestone marks a milestone as failed
	FailMilestone(id string, milestoneID string, reason string) error
	// GetProgress returns the current progress
	GetProgress(id string) (float64, error)
	// List returns all task flows
	List() []*types.TaskFlow
}

// TaskFlowService implements ITaskFlowService
type TaskFlowService struct {
	mu        sync.RWMutex
	taskFlows map[string]*types.TaskFlow
	ms        *MilestoneService // Milestone service for milestone operations
	ev        TaskFlowEventEmitter
}

// TaskFlowEventEmitter emits task flow events
type TaskFlowEventEmitter interface {
	EmitTaskFlowStarted(tf *types.TaskFlow)
	EmitTaskFlowProgress(tf *types.TaskFlow)
	EmitTaskFlowCompleted(tf *types.TaskFlow)
	EmitTaskFlowFailed(tf *types.TaskFlow, reason string)
}

// DefaultTaskFlowEventEmitter logs task flow events
type DefaultTaskFlowEventEmitter struct{}

func (e *DefaultTaskFlowEventEmitter) EmitTaskFlowStarted(tf *types.TaskFlow) {
	slog.Info("taskflow.started", "id", tf.ID, "name", tf.Name)
}

func (e *DefaultTaskFlowEventEmitter) EmitTaskFlowProgress(tf *types.TaskFlow) {
	slog.Debug("taskflow.progress", "id", tf.ID, "progress", tf.OverallProgress)
}

func (e *DefaultTaskFlowEventEmitter) EmitTaskFlowCompleted(tf *types.TaskFlow) {
	slog.Info("taskflow.completed", "id", tf.ID, "name", tf.Name)
}

func (e *DefaultTaskFlowEventEmitter) EmitTaskFlowFailed(tf *types.TaskFlow, reason string) {
	slog.Error("taskflow.failed", "id", tf.ID, "name", tf.Name, "reason", reason)
}

// NewTaskFlowService creates a new task flow service
func NewTaskFlowService(ms *MilestoneService) *TaskFlowService {
	return &TaskFlowService{
		taskFlows: make(map[string]*types.TaskFlow),
		ms:        ms,
		ev:        &DefaultTaskFlowEventEmitter{},
	}
}

// SetEventEmitter sets the event emitter
func (s *TaskFlowService) SetEventEmitter(ev TaskFlowEventEmitter) {
	s.ev = ev
}

// generateTaskFlowID generates a unique task flow ID
func generateTaskFlowID() string {
	return fmt.Sprintf("tf_%d_%d", time.Now().UnixMilli(), time.Now().UnixNano()%10000)
}

// Create creates a new task flow
func (s *TaskFlowService) Create(name string, dag *types.MilestoneDAG) (*types.TaskFlow, error) {
	if name == "" {
		return nil, fmt.Errorf("task flow name is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	id := generateTaskFlowID()
	tf := types.NewTaskFlow(id, name, dag)

	s.taskFlows[id] = tf

	slog.Info("task flow created",
		"id", id,
		"name", name,
		"milestone_count", len(dag.Milestones),
	)

	return tf, nil
}

// Get retrieves a task flow by ID
func (s *TaskFlowService) Get(id string) (*types.TaskFlow, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tf, exists := s.taskFlows[id]
	if !exists {
		return nil, fmt.Errorf("task flow %s not found", id)
	}

	return tf, nil
}

// Start starts the task flow execution
func (s *TaskFlowService) Start(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tf, exists := s.taskFlows[id]
	if !exists {
		return fmt.Errorf("task flow %s not found", id)
	}

	if err := tf.Start(); err != nil {
		return err
	}

	s.ev.EmitTaskFlowStarted(tf)

	slog.Info("task flow started",
		"id", id,
		"current_milestone", tf.CurrentMilestone,
	)

	return nil
}

// UpdateMilestoneProgress updates a milestone's progress within a task flow
func (s *TaskFlowService) UpdateMilestoneProgress(tfID, milestoneID string, progress float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tf, exists := s.taskFlows[tfID]
	if !exists {
		return fmt.Errorf("task flow %s not found", tfID)
	}

	if tf.CurrentMilestone != milestoneID {
		return fmt.Errorf("milestone %s is not currently executing", milestoneID)
	}

	// Update via milestone service
	if err := s.ms.UpdateProgress(milestoneID, progress); err != nil {
		return err
	}

	// Recalculate overall progress
	tf.UpdateProgress()

	s.ev.EmitTaskFlowProgress(tf)

	return nil
}

// CompleteMilestone marks a milestone as completed and advances to the next
func (s *TaskFlowService) CompleteMilestone(tfID, milestoneID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tf, exists := s.taskFlows[tfID]
	if !exists {
		return fmt.Errorf("task flow %s not found", tfID)
	}

	if tf.CurrentMilestone != milestoneID {
		return fmt.Errorf("milestone %s is not currently executing", milestoneID)
	}

	// Complete current milestone via milestone service
	if err := s.ms.Complete(milestoneID); err != nil {
		return err
	}

	// Advance to next milestone
	if err := tf.AdvanceToNext(); err != nil {
		s.ev.EmitTaskFlowFailed(tf, err.Error())
		return err
	}

	tf.UpdateProgress()

	if tf.Status == types.TaskFlowStatusCompleted {
		s.ev.EmitTaskFlowCompleted(tf)
	} else {
		s.ev.EmitTaskFlowProgress(tf)
	}

	slog.Info("task flow milestone completed",
		"taskflow_id", tfID,
		"milestone_id", milestoneID,
		"overall_progress", tf.OverallProgress,
	)

	return nil
}

// FailMilestone marks a milestone as failed
func (s *TaskFlowService) FailMilestone(tfID, milestoneID string, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tf, exists := s.taskFlows[tfID]
	if !exists {
		return fmt.Errorf("task flow %s not found", tfID)
	}

	// Fail via milestone service
	if err := s.ms.Fail(milestoneID, reason); err != nil {
		return err
	}

	// Mark task flow as failed
	if err := tf.Fail(reason); err != nil {
		return err
	}

	s.ev.EmitTaskFlowFailed(tf, reason)

	slog.Error("task flow milestone failed",
		"taskflow_id", tfID,
		"milestone_id", milestoneID,
		"reason", reason,
	)

	return nil
}

// GetProgress returns the current progress of a task flow
func (s *TaskFlowService) GetProgress(tfID string) (float64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tf, exists := s.taskFlows[tfID]
	if !exists {
		return 0, fmt.Errorf("task flow %s not found", tfID)
	}

	return tf.OverallProgress, nil
}

// List returns all task flows
func (s *TaskFlowService) List() []*types.TaskFlow {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*types.TaskFlow, 0, len(s.taskFlows))
	for _, tf := range s.taskFlows {
		result = append(result, tf)
	}
	return result
}

// GetTaskFlowStatus returns the status summary of a task flow
func (s *TaskFlowService) GetTaskFlowStatus(tfID string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tf, exists := s.taskFlows[tfID]
	if !exists {
		return "", fmt.Errorf("task flow %s not found", tfID)
	}

	return tf.GetStatusSummary(), nil
}
