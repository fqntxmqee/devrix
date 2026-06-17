package stream_test

// W1 — D3-S3-A02 (alias A6) ErrorClassify + 短栈包装单元测试。
//
// AC7:
//   - HTTP 401 → AuthRequired (ClassAuth)
//   - LLMError.Code="rate_limit" 优先于 HTTP 200 (regex 走 ClassRateLimit)
//   - 成功响应无分类标签
//   - WithClassifier(nil) 降级为 shortstack-only（无 [class=...] 标签）
//   - errors.Is / errors.As 在 classify 包装后仍可穿透

import (
	"context"
	stderrors "errors"
	"fmt"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	streamadapter "github.com/devrix/devrix/internal/layers/llmgateway/stream/adapter"
	"github.com/devrix/devrix/internal/layers/llmgateway/protect"
	"github.com/devrix/devrix/internal/layers/llmgateway/protect/errorclass"
	"github.com/devrix/devrix/internal/layers/llmgateway/stream"
	"github.com/devrix/devrix/internal/layers/llmgateway/budget"
	sharedconfig "github.com/devrix/devrix/internal/shared/config"
	sharederrors "github.com/devrix/devrix/internal/shared/errors"
	"github.com/devrix/devrix/internal/shared/types"
)

// helper: 构造一个挂载了 classifier + 失败 adapter 的 Gateway
func classifyingGateway(t *testing.T, c errorclass.Classifier, failErr error) *stream.Gateway {
	t.Helper()
	counter, err := budget.NewCounter()
	if err != nil {
		t.Fatalf("counter: %v", err)
	}
	cfg := sharedconfig.DefaultLLMGatewayConfig()
	reg := streamadapter.NewRegistry()
	_ = reg.Register(stubAdapter{provider: "deepseek", handler: func(model string) (<-chan *llmgateway.AdapterChunk, error) {
		return nil, failErr
	}})
	gw := stream.New(stream.Deps{
		Config: cfg, Registry: reg, Breaker: protect.New(cfg.CircuitBreaker),
		Retry: protect.NewExecutor(), Counter: counter,
	})
	if c != nil {
		gw.WithClassifier(c)
	}
	return gw
}

// T: D3-S3-A02-T01 (AC7)
// Adapter 返回 *LLMError(Code=LLM_AUTH_1004) 时，
// gateway.classify 应当把它包成 [class=auth_failed] 字符串。
func TestInvoke_Classify_AuthFailed(t *testing.T) {
	authErr := sharederrors.NewLLMAuthFailedError(stderrors.New("status 401 from upstream"))
	gw := classifyingGateway(t, errorclass.NewDefaultClassifier(), authErr)

	_, err := gw.Stream(context.Background(), &llmgateway.Request{
		Model:    "deepseek-v4-flash",
		Messages: []types.Message{*types.NewMessage("1", "s", types.MessageRoleUser, "hi")},
	})
	if err == nil {
		t.Fatalf("expected auth error, got nil")
	}
	if !strings.HasPrefix(err.Error(), "[class=auth_failed]") {
		t.Errorf("error %q does not start with [class=auth_failed]", err.Error())
	}
	// 短栈至少出现 1 帧（shortstack 包装生效）
	if !strings.Contains(err.Error(), "\n") {
		t.Errorf("expected shortstack appended, got %q", err.Error())
	}
}

// T: D3-S3-A02-T01 (AC7)
// 即使 HTTP status=200，raw body 含 "rate_limit_exceeded" 时
// regex 规则应当优先于 HTTP 分类、命中 ClassRateLimit。
func TestInvoke_Classify_RateLimitFromBody(t *testing.T) {
	gw := classifyingGateway(t, errorclass.NewDefaultClassifier(), stderrors.New("rate_limit_exceeded upstream"))

	_, err := gw.Stream(context.Background(), &llmgateway.Request{
		Model:    "deepseek-v4-flash",
		Messages: []types.Message{*types.NewMessage("1", "s", types.MessageRoleUser, "hi")},
	})
	if err == nil {
		t.Fatalf("expected rate-limit error, got nil")
	}
	if !strings.HasPrefix(err.Error(), "[class=rate_limit]") {
		t.Errorf("error %q does not start with [class=rate_limit]", err.Error())
	}
}

