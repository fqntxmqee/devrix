package sessionorchestrator

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	ifaces "github.com/devrix/devrix/internal/layers/orchestration/interfaces"
)

// =====================================================================
// SegmenterDispatcher tests (PR-A2 Q7 ADOPT-WITH-CHANGE)
//
// Coverage targets:
//   - Fast-path: no connective + min length → 1-element set, no LLM call
//   - Fast-path: short message (< min length) → fall through to LLM
//   - Fast-path: contains connective → fall through to LLM
//   - LLM success: returns LLM result
//   - LLM timeout (Q1 ACCEPT): falls back to RuleBased
//   - LLM error: falls back to RuleBased
//   - LLM + RuleBoth: lazy fallback 1-element set
//   - LLM empty (7122): falls back to RuleBased
//   - Config: defaults (800ms / 0.95 / 8)
//   - Lazy fallback: nil dispatcher, empty message
// =====================================================================

// dispatchStub is a controllable IntentSegmenter for dispatcher tests.
// Both LLM and Rule slots in SegmenterDispatcher are typed as
// IntentSegmenter, so this same stub serves both.
type dispatchStub struct {
	calls   int
	seg     ifaces.IntentSegmentSet
	err     error
	timeout time.Duration
}

func (s *dispatchStub) Segment(ctx context.Context, _ SegmentRequest) (ifaces.IntentSegmentSet, error) {
	s.calls++
	if s.timeout > 0 {
		select {
		case <-time.After(s.timeout):
		case <-ctx.Done():
			return ifaces.IntentSegmentSet{}, ctx.Err()
		}
	}
	if s.err != nil {
		return ifaces.IntentSegmentSet{}, s.err
	}
	return s.seg, nil
}

func TestSegmenterDispatcher_FastPath_NoLLMCall(t *testing.T) {
	// Directive is ≥ FastPathMinLength=4 runes AND has no connective.
	llm := &dispatchStub{}
	rule := &dispatchStub{}
	d := NewSegmenterDispatcher(llm, rule, Config{
		LLMTimeout:        800 * time.Millisecond,
		FastPathMinLength: 4,
	})
	set, err := d.Segment(context.Background(), SegmentRequest{
		SessionID: "sess_d1",
		Message:   "查 devrix 项目结构",
	})
	if err != nil {
		t.Fatalf("Segment: %v", err)
	}
	if len(set.Segments) != 1 {
		t.Errorf("fast path: expected 1-element set, got %d", len(set.Segments))
	}
	if llm.calls != 0 {
		t.Errorf("fast path should NOT call LLM, got %d calls", llm.calls)
	}
	if rule.calls != 0 {
		t.Errorf("fast path should NOT call Rule, got %d calls", rule.calls)
	}
}

func TestSegmenterDispatcher_FastPath_SkippedOnShortMessage(t *testing.T) {
	// "1+1" is 3 runes < min length 4 → fall through to LLM
	llm := &dispatchStub{
		seg: ifaces.NewIntentSegmentSet("1+1", time.Now(), []ifaces.IntentSegment{
			ifaces.NewIntentSegment("seg_0", "1+1", ifaces.IntentSegmentKindDeterministic),
		}),
	}
	d := NewSegmenterDispatcher(llm, nil, Config{
		FastPathMinLength: 4,
	})
	_, err := d.Segment(context.Background(), SegmentRequest{
		SessionID: "sess_d2",
		Message:   "1+1",
	})
	if err != nil {
		t.Fatalf("Segment: %v", err)
	}
	if llm.calls != 1 {
		t.Errorf("short message should fall through to LLM, got %d calls", llm.calls)
	}
}

func TestSegmenterDispatcher_FastPath_SkippedOnConnective(t *testing.T) {
	// "查 plan + 看 design" has connective → fall through
	llm := &dispatchStub{
		seg: ifaces.NewIntentSegmentSet("查 plan + 看 design", time.Now(),
			[]ifaces.IntentSegment{
				ifaces.NewIntentSegment("seg_0", "查 plan", ifaces.IntentSegmentKindExplore),
				ifaces.NewIntentSegment("seg_1", "看 design", ifaces.IntentSegmentKindExplore),
			}),
	}
	d := NewSegmenterDispatcher(llm, nil, Config{FastPathMinLength: 4})
	_, err := d.Segment(context.Background(), SegmentRequest{
		SessionID: "sess_d3",
		Message:   "查 plan + 看 design",
	})
	if err != nil {
		t.Fatalf("Segment: %v", err)
	}
	if llm.calls != 1 {
		t.Errorf("connective directive should fall through to LLM, got %d calls", llm.calls)
	}
}

