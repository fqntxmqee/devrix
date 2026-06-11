package tasks

import (
	"context"
	"strings"
)

// PlanAgent generates task plans via LLM.
// Inspired by Claude Code's PlanAgent - READ-ONLY exploration and planning.
type PlanAgent struct {
	llm LLMCompleter
}

// LLMCompleter interface for LLM calls.
type LLMCompleter interface {
	Complete(ctx context.Context, prompt string) (string, error)
}

// NewPlanAgent creates a new PlanAgent.
func NewPlanAgent(llm LLMCompleter) *PlanAgent {
	return &PlanAgent{llm: llm}
}

// PlanRequest is the input for plan generation.
type PlanRequest struct {
	UserGoal      string   // 用户目标描述
	WorkDir      string   // 工作目录
	Context      string   // 额外上下文（如 CLAUDE.md 内容）
	Tools        []string // 可用工具列表
}

// PlanResult is the output from plan generation.
type PlanResult struct {
	Tasks       []*Task  // 计划的任务列表
	Exploration string   // 探索发现
	CriticalFiles []string // 关键文件列表
	Err         error
}

// Plan generates a plan for the given goal.
// This is READ-ONLY - only explores and plans, never modifies files.
func (a *PlanAgent) Plan(ctx context.Context, req PlanRequest) *PlanResult {
	if a.llm == nil {
		return &PlanResult{Err: ErrLLMNotConfigured}
	}

	prompt := buildPlanPrompt(req)
	response, err := a.llm.Complete(ctx, prompt)
	if err != nil {
		return &PlanResult{Err: err}
	}

	return parsePlanResponse(response)
}

func buildPlanPrompt(req PlanRequest) string {
	var toolsHint string
	if len(req.Tools) > 0 {
		toolsHint = "Available tools: " + strings.Join(req.Tools, ", ")
	}

	return `You are a software architect and planning specialist. Your role is to explore the codebase and design implementation plans.

=== CRITICAL: READ-ONLY MODE - NO FILE MODIFICATIONS ===
You are STRICTLY PROHIBITED from:
- Creating new files (no Write, touch, or file creation of any kind)
- Modifying existing files (no Edit operations)
- Deleting files (no rm or deletion)
- Running commands that change system state (no mkdir, rm, cp, mv, git add, git commit)

You CAN:
- Read files using read tool
- Search using grep/find tools
- Run read-only commands (ls, git status, git log, git diff without modification)

## Your Process

1. **Understand Requirements**: Focus on the user's goal and identify scope.

2. **Explore Codebase**:
   - Read CLAUDE.md or AGENTS.md if exists
   - Find existing patterns and conventions
   - Understand current architecture
   - Identify similar features for reference

3. **Design Solution**:
   - Create implementation approach
   - Consider trade-offs and architectural decisions
   - Follow existing patterns

4. **Detail the Plan**:
   - Break down into 3-8 concrete tasks
   - Identify dependencies between tasks
   - Anticipate potential challenges

## Required Output Format

Output your plan in the following JSON format:

{
  "exploration": "What you discovered about the codebase",
  "tasks": [
    {
      "subject": "Task 1 title (imperative form)",
      "description": "Detailed description of what to do"
    },
    {
      "subject": "Task 2 title",
      "description": "Detailed description"
    }
  ],
  "critical_files": [
    "path/to/file1.go",
    "path/to/file2.ts"
  ]
}

=== User Goal ===
` + req.UserGoal + `

=== Context ===
` + req.Context + `

` + toolsHint + `
`
}

func parsePlanResponse(response string) *PlanResult {
	// Extract JSON from response
	jsonStr := extractJSON(response)
	if jsonStr == "" {
		return &PlanResult{Err: ErrInvalidPlanFormat}
	}

	// Simple parsing - in production, use proper JSON parsing
	tasks := parseTasksFromJSON(jsonStr)
	exploration := extractField(jsonStr, "exploration")
	criticalFiles := extractFilesFromJSON(jsonStr)

	return &PlanResult{
		Tasks:         tasks,
		Exploration:   exploration,
		CriticalFiles: criticalFiles,
	}
}

// Errors
var (
	ErrLLMNotConfigured = &PlanError{Message: "LLM not configured"}
	ErrInvalidPlanFormat = &PlanError{Message: "Invalid plan format"}
)

type PlanError struct {
	Message string
}

func (e *PlanError) Error() string {
	return e.Message
}

// Helper functions
func extractJSON(s string) string {
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start < 0 || end <= start {
		return ""
	}
	return s[start : end+1]
}

func extractField(json, field string) string {
	// Simple extraction - in production, use JSON parsing
	pattern := `"` + field + `":`
	idx := strings.Index(json, pattern)
	if idx < 0 {
		return ""
	}
	start := idx + len(pattern)
	// Find next comma or closing brace
	end := start
	for end < len(json) {
		if json[end] == ',' || json[end] == '}' {
			break
		}
		end++
	}
	value := strings.TrimSpace(json[start:end])
	value = strings.Trim(value, `" `)
	return value
}

func extractFilesFromJSON(json string) []string {
	// Extract files array
	var files []string
	idx := strings.Index(json, `"critical_files"`)
	if idx < 0 {
		return files
	}
	// Find array start
	start := idx
	for start < len(json) && json[start] != '[' {
		start++
	}
	if start >= len(json) {
		return files
	}
	start++

	// Extract file paths
	for start < len(json) && json[start] != ']' {
		// Skip whitespace and commas
		for start < len(json) && (json[start] == ' ' || json[start] == '\n' || json[start] == ',') {
			start++
		}
		if start >= len(json) || json[start] == ']' {
			break
		}
		// Extract quoted string
		if json[start] == '"' {
			start++
			end := start
			for end < len(json) && json[end] != '"' {
				end++
			}
			if end < len(json) {
				files = append(files, json[start:end])
				start = end + 1
			}
		}
	}
	return files
}

func parseTasksFromJSON(json string) []*Task {
	var tasks []*Task
	idx := strings.Index(json, `"tasks"`)
	if idx < 0 {
		return tasks
	}

	// Find array start
	start := idx + 6
	for start < len(json) && json[start] != '[' {
		start++
	}
	if start >= len(json) {
		return tasks
	}
	start++

	depth := 1
	pos := start
	for pos < len(json) && depth > 0 {
		if json[pos] == '[' {
			depth++
		} else if json[pos] == ']' {
			depth--
		}
		pos++
	}

	arrayContent := json[start : pos-1]
	if len(arrayContent) == 0 {
		return tasks
	}

	// Split by object
	objStart := 0
	for i := 0; i < len(arrayContent); i++ {
		if arrayContent[i] == '{' {
			objStart = i
			depth = 1
			for j := i + 1; j < len(arrayContent) && depth > 0; j++ {
				if arrayContent[j] == '{' {
					depth++
				} else if arrayContent[j] == '}' {
					depth--
				}
				if depth == 0 {
					obj := arrayContent[objStart : j+1]
					task := parseTaskObject(obj)
					if task != nil {
						tasks = append(tasks, task)
					}
					i = j
				}
			}
		}
	}

	return tasks
}

func parseTaskObject(obj string) *Task {
	subject := extractField(obj, "subject")
	description := extractField(obj, "description")
	if subject == "" {
		return nil
	}
	return NewTask(subject, description)
}
