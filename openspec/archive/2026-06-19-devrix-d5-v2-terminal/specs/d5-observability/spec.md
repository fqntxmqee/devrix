# D5 Observability — Specification Delta (devrix-d5-v2-terminal)

**Change ID:** devrix-d5-v2-terminal  
**Demand ID:** DM-20260619-006  
**Applies To:** `openspec/specs/d5-observability/spec.md` (v2.0.0 → v3.0.0)  
**Status:** S3_Design — 待 S7 归档合并入主 spec

---

## ADDED

### Requirement: Domain SoT Document (P0)

系统 MUST 维护 `d5-domain.md` 作为 D5 领域级单一入口文档。

**T:** — (AC-A1)

#### Scenario: d5-domain contains North Star and dual-track S table

- GIVEN Phase A documentation sync is complete
- WHEN `openspec/specs/d5-observability/d5-domain.md` is opened
- THEN it MUST contain North Star, 可验证承诺表, Out of Scope
- AND Canonical S21–S24 物理路径映射表
- AND Legacy S1–S9 冻结追溯表

#### Scenario: d5-domain cross-references boundary doc

- GIVEN Phase A is complete
- WHEN cross-domain contracts are needed
- THEN `d5-domain.md` links to `d5-boundary.md` as Cross-Domain SoT

---

### Requirement: Observability Guide (P0)

系统 MUST 维护 `observability-guide.md`，提供 Span↔T 绑定、Canonical Trace 树与 P0 Runbook。

**T:** D5-S22-A01-T02, D5-S22-A01-T03, D5-S23-A01-T02

#### Scenario: Guide documents D7 Turn trace tree

- GIVEN observability-guide is published
- WHEN engineer traces a production user message
- THEN the guide shows `gateway.message.receive` → `orchestration.turn.run` → `orchestration.llm.invoke` → `llm.stream`
- AND explicitly marks `query.loop.*` as REMOVED

#### Scenario: P0 Runbook covers zero_hit operations

- GIVEN HealthCheck reports `zero_hit_count > 0`
- WHEN operator follows observability-guide Runbook
- THEN steps include listing zero_hit ops from Health/coverage report
- AND cross-reference to `coverage.md` operation registry

#### Scenario: Sad path — observability disabled

- GIVEN `observability.enabled=false` in config
- WHEN application starts
- THEN business paths MUST still function via `NewNoOp()` / nil Bridge guards
- AND Health MUST report tracer/metrics/logging as `disabled` not `error`

**T:** D5-S0-A01 (Facade graceful degradation — existing behavior, guide documents)

---

### Requirement: Cross-Domain Boundary Document (P0)

系统 MUST 维护 `d5-boundary.md`，对称 D2 `d7-boundary.md` 风格。

**T:** D5-S23-A07-T01

#### Scenario: D2 Tracker read-only boundary

- GIVEN D2 `TrackerSurface` calls D5 tracker
- WHEN `Recent()` is invoked
- THEN D2 MUST NOT mutate tracker internal LRU state except through D5-defined write APIs invoked from tool execution hooks
- AND boundary doc names canonical package `diagnose/tracker`

#### Scenario: D7 owns Turn span creation

- GIVEN D7 TurnExecutor runs
- WHEN Turn spans are created
- THEN D7 code owns `orchestration.turn.*` and `orchestration.llm.invoke`
- AND D5 provides Bridge + Operation constants only

**T:** D5-S22-A01-T02

---

### Requirement: S23 Sub-Commitment Registration (P0)

`a-registry.md` MUST register S23 sub-commitments C3a–C3e without new S-layer IDs.

**T:** D5-S23-A07-T01, D5-S23-A09-T01, D5-S23-A03-T01

#### Scenario: C3d Tracker activity registered

- GIVEN a-registry v4.0
- WHEN locating file diagnostic tracking
- THEN Activity D5-S23-A07 TrackFileDiagnostics maps to `diagnose/tracker/tracker.go`
- AND sub-commitment label is C3d

#### Scenario: C3e FaultInject testbuild only

- GIVEN production binary without `testbuild` tag
- WHEN FaultInject hooks are invoked
- THEN injector is no-op stub
- AND boundary doc states FaultInject is test-only

**T:** D5-S23-A09-T02

#### Scenario: Doctor T frozen with canonical_a A10

- GIVEN Doctor environment checks
- WHEN tracing T layer
- THEN T IDs remain D5-S23-A03-T01/T02
- AND canonical Activity is D5-S23-A10 RunDoctorChecks

---

### Requirement: S21 DebugFilter Activity (P0)

DebugFilter MUST be Canonical Activity D5-S21-A14 FilterDebugLog.

**T:** D5-S23-A08-T01, D5-S23-A08-T02

#### Scenario: Debug level filtered by categories

- GIVEN DebugFilter configured with categories `["lsp","tracker"]`
- WHEN a debug log with category `lsp` is emitted
- THEN log passes filter rules per `filter_test.go`

#### Scenario: Non-debug level passthrough

- GIVEN DebugFilter active
- WHEN info/warn/error log is emitted
- THEN log MUST passthrough regardless of category

---

### Requirement: S0 SessionBridge Activity (P1)

