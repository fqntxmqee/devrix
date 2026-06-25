package sessionorchestrator

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/shared/types"
)

func TestCompressionSummarizer_NilGateway(t *testing.T) {
	s := NewCompressionSummarizer(CompressionSummarizerDeps{Gateway: nil})
	_, err := s.Summarize(context.Background(), "m", "p", 100)
	if err == nil {
		t.Fatal("expected error for nil gateway, got nil")
	}
}

func TestCompressionSummarizer_StreamCollection(t *testing.T) {
	gw := &fakeGateway{
		stream: func(ctx context.Context, req *llmgateway.Request) (<-chan llmgateway.Chunk, error) {
			ch := make(chan llmgateway.Chunk, 4)
			ch <- llmgateway.Chunk{Content: "This "}
			ch <- llmgateway.Chunk{Content: "is "}
			ch <- llmgateway.Chunk{Content: "a summary."}
			ch <- llmgateway.Chunk{Done: true}
			close(ch)
			return ch, nil
		},
	}
	s := NewCompressionSummarizer(CompressionSummarizerDeps{Gateway: gw, DefaultTier: "m"})
	out, err := s.Summarize(context.Background(), "m", "summarize this", 100)
	if err != nil {
		t.Fatalf("Summarize returned error: %v", err)
	}
	if out != "This is a summary." {
		t.Fatalf("expected 'This is a summary.', got %q", out)
	}
}

func TestCompressionSummarizer_StreamError(t *testing.T) {
	gw := &fakeGateway{
		stream: func(ctx context.Context, req *llmgateway.Request) (<-chan llmgateway.Chunk, error) {
			return nil, errors.New("stream failed")
		},
	}
	s := NewCompressionSummarizer(CompressionSummarizerDeps{Gateway: gw})
	_, err := s.Summarize(context.Background(), "m", "p", 100)
	if err == nil || err.Error() != "stream failed" {
		t.Fatalf("expected stream failed error, got %v", err)
	}
}

func TestCompressionSummarizer_Timeout(t *testing.T) {
	gw := &fakeGateway{
		stream: func(ctx context.Context, req *llmgateway.Request) (<-chan llmgateway.Chunk, error) {
			ch := make(chan llmgateway.Chunk)
			go func() {
				defer close(ch)
				for {
					select {
					case <-ctx.Done():
						return
					case ch <- llmgateway.Chunk{Content: "x"}:
						time.Sleep(50 * time.Millisecond)
					}
				}
			}()
			return ch, nil
		},
	}
	s := NewCompressionSummarizer(CompressionSummarizerDeps{
		Gateway: gw,
		Timeout: 10 * time.Millisecond,
	})
	_, err := s.Summarize(context.Background(), "m", "p", 100)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}

func TestCompressionSummarizer_TierResolverError(t *testing.T) {
	gw := &fakeGateway{}
	res := &errTierResolver{err: errors.New("tier boom")}
	s := NewCompressionSummarizer(CompressionSummarizerDeps{
		Gateway:      gw,
		TierResolver: res,
	})
	_, err := s.Summarize(context.Background(), "m", "p", 100)
	if err == nil {
		t.Fatal("expected tier resolve error, got nil")
	}
}

func TestCompressionSummarizer_EmptyStream(t *testing.T) {
	gw := &fakeGateway{
		stream: func(ctx context.Context, req *llmgateway.Request) (<-chan llmgateway.Chunk, error) {
			ch := make(chan llmgateway.Chunk, 1)
			ch <- llmgateway.Chunk{Done: true}
			close(ch)
			return ch, nil
		},
	}
	s := NewCompressionSummarizer(CompressionSummarizerDeps{Gateway: gw})
	out, err := s.Summarize(context.Background(), "m", "p", 100)
	if err != nil {
		t.Fatalf("Summarize returned error: %v", err)
	}
	if out != "" {
		t.Fatalf("expected empty string, got %q", out)
	}
}

func TestCompressionSummarizer_PassesPromptAsUserMessage(t *testing.T) {
	var captured *llmgateway.Request
	gw := &fakeGateway{
		stream: func(ctx context.Context, req *llmgateway.Request) (<-chan llmgateway.Chunk, error) {
			captured = req
			ch := make(chan llmgateway.Chunk, 1)
			ch <- llmgateway.Chunk{Done: true}
			close(ch)
			return ch, nil
		},
	}
	s := NewCompressionSummarizer(CompressionSummarizerDeps{Gateway: gw})
	if _, err := s.Summarize(context.Background(), "m", "please summarize X", 100); err != nil {
		t.Fatal(err)
	}
	if captured == nil {
		t.Fatal("expected gateway to receive a request")
	}
	if len(captured.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(captured.Messages))
	}
	if captured.Messages[0].Role != types.MessageRoleUser {
		t.Errorf("expected user role, got %q", captured.Messages[0].Role)
	}
	if !strings.Contains(captured.Messages[0].Content, "please summarize X") {
		t.Errorf("expected prompt to be passed through, got %q", captured.Messages[0].Content)
	}
}
