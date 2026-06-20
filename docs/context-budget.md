# Context Budget & Isolation — Mode Selection Guide

**Owner:** D7 Orchestration × D4 Multi-Agent × D2 Context Engine
**Phase:** B (devrix-context-budget-and-isolation, DM-20260620-001-B) +
Phase C nested-budget (devrix-context-budget-phase-c-nested, DM-20260620-002)
**Status:** Active
**Last Updated:** 2026-06-20

This guide explains the **3-mode context inheritance** for sub-agents spawned via
`delegate_*` / `free_fork` LLM tools, and how to choose between them.

For implementation details see `openspec/changes/2026-06-20-devrix-context-budget-and-isolation-phase-b/design.md`
and `openspec/archive/2026-06-20-devrix-context-budget-and-isolation/design.md` (Phase A).

---

## TL;DR

| Mode | Context to child LLM | Cache-friendly | Default? | Use when |
|------|----------------------|----------------|----------|----------|
| `brief` | Only the directive (last user message) | N/A (no parent) | **Yes** (Phase B default) | Most delegated tasks — explore / plan / small focused work |
| `fork` | `[cloned_assistant, directive_user_with_placeholder]` | **Yes** (byte-level stable prefix) | No | Sibling parallel workers sharing a common tool-heavy parent |
| `full` | Parent messages minus last user (legacy) | No (prefix drifts) | No (opt-in via `legacy_mode`) | Backward compat — pre-Phase-B behavior, must explicitly request |

---

## 1. Mode Semantics

### `brief` (default)

```yaml
mode: brief   # or omit; falls back to SubagentConfig.DefaultMode
```

The child LLM sees **only the new directive** as a single user message. No parent
history, no prior tool calls, no prior assistant reasoning.

**Trade-offs:**
- ✅ Cheapest context budget for the child (smallest `prompt_tokens`).
- ✅ No risk of stale parent context confusing the child.
- ❌ Child cannot reference parent's prior tool calls / findings.

**Use when:** the directive is self-contained and the child can work
independently of what came before. Example: `delegate_explore "find auth-related
files"` — the child does its own search from scratch.

### `fork`

```yaml
mode: fork
```

The child sees `[cloned_assistant_with_tool_calls, directive_user_with_placeholder]`.

- The cloned assistant message preserves the parent's last assistant turn
  (with its `tool_calls` metadata).
- The directive user message contains one `[tool_result id=X]\nForkPlaceholderResult`
  block **per tool_call** — the placeholder literal `"Fork started — processing in background"`
  is byte-exact identical across all sibling fork children.
