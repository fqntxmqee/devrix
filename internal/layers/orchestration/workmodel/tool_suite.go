package workmodel

import (
	"context"
	"encoding/json"
	"fmt"
)

// ToolSuite provides task management tools for LLM.
type ToolSuite struct {
	manager *TaskManager
}

// NewToolSuite creates a new tool suite.
func NewToolSuite(manager *TaskManager) *ToolSuite {
	return &ToolSuite{manager: manager}
}

// TaskToolInput is the input for task operations.
type TaskToolInput struct {
	SessionID   string `json:"session_id"`
	ToolName    string `json:"tool_name"`
	TaskID      string `json:"task_id,omitempty"`
	Subject     string `json:"subject,omitempty"`
	Description string `json:"description,omitempty"`
	Status      string `json:"status,omitempty"`
	Owner       string `json:"owner,omitempty"`
	BlockedBy   string `json:"blocked_by,omitempty"`
	Format      string `json:"format,omitempty"`
}

// TaskToolOutput is the output from task operations.
type TaskToolOutput struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Tool names
const (
	ToolNameTaskCreate   = "task_create"
	ToolNameTaskGet     = "task_get"
	ToolNameTaskList    = "task_list"
	ToolNameTaskUpdate  = "task_update"
	ToolNameTaskDelete  = "task_delete"
)

// Execute runs a task tool by name.
func (t *ToolSuite) Execute(ctx context.Context, input TaskToolInput) (*TaskToolOutput, error) {
	switch input.ToolName {
	case ToolNameTaskCreate:
		return t.Create(ctx, input)
	case ToolNameTaskGet:
		return t.Get(ctx, input)
	case ToolNameTaskList:
		return t.List(ctx, input)
	case ToolNameTaskUpdate:
		return t.Update(ctx, input)
	case ToolNameTaskDelete:
		return t.Delete(ctx, input)
	default:
		return nil, fmt.Errorf("unknown tool: %s", input.ToolName)
	}
}

// Create creates a new task.
func (t *ToolSuite) Create(_ context.Context, input TaskToolInput) (*TaskToolOutput, error) {
	if input.Subject == "" {
		return &TaskToolOutput{
			Success: false,
			Message: "subject is required",
		}, nil
	}

	task := t.manager.Create(input.SessionID, input.Subject, input.Description)

	return &TaskToolOutput{
		Success: true,
		Message: fmt.Sprintf("Task created: %s", task.ID),
		Data:    task,
	}, nil
}

// Get retrieves a task.
func (t *ToolSuite) Get(_ context.Context, input TaskToolInput) (*TaskToolOutput, error) {
	if input.TaskID == "" {
		return &TaskToolOutput{
			Success: false,
			Message: "task_id is required",
		}, nil
	}

	task, ok := t.manager.Get(input.SessionID, input.TaskID)
	if !ok {
		return &TaskToolOutput{
			Success: false,
			Message: fmt.Sprintf("Task not found: %s", input.TaskID),
		}, nil
	}

	return &TaskToolOutput{
		Success: true,
		Message: "Task found",
		Data:    task,
	}, nil
}

// List returns all tasks for session (optional format=tree).
func (t *ToolSuite) List(_ context.Context, input TaskToolInput) (*TaskToolOutput, error) {
	if input.Format == "tree" {
		tree := t.manager.Tree().BuildTree(input.SessionID, "")
		return &TaskToolOutput{
			Success: true,
			Message: fmt.Sprintf("Found %d root nodes", len(tree)),
			Data:    tree,
		}, nil
	}
	tasks := t.manager.List(input.SessionID)

	return &TaskToolOutput{
		Success: true,
		Message: fmt.Sprintf("Found %d tasks", len(tasks)),
		Data:    tasks,
	}, nil
}

// Update updates a task.
func (t *ToolSuite) Update(_ context.Context, input TaskToolInput) (*TaskToolOutput, error) {
	if input.TaskID == "" {
		return &TaskToolOutput{
			Success: false,
			Message: "task_id is required",
		}, nil
	}

	var err error
	var message string

	// Update status if provided
	if input.Status != "" {
		err = t.manager.UpdateStatus(input.SessionID, input.TaskID, TaskStatus(input.Status))
		if err != nil {
			return &TaskToolOutput{
				Success: false,
				Message: err.Error(),
			}, nil
		}
		message = fmt.Sprintf("Status updated to %s", input.Status)
	}

	// Update owner if provided
	if input.Owner != "" {
		err = t.manager.SetOwner(input.SessionID, input.TaskID, input.Owner)
		if err != nil {
			return &TaskToolOutput{
				Success: false,
				Message: err.Error(),
			}, nil
		}
		if message != "" {
			message += ", "
		}
		message += fmt.Sprintf("Owner set to %s", input.Owner)
	}

	// Add dependency if provided
	if input.BlockedBy != "" {
		err = t.manager.AddDependency(input.SessionID, input.TaskID, input.BlockedBy)
		if err != nil {
			return &TaskToolOutput{
				Success: false,
				Message: err.Error(),
			}, nil
		}
		if message != "" {
			message += ", "
		}
		message += fmt.Sprintf("Blocked by %s", input.BlockedBy)
	}

	if message == "" {
		message = "Task updated"
	}

	return &TaskToolOutput{
		Success: true,
		Message: message,
	}, nil
}

// Delete removes a task.
func (t *ToolSuite) Delete(_ context.Context, input TaskToolInput) (*TaskToolOutput, error) {
	if input.TaskID == "" {
		return &TaskToolOutput{
			Success: false,
			Message: "task_id is required",
		}, nil
	}

	err := t.manager.RemoveTask(input.SessionID, input.TaskID)
	if err != nil {
		return &TaskToolOutput{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	return &TaskToolOutput{
		Success: true,
		Message: fmt.Sprintf("Task deleted: %s", input.TaskID),
	}, nil
}

// ToJSON converts output to JSON string for LLM.
func (o *TaskToolOutput) ToJSON() string {
	data, _ := json.MarshalIndent(o, "", "  ")
	return string(data)
}
