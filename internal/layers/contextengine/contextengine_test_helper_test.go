package contextengine_test

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/registry"
)

func mustBuiltinRegistry(t *testing.T) *registry.BuiltinRegistry {
	reg, err := registry.NewBuiltinRegistry()
	if err != nil {
		t.Fatal(err)
	}
	return reg
}