- This produces a **byte-level stable prefix** across siblings sharing the same parent,
  so when Anthropic `cache_control` lands the prompt cache will be reused automatically
  (cache hit when child fork #2 starts after fork #1 with the same parent).

**Trade-offs:**
- ✅ Cache-friendly (future Anthropic prompt cache reuse).
- ✅ Child sees parent's prior tool calls as anchored placeholders → no hallucinated tool results.
- ❌ Slightly larger context than `brief` (~2 messages vs 1).
- ❌ **Requires** parent to have a recent assistant turn with `tool_calls` metadata.
  Without it, `BuildForkedMessages` falls back to a single directive-only message
  (no placeholder, no cache anchor) — see `D2-S15-A08-T07`.

**Use when:** spawning 2+ parallel siblings (`free_fork requests=[...]`) where
each child should appear to "continue" the parent's tool execution.

### `full`

```yaml
mode: full
```

The child sees `parent_messages[:-1]` (everything before the last user message)
plus the last user message as the directive. This is the **pre-Phase-B legacy behavior**.

**Trade-offs:**
- ✅ Byte-equivalent to pre-Phase-B `SubTurnRunner` (`D2-S15-A08-T07` regression guard).
- ✅ Child sees full parent history (can reference any earlier finding).
- ❌ Largest context budget — can blow past `MaxTokens`.
- ❌ No cache stability (prefix drifts as parent history grows).

**Use when:** backward compat is mandatory (e.g., a delegation pattern that
relies on the child reading parent's prior reasoning). Generally **avoid** in new code.

---

## 2. Default Resolution Chain

```
req.Mode (explicit)
    ↓ empty/unknown?
SubagentConfig.LegacyMode (operator override)
    ↓ empty?
SubagentConfig.DefaultMode (project default, defaults to "brief")
    ↓ not configured?
"brief"   ← hard-coded fallback
```

| Source | Where | Example |
|--------|-------|---------|
| Tool input | `delegate_*(..., mode="...")` | `{"mode":"full"}` |
| Operator override | `devrix.yaml` `subagent.legacy_mode` | `legacy_mode: "full"` (forces all delegates to `full`) |
| Project default | `devrix.yaml` `subagent.default_mode` | `default_mode: "brief"` |
| Hard-coded fallback | `internal/shared/config/contextengine.go` | `"brief"` |

**Resolution order (highest to lowest priority):**
1. `req.Mode` if set to a known value (`brief` / `fork` / `full`).
2. `SubagentConfig.LegacyMode` if non-empty (operator escape hatch).
3. `SubagentConfig.DefaultMode` (project default).
4. `"brief"` (compile-time default).

Unknown mode values (e.g., `"explore"`, `"light"`) are rejected before any LLM
call with `ErrSubagentInvalidMode` (code `AGT_DEPTH_5012`).

---

## 3. Configuration Reference

`devrix.yaml`:

```yaml
subagent:
  default_mode: brief        # brief | fork | full
  legacy_mode: ""            # brief | fork | full (operator override, optional)
  max_depth: 3               # integer >= 1; recursion cap
```

### `default_mode`

- **Type:** `brief` | `fork` | `full`
- **Default:** `brief` (Phase B onward; was `full` pre-Phase-B).
- **Effect:** Used when `req.Mode` is empty and `legacy_mode` is unset.
- **Recommendation:** keep `brief` for most projects. Switch to `fork` only if
  you frequently spawn sibling workers.

### `legacy_mode`

- **Type:** `brief` | `fork` | `full` (string, empty = disabled)
- **Default:** `""` (disabled)
- **Effect:** When non-empty, **forces** all empty-`Mode` requests to use this mode,
  bypassing `default_mode`. Used for transitional periods when migrating from
  pre-Phase-B behavior.

**Migration example** (pre-Phase-B → Phase B gradual rollout):

```yaml
# Week 1: force full backward compat while you audit contexts
subagent:
  default_mode: brief
  legacy_mode: "full"    # ← every delegate without explicit mode → full

# Week 2: audit shows no regressions
subagent:
  default_mode: brief
  legacy_mode: ""        # ← operators can now opt-in per-call
```

### `max_depth`

- **Type:** `integer >= 1`
- **Default:** `3`
- **Effect:** `SubTurnRunner.RunSubTurn` rejects requests with `Depth >= MaxDepth`
  via `ErrSubagentDepthExceeded` (code `AGT_DEPTH_5011`). At least one tool call
  in the stack must be at depth 0 (the orchestrator).

**Recursion budget:**
```
0: orchestrator (RunTurn — no depth limit)
1: first delegate / fork  (allowed)
2: nested delegate from depth=1 (allowed)
3: nested delegate from depth=2 (allowed)
4: nested delegate from depth=3 → REJECTED (Depth >= MaxDepth=3)
```

---

## 4. Tool Schema Reference

`delegate_explore`, `delegate_plan`, `delegate_implement`:

```json
{
  "type": "object",
  "properties": {
    "directive": {"type": "string", "description": "Task to delegate"},
    "mode": {
      "type": "string",
      "enum": ["brief", "fork", "full"],
      "default": "brief",
      "description": "Sub-agent context inheritance mode (DM-20260620-001-B / AC10). brief = no parent history (default); fork = cache-friendly prefix for sibling workers; full = full parent history (legacy)."
    },
    "task_id": {"type": "string", "description": "Optional task ID"}
  },
  "required": ["directive"]
}
```

`free_fork`:

```json
{
  "type": "object",
  "properties": {
    "requests": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "directive": {"type": "string"},
          "mode": {
            "type": "string",
            "enum": ["brief", "fork", "full"],
            "default": "brief",
            "description": "..."
          },
          "task_id": {"type": "string"}
        },
        "required": ["directive"]
      }
    }
  }
}
```

Unknown `mode` values cause the tool call to fail at the schema-validation
step (before reaching `SubTurnRunner`), with a clear error message.

---

## 5. Decision Tree

```
Is the directive self-contained?
├── Yes → mode: brief (default)
└── No (child needs parent context)
    ├── Will you spawn 2+ siblings sharing this parent?
    │   └── Yes → mode: fork (cache-friendly prefix)
    │   └── No  → mode: brief (saves budget) OR fork (if child needs parent's tool anchors)
    └── Child needs full parent reasoning (legacy behavior)
        └── mode: full (explicit; legacy compat)
```

**Quick heuristic:** if unsure, use `brief`. It's the cheapest and most predictable.

---

## 6. Error Codes

| Code | Sentinel | Meaning | Recovery |
|------|----------|---------|----------|
| `AGT_DEPTH_5011` | `ErrSubagentDepthExceeded` | `Depth >= MaxDepth`; recursion budget exhausted | Use `mode=brief` to drop parent history; reduce nesting depth; raise `max_depth` if intentional |
| `AGT_DEPTH_5012` | `ErrSubagentInvalidMode` | `Mode` value not in `[brief, fork, full]` | Fix the tool input; omit `mode` to use `DefaultMode` |

Both errors are returned **before any LLM call**, so they incur no API cost.

---

## 7. Observability

D5 spans emitted by `SubTurnRunner`:

| Span attr | Value | Notes |
|-----------|-------|-------|
| `subagent.mode.resolved` | `brief` / `fork` / `full` | Final mode after `resolveMode` chain |
| `subagent.depth` | `int` | `req.Depth` at dispatch time |
| `subagent.depth.max` | `int` | `MaxDepth` config |
| `subagent.context.size` | `int` | Bytes of `PreloadedMessages + UserMessage` sent to LLM |
| `subagent.fork.siblings` | `int` | (fork mode only) sibling count sharing this prefix |

These are queryable via `orchestration.subagent.*` Prometheus metrics.

---

## 8. References

- Phase A design: `openspec/archive/2026-06-20-devrix-context-budget-and-isolation/design.md`
- Phase B design: `openspec/changes/2026-06-20-devrix-context-budget-and-isolation-phase-b/design.md`
- Phase C design: `openspec/changes/2026-06-20-devrix-context-budget-phase-c-nested/design.md`
- D7 spec: `openspec/specs/d7-orchestration/spec.md` (§ SubTurn 3-Mode Context Isolation)
- D4 spec: `openspec/specs/d4-multi-agent/spec.md` (§ Sub-Agent Mode Field on delegate/free_fork Tools)
- D2 t-registry: `openspec/specs/d2-context-engine/t-registry.md` (D2-S15-A08-T06/T07/T08)
- D7 t-registry: `openspec/specs/d7-orchestration/t-registry.md` (D7-S2-A06-T14/T15/T16/T17)
- D4 t-registry: `openspec/specs/d4-multi-agent/t-registry.md` (D4-S14-A07-T01/T02)
- Code: `internal/layers/orchestration/turn/subturn.go`, `internal/layers/contextengine/prepare/conversation/fork.go`

---

## 9. Nested Branch Budget Injection (Phase C)

Phase A wired four budget controls (token audit, proactive fold, tool result
cap, budget tracker) into the main-scope turn loop. Phase B (above) shrunk
the **entry messages** that a sub-agent inherits from its parent. Phase C
completes the picture: even with a small entry, the sub-agent's own multi-turn
loop can still grow past the LLM context window, and that's where the
nested branch in `runLoop` (`internal/layers/orchestration/turn/orchestrator.go:221-268`)
used to short-circuit every budget control.

### 9.1 Why this matters

`runLoop`'s nested branch fires for any sub-agent turn (`Scope=SubQuery`,
`Scope=Background`, `Scope=WaveWorker`, or any turn with `PreloadedMessages`
non-empty). It **skips** `o.context.Prepare` because the parent already
assembled the context. As a side-effect, `prepared.MaxContextTokens` stays at
its zero value, and `runTokenAudit`'s three-guard no-op short-circuits:

```go
// runTokenAudit (orchestrator.go:911)
if o.toolResultStore == nil || o.maxAssistantCh <= 0 || maxContextTokens <= 0 {
    return  // ← nested branch always lands here, every iteration
}
```

Net effect: a 4-parallel deep-review sub-agent (e.g. "review D1", "review D2",
"review D3", "review D7" after 10 tool rounds each) accumulates ~50K chars of
oversized read_file results + ~96K chars of system prompt, exceeds the LLM
context window, and gets rejected with `unsupported llm model` / `context
length exceeded` from the provider.

### 9.2 The fix (DM-20260620-002 / AC1)

`maxContextTokens` is now an explicit input to the nested turn — it travels
with the request instead of being a Prepare output:

```
SubTurnRequest.MaxContextTokens   (new field, optional override)
  ↓ (SubTurnRunner.RunSubTurn, subturn.go:116-119)
  maxCtx = req.MaxContextTokens; if ≤ 0 → r.Cfg.MaxContextTokens
  ↓
TurnRequest.MaxContextTokens      (new field, contracts.go:40-45)
  ↓
runLoop nested branch             (orchestrator.go:271-274)
  maxContextTokens = req.MaxContextTokens
  if maxContextTokens ≤ 0 → o.maxContextTokens  (Phase A wiring fallback)
  ↓
runTokenAudit + ShouldFoldProactively + tool result cap + budgetTracker
  all fire normally
```

`bootstrap/wire_coordinator.go:79-86` already reads `maxContextTokens` from
config (default `128000`) for the orchestrator's `OrchestratorDeps`. The same
variable now feeds `NewSubTurnRunner(turn.SubTurnConfig{MaxContextTokens: maxContextTokens})`
so the global config reaches nested turns without any new config wiring.

| Caller path | New field | Fallback chain |
|-------------|-----------|----------------|
| `enforce.Run` (D2 sub-agent) | `SubQueryParams.MaxContextTokens` | `params → SubTurnRunner.Cfg → 0` |
| `SubTurnRunner.RunSubTurn` | `SubTurnRequest.MaxContextTokens` | `req → Cfg → 0` |
| `DefaultOrchestrator.runLoop` (nested) | `TurnRequest.MaxContextTokens` | `req → o.maxContextTokens → 0` |

### 9.3 AC verification (4-parallel deep-review)

`tests/integration/d7/d7_nested_budget_test.go::TestIntegration_D7NestedBudget_4ParallelDeepReview`
spins 4 parallel `SubQuery.Run` calls (D1/D2/D3/D7 review topics) with the
exact budget-pressure shape that broke in production: 80K-char preloaded
assistant + 96K-char system prompt + 32K-token explicit budget. Captured
LLM stub records the post-audit payload:

| Log line (orchestrator: token audit) | Value |
|---------------------------------------|-------|
| `audit.total_tokens` | 44005 (system 24000 + messages 20005) |
| `audit.budget` | 32000 |
| `audit.budget_percent` | 1.375 (over budget) |
| `audit.over_budget` | true |
| `audit.proactive_fold_triggered` | true |
| `proactive fold applied` | `orig_chars=80000 folded_chars=1186` |

All 4 sub-agents complete; LLM stub invocation count = 4 (one per sub-agent);
largest message reaching the LLM = 1186 chars (down from 80000 pre-fold, i.e.
the disk-persisted preview marker that `persist.FoldAssistantOutput` produces).

### 9.4 T-point coverage

| T | Layer | Description |
|---|-------|-------------|
| `D7-S2-A06-T18` | Code | `TurnRequest.MaxContextTokens` field added |
| `D7-S2-A06-T19` | Code | `runLoop` nested branch reads `req.MaxContextTokens` with `o.maxContextTokens` fallback |
| `D7-S2-A06-T20` | Code | `SubTurnRunner.Cfg.MaxContextTokens` field added |
| `D7-S2-A06-T21` | Test | `TestOrchestrator_RunTurn_NestedBranch_BudgetInjection_DM_20260620_002` — explicit injection path |
| `D7-S2-A06-T22` | Test | `TestOrchestrator_RunTurn_NestedBranch_FallbackToDeps_PhaseA_AC1_DM_20260620_002` — fallback to Phase A wiring |
| `D7-S2-A06-T23` | Test | `TestIntegration_D7NestedBudget_4ParallelDeepReview` — 4-parallel end-to-end with capture adapter |
| `D2-S15-A08-T09` | Test | `TestSubTurnRunner_MaxContextTokens_Propagated_DM_20260620_002` — `SubTurnRequest` → `TurnRequest` propagation |
| `D2-S15-A08-T10` | Code | `SubQueryParams.MaxContextTokens` + `enforce.Run` pass-through to `SubTurnRequest` |

### 9.5 D7TestStack infrastructure fix (orthogonal to Phase C)

The integration test above also forced a small fix in `tests/testutil/d7_stack.go`
that unblocks **every** D7 integration test (not just the new one):

- The compiled default `LLMProviderRuntimeConfig.DefaultModel` is empty
  (operator fills it via env). This propagated to `LLMInvoker.defaultTier=""`
  and caused `bridge.ResolveTier("")` to fail every nested turn call. Set
  a deterministic `"deepseek-v4-flash"` test default.
- The compiled default `LLMGatewayConfig.ModelRouting` is empty, so
  `router.Resolve` returns `unsupported llm model` for any LLM call. Set
  the production-equivalent `"deepseek-*" → "deepseek"` pattern.

These are test-infra-only changes; production code paths are unchanged.
Before the fix, `d7_fastpath_test.go::TestIntegration_D7FastPath_FullStackTurnLoop`
was also failing with the same symptoms — verified PASS after the fix.

