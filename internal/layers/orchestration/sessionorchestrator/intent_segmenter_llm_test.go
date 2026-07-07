package sessionorchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	ifaces "github.com/devrix/devrix/internal/layers/orchestration/interfaces"
	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
	"github.com/devrix/devrix/internal/shared/types"
)

// =====================================================================
// LLMIntentSegmenter tests (PR-A2 Q7 ADOPT-WITH-CHANGE)
//
// Coverage targets:
//   - Prompt: contains the 4 enum kinds + 6 example patterns
//   - Parsing: accepts valid JSON array, single object, fenced markdown
//   - Parsing: rejects malformed JSON → 7121 error
//   - Parsing: empty response → 7121 error
//   - Parsing: empty array → 7122 error
//   - Parsing: clamps out-of-range priority/confidence
//   - Parsing: unknown kind → falls back to "explore" (safest default)
// =====================================================================

// stubSegLLM lets each test inject a raw string response.
type stubSegLLM struct {
	raw         string
	invokeErr   error
	systemLast  string
	messageLast []types.Message
}

func (s *stubSegLLM) InvokeStream(_ context.Context, req orchtypes.LLMInvokeRequest) (<-chan llmgateway.Chunk, error) {
	s.systemLast = req.SystemPrompt
	s.messageLast = req.Messages
	if s.invokeErr != nil {
		return nil, s.invokeErr
	}
	ch := make(chan llmgateway.Chunk, 1)
	ch <- llmgateway.Chunk{Content: s.raw}
	close(ch)
	return ch, nil
}

func TestLLMIntentSegmenter_PromptContainsExamples(t *testing.T) {
	stub := &stubSegLLM{raw: `[{"id":"seg_0","text":"x","kind":"explore","priority":50,"confidence":0.8}]`}
	s := NewLLMIntentSegmenter(stub)
	set, err := s.Segment(context.Background(), SegmentRequest{
		SessionID: "sess_l1",
		Message:   "查 devrix",
	})
	if err != nil {
		t.Fatalf("Segment: %v", err)
	}
	if len(set.Segments) != 1 {
		t.Errorf("len(Segments) = %d, want 1", len(set.Segments))
	}
	// System prompt must include the 4 enum kinds and at least 1 example.
	if !strings.Contains(stub.systemLast, "deterministic") ||
		!strings.Contains(stub.systemLast, "explore") ||
		!strings.Contains(stub.systemLast, "commit") ||
		!strings.Contains(stub.systemLast, "analyze") {
		t.Errorf("system prompt missing enum kind names: %q", stub.systemLast)
	}
	if !strings.Contains(stub.systemLast, "Example 1") {
		t.Errorf("system prompt missing 6-shot examples: %q", stub.systemLast)
	}
	// User prompt must contain the directive verbatim.
	if len(stub.messageLast) != 1 || !strings.Contains(stub.messageLast[0].Content, "查 devrix") {
		t.Errorf("user prompt missing directive, got %+v", stub.messageLast)
	}
}

func TestLLMIntentSegmenter_ParsesValidResponse_SingleObject(t *testing.T) {
	stub := &stubSegLLM{raw: `[{"id":"seg_0","text":"1+1=几?","kind":"deterministic","priority":50,"confidence":0.95}]`}
	s := NewLLMIntentSegmenter(stub)
	set, err := s.Segment(context.Background(), SegmentRequest{
		SessionID: "sess_l2",
		Message:   "1+1=几?",
	})
	if err != nil {
		t.Fatalf("Segment: %v", err)
	}
	if len(set.Segments) != 1 {
		t.Fatalf("len(Segments) = %d, want 1", len(set.Segments))
	}
	seg := set.Segments[0]
	if seg.Text != "1+1=几?" {
		t.Errorf("Text = %q, want %q", seg.Text, "1+1=几?")
	}
	if seg.Kind != ifaces.IntentSegmentKindDeterministic {
		t.Errorf("Kind = %q, want %q", seg.Kind, ifaces.IntentSegmentKindDeterministic)
	}
	if seg.Confidence != 0.95 {
		t.Errorf("Confidence = %v, want 0.95", seg.Confidence)
	}
}

func TestLLMIntentSegmenter_ParsesValidResponse_MultiObject(t *testing.T) {
	raw := `[
		{"id":"seg_0","text":"1+1=几?","kind":"deterministic","priority":50,"confidence":0.95},
		{"id":"seg_1","text":"巴黎时区?","kind":"deterministic","priority":40,"confidence":0.9}
	]`
	stub := &stubSegLLM{raw: raw}
	s := NewLLMIntentSegmenter(stub)
	set, err := s.Segment(context.Background(), SegmentRequest{
		SessionID: "sess_l3",
		Message:   "1+1=几? 巴黎时区?",
	})
	if err != nil {
		t.Fatalf("Segment: %v", err)
	}
	if len(set.Segments) != 2 {
		t.Fatalf("len(Segments) = %d, want 2", len(set.Segments))
	}
	if set.Segments[0].ID != "seg_0" || set.Segments[1].ID != "seg_1" {
		t.Errorf("IDs = [%q, %q], want [seg_0, seg_1]", set.Segments[0].ID, set.Segments[1].ID)
	}
}

