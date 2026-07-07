package interfaces

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	sharederrors "github.com/devrix/devrix/internal/shared/errors"
)

// =====================================================================
// Stage 1 (T01/T02) — happy-path JSON round-trip
//
// PR-A1 codex consensus 2026-07-07: TDD order = happy-path JSON first to
// lock the wire format, then Validate error-path. This file combines both
// stages under the intent_segment.go surface so PR-A1 ships as one test
// group. Code coverage target ≥ 80% per devrix/testing.md.
// =====================================================================

func TestIntentSegment_NewIntentSegment_DefaultsApplied(t *testing.T) {
	s := NewIntentSegment("seg_001", "1+1=几?", IntentSegmentKindDeterministic)
	if s.ID != "seg_001" {
		t.Errorf("ID = %q, want seg_001", s.ID)
	}
	if s.Text != "1+1=几?" {
		t.Errorf("Text = %q, want %q", s.Text, "1+1=几?")
	}
	if s.Kind != IntentSegmentKindDeterministic {
		t.Errorf("Kind = %q, want %q", s.Kind, IntentSegmentKindDeterministic)
	}
	if s.Priority != 50 {
		t.Errorf("Priority default = %d, want 50", s.Priority)
	}
	if s.Confidence != 0.5 {
		t.Errorf("Confidence default = %v, want 0.5", s.Confidence)
	}
}

func TestIntentSegment_JSONRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		in   IntentSegment
	}{
		{
			name: "deterministic-low-priority",
			in: IntentSegment{
				ID:         "seg_a",
				Text:       "1+1=几?",
				Kind:       IntentSegmentKindDeterministic,
				Priority:   10,
				Confidence: 0.95,
			},
		},
		{
			name: "explore-mid",
			in: IntentSegment{
				ID:         "seg_b",
				Text:       "查 devrix 项目结构",
				Kind:       IntentSegmentKindExplore,
				Priority:   50,
				Confidence: 0.7,
			},
		},
		{
			name: "commit-edge-priority-100",
			in: IntentSegment{
				ID:         "seg_c",
				Text:       "deploy this build",
				Kind:       IntentSegmentKindCommit,
				Priority:   100,
				Confidence: 1.0,
			},
		},
		{
			name: "analyze-edge-priority-0",
			in: IntentSegment{
				ID:         "seg_d",
				Text:       "评估 v7 演进风险",
				Kind:       IntentSegmentKindAnalyze,
				Priority:   0,
				Confidence: 0.0,
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b, err := json.Marshal(tc.in)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			var got IntentSegment
			if err := json.Unmarshal(b, &got); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if got != tc.in {
				t.Errorf("round-trip mismatch: got %+v, want %+v", got, tc.in)
			}
		})
	}
}

func TestIsKnownIntentSegmentKind(t *testing.T) {
	for _, k := range []IntentSegmentKind{
		IntentSegmentKindDeterministic,
		IntentSegmentKindExplore,
		IntentSegmentKindCommit,
		IntentSegmentKindAnalyze,
	} {
		if !IsKnownIntentSegmentKind(k) {
			t.Errorf("IsKnownIntentSegmentKind(%q) = false, want true", k)
		}
	}
	notKind := []IntentSegmentKind{"", "garbage", "fast", "FAST", "Deterministic"}
	for _, k := range notKind {
		if IsKnownIntentSegmentKind(k) {
			t.Errorf("IsKnownIntentSegmentKind(%q) = true, want false", k)
		}
	}
}

func TestIntentSegmentSet_NewIntentSegmentSet_WrapsFields(t *testing.T) {
	now := time.Now().UTC()
	segs := []IntentSegment{
		NewIntentSegment("seg_001", "1+1=几?", IntentSegmentKindDeterministic),
		NewIntentSegment("seg_002", "巴黎时区?", IntentSegmentKindDeterministic),
	}
	got := NewIntentSegmentSet("1+1=几? 巴黎时区?", now, segs)
	if got.SourceDirective != "1+1=几? 巴黎时区?" {
		t.Errorf("SourceDirective = %q", got.SourceDirective)
	}
	if !got.DetectedAt.Equal(now) {
		t.Errorf("DetectedAt = %v, want %v", got.DetectedAt, now)
	}
	if len(got.Segments) != 2 {
		t.Errorf("len(Segments) = %d, want 2", len(got.Segments))
	}
}

