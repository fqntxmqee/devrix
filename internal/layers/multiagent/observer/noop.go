// Package observer — LEGACY re-export shim for backward compatibility.
//
// Deprecated: migrated to multiagent/kernel (v2.0-d).
// New code should import "github.com/devrix/devrix/internal/layers/multiagent/kernel".
// This package will be removed in the re-export cleanup cycle (v2.0-e).
package observer

import "github.com/devrix/devrix/internal/layers/multiagent/kernel"

// Re-export types so existing importers continue to compile.
type (
	NoOpAgentObserver = kernel.NoOpAgentObserver
)
