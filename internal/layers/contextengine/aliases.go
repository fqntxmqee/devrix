// Package contextengine root — kernel re-exports (contracts.go) and
// legacy/ aliases. Process orchestration has been moved to legacy/ (P5
// retirement); the production hot path is D7 → D2 turn adapters, not
// legacy.Process(). New callers must go through the D7 SessionOrchestrator
// (bootstrap/wire_coordinator.go) or the turn adapter (D7-S2-A06).
package contextengine

import "github.com/devrix/devrix/internal/layers/contextengine/legacy"

// ContextEngine is the D2 domain entry point (S15→S17 glue via D7 PreparedTurn).
//
// Deprecated: prefer D7 SessionOrchestrator (or the turn adapter for
// worker forking). This type alias is kept during the P5 deprecation
// window so existing callers keep compiling; new code MUST NOT use it.
type ContextEngine = legacy.ContextEngine

// EngineDeps holds dependencies for NewContextEngine.
type EngineDeps = legacy.EngineDeps

// NewContextEngine creates the Layer 2 context engine.
//
// Deprecated: same as ContextEngine. Kept for the deprecation window.
var NewContextEngine = legacy.NewContextEngine
