package tasks

import (
	"fmt"
	"os"
	"strings"
)

// CLICommands provides CLI command handlers for task operations.
type CLICommands struct {
	manager *TaskManager
}

// NewCLICommands creates a new CLI commands handler.
func NewCLICommands(manager *TaskManager) *CLICommands {
	return &CLICommands{manager: manager}
}

// Command represents a parsed CLI command.
type Command struct {
	Name string
	Args []string
	Raw  string
}

// ParseCommand parses a task command string.
func ParseCommand(input string) *Command {
	input = strings.TrimSpace(input)
	if !strings.HasPrefix(input, "/task") {
		return nil
	}

	parts := strings.Fields(input)
	if len(parts) < 2 {
		return &Command{Name: "help"}
	}

	cmd := &Command{
		Name: parts[1],
		Raw:  input,
	}
	if len(parts) > 2 {
		cmd.Args = parts[2:]
	}
	return cmd
}

// Handle processes a task command and returns the output.
func (c *CLICommands) Handle(cmd *Command, sessionID string) string {
	switch cmd.Name {
	case "help", "h", "?":
		return c.help()

	case "create", "c":
		return c.create(sessionID, cmd.Args)

	case "list", "ls", "l":
		return c.list(sessionID)

	case "get", "g":
		return c.get(sessionID, cmd.Args)

	case "update", "u":
		return c.update(sessionID, cmd.Args)

	case "delete", "d":
		return c.delete(sessionID, cmd.Args)

	case "ready", "r":
		return c.ready(sessionID)

	case "dep", "add-dep":
		return c.addDep(sessionID, cmd.Args)

	case "plan", "p":
		return c.plan(cmd.Args)

	case "verify", "v":
		return c.verify(cmd.Args)

	default:
		return fmt.Sprintf("Unknown command: %s\nType /task help for usage.", cmd.Name)
	}
}

func (c *CLICommands) help() string {
	return `Task Commands:
  /task create <subject> [description]  - Create a new task
  /task list                          - List all tasks
  /task get <task_id>                  - Get task details
  /task update <task_id> [status]      - Update task status
  /task delete <task_id>               - Delete a task
  /task ready                         - Show ready tasks (not blocked)
  /task dep <task_id> <blocked_by>    - Add dependency
  /task plan <goal>                   - Generate plan for goal
  /task verify [files...]              - Verify changes

Examples:
  /task create "Fix bug" "Fix authentication issue"
  /task update task_abc123 in_progress
  /task dep task_b task_a
  /task plan "Add user authentication"
`
}

func (c *CLICommands) create(sessionID string, args []string) string {
	if len(args) < 1 {
		return "Usage: /task create <subject> [description]"
	}

	subject := args[0]
	description := ""
	if len(args) > 1 {
		description = strings.Join(args[1:], " ")
	}

	task := c.manager.Create(sessionID, subject, description)
	return fmt.Sprintf("✓ Task created: %s\n  Subject: %s\n  Status: %s",
		task.ID, task.Subject, task.Status)
}

