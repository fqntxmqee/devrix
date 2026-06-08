package milestone

import (
	"testing"

	"github.com/devrix/devrix/internal/shared/types"
)

// mockEventEmitter implements EventEmitter for testing
type mockEventEmitter struct {
	created []*types.Milestone
	updated []*types.Milestone
	completed []*types.Milestone
	failed   []*struct {
		m      *types.Milestone
		reason string
	}
}

func (m *mockEventEmitter) EmitMilestoneCreated(milestone *types.Milestone) {
	m.created = append(m.created, milestone)
}

func (m *mockEventEmitter) EmitMilestoneUpdated(milestone *types.Milestone) {
	m.updated = append(m.updated, milestone)
}

func (m *mockEventEmitter) EmitMilestoneCompleted(milestone *types.Milestone) {
	m.completed = append(m.completed, milestone)
}

func (m *mockEventEmitter) EmitMilestoneFailed(milestone *types.Milestone, reason string) {
	m.failed = append(m.failed, &struct {
		m      *types.Milestone
		reason string
	}{milestone, reason})
}

// TestNewMilestoneService tests creating a new milestone service
func TestNewMilestoneService(t *testing.T) {
	svc := NewMilestoneService(nil)
	if svc == nil {
		t.Fatal("NewMilestoneService returned nil")
	}
}

// TestMilestoneService_Create tests creating milestones
func TestMilestoneService_Create(t *testing.T) {
	svc := NewMilestoneService(nil)
	mock := &mockEventEmitter{}
	svc.SetEventEmitter(mock)

	m := &types.Milestone{
		ID:   "milestone_1",
		Name: "Test Milestone",
		TaskID: "task_1",
	}

	err := svc.Create(m)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Verify milestone was created
	got, err := svc.Get("milestone_1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Name != "Test Milestone" {
		t.Errorf("Name = %s, want Test Milestone", got.Name)
	}

	// Verify event was emitted
	if len(mock.created) != 1 {
		t.Errorf("created count = %d, want 1", len(mock.created))
	}
}

// TestMilestoneService_Create_Duplicate tests creating duplicate milestone
func TestMilestoneService_Create_Duplicate(t *testing.T) {
	svc := NewMilestoneService(nil)

	m := &types.Milestone{
		ID:   "milestone_1",
		Name: "Test Milestone",
		TaskID: "task_1",
	}

	err := svc.Create(m)
	if err != nil {
		t.Fatalf("first Create() error = %v", err)
	}

	err = svc.Create(m)
	if err == nil {
		t.Error("duplicate Create() should return error")
	}
}

// TestMilestoneService_Create_NoID tests creating milestone without ID
func TestMilestoneService_Create_NoID(t *testing.T) {
	svc := NewMilestoneService(nil)

	m := &types.Milestone{
		Name: "Test Milestone",
		TaskID: "task_1",
	}

	err := svc.Create(m)
	if err == nil {
		t.Error("Create() with no ID should return error")
	}
}

// TestMilestoneService_Get_NotFound tests getting non-existent milestone
func TestMilestoneService_Get_NotFound(t *testing.T) {
	svc := NewMilestoneService(nil)

	_, err := svc.Get("nonexistent")
	if err == nil {
		t.Error("Get() for non-existent should return error")
	}
}

// TestMilestoneService_UpdateProgress tests updating milestone progress
func TestMilestoneService_UpdateProgress(t *testing.T) {
	svc := NewMilestoneService(nil)
	mock := &mockEventEmitter{}
	svc.SetEventEmitter(mock)

	m := &types.Milestone{
		ID:   "milestone_1",
		Name: "Test Milestone",
		TaskID: "task_1",
	}
	svc.Create(m)

	err := svc.UpdateProgress("milestone_1", 0.5)
	if err != nil {
		t.Fatalf("UpdateProgress() error = %v", err)
	}

	// Verify progress was updated
	got, _ := svc.Get("milestone_1")
	if got.Progress != 0.5 {
		t.Errorf("Progress = %f, want 0.5", got.Progress)
	}

	// Verify event was emitted
	if len(mock.updated) != 1 {
		t.Errorf("updated count = %d, want 1", len(mock.updated))
	}
}

// TestMilestoneService_UpdateProgress_NotFound tests updating non-existent milestone
func TestMilestoneService_UpdateProgress_NotFound(t *testing.T) {
	svc := NewMilestoneService(nil)

	err := svc.UpdateProgress("nonexistent", 0.5)
	if err == nil {
		t.Error("UpdateProgress() for non-existent should return error")
	}
}

// TestMilestoneService_Complete tests completing a milestone
func TestMilestoneService_Complete(t *testing.T) {
	svc := NewMilestoneService(nil)
	mock := &mockEventEmitter{}
	svc.SetEventEmitter(mock)

	m := &types.Milestone{
		ID:   "milestone_1",
		Name: "Test Milestone",
		TaskID: "task_1",
	}
	svc.Create(m)

	err := svc.Complete("milestone_1")
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	// Verify status and progress
	got, _ := svc.Get("milestone_1")
	if got.Status != types.MilestoneStatusCompleted {
		t.Errorf("Status = %s, want %s", got.Status, types.MilestoneStatusCompleted)
	}
	if got.Progress != 1.0 {
		t.Errorf("Progress = %f, want 1.0", got.Progress)
	}

	// Verify event was emitted
	if len(mock.completed) != 1 {
		t.Errorf("completed count = %d, want 1", len(mock.completed))
	}
}

