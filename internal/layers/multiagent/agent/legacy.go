// Package agent — LEGACY re-export shim for backward compatibility.
//
// Deprecated: migrated to multiagent/run (v2.0-d).
// New code should import "github.com/devrix/devrix/internal/layers/multiagent/run".
// This package will be removed in the re-export cleanup cycle (v2.0-e).
package agent

import "github.com/devrix/devrix/internal/layers/multiagent/run"

// Re-export types so existing importers continue to compile.
type (
	Impl        = run.Impl
	StubEngine  = run.StubEngine
)

// Re-export functions.
var (
	New              = run.New
	NewWorkerEngine  = run.NewWorkerEngine
)
