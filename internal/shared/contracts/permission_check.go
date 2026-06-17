package contracts

import (
	"encoding/json"
	"fmt"
)

// Decision is the per-tool permission decision result (DM-20260618-002
// devrix-surface-permission-extension). The 3-state model (Allow /
// Deny / Ask) matches clawcode Tool.ts:101-110 PermissionResult so the
// surface layer can make fine-grained decisions that the legacy
// IPermissionGate.Request boolean could not express.
//
// DSAFT: TOOL-SURFACE-1-A01-F07 (DM-20260618-002).
type Decision string

const (
	// DecisionAllow: the tool is permitted to run; surface.Execute
	// is invoked next.
	DecisionAllow Decision = "allow"

	// DecisionDeny: the tool is refused; surface.Execute is NOT
	// invoked. The turn_adapter writes a PermissionDeniedError to
	// results[i].Error and the LLM can pick a different tool.
	DecisionDeny Decision = "deny"

	// DecisionAsk: the tool needs explicit user confirmation. The
	// turn_adapter delegates to IPermissionGate.CheckPermission for
	// the policy decision. v1 simplification: Ask still becomes an
	// error envelope (PermissionAskRequiredError) — the interactive
	// prompt is left to the v2 DSL change (DM-005).
	DecisionAsk Decision = "ask"
)

// PermissionDeniedError is returned to the LLM when a tool is denied
// by the surface's CheckPermission or the gate's CheckPermission.
// Carries the spec + input + reason so the LLM can retry with a
// different tool and audit logs can attribute the refusal.
//
// DSAFT: TOOL-SURFACE-1-A01-F07.
type PermissionDeniedError struct {
	Spec   ToolSpec
	Input  json.RawMessage
	Reason string
}

func (e *PermissionDeniedError) Error() string {
	return fmt.Sprintf("permission denied: tool=%s reason=%s", e.Spec.Name, e.Reason)
}

// PermissionAskRequiredError is returned to the LLM when a tool needs
// explicit user confirmation. v1 treats this as a hard error (the turn
// terminates with a clear message); v2 (DM-005 DSL) will dispatch an
// interactive prompt and resume the turn on confirmation.
//
// DSAFT: TOOL-SURFACE-1-A01-F07.
type PermissionAskRequiredError struct {
	Spec   ToolSpec
	Input  json.RawMessage
	Reason string
}

func (e *PermissionAskRequiredError) Error() string {
	return fmt.Sprintf("permission ask required: tool=%s reason=%s", e.Spec.Name, e.Reason)
}
