package sessionorchestrator

import (
	"context"
	"strings"

	"github.com/devrix/devrix/internal/layers/orchestration/interfaces"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/types"
)

// SemanticVerifier asks the LLM to judge whether a round's ArtifactSummary
// actually answers the user's original question — as opposed to a templated
// re-emission of the deliverable envelope (DM-20260706-006 root cause).
//
// The MUPS pipeline already has a code-based Verdict in `verifyArtifact`
// (5 detector + decision table, ~5ms, no LLM). That code path is correct
// for syntax-shape checks: nil artifact, max_iters + tool_calls, execute
// fail, side-effect rolled-back, side-effect uncertain. But it CANNOT
// detect "the LLM finished with status=final_answer but the content is a
// <findings_json> template that mimics the previous turn's envelope
// without addressing the user's question". That requires semantic
// judgement, which is what this interface provides.
//
// DM-20260706-006 (Semantic Convergence):
//
//   - The default code-based verdict rubber-stamps the round as Pass
//     because the LLM did not Error out, did not hit max_iters, and did
//     not roll back side-effects. The session loop then re-runs the
//     focus item and the LLM re-emits the same template → 20 rounds ×
//     ~67s = 1340s before /stop.
//   - This interface lets ItemPipelineRunner ask "did the LLM actually
//     answer?" via a low-cost semantic verifier call. When the verifier
//     returns VerdictFail (or Partial with reason "template_mimicry"),
//     the Decide node forces SpawnNone so the loop terminates.
//
// The interface lives in sessionorchestrator (not orchtypes) because:
//
//   - It depends on workmodel.Verdict which orchtypes does not import
//     (orchtypes is the cross-package boundary; workmodel is downstream).
//   - The default implementation DefaultSemanticVerifier is wired only
//     in production ItemPipelineRunner; tests inject stubs.
//
// Optional dependency: ItemPipelineRunner.SemanticVerifier may be nil.
// When nil, the pipeline falls back to the code-based verdict (no
// behavioral change for callers that don't wire it — preserves the
// hotfix-path principle from feedback-devrix-bugfix-skip-openspec).
type SemanticVerifier interface {
	// VerifySemantically judges whether artifactSummary actually addresses
	// the user's original question. The implementation may consult an LLM
	// (production DefaultSemanticVerifier) or use a heuristic (test stub).
	//
	// Inputs:
	//   - req.SessionID / ItemID: for tracing/Jaeger span attribution.
	//   - req.UserOriginalQuestion: the directive the user typed. This is
	//     the un-enriched directive — NOT the augmented llmDirective that
	//     carries <deliverable_contract> / <deliverable_format> hints. The
	//     verifier compares answer↔question semantic alignment.
	//   - req.ArtifactSummary: round.ArtifactSummary (LLM's final answer).
	//   - req.PriorRoundSummaries: last N rounds' summaries for stagnation
	//     detection. Empty on the first round of a focus.
	//   - req.CodeBasedVerdict: the existing verifyArtifact result. The
	//     verifier may use this as a fast-path skip (if code says Fail,
	//     don't bother with LLM).
	//
	// Output:
	//   - returned Verdict overrides codeBasedVerdict when Kind != Pass.
	//     When Kind == Pass the verifier agrees with the code path and
	//     the pipeline keeps the original verdict + spawn policy.
	//   - error is non-nil ONLY on infrastructure failures (LLM call
	//     timeout, network). Caller falls back to codeBasedVerdict.
	VerifySemantically(ctx context.Context, req SemanticVerifyRequest) (workmodel.Verdict, error)
}

// SemanticVerifyRequest carries the inputs needed for one semantic
// verification call. See SemanticVerifier docs.
type SemanticVerifyRequest struct {
	SessionID           string
	ItemID              string
	RoundNo             int
	UserOriginalQuestion string
	ArtifactSummary     string
	PriorRoundSummaries []string // last N rounds for stagnation signal
	CodeBasedVerdict    workmodel.Verdict
}

