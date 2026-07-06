package sessionorchestrator

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/types"
)

// stubLLMInvoker is a test double for orchtypes.LLMInvoker that returns a
// scripted sequence of content strings from a single InvokeStream call.
type stubLLMInvoker struct {
	chunks   []string
	err      error
	invokes  int
	lastTier string
}

func (s *stubLLMInvoker) InvokeStream(ctx context.Context, req orchtypes.LLMInvokeRequest) (<-chan llmgateway.Chunk, error) {
	s.invokes++
	s.lastTier = req.Tier
	if s.err != nil {
		return nil, s.err
	}
	ch := make(chan llmgateway.Chunk, len(s.chunks))
	for _, c := range s.chunks {
		ch <- llmgateway.Chunk{Content: c}
	}
	close(ch)
	return ch, nil
}

// TestParseSemanticVerdictJSON_Pass confirms a clean pass JSON parses to
// VerdictPass with confidence preserved.
func TestParseSemanticVerdictJSON_Pass(t *testing.T) {
	raw := `{"verdict":"pass","confidence":0.9,"reason":"addresses the d2 kernel question with file:line"}`
	req := SemanticVerifyRequest{ItemID: "wi-1", SessionID: "sess-1"}
	v, decision, err := parseSemanticVerdictJSON(raw, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.Kind != types.VerdictPass {
		t.Errorf("expected VerdictPass, got %v", v.Kind)
	}
	if v.Confidence != 0.9 {
		t.Errorf("confidence: got %v want 0.9", v.Confidence)
	}
	// decision defaults to "" for pass
	if decision != "" {
		t.Errorf("expected empty decision on pass, got %q", decision)
	}
}

// TestParseSemanticVerdictJSON_FailWithStop confirms "fail"+"stop" parses
// to (VerdictFail, "stop").
func TestParseSemanticVerdictJSON_FailWithStop(t *testing.T) {
	raw := `{"verdict":"fail","confidence":0.95,"reason":"template-mimicry confirmed","decision":"stop"}`
	req := SemanticVerifyRequest{ItemID: "wi-1", SessionID: "sess-1"}
	v, decision, err := parseSemanticVerdictJSON(raw, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.Kind != types.VerdictFail {
		t.Errorf("expected VerdictFail, got %v", v.Kind)
	}
	if decision != "stop" {
		t.Errorf("expected decision=stop, got %q", decision)
	}
}

// TestParseSemanticVerdictJSON_FailDefaultStop confirms "fail" without
// explicit decision defaults to "stop" (terminating, not retrying).
func TestParseSemanticVerdictJSON_FailDefaultStop(t *testing.T) {
	raw := `{"verdict":"fail","confidence":0.8,"reason":"answer is template-only"}`
	req := SemanticVerifyRequest{ItemID: "wi-1", SessionID: "sess-1"}
	_, decision, err := parseSemanticVerdictJSON(raw, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision != "stop" {
		t.Errorf("expected default decision=stop on fail, got %q", decision)
	}
}

// TestParseSemanticVerdictJSON_PartialDefaultRetry confirms "partial"
// without explicit decision defaults to "retry" (self-correct path).
func TestParseSemanticVerdictJSON_PartialDefaultRetry(t *testing.T) {
	raw := `{"verdict":"partial","confidence":0.6,"reason":"addresses 1 of 3 sub-questions"}`
	req := SemanticVerifyRequest{ItemID: "wi-1", SessionID: "sess-1"}
	_, decision, err := parseSemanticVerdictJSON(raw, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision != "retry" {
		t.Errorf("expected default decision=retry on partial, got %q", decision)
	}
}

// TestParseSemanticVerdictJSON_PartialExplicitRetry confirms explicit
// "decision":"retry" is preserved verbatim.
func TestParseSemanticVerdictJSON_PartialExplicitRetry(t *testing.T) {
	raw := `{"verdict":"partial","confidence":0.7,"reason":"on the right track","decision":"retry"}`
	req := SemanticVerifyRequest{ItemID: "wi-1", SessionID: "sess-1"}
	v, decision, err := parseSemanticVerdictJSON(raw, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.Kind != types.VerdictPartial {
		t.Errorf("expected VerdictPartial, got %v", v.Kind)
	}
	if decision != "retry" {
		t.Errorf("expected decision=retry, got %q", decision)
	}
}

// TestParseSemanticVerdictJSON_CodeFenceStrips confirms markdown fences
// around the JSON are stripped (LLMs frequently wrap JSON in ```json).
func TestParseSemanticVerdictJSON_CodeFenceStrips(t *testing.T) {
	raw := "```json\n{\"verdict\":\"pass\",\"confidence\":0.9,\"reason\":\"ok\"}\n```"
	req := SemanticVerifyRequest{ItemID: "wi-1", SessionID: "sess-1"}
	v, _, err := parseSemanticVerdictJSON(raw, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.Kind != types.VerdictPass {
		t.Errorf("expected VerdictPass, got %v", v.Kind)
	}
}

// TestParseSemanticVerdictJSON_NoJSON confirms an LLM response with no
// JSON object returns an error (caller falls back to code verdict).
func TestParseSemanticVerdictJSON_NoJSON(t *testing.T) {
	raw := "I am not sure what the user asked. Let me think about it."
	req := SemanticVerifyRequest{ItemID: "wi-1", SessionID: "sess-1"}
	_, _, err := parseSemanticVerdictJSON(raw, req)
	if err == nil {
		t.Fatal("expected error when no JSON object is present")
	}
}

// TestParseSemanticVerdictJSON_UnknownVerdict confirms unknown verdict
// string returns an error (caller falls back to code verdict).
func TestParseSemanticVerdictJSON_UnknownVerdict(t *testing.T) {
	raw := `{"verdict":"banana","confidence":0.5,"reason":"huh"}`
	req := SemanticVerifyRequest{ItemID: "wi-1", SessionID: "sess-1"}
	_, _, err := parseSemanticVerdictJSON(raw, req)
	if err == nil {
		t.Fatal("expected error for unknown verdict")
	}
}

// TestParseSemanticVerdictJSON_ConfidenceClamp confirms out-of-range
// confidence is clamped to 0.5 (defensive — caller falls back to code).
func TestParseSemanticVerdictJSON_ConfidenceClamp(t *testing.T) {
	raw := `{"verdict":"pass","confidence":2.5,"reason":"overconfident"}`
	req := SemanticVerifyRequest{ItemID: "wi-1", SessionID: "sess-1"}
	v, _, err := parseSemanticVerdictJSON(raw, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.Confidence != 0.5 {
		t.Errorf("expected confidence clamped to 0.5, got %v", v.Confidence)
	}
}

// TestMapSemanticVerdictKind_Aliases confirms the alias set covers common
// LLM phrasings (e.g. "complete", "answered", "mimicry").
func TestMapSemanticVerdictKind_Aliases(t *testing.T) {
	passes := []string{"pass", "complete", "answered", " PASS "}
	for _, p := range passes {
		k, err := mapSemanticVerdictKind(p)
		if err != nil || k != types.VerdictPass {
			t.Errorf("expected VerdictPass for %q, got %v err=%v", p, k, err)
		}
	}
	fails := []string{"fail", "template", "mimicry", "unanswered", " FAIL "}
	for _, p := range fails {
		k, err := mapSemanticVerdictKind(p)
		if err != nil || k != types.VerdictFail {
			t.Errorf("expected VerdictFail for %q, got %v err=%v", p, k, err)
		}
	}
	partials := []string{"partial", "incomplete", " Partial "}
	for _, p := range partials {
		k, err := mapSemanticVerdictKind(p)
		if err != nil || k != types.VerdictPartial {
			t.Errorf("expected VerdictPartial for %q, got %v err=%v", p, k, err)
		}
	}
}

// TestDefaultSemanticVerifier_NilLLM confirms nil LLM → fail-open returns
// codeBasedVerdict + an error so caller falls back.
func TestDefaultSemanticVerifier_NilLLM(t *testing.T) {
	v := &DefaultSemanticVerifier{LLM: nil}
	req := SemanticVerifyRequest{
		SessionID:        "sess-1",
		ItemID:           "wi-1",
		CodeBasedVerdict: workmodel.Verdict{Kind: types.VerdictPass, Confidence: 0.9},
	}
	got, err := v.VerifySemantically(context.Background(), req)
	if err == nil {
		t.Fatal("expected error when LLM is nil")
	}
	if got.Kind != types.VerdictPass {
		t.Errorf("expected to fall back to code verdict (Pass), got %v", got.Kind)
	}
}

// TestDefaultSemanticVerifier_FastFailSkipsLLM confirms codeBasedVerdict
// Fail short-circuits (no LLM call needed for an obvious failure).
func TestDefaultSemanticVerifier_FastFailSkipsLLM(t *testing.T) {
	stub := &stubLLMInvoker{}
	v := &DefaultSemanticVerifier{LLM: stub, Timeout: 1 * time.Second}
	req := SemanticVerifyRequest{
		SessionID:        "sess-1",
		ItemID:           "wi-1",
		CodeBasedVerdict: workmodel.Verdict{Kind: types.VerdictFail, Confidence: 0.9},
	}
	got, err := v.VerifySemantically(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Kind != types.VerdictFail {
		t.Errorf("expected VerdictFail preserved, got %v", got.Kind)
	}
	if stub.invokes != 0 {
		t.Errorf("expected LLM to be skipped (0 invokes), got %d", stub.invokes)
	}
}

// TestDefaultSemanticVerifier_PassFromLLM confirms the LLM-emitting
// VerdictPass returns the LLM verdict (with decision prefix preserved
// on Reason) so the runner can keep the code path.
func TestDefaultSemanticVerifier_PassFromLLM(t *testing.T) {
	stub := &stubLLMInvoker{
		chunks: []string{`{"verdict":"pass","confidence":0.92,"reason":"addresses the question"}`},
	}
	v := &DefaultSemanticVerifier{LLM: stub, Timeout: 1 * time.Second}
	req := SemanticVerifyRequest{
		SessionID:        "sess-1",
		ItemID:           "wi-1",
		CodeBasedVerdict: workmodel.Verdict{Kind: types.VerdictPass, Confidence: 0.9},
	}
	got, err := v.VerifySemantically(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Kind != types.VerdictPass {
		t.Errorf("expected VerdictPass, got %v", got.Kind)
	}
	if !containsString(got.Reason, "addresses the question") {
		t.Errorf("expected reason to carry LLM reason, got %q", got.Reason)
	}
}

// TestDefaultSemanticVerifier_FailStopAppliesDecisionPrefix confirms the
// fail-with-stop path attaches "[decision=stop] " to the reason so the
// runner's extractDecisionAndHint can read it.
func TestDefaultSemanticVerifier_FailStopAppliesDecisionPrefix(t *testing.T) {
	stub := &stubLLMInvoker{
		chunks: []string{`{"verdict":"fail","confidence":0.9,"reason":"template-mimicry","decision":"stop"}`},
	}
	v := &DefaultSemanticVerifier{LLM: stub, Timeout: 1 * time.Second}
	req := SemanticVerifyRequest{
		SessionID:        "sess-1",
		ItemID:           "wi-1",
		CodeBasedVerdict: workmodel.Verdict{Kind: types.VerdictPass, Confidence: 0.9},
	}
	got, err := v.VerifySemantically(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Kind != types.VerdictFail {
		t.Errorf("expected VerdictFail, got %v", got.Kind)
	}
	d, _ := extractDecisionAndHint(got)
	if d != "stop" {
		t.Errorf("expected decision=stop, got %q", d)
	}
}

// TestDefaultSemanticVerifier_LLMErrorFallback confirms an LLM infra
// error returns codeBasedVerdict + error (fail-open).
func TestDefaultSemanticVerifier_LLMErrorFallback(t *testing.T) {
	stub := &stubLLMInvoker{err: errors.New("network timeout")}
	v := &DefaultSemanticVerifier{LLM: stub, Timeout: 1 * time.Second}
	req := SemanticVerifyRequest{
		SessionID:        "sess-1",
		ItemID:           "wi-1",
		CodeBasedVerdict: workmodel.Verdict{Kind: types.VerdictPass, Confidence: 0.9},
	}
	got, err := v.VerifySemantically(context.Background(), req)
	if err == nil {
		t.Fatal("expected error to surface to caller")
	}
	if got.Kind != types.VerdictPass {
		t.Errorf("expected to fall back to code verdict (Pass), got %v", got.Kind)
	}
}

// TestDefaultSemanticVerifier_EmptyResponseFallback confirms an LLM
// that returns no content is treated as fail-open.
func TestDefaultSemanticVerifier_EmptyResponseFallback(t *testing.T) {
	stub := &stubLLMInvoker{chunks: []string{""}}
	v := &DefaultSemanticVerifier{LLM: stub, Timeout: 1 * time.Second}
	req := SemanticVerifyRequest{
		SessionID:        "sess-1",
		ItemID:           "wi-1",
		CodeBasedVerdict: workmodel.Verdict{Kind: types.VerdictPass, Confidence: 0.9},
	}
	got, err := v.VerifySemantically(context.Background(), req)
	if err == nil {
		t.Fatal("expected error on empty response")
	}
	if got.Kind != types.VerdictPass {
		t.Errorf("expected fall back to Pass, got %v", got.Kind)
	}
}

// TestDefaultSemanticVerifier_TimeoutFallback confirms a stuck LLM
// (context-cancelled) falls back to code verdict via the timeout
// context. We close the channel only after the verifier's timeout
// fires.
func TestDefaultSemanticVerifier_TimeoutFallback(t *testing.T) {
	slow := &slowLLMInvoker{}
	v := &DefaultSemanticVerifier{LLM: slow, Timeout: 50 * time.Millisecond}
	req := SemanticVerifyRequest{
		SessionID:        "sess-1",
		ItemID:           "wi-1",
		CodeBasedVerdict: workmodel.Verdict{Kind: types.VerdictPass, Confidence: 0.9},
	}
	got, err := v.VerifySemantically(context.Background(), req)
	t.Logf("got verdict: %+v", got)
	t.Logf("got err: %v", err)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if got.Kind != types.VerdictPass {
		t.Errorf("expected fall back to Pass, got %v", got.Kind)
	}
}

// slowLLMInvoker is a stub that never sends any chunk and only closes
// the stream when the caller's context is cancelled. Used to exercise
// the verifier's ctx.Done() path (timeout fallback).
type slowLLMInvoker struct{}

func (s *slowLLMInvoker) InvokeStream(ctx context.Context, req orchtypes.LLMInvokeRequest) (<-chan llmgateway.Chunk, error) {
	ch := make(chan llmgateway.Chunk)
	go func() {
		defer close(ch)
		// Block on ctx.Done() to simulate a stuck LLM stream that
		// produces no content. The verifier's select will hit the
		// cctx.Done() branch, log a warning, and break out with
		// empty content → fall back to code verdict + error.
		<-ctx.Done()
	}()
	return ch, nil
}
