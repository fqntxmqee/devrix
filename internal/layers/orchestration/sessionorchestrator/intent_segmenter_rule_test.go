package sessionorchestrator

import (
	"context"
	"strings"
	"testing"

	ifaces "github.com/devrix/devrix/internal/layers/orchestration/interfaces"
)

// =====================================================================
// RuleBasedSegmenter tests (PR-A2 Q7 ADOPT-WITH-CHANGE: split test files per impl)
//
// Coverage targets:
//   - Single-intent Chinese (Q4 ACCEPT): no connective, returns 1-element lazy fallback
//   - Single-intent English (Q4 ADOPT-WITH-CHANGE): English connectives fall through
//     to lazy fallback (handled by LLM in Dispatcher, not silently degraded)
//   - Multi-intent Chinese: +, 另外, 并且, 还有, 然后, 顺便, ;
//   - Multi-intent with comma + connective
//   - Multi-intent with ? + connective
//   - Lazy fallback paths: nil receiver, empty message, no hit, split empty
// =====================================================================

func TestRuleBasedSegmenter_SingleIntent_Chinese(t *testing.T) {
	r := NewRuleBasedSegmenter()
	set, err := r.Segment(context.Background(), SegmentRequest{
		SessionID: "sess_r1",
		Message:   "查 devrix 项目结构",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Single intent → lazy fallback (1-element whole-directive)
	if len(set.Segments) != 1 {
		t.Errorf("len(Segments) = %d, want 1 (lazy fallback)", len(set.Segments))
	}
	if set.Segments[0].Text != "查 devrix 项目结构" {
		t.Errorf("Text = %q, want %q", set.Segments[0].Text, "查 devrix 项目结构")
	}
	if set.SourceDirective != "查 devrix 项目结构" {
		t.Errorf("SourceDirective = %q", set.SourceDirective)
	}
}

func TestRuleBasedSegmenter_SingleIntent_English_FallsThrough(t *testing.T) {
	// Q4 ADOPT-WITH-CHANGE: English connectives fall through to lazy fallback.
	// Dispatcher will pick this up and route to LLM, NOT silently degrade.
	r := NewRuleBasedSegmenter()
	set, err := r.Segment(context.Background(), SegmentRequest{
		SessionID: "sess_r2",
		Message:   "deploy this build and run the tests",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(set.Segments) != 1 {
		t.Errorf("English single-intent with English connective: expected 1-element lazy fallback, got %d segments",
			len(set.Segments))
	}
	if set.Segments[0].ID != "seg_0" {
		t.Errorf("ID = %q, want seg_0", set.Segments[0].ID)
	}
}

func TestRuleBasedSegmenter_MultiIntent_PlusConjunctive(t *testing.T) {
	r := NewRuleBasedSegmenter()
	set, err := r.Segment(context.Background(), SegmentRequest{
		SessionID: "sess_r3",
		Message:   "查 devrix 架构 + 看 plan 文件",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(set.Segments) != 2 {
		t.Fatalf("len(Segments) = %d, want 2 (split on +)", len(set.Segments))
	}
	if set.Segments[0].Text != "查 devrix 架构" {
		t.Errorf("Segments[0].Text = %q, want %q", set.Segments[0].Text, "查 devrix 架构")
	}
	if set.Segments[1].Text != "看 plan 文件" {
		t.Errorf("Segments[1].Text = %q, want %q", set.Segments[1].Text, "看 plan 文件")
	}
}

func TestRuleBasedSegmenter_MultiIntent_Lingwai(t *testing.T) {
	r := NewRuleBasedSegmenter()
	set, err := r.Segment(context.Background(), SegmentRequest{
		SessionID: "sess_r4",
		Message:   "查 devrix 架构? 另外 看 plan 文件",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(set.Segments) < 2 {
		t.Errorf("len(Segments) = %d, want >=2 (split on ? 另外)", len(set.Segments))
	}
	// Verify the segments capture both halves
	allText := strings.Join(
		func() []string {
			out := make([]string, len(set.Segments))
			for i, s := range set.Segments {
				out[i] = s.Text
			}
			return out
		}(), "|",
	)
	if !strings.Contains(allText, "查 devrix 架构") {
		t.Errorf("segments should include '查 devrix 架构', got %q", allText)
	}
	if !strings.Contains(allText, "看 plan 文件") {
		t.Errorf("segments should include '看 plan 文件', got %q", allText)
	}
}

func TestRuleBasedSegmenter_MultiIntent_CommaList(t *testing.T) {
	// "另外" with comma prefix: "X, 另外 Y"
	r := NewRuleBasedSegmenter()
	set, err := r.Segment(context.Background(), SegmentRequest{
		SessionID: "sess_r5",
		Message:   "查 plan, 另外 看 design",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(set.Segments) < 2 {
		t.Errorf("len(Segments) = %d, want >=2 (split on , 另外)", len(set.Segments))
	}
}

func TestRuleBasedSegmenter_MultiIntent_Bingqie(t *testing.T) {
	r := NewRuleBasedSegmenter()
	set, err := r.Segment(context.Background(), SegmentRequest{
		SessionID: "sess_r6",
		Message:   "看 plan 文件 并且 看 design 文件",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(set.Segments) < 2 {
		t.Errorf("len(Segments) = %d, want >=2 (split on 并且)", len(set.Segments))
	}
}

func TestRuleBasedSegmenter_LazyFallback_NilReceiver(t *testing.T) {
	var r *RuleBasedSegmenter
	set, err := r.Segment(context.Background(), SegmentRequest{
		SessionID: "sess_r7",
		Message:   "1+1=几?",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(set.Segments) != 1 {
		t.Errorf("nil receiver: expected 1-element lazy fallback, got %d", len(set.Segments))
	}
}

func TestRuleBasedSegmenter_LazyFallback_EmptyMessage(t *testing.T) {
	r := NewRuleBasedSegmenter()
	set, err := r.Segment(context.Background(), SegmentRequest{
		SessionID: "sess_r8",
		Message:   "   ",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(set.Segments) != 1 {
		t.Errorf("empty message: expected 1-element lazy fallback, got %d", len(set.Segments))
	}
	if set.Segments[0].Text != "   " {
		t.Errorf("Text = %q, want the whitespace input preserved", set.Segments[0].Text)
	}
}

func TestRuleBasedSegmenter_LazyFallback_NoHit(t *testing.T) {
	// No connective pattern, no min length conflict → lazy fallback
	r := NewRuleBasedSegmenter()
	set, err := r.Segment(context.Background(), SegmentRequest{
		SessionID: "sess_r9",
		Message:   "1+1=几?",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(set.Segments) != 1 {
		t.Errorf("no connective: expected 1-element lazy fallback, got %d", len(set.Segments))
	}
}

func TestRuleBasedSegmenter_ClassifyKind(t *testing.T) {
	cases := []struct {
		msg  string
		want ifaces.IntentSegmentKind
	}{
		{"1+1=几?", ifaces.IntentSegmentKindDeterministic},
		{"what is X?", ifaces.IntentSegmentKindDeterministic},
		{"查 plan 文件", ifaces.IntentSegmentKindExplore},
		{"list the files", ifaces.IntentSegmentKindExplore},
		{"deploy the build", ifaces.IntentSegmentKindCommit},
		{"改 devrix.yaml", ifaces.IntentSegmentKindCommit},
	}
	for _, tc := range cases {
		t.Run(tc.msg, func(t *testing.T) {
			got := classifyFastPath(tc.msg)
			if got != tc.want {
				t.Errorf("classifyFastPath(%q) = %q, want %q", tc.msg, got, tc.want)
			}
		})
	}
}