// SemanticSimilarityConfig controls when the ItemPipelineRunner should
// even CALL the SemanticVerifier — gating LLM cost on a cheap Jaccard
// pre-check (DM-20260706-006 token-cost concern).
//
// ItemPipelineRunner invokes SemanticVerifier only when:
//
//   - PriorRoundSummaries is non-empty (round > 1 for this focus), AND
//   - max(Jaccard(ArtifactSummary, prior)) >= MinSimilarityForVerify
//
// Rationale: if the round's summary is substantively different from
// prior rounds, the LLM is making progress and the code-based Pass is
// trustworthy. If the summary is structurally identical to a prior
// round, the LLM may be template-mimicking and an LLM verdict call is
// warranted.
//
// 0.85 is the same threshold used by interfaces.SimilarityConfig.
// DefaultInterceptThreshold — sharing the threshold keeps the
// stagnation-detection surface aligned across the two detection paths.
type SemanticSimilarityConfig struct {
	// MinSimilarityForVerify: 0.0-1.0. max(Jaccard to prior rounds) must
	// be >= this value to trigger the LLM semantic verify. 0.85 mirrors
	// interfaces.DefaultInterceptThreshold.
	MinSimilarityForVerify float64

	// MinArtifactChars: don't bother verifying very short summaries
	// (LLM greetings, single-line acknowledgements). Templates usually
	// have hundreds of chars; useful prose is sometimes short. 100 chars
	// is a pragmatic floor.
	MinArtifactChars int

	// Enabled: master switch. When false, ItemPipelineRunner skips the
	// similarity check + LLM call entirely (preserves code-based behavior).
	// Bound to devrix.yaml d7.semantic_convergence.enabled.
	Enabled bool
}

// DefaultSemanticSimilarityConfig returns the production defaults.
//
// DM-20260706-006: production default is Enabled=true. The Jaccard
// pre-check is cheap (O(priors) interfaces.Jaccard calls) and only fires
// the LLM call when there's a structural stagnation signal, so the
// always-on default adds near-zero cost to healthy rounds and a
// single 8s timeout-bounded LLM call to stagnation-suspect rounds.
// Set Enabled=false explicitly to disable (tests / emergency rollback).
func DefaultSemanticSimilarityConfig() SemanticSimilarityConfig {
	return SemanticSimilarityConfig{
		MinSimilarityForVerify: interfaces.DefaultInterceptThreshold, // 0.85
		MinArtifactChars:       100,
		Enabled:                true, // production default ON
	}
}

// looksLikeTemplateMimicry is the cheap pre-check that decides whether
// the LLM semantic verify call is worth making. It returns true when
// the current round's ArtifactSummary is highly similar (Jaccard >= cfg
// .MinSimilarityForVerify) to ANY of the prior rounds' summaries for
// the same focus item.
//
// This is a STAGNATION SIGNAL, not a hardcoded cap. The user's
// requirement (DM-20260706-006) is "let the LLM use MUPS to converge";
// this function decides WHEN to ask the LLM the convergence question
// (token-cost guard). The convergence DECISION itself is delegated to
// the LLM via VerifySemantically below — the LLM is free to answer
// "yes this is template mimicry" OR "no, my prior answers were
// template but this round actually addresses the question". The LLM
// owns the convergence judgement; this gate owns the trigger timing.
//
// Reuses interfaces.Jaccard + interfaces.Tokenize for the comparison —
// same algorithm as interfaces.CheckSimilarity but without the
// VersionChain dependency (we have a flat []string of summaries, not
// a hash chain).
//
// Empty prior summaries → no mimicry possible → false.
//
// Why we don't reuse interfaces.CheckSimilarity directly: that function
// reads entries from a VersionChain by hash, which requires the caller
// to maintain a chain. For per-round use in a hot loop, a flat
// max-Jaccard scan is simpler and the same algorithm.
func looksLikeTemplateMimicry(current string, priors []string, cfg SemanticSimilarityConfig) bool {
	if !cfg.Enabled {
		return false
	}
	if len(priors) == 0 {
		return false
	}
	if len(current) < cfg.MinArtifactChars {
		return false
	}
	currentTokens := interfaces.Tokenize(current)
	if len(currentTokens) == 0 {
		return false
	}
	threshold := cfg.MinSimilarityForVerify
	if threshold <= 0 {
		threshold = interfaces.DefaultInterceptThreshold
	}
	maxScore := 0.0
	for _, prior := range priors {
		priorTokens := interfaces.Tokenize(prior)
		if len(priorTokens) == 0 {
			continue
		}
		s := interfaces.Jaccard(currentTokens, priorTokens)
		if s > maxScore {
			maxScore = s
		}
	}
	return maxScore >= threshold
}