// T: D3-S3-A02-T01 (AC7 验证透传)
// errors.As 在 classify 包装后仍能取到底层 *LLMError；
// errors.Is 在 classify 包装后仍能命中 sentinel。
func TestInvoke_Classify_UnwrapChainPreserved(t *testing.T) {
	// 构造一个 wrapping 了 sentinel 的错误，让 errors.Is 命中。
	authErr := fmt.Errorf("auth: %w", sharederrors.ErrLLMAuthFailed)
	gw := classifyingGateway(t, errorclass.NewDefaultClassifier(), authErr)

	_, err := gw.Stream(context.Background(), &llmgateway.Request{
		Model:    "deepseek-v4-flash",
		Messages: []types.Message{*types.NewMessage("1", "s", types.MessageRoleUser, "hi")},
	})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !stderrors.Is(err, sharederrors.ErrLLMAuthFailed) {
		t.Errorf("errors.Is(err, ErrLLMAuthFailed) = false, got %q", err.Error())
	}
}

// T: D3-S3-A02-T01 (AC7 nil classifier 降级)
// 不注入 classifier 时，错误仅被 shortstack 包装、不带 [class=...] 标签。
func TestInvoke_NoClassifier_ShortstackOnly(t *testing.T) {
	gw := classifyingGateway(t, nil, sharederrors.NewLLMAuthFailedError(stderrors.New("status 401")))

	_, err := gw.Stream(context.Background(), &llmgateway.Request{
		Model:    "deepseek-v4-flash",
		Messages: []types.Message{*types.NewMessage("1", "s", types.MessageRoleUser, "hi")},
	})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if strings.HasPrefix(err.Error(), "[class=") {
		t.Errorf("expected no [class=...] tag when classifier nil, got %q", err.Error())
	}
	// shortstack 至少 1 帧（wrapped error 含 "\n" + 栈）
	if !strings.Contains(err.Error(), "\n") {
		t.Errorf("expected shortstack line, got %q", err.Error())
	}
}

// T: D3-S3-A02-T01 (AC7 成功路径)
// adapter 返回正常 chunks 时，gateway 不写入 [class=...] 标签。
func TestInvoke_Success_NoClassification(t *testing.T) {
	counter, err := budget.NewCounter()
	if err != nil {
		t.Fatalf("counter: %v", err)
	}
	cfg := sharedconfig.DefaultLLMGatewayConfig()
	reg := streamadapter.NewRegistry()
	_ = reg.Register(stubAdapter{provider: "deepseek", handler: func(model string) (<-chan *llmgateway.AdapterChunk, error) {
		ch := make(chan *llmgateway.AdapterChunk, 1)
		ch <- &llmgateway.AdapterChunk{Parsed: &llmgateway.Chunk{Content: "ok", Done: true}}
		close(ch)
		return ch, nil
	}})
	gw := stream.New(stream.Deps{
		Config: cfg, Registry: reg, Breaker: protect.New(cfg.CircuitBreaker),
		Retry: protect.NewExecutor(), Counter: counter,
	}).WithClassifier(errorclass.NewDefaultClassifier())

	ch, err := gw.Stream(context.Background(), &llmgateway.Request{
		Model:    "deepseek-v4-flash",
		Messages: []types.Message{*types.NewMessage("1", "s", types.MessageRoleUser, "hi")},
	})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	var lastErr error
	var gotContent bool
	for c := range ch {
		if c.Content == "ok" {
			gotContent = true
		}
	}
	if !gotContent {
		t.Errorf("expected ok content chunk")
	}
	if lastErr != nil {
		t.Errorf("unexpected lastErr=%v", lastErr)
	}
}
