// Package tool — LEGACY re-export shim for backward compatibility.
//
// Deprecated: migrated to multiagent/external (v2.0-d).
// New code should import "github.com/devrix/devrix/internal/layers/multiagent/external".
// This package will be removed in the re-export cleanup cycle (v2.0-e).
package tool

import "github.com/devrix/devrix/internal/layers/multiagent/external"

// Re-export types so existing importers continue to compile.
type (
	Info      = external.Info
	Request   = external.Request
	Event     = external.Event
	AgentTool = external.AgentTool
	CLIConfig = external.CLIConfig
	CursorConfig = external.CursorConfig
	Registry  = external.Registry
)

// Re-export functions.
var (
	NewRegistry        = external.NewRegistry
	NewCLIAgentTool    = external.NewCLIAgentTool
	NewCursorAgentTool = external.NewCursorAgentTool
)