func TestLLMIntentSegmenter_ParsesValidResponse_FencedMarkdown(t *testing.T) {
	raw := "```json\n[{\"id\":\"seg_0\",\"text\":\"x\",\"kind\":\"explore\",\"priority\":50,\"confidence\":0.8}]\n```"
	stub := &stubSegLLM{raw: raw}
	s := NewLLMIntentSegmenter(stub)
	set, err := s.Segment(context.Background(), SegmentRequest{
		SessionID: "sess_l4",
		Message:   "查 devrix",
	})
	if err != nil {
		t.Fatalf("Segment (fenced markdown): %v", err)
	}
	if len(set.Segments) != 1 {
		t.Errorf("len(Segments) = %d, want 1 (fenced markdown should still parse)", len(set.Segments))
	}
}

func TestLLMIntentSegmenter_RejectsMalformedJSON(t *testing.T) {
	stub := &stubSegLLM{raw: "this is not json at all"}
	s := NewLLMIntentSegmenter(stub)
	_, err := s.Segment(context.Background(), SegmentRequest{
		SessionID: "sess_l5",
		Message:   "查 devrix",
	})
	if err == nil {
		t.Fatalf("expected error for malformed JSON, got nil")
	}
	if !ifaces.IsIntentSegmenterNoSegmentError(err) &&
		!errors.Is(err, ifaces.ErrIntentSegmenterLLMInvalidResponse) {
		t.Errorf("expected 7121 LLM invalid response, got %v", err)
	}
}

func TestLLMIntentSegmenter_EmptyResponse_Errors(t *testing.T) {
	stub := &stubSegLLM{raw: ""}
	s := NewLLMIntentSegmenter(stub)
	_, err := s.Segment(context.Background(), SegmentRequest{
		SessionID: "sess_l6",
		Message:   "查 devrix",
	})
	if err == nil {
		t.Fatalf("expected error for empty response, got nil")
	}
	if !errors.Is(err, ifaces.ErrIntentSegmenterLLMInvalidResponse) {
		t.Errorf("expected 7121, got %v", err)
	}
}

func TestLLMIntentSegmenter_EmptyArray_Errors(t *testing.T) {
	stub := &stubSegLLM{raw: "[]"}
	s := NewLLMIntentSegmenter(stub)
	_, err := s.Segment(context.Background(), SegmentRequest{
		SessionID: "sess_l7",
		Message:   "查 devrix",
	})
	if err == nil {
		t.Fatalf("expected error for empty array, got nil")
	}
	if !errors.Is(err, ifaces.ErrIntentSegmenterNoSegment) {
		t.Errorf("expected 7122 no-segment, got %v", err)
	}
}

func TestLLMIntentSegmenter_ClampsOutOfRange(t *testing.T) {
	raw := `[{"id":"seg_0","text":"x","kind":"explore","priority":-5,"confidence":1.5}]`
	stub := &stubSegLLM{raw: raw}
	s := NewLLMIntentSegmenter(stub)
	set, err := s.Segment(context.Background(), SegmentRequest{
		SessionID: "sess_l8",
		Message:   "查 devrix",
	})
	if err != nil {
		t.Fatalf("Segment: %v", err)
	}
	if len(set.Segments) != 1 {
		t.Fatalf("len(Segments) = %d, want 1", len(set.Segments))
	}
	if set.Segments[0].Priority != 0 {
		t.Errorf("Priority clamp: got %d, want 0", set.Segments[0].Priority)
	}
	if set.Segments[0].Confidence != 1.0 {
		t.Errorf("Confidence clamp: got %v, want 1.0", set.Segments[0].Confidence)
	}
}

func TestLLMIntentSegmenter_UnknownKind_FallsBackToExplore(t *testing.T) {
	raw := `[{"id":"seg_0","text":"x","kind":"unknown_kind","priority":50,"confidence":0.8}]`
	stub := &stubSegLLM{raw: raw}
	s := NewLLMIntentSegmenter(stub)
	set, err := s.Segment(context.Background(), SegmentRequest{
		SessionID: "sess_l9",
		Message:   "查 devrix",
	})
	if err != nil {
		t.Fatalf("Segment: %v", err)
	}
	if set.Segments[0].Kind != ifaces.IntentSegmentKindExplore {
		t.Errorf("unknown kind: got %q, want %q (safest default)",
			set.Segments[0].Kind, ifaces.IntentSegmentKindExplore)
	}
}

