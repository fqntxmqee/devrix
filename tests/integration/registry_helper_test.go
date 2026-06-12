//go:build integration

package integration

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/registry"
)

// mustBuiltinRegistry is a small wrapper that fails the test if the
// built-in registry cannot be constructed. Replaces direct calls to
// registry.NewBuiltinRegistry() now that the constructor returns
// (*registry.BuiltinRegistry, error).
func mustBuiltinRegistry(t *testing.T) *registry.BuiltinRegistry {
	t.Helper()
	reg, err := registry.NewBuiltinRegistry()
	if err != nil {
		t.Fatalf("NewBuiltinRegistry: %v", err)
	}
	return reg
}
