# Spec Delta — D2-Context-Engine — WorkItem Materialize

**Change ID:** `devrix-d7-layer-subcontext`  
**Target Spec:** `openspec/specs/d2-context-engine/spec.md`  
**Target Version:** TBD → increment minor  
**Demand ID:** DM-20260627-003  
**Created:** 2026-06-27  
**Status:** S3_Design — 待 Review  
**Boundary SoT:** `openspec/specs/d2-context-engine/d7-boundary.md`

---

## ADDED Requirements

本 delta 新增 **Scenario D2-S16（WorkItem Context Materialize）** 下 2 个 A 能力（Phase 1）。

### D2-S16-A20: ContextMaterializer.Materialize

D2 shall expose a **ContextMaterializer** that assembles LLM-ready context from partitioned storage and inbound signals. D7 shall not assemble message lists directly.

#### Scenario: materialize composes base prompt signals and private chain

- **Given** `MaterializeRequest` with partition `wi:<sid>:<wi_id>`, policy `{Mode: Fresh, TokenBudget: B}`, directive D, and inbound signals (ChildDownlink, bubbles)
- **When** `ContextMaterializer.Materialize` is called
- **Then** returned `MaterializedContext` shall contain `SystemPrompt`, `Messages`, `Tools`, and `TokenEstimate`
- **And** composition order shall be: BasePrompt(layer, work_kind, locale) → InjectSignals → LoadPrivateChain(self) → Compress(B)

#### Scenario: token budget decays by tree depth

- **Given** WorkItem at depth n > 0
- **When** Materialize computes TokenBudget
- **Then** effective budget shall be ≤ `MaxContext × 0.5^n` (configurable floor)

#### Scenario: tool profile by work kind

- **Given** WorkItem kind `explore` or `plan`
- **When** Materialize selects tools
- **Then** profile shall be `readonly` (no write_file / edit)
- **Given** kind `implement`
- **Then** profile shall be `implement`

#### Scenario: jaeger observability

- **When** Materialize completes
- **Then** span `D2_Context_Materialize` shall be emitted with attributes `session_id`, `wi_id`, `policy_mode`, `message_count`, `token_est`

---

### D2-S16-A21: Partition Store — WorkItemPrivate + Cohort Meta

D2 shall persist WorkItem execution transcript in partition-scoped append-only storage, distinct from session main transcript.

#### Scenario: append writes only to workitem partition

- **Given** Execute completed for WorkItem `C`
- **When** orchestrator calls `Append(wi:<sid>:C, msgs)`
- **Then** messages shall be appended to `~/.devrix/sessions/<sid>/wi/<C>.jsonl`
- **And** session main transcript shall not receive those messages when LayerSubContext flag is on

#### Scenario: cohort partition stores signals not react

- **Given** cohort partition `cohort:<sid>:<parent>`
- **When** ScopeContract or PeerStatusSignal is written
- **Then** storage shall be structured metadata or signals.jsonl
- **And** cohort partition shall not contain full ReAct tool-call chains by default

#### Scenario: concurrent append serialization

- **Given** two concurrent Append calls to the same wi partition
- **When** both complete
- **Then** resulting jsonl shall preserve total order without corruption (file lock or equivalent)

---

## MODIFIED Requirements

### D7 Boundary — Prepare vs Materialize

**Clarification (no behavior change to main Turn):**

| Caller | API | Partition |
|--------|-----|-----------|
| Session main Turn (depth 0) | `PrepareForTurn` / existing Prepare | SessionContext |
| WorkItem Execute (depth ≥ 1, flag on) | `ContextMaterializer.Materialize` | wi + cohort |
| SubTurn delegate (Phase 3) | Materialize with AgentSubContext | agent sidechain |

D7 **MUST NOT** read `SessionContext.Messages` to build WorkItem LLM requests when `FeatureLayerSubContextEnabled=true`.

D2 Materialize templates for WorkItem Execute **MAY** include soft-structured delivery hints (`<conclusion>`, `<open_questions>`) and **MUST NOT** require per-iteration MUPS `ObservationKind` self-labeling in the assembled prompt (see D7-S16-A73).

---

## Interface Sketch (Informative)

```go
type ContextMaterializer interface {
    Materialize(ctx context.Context, req MaterializeRequest) (MaterializedContext, error)
    Append(ctx context.Context, partition ContextPartition, msgs []types.Message) error
}
```

Suggested package: `internal/layers/contextengine/materialize/` or extend `prepare/`.

---

## Dependencies

| Upstream | Relationship |
|----------|--------------|
| D2-S15 PrepareExecutionContext | Reuse Compress, locale, tool catalog |
| D2-S15-A08 BuildForkedMessages | Phase 3 SubTurn mapping |
| context-budget fold/cap | Materialize Compress step |

---

## Non-Goals

- Rewrite session main transcript storage format
- Anthropic cache_control anchor (separate change)
- Per-iteration full Prepare for main Turn (deferred AC3)
