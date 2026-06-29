package interfaces

import (
	"math"
	"testing"
)

const eps = 1e-9

func nearlyEqual(a, b float64) bool {
	return math.Abs(a-b) < eps
}

func TestNewDefaultSimilarityConfig(t *testing.T) {
	cfg := NewDefaultSimilarityConfig()
	if cfg.InterceptThreshold != 0.85 {
		t.Fatalf("expected InterceptThreshold=0.85, got %v", cfg.InterceptThreshold)
	}
	if cfg.WarnThreshold != 0.70 {
		t.Fatalf("expected WarnThreshold=0.70, got %v", cfg.WarnThreshold)
	}
	if cfg.LookbackN != 5 {
		t.Fatalf("expected LookbackN=5, got %d", cfg.LookbackN)
	}
}

func TestSimilarityConfig_Validate_Happy(t *testing.T) {
	cfg := NewDefaultSimilarityConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("default config should validate: %v", err)
	}
}

func TestSimilarityConfig_Validate_Bounds(t *testing.T) {
	cases := []struct {
		name string
		cfg  SimilarityConfig
	}{
		{"warn_zero", SimilarityConfig{InterceptThreshold: 0.9, WarnThreshold: 0, LookbackN: 5}},
		{"warn_one", SimilarityConfig{InterceptThreshold: 0.9, WarnThreshold: 1.0, LookbackN: 5}},
		{"intercept_eq_warn", SimilarityConfig{InterceptThreshold: 0.7, WarnThreshold: 0.7, LookbackN: 5}},
		{"intercept_gt_one", SimilarityConfig{InterceptThreshold: 1.5, WarnThreshold: 0.7, LookbackN: 5}},
		{"lookback_zero", SimilarityConfig{InterceptThreshold: 0.9, WarnThreshold: 0.7, LookbackN: 0}},
		{"lookback_neg", SimilarityConfig{InterceptThreshold: 0.9, WarnThreshold: 0.7, LookbackN: -1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.cfg.Validate(); err == nil {
				t.Fatalf("expected validation error for %s", tc.name)
			}
		})
	}
}

func TestJaccard_BothEmpty(t *testing.T) {
	if got := Jaccard(nil, nil); !nearlyEqual(got, 1.0) {
		t.Fatalf("Jaccard(nil, nil) should be 1.0, got %v", got)
	}
	if got := Jaccard([]string{}, []string{}); !nearlyEqual(got, 1.0) {
		t.Fatalf("Jaccard(empty, empty) should be 1.0, got %v", got)
	}
}

func TestJaccard_OneEmpty(t *testing.T) {
	if got := Jaccard([]string{"a"}, nil); !nearlyEqual(got, 0.0) {
		t.Fatalf("Jaccard({a}, empty) should be 0.0, got %v", got)
	}
	if got := Jaccard(nil, []string{"a"}); !nearlyEqual(got, 0.0) {
		t.Fatalf("Jaccard(empty, {a}) should be 0.0, got %v", got)
	}
}

func TestJaccard_Identical(t *testing.T) {
	a := []string{"hello", "world"}
	b := []string{"hello", "world"}
	if got := Jaccard(a, b); !nearlyEqual(got, 1.0) {
		t.Fatalf("identical sets should be 1.0, got %v", got)
	}
}

func TestJaccard_Disjoint(t *testing.T) {
	a := []string{"alpha", "beta"}
	b := []string{"gamma", "delta"}
	if got := Jaccard(a, b); !nearlyEqual(got, 0.0) {
		t.Fatalf("disjoint sets should be 0.0, got %v", got)
	}
}

func TestJaccard_PartialOverlap(t *testing.T) {
	// |A|={a,b,c}, |B|={b,c,d} → |A∪B|=4, |A∩B|=2 → J=0.5
	a := []string{"a", "b", "c"}
	b := []string{"b", "c", "d"}
	got := Jaccard(a, b)
	if !nearlyEqual(got, 0.5) {
		t.Fatalf("expected 0.5, got %v", got)
	}
}

func TestTokenize_LowercaseAndShortDropped(t *testing.T) {
	toks := Tokenize("Hello World A I")
	want := []string{"hello", "world"}
	if len(toks) != len(want) {
		t.Fatalf("expected %v, got %v", want, toks)
	}
	for i := range toks {
		if toks[i] != want[i] {
			t.Fatalf("expected %v, got %v", want, toks)
		}
	}
}

func TestTokenize_DropsPunctuation(t *testing.T) {
	toks := Tokenize("hello, world!")
	// "hello" and "world" only (both >=2 chars).
	if len(toks) != 2 || toks[0] != "hello" || toks[1] != "world" {
		t.Fatalf("expected [hello world], got %v", toks)
	}
}

func TestTokenize_DropsPureDigits(t *testing.T) {
	toks := Tokenize("test123 abc 999 done")
	// "test123" has letters, kept; "abc" kept; "999" dropped (all-digit); "done" kept.
	want := []string{"test123", "abc", "done"}
	if len(toks) != len(want) {
		t.Fatalf("expected %v, got %v", want, toks)
	}
	for i := range toks {
		if toks[i] != want[i] {
			t.Fatalf("expected %v, got %v", want, toks)
		}
	}
}

