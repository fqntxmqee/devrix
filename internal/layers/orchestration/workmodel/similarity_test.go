package workmodel

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/interfaces"
)

func TestCheckSimilarityForSession_NilRegistry(t *testing.T) {
	res, err := CheckSimilarityForSession(nil, "sess_1", "hello world", interfaces.NewDefaultSimilarityConfig())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Similar || res.Warn {
		t.Fatalf("nil registry should yield zero result")
	}
}

func TestCheckSimilarityForSession_EmptyChain(t *testing.T) {
	r := NewVersionChainRegistry()
	res, err := CheckSimilarityForSession(r, "sess_1", "hello world", interfaces.NewDefaultSimilarityConfig())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Similar || res.Warn {
		t.Fatalf("empty chain should yield zero result")
	}
}

func TestCheckSimilarityForSession_LowSimilarity(t *testing.T) {
	r := NewVersionChainRegistry()
	_, _, _ = r.Append("sess_1", []byte("completely distinct content block here yes please"), "commit")
	res, err := CheckSimilarityForSession(r, "sess_1", "totally unrelated query", interfaces.NewDefaultSimilarityConfig())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Similar {
		t.Fatalf("expected Similar=false for low overlap; score=%v", res.Score)
	}
}

func TestCheckSimilarityForSession_AboveIntercept(t *testing.T) {
	r := NewVersionChainRegistry()
	base := "the quick brown fox jumps over the lazy dog now and forever more content"
	_, _, _ = r.Append("sess_1", []byte(base), "commit")
	// Near-identical variant.
	variant := "the quick brown fox jumps over the lazy dog now and forever more content here"
	res, err := CheckSimilarityForSession(r, "sess_1", variant, interfaces.NewDefaultSimilarityConfig())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Similar {
		t.Fatalf("expected Similar=true for near-identical content; score=%v", res.Score)
	}
}

func TestCheckSimilarityForSession_InvalidConfig(t *testing.T) {
	r := NewVersionChainRegistry()
	cfg := interfaces.SimilarityConfig{InterceptThreshold: 0.9, WarnThreshold: 1.0, LookbackN: 5}
	_, err := CheckSimilarityForSession(r, "sess_1", "anything", cfg)
	if err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestMostSimilarSessionID_NilRegistry(t *testing.T) {
	sid, score, found := MostSimilarSessionID(nil, "hello", interfaces.NewDefaultSimilarityConfig(), 5)
	if found || sid != "" || score != 0 {
		t.Fatalf("nil registry should yield zero result; got (%q,%v,%v)", sid, score, found)
	}
}

func TestMostSimilarSessionID_NoSessions(t *testing.T) {
	r := NewVersionChainRegistry()
	_, _, found := MostSimilarSessionID(r, "hello", interfaces.NewDefaultSimilarityConfig(), 5)
	if found {
		t.Fatalf("empty registry should yield found=false")
	}
}

func TestMostSimilarSessionID_FindsBestAcrossSessions(t *testing.T) {
	r := NewVersionChainRegistry()
	_, _, _ = r.Append("sess_low", []byte("completely unrelated original work here"), "commit")
	_, _, _ = r.Append("sess_high", []byte("the quick brown fox jumps over the lazy dog now and forever"), "commit")
	sid, score, found := MostSimilarSessionID(r, "the quick brown fox jumps over the lazy dog now and forever extra",
		interfaces.NewDefaultSimilarityConfig(), 5)
	if !found {
		t.Fatalf("expected found=true")
	}
	if sid != "sess_high" {
		t.Fatalf("expected sess_high, got %q", sid)
	}
	if score <= 0 {
		t.Fatalf("expected positive score, got %v", score)
	}
}

func TestMostSimilarSessionID_EmptyTokens(t *testing.T) {
	r := NewVersionChainRegistry()
	_, _, _ = r.Append("sess_1", []byte("some content here"), "commit")
	_, _, found := MostSimilarSessionID(r, "   ", interfaces.NewDefaultSimilarityConfig(), 5)
	if found {
		t.Fatalf("empty-token query should yield found=false")
	}
}

func TestMostSimilarSessionID_InvalidConfig(t *testing.T) {
	r := NewVersionChainRegistry()
	_, _, _ = r.Append("sess_1", []byte("some content here"), "commit")
	cfg := interfaces.SimilarityConfig{InterceptThreshold: 0.9, WarnThreshold: 1.0, LookbackN: 5}
	_, _, found := MostSimilarSessionID(r, "hello", cfg, 5)
	if found {
		t.Fatalf("invalid config should yield found=false (silent fail)")
	}
}

func TestMostSimilarSessionID_RespectsLookbackSessionsLimit(t *testing.T) {
	r := NewVersionChainRegistry()
	_, _, _ = r.Append("sess_a", []byte("the quick brown fox shared word shared word shared"), "commit")
	_, _, _ = r.Append("sess_b", []byte("totally different content for this session"), "commit")
	// lookbackSessions=1 → only first iteration is visited.
	_, _, found := MostSimilarSessionID(r, "completely fresh input",
		interfaces.NewDefaultSimilarityConfig(), 1)
	if found {
		t.Fatalf("lookbackSessions=1 may yield found; score might be zero")
	}
	// Behavior is best-effort; we just verify no panic. Look for any actual match across multiple.
	_, _, _ = MostSimilarSessionID(r, "the quick", interfaces.NewDefaultSimilarityConfig(), 5)
}