func TestIntentSegmentSet_JSONRoundTrip(t *testing.T) {
	now := time.Date(2026, 7, 7, 0, 0, 0, 0, time.UTC)
	in := NewIntentSegmentSet("dir ctx", now, []IntentSegment{
		{ID: "s1", Text: "t1", Kind: IntentSegmentKindExplore, Priority: 70, Confidence: 0.9},
		{ID: "s2", Text: "t2", Kind: IntentSegmentKindAnalyze, Priority: 30, Confidence: 0.6},
	})
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out IntentSegmentSet
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.SourceDirective != in.SourceDirective {
		t.Errorf("SourceDirective = %q, want %q", out.SourceDirective, in.SourceDirective)
	}
	if !out.DetectedAt.Equal(in.DetectedAt) {
		t.Errorf("DetectedAt = %v, want %v", out.DetectedAt, in.DetectedAt)
	}
	if len(out.Segments) != len(in.Segments) {
		t.Fatalf("len(Segments) = %d, want %d", len(out.Segments), len(in.Segments))
	}
	for i := range in.Segments {
		if out.Segments[i] != in.Segments[i] {
			t.Errorf("Segments[%d] mismatch", i)
		}
	}
}

// =====================================================================
// Stage 2 (T04) — Validate error-path
//
// Each error must:
//   - return a *sharederrors.SentinelError carrying the canonical ORCH_*_71xx code
//   - pass errors.Is(innerErr) check
//   - carry a non-empty Message
// =====================================================================

func TestIntentSegment_Validate_HappyPath(t *testing.T) {
	s := IntentSegment{
		ID: "seg_ok", Text: "1+1=几?", Kind: IntentSegmentKindDeterministic,
		Priority: 50, Confidence: 0.9,
	}
	if err := s.Validate(); err != nil {
		t.Errorf("Validate happy-path: unexpected %v", err)
	}
}

func TestIntentSegment_Validate_Errors(t *testing.T) {
	type tc struct {
		name    string
		mutator func(*IntentSegment)
		wantErr error // unwrapped sentinel
		wantCode string
	}
	cases := []tc{
		{
			name:     "empty_id",
			mutator:  func(s *IntentSegment) { s.ID = "" },
			wantErr:  ErrIntentSegmentInvalidID,
			wantCode: "ORCH_INTENT_SEGMENT_ID_7114",
		},
		{
			name:     "empty_text",
			mutator:  func(s *IntentSegment) { s.Text = "" },
			wantErr:  ErrIntentSegmentInvalidText,
			wantCode: "ORCH_INTENT_SEGMENT_TEXT_7115",
		},
		{
			name:     "kind_empty",
			mutator:  func(s *IntentSegment) { s.Kind = "" },
			wantErr:  ErrIntentSegmentInvalidKind,
			wantCode: "ORCH_INTENT_SEGMENT_KIND_7116",
		},
		{
			name:     "kind_unknown_string",
			mutator:  func(s *IntentSegment) { s.Kind = "garbage" },
			wantErr:  ErrIntentSegmentInvalidKind,
			wantCode: "ORCH_INTENT_SEGMENT_KIND_7116",
		},
		{
			name:     "kind_collides_with_IntentKind_fast",
			mutator:  func(s *IntentSegment) { s.Kind = "fast" },
			wantErr:  ErrIntentSegmentInvalidKind,
			wantCode: "ORCH_INTENT_SEGMENT_KIND_7116",
		},
		{
			name:     "priority_too_high",
			mutator:  func(s *IntentSegment) { s.Priority = 101 },
			wantErr:  ErrIntentSegmentInvalidPriority,
			wantCode: "ORCH_INTENT_SEGMENT_PRIORITY_7117",
		},
		{
			name:     "priority_too_low",
			mutator:  func(s *IntentSegment) { s.Priority = -1 },
			wantErr:  ErrIntentSegmentInvalidPriority,
			wantCode: "ORCH_INTENT_SEGMENT_PRIORITY_7117",
		},
		{
			name:     "confidence_too_high",
			mutator:  func(s *IntentSegment) { s.Confidence = 1.01 },
			wantErr:  ErrIntentSegmentInvalidConfidence,
			wantCode: "ORCH_INTENT_SEGMENT_CONFIDENCE_7118",
		},
		{
			name:     "confidence_too_low",
			mutator:  func(s *IntentSegment) { s.Confidence = -0.01 },
			wantErr:  ErrIntentSegmentInvalidConfidence,
			wantCode: "ORCH_INTENT_SEGMENT_CONFIDENCE_7118",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := IntentSegment{
				ID: "seg_x", Text: "ok", Kind: IntentSegmentKindAnalyze,
				Priority: 50, Confidence: 0.5,
			}
			c.mutator(&s)
			err := s.Validate()
			if err == nil {
				t.Fatalf("Validate: expected error, got nil")
			}
			if !errors.Is(err, c.wantErr) {
				t.Errorf("errors.Is(%v, %v) = false, want true", err, c.wantErr)
			}
			var sErr *sharederrors.SentinelError
			if !errors.As(err, &sErr) {
				t.Fatalf("error is not *sharederrors.SentinelError: %T", err)
			}
			if sErr.Code != c.wantCode {
				t.Errorf("code = %q, want %q", sErr.Code, c.wantCode)
			}
			if sErr.Message == "" {
				t.Errorf("Message is empty")
			}
		})
	}
}

