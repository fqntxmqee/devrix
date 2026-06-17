package errorclass

import (
	"context"
	stderrors "errors"
	"fmt"
	"testing"

	sharederrors "github.com/devrix/devrix/internal/shared/errors"
)

// TestClassify_BySentinel — 已知 sentinel 走第一层匹配。
func TestClassify_BySentinel(t *testing.T) {
	c := NewDefaultClassifier()
	wrapped := sharederrors.NewLLMTimeoutError(stderrors.New("net"))
	cls := c.Classify(wrapped, 0, "")
	if cls.Class != ClassTimeout {
		t.Fatalf("expected Timeout, got %s", cls.Class)
	}
	if !cls.Retry {
		t.Fatalf("expected Retry=true for timeout")
	}
	if cls.UserHint == "" {
		t.Fatalf("UserHint should not be empty")
	}
}

// TestClassify_ByHTTPStatus — 429 / 503 / 500 状态码优先。
func TestClassify_ByHTTPStatus(t *testing.T) {
	c := NewDefaultClassifier()
	cases := []struct {
		status int
		want   Class
	}{
		{429, ClassRateLimit},
		{401, ClassAuth},
		{403, ClassPermission},
		{404, ClassNotFound},
		{408, ClassTimeout},
		{413, ClassPromptTooLong},
		{500, ClassUpstreamDown},
		{502, ClassUpstreamDown},
		{503, ClassOverloaded},
		{509, ClassQuota},
	}
	for _, tc := range cases {
		cls := c.Classify(stderrors.New("x"), tc.status, "")
		if cls.Class != tc.want {
			t.Errorf("status %d: expected %s, got %s", tc.status, tc.want, cls.Class)
		}
	}
}

// TestClassify_ByRegexRateLimit — body 含 "rate limit" 字符串。
func TestClassify_ByRegexRateLimit(t *testing.T) {
	c := NewDefaultClassifier()
	cls := c.Classify(nil, 0, "429 Too many requests: rate limit exceeded")
	if cls.Class != ClassRateLimit {
		t.Fatalf("expected RateLimit, got %s", cls.Class)
	}
	if !cls.Retry {
		t.Fatalf("RateLimit should be retryable")
	}
}

// TestClassify_ByRegexContextOverflow — body 含 "context_length_exceeded"。
func TestClassify_ByRegexContextOverflow(t *testing.T) {
	c := NewDefaultClassifier()
	cls := c.Classify(nil, 0, "context_length_exceeded: maximum context length is 8192 tokens")
	if cls.Class != ClassContextOverflow {
		t.Fatalf("expected ContextOverflow, got %s", cls.Class)
	}
	if cls.Retry {
		t.Fatalf("ContextOverflow should NOT be retryable")
	}
}

// TestClassify_ByRegexContentFilter — body 含 "content_filter"。
func TestClassify_ByRegexContentFilter(t *testing.T) {
	c := NewDefaultClassifier()
	cls := c.Classify(nil, 0, "content_filter triggered: violates usage policy")
	if cls.Class != ClassContentFilter {
		t.Fatalf("expected ContentFilter, got %s", cls.Class)
	}
}

// TestClassify_FallbackUnknown — 无匹配时返回 ClassUnknown。
func TestClassify_FallbackUnknown(t *testing.T) {
	c := NewDefaultClassifier()
	cls := c.Classify(stderrors.New("weird unrelated thing"), 0, "")
	if cls.Class != ClassUnknown {
		t.Fatalf("expected Unknown, got %s", cls.Class)
	}
}

// TestClassify_DetailTruncation — Detail 字段被截断到 256 字符。
func TestClassify_DetailTruncation(t *testing.T) {
	c := NewDefaultClassifier()
	big := ""
	for i := 0; i < 1000; i++ {
		big += "x"
	}
	cls := c.Classify(nil, 0, big)
	if len(cls.Detail) > 270 { // 256 + ellipsis "…"
		t.Fatalf("Detail too long: %d", len(cls.Detail))
	}
}

// TestAllClasses_A20 — AllClasses 返回 ≥20 个分类常量。
func TestAllClasses_A20(t *testing.T) {
	all := AllClasses()
	if len(all) < 20 {
		t.Fatalf("expected >=20 classes, got %d", len(all))
	}
	// 检查没有重复
	seen := make(map[Class]bool)
	for _, c := range all {
		if seen[c] {
			t.Fatalf("duplicate class: %s", c)
		}
		seen[c] = true
	}
}