func TestSegmenterDispatcher_LLMTimeout_FallbackToRule(t *testing.T) {
	// LLM times out (exceeds 50ms budget), Rule returns valid set.
	llm := &dispatchStub{timeout: 200 * time.Millisecond}
	rule := &dispatchStub{
		seg: ifaces.NewIntentSegmentSet("查 plan + 看 design", time.Now(),
			[]ifaces.IntentSegment{
				ifaces.NewIntentSegment("seg_0", "查 plan", ifaces.IntentSegmentKindExplore),
				ifaces.NewIntentSegment("seg_1", "看 design", ifaces.IntentSegmentKindExplore),
			}),
	}
	d := NewSegmenterDispatcher(llm, rule, Config{
		LLMTimeout:        50 * time.Millisecond,
		FastPathMinLength: 4,
	})
	set, err := d.Segment(context.Background(), SegmentRequest{
		SessionID: "sess_d4",
		Message:   "查 plan + 看 design",
	})
	if err != nil {
		t.Fatalf("Segment: %v", err)
	}
	if len(set.Segments) != 2 {
		t.Errorf("expected 2-segment set from RuleBased fallback, got %d", len(set.Segments))
	}
	if rule.calls != 1 {
		t.Errorf("Rule should be called once on LLM timeout, got %d", rule.calls)
	}
}

func TestSegmenterDispatcher_LLMError_FallbackToRule(t *testing.T) {
	llm := &dispatchStub{err: errors.New("llm gateway 503")}
	rule := &dispatchStub{
		seg: ifaces.NewIntentSegmentSet("查 plan + 看 design", time.Now(),
			[]ifaces.IntentSegment{
				ifaces.NewIntentSegment("seg_0", "查 plan", ifaces.IntentSegmentKindExplore),
			}),
	}
	d := NewSegmenterDispatcher(llm, rule, Config{FastPathMinLength: 4})
	set, err := d.Segment(context.Background(), SegmentRequest{
		SessionID: "sess_d5",
		Message:   "查 plan + 看 design",
	})
	if err != nil {
		t.Fatalf("Segment: %v", err)
	}
	if len(set.Segments) != 1 {
		t.Errorf("expected Rule's 1-segment set, got %d", len(set.Segments))
	}
}

func TestSegmenterDispatcher_LLMNoSegment_FallbackToRule(t *testing.T) {
	// LLM returns empty set.
	llm := &dispatchStub{seg: ifaces.IntentSegmentSet{}} // empty
	rule := &dispatchStub{
		seg: ifaces.NewIntentSegmentSet("查 plan + 看 design", time.Now(),
			[]ifaces.IntentSegment{
				ifaces.NewIntentSegment("seg_0", "查 plan", ifaces.IntentSegmentKindExplore),
			}),
	}
	d := NewSegmenterDispatcher(llm, rule, Config{FastPathMinLength: 4})
	set, err := d.Segment(context.Background(), SegmentRequest{
		SessionID: "sess_d6",
		Message:   "查 plan + 看 design",
	})
	if err != nil {
		t.Fatalf("Segment: %v", err)
	}
	if len(set.Segments) != 1 {
		t.Errorf("expected Rule's 1-segment fallback, got %d", len(set.Segments))
	}
}

func TestSegmenterDispatcher_BothFail_LazySingleSegment(t *testing.T) {
	llm := &dispatchStub{err: errors.New("llm offline")}
	rule := &dispatchStub{err: errors.New("rule miss too")}
	d := NewSegmenterDispatcher(llm, rule, Config{FastPathMinLength: 4})
	set, err := d.Segment(context.Background(), SegmentRequest{
		SessionID: "sess_d7",
		Message:   "查 plan + 看 design",
	})
	if err != nil {
		t.Fatalf("lazy fallback should NOT return error, got %v", err)
	}
	if len(set.Segments) != 1 {
		t.Errorf("lazy fallback: expected 1-element whole-directive set, got %d", len(set.Segments))
	}
	if set.Segments[0].Text != "查 plan + 看 design" {
		t.Errorf("lazy fallback Text = %q, want %q", set.Segments[0].Text, "查 plan + 看 design")
	}
}