func (c *CLICommands) list(sessionID string) string {
	tasks := c.manager.List(sessionID)
	if len(tasks) == 0 {
		return "No tasks found. Use /task create to add tasks."
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Tasks (%d):\n", len(tasks)))
	b.WriteString(strings.Repeat("-", 40) + "\n")

	for _, t := range tasks {
		status := "○"
		switch t.Status {
		case TaskStatusInProgress:
			status = "◐"
		case TaskStatusCompleted:
			status = "●"
		case TaskStatusFailed:
			status = "✗"
		}
		b.WriteString(fmt.Sprintf("%s %s [%s]\n", status, t.ID, t.Subject))
		if t.Status == TaskStatusInProgress {
			b.WriteString(fmt.Sprintf("  └─ %s\n", t.Description))
		}
		if len(t.BlockedBy) > 0 {
			b.WriteString(fmt.Sprintf("  └─ blocked by: %s\n", strings.Join(t.BlockedBy, ", ")))
		}
	}
	return b.String()
}

func (c *CLICommands) get(sessionID string, args []string) string {
	if len(args) < 1 {
		return "Usage: /task get <task_id>"
	}

	taskID := args[0]
	task, ok := c.manager.Get(sessionID, taskID)
	if !ok {
		return fmt.Sprintf("Task not found: %s", taskID)
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Task: %s\n", task.ID))
	b.WriteString(fmt.Sprintf("Subject: %s\n", task.Subject))
	b.WriteString(fmt.Sprintf("Status: %s\n", task.Status))
	b.WriteString(fmt.Sprintf("Description: %s\n", task.Description))
	if task.Owner != "" {
		b.WriteString(fmt.Sprintf("Owner: %s\n", task.Owner))
	}
	if len(task.BlockedBy) > 0 {
		b.WriteString(fmt.Sprintf("Blocked by: %s\n", strings.Join(task.BlockedBy, ", ")))
	}
	if len(task.Blocks) > 0 {
		b.WriteString(fmt.Sprintf("Blocks: %s\n", strings.Join(task.Blocks, ", ")))
	}
	b.WriteString(fmt.Sprintf("Created: %s\n", task.CreatedAt.Format("2006-01-02 15:04")))
	b.WriteString(fmt.Sprintf("Updated: %s\n", task.UpdatedAt.Format("2006-01-02 15:04")))
	return b.String()
}

func (c *CLICommands) update(sessionID string, args []string) string {
	if len(args) < 1 {
		return "Usage: /task update <task_id> [status] [owner]"
	}

	taskID := args[0]
	var err error

	if len(args) > 1 {
		status := TaskStatus(args[1])
		err = c.manager.UpdateStatus(sessionID, taskID, status)
		if err != nil {
			return fmt.Sprintf("Error: %v", err)
		}
	}

	if len(args) > 2 {
		owner := args[2]
		err = c.manager.SetOwner(sessionID, taskID, owner)
		if err != nil {
			return fmt.Sprintf("Error: %v", err)
		}
	}

	task, _ := c.manager.Get(sessionID, taskID)
	return fmt.Sprintf("✓ Task updated: %s (status: %s)", task.ID, task.Status)
}

func (c *CLICommands) delete(sessionID string, args []string) string {
	if len(args) < 1 {
		return "Usage: /task delete <task_id>"
	}

	taskID := args[0]
	err := c.manager.RemoveTask(sessionID, taskID)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	return fmt.Sprintf("✓ Task deleted: %s", taskID)
}

func (c *CLICommands) ready(sessionID string) string {
	tasks := c.manager.GetReadyTasks(sessionID)
	if len(tasks) == 0 {
		return "No ready tasks. All tasks are blocked or completed."
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Ready tasks (%d):\n", len(tasks)))
	b.WriteString(strings.Repeat("-", 40) + "\n")
	for _, t := range tasks {
		b.WriteString(fmt.Sprintf("○ %s [%s]\n", t.ID, t.Subject))
	}
	return b.String()
}

func (c *CLICommands) addDep(sessionID string, args []string) string {
	if len(args) < 2 {
		return "Usage: /task dep <task_id> <blocked_by_task_id>"
	}

	taskID := args[0]
	blockedByID := args[1]

	err := c.manager.AddDependency(sessionID, taskID, blockedByID)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	return fmt.Sprintf("✓ Added dependency: %s blocked by %s", taskID, blockedByID)
}

func (c *CLICommands) plan(args []string) string {
	if len(args) < 1 {
		return "Usage: /task plan <goal description>"
	}

	goal := strings.Join(args, " ")
	return fmt.Sprintf(`Plan generation for: "%s"

To generate a plan, use /plan command instead:
  /plan %s

Use /task create to manually create tasks.
`, goal, goal)
}

func (c *CLICommands) verify(args []string) string {
	if len(args) < 1 {
		entries, err := os.ReadDir(".")
		if err != nil {
			return "Usage: /task verify [files...]"
		}
		var files []string
		for _, e := range entries {
			if !e.IsDir() {
				files = append(files, e.Name())
			}
		}
		if len(files) > 10 {
			files = files[:10]
		}
		args = files
	}

	return fmt.Sprintf(`Verification for files: %s

To verify changes, the VerificationAgent will:
1. Run tests
2. Check edge cases
3. Attempt adversarial probes
4. Report PASS/FAIL/PARTIAL

Note: VerificationAgent integration with LLM is pending.
`, strings.Join(args, ", "))
}

// PlanCLICommands handles /plan commands.
type PlanCLICommands struct {
	planMode *PlanMode
}

// NewPlanCLICommands creates plan CLI commands.
func NewPlanCLICommands(pm *PlanMode) *PlanCLICommands {
	return &PlanCLICommands{planMode: pm}
}

// Handle processes a /plan command.
func (c *PlanCLICommands) Handle(args []string, sessionID, workDir string, tools []string) string {
	if len(args) < 1 {
		return c.help()
	}

	switch args[0] {
	case "help", "h", "?":
		return c.help()
	case "enter", "e":
		return c.enter(args[1:], sessionID)
	case "approve", "a", "yes", "y":
		return c.approve(sessionID)
	case "reject", "r", "no", "n":
		return c.reject()
	case "status", "s":
		return c.status()
	case "show":
		return c.show()
	default:
		return fmt.Sprintf("Unknown subcommand: %s\n%s", args[0], c.help())
	}
}

func (c *PlanCLICommands) help() string {
	return `Plan Commands:
  /plan <goal>   - Enter plan mode with a goal
  /plan enter    - Enter plan mode (same as /plan)
  /plan approve  - Approve the plan and execute
  /plan reject   - Reject the plan
  /plan status   - Show current plan mode status
  /plan show     - Show the current plan

Examples:
  /plan Add user authentication
  /plan Add unit tests for auth module
  /plan approve
  /plan reject
`
}

func (c *PlanCLICommands) enter(args []string, sessionID string) string {
	if c.planMode == nil {
		return "Plan mode not available (LLM not configured)"
	}

	goal := strings.Join(args, " ")
	if goal == "" {
		return "Please provide a goal: /plan <goal description>"
	}

	_ = c.planMode.Enter(nil, sessionID, goal)

	return fmt.Sprintf(`Entered plan mode.

Goal: %s

PlanAgent will explore the codebase and design an implementation plan.
Use /plan show to see the generated plan.`, goal)
}

func (c *PlanCLICommands) approve(sessionID string) string {
	if c.planMode == nil {
		return "Plan mode not available"
	}

	if !c.planMode.IsActive() {
		return "Plan mode is not active"
	}

	tasks := c.planMode.Approve()
	if len(tasks) == 0 {
		return "No tasks in the plan"
	}

	c.planMode.Exit()

	var b strings.Builder
	b.WriteString("Plan approved. Creating tasks:\n\n")
	for _, task := range tasks {
		b.WriteString(fmt.Sprintf("✓ %s: %s\n", task.ID, task.Subject))
	}
	b.WriteString("\nTasks have been added to the task list.")

	return b.String()
}

func (c *PlanCLICommands) reject() string {
	if c.planMode == nil {
		return "Plan mode not available"
	}

	c.planMode.Reject()
	return "Plan rejected. Plan mode exited."
}

func (c *PlanCLICommands) status() string {
	if c.planMode == nil {
		return "Plan mode not available (LLM not configured)"
	}

	state := c.planMode.GetState()
	switch state {
	case PlanModeInactive:
		return "Plan mode: Inactive"
	case PlanModeActive:
		return "Plan mode: Active (generating plan...)"
	case PlanModePending:
		return "Plan mode: Pending approval\n\nUse /plan show to see the plan, /plan approve to proceed, or /plan reject to cancel."
	default:
		return fmt.Sprintf("Plan mode: Unknown state %s", state)
	}
}

func (c *PlanCLICommands) show() string {
	if c.planMode == nil {
		return "Plan mode not available"
	}

	return c.planMode.GetDisplayPlan()
}
