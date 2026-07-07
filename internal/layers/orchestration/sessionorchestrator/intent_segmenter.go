package sessionorchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	ifaces "github.com/devrix/devrix/internal/layers/orchestration/interfaces"
	"github.com/devrix/devrix/internal/layers/orchestration/mups/learn"
	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
	"github.com/devrix/devrix/internal/shared/prompttags"
	"github.com/devrix/devrix/internal/shared/types"
)

// =====================================================================
// IntentSegmenter (DM-20260707-001 PR-A2 T05-T09)
//
// The IntentSegmenter is invoked by Observe node (wiring deferred to
// PR-B) to decompose a user directive into 1+ IntentSegment. Each segment
// becomes one child WorkItem in the multi-intent path; Plan node then
// builds a PlanDAG over them (PR-B).
//
// Three implementations form a fallback chain:
//
//   1. SegmenterDispatcher (orchestration): precheck fast-path + LLM
//      with 800ms timeout → fallback to RuleBased.
//   2. LLMIntentSegmenter: D2→D3 LLM call, 6-shot prompt, JSON parse.
//   3. RuleBasedSegmenter: Chinese-connective regex (v1). On miss,
//      lazy fallback returns 1-element set with the whole directive.
//
// Production code should always use SegmenterDispatcher. The two
// primitives are exposed separately so tests can pin behaviour.
//
// Boundaries (per design.md §2.2):
//   - IntentSegmenter does NOT touch Plan. Output is IntentSegmentSet
//     only; PlanDAG construction lives in PR-B.
//   - Lazy fallback returns slog.Warn + 1-element set, NOT an error.
//   - All errors return *sharederrors.SentinelError with codes 7120-7122
//     (LLM timeout, invalid response, no segment). Inner errors are
//     unwrapped; wrap helpers expose canonical codes for audit.
//
// Consensus: see reviews/pr-a2-codex-consensus-2026-07-07.md (5 ACCEPT
// + 3 ADOPT-WITH-CHANGE + 2 risks adopted). Notable adoptions:
//   - Fast-path lives in Dispatcher (not RuleBased) — adopted Q2.
//   - FastPathConfidence in Config (not hardcoded 0.95) — adopted R1.
//   - slog.Warn + reason field on lazy fallback — adopted R2.
// =====================================================================

// SegmentRequest is the input to IntentSegmenter.Segment.
//
// Mirrors orchtypes.ObserveRequest's surface (SessionID + Message + Prior)
// but is independent — Segmenter is a grammar-only consumer and must
// stay decoupled from Observer's full surface (which carries signals,
// scope, child bubbles, etc.).
type SegmentRequest struct {
	SessionID string
	Message   string
	Prior     *learn.AdaptivePrior
	// UserContextPrepend (DM-20260706-008) is the runtime user-context
	// prepend (AGENTS.md / D{N}→path mapping) that the D2 contextengine
	// surfaces for the active session. The LLMIntentSegmenter routes its
	// messages through messagesForLLMInvoke with this map so the AGENTS.md
	// prepend reaches the LLM exactly like Observe/Plan proposers do.
	// Default nil = no prepend (legacy callers unaffected).
	UserContextPrepend map[string]string
}

// IntentSegmenter produces an IntentSegmentSet from a raw user message.
type IntentSegmenter interface {
	Segment(ctx context.Context, req SegmentRequest) (ifaces.IntentSegmentSet, error)
}

// Config tunes SegmenterDispatcher behaviour.
type Config struct {
	// LLMTimeout — hard cap on the LLM call. Default 800ms (per T08).
	LLMTimeout time.Duration

	// FastPathConfidence — minimum confidence to short-circuit a
	// single-intent directive to a 1-element set without calling LLM.
	// Default 0.95 (per T09). Configurable per codex consensus R1.
	FastPathConfidence float64

	// FastPathMinLength — directives shorter than this skip the
	// connective-pattern check entirely (cheap short-circuit).
	// Default 8 runes.
	FastPathMinLength int

	// Now — clock injection for tests; nil → time.Now.
	Now func() time.Time
}

func (c Config) llmTimeout() time.Duration {
	if c.LLMTimeout > 0 {
		return c.LLMTimeout
	}
	return 800 * time.Millisecond
}