// TestMilestoneService_Complete_AlreadyCompleted tests completing already completed milestone
func TestMilestoneService_Complete_AlreadyCompleted(t *testing.T) {
	svc := NewMilestoneService(nil)

	m := &types.Milestone{
		ID:   "milestone_1",
		Name: "Test Milestone",
		TaskID: "task_1",
	}
	svc.Create(m)

	svc.Complete("milestone_1")
	err := svc.Complete("milestone_1")
	if err == nil {
		t.Error("Complete() on already completed should return error")
	}
}

// TestMilestoneService_Fail tests failing a milestone
func TestMilestoneService_Fail(t *testing.T) {
	svc := NewMilestoneService(nil)
	mock := &mockEventEmitter{}
	svc.SetEventEmitter(mock)

	m := &types.Milestone{
		ID:   "milestone_1",
		Name: "Test Milestone",
		TaskID: "task_1",
	}
	svc.Create(m)

	err := svc.Fail("milestone_1", "something went wrong")
	if err != nil {
		t.Fatalf("Fail() error = %v", err)
	}

	// Verify status
	got, _ := svc.Get("milestone_1")
	if got.Status != types.MilestoneStatusFailed {
		t.Errorf("Status = %s, want %s", got.Status, types.MilestoneStatusFailed)
	}

	// Verify event was emitted
	if len(mock.failed) != 1 {
		t.Errorf("failed count = %d, want 1", len(mock.failed))
	}
	if mock.failed[0].reason != "something went wrong" {
		t.Errorf("reason = %s, want 'something went wrong'", mock.failed[0].reason)
	}
}

// TestMilestoneService_AddDependency tests adding dependencies between milestones
func TestMilestoneService_AddDependency(t *testing.T) {
	svc := NewMilestoneService(nil)

	m1 := &types.Milestone{ID: "milestone_1", Name: "First", TaskID: "task_1"}
	m2 := &types.Milestone{ID: "milestone_2", Name: "Second", TaskID: "task_1"}
	svc.Create(m1)
	svc.Create(m2)

	err := svc.AddDependency("milestone_2", "milestone_1")
	if err != nil {
		t.Fatalf("AddDependency() error = %v", err)
	}

	// Verify dependency was added
	dag := svc.GetDAG()
	m2Obj, _ := dag.GetMilestone("milestone_2")
	deps := m2Obj.Dependencies
	if len(deps) != 1 || deps[0] != "milestone_1" {
		t.Errorf("Dependencies = %v, want [milestone_1]", deps)
	}
}

// TestMilestoneService_AddDependency_Self tests adding self-dependency
func TestMilestoneService_AddDependency_Self(t *testing.T) {
	svc := NewMilestoneService(nil)

	m := &types.Milestone{ID: "milestone_1", Name: "Test", TaskID: "task_1"}
	svc.Create(m)

	err := svc.AddDependency("milestone_1", "milestone_1")
	if err == nil {
		t.Error("AddDependency() with self should return error")
	}
}

// TestMilestoneService_GetMilestonesByTaskID tests filtering milestones by task ID
func TestMilestoneService_GetMilestonesByTaskID(t *testing.T) {
	svc := NewMilestoneService(nil)

	m1 := &types.Milestone{ID: "milestone_1", Name: "First", TaskID: "task_1"}
	m2 := &types.Milestone{ID: "milestone_2", Name: "Second", TaskID: "task_2"}
	m3 := &types.Milestone{ID: "milestone_3", Name: "Third", TaskID: "task_1"}
	svc.Create(m1)
	svc.Create(m2)
	svc.Create(m3)

	milestones := svc.GetMilestonesByTaskID("task_1")
	if len(milestones) != 2 {
		t.Errorf("milestones count = %d, want 2", len(milestones))
	}
}

// TestMilestoneService_CalculateOverallProgress tests overall progress calculation
func TestMilestoneService_CalculateOverallProgress(t *testing.T) {
	svc := NewMilestoneService(nil)

	m1 := &types.Milestone{ID: "milestone_1", Name: "First", TaskID: "task_1"}
	m2 := &types.Milestone{ID: "milestone_2", Name: "Second", TaskID: "task_1"}
	svc.Create(m1)
	svc.Create(m2)

	// Set different progress values
	svc.UpdateProgress("milestone_1", 0.5)
	svc.UpdateProgress("milestone_2", 1.0)

	progress := svc.CalculateOverallProgress()
	// (0.5 + 1.0) / 2 = 0.75
	if progress != 0.75 {
		t.Errorf("OverallProgress = %f, want 0.75", progress)
	}
}

// Covers: L5-1-5-01
func TestMilestoneService_AddDependency_CycleRejected(t *testing.T) {
	svc := NewMilestoneService(nil)

	m1 := &types.Milestone{ID: "m1", Name: "A", TaskID: "task_1"}
	m2 := &types.Milestone{ID: "m2", Name: "B", TaskID: "task_1"}
	m3 := &types.Milestone{ID: "m3", Name: "C", TaskID: "task_1"}
	for _, m := range []*types.Milestone{m1, m2, m3} {
		if err := svc.Create(m); err != nil {
			t.Fatalf("Create(%s) error = %v", m.ID, err)
		}
	}

	if err := svc.AddDependency("m2", "m1"); err != nil {
		t.Fatalf("AddDependency(m2,m1) error = %v", err)
	}
	if err := svc.AddDependency("m3", "m2"); err != nil {
		t.Fatalf("AddDependency(m3,m2) error = %v", err)
	}
	if err := svc.AddDependency("m1", "m3"); err == nil {
		t.Fatal("AddDependency(m1,m3) should reject cycle")
	}
}
