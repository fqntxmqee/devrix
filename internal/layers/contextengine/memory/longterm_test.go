package memory_test

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/memory"
	"github.com/devrix/devrix/internal/shared/errors"
)

// Covers: L5-CTX-10
func TestLongTermMemory_should_return_not_implemented(t *testing.T) {
	lt := memory.NewLongTermMemory()
	err := lt.Recall("query")
	if err == nil {
		t.Fatal("expected error")
	}
	if errors.ErrorCode(err) != errors.CodeMemoryNotImplemented {
		t.Errorf("unexpected code: %v", err)
	}
}