func (c Config) fastPathConfidence() float64 {
	if c.FastPathConfidence > 0 {
		return c.FastPathConfidence
	}
	return 0.95
}

func (c Config) fastPathMinLength() int {
	if c.FastPathMinLength > 0 {
		return c.FastPathMinLength
	}
	return 8
}

func (c Config) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

// =====================================================================
// SegmenterDispatcher
// =====================================================================

// SegmenterDispatcher is the production entry point. It enforces:
//
//   1. Fast-path precheck (Q2 adopted): if the directive has no connective
//      patterns and is at least FastPathMinLength runes long, return a
//      single-segment set with confidence = FastPathConfidence. No LLM
//      call, no cost.
//
//   2. LLM call (800ms timeout): if the LLM responds successfully AND
//      returns ≥1 valid segment, use that result.
//
//   3. RuleBased fallback: on LLM timeout, error, or invalid response,
//      fall back to RuleBasedSegmenter. If RuleBased also produces 0
//      segments (rare — only when lazy fallback is disabled), use the
//      lazy 1-element whole-directive fallback.
//
//   4. Lazy fallback (Q3/R2 adopted): if both paths fail to produce ≥1
//      segment, slog.Warn with reason="all_paths_failed" and return a
//      1-element set with the whole directive. NEVER returns an error
//      from Segment() — the worst case is a 1-element whole-directive
//      set, which downstream Plan degrades to the PR-B1 4-channel path.
type SegmenterDispatcher struct {
	LLM  IntentSegmenter
	Rule IntentSegmenter
	Cfg  Config
}

// NewSegmenterDispatcher wires a Dispatcher. Either LLM or Rule may be nil;
// nil paths are skipped. At least one must be non-nil.
func NewSegmenterDispatcher(llm, rule IntentSegmenter, cfg Config) *SegmenterDispatcher {
	if llm == nil && rule == nil {
		return nil
	}
	return &SegmenterDispatcher{LLM: llm, Rule: rule, Cfg: cfg}
}

// Segment implements the IntentSegmenter interface.
func (d *SegmenterDispatcher) Segment(ctx context.Context, req SegmentRequest) (ifaces.IntentSegmentSet, error) {
	if d == nil {
		return lazyFallback(req, "dispatcher_nil"), nil
	}
	msg := strings.TrimSpace(req.Message)
	if msg == "" {
		return lazyFallback(req, "empty_message"), nil
	}

	// 1. Fast-path precheck (Q2 adopted).
	if isSingleIntent(msg, d.Cfg.fastPathMinLength()) {
		return ifaces.NewIntentSegmentSet(
			req.Message,
			d.Cfg.now(),
			[]ifaces.IntentSegment{
				{
					ID:         "seg_0",
					Text:       req.Message,
					Kind:       classifyFastPath(msg),
					Priority:   50,
					Confidence: d.Cfg.fastPathConfidence(),
				},
			},
		), nil
	}

	// 2. LLM call (with 800ms timeout).
	if d.LLM != nil {
		set, err := d.callLLMWithTimeout(ctx, req)
		if err == nil && len(set.Segments) > 0 {
			return set, nil
		}
		if err != nil {
			slog.Warn("intent_segmenter_llm_failed",
				"session_id", req.SessionID,
				"reason", classifyLLMError(err),
				"error", err.Error(),
			)
		}
	}

	// 3. RuleBased fallback.
	if d.Rule != nil {
		set, err := d.Rule.Segment(ctx, req)
		if err == nil && len(set.Segments) > 0 {
			return set, nil
		}
		if err != nil {
			slog.Warn("intent_segmenter_rule_failed",
				"session_id", req.SessionID,
				"error", err.Error(),
			)
		}
	}

	// 4. Lazy fallback (Q3/R2 adopted): slog.Warn + 1-element set.
	return lazyFallback(req, "all_paths_failed"), nil
}

