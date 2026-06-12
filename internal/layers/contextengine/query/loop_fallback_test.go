package query

import (
	"context"
	stderrors "errors"
	"sync/atomic"
	"testing"

	"github.com/devrix/devrix/internal/shared/types"
)

// L5-2-9-TD03: 第一次 LLM.Call 返回 overload → loop 切换 fallback model
// → 重试成功。
func TestLoop_FallbackModel_OverloadRetry(t *testing.T) {
	primary := &fallbackLLM{
		failErr:    stderrors.New("overloaded 529"),
		modelTag:   "primary",
		finalChunk: "fallback ok",
	}
	compressorCalled := atomic.Int32{}

	compress := func(ctx context.Context, msgs []types.Message) ([]types.Message, error) {
		compressorCalled.Add(1)
		return msgs, nil
	}

	loop := &Loop{
		LLM:           primary,
		Tools:         &noopTools{},
		Compress:      compress,
		FallbackLLM:   &fallbackLLM{finalChunk: "from-fallback", modelTag: "fallback"},
		FallbackOnErr: isOverloadOr5xx,
	}

	res, err := loop.Run(context.Background(), &types.SessionContext{SessionID: "sess_td03"}, Params{
		Messages: []types.Message{{Role: types.MessageRoleUser, Content: "hi"}},
	}, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.AssistantText != "from-fallback" {
		t.Errorf("AssistantText = %q, want from-fallback", res.AssistantText)
	}
	if primary.calls.Load() != 1 {
		t.Errorf("primary.calls = %d, want 1", primary.calls.Load())
	}
}

// 非 overload 错误（如 400 bad request）不应触发 fallback。
func TestLoop_FallbackModel_NonOverload_NoFallback(t *testing.T) {
	primary := &fallbackLLM{
		failErr:    stderrors.New("400 bad request"),
		finalChunk: "should not run",
	}
	loop := &Loop{
		LLM:           primary,
		Tools:         &noopTools{},
		FallbackLLM:   &fallbackLLM{finalChunk: "fallback should not run"},
		FallbackOnErr: isOverloadOr5xx,
	}
	_, err := loop.Run(context.Background(), &types.SessionContext{SessionID: "sess_td03_2"}, Params{
		Messages: []types.Message{{Role: types.MessageRoleUser, Content: "hi"}},
	}, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// IsOverloadOr5xx 单元测试。
func TestIsOverloadOr5xx(t *testing.T) {
	cases := []struct {
		err  error
		want bool
	}{
		{stderrors.New("overloaded 529"), true},
		{stderrors.New("529 service overloaded"), true},
		{stderrors.New("HTTP 503"), true},
		{stderrors.New("internal server error 500"), true},
		{stderrors.New("bad gateway 502"), true},
		{stderrors.New("rate limit 429"), true},
		{stderrors.New("deadline exceeded"), true},
		{stderrors.New("400 bad request"), false},
		{stderrors.New("prompt too long 413"), false}, // 413 → 走 413 recovery，不走 fallback
		{stderrors.New("context_length_exceeded"), false},
		{stderrors.New("permission denied"), false},
	}
	for _, c := range cases {
		if got := isOverloadOr5xx(c.err); got != c.want {
			t.Errorf("isOverloadOr5xx(%q) = %v, want %v", c.err, got, c.want)
		}
	}
}

type fallbackLLM struct {
	calls      atomic.Int32
	failErr    error
	finalChunk string
	modelTag   string
}

func (m *fallbackLLM) Call(ctx context.Context, req LLMRequest) (<-chan LLMChunk, error) {
	m.calls.Add(1)
	if m.failErr != nil {
		return nil, m.failErr
	}
	ch := make(chan LLMChunk, 1)
	go func() {
		defer close(ch)
		ch <- LLMChunk{Content: m.finalChunk, Done: true}
	}()
	return ch, nil
}