func TestLLMIntentSegmenter_InvokeError_Propagates(t *testing.T) {
	stub := &stubSegLLM{invokeErr: errors.New("llm gateway offline")}
	s := NewLLMIntentSegmenter(stub)
	_, err := s.Segment(context.Background(), SegmentRequest{
		SessionID: "sess_l10",
		Message:   "查 devrix",
	})
	if err == nil {
		t.Fatalf("expected error from invoke failure, got nil")
	}
	if !strings.Contains(err.Error(), "llm invoke") {
		t.Errorf("error should mention llm invoke, got %v", err)
	}
}

func TestLLMIntentSegmenter_NilInvoker(t *testing.T) {
	s := NewLLMIntentSegmenter(nil)
	if s != nil {
		t.Errorf("NewLLMIntentSegmenter(nil) should return nil, got %+v", s)
	}
}

// =====================================================================
// parseLLMSegmenterJSON unit tests (white-box: skip dispatcher; pin
// parse edge cases including markdown fencing + empty + boundary clamp).
// =====================================================================

func TestParseLLMSegmenterJSON_AcceptsValidArray(t *testing.T) {
	raw := `[{"id":"seg_0","text":"1+1=几?","kind":"deterministic","priority":50,"confidence":0.95}]`
	now := time.Now()
	set, err := parseLLMSegmenterJSON(raw, "src", now)
	if err != nil {
		t.Fatalf("parseLLMSegmenterJSON: %v", err)
	}
	if len(set.Segments) != 1 {
		t.Fatalf("len(Segments) = %d, want 1", len(set.Segments))
	}
	if set.SourceDirective != "src" {
		t.Errorf("SourceDirective = %q, want %q", set.SourceDirective, "src")
	}
}

func TestParseLLMSegmenterJSON_SliceFallback(t *testing.T) {
	// Raw text with preamble → must still extract [...] slice.
	raw := "OK, here you go: [{\"id\":\"seg_0\",\"text\":\"x\",\"kind\":\"explore\",\"priority\":50,\"confidence\":0.8}] done"
	set, err := parseLLMSegmenterJSON(raw, "src", time.Now())
	if err != nil {
		t.Fatalf("slice fallback: %v", err)
	}
	if len(set.Segments) != 1 {
		t.Errorf("len(Segments) = %d, want 1", len(set.Segments))
	}
}

func TestParseLLMSegmenterJSON_JSONMarshalRoundTrip(t *testing.T) {
	// Sanity check: ensure rawSegmenterLLMResponse is JSON-marshallable.
	r := rawSegmenterLLMResponse{ID: "seg_0", Text: "x", Kind: "explore", Priority: 50, Confidence: 0.8}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(b), "explore") {
		t.Errorf("marshal output missing kind: %s", b)
	}
}

// errReaderLLM is a test helper that returns a context-deadline error.
type errReaderLLM struct{}

func (errReaderLLM) InvokeStream(_ context.Context, _ orchtypes.LLMInvokeRequest) (<-chan llmgateway.Chunk, error) {
	return nil, fmt.Errorf("wrapped: %w", context.DeadlineExceeded)
}

func TestIsContextDeadline_WrappedSentinel(t *testing.T) {
	err := fmt.Errorf("wrapped: %w", context.DeadlineExceeded)
	if !isContextDeadline(err) {
		t.Errorf("isContextDeadline(wrapped DeadlineExceeded) = false, want true")
	}
}

func TestIsJSONParseError_SyntaxError(t *testing.T) {
	raw := `{"id":"x"`
	var rawObj map[string]interface{}
	err := json.Unmarshal([]byte(raw), &rawObj)
	if err == nil {
		t.Fatalf("expected syntax error")
	}
	if !isJSONParseError(err) {
		t.Errorf("isJSONParseError(syntax error) = false, want true")
	}
}

func TestCollectSegmenterLLMText_Empty(t *testing.T) {
	ch := make(chan llmgateway.Chunk)
	close(ch)
	got := collectSegmenterLLMText(ch)
	if got != "" {
		t.Errorf("collectSegmenterLLMText(empty) = %q, want \"\"", got)
	}
}

func TestCollectSegmenterLLMText_Concatenates(t *testing.T) {
	ch := make(chan llmgateway.Chunk, 3)
	ch <- llmgateway.Chunk{Content: "a"}
	ch <- llmgateway.Chunk{Content: "b"}
	ch <- llmgateway.Chunk{Content: "c"}
	close(ch)
	got := collectSegmenterLLMText(ch)
	if got != "abc" {
		t.Errorf("collectSegmenterLLMText = %q, want %q", got, "abc")
	}
}
