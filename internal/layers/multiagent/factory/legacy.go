// Package factory — LEGACY re-export shim for backward compatibility.
//
// Deprecated: migrated to multiagent/provision (v2.0-d).
// New code should import "github.com/devrix/devrix/internal/layers/multiagent/provision".
// This package will be removed in the re-export cleanup cycle (v2.0-e).
package factory

import "github.com/devrix/devrix/internal/layers/multiagent/provision"

// Re-export types so existing importers continue to compile.
type (
	EngineBuilder = provision.EngineBuilder
	AgentFactory  = provision.AgentFactory
)

// Re-export functions.
var (
	NewAgentFactory           = provision.NewAgentFactory
	NewAgentFactoryWithBuilder = provision.NewAgentFactoryWithBuilder
)
