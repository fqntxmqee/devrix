package workmodel

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/devrix/devrix/internal/layers/contextengine/policy/toolrunner"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

// RegisterTaskTools registers task_* tools when Tasks.Mode is v2.
func RegisterTaskTools(reg *toolrunner.ToolRegistry, cfg *config.ContextEngineConfig, manager *TaskManager) error {
	if reg == nil || cfg == nil || manager == nil {
		return nil
	}
	if cfg.Tasks.Mode != "v2" {
		return nil
	}
	for _, runner := range newTaskToolRunners(manager) {
		if err := reg.Register(runner); err != nil {
			return err
		}
	}
	return nil
}

type taskAction int

const (
	taskActionCreate taskAction = iota
	taskActionGet
	taskActionList
	taskActionUpdate
)

type taskToolRunner struct {
	name    string
	manager *TaskManager
	action  taskAction
}

func newTaskToolRunners(manager *TaskManager) []toolrunner.PluginRunner {
	return []toolrunner.PluginRunner{
		&taskToolRunner{name: ToolNameTaskCreate, manager: manager, action: taskActionCreate},
		&taskToolRunner{name: ToolNameTaskGet, manager: manager, action: taskActionGet},
		&taskToolRunner{name: ToolNameTaskList, manager: manager, action: taskActionList},
		&taskToolRunner{name: ToolNameTaskUpdate, manager: manager, action: taskActionUpdate},
	}
}

func (r *taskToolRunner) Name() string { return r.name }

func (r *taskToolRunner) RiskLevel() types.RiskLevel { return types.RiskLevelLow }

func (r *taskToolRunner) Schema() toolrunner.ToolSchema {
	switch r.action {
	case taskActionCreate:
		return toolrunner.ToolSchema{
			Name: r.name, Description: "Create a persisted task",
			Parameters: `{"type":"object","required":["subject"],"properties":{"subject":{"type":"string"},"description":{"type":"string"}}}`,
		}
	case taskActionGet:
		return toolrunner.ToolSchema{
			Name: r.name, Description: "Get a task by id",
			Parameters: `{"type":"object","required":["task_id"],"properties":{"task_id":{"type":"string"}}}`,
		}
	case taskActionList:
		return toolrunner.ToolSchema{Name: r.name, Description: "List all tasks for the session", Parameters: `{"type":"object","properties":{}}`}
	default:
		return toolrunner.ToolSchema{
			Name: r.name, Description: "Update a task",
			Parameters: `{"type":"object","required":["task_id"],"properties":{"task_id":{"type":"string"},"status":{"type":"string"},"owner":{"type":"string"},"blocked_by":{"type":"string"}}}`,
		}
	}
}

func (r *taskToolRunner) Execute(ctx context.Context, _, input string) (*toolrunner.ToolResult, error) {
	sessionID := toolrunner.ToolSessionIDFromContext(ctx)
	if sessionID == "" {
		return &toolrunner.ToolResult{Error: r.name + ": session_id unavailable"}, nil
	}
	fields := parseToolInput(input)
	suite := NewToolSuite(r.manager)
	out, err := suite.Execute(ctx, TaskToolInput{
		SessionID:   sessionID,
		ToolName:    r.name,
		TaskID:      fields["task_id"],
		Subject:     fields["subject"],
		Description: fields["description"],
		Status:      fields["status"],
		Owner:       fields["owner"],
		BlockedBy:   fields["blocked_by"],
	})
	if err != nil {
		return &toolrunner.ToolResult{Error: err.Error()}, nil
	}
	if out == nil || !out.Success {
		msg := "task operation failed"
		if out != nil && out.Message != "" {
			msg = out.Message
		}
		return &toolrunner.ToolResult{Error: msg}, nil
	}
	data, _ := json.Marshal(out)
	return &toolrunner.ToolResult{Output: string(data)}, nil
}

func parseToolInput(input string) map[string]string {
	fields := make(map[string]string)
	if input == "" {
		return fields
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(input), &raw); err != nil {
		return fields
	}
	for k, v := range raw {
		if s, ok := v.(string); ok {
			fields[k] = s
		} else if v != nil {
			fields[k] = fmt.Sprint(v)
		}
	}
	return fields
}