SessionBridge ActiveSessions gauge MUST be Activity D5-S0-A03 TrackActiveSessions.

**T:** D5-S23-A06-T01 (canonical_s=S0)

#### Scenario: ActiveSessions increments on session start

- GIVEN metrics enabled and SessionBridge wired
- WHEN a new session is registered
- THEN `devrix_active_sessions` gauge increases
- AND decreases on session end

**T:** D5-S23-A06-T01

---

## MODIFIED

### Requirement: Canonical DSAFT Structure in spec.md (P0)

`spec.md` Overview, DSAFT 结构, Scenarios 表 MUST use S21–S24 as primary IDs.

**T:** — (AC-A2)

#### Scenario: Scenarios table lists four value streams

- GIVEN spec.md v3.0
- WHEN Scenarios section is read
- THEN rows include D5-S21 Instrument through D5-S24 Configure with IMPLEMENTED status
- AND Legacy S1–S9 appear only in frozen index

---

### Requirement: D7 Turn Span Hierarchy (P0) — supersedes stale Overview text

主路径 LLM↔Tool span MUST remain under D7 Turn family; Overview MUST NOT claim `query.loop.*` as primary.

**T:** D5-S22-A01-T02, D5-S22-A01-T03

#### Scenario: Turn span parent chain in integration test

- GIVEN tracing enabled
- WHEN D7 `TurnExecutor` completes one iteration with LLM
- THEN `orchestration.turn.run` exists
- AND `orchestration.llm.invoke` parent is turn iteration
- AND `llm.stream` parent is `orchestration.llm.invoke`

#### Scenario: Trace propagation Adapter to Gateway

- GIVEN OTLP tracing across D3 adapter
- WHEN stream completes
- THEN child spans share root trace_id with gateway span

**T:** D5-S22-A01-T03

---

### Requirement: Bridge Package Removal (P0)

Deprecated observability bridge packages MUST be removed; Facade MUST import canonical packages only.

**T:** AC-B1 — 全量 T 回归

#### Scenario: bridge.go uses instrument packages

- GIVEN Phase B2 merged
- WHEN reading `internal/layers/observability/bridge.go` imports
- THEN imports are `instrument/tracer`, `instrument/metrics`, `instrument/logger`
- AND NOT `observability/tracer` bridge package

#### Scenario: Sad path — external import attempt fails compile

- GIVEN bridge packages deleted
- WHEN a new file imports `internal/layers/observability/tracer`
- THEN compile fails
- AND developer must use `instrument/tracer`

---

### Requirement: Root File Relocation (P1)

`genai_tokens` and `llm_log` MUST live under scenario directories.

**T:** D5-S21-A07-T01, D5-S23-A04/A05

#### Scenario: GenAI token metrics under instrument/metrics

- GIVEN Phase B1 merged
- WHEN `RecordGenAITokenUsage` is located
- THEN source file is under `instrument/metrics/`

#### Scenario: LLM JSONL under diagnose/incident

- GIVEN Phase B1 merged
- WHEN `RecordLLMSpanPayload` is located
- THEN source file is under `diagnose/incident/`

---

### Requirement: PLANNED T Closure (P1)

Remaining PLANNED T entries MUST be closed with tests or documented IMPLEMENTED mapping.

**T:** D5-S21-A05-T01, D5-S21-A05-T02, D5-S23-A06-T02

#### Scenario: HealthCheck exposes coverage summary

- GIVEN observability initialized with coverage
- WHEN `HealthCheck()` is called
- THEN response includes `coverage.operations_total`, `operations_hit`, `coverage_ratio`, `zero_hit_count`

**T:** D5-S23-A06-T02

---

## REMOVED

### Requirement: QueryLoop Primary Path Documentation

`query.loop.*` as **active primary path** in Overview / Architecture diagrams MUST be removed from spec.md and design.md main flow.

**退役日期:** DM-20260618-010（代码已删）；本 change 完成文档退役。

#### Scenario: query.loop only in RETIRED section

- GIVEN spec.md v3.0
- WHEN searching for `query.loop`
- THEN occurrences are confined to RETIRED or Legacy Module Index
- AND NOT in active Requirements or primary Architecture tree

---

### Requirement: Deprecated Bridge Packages

Nine bridge-only directories under `observability/` MUST be deleted in Phase B2.

| Legacy path | Canonical replacement |
|-------------|----------------------|
| `tracer/` | `instrument/tracer/` |
| `metrics/` | `instrument/metrics/` |
| `logger/` | `instrument/logger/` |
| `telemetry/` | `instrument/telemetry/` |
| `exporter/` | `export/` |
| `coverage/` | `diagnose/coverage/` |
| `incident/` | `diagnose/incident/` |
| `settings/` | `configure/settings/` |
| `runtime/` | `configure/runtime/` |

#### Scenario: bridge directory absent after B2

- GIVEN Phase B2 merged
- WHEN listing `internal/layers/observability/tracer/`
- THEN directory does not exist

---

## UNCHANGED (Explicit)

- Operation Registry 56 条 canonical ops — 本 change 不增删
- T ID 字符串 — 不 renumber（仅 canonical_s / canonical_a 列）
- D2/D7 代码 — 仅边界文档更新
- OTLP / Prometheus exporter 行为契约 — 不变
