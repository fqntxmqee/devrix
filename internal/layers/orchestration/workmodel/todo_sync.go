package workmodel

import (
	"github.com/devrix/devrix/internal/shared/types"
)

// TodoWriteBackend syncs todo_write snapshots to WorkTree checklist nodes.
type TodoWriteBackend struct {
	Manager *TaskManager
}

// Sync replaces checklist children under session focus and returns projected todos.
func (b *TodoWriteBackend) Sync(sessionID string, todos []types.TodoItem) []types.TodoItem {
	if b == nil || b.Manager == nil || sessionID == "" {
		return todos
	}
	tree := b.Manager.Tree()
	focus, _ := tree.GetFocus(sessionID)
	parentID := ""
	if focus != nil {
		parentID = focus.ID
	} else if goal, err := tree.EnsureGoal(sessionID, "session"); err == nil && goal != nil {
		parentID = goal.ID
	}
	if parentID == "" {
		return todos
	}
	entries := make([]ChecklistEntry, 0, len(todos))
	for _, t := range todos {
		st := TaskStatusPending
		switch t.Status {
		case types.TodoStatusInProgress:
			st = TaskStatusInProgress
		case types.TodoStatusCompleted:
			st = TaskStatusCompleted
		}
		entries = append(entries, ChecklistEntry{
			Content:    t.Content,
			Status:     st,
			ActiveForm: t.ActiveForm,
		})
	}
	_ = tree.UpsertChecklist(sessionID, parentID, entries)
	return projectTodos(tree.ListChildren(sessionID, parentID))
}

func projectTodos(items []*WorkItem) []types.TodoItem {
	out := make([]types.TodoItem, 0, len(items))
	for _, item := range items {
		if item == nil || item.Kind != WorkKindChecklist {
			continue
		}
		st := types.TodoStatusPending
		switch item.Status {
		case TaskStatusInProgress:
			st = types.TodoStatusInProgress
		case TaskStatusCompleted:
			st = types.TodoStatusCompleted
		}
		out = append(out, types.TodoItem{
			Content:    item.Directive,
			Status:     st,
			ActiveForm: item.Title,
		})
	}
	return out
}