// TestContextRoundTrip — InjectClassification + FromContext 往返。
func TestContextRoundTrip(t *testing.T) {
	ctx := InjectClassification(context.Background(), Classification{
		Class:    ClassRateLimit,
		Retry:    true,
		UserHint: "hint",
		Detail:   "d",
	})
	got, ok := FromContext(ctx)
	if !ok {
		t.Fatal("FromContext returned ok=false")
	}
	if got.Class != ClassRateLimit || got.UserHint != "hint" {
		t.Fatalf("roundtrip mismatch: %+v", got)
	}
}

// TestContextRoundTrip_Empty — 未注入的 ctx 返回 ok=false。
func TestContextRoundTrip_Empty(t *testing.T) {
	_, ok := FromContext(context.Background())
	if ok {
		t.Fatal("expected ok=false for empty ctx")
	}
}

// TestClassify_SentinelBeatsHTTPStatus — sentinel 优先于 HTTP 状态码。
func TestClassify_SentinelBeatsHTTPStatus(t *testing.T) {
	c := NewDefaultClassifier()
	wrapped := sharederrors.NewLLMTimeoutError(stderrors.New("net"))
	cls := c.Classify(wrapped, 503, "")
	if cls.Class != ClassTimeout {
		t.Fatalf("expected Timeout (sentinel priority), got %s", cls.Class)
	}
}

// TestClassify_AllHTTPStatus — 5xx 状态码 → UpstreamDown/Overloaded/Quota 之一。
func TestClassify_AllHTTPStatus(t *testing.T) {
	c := NewDefaultClassifier()
	allowed := map[Class]bool{
		ClassUpstreamDown: true,
		ClassOverloaded:   true,
		ClassQuota:        true,
	}
	for status := 500; status <= 599; status++ {
		cls := c.Classify(nil, status, "")
		if !allowed[cls.Class] {
			t.Errorf("status %d: expected UpstreamDown/Overloaded/Quota, got %s", status, cls.Class)
		}
	}
}

// TestClassify_QuotaStrings — quota / billing 关键字匹配。
func TestClassify_QuotaStrings(t *testing.T) {
	c := NewDefaultClassifier()
	cases := []string{
		"insufficient_quota: please upgrade your plan",
		"Your account has reached its billing limit",
		"insufficient credit remaining",
	}
	for _, s := range cases {
		cls := c.Classify(nil, 0, s)
		if cls.Class != ClassQuota {
			t.Errorf("input %q: expected Quota, got %s", s, cls.Class)
		}
	}
}

// TestClassify_Cancelled — 取消错误归 ClassCancelled。
func TestClassify_Cancelled(t *testing.T) {
	c := NewDefaultClassifier()
	cls := c.Classify(context.Canceled, 0, "")
	if cls.Class != ClassCancelled {
		t.Fatalf("expected Cancelled, got %s", cls.Class)
	}
}

// TestClassify_NetworkEOF — EOF 归 Network。
func TestClassify_NetworkEOF(t *testing.T) {
	c := NewDefaultClassifier()
	cls := c.Classify(stderrors.New("unexpected EOF"), 0, "")
	if cls.Class != ClassNetwork {
		t.Fatalf("expected Network, got %s", cls.Class)
	}
}

// TestClassify_ProviderAuth — provider 鉴权失败归 Auth。
func TestClassify_ProviderAuth(t *testing.T) {
	c := NewDefaultClassifier()
	cls := c.Classify(nil, 401, "Invalid API key provided")
	if cls.Class != ClassAuth {
		t.Fatalf("expected Auth, got %s", cls.Class)
	}
}

// TestClassify_ToolUseError — tool_use_error 归类。
func TestClassify_ToolUseError(t *testing.T) {
	c := NewDefaultClassifier()
	cls := c.Classify(nil, 0, "tool_use_error: tool not found")
	if cls.Class != ClassToolUseError {
		t.Fatalf("expected ToolUseError, got %s", cls.Class)
	}
}

// TestClassify_CircuitOpen — ErrCircuitOpen 优先级高于 HTTP 200。
func TestClassify_CircuitOpen(t *testing.T) {
	c := NewDefaultClassifier()
	wrapped := fmt.Errorf("wrapped: %w", sharederrors.ErrCircuitOpen)
	cls := c.Classify(wrapped, 200, "")
	if cls.Class != ClassCircuitOpen {
		t.Fatalf("expected CircuitOpen, got %s", cls.Class)
	}
}
