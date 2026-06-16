// Package enforce — D2-S18 EnforceExecutionPolicy orchestrator.
//
// PolicyOrchestrator coordinates the 4 A-level activities:
//
//	A01 CheckPermission  → permission gate (Ask/Allow/Deny)
//	A02 FilterTools      → tool visibility filtering (Plan Mode + Agent Role)
//	A03 SandboxExecution → tool execution isolation + sandbox
//	A04 RegisterTools    → tool registration + discovery (Builtin + Custom)
package enforce

import (
	"context"

	"github.com/devrix/devrix/internal/shared/types"
)

// ToolCall describes a pending tool invocation.
type ToolCall struct {
	ID       string
	Name     string
	Input    string
	WorkDir  string
	RiskLevel types.RiskLevel
}

// ToolResult describes the outcome of a tool invocation.
type ToolResult struct {
	CallID string
	Output string
	Error  string
}

// PermissionGate checks whether a tool call is allowed.
//
// DSAFT: D2-S18-A01 (CheckPermission)
type PermissionGate interface {
	Check(call ToolCall) (allowed bool, denyReason string)
}

// ToolFilter filters the visible tool surface based on execution context.
//
// DSAFT: D2-S18-A02 (FilterTools)
type ToolFilter interface {
	Filter(allTools []string, mode string) []string
}

// ToolSandbox isolates tool execution within a workdir.
//
// DSAFT: D2-S18-A03 (SandboxExecution)
type ToolSandbox interface {
	Execute(ctx context.Context, call ToolCall) ToolResult
}

// PolicyDeps bundles dependencies for the policy enforcement orchestration.
type PolicyDeps struct {
	PermissionGate PermissionGate
	ToolFilter     ToolFilter
	ToolSandbox    ToolSandbox
}

// PolicyOrchestrator orchestrates the D2-S18 EnforceExecutionPolicy scenario.
//
// DSAFT: D2-S18 (EnforceExecutionPolicy)
type PolicyOrchestrator struct {
	deps PolicyDeps
}

// NewPolicyOrchestrator creates a policy orchestrator.
func NewPolicyOrchestrator(deps PolicyDeps) *PolicyOrchestrator {
	return &PolicyOrchestrator{deps: deps}
}

// EnforceToolRound checks permissions, filters, and executes a tool round.
func (o *PolicyOrchestrator) EnforceToolRound(ctx context.Context, calls []ToolCall) []ToolResult {
	results := make([]ToolResult, 0, len(calls))
	for _, call := range calls {
		if o.deps.PermissionGate != nil {
			allowed, _ := o.deps.PermissionGate.Check(call)
			if !allowed {
				results = append(results, ToolResult{
					CallID: call.ID,
					Error:  "permission denied",
				})
				continue
			}
		}
		if o.deps.ToolSandbox != nil {
			results = append(results, o.deps.ToolSandbox.Execute(ctx, call))
		}
	}
	return results
}