// callLLMWithTimeout enforces the 800ms budget and converts timeout into
// a *sharederrors.SentinelError ORCH_INTENT_SEGMENTER_LLM_TIMEOUT_7120.
func (d *SegmenterDispatcher) callLLMWithTimeout(ctx context.Context, req SegmentRequest) (ifaces.IntentSegmentSet, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, d.Cfg.llmTimeout())
	defer cancel()
	start := d.Cfg.now()
	set, err := d.LLM.Segment(timeoutCtx, req)
	if err != nil {
		if timeoutCtx.Err() == context.DeadlineExceeded {
			elapsed := time.Since(start).Milliseconds()
			return ifaces.IntentSegmentSet{}, ifaces.NewIntentSegmenterLLMTimeoutError(elapsed, d.Cfg.llmTimeout().Milliseconds())
		}
		return ifaces.IntentSegmentSet{}, err
	}
	if len(set.Segments) == 0 {
		return ifaces.IntentSegmentSet{}, ifaces.NewIntentSegmenterNoSegmentError()
	}
	return set, nil
}

// classifyLLMError maps an error to a short reason code for slog/metric
// tagging. Strips the inner error wrapping for brevity.
func classifyLLMError(err error) string {
	if err == nil {
		return "unknown"
	}
	switch {
	case isContextDeadline(err):
		return "llm_timeout"
	case isJSONParseError(err):
		return "llm_invalid_response"
	case isNoSegmentError(err):
		return "llm_no_segment"
	default:
		return "llm_error"
	}
}

// =====================================================================
// LLMIntentSegmenter
// =====================================================================

// LLMIntentSegmenter uses the D2→D3 LLM pathway to decompose a directive.
// Produces ≥1 segment via 6-shot prompting + JSON parse.
//
// This is the primary path. RuleBased is only a fallback when this
// returns error / empty / timeout.
type LLMIntentSegmenter struct {
	LLM orchtypes.LLMInvoker
}

// NewLLMIntentSegmenter wires an LLM-backed segmenter. Returns nil if llm
// is nil (caller should fall back to RuleBased).
func NewLLMIntentSegmenter(llm orchtypes.LLMInvoker) *LLMIntentSegmenter {
	if llm == nil {
		return nil
	}
	return &LLMIntentSegmenter{LLM: llm}
}

// Segment implements IntentSegmenter.
func (s *LLMIntentSegmenter) Segment(ctx context.Context, req SegmentRequest) (ifaces.IntentSegmentSet, error) {
	if s == nil || s.LLM == nil {
		return ifaces.IntentSegmentSet{}, fmt.Errorf("intent_segmenter: nil LLM segmenter")
	}
	if strings.TrimSpace(req.Message) == "" {
		return ifaces.IntentSegmentSet{}, fmt.Errorf("intent_segmenter: empty message")
	}
	systemPrompt := llmSegmenterSystemPrompt
	userPrompt := buildLLMSegmenterUserPrompt(req)
	msgs := messagesForLLMInvoke([]types.Message{{
		SessionID: req.SessionID,
		Role:      types.MessageRoleUser,
		Content:   userPrompt,
	}}, req.UserContextPrepend)
	ch, err := s.LLM.InvokeStream(ctx, orchtypes.LLMInvokeRequest{
		SessionID:    req.SessionID,
		SystemPrompt: systemPrompt,
		Messages:     msgs,
	})
	if err != nil {
		return ifaces.IntentSegmentSet{}, fmt.Errorf("intent_segmenter: llm invoke: %w", err)
	}
	raw := collectSegmenterLLMText(ch)
	return parseLLMSegmenterJSON(raw, req.Message, time.Now())
}

