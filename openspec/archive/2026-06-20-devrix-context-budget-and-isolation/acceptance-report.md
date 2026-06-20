# Acceptance Report: Context Budget & Isolation (Phase A)

**Change ID:** `2026-06-20-devrix-context-budget-and-isolation`
**Demand ID:** DM-20260620-001
**Status:** S5_Accepted (Phase A)
**Acceptance Date:** 2026-06-20

---

## 1. Scope

This report covers **Phase A** of the change (AC1, AC2, AC4, AC5, AC13).
AC3 (per-iter Prepare) is implemented as audit + proactive fold only —
the actual `Prepare` call cadence is left unchanged from master because
the Prepare → LLM → Tool pipeline is expensive to repeat and the
systemPrompt + Tools set is stable across a turn. The audit is the
high-leverage piece; a follow-up OpenSpec can re-evaluate the Prepare
cadence if needed (tracked in proposal.md Q3 + design.md §1.3).

Phase B (AC6-AC11) is **out of scope** for this report and will be
covered in a separate acceptance report after PR #5–#7 land.

---

## 2. AC Traceability

### AC1 — Tool result size cap with disk persistence ✅

- **Code**:
  - `internal/layers/contextengine/prepare/persist/tool_result_store.go`
  - `internal/layers/orchestration/turn/orchestrator.go` — `buildToolResultMsgWithCap`
  - `internal/bootstrap/wire_coordinator.go` — wires the store
- **Tests**:
  - `tool_result_store_test.go` — 9 sub-tests (whitelist, below/at/above limit, GC, UTF-8, sanitise)
  - `orchestrator_toolcap_test.go` — 5 sub-tests (below / above / non-capped / no-store / error)
- **Verification**: `go test ./internal/layers/contextengine/prepare/persist ./internal/layers/orchestration/turn` → **PASS** (race-clean)

### AC2 — Assistant output head/tail fold ✅

- **Code**:
  - `internal/layers/contextengine/prepare/persist/turn_output_store.go`
  - `internal/layers/orchestration/turn/orchestrator.go` — `buildAssistantToolCallMsgFolded`
- **Tests**:
  - `turn_output_store_test.go` — 5 sub-tests (below / above / nil-store / defaults / UTF-8 tail)
  - `orchestrator_toolcap_test.go` — 3 sub-tests (below / above / no-store; tool_calls metadata preserved)
- **Verification**: `go test ./internal/layers/contextengine/prepare/persist ./internal/layers/orchestration/turn` → **PASS**

### AC4 + AC13 — Per-iteration token audit + proactive fold + audit logging ✅

- **Code**:
  - `internal/layers/contextengine/prepare/audit/token_audit.go` — `AuditMessages` + `ShouldFoldProactively`
  - `internal/layers/orchestration/turn/orchestrator.go` — `runTokenAudit` (invoked at top of every iteration)
- **Tests**:
  - `audit/token_audit_test.go` — 9 sub-tests (basic / over-budget / no-budget / nil-counter + 5 fold-decision cases)
  - `orchestrator_toolcap_test.go` — 3 sub-tests (over-budget fold / below-budget no-op / no-store skip)
- **Audit logging**:
  - Structured slog line `orchestrator: token audit` with `total_tokens`,
    `system_tokens`, `messages_tokens`, `largest_msg_tokens`,
    `budget_percent`, `over_budget`, `proactive_fold`.
  - Span attributes `audit.*` attached to the per-iteration turn span
    (consumed by Jaeger / observability stack).
  - Proactive fold emits `orchestrator: proactive fold applied` with
    `msg_idx`, `orig_chars`, `folded_chars`.
- **Verification**: `go test ./internal/layers/contextengine/prepare/audit ./internal/layers/orchestration/turn` → **PASS**

### AC5 — Feishu card table-count / size precheck ✅

- **Code**:
  - `internal/layers/communication/channel/adapters/card_precheck.go`
  - `internal/layers/communication/channel/adapters/feishu_card_precheck.go`
  - `internal/layers/communication/channel/adapters/feishu.go` — `SendCard` + `sendCardToSession` wired
  - `internal/layers/communication/channel/adapters/feishu_card.go` — fix `flattenMarkdownTablesForFeishu` separator row bug
