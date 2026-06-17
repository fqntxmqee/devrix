package anthropic_test

import (
	"errors"
	"testing"

	"github.com/devrix/devrix/internal/layers/anthropic"
)

// T: anthropic package stub — every method must return ErrNotImplemented.
// This pins the v1.0 contract so v1.1 can land real implementations
// without breaking callers that already import the package.
func TestClient_ListTools_NotImplemented(t *testing.T) {
	c := anthropic.NewClient("test-key")
	out, err := c.ListTools()
	if !errors.Is(err, anthropic.ErrNotImplemented) {
		t.Errorf("err = %v, want ErrNotImplemented", err)
	}
	if out != nil {
		t.Errorf("out = %v, want nil", out)
	}
}

func TestClient_ToolSearch_NotImplemented(t *testing.T) {
	c := anthropic.NewClient("test-key")
	out, err := c.ToolSearch("delegate_research")
	if !errors.Is(err, anthropic.ErrNotImplemented) {
		t.Errorf("err = %v, want ErrNotImplemented", err)
	}
	if out != nil {
		t.Errorf("out = %v, want nil", out)
	}
}

func TestErrNotImplemented_Message(t *testing.T) {
	if anthropic.ErrNotImplemented.Error() == "" {
		t.Error("ErrNotImplemented.Error() empty")
	}
}
