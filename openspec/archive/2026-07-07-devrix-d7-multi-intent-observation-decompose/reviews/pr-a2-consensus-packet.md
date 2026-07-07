# PR-A2 Consensus Packet — IntentSegmenter (Observe 节点 LLM 切分)

**Date:** 2026-07-07
**PR:** PR-A2 (DM-20260707-001, 7-PR split)
**Author:** Claude (Sonnet 4.6)
**Reviewers:** codex (MiniMax-M3), cursor-agent

---

## 1. PROBLEM

DM-20260707-001 PR-A1 (97922993) shipped the IntentSegment **grammar
surface** (types + DAG + validator). Now PR-A2 ships the **LLM cut
implementation**: Observe node must call LLM to segment a multi-intent
directive into IntentSegmentSet.

Tasks (per `openspec/changes/devrix-d7-multi-intent-observation-decompose/tasks.md`):
- **T05** `IntentSegmenter` interface { `Segment(ctx, req) (IntentSegmentSet, error)` }
- **T06** `LLMIntentSegmenter` via D2 ctxengine + D3 llmgateway, prompt 含 6-shot example
- **T07** `RuleBasedSegmenter` 兜底: regex 拆 "X + Y" / "X?另外 Y" / "X,Y"
- **T08** `SegmenterDispatcher` 先 LLM (timeout 800ms) → fallback RuleBased
- **T09** 单意图 directive (无连接词) → 1 segment, confidence ≥ 0.95 走 fast-path

---

## 2. CONSTRAINTS

### 2.1 Hard constraints
1. **DAG-validate invariant**: PR-A1's `Plan.Validate()` step 4 rejects
   `Plan.DAG == nil` AND `Plan.IntentSegmentSet != nil`. Either both nil
   (PR-B1 4-channel path) or both non-nil (multi-intent path).
   **Segmenter output must be `IntentSegmentSet` only; PlanDAG construction
   is PR-B's job.** Segmenter MUST NOT touch `Plan`.
2. **LLM call timeout ≤ 800ms** (T08 hard cap). Exceeding → fallback RuleBased.
3. **Single-intent fast-path (T09)**: directive with no connective patterns
   → 1 segment, confidence ≥ 0.95. MUST skip LLM call entirely (no cost).
4. **RuleBased failure floor**: If RuleBased also fails (no regex hit on
   multi-intent-looking directive), produce **1-element set with the
   whole directive** (lazy fallback, NOT an error — downstream Plan
   degrades to 4-channel path).

### 2.2 Soft constraints
1. 6-shot prompt examples fit in prompt appendix (~1500 tokens).
2. Segmenter must NOT block Observe node startup; concurrency-safe.
3. Errors propagated to caller via `fmt.Errorf("intent_segmenter: %w", err)`.
4. New sentinels MUST use the existing `ORCH_INTENT_SEGMENT_*_71xx` /
   `ORCH_INTENT_SET_*_71xx` code ranges (7114-7119 reserved by PR-A1;
   new ones: 7120-7122 for Segmenter-specific errors).

---

## 3. SCOPE

### 3.1 NEW file: `sessionorchestrator/intent_segmenter.go`

**Interface:**
```go
type IntentSegmenter interface {
    Segment(ctx context.Context, req SegmentRequest) (IntentSegmentSet, error)
}

type SegmentRequest struct {
    SessionID  string  // from ObserveRequest.SessionID
    Message    string  // from ObserveRequest.Message (the directive)
    Prior      *learn.AdaptivePrior  // from ObserveRequest.Prior (optional)
}
```

**Implementations:**
- `RuleBasedSegmenter`: regex-based, ~50 LOC, no LLM call
- `LLMIntentSegmenter`: D2→D3, ~200 LOC including prompt
- `SegmenterDispatcher`: thin wrapper, ~30 LOC, enforces 800ms timeout

### 3.2 NEW file: `sessionorchestrator/intent_segmenter_test.go`

