package types

import (
	"fmt"
	"time"
)

// MilestoneStatus represents the status of a milestone
type MilestoneStatus string

const (
	MilestoneStatusPending    MilestoneStatus = "pending"
	MilestoneStatusInProgress MilestoneStatus = "in_progress"
	MilestoneStatusCompleted  MilestoneStatus = "completed"
	MilestoneStatusFailed    MilestoneStatus = "failed"
)

// Milestone represents a task milestone (Entity)
type Milestone struct {
	ID           string           // 唯一标识
	TaskID       string           // 所属任务 ID
	Name         string           // 里程碑名称
	Description  string           // 详细描述
	Status       MilestoneStatus  // pending | in_progress | completed | failed
	Progress     float64          // 进度 0.0-1.0
	Dependencies []string         // 依赖的里程碑 IDs
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// NewMilestone creates a new milestone with default values
func NewMilestone(id, taskID, name string) *Milestone {
	now := time.Now()
	return &Milestone{
		ID:           id,
		TaskID:       taskID,
		Name:         name,
		Status:       MilestoneStatusPending,
		Progress:     0.0,
		Dependencies: make([]string, 0),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

// SetStatus sets the milestone status
func (m *Milestone) SetStatus(status MilestoneStatus) {
	m.Status = status
	m.UpdatedAt = time.Now()
}

// SetProgress sets the milestone progress
func (m *Milestone) SetProgress(progress float64) error {
	if progress < 0.0 || progress > 1.0 {
		return fmt.Errorf("progress must be between 0.0 and 1.0, got %f", progress)
	}
	m.Progress = progress
	m.UpdatedAt = time.Now()
	return nil
}

// AddDependency adds a dependency to this milestone
func (m *Milestone) AddDependency(depID string) {
	for _, id := range m.Dependencies {
		if id == depID {
			return // Already exists
		}
	}
	m.Dependencies = append(m.Dependencies, depID)
	m.UpdatedAt = time.Now()
}

// IsBlocked returns true if any dependency is not completed
func (m *Milestone) IsBlocked(milestones map[string]*Milestone) bool {
	for _, depID := range m.Dependencies {
		dep, exists := milestones[depID]
		if !exists {
			continue // Treat missing as blocked
		}
		if dep.Status != MilestoneStatusCompleted {
			return true
		}
	}
	return false
}

// MilestoneDAG represents a directed acyclic graph of milestones
type MilestoneDAG struct {
	ID              string
	RootMilestoneID string
	Milestones      map[string]*Milestone
}

// NewMilestoneDAG creates a new milestone DAG
func NewMilestoneDAG(id, rootMilestoneID string) *MilestoneDAG {
	return &MilestoneDAG{
		ID:              id,
		RootMilestoneID: rootMilestoneID,
		Milestones:      make(map[string]*Milestone),
	}
}

// AddMilestone adds a milestone to the DAG
func (d *MilestoneDAG) AddMilestone(m *Milestone) error {
	if _, exists := d.Milestones[m.ID]; exists {
		return fmt.Errorf("milestone %s already exists", m.ID)
	}
	d.Milestones[m.ID] = m
	return nil
}

// GetMilestone returns a milestone by ID
func (d *MilestoneDAG) GetMilestone(id string) (*Milestone, bool) {
	m, exists := d.Milestones[id]
	return m, exists
}

// HasCycle checks if adding a dependency would create a cycle
func (d *MilestoneDAG) HasCycle(fromID, toID string) bool {
	visited := make(map[string]bool)
	return d.dfs(toID, fromID, visited)
}

// dfs performs depth-first search to detect cycles
func (d *MilestoneDAG) dfs(current, target string, visited map[string]bool) bool {
	if current == target {
		return true
	}
	if visited[current] {
		return false
	}
	visited[current] = true

	m, exists := d.Milestones[current]
	if !exists {
		return false
	}

	for _, depID := range m.Dependencies {
		if d.dfs(depID, target, visited) {
			return true
		}
	}
	return false
}

// GetExecutionOrder returns milestones in topological order
func (d *MilestoneDAG) GetExecutionOrder() ([]*Milestone, error) {
	order := make([]*Milestone, 0, len(d.Milestones))
	visited := make(map[string]bool)
	inDegree := make(map[string]int)

	// Calculate in-degrees
	for id := range d.Milestones {
		inDegree[id] = 0
	}
	for _, m := range d.Milestones {
		inDegree[m.ID] += len(m.Dependencies)
	}

	// Start with nodes that have no dependencies
	var queue []string
	for id, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, id)
		}
	}

	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]

		if visited[id] {
			continue
		}

		m := d.Milestones[id]
		order = append(order, m)
		visited[id] = true

		// Reduce in-degree for milestones that depend on the completed node.
		for _, other := range d.Milestones {
			for _, depID := range other.Dependencies {
				if depID != id {
					continue
				}
				inDegree[other.ID]--
				if inDegree[other.ID] == 0 {
					queue = append(queue, other.ID)
				}
			}
		}
	}

	// Check if all nodes were visited (no cycles)
	if len(order) != len(d.Milestones) {
		return nil, fmt.Errorf("cycle detected in DAG")
	}

	return order, nil
}

// CalculateOverallProgress calculates the overall progress of all milestones
func (d *MilestoneDAG) CalculateOverallProgress() float64 {
	if len(d.Milestones) == 0 {
		return 0.0
	}

	var totalProgress float64
	for _, m := range d.Milestones {
		totalProgress += m.Progress
	}
	return totalProgress / float64(len(d.Milestones))
}
