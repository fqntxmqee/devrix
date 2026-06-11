package adapter_test

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/llmgateway/adapter"
)

type stubAdapter struct {
	provider string
}

func (s stubAdapter) Stream(ctx context.Context, req *llmgateway.Request) (<-chan *llmgateway.AdapterChunk, error) {
	ch := make(chan *llmgateway.AdapterChunk)
	close(ch)
	return ch, nil
}

func (s stubAdapter) Provider() string { return s.provider }

func TestRegistry_should_register_and_get_adapter(t *testing.T) {
	reg := adapter.NewRegistry()
	if err := reg.Register(stubAdapter{provider: "deepseek"}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	got, err := reg.Get("deepseek")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Provider() != "deepseek" {
		t.Errorf("provider: got %s", got.Provider())
	}
}

func TestRegistry_should_error_on_missing_provider(t *testing.T) {
	reg := adapter.NewRegistry()
	_, err := reg.Get("minimax")
	if err == nil {
		t.Fatal("expected error")
	}
}


