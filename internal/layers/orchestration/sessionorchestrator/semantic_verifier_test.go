package sessionorchestrator

import (
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/interfaces"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/types"
)

// TestLooksLikeTemplateMimicry_Disabled confirms the gate short-circuits
// when Enabled=false. The hotfix default — preserves code-only behavior.
func TestLooksLikeTemplateMimicry_Disabled(t *testing.T) {
	cfg := DefaultSemanticSimilarityConfig() // Enabled=false
	cur := strings.Repeat("the quick brown fox jumps over the lazy dog. ", 30)
	if looksLikeTemplateMimicry(cur, []string{cur}, cfg) {
		t.Fatal("expected false when Enabled=false; got true (feature flag leak)")
	}
}

// TestLooksLikeTemplateMimicry_NoPriors confirms no mimicry possible when
// priors is empty (round 1).
func TestLooksLikeTemplateMimicry_NoPriors(t *testing.T) {
	cfg := SemanticSimilarityConfig{Enabled: true, MinSimilarityForVerify: 0.85, MinArtifactChars: 100}
	cur := "this is a long enough artifact summary to pass the minimum chars check."
	if looksLikeTemplateMimicry(cur, nil, cfg) {
		t.Fatal("expected false on empty priors")
	}
	if looksLikeTemplateMimicry(cur, []string{}, cfg) {
		t.Fatal("expected false on empty priors slice")
	}
}

// TestLooksLikeTemplateMimicry_TooShort confirms MinArtifactChars short-circuits.
func TestLooksLikeTemplateMimicry_TooShort(t *testing.T) {
	cfg := SemanticSimilarityConfig{Enabled: true, MinSimilarityForVerify: 0.85, MinArtifactChars: 100}
	cur := "short" // 5 chars, well below 100
	priors := []string{"short"}
	if looksLikeTemplateMimicry(cur, priors, cfg) {
		t.Fatal("expected false when artifact is shorter than MinArtifactChars")
	}
}

// TestLooksLikeTemplateMimicry_IdenticalSummary confirms two near-identical
// summaries DO trigger mimicry (the production failure mode).
func TestLooksLikeTemplateMimicry_IdenticalSummary(t *testing.T) {
	cfg := SemanticSimilarityConfig{Enabled: true, MinSimilarityForVerify: 0.85, MinArtifactChars: 100}
	// Build two summaries that share ≥85% of their token set. Use the
	// same "findings_json" template with a small variation to mimic
	// template-mimicry.
	tmpl := `<findings_json>{"severity":"info","summary":"No issues found in the d2 kernel observation layer","files_reviewed":["internal/layers/contextengine/d2_kernel.go"],"scope":"out-of-scope"}</findings_json>`
	if !looksLikeTemplateMimicry(tmpl, []string{tmpl}, cfg) {
		t.Fatal("expected true on identical template (Jaccard=1.0 ≥ 0.85)")
	}
}

// TestLooksLikeTemplateMimicry_SubstantiveChange confirms summaries that
// differ substantively (not template mimicry) skip the LLM call.
func TestLooksLikeTemplateMimicry_SubstantiveChange(t *testing.T) {
	cfg := SemanticSimilarityConfig{Enabled: true, MinSimilarityForVerify: 0.85, MinArtifactChars: 100}
	prior := "The d2 kernel returns ErrStaleObservation when the cached value is older than 5 minutes; client should retry with backoff."
	cur := "I have a question about how to read the devrix config file — please explain the structure of the YAML schema."
	if looksLikeTemplateMimicry(cur, []string{prior}, cfg) {
		t.Fatal("expected false when summaries are substantively different")
	}
}

// TestDefaultSemanticSimilarityConfig verifies the production defaults.
// Regression guard: the hotfix path must remain default-OFF until
// production validation flips it on.
func TestDefaultSemanticSimilarityConfig(t *testing.T) {
	cfg := DefaultSemanticSimilarityConfig()
	if cfg.Enabled {
		t.Fatal("default config must have Enabled=false (hotfix path)")
	}
	if cfg.MinSimilarityForVerify != interfaces.DefaultInterceptThreshold {
		t.Errorf("MinSimilarityForVerify=%v want=%v", cfg.MinSimilarityForVerify, interfaces.DefaultInterceptThreshold)
	}
	if cfg.MinArtifactChars <= 0 {
		t.Errorf("MinArtifactChars must be positive, got %d", cfg.MinArtifactChars)
	}
}

// TestExtractDecisionAndHint_NoPrefix confirms no decision prefix → ("", "").
func TestExtractDecisionAndHint_NoPrefix(t *testing.T) {
	v := workmodel.Verdict{Reason: "semantic_verify: looks like template-mimicry"}
	d, h := extractDecisionAndHint(v)
	if d != "" || h != "" {
		t.Errorf("expected empty decision+hint, got decision=%q hint=%q", d, h)
	}
}

// TestExtractDecisionAndHint_StopPrefix confirms "[decision=stop] ..."
// yields ("stop", cleaned-reason).
func TestExtractDecisionAndHint_StopPrefix(t *testing.T) {
	v := workmodel.Verdict{Reason: "[decision=stop] semantic_verify: template-mimicry confirmed"}
	d, h := extractDecisionAndHint(v)
	if d != "stop" {
		t.Errorf("expected decision=stop, got %q", d)
	}
	if h != "semantic_verify: template-mimicry confirmed" {
		t.Errorf("expected cleaned reason, got %q", h)
	}
}

// TestExtractDecisionAndHint_RetryPrefix confirms "[decision=retry] ..."
// yields ("retry", cleaned-reason).
func TestExtractDecisionAndHint_RetryPrefix(t *testing.T) {
	v := workmodel.Verdict{Reason: "[decision=retry] semantic_verify: missed the user's actual question"}
	d, h := extractDecisionAndHint(v)
	if d != "retry" {
		t.Errorf("expected decision=retry, got %q", d)
	}
	if h != "semantic_verify: missed the user's actual question" {
		t.Errorf("expected cleaned reason, got %q", h)
	}
}

// TestExtractDecisionAndHint_MalformedPrefix confirms a malformed
// "[decision=..." with no closing bracket → ("", "") (fail-open).
func TestExtractDecisionAndHint_MalformedPrefix(t *testing.T) {
	v := workmodel.Verdict{Reason: "[decision=stop but no closing bracket"}
	d, h := extractDecisionAndHint(v)
	if d != "" || h != "" {
		t.Errorf("expected empty on malformed prefix, got decision=%q hint=%q", d, h)
	}
}

// TestSemanticStagnationVerdict_KindAndSourceID confirms the stagnation
// helper produces a VerdictFail with a stable SourceID.
func TestSemanticStagnationVerdict_KindAndSourceID(t *testing.T) {
	v := semanticStagnationVerdict(SemanticVerifyRequest{
		ItemID:           "wi-abc",
		ArtifactSummary:  "a b c d e f g h",
		PriorRoundSummaries: []string{"a b c d e f g h"},
	}, DefaultSemanticSimilarityConfig())
	if v.Kind != types.VerdictFail {
		t.Errorf("expected VerdictFail, got %v", v.Kind)
	}
	if v.SourceID != "semantic_stagnation:wi-abc" {
		t.Errorf("unexpected SourceID: %q", v.SourceID)
	}
	if v.Confidence <= 0 || v.Confidence > 1 {
		t.Errorf("Confidence out of range: %v", v.Confidence)
	}
}