func TestIntentSegmentSet_Validate_Empty(t *testing.T) {
	s := IntentSegmentSet{}
	err := s.Validate()
	if err == nil {
		t.Fatalf("Validate on empty set: expected error, got nil")
	}
	if !errors.Is(err, ErrIntentSegmentSetEmpty) {
		t.Errorf("errors.Is(err, ErrIntentSegmentSetEmpty) = false")
	}
	var sErr *sharederrors.SentinelError
	if !errors.As(err, &sErr) {
		t.Fatalf("not SentinelError: %T", err)
	}
	if sErr.Code != "ORCH_INTENT_SET_EMPTY_7119" {
		t.Errorf("code = %q, want ORCH_INTENT_SET_EMPTY_7119", sErr.Code)
	}
}

func TestIntentSegmentSet_Validate_PropagatesInnerError(t *testing.T) {
	now := time.Now()
	s := IntentSegmentSet{
		SourceDirective: "ok",
		DetectedAt:      now,
		Segments: []IntentSegment{
			{ID: "good", Text: "ok", Kind: IntentSegmentKindExplore, Priority: 50, Confidence: 0.5},
			{ID: "good2", Text: "ok", Kind: IntentSegmentKindExplore, Priority: -5, Confidence: 0.5}, // bad priority
		},
	}
	err := s.Validate()
	if err == nil {
		t.Fatalf("expected inner error, got nil")
	}
	if !errors.Is(err, ErrIntentSegmentInvalidPriority) {
		t.Errorf("errors.Is did not find ErrIntentSegmentInvalidPriority")
	}
	if !strings.Contains(err.Error(), "segment[1]") {
		t.Errorf("error message should index to segment[1], got: %v", err)
	}
}

func TestIntentSegmentSet_Validate_EmptySourceDirective_DoesNotError(t *testing.T) {
	// Codex consensus 2026-07-07: empty SourceDirective logs slog.Warn but
	// does NOT return an error. This test asserts the absence of an error;
	// the slog call is verified manually (visual check in dev run logs).
	s := IntentSegmentSet{
		SourceDirective: "",
		DetectedAt:      time.Now(),
		Segments: []IntentSegment{
			NewIntentSegment("seg_x", "ok", IntentSegmentKindExplore),
		},
	}
	if err := s.Validate(); err != nil {
		t.Errorf("Validate on empty SourceDirective should NOT error, got: %v", err)
	}
}
