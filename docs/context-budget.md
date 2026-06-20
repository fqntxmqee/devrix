# Context Budget & Isolation — Mode Selection Guide

**Owner:** D7 Orchestration × D4 Multi-Agent × D2 Context Engine
**Phase:** B (devrix-context-budget-and-isolation, DM-20260620-001-B)
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
- D7 spec: `openspec/specs/d7-orchestration/spec.md` (§ SubTurn 3-Mode Context Isolation)
- D4 spec: `openspec/specs/d4-multi-agent/spec.md` (§ Sub-Agent Mode Field on delegate/free_fork Tools)
- D2 t-registry: `openspec/specs/d2-context-engine/t-registry.md` (D2-S15-A08-T06/T07/T08)
- D7 t-registry: `openspec/specs/d7-orchestration/t-registry.md` (D7-S2-A06-T14/T15/T16/T17)
- D4 t-registry: `openspec/specs/d4-multi-agent/t-registry.md` (D4-S14-A07-T01/T02)
- Code: `internal/layers/orchestration/turn/subturn.go`, `internal/layers/contextengine/prepare/conversation/fork.go`