// llmSegmenterSystemPrompt is the 6-shot directive for the LLM
// segmenter. 6 examples cover common multi-intent patterns; the LLM is
// told to return a JSON array of {id, text, kind, priority, confidence}.
const llmSegmenterSystemPrompt = `你是编排 Observe 节点的 Intent Segmenter。

角色定位：
- 输入 = 一段用户 directive（可能包含 1 个或多个意图）
- 输出 = JSON 数组，每个元素: {id, text, kind, priority, confidence}
- 字段定义：
  * id: "seg_0", "seg_1", ...（从 0 开始）
  * text: 该 segment 的子指令文本（保留原 directive 的措辞）
  * kind: "deterministic"（确定性问答） / "explore"（只读探查） /
          "commit"（单步变更） / "analyze"（测量/分析）
  * priority: 0-100 整数
  * confidence: 0.0-1.0 浮点

6 个示例：

Example 1:
  input: "1+1=几?"
  output: [{"id":"seg_0","text":"1+1=几?","kind":"deterministic","priority":50,"confidence":0.95}]

Example 2:
  input: "1+1=几? 巴黎时区?"
  output: [
    {"id":"seg_0","text":"1+1=几?","kind":"deterministic","priority":50,"confidence":0.95},
    {"id":"seg_1","text":"巴黎时区?","kind":"deterministic","priority":40,"confidence":0.9}
  ]

Example 3:
  input: "查 devrix 项目结构"
  output: [{"id":"seg_0","text":"查 devrix 项目结构","kind":"explore","priority":50,"confidence":0.8}]

Example 4:
  input: "查 devrix 架构? 另外 看 plan 文件"
  output: [
    {"id":"seg_0","text":"查 devrix 架构?","kind":"explore","priority":60,"confidence":0.85},
    {"id":"seg_1","text":"看 plan 文件","kind":"explore","priority":50,"confidence":0.8}
  ]

Example 5:
  input: "deploy this build"
  output: [{"id":"seg_0","text":"deploy this build","kind":"commit","priority":90,"confidence":0.9}]

Example 6:
  input: "评估 v7 演进风险"
  output: [{"id":"seg_0","text":"评估 v7 演进风险","kind":"analyze","priority":70,"confidence":0.85}]

规则：
- 仅返回 JSON 数组（不要 markdown / 散文 / 解释）
- directive 是单意图 → 1 元素数组
- directive 含 ≥2 个独立意图 → N 元素数组
- confidence 必须反映切分置信度（清晰边界=0.9+，模糊边界=0.5-0.7）
`

func buildLLMSegmenterUserPrompt(req SegmentRequest) string {
	return fmt.Sprintf("directive: %s", req.Message)
}

// collectSegmenterLLMText reads all chunks from the LLM stream and
// returns the concatenated content. Mirrors the existing
// collectLLMText helper in llm_observation_proposer.go but scoped
// locally to avoid cross-file coupling.
func collectSegmenterLLMText(ch <-chan llmgateway.Chunk) string {
	var b strings.Builder
	for chunk := range ch {
		if chunk.Content != "" {
			b.WriteString(chunk.Content)
		}
	}
	return strings.TrimSpace(b.String())
}

// rawSegmenterLLMResponse mirrors the LLM JSON wire format.
type rawSegmenterLLMResponse struct {
	ID         string  `json:"id"`
	Text       string  `json:"text"`
	Kind       string  `json:"kind"`
	Priority   int     `json:"priority"`
	Confidence float64 `json:"confidence"`
}

// parseLLMSegmenterJSON extracts a JSON array from the LLM's raw text
// (which may include markdown fencing or preamble) and converts each
// element to an IntentSegment.
//
// Mirrors parseObservationProposalsJSON pattern: try prompttags.ParseWholeBody
// first, then fall back to slice extraction.
func parseLLMSegmenterJSON(raw, sourceDirective string, now time.Time) (ifaces.IntentSegmentSet, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ifaces.IntentSegmentSet{}, ifaces.NewIntentSegmenterLLMInvalidResponseError("[empty]")
	}
	var rows []rawSegmenterLLMResponse
	if arr, ok := prompttags.ParseWholeBody[[]rawSegmenterLLMResponse](raw); ok {
		rows = arr
	} else {
		start := strings.Index(raw, "[")
		end := strings.LastIndex(raw, "]")
		candidate := raw
		if start >= 0 && end > start {
			candidate = raw[start : end+1]
		}
		if err := json.Unmarshal([]byte(candidate), &rows); err != nil {
			snippet := raw
			if len(snippet) > 50 {
				snippet = snippet[:50] + "..."
			}
			return ifaces.IntentSegmentSet{}, ifaces.NewIntentSegmenterLLMInvalidResponseError(snippet)
		}
	}
	if len(rows) == 0 {
		return ifaces.IntentSegmentSet{}, ifaces.NewIntentSegmenterNoSegmentError()
	}
	segments := make([]ifaces.IntentSegment, 0, len(rows))
	for _, r := range rows {
		kind := ifaces.IntentSegmentKind(strings.ToLower(strings.TrimSpace(r.Kind)))
		if !ifaces.IsKnownIntentSegmentKind(kind) {
			// Unknown kind → fall back to "explore" (safest non-actionable default).
			kind = ifaces.IntentSegmentKindExplore
		}
		segments = append(segments, ifaces.IntentSegment{
			ID:         r.ID,
			Text:       r.Text,
			Kind:       kind,
			Priority:   clampPriority(r.Priority),
			Confidence: clampConfidence(r.Confidence),
		})
	}
	return ifaces.NewIntentSegmentSet(sourceDirective, now, segments), nil
}

