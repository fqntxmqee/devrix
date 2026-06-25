package sessionorchestrator

import (
	"context"

	"github.com/devrix/devrix/internal/layers/llmgateway"
)

type fakeGateway struct {
	stream  func(ctx context.Context, req *llmgateway.Request) (<-chan llmgateway.Chunk, error)
	resolve func(tier string) string
	close   func() error
}

func (f *fakeGateway) Stream(ctx context.Context, req *llmgateway.Request) (<-chan llmgateway.Chunk, error) {
	if f.stream != nil {
		return f.stream(ctx, req)
	}
	ch := make(chan llmgateway.Chunk)
	close(ch)
	return ch, nil
}

func (f *fakeGateway) ResolveTier(tier string) string {
	if f.resolve != nil {
		return f.resolve(tier)
	}
	return tier
}

func (f *fakeGateway) Close() error {
	if f.close != nil {
		return f.close()
	}
	return nil
}

type errTierResolver struct{ err error }

func (e errTierResolver) ResolveTier(string) (string, error) { return "", e.err }
