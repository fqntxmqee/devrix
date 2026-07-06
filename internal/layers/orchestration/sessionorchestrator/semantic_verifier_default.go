package sessionorchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/types"
)

// DefaultSemanticVerifier is the production SemanticVerifier — it asks
// an LLM "did your prior round actually answer the user's question?"
// and converts the LLM's answer into a workmodel.Verdict that the
// Decide node can route on.
//
// DM-20260706-006 (Semantic Convergence):
//
//   - The user explicitly rejected hardcoded loop caps ("不要设置loop
//     循环具体次数, 不要硬编码, 从本质上解决问题, 让大模型来结合我们
//     的 mups 架构来发散和收敛").
//   - The MUPS Verify node was previously code-only (5 detector + default
//     Pass). It could not detect template-mimicry, where the LLM emits
//     a near-identical <findings_json> envelope on every round.
//   - DefaultSemanticVerifier gives the LLM authority to override the
//     code-based verdict when it sees the LLM is repeating itself.
//
// Trigger: ItemPipelineRunner only CALLS this when the cheap Jaccard
// pre-check (looksLikeTemplateMimicry) returns true. This keeps the
// token cost bounded — we only ask the LLM the convergence question
// when there's a structural signal that the previous answer may have
// been a template re-emission.
//
// Failure mode: if the LLM call times out / errors / returns invalid
// JSON, we return the unmodified codeBasedVerdict (preserves
// backward-compat / fail-open on infra issues). Stagnation verdict
// (Fail) only happens when the LLM explicitly says "no, I am not
// answering the question".
type DefaultSemanticVerifier struct {
	// LLM is the streaming invoker used for the semantic verdict call.
	// Required; nil → ItemPipelineRunner falls back to code-based path.
	LLM orchtypes.LLMInvoker

	// ModelTier selects the gateway tier for the semantic call. Empty
	// → use LLM's default. Typically the verifier uses a cheaper/faster
	// tier than Execute so the convergence check is cheap.
	ModelTier string

	// Timeout caps the LLM call so a stuck verifier doesn't block the
	// pipeline. 0 → DefaultSemanticVerifierTimeout.
	Timeout time.Duration

	// Now is the clock injection point for tests. nil → time.Now.
	Now func() time.Time
}

// DefaultSemanticVerifierTimeout caps the LLM call.
const DefaultSemanticVerifierTimeout = 8 * time.Second

// semanticVerifierSystemPrompt is the prompt the verifier LLM sees.
// Kept terse and directive — this is a single yes/no question, not a
// multi-turn chat. The verifier is allowed to be brief; it owns the
// convergence judgement.
//
// Note: the prompt explicitly forbids the verifier from emitting the
// deliverable envelope. The verifier should not need to follow the
// <deliverable_schema> contract — its output is a tiny JSON line, not
// a user-facing answer.
const semanticVerifierSystemPrompt = `You are a D7 MUPS semantic-convergence verifier.

Your single job: judge whether the LLM's latest answer ACTUALLY addresses the user's original question, as opposed to re-emitting a templated deliverable envelope (e.g. <findings_json>, <deliverable_schema>) that mimics the structure of a previous round without substantive content.

Inputs you receive:
- USER_QUESTION: the original question the user asked.
- PRIOR_ROUND_ANSWERS: the previous N rounds' answers for the same focus item (may be empty on the first round).
- CURRENT_ANSWER: this round's answer.

Reply with ONE LINE of JSON, nothing else:
{"verdict": "pass" | "partial" | "fail", "confidence": 0.0-1.0, "reason": "..."}

Verdict rubric:
- "pass":  CURRENT_ANSWER substantively addresses USER_QUESTION with concrete content (specific files, line numbers, facts). Even if the format mirrors prior rounds, the content is fresh.
- "partial": CURRENT_ANSWER addresses some aspects of USER_QUESTION but is incomplete or shallow (e.g. only restates the question, only says "see above", or covers 1 of 3 sub-questions).
- "fail":  CURRENT_ANSWER is a template re-emission of PRIOR_ROUND_ANSWERS without addressing USER_QUESTION. Common signals: same <findings_json> skeleton, same "see observation.go:120" citations that don't match the user's question, generic "no issues found" without specifics.

Decision field (only relevant when verdict != "pass"):
- "stop":  SpawnNone — terminate. The current answer (and any further rounds) cannot address USER_QUESTION without fresh evidence. Use this when the LLM's content is clearly template-mimicry of PRIOR_ROUND_ANSWERS with no new information.
- "retry": SpawnInline — give the LLM one more chance with the "reason" field as a retry hint. Use this when the LLM's content is on the right track but missed concrete details (e.g. "see observation.go:120" but the user asked about d2 kernel — the LLM cited the wrong file). The next round's LLM may self-correct.
- Default: when decision is omitted, treat "fail" as "stop" and "partial" as "retry".

DO NOT emit any other text. No markdown, no XML envelope, no preamble. ONE LINE JSON ONLY.`