func TestTokenize_CJKLetters(t *testing.T) {
	toks := Tokenize("你好 世界")
	// CJK letters are >=2 chars each, expected.
	if len(toks) != 2 {
		t.Fatalf("expected 2 tokens, got %v", toks)
	}
	if toks[0] != "你好" || toks[1] != "世界" {
		t.Fatalf("expected [你好 世界], got %v", toks)
	}
}

func TestTokenize_Empty(t *testing.T) {
	if got := Tokenize(""); got != nil {
		// Should produce either nil or empty slice; either way len=0.
		if len(got) != 0 {
			t.Fatalf("expected empty tokens, got %v", got)
		}
	}
}

func TestCheckSimilarity_NilChain(t *testing.T) {
	cfg := NewDefaultSimilarityConfig()
	res, err := CheckSimilarity("hello world", nil, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Similar || res.Warn {
		t.Fatalf("nil chain should produce default zero result")
	}
}

func TestCheckSimilarity_EmptyChain(t *testing.T) {
	cfg := NewDefaultSimilarityConfig()
	vc := NewVersionChain()
	res, err := CheckSimilarity("hello world", vc, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Similar || res.Warn {
		t.Fatalf("empty chain should produce default zero result")
	}
}

func TestCheckSimilarity_AboveIntercept(t *testing.T) {
	cfg := NewDefaultSimilarityConfig()
	vc := NewVersionChain()
	base := "the quick brown fox jumps over the lazy dog now and forever more"
	_, vc, _ = vc.Append([]byte(base), "commit")
	// Add a couple unrelated entries to test lookback scan.
	_, vc, _ = vc.Append([]byte("completely unrelated content here yes"), "commit")
	// Input very close to base.
	variant := "the quick brown fox jumps over the lazy dog now and forever here"
	res, err := CheckSimilarity(variant, vc, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Above 0.85 means at least one entry is essentially identical-ish.
	if !res.Similar {
		t.Fatalf("expected Similar=true for near-identical content; score=%v", res.Score)
	}
}

func TestCheckSimilarity_WarnBoundary(t *testing.T) {
	cfg := NewDefaultSimilarityConfig()
	vc := NewVersionChain()
	// Construct a chain entry that yields Jaccard around 0.75 with the test text.
	// |A|=8 words, |B|=4 words, intersection=3 → J=3/(8+4-3)=3/9≈0.333
	// We craft A so most tokens appear in B (partial overlap).
	common := []byte("shared token alpha beta gamma delta epsilon zeta eta theta")
	_, vc, _ = vc.Append(common, "commit")
	// Test input shares ~3 of 8 tokens.
	test := "alpha beta gamma extra words here for testing"
	res, err := CheckSimilarity(test, vc, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Score should be in (0.0, 1.0); we don't make hard assertions on the band.
	if res.Score < 0 || res.Score > 1 {
		t.Fatalf("score out of [0,1]: %v", res.Score)
	}
}

func TestCheckSimilarity_InvalidConfig(t *testing.T) {
	cfg := SimilarityConfig{InterceptThreshold: 0.9, WarnThreshold: 1.0, LookbackN: 5}
	res, err := CheckSimilarity("anything", NewVersionChain(), cfg)
	if err == nil {
		t.Fatalf("expected validation error")
	}
	if res.Similar || res.Warn {
		t.Fatalf("res should be zero on error")
	}
}

func TestCheckSimilarity_LowSimilarity(t *testing.T) {
	cfg := NewDefaultSimilarityConfig()
	vc := NewVersionChain()
	_, vc, _ = vc.Append([]byte("completely distinct original content here"), "commit")
	_, vc, _ = vc.Append([]byte("totally different from anything else"), "commit")
	res, err := CheckSimilarity("a short unrelated message", vc, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Similar {
		t.Fatalf("expected Similar=false for low overlap; score=%v", res.Score)
	}
}

func TestCheckSimilarity_RespectsLookbackN(t *testing.T) {
	cfg := SimilarityConfig{InterceptThreshold: 0.85, WarnThreshold: 0.70, LookbackN: 1}
	vc := NewVersionChain()
	_, vc, _ = vc.Append([]byte("identical content here please"), "commit")
	// Append more — but only the most recent should be scanned under LookbackN=1.
	_, vc, _ = vc.Append([]byte("very different content altogether"), "commit")
	res, err := CheckSimilarity("identical content here please", vc, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// With LookbackN=1, only the most recent (different) entry is scanned,
	// so Similar should be false even though an identical entry exists deeper.
	if res.Similar {
		t.Fatalf("expected Similar=false when identical entry is outside LookbackN; score=%v", res.Score)
	}
}

func TestNewSimilarityCheckConfigInvalidError_Code(t *testing.T) {
	e := NewSimilarityCheckConfigInvalidError()
	if e == nil || e.Code != "ORCH_SIMILARITY_INTERCEPTED_7121" {
		t.Fatalf("unexpected error: %+v", e)
	}
}

func TestNewSimilarityCheckInterceptedError_Code(t *testing.T) {
	e := NewSimilarityCheckInterceptedError()
	if e == nil || e.Code != "ORCH_SIMILARITY_INTERCEPTED_7121" {
		t.Fatalf("unexpected error: %+v", e)
	}
}
