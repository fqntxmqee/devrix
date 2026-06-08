package token_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/llmgateway/token"
	sharederrors "github.com/devrix/devrix/internal/shared/errors"
	"github.com/devrix/devrix/internal/shared/types"
	"github.com/pkoukk/tiktoken-go"
)

// Covers: L5-LLM-07
func TestCounter_should_count_known_text_within_tolerance(t *testing.T) {
	c, err := token.NewCounter()
	if err != nil {
		t.Fatalf("NewCounter: %v", err)
	}

	enc, err := tiktoken.GetEncoding("cl100k_base")
	if err != nil {
		t.Fatalf("GetEncoding: %v", err)
	}

	cases := []string{
		"hello world",
		"package main\n\nfunc main() {}",
		strings.Repeat("token ", 100),
	}

	for _, text := range cases {
		expected := len(enc.Encode(text, nil, nil))
		got := c.CountText(text)
		delta := abs(got-expected) * 100 / max(1, expected)
		if delta > 5 {
			t.Errorf("CountText(%q): got %d want %d (delta %d%%)", text, got, expected, delta)
		}
	}
}

// Covers: L5-LLM-07
func TestCounter_should_count_messages_with_role_overhead(t *testing.T) {
	c, err := token.NewCounter()
	if err != nil {
		t.Fatalf("NewCounter: %v", err)
	}

	msgs := []types.Message{
		*types.NewMessage("1", "s", types.MessageRoleUser, "hi"),
		*types.NewMessage("2", "s", types.MessageRoleAssistant, "hello"),
	}
	count := c.CountMessages(msgs)
	if count <= c.CountText("hi")+c.CountText("hello") {
		t.Errorf("expected role overhead, got %d", count)
	}
}

// Covers: L5-LLM-07
func TestCounter_should_include_system_prompt(t *testing.T) {
	c, err := token.NewCounter()
	if err != nil {
		t.Fatalf("NewCounter: %v", err)
	}

	msgs := []types.Message{*types.NewMessage("1", "s", types.MessageRoleUser, "question")}
	withSystem := c.CountWithSystemPrompt("You are helpful.", msgs)
	without := c.CountMessages(msgs)
	if withSystem <= without {
		t.Errorf("withSystem=%d without=%d", withSystem, without)
	}
}

// Covers: L5-LLM-08
func TestCounter_should_pass_budget_when_within_limit(t *testing.T) {
	c, err := token.NewCounter()
	if err != nil {
		t.Fatalf("NewCounter: %v", err)
	}
	if err := c.CheckBudget(10, 100); err != nil {
		t.Fatalf("CheckBudget: %v", err)
	}
}

// Covers: L5-LLM-08
func TestCounter_should_fail_budget_when_over_limit(t *testing.T) {
	c, err := token.NewCounter()
	if err != nil {
		t.Fatalf("NewCounter: %v", err)
	}
	err = c.CheckBudget(200, 100)
	if err == nil {
		t.Fatal("expected budget error")
	}
	var llmErr *sharederrors.LLMError
	if !errors.As(err, &llmErr) {
		t.Fatalf("expected LLMError, got %T", err)
	}
	if llmErr.Code != sharederrors.CodeLLMTokenBudgetExceeded {
		t.Errorf("code: got %s", llmErr.Code)
	}
}

// Covers: L5-LLM-07
func TestCounter_should_truncate_to_max_tokens(t *testing.T) {
	c, err := token.NewCounter()
	if err != nil {
		t.Fatalf("NewCounter: %v", err)
	}
	long := strings.Repeat("word ", 500)
	if c.CountText(long) <= 50 {
		t.Fatal("fixture too short")
	}
	truncated := c.TruncateToTokens(long, 20)
	if len(truncated) >= len(long) {
		t.Error("expected truncated text shorter than input")
	}
	if c.CountText(truncated) > 40 {
		t.Errorf("truncated count too high: %d", c.CountText(truncated))
	}
}

func TestCounter_should_return_cl100k_base_encoding(t *testing.T) {
	c, err := token.NewCounter()
	if err != nil {
		t.Fatalf("NewCounter: %v", err)
	}
	if got := c.EncodingForModel("deepseek-v4-flash"); got != "cl100k_base" {
		t.Errorf("EncodingForModel: got %s", got)
	}
}

func TestEstimateRemaining_should_not_go_negative(t *testing.T) {
	if got := token.EstimateRemaining(200, 100); got != 0 {
		t.Errorf("got %d", got)
	}
	if got := token.EstimateRemaining(40, 100); got != 60 {
		t.Errorf("got %d", got)
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func TestCounter_should_apply_cjk_multiplier_when_configured(t *testing.T) {
	c, err := token.NewCounter()
	if err != nil {
		t.Fatalf("NewCounter: %v", err)
	}
	base := c.CountText("你好世界")
	c.WithCJKMultiplier(1.5)
	got := c.CountText("你好世界")
	want := int(float64(base) * 1.5)
	if got != want {
		t.Fatalf("CountText CJK: got %d want %d", got, want)
	}
}
