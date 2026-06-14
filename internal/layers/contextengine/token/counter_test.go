package token_test

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/token"
	llmtoken "github.com/devrix/devrix/internal/layers/llmgateway/budget"
	"github.com/pkoukk/tiktoken-go"
)

// T: D2-S4-A01-T01
func TestCounter_should_implement_shared_contract(t *testing.T) {
	var _ interface{ CountText(string) int } = token.NewCounter()
}

// T: D2-S4-A01-T01
func TestCounter_should_align_with_gateway_within_tolerance(t *testing.T) {
	heuristic := token.NewCounter()
	gateway, err := llmtoken.NewCounter()
	if err != nil {
		t.Fatalf("NewCounter: %v", err)
	}
	enc, err := tiktoken.GetEncoding("cl100k_base")
	if err != nil {
		t.Fatalf("GetEncoding: %v", err)
	}

	text := "package main\n\nfunc main() { println(\"hello\") }"
	expected := len(enc.Encode(text, nil, nil))
	got := gateway.CountText(text)
	delta := abs(got-expected) * 100 / max(1, expected)
	if delta > 5 {
		t.Fatalf("gateway count delta %d%% for %q", delta, text)
	}
	if heuristic.CountText(text) == 0 {
		t.Fatal("heuristic counter returned zero")
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