// semanticVerifyUserPrompt formats the user-question + prior-answers +
// current-answer triple for the verifier LLM. Kept as a separate
// function so tests can verify the exact wire format.
func semanticVerifyUserPrompt(req SemanticVerifyRequest) string {
	var b strings.Builder
	b.WriteString("USER_QUESTION:\n")
	b.WriteString(strings.TrimSpace(req.UserOriginalQuestion))
	b.WriteString("\n\nPRIOR_ROUND_ANSWERS:\n")
	if len(req.PriorRoundSummaries) == 0 {
		b.WriteString("(none — this is the first round for this focus)\n")
	} else {
		for i, p := range req.PriorRoundSummaries {
			fmt.Fprintf(&b, "[round -%d]\n%s\n\n", len(req.PriorRoundSummaries)-i, strings.TrimSpace(p))
		}
	}
	b.WriteString("CURRENT_ANSWER:\n")
	b.WriteString(strings.TrimSpace(req.ArtifactSummary))
	b.WriteString("\n\nReply ONE LINE JSON: {\"verdict\":\"pass|partial|fail\",\"confidence\":0.0-1.0,\"reason\":\"...\"}")
	return b.String()
}

// VerifySemantically calls the LLM and converts the JSON response into a
// workmodel.Verdict. See SemanticVerifier interface for contract.
//
// Fast-path: when codeBasedVerdict is already Fail (not Partial), we
// trust the code path and return it unmodified — no need to ask the LLM
// to re-confirm an infrastructure failure.
//
// Token-cost guard: the cheap Jaccard pre-check (looksLikeTemplateMimicry)
// gates whether this function is called at all. This function does NOT
// re-check similarity — the caller (ItemPipelineRunner) does that.
//
// On any infra failure (LLM error / timeout / unparseable JSON), we
// return codeBasedVerdict unchanged + the underlying error so the
// caller can log + fall back. This is the fail-open stance: a missing
// semantic verdict must NEVER make things worse than the current
// behavior (which is always Pass).
func (v *DefaultSemanticVerifier) VerifySemantically(ctx context.Context, req SemanticVerifyRequest) (workmodel.Verdict, error) {
	if v == nil || v.LLM == nil {
		return req.CodeBasedVerdict, errors.New("semantic verifier: nil LLM")
	}
	// Fast-path: code already says Fail, trust it.
	if req.CodeBasedVerdict.Kind == types.VerdictFail {
		return req.CodeBasedVerdict, nil
	}

	timeout := v.Timeout
	if timeout <= 0 {
		timeout = DefaultSemanticVerifierTimeout
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	messages := []types.Message{{
		SessionID: req.SessionID,
		Role:      types.MessageRoleUser,
		Content:   semanticVerifyUserPrompt(req),
	}}

	ch, err := v.LLM.InvokeStream(cctx, orchtypes.LLMInvokeRequest{
		SessionID:    req.SessionID,
		Tier:         v.ModelTier,
		SystemPrompt: semanticVerifierSystemPrompt,
		Messages:     messages,
		Tools:        nil, // verifier must NOT call tools
	})
	if err != nil {
		slog.Warn("semantic_verifier: llm invoke failed; fall back to code verdict",
			"session_id", req.SessionID, "item_id", req.ItemID, "err", err)
		return req.CodeBasedVerdict, err
	}

	var content strings.Builder
loop:
	for {
		select {
		case chunk, ok := <-ch:
			if !ok {
				break loop
			}
			if chunk.Content != "" {
				content.WriteString(chunk.Content)
			}
			if chunk.Thinking != "" {
				// Verifier LLM is not allowed to think out loud; ignore
				// thinking chunks silently. (Defensive — the prompt says
				// ONE LINE JSON ONLY but some models still emit thinking.)
				_ = chunk.Thinking
			}
		case <-cctx.Done():
			// Defensive: if the LLM client fails to close the channel
			// on context cancellation, bail out so the timeout
			// actually fires. Falls through to parse whatever
			// partial content we accumulated.
			slog.Warn("semantic_verifier: ctx cancelled mid-stream; partial result",
				"session_id", req.SessionID, "item_id", req.ItemID, "err", cctx.Err())
			break loop
		}
	}
	raw := strings.TrimSpace(content.String())
	if raw == "" {
		return req.CodeBasedVerdict, errors.New("semantic verifier: empty llm response")
	}

	verdict, decision, parseErr := parseSemanticVerdictJSON(raw, req)
	if parseErr != nil {
		slog.Warn("semantic_verifier: unparseable llm response; fall back to code verdict",
			"session_id", req.SessionID, "item_id", req.ItemID, "raw_preview", truncateForArtifact(raw, 200), "err", parseErr)
		return req.CodeBasedVerdict, parseErr
	}
	// Attach the decision + reason to the verdict's Reason so callers
	// (ItemPipelineRunner.maybeRunSemanticVerifier) can branch on it
	// without changing the workmodel.Verdict struct.
	if decision != "" {
		verdict.Reason = "[decision=" + decision + "] " + verdict.Reason
	}
	return verdict, nil
}

// semanticLLMResponse is the wire format the verifier LLM emits. Mirrors
// the JSON shape in semanticVerifierSystemPrompt.
//
// Decision field (DM-20260706-006 plan B): the LLM owns the convergence
// decision, not just the verdict. When verdict="fail", the LLM picks:
//
//   - "stop":  SpawnNone — terminate the session loop (template-mimicry
//              is hopeless; further rounds will reproduce the same template).
//   - "retry": SpawnInline — feed the reason back as PriorVerifyReason
//              so the next round's LLM can self-correct. Bounded by the
//              existing DeliverableContinuationRequired inline-retry budget.
//
// When verdict="pass", decision is ignored. When verdict="partial", the
// caller (ItemPipelineRunner) treats it as the code-based Partial verdict
// (existing InlineRetry budget applies).
type semanticLLMResponse struct {
	Verdict    string  `json:"verdict"`
	Confidence float64 `json:"confidence"`
	Reason     string  `json:"reason"`
	Decision   string  `json:"decision,omitempty"` // "stop" | "retry" | "" (default: stop on fail, retry on partial)
}

// parseSemanticVerdictJSON converts the LLM's one-line JSON into a
// workmodel.Verdict + a decision hint ("stop"/"retry"/""). Robust to
// common LLM deviations:
//
//   - Surrounding markdown fences (```json ... ```) are stripped.
//   - Leading prose before the JSON is dropped (we extract the first
//     balanced {...} block).
//   - Verdict strings are lowercased + trimmed before mapping.
//   - Unknown verdict values → error (caller falls back).
//   - Unknown decision values → "stop" for fail, "retry" for partial
//     (matches the default documented in the system prompt).
func parseSemanticVerdictJSON(raw string, req SemanticVerifyRequest) (workmodel.Verdict, string, error) {
	cleaned := stripCodeFence(raw)
	cleaned = extractFirstJSONObject(cleaned)
	if cleaned == "" {
		return workmodel.Verdict{}, "", fmt.Errorf("semantic verifier: no JSON object in response")
	}
	var resp semanticLLMResponse
	if err := json.Unmarshal([]byte(cleaned), &resp); err != nil {
		return workmodel.Verdict{}, "", fmt.Errorf("semantic verifier: json unmarshal: %w", err)
	}

	kind, err := mapSemanticVerdictKind(resp.Verdict)
	if err != nil {
		return workmodel.Verdict{}, "", err
	}
	confidence := resp.Confidence
	if confidence < 0 || confidence > 1 {
		confidence = 0.5
	}
	decision := strings.ToLower(strings.TrimSpace(resp.Decision))
	switch decision {
	case "stop", "retry":
		// explicit — keep
	default:
		// default fallback: fail→stop, partial→retry
		switch kind {
		case types.VerdictFail:
			decision = "stop"
		case types.VerdictPartial:
			decision = "retry"
		default:
			decision = ""
		}
	}
	return workmodel.Verdict{
		Kind:       kind,
		Confidence: confidence,
		Reason:     "semantic_verify: " + strings.TrimSpace(resp.Reason),
		SourceID:   "semantic_verifier:" + req.ItemID,
	}, decision, nil
}

func mapSemanticVerdictKind(s string) (types.VerdictKind, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "pass", "complete", "answered":
		return types.VerdictPass, nil
	case "partial", "incomplete":
		return types.VerdictPartial, nil
	case "fail", "template", "mimicry", "unanswered":
		return types.VerdictFail, nil
	default:
		return 0, fmt.Errorf("semantic verifier: unknown verdict %q", s)
	}
}

func stripCodeFence(s string) string {
	s = strings.TrimSpace(s)
	// Strip ```json ... ``` fences (LLMs frequently wrap JSON in them
	// despite being told not to).
	if strings.HasPrefix(s, "```") {
		// Skip the opening fence (e.g. "```json" or just "```")
		end := strings.Index(s, "\n")
		if end < 0 {
			return s
		}
		s = s[end+1:]
	}
	if strings.HasSuffix(s, "```") {
		s = s[:len(s)-3]
	}
	return strings.TrimSpace(s)
}

// extractFirstJSONObject returns the first balanced {...} block in s.
// Handles nested objects (LLM emits nested reasoning sometimes).
// Returns "" if no balanced object is found.
func extractFirstJSONObject(s string) string {
	start := strings.Index(s, "{")
	if start < 0 {
		return ""
	}
	depth := 0
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return ""
}

// Compile-time check.
var _ SemanticVerifier = (*DefaultSemanticVerifier)(nil)