- **Tests**:
  - `card_precheck_test.go` — 12 sub-tests covering empty / single / multiple / nested-prefix (regex fix) / attribute / markdown pipe / mixed / self-closing / at-limit / above-limit / too-long / Name
  - 3 `CardFallbackText_*` tests for the plain-text fallback path
- **Production validation**: observed in real sessions — the D5-spans
  session (`sess_1781916669178_3000`) previously triggered ErrCode 11310
  on every streaming reply because of accumulated `<table>` blocks; with
  this fix the card precheck trips BEFORE the API call and falls back
  to a flattened plain-text path.
- **Verification**: `go test ./internal/layers/communication/channel/adapters -race` → **PASS**

### AC3 — Per-iteration Prepare (partial / deferred) ⚠️

- **Status**: NOT moved inside the loop. See §1 for rationale.
- **Audit + fold (AC4) covers the high-leverage case** — runaway
  sessions are now caught and folded before the LLM invoke, regardless
  of whether Prepare is re-run.

---

## 3. Verification Matrix

| Layer | Package | Race | Vet | Result |
|-------|---------|------|-----|--------|
| D2 persist | `internal/layers/contextengine/prepare/persist` | ✅ | ✅ | 14 tests |
| D2 audit | `internal/layers/contextengine/prepare/audit` | ✅ | ✅ | 9 tests |
| D7 orchestrator | `internal/layers/orchestration/turn` | ✅ | ✅ | full suite + 16 new |
| D1 feishu | `internal/layers/communication/channel/adapters` | ✅ | ✅ | full suite |
| Bootstrap | `internal/bootstrap` | ✅ | ✅ | full suite |
| Full project | `./...` | — | ✅ | 104 packages PASS |

```
$ go test -race -count=1 \
    ./internal/layers/contextengine/prepare/persist \
    ./internal/layers/contextengine/prepare/audit \
    ./internal/layers/orchestration/turn \
    ./internal/layers/communication/channel/adapters \
    ./internal/bootstrap
ok  ...persist        1.375s
ok  ...audit          1.625s
ok  ...turn           1.962s
ok  ...adapters       1.792s
ok  ...bootstrap      3.425s
```

---

## 4. Code Change Summary

| Commit | Files | LoC | Purpose |
|--------|-------|-----|---------|
| `6c489ce` | 5 | +344 | AC5: feishu card precheck + plain-text fallback |
| `17b1f96` | 1 | +14/-6 | AC5 fix: anchor `<table>` regex |
| `14c5212` | 5 | +694 | AC1: tool result size cap with disk persistence |
| `abba69f` | 4 | +387 | AC2: assistant output head/tail fold |
| `56a71a0` | 4 | +396 | AC4+AC13: per-iter token audit + proactive fold |

Total: 19 files, ~1829 lines (incl. tests).

---

## 5. PR Stack

The Phase A work is delivered as a stacked PR series on
`feat/context-budget-phase-a`:

| PR | Title | AC | Branch (current) |
|----|-------|----|----|
| #1 | AC5 feishu card precheck | AC5 | commits `6c489ce` + `17b1f96` |
| #2 | AC1 tool result size cap | AC1 | commit `14c5212` |
| #3 | AC2 assistant output fold | AC2 | commit `abba69f` |
| #4 | AC4+AC13 per-iter token audit | AC4+AC13 | commit `56a71a0` |

All four PRs share the branch `feat/context-budget-phase-a` (per
feedback-devrix-bugfix-pr-grouping). They will land as a single squash
merge to master once CI is green.

---

## 6. Risks + Follow-ups

- **AC3 deferred** — see §1. Revisit in a follow-up OpenSpec if
  production sessions show audit-fold isn't enough.
- **Phase B (AC6-AC11)** — sub-agent mode + cache anchor is the
  next priority; proposal + design are already drafted.
- **D5-spans replay** (AC12) — gated on Phase B landing.
- **Disk growth** — `~/.devrix/tool-results/` is unbounded per
  session; `GC()` helper exists but is not yet wired to a background
  ticker. TODO follow-up.