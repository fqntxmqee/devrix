package tasks

import (
	"context"
	"strings"
)

// VerificationAgent performs adversarial verification of implementation.
// Inspired by Claude Code's VerificationAgent - tries to BREAK the implementation.
type VerificationAgent struct {
	llm LLMCompleter
}

// NewVerificationAgent creates a new VerificationAgent.
func NewVerificationAgent(llm LLMCompleter) *VerificationAgent {
	return &VerificationAgent{llm: llm}
}

// VerifyRequest is the input for verification.
type VerifyRequest struct {
	UserGoal      string   // 原始用户目标
	TasksDone    []*Task  // 已完成的任务
	FilesChanged []string // 修改的文件列表
	Approach     string   // 采取的实现方案
	Tools        []string // 可用工具
}

// VerifyResult is the output from verification.
type VerifyResult struct {
	Checks    []CheckResult // 验证检查结果
	Verdict   Verdict      // 最终结论
	Summary   string       // 总结
	Commands  []string     // 执行的命令列表
	Err       error
}

// CheckResult represents a single verification check.
type CheckResult struct {
	Name        string // 检查名称
	Description string // 检查描述
	Command     string // 执行的命令
	Output      string // 命令输出
	Expected    string // 期望结果
	Actual      string // 实际结果
	Passed      bool   // 是否通过
}

// Verdict is the final verification verdict.
type Verdict string

const (
	VerdictPass    Verdict = "PASS"
	VerdictFail    Verdict = "FAIL"
	VerdictPartial Verdict = "PARTIAL"
)

// Verify performs adversarial verification.
// This tries to BREAK the implementation, not just confirm it works.
func (a *VerificationAgent) Verify(ctx context.Context, req VerifyRequest) *VerifyResult {
	if a.llm == nil {
		return &VerifyResult{Err: ErrLLMNotConfigured}
	}

	prompt := buildVerificationPrompt(req)
	response, err := a.llm.Complete(ctx, prompt)
	if err != nil {
		return &VerifyResult{Err: err}
	}

	return parseVerificationResponse(response)
}

func buildVerificationPrompt(req VerifyRequest) string {
	var toolsHint string
	if len(req.Tools) > 0 {
		toolsHint = "Available tools: " + strings.Join(req.Tools, ", ")
	}

	var tasksStr string
	for _, t := range req.TasksDone {
		tasksStr += "- " + t.Subject + ": " + t.Description + "\n"
	}

	var filesStr string
	for _, f := range req.FilesChanged {
		filesStr += "- " + f + "\n"
	}

	return `You are a verification specialist. Your job is NOT to confirm the implementation works — it's to try to BREAK it.

You have two documented failure patterns:
1. **Verification avoidance**: when faced with a check, you find reasons not to run it — you read code, narrate what you would test, write "PASS", and move on.
2. **80% seduction**: you see a polished UI or a passing test suite and feel inclined to pass it, not noticing half the buttons do nothing or the backend crashes on bad input.

=== CRITICAL: DO NOT MODIFY THE PROJECT ===
You are STRICTLY PROHIBITED from:
- Creating, modifying, or deleting files IN THE PROJECT DIRECTORY
- Installing dependencies
- Running git write operations (add, commit, push)

You MAY write ephemeral test scripts to /tmp or $TMPDIR.

## What You Receive

- **Original goal**: What the user asked for
- **Tasks done**: What was implemented
- **Files changed**: What files were modified
- **Approach**: How it was implemented

## Your Verification Strategy

Adapt your strategy based on what was changed:

**Frontend changes**: Start dev server → use browser automation if available → check page subresources → run frontend tests

**Backend/API changes**: Start server → curl/fetch endpoints → verify response shapes → test error handling → check edge cases

**CLI/script changes**: Run with representative inputs → verify stdout/stderr/exit codes → test edge inputs (empty, malformed, boundary)

**Bug fixes**: Reproduce the original bug → verify fix → run regression tests → check related functionality

**Refactoring**: Existing test suite MUST pass unchanged → diff the public API surface → spot-check behavior

## Required Checks

You MUST include these adversarial probes:

1. **Boundary values**: 0, -1, empty string, very long strings, unicode, MAX_INT
2. **Idempotency**: same mutating request twice — duplicate created? error? correct no-op?
3. **Orphan operations**: delete/reference IDs that don't exist

## Required Output Format

For EACH check, you MUST include the command you ran:

{
  "checks": [
    {
      "name": "Check: what you're verifying",
      "description": "Why this check matters",
      "command": "exact command executed",
      "output": "actual terminal output",
      "expected": "what you expected",
      "actual": "what you observed",
      "passed": true or false
    }
  ],
  "verdict": "PASS or FAIL or PARTIAL",
  "summary": "Brief summary of what was verified and what failed",
  "commands": ["list of all commands you ran"]
}

=== Original Goal ===
` + req.UserGoal + `

=== Tasks Done ===
` + tasksStr + `

=== Files Changed ===
` + filesStr + `

=== Approach ===
` + req.Approach + `

` + toolsHint + `

End with exactly: VERDICT: PASS or VERDICT: FAIL or VERDICT: PARTIAL
`
}

func parseVerificationResponse(response string) *VerifyResult {
	// Extract verdict
	verdict := VerdictPartial
	if strings.Contains(response, "VERDICT: PASS") {
		verdict = VerdictPass
	} else if strings.Contains(response, "VERDICT: FAIL") {
		verdict = VerdictFail
	}

	// Extract JSON if present
	jsonStr := extractJSON(response)
	checks := parseChecksFromJSON(jsonStr)
	summary := extractField(jsonStr, "summary")
	commands := extractCommandsFromJSON(jsonStr)

	return &VerifyResult{
		Checks:   checks,
		Verdict: verdict,
		Summary: summary,
		Commands: commands,
	}
}

func parseChecksFromJSON(json string) []CheckResult {
	var checks []CheckResult
	if json == "" {
		return checks
	}

	idx := strings.Index(json, `"checks"`)
	if idx < 0 {
		return checks
	}

	// Simple parsing - in production use proper JSON parsing
	// For now, extract key information
	check := CheckResult{}
	if strings.Contains(json, `"passed":true`) || strings.Contains(json, `"passed" :true`) {
		check.Passed = true
	}

	checks = append(checks, check)
	return checks
}

func extractCommandsFromJSON(json string) []string {
	var commands []string
	idx := strings.Index(json, `"commands"`)
	if idx < 0 {
		return commands
	}
	// Extract array content
	start := idx + 10
	for start < len(json) && json[start] != '[' {
		start++
	}
	if start >= len(json) {
		return commands
	}
	start++

	for start < len(json) && json[start] != ']' {
		if json[start] == '"' {
			start++
			end := start
			for end < len(json) && json[end] != '"' {
				end++
			}
			if end < len(json) {
				commands = append(commands, json[start:end])
				start = end + 1
			}
		}
		start++
	}

	return commands
}