// semanticStagnationReason returns a human-readable reason string for
// the stagnation detection (logged in slog + Jaeger span attribute).
func semanticStagnationReason(current string, priors []string, cfg SemanticSimilarityConfig) string {
	currentTokens := interfaces.Tokenize(current)
	if len(currentTokens) == 0 {
		return ""
	}
	var bestPrior string
	bestScore := 0.0
	for _, p := range priors {
		s := interfaces.Jaccard(currentTokens, interfaces.Tokenize(p))
		if s > bestScore {
			bestScore = s
			bestPrior = p
		}
	}
	if len(bestPrior) > 200 {
		bestPrior = bestPrior[:200] + "…"
	}
	return "max_jaccard=" + ftoa(bestScore) + " threshold=" + ftoa(cfg.MinSimilarityForVerify) + " prior_preview=" + strings.TrimSpace(bestPrior)
}

// ftoa formats float64 with 3-decimal precision without importing strconv
// into the hot path. Local helper to keep this file dependency-light.
func ftoa(v float64) string {
	if v == 0 {
		return "0.000"
	}
	// 3-decimal precision: multiply, round, format.
	intPart := int(v)
	frac := v - float64(intPart)
	if frac < 0 {
		frac = -frac
	}
	fracInt := int(frac*1000 + 0.5)
	if fracInt >= 1000 {
		intPart++
		fracInt -= 1000
	}
	// 3-digit zero-pad
	d0 := fracInt / 100
	d1 := (fracInt / 10) % 10
	d2 := fracInt % 10
	out := make([]byte, 0, 8)
	if intPart < 0 {
		out = append(out, '-')
		intPart = -intPart
	}
	out = appendInt(out, intPart)
	out = append(out, '.', byte('0'+d0), byte('0'+d1), byte('0'+d2))
	return string(out)
}

func appendInt(b []byte, n int) []byte {
	if n == 0 {
		return append(b, '0')
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return append(b, digits...)
}

// --- Verdict construction helpers ---

// semanticStagnationVerdict returns the Verdict that says "this round
// looks like a template-mimicry repeat — Decide should force SpawnNone
// to break the loop". Kind=VerdictFail with reason=template_mimicry so
// spawn_decision_algebra.go's checkVerdictDirection falls into the
// "Fail + non-Scenario + non-Exploration → SpawnNone" branch.
//
// Confidence 0.7 is intentionally below 0.85 so that downstream
// confidence-weighted reasoning (e.g. uncertainty reconciliation) does
// not over-trust a stagnation verdict over concrete evidence.
func semanticStagnationVerdict(req SemanticVerifyRequest, cfg SemanticSimilarityConfig) workmodel.Verdict {
	return workmodel.Verdict{
		Kind:       types.VerdictFail,
		Confidence: 0.7,
		Reason:     "template_mimicry: " + semanticStagnationReason(req.ArtifactSummary, req.PriorRoundSummaries, cfg),
		SourceID:   "semantic_stagnation:" + req.ItemID,
	}
}