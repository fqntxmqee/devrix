// Package sessionview — LEGACY re-export shim for backward compatibility.
//
// Deprecated: migrated to multiagent/isolate (v2.0-d).
// New code should import "github.com/devrix/devrix/internal/layers/multiagent/isolate".
// This package will be removed in the re-export cleanup cycle (v2.0-e).
package sessionview

import "github.com/devrix/devrix/internal/layers/multiagent/isolate"

// Re-export types so existing importers continue to compile.
type (
	View = isolate.View
)

// Re-export functions.
var (
	Fork = isolate.Fork
)
