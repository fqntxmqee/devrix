package tasks

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/observability/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/tracer"
)

// PlanAgent generates task plans via LLM.
// Inspired by Claude Code's PlanAgent - READ-ONLY exploration and planning.
type PlanAgent struct {
	llm       LLMCompleter
	obsBridge *observability.Bridge
}

// PlanAgentReadOnlyTools is the whitelist of tools that PlanAgent allows
// the LLM to invoke during read-only exploration. Tools NOT in this list
// are not part of the read-only contract.
//
// Contract (enforced by D7-S5-T02):
//   - non-empty
//   - disjoint from PlanAgentForbiddenTools
//   - injected into buildPlanPrompt's "Available tools" hint
var PlanAgentReadOnlyTools = []string{
	"read",       // file read
	"grep",       // code search
	"find",       // file finder
	"ls",         // directory listing
	"git_status", // git status (read-only)
	"git_log",    // git history (read-only)
	"git_diff",   // git diff (read-only)
}

// PlanAgentForbiddenTools is the blacklist of tools that MUST NOT appear
// in the read-only whitelist. This slice is contract-only: it does not
// participate in runtime logic, but its presence enables the test point
// D7-S5-T02 to assert "these tools must never be in the whitelist".
var PlanAgentForbiddenTools = []string{
	"write",  // file write
	"edit",   // file edit
	"bash",   // shell exec
	"delete", // file delete
	"mkdir",  // create dir
	"rm",     // remove
	"mv",     // rename
	"cp",     // copy (may have side effects)
}

// AllowedTools returns the read-only tool whitelist for this PlanAgent.
// Currently returns the package-level constant; the method signature is
// stable so future per-session injection is a no-op for callers.
func (a *PlanAgent) AllowedTools() []string {
	return PlanAgentReadOnlyTools
}

// IsReadOnlyTool reports whether the named tool is in the read-only
// whitelist. nil-receiver safe: returns false instead of panicking.
func (a *PlanAgent) IsReadOnlyTool(name string) bool {
	if a == nil {
		return false
	}
	for _, t := range PlanAgentReadOnlyTools {
		if t == name {
			return true
		}
	}
	return false
}

// LLMCompleter interface for LLM calls.
type LLMCompleter interface {
	Complete(ctx context.Context, prompt string) (string, error)
}

// NewPlanAgent creates a new PlanAgent.
func NewPlanAgent(llm LLMCompleter, obsBridge *observability.Bridge) *PlanAgent {
	return &PlanAgent{llm: llm, obsBridge: obsBridge}
}

// startSpan creates a child span for plan operations.
func (a *PlanAgent) startSpan(ctx context.Context, operation string, kind tracer.SpanKind, attrs ...tracer.Attribute) (context.Context, tracer.Span) {
	if a.obsBridge == nil || !a.obsBridge.IsEnabled() {
		return ctx, nil
	}
	opts := []tracer.SpanStartOption{
		tracer.WithSpanKind(kind),
		tracer.WithSpanAttributes(telemetry.SpanAttrs(operation, attrs...)...),
	}
	if parentSC := tracer.SpanContextFromContext(ctx); parentSC != nil {
		opts = append(opts, tracer.WithParent(*parentSC))
	}
	return a.obsBridge.Tracer().Start(ctx, operation, opts...)
}

// planStartSpan is a simpler helper for PlanAgent methods that hold the bridge directly.
func planStartSpan(ctx context.Context, obsBridge *observability.Bridge, operation string, kind tracer.SpanKind, attrs ...tracer.Attribute) (context.Context, tracer.Span) {
	if obsBridge == nil || !obsBridge.IsEnabled() {
		return ctx, nil
	}
	opts := []tracer.SpanStartOption{
		tracer.WithSpanKind(kind),
		tracer.WithSpanAttributes(telemetry.SpanAttrs(operation, attrs...)...),
	}
	if parentSC := tracer.SpanContextFromContext(ctx); parentSC != nil {
		opts = append(opts, tracer.WithParent(*parentSC))
	}
	return obsBridge.Tracer().Start(ctx, operation, opts...)
}

// PlanRequest is the input for plan generation.
type PlanRequest struct {
	UserGoal string   // 用户目标描述
	WorkDir  string   // 工作目录
	Context  string   // 额外上下文（如 CLAUDE.md 内容）
	Tools    []string // 可用工具列表
}

// PlanResult is the output from plan generation.
type PlanResult struct {
	Tasks         []*Task  // 计划的任务列表
	Exploration   string   // 探索发现
	CriticalFiles []string // 关键文件列表
	Err           error
}

// Plan generates a plan for the given goal.
// This is READ-ONLY - only explores and plans, never modifies files.
func (a *PlanAgent) Plan(ctx context.Context, req PlanRequest) *PlanResult {
	start := time.Now()
	ctx, planSpan := a.startSpan(ctx, telemetry.OpD2_S8_Task_Plan_Generate, tracer.SpanKindInternal,
		tracer.Attribute{Key: "task.user_goal", Value: truncateStr(req.UserGoal, 200)},
		tracer.Attribute{Key: "task.tool_count", Value: fmt.Sprintf("%d", len(req.Tools))},
	)

	if a.llm == nil {
		if planSpan != nil {
			planSpan.End()
		}
		return &PlanResult{Err: ErrLLMNotConfigured}
	}

	prompt := buildPlanPrompt(req)
	response, err := a.llm.Complete(ctx, prompt)
	if err != nil {
		if planSpan != nil {
			planSpan.RecordError(err)
			planSpan.End()
		}
		return &PlanResult{Err: err}
	}

	result := parsePlanResponse(response)
	if planSpan != nil {
		planSpan.SetAttributes(
			tracer.Attribute{Key: "task.result_count", Value: fmt.Sprintf("%d", len(result.Tasks))},
			tracer.Attribute{Key: "task.critical_files_count", Value: fmt.Sprintf("%d", len(result.CriticalFiles))},
			tracer.Attribute{Key: "task.plan_duration_ms", Value: fmt.Sprintf("%d", time.Since(start).Milliseconds())},
		)
		if result.Err != nil {
			planSpan.RecordError(result.Err)
		}
		planSpan.End()
	}

	return result
}

// truncateStr truncates a string to maxLen characters.
func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func buildPlanPrompt(req PlanRequest) string {
	// Always include the PlanAgent read-only whitelist (D7-S5-T02 contract).
	// Merge with caller's req.Tools: whitelist first, then caller's additions
	// de-duplicated. Caller-supplied write tools are kept in the merged list
	// for transparency but the whitelist prefix signals the read-only intent.
	allowed := PlanAgentReadOnlyTools
	merged := make([]string, 0, len(allowed)+len(req.Tools))
	merged = append(merged, allowed...)
	for _, t := range req.Tools {
		dup := false
		for _, m := range merged {
			if m == t {
				dup = true
				break
			}
		}
		if !dup {
			merged = append(merged, t)
		}
	}
	toolsHint := "Available tools (read-only whitelist + extras): " + strings.Join(merged, ", ")

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
	ErrLLMNotConfigured  = &PlanError{Message: "LLM not configured"}
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
