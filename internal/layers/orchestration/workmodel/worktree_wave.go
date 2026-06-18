package workmodel

import (
	"fmt"

	"github.com/devrix/devrix/internal/layers/orchestration/wave"
)

// SyncWaveNodes writes wave TaskNodes into the work tree under a batch root.
func (m *TaskManager) SyncWaveNodes(sessionID string, nodes []wave.TaskNode) (batchRootID string, err error) {
	if m == nil || len(nodes) == 0 {
		return "", nil
	}
	goal, _ := m.tree.EnsureGoal(sessionID, "wave batch")
	batch, err := m.tree.Create(sessionID, CreateWorkItemInput{
		ParentID: goal.ID,
		Kind:     WorkKindPlan,
		Title:    "Wave batch",
		Directive: "parallel implement batch",
		Policy:   ExecPolicyParallelOK,
	})
	if err != nil {
		return "", err
	}
	idMap := make(map[string]string, len(nodes))
	for _, n := range nodes {
		item, err := m.tree.Create(sessionID, CreateWorkItemInput{
			ParentID: batch.ID,
			Kind:     WorkKindImplement,
			Title:    n.Title,
			Directive: n.Directive,
			Policy:   ExecPolicyParallelOK,
		})
		if err != nil {
			return "", err
		}
		idMap[n.ID] = item.ID
	}
	for i, n := range nodes {
		wiID := idMap[n.ID]
		for _, dep := range n.DependsOn {
			if mapped, ok := idMap[dep]; ok {
				_ = m.tree.AddDependency(sessionID, wiID, mapped)
			}
		}
		nodes[i].ID = wiID
		nodes[i].Metadata = map[string]any{"work_item_id": wiID}
	}
	return batch.ID, nil
}

// WaveNodesFromSubtree projects ready implement items to wave TaskNodes.
func (m *TaskManager) WaveNodesFromSubtree(sessionID, batchRootID string) []wave.TaskNode {
	subtree := m.tree.ListSubtree(sessionID, batchRootID)
	var nodes []wave.TaskNode
	for _, item := range subtree {
		if item == nil || item.Kind != WorkKindImplement {
			continue
		}
		if item.Status != TaskStatusPending {
			continue
		}
		nodes = append(nodes, wave.TaskNode{
			ID:            item.ID,
			Title:         item.Title,
			Directive:     item.Directive,
			WorkerType:    wave.WorkerSubAgent,
			ContextPolicy: wave.ContextFresh,
			DependsOn:     append([]string(nil), item.BlockedBy...),
			Metadata:      map[string]any{"work_item_id": item.ID},
		})
	}
	return nodes
}

// ProjectTodosFromChecklist builds session todo projection from checklist children.
func (m *TaskManager) ProjectTodosFromChecklist(sessionID, parentID string) []ChecklistEntry {
	children := m.tree.ListChildren(sessionID, parentID)
	var out []ChecklistEntry
	for _, c := range children {
		if c.Kind != WorkKindChecklist {
			continue
		}
		out = append(out, ChecklistEntry{
			Content: c.Directive,
			Status:  c.Status,
			ActiveForm: c.Title,
		})
	}
	return out
}

// ResolveFocusKind maps delegate role to work kind.
func ResolveFocusKind(role string) WorkKind {
	switch role {
	case "explore":
		return WorkKindExplore
	case "plan":
		return WorkKindPlan
	case "implement":
		return WorkKindImplement
	case "verify":
		return WorkKindVerify
	default:
		return WorkKindImplement
	}
}

// ErrInvalidDispatch indicates spawn is not allowed for the kind.
var ErrInvalidDispatch = fmt.Errorf("invalid dispatch for work kind")

// CanSpawn reports whether a kind supports spawn dispatch.
func CanSpawn(kind WorkKind) bool {
	switch kind {
	case WorkKindGoal, WorkKindChecklist:
		return false
	default:
		return true
	}
}
