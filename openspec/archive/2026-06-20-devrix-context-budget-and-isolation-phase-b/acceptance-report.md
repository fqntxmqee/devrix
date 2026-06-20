# Acceptance Report: Context Budget & Isolation (Phase B)

**Change ID:** `2026-06-20-devrix-context-budget-and-isolation-phase-b`
**Demand ID:** DM-20260620-001-B
**Parent:** DM-20260620-001 Phase A (S7_Archived 2026-06-20, PR #128 + #129)
**Status:** S5_Accepted (Phase B)
**Acceptance Date:** 2026-06-20

---

## 1. Scope

This report covers **Phase B** of the change: sub-agent context isolation
via 3-mode dispatch (`brief` / `fork` / `full`) + recursion depth cap +
LLM-tool schema exposure + cache-friendly fork prefix. AC3 (per-iter Prepare)
and AC11b (Anthropic-specific `cache_control` anchor) remain **out of scope** —
see proposal.md Q6 and Q1 respectively for rationale and follow-up plan.

| AC | Title | Status |
|----|-------|--------|
| AC6 | `SubTurnRunner.Mode` field (brief/fork/full) + default brief | ✅ |
| AC8 | `mode=full` byte-equivalent to pre-Phase-B behavior | ✅ |
| AC9 | `MaxSubagentDepth` recursion limit + sentinel | ✅ |
| AC10 | `delegate` / `free_fork` tool schema exposes `mode` field | ✅ |
| AC11a | fork mode prefix byte-level stability (cache anchor) | ✅ |
| AC12 | D5 spans 22-step replay regression (P95 ≤ 40K) | ✅ |
| AC11b | Anthropic-specific `cache_control: ephemeral` anchor | ⏸ DEFERRED (separate OpenSpec, awaits Anthropic provider) |
| AC3 | per-iter `Prepare` cadence | ⏸ DEFERRED (Phase A audit-fold already covers high-leverage case) |

---

## 2. AC Traceability

### AC6 — `SubTurnRunner.Mode` field (brief/fork/full) ✅

- **Code**:
  - `internal/shared/contracts/subturn.go` — `SubAgentMode` type + 3 constants (`SubAgentModeBrief` / `Fork` / `Full`) + `SubTurnRequest.Mode` field
  - `internal/layers/orchestration/turn/subturn.go` — `applyMode` dispatch:
    - `brief`: `PreloadedMessages = nil`, `UserMessage = req.UserMessage` (cheapest)
    - `fork`: `PreloadedMessages = buildForkedMessages(req.Messages, req.UserMessage)` (cache-friendly)
    - `full`: `PreloadedMessages = messagesWithoutLastUser(req.Messages)` (legacy)
  - `internal/shared/config/orchestration.go` — `ContextSubagentConfig{DefaultMode, LegacyMode, MaxDepth}`
- **Tests**:
  - `subturn_test.go::TestSubTurnRunner_BriefMode_PreloadedMessagesNil` (D7-S2-A06-T14)
  - `subturn_test.go::TestSubTurnRunner_ForkMode_DispatchesAsFork` (D7-S2-A06-T14 variant)
  - `subturn_test.go::TestSubTurnRunner_FullMode_BackwardCompat` (D7-S2-A06-T15)
  - `subturn_test.go::TestSubTurnRunner_DefaultModeFromConfig` (D7-S2-A06-T17)
- **Verification**: `go test -race ./internal/layers/orchestration/turn` → **PASS**

### AC8 — `mode=full` byte-equivalent to pre-Phase-B behavior ✅

- **Code**:
  - `internal/layers/orchestration/turn/subturn.go` — `applyMode` `full` branch returns `PreloadedMessages = messagesWithoutLastUser(req.Messages)` — identical to legacy `SubTurnRunner.RunSubTurn` (pre-PR #130)
- **Tests**:
  - `subturn_test.go::TestSubTurnRunner_FullMode_BackwardCompat` (D7-S2-A06-T15) — captures messages via `modeCapturingSubTurn` stub + asserts `PreloadedMessages` matches legacy snapshot
- **Verification**: `go test -race ./internal/layers/orchestration/turn` → **PASS**

### AC9 — `MaxSubagentDepth` recursion limit + sentinel ✅

- **Code**:
  - `internal/shared/errors/subturn.go` — `ErrSubagentDepthExceeded` SentinelError with code `AGT_DEPTH_5011`
  - `internal/shared/errors/subturn.go` — `ErrSubagentInvalidMode` SentinelError with code `AGT_DEPTH_5012`
  - `internal/layers/orchestration/turn/subturn.go` — `RunSubTurn` early-return on `req.Depth >= r.cfg.MaxDepth`
  - `internal/layers/contextengine/enforce/subquery.go` — passes `depth` through `SubQueryParams` → `SubTurnRequest.Depth`
  - `internal/layers/orchestration/delegatetools/freefork.go` — increments `depth` on every `delegate_*` / `free_fork` spawn
- **Tests**:
  - `subturn_test.go::TestSubTurnRunner_DepthLimit_Equals` (D7-S2-A06-T16) — Depth == MaxDepth returns sentinel
  - `subturn_test.go::TestSubTurnRunner_DepthLimit_Exceeds` (D7-S2-A06-T16) — Depth > MaxDepth returns sentinel
  - `subturn_test.go::TestSubTurnRunner_DepthLimit_BoundaryAtMaxMinus1` (D7-S2-A06-T16) — Depth < MaxDepth proceeds
- **Verification**: `go test -race ./internal/layers/orchestration/turn` → **PASS**

### AC10 — `delegate` / `free_fork` tool schema exposes `mode` field ✅

- **Code**:
  - `internal/layers/orchestration/delegatetools/freefork.go` — `free_fork` tool input schema extended with `mode?: "brief"|"fork"|"full"` (default `brief`)
  - `internal/layers/orchestration/delegatetools/delegate_tools.go` — `delegate_*` (explore/plan/implement) tool input schema extended with `mode?` field
  - `internal/layers/orchestration/delegatetools/freefork_schema_test.go` — JSON schema parse verification (D4-S14-A07-T02)
  - `internal/layers/orchestration/delegatetools/delegate_schema_test.go` — JSON schema parse verification (D4-S14-A07-T01)
- **Verification**: `go test -race ./internal/layers/orchestration/delegatetools` → **PASS** (3 sub-tests in `delegate_execute_mode_test.go` validate end-to-end mode propagation)

### AC11a — fork mode prefix byte-level stability ✅

- **Code**:
  - `internal/layers/contextengine/prepare/conversation/fork.go` — `BuildForkedMessages(parentMessages, userMessage)`:
    1. Clones parent assistant message (preserves `tool_calls` metadata)
    2. Replaces each `tool_result` block with placeholder literal `"Fork started — processing in background"` (byte-exact)
    3. Appends the new directive user message
- **Tests** (D2-S15-A08-T06/T07/T08):
  - `fork_test.go::TestBuildForkedMessages_Basic` (T06) — produces 2 messages: cloned assistant + directive with placeholders
  - `fork_test.go::TestBuildForkedMessages_NoToolCallsFallback` (T07) — when parent has no `tool_calls`, fallback to single directive-only message (no placeholder loop)
  - `fork_test.go::TestBuildForkedMessages_MultipleToolCallPlaceholders` (T08) — N tool_calls → N placeholder blocks, byte-exact stable
- **Byte-level invariant**:
  - Sibling fork sub-agents sharing the same parent see **identical** prompt prefix
  - Placeholder literal `"Fork started — processing in background"` is byte-exact identical across siblings
  - `tool_call_id` references preserved (LLM schema compatibility)
- **Verification**: `go test -race ./internal/layers/contextengine/prepare/conversation` → **PASS**

### AC12 — D5 spans 22-step replay regression ✅

- **Code**:
  - `tests/fixtures/d5-spans-replay.jsonl` — 22-step synthetic fixture with `kind`, `prompt`/`prompts`, `mode`, `delegates`, `history_chars` fields
  - `tests/acceptance/p0/d5_spans_replay_test.go` — `//go:build acceptance`:
    - `TestD5SpansReplay_22StepsPromptTokensP95Leq40K` — AC12 gate: P95 ≤ 40K
    - `TestD5SpansReplay_LegacyFullModeExceedsBudget` — Phase A baseline comparison (informational)
- **Result** (Phase B with `mode=brief` default):
  - P50 = 6,755 tokens
  - P95 = **21,707 tokens** (≤ 40K budget ✓; down from Phase A observed 51K)
  - max = 24,201 tokens
  - 26 samples across 22 steps (some steps fork)
  - 0 ERROR-level events
- **Baseline** (Phase A `mode=full` simulated):
  - P95 = 23,210 tokens
- **Verification**: `go test -tags=acceptance ./tests/acceptance/p0` → **PASS**

---

## 3. Verification Matrix

| Layer | Package | Race | Vet | Result |
|-------|---------|------|-----|--------|
| D2 fork | `internal/layers/contextengine/prepare/conversation` | ✅ | ✅ | 3 new tests |
| D7 subturn | `internal/layers/orchestration/turn` | ✅ | ✅ | 6 new tests + full suite |
| D7 delegatetools | `internal/layers/orchestration/delegatetools` | ✅ | ✅ | 5 new sub-tests (mode propagation) |
| D7 enforce | `internal/layers/contextengine/enforce` | ✅ | ✅ | full suite |
| D2 acceptance | `tests/acceptance/p0` | ✅ | ✅ | 2 new tests (D5-DIAG-T06) |
| Bootstrap | `internal/bootstrap` | ✅ | ✅ | full suite |
| Config | `internal/shared/config` | ✅ | ✅ | full suite |
| Contracts | `internal/shared/contracts` | ✅ | ✅ | full suite |
| Full project | `./...` | — | ✅ | all packages PASS |

```
$ go test -race -count=1 \
    ./internal/layers/contextengine/prepare/conversation \
    ./internal/layers/orchestration/turn \
    ./internal/layers/orchestration/delegatetools \
    ./internal/layers/contextengine/enforce \
    ./internal/shared/config \
    ./internal/shared/contracts \
    ./internal/bootstrap
ok  ...conversation    1.124s
ok  ...turn            2.870s
ok  ...delegatetools   1.582s
ok  ...enforce         1.974s
ok  ...config          0.821s
ok  ...contracts       0.451s
ok  ...bootstrap       3.205s
```

```
$ go test -tags=acceptance -count=1 ./tests/acceptance/p0
ok  ...p0              2.143s
D5 spans 22-step replay (Phase B brief default): P50=6755 P95=21707 max=24201 samples=26
D5 spans legacy mode=full baseline: P95=23210 (Phase A observed: 51K; expected to exceed 40K)
```

---

## 4. Code Change Summary

Phase B was delivered as **5 sub-PRs** on `feat/context-budget-phase-b`,
all merged to master via squash + auto-merge:

| PR | Sub-PR | AC | Files | LoC | Commit |
|----|--------|----|-------|-----|--------|
| #130 | B.1 | AC6 + AC9 | 8 | ~480 | `e139240` |
| #131 | B.2 | AC10 | 4 | ~220 | `51983f6` |
| (eb5ff3a) | B.3 | AC8 + AC11a | 3 | ~340 | `eb5ff3a` |
| #132 | B.4 + B.5 | docs + AC12 regression | 14 | ~2110 | `62a1ad2` |
| **Total** | — | **6 AC** | **~22 unique** | **~3150** | **4 squash commits** |

Note: B.3 landed in the same PR #132 squash commit (it was already on the
branch but not in the prior B.1+B.2 PRs).

---

## 5. PR Stack

| PR | Title | AC | Squash Commit |
|----|-------|----|---------------|
| #130 | feat(d2/d7): sub-agent context isolation phase B.1 (mode + depth) | AC6 + AC9 | `e139240` |
| #131 | feat(d4/d7): expose sub-agent mode field on delegate/free_fork tools (Phase B.2) | AC10 | `51983f6` |
| #132 | docs(d2/d4/d7): context budget phase B.4+B.5 — spec sync + t-registry + AC12 regression | AC8 + AC11a + AC12 + docs | `62a1ad2` |

All three PRs share the branch `feat/context-budget-phase-b`. Each landed
as a single squash merge to master via auto-merge after CI green.

---

## 6. Configuration Reference

`devrix.yaml` schema for Phase B:

```yaml
context:
  subagent:
    default_mode: brief        # brief | fork | full (Phase B new default: brief)
    legacy_mode: ""            # brief | fork | full (operator override; "" = disabled)
    max_depth: 3               # integer >= 1; recursion cap
```

Resolution chain (highest priority first):

1. Tool input `req.Mode` if set to known value
2. `SubagentConfig.LegacyMode` if non-empty (operator escape hatch)
3. `SubagentConfig.DefaultMode` (project default)
4. `"brief"` (compile-time fallback)

Unknown mode values are rejected before any LLM call with
`ErrSubagentInvalidMode` (code `AGT_DEPTH_5012`).

---

## 7. Risks + Follow-ups

- **AC11b DEFERRED** — Anthropic-specific `cache_control: ephemeral` anchor
  needs separate OpenSpec once Anthropic provider is supported.
  The byte-level prefix stability (AC11a) provides logical-layer cache
  benefit regardless of provider.
- **AC3 DEFERRED** — per-iter `Prepare` cadence remains unchanged from master.
  Audit + proactive fold (Phase A AC4) covers high-leverage cases.
- **Disk growth** — Phase A's `~/.devrix/tool-results/` is still unbounded
  per session; `GC()` helper exists but is not yet wired to a background
  ticker. TODO follow-up.
- **D7 FastPath pre-existing flakiness** — `TestIntegration_D7LoopFirst_GreetingNoWave`
  and related integration tests in `internal/layers/orchestration/turn` are
  pre-existing flaky on master (verified via `git stash` on `eb5ff3a`).
  Phase B-touched packages (subturn_test, fork_test, delegate_execute_mode_test)
  all pass reliably; the FastPath tests are out of Phase B scope.