Test surface (10 tests):
- `TestRuleBasedSegmenter_SingleIntent_NoConnectives`: "1+1=几?" → 1 segment
- `TestRuleBasedSegmenter_MultiIntent_PlusConjunctive`: "1+1=几? + 巴黎时区?" → 2 segments
- `TestRuleBasedSegmenter_MultiIntent_另外`: "查 devrix 架构? 另外 看 plan 文件" → 2 segments
- `TestRuleBasedSegmenter_MultiIntent_CommaList`: "查 A, 看 B, 评估 C" → 3 segments
- `TestRuleBasedSegmenter_LazyFallback`: "xyz qrs" (no hit) → 1 segment (whole)
- `TestLLMIntentSegmenter_PromptContainsExamples`: golden hash on the
  prompt appendix (length + 6-shot count)
- `TestLLMIntentSegmenter_ParsesValidResponse`: mock LLM returns
  JSON → 2-segment set
- `TestLLMIntentSegmenter_RejectsMalformedJSON`: LLM returns non-JSON →
  error
- `TestSegmenterDispatcher_LLMTimeout_FallbackToRule`: LLM takes 1s →
  RuleBased output wins
- `TestSegmenterDispatcher_LLMError_FallbackToRule`: LLM returns error →
  RuleBased output wins
- `TestSegmenterDispatcher_BothFail_LazySingleSegment`: LLM fails + RuleBased
  no hit → 1-element set (no error)

### 3.3 NEW sentinels (interfaces/intent_segment.go OR new file)

```go
// ORCH_INTENT_SEGMENT_LLM_TIMEOUT_7120
ErrIntentSegmenterLLMTimeout = errors.New("...")
// ORCH_INTENT_SEGMENT_LLM_INVALID_7121
ErrIntentSegmenterLLMInvalidResponse = errors.New("...")
// ORCH_INTENT_SEGMENTER_NONE_PRODUCED_7122
ErrIntentSegmenterNoSegment = errors.New("...")
```

Wrap helpers: `NewIntentSegmenterLLMTimeoutError()` etc.

### 3.4 NOT in scope (deferred to PR-B or later)
- Wiring IntentSegmenter into Observe node (`item_observe.go`)
- Plan node accepting IntentSegmentSet → building PlanDAG
- DAG executor runtime
- Streaming emit
- Reputation per-segment

---

## 4. OPEN QUESTIONS

### Q1. Where does `SegmentRequest` live?
**Option A**: `interfaces/intent_segment.go` (alongside IntentSegment types)
**Option B**: `orchtypes/` (where ObserveRequest lives)
**Option C**: new file `interfaces/segment_request.go`

→ **Recommendation: A** (cohesion — SegmentRequest is intrinsically
about IntentSegmentSet; same package as the type it produces).

### Q2. Single-intent fast-path lives where?
The 0.95 confidence single-intent check (T09) — does it live in
`RuleBasedSegmenter` or `SegmenterDispatcher`?

→ **Recommendation: `RuleBasedSegmenter`**. The check is "no connective
patterns detected" — purely lexical, doesn't touch LLM. Dispatcher
just times out + falls back.

### Q3. Lazy 1-element fallback — error or warning?
When both LLM and RuleBased fail to produce segments, do we:
**(a)** Return 1-element set with whole directive, no error (lazy fallback)
**(b)** Return error `ErrIntentSegmenterNoSegment`, caller decides
**(c)** slog.Warn + return 1-element set

→ **Recommendation: (c)** slog.Warn + return 1-element set. Matches the
DM-20260707-001 fact-correction note: "Observe either emits ≥1 segment or
falls back to the original 4-channel plan path" — silent degradation,
not a hard fail.

### Q4. RuleBased regex set — Chinese only or multi-locale?
T07 mentions "X + Y" / "X?另外 Y" / "X,Y" — all Chinese. Should
RuleBased also handle English connectives ("and", "also", "then")?

