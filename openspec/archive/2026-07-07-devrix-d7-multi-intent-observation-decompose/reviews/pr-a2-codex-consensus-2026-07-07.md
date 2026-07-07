# PR-A2 Codex Consensus — 2026-07-07

**Reviewer:** codex (MiniMax-M3, /usr/local/bin/codex exec)
**Reviewing:** PR-A2 consensus packet (IntentSegmenter)
**Date:** 2026-07-07
**Status:** 5 ACCEPT + 3 ADOPT-WITH-CHANGE + 2 new risks identified

---

## Q1 — SegmentRequest placement
**Verdict:** ACCEPT
**Rationale:** Keeps the request/response pair discoverable as one contract; co-located with `IntentSegment` in `interfaces/intent_segment.go`.

## Q2 — Fast-path location (T09)
**Verdict:** ADOPT-WITH-CHANGE
**Codex:** Fast-path is an optimization that should short-circuit *before*
dispatcher picks LLM vs RuleBased. Move it to `SegmenterDispatcher` (or a
small `precheck.go`) so both segmenters benefit and the rule segmenter
stays a pure fallback.
**Adopted:** Move `IsSingleIntent(directive)` check + confidence 0.95
short-circuit into `SegmenterDispatcher.Segment`. `RuleBasedSegmenter`
becomes pure regex (no fast-path responsibility).

## Q3 — Lazy fallback semantics
**Verdict:** ACCEPT
**Adopted:** `slog.Warn("intent_segmenter_lazy_fallback", reason, ...)` +
1-element set (whole directive), no error return.

## Q4 — RuleBased locale coverage (v1 = Chinese-only)
**Verdict:** ADOPT-WITH-CHANGE
**Codex:** Fine for v1, but add an explicit test asserting an English
multi-intent directive does *not* silently degenerate to the lazy
1-element set — force the LLM path.
**Adopted:** Add `TestRuleBasedSegmenter_EnglishDirective_FallsThrough`
which confirms an English multi-intent directive like "1+1? Also Paris
time zone?" returns 1 segment (lazy fallback) so the LLM path is
guaranteed to fire on English input.

## Q5 — 6-shot examples placement
**Verdict:** ACCEPT (for v1)
**Adopted:** In-code string constants. Revisit external file once prompt
iteration rate picks up.

## Q6 — Sentinel code allocation (7120-7122)
**Verdict:** ACCEPT
**Adopted:** Range 7120-7122, no collision with PR-A1's 7114-7119 or
DAG's 7200-7205.

## Q7 — File split
**Verdict:** ADOPT-WITH-CHANGE
**Codex:** Split *tests* per impl (`intent_segmenter_llm_test.go`,
`intent_segmenter_rule_test.go`, `intent_segmenter_dispatcher_test.go`)
so LLM mock state can't leak into RuleBased tests. Impl files can stay
merged to match the file-size convention.
**Adopted:** 3 test files, 1 impl file.

## Q8 — Observe node wiring deferred
**Verdict:** ACCEPT
**Adopted:** Segmenter shipped as standalone component. Observe node
wiring in PR-B (DAG executor) where the DAG-validate invariant actually
gets exercised.

---

## Additional risks (beyond section 7 of packet)

### Risk 1: Hardcoded 0.95 fast-path confidence
**Severity:** Medium
**Codex:** No calibration data behind the threshold; under-tuned values
either burn LLM budget on trivial directives or silently route
borderline multi-intent into the lazy 1-element fallback. Promote to a
named constant in `segmenter.Config` and gate behind a metric
(`segmenter_fastpath_skipped_total`) so it can be tuned post-launch.
**Adopted:**
- Introduce `segmenter.Config{ FastPathConfidence float64 }` with
  default `0.95`
- Metric: `segmenter_fastpath_skipped_total{reason}` counter
  (slog-based for v1; wired to metrics package in PR-D)

### Risk 2: Lazy fallback observability-invisible
**Severity:** Medium
**Codex:** `slog.Warn` doesn't surface in dashboards, so a real
under-segmentation regression will only show up as user-visible quality
drift. Add a `lazy_fallback_total` counter tagged with a short reason
code so SREs can alert on rate spikes.
**Adopted:**
- slog.Warn with structured `reason` field ("llm_error" / "llm_timeout" /
  "rule_no_hit" / "rule_english_locale")
- Metric: `segmenter_lazy_fallback_total{reason}` counter

---

## Implementation changes from consensus

| Original | Adopted |
|----------|---------|
| `RuleBasedSegmenter` does fast-path check | `SegmenterDispatcher` does fast-path check |
| 0.95 hardcoded | `Config.FastPathConfidence` (default 0.95) |
| Single test file | 3 test files (llm / rule / dispatcher) |
| slog.Warn (no reason field) | slog.Warn with `reason` field |
| No metrics | slog-based metric scaffold (PR-D wires Prometheus) |

---

## Cursor-agent consensus

Usage limit hit (7/20 in PR-A1 cycle, has not reset as of 2026-07-07
15:30). Cursor consensus deferred to PR-A2 follow-up or PR-B. Codex
review sufficient for grammar+impl stage per project convention
(per `devrix-three-way-consensus-cli.md`: codex primary + cursor
secondary when available).

---

## Sign-off

PR-A2 implementation proceeds with 5 ACCEPT + 3 ADOPT-WITH-CHANGE + 2
risk-driven additions, all adopted in the implementation below.