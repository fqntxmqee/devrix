package testutil

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/registry"
)

// MustBuiltinRegistry fails the test when the built-in registry cannot be constructed.
func MustBuiltinRegistry(t *testing.T) *registry.BuiltinRegistry {
	t.Helper()
	reg, err := registry.NewBuiltinRegistry()
	if err != nil {
		t.Fatalf("NewBuiltinRegistry: %v", err)
	}
	return reg
}