→ **Recommendation: v1 = Chinese-only** (matches the ZH-directive
observation in the user's prod data). English coverage in v2 if needed;
RuleBased's role is a fallback, not the primary path.

### Q5. 6-shot prompt examples — where do they live?
**(a)** In-code string constants (`intent_segmenter.go`)
**(b)** External YAML file (`prompts/intent_segmenter_6shot.yaml`)
**(c)** Append to existing i18n prompttags (`format_hints_mups.go`)

→ **Recommendation: (a)** — keeps it adjacent to the LLMIntentSegmenter
impl. Following the existing pattern in `strategic_plan_proposer.go` /
`llm_observation_proposer.go`. If we later need i18n, refactor.

---

## 5. DELIVERABLES

1. `sessionorchestrator/intent_segmenter.go` (~280 LOC)
2. `sessionorchestrator/intent_segmenter_test.go` (~250 LOC)
3. `interfaces/intent_segment.go` +3 sentinel errors + 3 wrap helpers
4. `interfaces/intent_segment_test.go` +3 sentinel tests
5. Tests:
   - 10 segmenter tests (rule/LLM/dispatcher)
   - 3 sentinel tests
6. Coverage ≥ 80% on `intent_segmenter.go`
7. `go vet` clean
8. `go build ./...` clean

---

## 6. TEST MATRIX

| Scenario | Input | Expected | Test |
|----------|-------|----------|------|
| Single, deterministic | "1+1=几?" | 1 seg, conf 0.95 | TestRuleBased_*_SingleIntent |
| Single, vague | "查 devrix" | 1 seg, conf 0.7 | TestRuleBased_*_SingleIntent |
| Multi, + | "1+1 + 巴黎时区" | 2 seg | TestRuleBased_*_PlusConjunctive |
| Multi, 另外 | "查 A? 另外 B" | 2 seg | TestRuleBased_*_另外 |
| Multi, comma | "查 A, 看 B" | 2 seg | TestRuleBased_*_CommaList |
| Lazy fallback | "xyz" | 1 seg + slog.Warn | TestRuleBased_*_LazyFallback |
| LLM happy | "A + B" + LLM returns JSON | 2 seg via LLM | TestLLM_*_ParsesValidResponse |
| LLM bad JSON | "A + B" + LLM returns "nonsense" | error | TestLLM_*_RejectsMalformedJSON |
| LLM timeout | "A + B" + LLM takes 1s | RuleBased fallback | TestDispatcher_*_LLMTimeout |
| LLM error | "A + B" + LLM errs | RuleBased fallback | TestDispatcher_*_LLMError |
| Both fail | "xyz" + LLM errs | 1 seg + slog.Warn | TestDispatcher_*_BothFail |

---

## 7. RISK REGISTER

| Risk | Severity | Mitigation |
|------|----------|-----------|
| RuleBased regex misses common Chinese connectives | Med | 6-shot LLM is primary path; RuleBased is fallback; v2 expands coverage |
| 800ms timeout too tight for slow LLM | Low | Plan node degrades to 4-channel path on no-IntentSegmentSet; safe |
| LLM JSON format drift (commas, trailing whitespace) | Med | Use `prompttags.ParseWholeBody` for first attempt, fallback to regex slice extraction (existing pattern) |
| Sentinels conflict with PR-A1 7114-7119 | Low | Range 7120-7122 explicitly reserved in PR-A1 reviews packet |
| Test pollutes LLM mock — race conditions | Low | Use `chan llmgateway.Chunk` per existing pattern, no shared state |

---

## 8. CONSENSUS QUESTIONS (for codex/cursor)

Please respond with **ACCEPT / ADOPT-WITH-CHANGE / REJECT** for each:

1. **Q1**: SegmentRequest in `interfaces/intent_segment.go` (alongside IntentSegment)?
2. **Q2**: Single-intent fast-path lives in `RuleBasedSegmenter`?
3. **Q3**: Lazy fallback = slog.Warn + 1-element set (not error)?
4. **Q4**: RuleBased v1 = Chinese-only regex?
5. **Q5**: 6-shot examples in-code string constants?
6. **Sentinel codes 7120-7122** for new segmenter errors?
7. **File split**: `intent_segmenter.go` (impl) + `intent_segmenter_test.go` (test)?
8. **NOT wiring Observe node in PR-A2** (deferred to PR-B or follow-up)?

---

## 9. REVIEW DELIVERABLES

After review, expected output:
- `reviews/pr-a2-codex-consensus-2026-07-07.md` — codex's responses to
  Q1-Q8 above with adopted changes
- (Optional) `reviews/pr-a2-cursor-consensus-2026-07-07.md` — cursor's
  responses
- Implementation follows adopted consensus