func clampPriority(p int) int {
	if p < 0 {
		return 0
	}
	if p > 100 {
		return 100
	}
	return p
}

func clampConfidence(c float64) float64 {
	if c < 0 {
		return 0
	}
	if c > 1 {
		return 1
	}
	return c
}

// =====================================================================
// RuleBasedSegmenter (Chinese connectives, v1)
// =====================================================================

// chineseConnectives is the regex set for v1 multi-intent detection.
// Go's regexp engine (RE2) does not support lookahead, so each connective
// is matched in isolation; ordering matters (the first hit wins).
//
// Note: `+` is gated by `\s+` on BOTH sides so that arithmetic
// ("1+1=几?") does NOT trip the fast-path; only directive-style "X + Y"
// splits. The list is intentionally short — the LLM is the authoritative
// multi-intent handler, and the rule is only a fast pre-filter.
var chineseConnectives = []*regexp.Regexp{
	regexp.MustCompile(`\s+\+\s+`),  // "X + Y" (with surrounding spaces)
	regexp.MustCompile(`\s*另外\s*`), // "另外"
	regexp.MustCompile(`\s*并且\s*`), // "并且"
	regexp.MustCompile(`\s*还有\s*`), // "还有"
	regexp.MustCompile(`\s*然后\s*`), // "然后"
	regexp.MustCompile(`\s*顺便\s*`), // "顺便"
	regexp.MustCompile(`[;；]\s*`),  // "X; Y" or "X；Y"
	regexp.MustCompile(`[，,]\s*(另外|并且|还有|然后|顺便)`), // "X, 另外 Y" form
}

// RuleBasedSegmenter uses regex to detect multi-intent directives. When
// a pattern matches, the directive is split on the connective boundary.
// When no pattern matches, the lazy fallback returns a 1-element set
// with the whole directive.
type RuleBasedSegmenter struct{}

// NewRuleBasedSegmenter returns a fresh RuleBasedSegmenter (stateless).
func NewRuleBasedSegmenter() *RuleBasedSegmenter {
	return &RuleBasedSegmenter{}
}

// Segment implements IntentSegmenter.
//
// v1 is Chinese-only (per codex consensus Q4). English connectives
// ("and", "also", "then") fall through to lazy fallback → Dispatcher
// falls back to LLM. This guarantees English multi-intent directives
// are handled by the primary path (LLM), not silently degraded.
func (r *RuleBasedSegmenter) Segment(_ context.Context, req SegmentRequest) (ifaces.IntentSegmentSet, error) {
	if r == nil {
		return lazyFallback(req, "rule_nil"), nil
	}
	msg := strings.TrimSpace(req.Message)
	if msg == "" {
		return lazyFallback(req, "rule_empty_message"), nil
	}
	segments, hit := splitOnConnectives(msg)
	if !hit {
		return lazyFallback(req, "rule_no_hit"), nil
	}
	now := time.Now()
	out := make([]ifaces.IntentSegment, 0, len(segments))
	for i, s := range segments {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		out = append(out, ifaces.IntentSegment{
			ID:         fmt.Sprintf("seg_%d", i),
			Text:       s,
			Kind:       classifyFastPath(s),
			Priority:   50,
			Confidence: 0.8,
		})
	}
	if len(out) == 0 {
		return lazyFallback(req, "rule_split_empty"), nil
	}
	return ifaces.NewIntentSegmentSet(req.Message, now, out), nil
}

// splitOnConnectives returns the split segments + whether a connective
// matched. The first matching connective wins (deterministic).
func splitOnConnectives(msg string) ([]string, bool) {
	for _, re := range chineseConnectives {
		loc := re.FindStringIndex(msg)
		if loc == nil {
			continue
		}
		// Split at the boundary; left = head, right = tail.
		left := strings.TrimSpace(msg[:loc[0]])
		right := strings.TrimSpace(msg[loc[1]:])
		if left == "" || right == "" {
			continue
		}
		return []string{left, right}, true
	}
	return nil, false
}

