package query

import (
	"context"
	stderrors "errors"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/devrix/devrix/internal/shared/errors"
	"github.com/devrix/devrix/internal/shared/types"
)

// errLLM 是个一次返回错误（413 风格），第二次成功的 LLM stub。
type errLLM struct {
	calls atomic.Int32
	err   error
}

func (m *errLLM) Call(ctx context.Context, req LLMRequest) (<-chan LLMChunk, error) {
	n := m.calls.Add(1)
	if n == 1 {
		return nil, m.err
	}
	ch := make(chan LLMChunk, 1)
	go func() {
		defer close(ch)
		ch <- LLMChunk{Content: "ok", Done: true}
	}()
	return ch, nil
}

// D2-S11-A01-TD01: 第一次 LLM.Call 返回 413 → loop 触发 CompressFn 一次
// → 重试成功，且不返回错误。
func TestLoop_ContextLengthRecovery(t *testing.T) {
	compressorCalled := atomic.Int32{}
	stubLLM := &errLLM{err: stderrors.New("prompt too long: 413")}

	compress := func(ctx context.Context, msgs []types.Message) ([]types.Message, error) {
		compressorCalled.Add(1)
		return msgs[:1], nil
	}

	loop := &Loop{
		LLM:      stubLLM,
		Tools:    &noopTools{},
		Compress: compress,
	}
	sc := &types.SessionContext{SessionID: "sess_td01"}
	params := Params{
		Messages: []types.Message{
			{Role: types.MessageRoleUser, Content: "hello"},
		},
		MaxTurns: 3,
	}
	res, err := loop.Run(context.Background(), sc, params, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.AssistantText != "ok" {
		t.Errorf("AssistantText = %q, want %q", res.AssistantText, "ok")
	}
	if stubLLM.calls.Load() != 2 {
		t.Errorf("LLM.Call invoked %d times, want 2 (1 fail + 1 retry)", stubLLM.calls.Load())
	}
	// 期望: pre-call compress (1) + 413 recovery compress (1) = 2 次。
	// Loop.Run 每轮 LLM.Call 前都会先调 Compress（如果 Compress != nil），
	// recovery 路径在 413 时再调一次。
	if compressorCalled.Load() != 2 {
		t.Errorf("CompressFn invoked %d times, want 2 (1 pre-call + 1 recovery)", compressorCalled.Load())
	}
}

// D2-S11-A01-TD01 副: 非超长错误不应触发 recovery；错误原样透传。
func TestLoop_NonContextError_NoRecovery(t *testing.T) {
	stubLLM := &errLLM{err: stderrors.New("rate limit 429")}

	compressorCalled := atomic.Int32{}
	compress := func(ctx context.Context, msgs []types.Message) ([]types.Message, error) {
		// pre-call compress 仍然会调（与是否 413 无关）。但 recovery
		// 不应触发，所以总调用次数 ≤ 1。
		compressorCalled.Add(1)
		return msgs, nil
	}

	loop := &Loop{
		LLM:      stubLLM,
		Tools:    &noopTools{},
		Compress: compress,
	}
	_, err := loop.Run(context.Background(), &types.SessionContext{SessionID: "x"}, Params{
		Messages: []types.Message{{Role: types.MessageRoleUser, Content: "hi"}},
	}, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "rate limit") {
		t.Errorf("error = %v, want rate limit to surface", err)
	}
	// 关键断言：recovery 路径在非 413 错误时**没有**触发第二次 Compress。
	if compressorCalled.Load() != 1 {
		t.Errorf("CompressFn invoked %d times, want 1 (pre-call only, no recovery)", compressorCalled.Load())
	}
}

// IsContextLengthError (TD-QL-01) 识别 LLM 拒绝"prompt too long"。
func TestIsContextLengthError(t *testing.T) {
	if !IsContextLengthError(errors.NewContextExceededError()) {
		t.Fatal("SentinelError CTX_EXCEEDED_4001 must be detected")
	}
	if !IsContextLengthError(stderrors.New("prompt too long: 413")) {
		t.Fatal("plain 413 error must be detected")
	}
	if !IsContextLengthError(stderrors.New("context_length_exceeded")) {
		t.Fatal("openai-style 'context_length_exceeded' must be detected")
	}
	if IsContextLengthError(stderrors.New("rate limit")) {
		t.Fatal("'rate limit' must not be classified as context length error")
	}
	if IsContextLengthError(stderrors.New("overloaded 529")) {
		t.Fatal("overload (529) must not be classified as context length error")
	}
	if IsContextLengthError(stderrors.New("internal server error 500")) {
		t.Fatal("500 must not be classified as context length error")
	}
}

type noopTools struct{}

func (n *noopTools) Execute(ctx context.Context, call ToolCall) (string, string, error) {
	return "", "", nil
}
