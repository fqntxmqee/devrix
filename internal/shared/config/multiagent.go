// Package config — LEGACY re-export shim for MultiAgent configuration types.
//
// Deprecated: MultiAgent config types migrated to multiagent/configure (v2.0-d).
// New code should import "github.com/devrix/devrix/internal/layers/multiagent/configure".
// This re-export will be removed in the cleanup cycle (v2.0-e).
package config

import "github.com/devrix/devrix/internal/layers/multiagent/configure"

// Re-export types so existing importers continue to compile.
type (
	AgentToolFileConfig  = configure.AgentToolFileConfig
	AgentToolsFileConfig = configure.AgentToolsFileConfig
	AgentToolConfig      = configure.AgentToolConfig
	AgentToolsConfig     = configure.AgentToolsConfig
	DelegateConfig       = configure.DelegateConfig
	MultiAgentFileConfig = configure.MultiAgentFileConfig
	DelegateFileConfig   = configure.DelegateFileConfig
	MultiAgentConfig     = configure.MultiAgentConfig
)

// Re-export functions.
var (
	DefaultAgentToolsConfig  = configure.DefaultAgentToolsConfig
	BuildAgentToolsConfig    = configure.BuildAgentToolsConfig
	DefaultMultiAgentConfig  = configure.DefaultMultiAgentConfig
	BuildMultiAgentConfig    = configure.BuildMultiAgentConfig
)