// =====================================================================
// Helpers
// =====================================================================

// lazyFallback returns a 1-element IntentSegmentSet with the whole
// directive. Used when:
//   - dispatcher is nil
//   - message is empty
//   - all paths failed (LLM error + RuleBased miss)
//   - RuleBased found no connective pattern
//
// Always slog.Warn with a reason field for SRE triage (R2 adopted).
func lazyFallback(req SegmentRequest, reason string) ifaces.IntentSegmentSet {
	slog.Warn("intent_segmenter_lazy_fallback",
		"reason", reason,
		"session_id", req.SessionID,
		"message_len", len(req.Message),
	)
	now := time.Now()
	seg := ifaces.IntentSegment{
		ID:         "seg_0",
		Text:       req.Message,
		Kind:       classifyFastPath(req.Message),
		Priority:   50,
		Confidence: 0.5,
	}
	return ifaces.NewIntentSegmentSet(req.Message, now, []ifaces.IntentSegment{seg})
}

// isSingleIntent reports whether the directive looks like a single
// intent (no connective patterns + min length). Used by
// SegmenterDispatcher's fast-path precheck (Q2 adopted).
func isSingleIntent(msg string, minLength int) bool {
	if len([]rune(msg)) < minLength {
		return false
	}
	for _, re := range chineseConnectives {
		if re.MatchString(msg) {
			return false
		}
	}
	return true
}

// classifyFastPath picks a kind for the fast-path / lazy fallback path.
// Heuristic only — the LLM is the authoritative source.
func classifyFastPath(msg string) ifaces.IntentSegmentKind {
	trimmed := strings.TrimSpace(msg)
	lower := strings.ToLower(trimmed)
	switch {
	case strings.HasSuffix(trimmed, "?"), strings.HasSuffix(trimmed, "？"),
		strings.HasPrefix(lower, "what"), strings.HasPrefix(lower, "how"),
		strings.HasPrefix(lower, "why"), strings.HasPrefix(lower, "when"),
		strings.HasPrefix(lower, "where"), strings.HasPrefix(lower, "who"):
		return ifaces.IntentSegmentKindDeterministic
	case strings.HasPrefix(trimmed, "查"), strings.HasPrefix(trimmed, "看"),
		strings.HasPrefix(trimmed, "找"), strings.HasPrefix(trimmed, "list"),
		strings.HasPrefix(lower, "show"), strings.HasPrefix(lower, "find"):
		return ifaces.IntentSegmentKindExplore
	case strings.HasPrefix(trimmed, "deploy"), strings.HasPrefix(trimmed, "改"),
		strings.HasPrefix(trimmed, "删"), strings.HasPrefix(trimmed, "写"),
		strings.HasPrefix(trimmed, "建"), strings.HasPrefix(lower, "commit"):
		return ifaces.IntentSegmentKindCommit
	default:
		return ifaces.IntentSegmentKindExplore
	}
}

// =====================================================================
// Error classification helpers
// =====================================================================

// isContextDeadline detects context.DeadlineExceeded (direct or wrapped).
// Uses errors.Is + a string-content fallback because LLM gateways often
// wrap the sentinel in a custom error type that does not unwrap.
func isContextDeadline(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	return strings.Contains(err.Error(), "context deadline exceeded")
}

// isJSONParseError detects json.UnmarshalTypeError / SyntaxError via
// errors.As (preferred) + a string-content fallback for custom-wrapped
// parse errors.
func isJSONParseError(err error) bool {
	if err == nil {
		return false
	}
	var syn *json.SyntaxError
	if errors.As(err, &syn) {
		return true
	}
	var typErr *json.UnmarshalTypeError
	if errors.As(err, &typErr) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "invalid character") ||
		strings.Contains(msg, "json: cannot unmarshal") ||
		strings.Contains(msg, "unexpected end of JSON")
}

func isNoSegmentError(err error) bool {
	return ifaces.IsIntentSegmenterNoSegmentError(err)
}