func TestSegmenterDispatcher_NilDispatcher_LazyFallback(t *testing.T) {
	var d *SegmenterDispatcher
	set, err := d.Segment(context.Background(), SegmentRequest{
		SessionID: "sess_d8",
		Message:   "查 devrix",
	})
	if err != nil {
		t.Fatalf("nil dispatcher should not error, got %v", err)
	}
	if len(set.Segments) != 1 {
		t.Errorf("expected 1-element lazy fallback, got %d", len(set.Segments))
	}
}

func TestSegmenterDispatcher_EmptyMessage_LazyFallback(t *testing.T) {
	d := NewSegmenterDispatcher(nil, nil, Config{})
	set, err := d.Segment(context.Background(), SegmentRequest{
		SessionID: "sess_d9",
		Message:   "   ",
	})
	if err != nil {
		t.Fatalf("empty message should not error, got %v", err)
	}
	if len(set.Segments) != 1 {
		t.Errorf("expected 1-element lazy fallback, got %d", len(set.Segments))
	}
}

func TestSegmenterDispatcher_NewConstructor_RejectsBothNil(t *testing.T) {
	d := NewSegmenterDispatcher(nil, nil, Config{})
	if d != nil {
		t.Errorf("NewSegmenterDispatcher(nil, nil) should return nil, got %+v", d)
	}
}

func TestSegmenterDispatcher_ConfigDefaults(t *testing.T) {
	c := Config{}
	if c.llmTimeout() != 800*time.Millisecond {
		t.Errorf("llmTimeout default = %v, want 800ms", c.llmTimeout())
	}
	if c.fastPathConfidence() != 0.95 {
		t.Errorf("fastPathConfidence default = %v, want 0.95", c.fastPathConfidence())
	}
	if c.fastPathMinLength() != 8 {
		t.Errorf("fastPathMinLength default = %d, want 8", c.fastPathMinLength())
	}
}

func TestSegmenterDispatcher_ConfigOverrides(t *testing.T) {
	c := Config{
		LLMTimeout:         200 * time.Millisecond,
		FastPathConfidence: 0.7,
		FastPathMinLength:  3,
	}
	if c.llmTimeout() != 200*time.Millisecond {
		t.Errorf("override llmTimeout = %v, want 200ms", c.llmTimeout())
	}
	if c.fastPathConfidence() != 0.7 {
		t.Errorf("override fastPathConfidence = %v, want 0.7", c.fastPathConfidence())
	}
	if c.fastPathMinLength() != 3 {
		t.Errorf("override fastPathMinLength = %d, want 3", c.fastPathMinLength())
	}
}

func TestSegmenterDispatcher_ConfigNowOverride(t *testing.T) {
	fixed := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	c := Config{Now: func() time.Time { return fixed }}
	if !c.now().Equal(fixed) {
		t.Errorf("now() = %v, want %v", c.now(), fixed)
	}
}

func TestSegmenterDispatcher_ClassifyLLMError(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{fmt.Errorf("wrapped: %w", context.DeadlineExceeded), "llm_timeout"},
		{fmt.Errorf("invalid character 'x' looking for beginning of value"), "llm_invalid_response"},
		{errors.New("random error"), "llm_error"},
		{nil, "unknown"},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			if got := classifyLLMError(tc.err); got != tc.want {
				t.Errorf("classifyLLMError(%v) = %q, want %q", tc.err, got, tc.want)
			}
		})
	}
}

func TestSegmenterDispatcher_LLMSuccess_FastPathNoConflict(t *testing.T) {
	// "1+1" is 3 runes (< min length 4) → fast-path skipped → LLM is
	// called. The LLM stub returns 1 element, which the dispatcher
	// passes through.
	llm := &dispatchStub{
		seg: ifaces.NewIntentSegmentSet("1+1", time.Now(),
			[]ifaces.IntentSegment{
				ifaces.NewIntentSegment("seg_0", "1+1", ifaces.IntentSegmentKindDeterministic),
			}),
	}
	d := NewSegmenterDispatcher(llm, nil, Config{FastPathMinLength: 4})
	set, err := d.Segment(context.Background(), SegmentRequest{
		SessionID: "sess_d10",
		Message:   "1+1",
	})
	if err != nil {
		t.Fatalf("Segment: %v", err)
	}
	if llm.calls != 1 {
		t.Errorf("expected 1 LLM call, got %d", llm.calls)
	}
	if len(set.Segments) != 1 {
		t.Errorf("expected 1-segment LLM set, got %d", len(set.Segments))
	}
}
