// Package contextengine root — kernel re-exports (contracts.go) and facade aliases.
// Process orchestration lives in facade/; scenario code in prepare/, persist/, enforce/.
package contextengine

import "github.com/devrix/devrix/internal/layers/contextengine/facade"

// ContextEngine is the D2 domain entry point (S15→S17 glue via D7 PreparedTurn).
type ContextEngine = facade.ContextEngine

// EngineDeps holds dependencies for NewContextEngine.
type EngineDeps = facade.EngineDeps

// NewContextEngine creates the Layer 2 context engine.
var NewContextEngine = facade.NewContextEngine
