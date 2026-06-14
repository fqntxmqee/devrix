// Package multiagent — LEGACY re-export shim for domain kernel types.
//
// Deprecated: kernel types migrated to multiagent/kernel (v2.0-d).
// New code should import "github.com/devrix/devrix/internal/layers/multiagent/kernel".
// This re-export will be removed in the cleanup cycle (v2.0-e).
package multiagent

import "github.com/devrix/devrix/internal/layers/multiagent/kernel"

// Re-export types so existing importers continue to compile.
type (
	AgentState           = kernel.AgentState
	CollaborationMode    = kernel.CollaborationMode
	AgentConfig          = kernel.AgentConfig
	AgentResult          = kernel.AgentResult
	PermissionGate       = kernel.PermissionGate
	AgentDeps            = kernel.AgentDeps
	AgentObserver        = kernel.AgentObserver
	AgentObserverChain   = kernel.AgentObserverChain
	AgentEvent           = kernel.AgentEvent
	IAgentFactory        = kernel.IAgentFactory
	Agent                = kernel.Agent
)

// Re-export constants.
const MetaToolCallID = kernel.MetaToolCallID

// Re-export AgentState values.
const (
	AgentStateCreated           = kernel.AgentStateCreated
	AgentStateRunning           = kernel.AgentStateRunning
	AgentStateIterating         = kernel.AgentStateIterating
	AgentStateWaitingPermission = kernel.AgentStateWaitingPermission
	AgentStateTerminated        = kernel.AgentStateTerminated
)

// Re-export CollaborationMode values.
const (
	ModeChainOfThought      = kernel.ModeChainOfThought
	ModeIterativeRefinement = kernel.ModeIterativeRefinement
	ModeDefault             = kernel.ModeDefault
)

// Re-export functions.
var (
	NewAgentObserverChain = kernel.NewAgentObserverChain
)
