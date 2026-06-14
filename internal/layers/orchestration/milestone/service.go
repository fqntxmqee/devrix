package milestone

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/devrix/devrix/internal/shared/types"
)

// IMilestoneService defines the interface for milestone operations
type IMilestoneService interface {
	// Create creates a new milestone
	Create(milestone *types.Milestone) error
	// Get retrieves a milestone by ID
	Get(id string) (*types.Milestone, error)
	// UpdateProgress updates the progress of a milestone
	UpdateProgress(id string, progress float64) error
	// Complete marks a milestone as completed
	Complete(id string) error
	// Fail marks a milestone as failed
	Fail(id string, reason string) error
	// AddDependency adds a dependency to a milestone
	AddDependency(milestoneID, dependencyID string) error
	// GetDAG returns the milestone DAG
	GetDAG() *types.MilestoneDAG
}

// MilestoneService implements IMilestoneService
type MilestoneService struct {
	mu   sync.RWMutex
	dag  *types.MilestoneDAG
	ev   EventEmitter
}

// EventEmitter emits milestone events
type EventEmitter interface {
	EmitMilestoneCreated(m *types.Milestone)
	EmitMilestoneUpdated(m *types.Milestone)
	EmitMilestoneCompleted(m *types.Milestone)
	EmitMilestoneFailed(m *types.Milestone, reason string)
}

// DefaultEventEmitter is a simple event emitter that logs events
type DefaultEventEmitter struct{}

func (e *DefaultEventEmitter) EmitMilestoneCreated(m *types.Milestone) {
	slog.Debug("milestone.created", "id", m.ID, "name", m.Name)
}

func (e *DefaultEventEmitter) EmitMilestoneUpdated(m *types.Milestone) {
	slog.Debug("milestone.updated", "id", m.ID, "progress", m.Progress)
}

func (e *DefaultEventEmitter) EmitMilestoneCompleted(m *types.Milestone) {
	slog.Info("milestone.completed", "id", m.ID, "name", m.Name)
}

func (e *DefaultEventEmitter) EmitMilestoneFailed(m *types.Milestone, reason string) {
	slog.Error("milestone.failed", "id", m.ID, "name", m.Name, "reason", reason)
}

// NewMilestoneService creates a new milestone service
func NewMilestoneService(dag *types.MilestoneDAG) *MilestoneService {
	if dag == nil {
		dag = types.NewMilestoneDAG("default", "")
	}
	return &MilestoneService{
		dag: dag,
		ev:  &DefaultEventEmitter{},
	}
}

// SetEventEmitter sets the event emitter
func (s *MilestoneService) SetEventEmitter(ev EventEmitter) {
	s.ev = ev
}

// Create creates a new milestone
func (s *MilestoneService) Create(m *types.Milestone) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if m.ID == "" {
		return fmt.Errorf("milestone ID is required")
	}

	if _, exists := s.dag.Milestones[m.ID]; exists {
		return fmt.Errorf("milestone %s already exists", m.ID)
	}

	s.dag.AddMilestone(m)
	s.ev.EmitMilestoneCreated(m)

	slog.Info("milestone created",
		"id", m.ID,
		"name", m.Name,
		"task_id", m.TaskID,
	)

	return nil
}

// Get retrieves a milestone by ID
func (s *MilestoneService) Get(id string) (*types.Milestone, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	m, exists := s.dag.Milestones[id]
	if !exists {
		return nil, fmt.Errorf("milestone %s not found", id)
	}

	return m, nil
}

// UpdateProgress updates the progress of a milestone
func (s *MilestoneService) UpdateProgress(id string, progress float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	m, exists := s.dag.Milestones[id]
	if !exists {
		return fmt.Errorf("milestone %s not found", id)
	}

	if err := m.SetProgress(progress); err != nil {
		return err
	}

	s.ev.EmitMilestoneUpdated(m)

	slog.Debug("milestone progress updated",
		"id", id,
		"progress", progress,
	)

	return nil
}

// Complete marks a milestone as completed
func (s *MilestoneService) Complete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	m, exists := s.dag.Milestones[id]
	if !exists {
		return fmt.Errorf("milestone %s not found", id)
	}

	if m.Status == types.MilestoneStatusCompleted {
		return fmt.Errorf("milestone %s already completed", id)
	}

	m.SetStatus(types.MilestoneStatusCompleted)
	m.SetProgress(1.0)

	s.ev.EmitMilestoneCompleted(m)

	slog.Info("milestone completed",
		"id", id,
		"name", m.Name,
	)

	return nil
}

// Fail marks a milestone as failed
func (s *MilestoneService) Fail(id string, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	m, exists := s.dag.Milestones[id]
	if !exists {
		return fmt.Errorf("milestone %s not found", id)
	}

	m.SetStatus(types.MilestoneStatusFailed)

	s.ev.EmitMilestoneFailed(m, reason)

	slog.Error("milestone failed",
		"id", id,
		"name", m.Name,
		"reason", reason,
	)

	return nil
}

// AddDependency adds a dependency to a milestone
func (s *MilestoneService) AddDependency(milestoneID, dependencyID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	m, exists := s.dag.Milestones[milestoneID]
	if !exists {
		return fmt.Errorf("milestone %s not found", milestoneID)
	}

	if _, exists = s.dag.Milestones[dependencyID]; !exists {
		return fmt.Errorf("dependency milestone %s not found", dependencyID)
	}

	// Check for self-dependency
	if milestoneID == dependencyID {
		return fmt.Errorf("cannot add self-dependency")
	}

	// Check for cycle
	if s.dag.HasCycle(milestoneID, dependencyID) {
		return fmt.Errorf("adding dependency would create a cycle")
	}

	m.AddDependency(dependencyID)

	slog.Info("milestone dependency added",
		"milestone_id", milestoneID,
		"dependency_id", dependencyID,
	)

	return nil
}

// GetDAG returns the milestone DAG
func (s *MilestoneService) GetDAG() *types.MilestoneDAG {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.dag
}

// GetExecutionOrder returns milestones in execution order
func (s *MilestoneService) GetExecutionOrder() ([]*types.Milestone, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.dag.GetExecutionOrder()
}

// CalculateOverallProgress calculates the overall progress
func (s *MilestoneService) CalculateOverallProgress() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.dag.CalculateOverallProgress()
}

// GetMilestonesByTaskID returns all milestones for a given task
func (s *MilestoneService) GetMilestonesByTaskID(taskID string) []*types.Milestone {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*types.Milestone
	for _, m := range s.dag.Milestones {
		if m.TaskID == taskID {
			result = append(result, m)
		}
	}
	return result
}
