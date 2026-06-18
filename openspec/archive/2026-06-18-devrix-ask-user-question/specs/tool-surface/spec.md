# Spec Delta: ask_user_question ToolSurface

**Capability**: TOOL-SURFACE-1-A04 — ask_user_question 工具
**Change**: devrix-ask-user-question (DM-20260618-006)
**Status**: s7_archived

---

## ADDED Requirements

### Requirement: ask_user_question ToolSurface

The devrix engine SHALL provide an `ask_user_question` ToolSurface that
allows the LLM to ask the user 1-4 multiple-choice questions via the
IM channel.

The surface MUST:
- Implement the standard `contracts.ToolSurface` interface (Name / Tools / RiskLevel / Execute)
- Implement `InterruptBehavior` returning `InterruptCancel` (long-run, cancellable on ctx.Done)
- Emit a `ToolSpec` with orthogonal flags: `ReadOnly=true, Destructive=false, OpenWorld=true, ConcurrencySafe=false`
- Validate input: 1-4 questions, 2-4 options each, header chip ≤ 12 chars, unique option labels, non-empty labels, non-empty question text
- Render questions as plain-text IM message with numbered options and an explicit "其他" (Other) hint per question
- Return JSON output to the LLM containing: `delivered` (bool), `sent_at` (RFC3339), `question_text` (the actual IM text), `questions` (echoed input), `hint` (next-message instruction)
- Send the formatted question via a process-global `AskUserQuestionSender` hook installed by `main.go` to bridge into `CommunicationGateway.RouteOutbound`
- Gracefully degrade when no sender is wired: return `Delivered=false` with success (no error)
- Surface sender errors as `ToolResult.Error` so the LLM can see and retry
- Be safe under concurrent `SetAskUserQuestionSender` / `currentAskSender` calls (`-race` clean)

The surface MUST NOT:
- Block on user reply (non-blocking by design — IM bot has no UI thread)
- Persist question state across turns (single-round-trip; user reply arrives as the next inbound message)
- Mutate any session state beyond routing the outbound message

### Requirement: sender bridge in main.go

`cmd/devrix/main.go` MUST wire `asksurface.SetAskUserQuestionSender` to a
closure that calls `gw.RouteOutbound` with:
- `MessageID` = `"ask_<sessionID>_<UTC-timestamp>"`
- `SessionID` from the caller's `ctx` (via `toolrunner.ToolSessionIDFromContext`)
- `Content` = the formatted IM text
- `Role` = `types.MessageRoleAssistant`
- `Metadata` = `{source: "ask_user_question", blocking: "false"}`
- `SentAt` = current UTC time

The wiring MUST happen after `gw := capture.NewCommunicationGateway(...)`
and before the gateway starts its cleanup routine.

### Requirement: orthogonal flag table update

`OrthogonalFlagFor("ask_user_question")` MUST return `(true, false, true, false)`.
`InterruptBehaviorFor("ask_user_question")` MUST return `contracts.InterruptCancel`.
The truth-table comment in `orthogonal_flags.go` MUST be updated to include the
`ask_user_question` row.

### Requirement: BuildSurfaces integration

`BuildSurfaces(SurfaceBuildOpts{})` MUST include `AskUserQuestionSurface`
unconditionally (stateless, no opts required). The returned surface list
MUST remain sorted alphabetically by name (so the LLM tool list order
is stable across processes — preserves the prompt cache contract from
DM-20260618-001).

### Requirement: architectural note on task_* tools

`context_engine.go` and `context_engine_builder.go` MUST carry a comment
explaining that `task_create/get/list/update/delete` are provided by
`workmodel.RegisterTaskTools` when `ctxCfg.Tasks.Mode == "v2"`, and that
the enforce layer does NOT add a separate registration. This documents
the ownership boundary discovered during the DM-20260618-006 hotfix
(an earlier draft attempted to add `RegisterTaskLifecycleTools` and
collided at startup with the v2 workmodel tools).

---

## MODIFIED Requirements

(None — this change is purely additive.)

---

## REMOVED Requirements

(None.)

---

## Cross-References

- Upstream: TOOL-SURFACE-1-A01 (DM-007, devrix-tool-surface-contract) — 4-method `ToolSurface` interface baseline
- Upstream: TOOL-SURFACE-1-A01-F02 (DM-001, devrix-tool-spec-enrichment) — `ToolSpec` 4 bool flags + `InterruptBehavior`
- Upstream: TOOL-SURFACE-1-A05 (DM-001) — `BuildSurfaces` alphabetical sort for prompt cache stability
- Source: clawcode `AskUserQuestionTool` (`src/tools/AskUserQuestionTool/`) — schema and intent
- Change archive: `openspec/archive/2026-06-18-devrix-ask-user-question/`
