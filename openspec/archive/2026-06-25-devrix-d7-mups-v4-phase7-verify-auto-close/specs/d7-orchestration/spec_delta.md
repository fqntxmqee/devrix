# Spec Delta — D7-Orchestration — Phase 7: Verify→Learn Auto-Close + Operator TrackMode + D5 增强

**Change ID:** `devrix-d7-mups-v4-phase7-verify-auto-close`
**Target Spec:** `openspec/specs/d7-orchestration/spec.md`
**Target Version:** v4.6.0 → v4.7.0
**Demand ID:** DM-20260625-001
**Created:** 2026-06-25

---

## ADDED Requirements

This delta adds 3 new requirement sections (D7-S13-A47 / D7-S13-A48 / D7-S13-A49) under a new "Scenario D7-S13" for Phase 7 运行时 5 节点闭环.

### D7-S13-A47: SessionOrchestrator.processAutoClose — Verify→Learn 运行时闭环

The `SessionOrchestrator` shall automatically close the LP-1 loop at runtime by wrapping the execution path's EngineEvent channel, synthesizing a Verdict from the terminal event, and asynchronously calling `learner.Learn` to trigger BayesianUpdate + ReputationStore update.

#### Scenario: processAutoClose synthesizes VerdictPass from complete event

- **Given** orchestrator with `learner != nil`
- **When** the execution path emits events `[thinking, text, complete]` and closes the channel
- **Then** `processAutoClose` shall synthesize `workmodel.Verdict{Kind: VerdictPass}`
- **And** call `o.learner.Learn(ctx, LearnRequest{Verdict: VerdictPass, SessionID})` asynchronously
- **And** the synchronous return value of `ProcessMessage` shall be the channel proxy (non-blocking)

#### Scenario: processAutoClose synthesizes VerdictFail from error event

- **Given** orchestrator with `learner != nil`
- **When** the execution path emits events `[text, error]` with `error.Content = "OOM at line 42"` and closes the channel
- **Then** `processAutoClose` shall synthesize `workmodel.Verdict{Kind: VerdictFail, Reason: "OOM at line 42"}`
- **And** call `o.learner.Learn(ctx, LearnRequest{Verdict: VerdictFail, SessionID})` asynchronously

#### Scenario: processAutoClose synthesizes VerdictIndeterminate from tombstone event

- **Given** orchestrator with `learner != nil`
- **When** the execution path emits a `tombstone` event and closes the channel
- **Then** `processAutoClose` shall synthesize `workmodel.Verdict{Kind: VerdictIndeterminate, IndeterminateReason: "interrupt"}`
- **And** call `o.learner.Learn(ctx, LearnRequest{Verdict: VerdictIndeterminate, SessionID})` asynchronously

#### Scenario: processAutoClose skips non-terminal event types

- **Given** orchestrator with `learner != nil`
- **When** the execution path closes the channel without emitting `complete` / `error` / `tombstone` (e.g. last event is `text`, `thinking`, `tool_call`, `tool_result`, `status`, or `permission`)
- **Then** `synthesizeVerdict` shall return nil
- **And** `processAutoClose` shall NOT call `o.learner.Learn`

#### Scenario: processAutoClose nil learner is a no-op passthrough

- **Given** orchestrator with `learner == nil` (no `WithLearner` option)
- **When** `ProcessMessage` is called
- **Then** `processAutoClose` shall NOT be called
- **And** the path-returned channel is wrapped by `endSpanWhenChannelClosed` directly (no Learn attempt)

#### Scenario: processAutoClose IntentSkip path does not trigger Learn

- **Given** intent classification returns `IntentSkip` (e.g. empty message)
- **When** `ProcessMessage` routes to the skip branch (orchestrator.go:373-376)
- **Then** `processAutoClose` is NOT invoked
- **And** an empty closed channel is returned synchronously
- **And** no `o.learner.Learn` call is made (skip path has no execution results to learn from)

#### Scenario: processAutoClose Learn error is logged and does not block caller

- **Given** orchestrator with `learner != nil` and `learner.Learn` returns error
- **When** the execution path closes the channel
- **Then** `processAutoClose` shall log a `slog.Warn` with `(session_id, verdict_kind, err)`
- **And** the channel proxy returned to caller shall be unaffected (caller observes the original events)
- **And** the caller (D1 gateway) does not see the Learn error

#### Scenario: processAutoClose context cancellation skips Learn

- **Given** orchestrator with `learner != nil`
- **When** the execution path's context is cancelled mid-flight
- **Then** the channel closes prematurely
- **And** `processAutoClose` shall detect the empty/partial channel and log a `slog.Warn`
- **And** `o.learner.Learn` shall NOT be called (no terminal Verdict available)

### D7-S13-A48: ProcessRequest.TrackMode — Operator 角色支持

The `ProcessRequest` shall support a `TrackMode` field that the Orchestrator threads into `learn.BuildAdaptivePrior` to switch between Developer (Beta(5,3)) and Operator (Beta(8,1)) prior defaults.

#### Scenario: ProcessRequest.TrackMode defaults to empty (developer)

- **Given** a `ProcessRequest{}` is constructed
- **Then** `TrackMode` shall be `""` (zero value)
- **And** the orchestrator shall treat empty string as `learn.TrackModeDeveloper`
- **And** `buildObserveRequest` shall use `DefaultDeveloperPrior` (Beta(5,3), Mean=0.625)

#### Scenario: ProcessRequest.TrackMode="operator" uses DefaultOperatorPrior

- **Given** a `ProcessRequest{TrackMode: "operator"}` is passed to `ProcessMessage`
- **When** `buildObserveRequest` is called
- **Then** the prior's `PriorBeta` shall be `learn.DefaultOperatorPrior` (Beta(8,1), Mean=0.889)
- **And** the classification confidence shall be adjusted by `Mean = 0.889`

#### Scenario: ProcessRequest.TrackMode="developer" uses DefaultDeveloperPrior

- **Given** a `ProcessRequest{TrackMode: "developer"}` is passed to `ProcessMessage`
- **When** `buildObserveRequest` is called
- **Then** the prior's `PriorBeta` shall be `learn.DefaultDeveloperPrior` (Beta(5,3), Mean=0.625)

#### Scenario: ProcessRequest.TrackMode invalid value falls back to developer

- **Given** a `ProcessRequest{TrackMode: "garbage"}` is passed to `ProcessMessage`
- **When** `buildObserveRequest` is called
- **Then** the orchestrator shall log a `slog.Warn` for the invalid value
- **And** fall back to `learn.TrackModeDeveloper` (Beta(5,3))

#### Scenario: ProcessMessageContract sets TrackMode="" for backward compatibility

- **Given** D1 gateway calls `ProcessMessageContract(ctx, sessionID, message)` (string-args variant)
- **When** the contract method constructs a `ProcessRequest`
- **Then** `TrackMode` shall be `""` (zero value)
- **And** the prior shall default to `learn.TrackModeDeveloper`
- **And** the existing v1.0 D1 gateway callers see no behavior change

### D7-S13-A49: sessionSpan 6 prior attributes — D5 可观测化增强

The `sessionSpan` for `ProcessMessage` shall record 6 prior-related attributes for Jaeger trace inspection, enabling operators to identify whether a prior is cold-start failsafe or real injection.

#### Scenario: sessionSpan records all 6 prior attributes when prior is injected

- **Given** orchestrator with `learner != nil` and a `ReputationStore` row with `Alpha=8, Beta=1, TrackMode=operator`
- **When** `ProcessMessage` is called
- **Then** `sessionSpan` shall contain the following 6 attributes:
  - `learn.prior.alpha` = `"8"` (string)
  - `learn.prior.beta` = `"1"` (string)
  - `learn.prior.mean` = `"0.889"` (string, 8/(8+1) formatted to 3 decimal places)
  - `learn.prior.track_mode` = `"operator"`
  - `learn.prior.injected_at` = `"phase6_lp1"` (real injection, not failsafe)
  - `learn.classifier_source` = `"rule"` (or `"shadow"` if `WithShadowClassifier` is wired)

#### Scenario: sessionSpan marks cold_start_failsafe when learner is nil

- **Given** orchestrator with `learner == nil` (no `WithLearner` option)
- **When** `ProcessMessage` is called
- **Then** `sessionSpan` shall contain:
  - `learn.prior.alpha` = `"5"`
  - `learn.prior.beta` = `"3"`
  - `learn.prior.mean` = `"0.625"` (5/(5+3) formatted to 3 decimal places)
  - `learn.prior.track_mode` = `"developer"`
  - `learn.prior.injected_at` = `"cold_start_failsafe"` (failsafe, not injection)
  - `learn.classifier_source` = `"rule"` (or `"shadow"`)

#### Scenario: sessionSpan classifier_source reflects shadow wiring

- **Given** orchestrator with `WithShadowClassifier` wired
- **When** `ProcessMessage` is called
- **Then** `sessionSpan` shall contain `learn.classifier_source = "shadow"`
- **And** all other 5 prior attributes shall still be recorded

## Scenarios Table Update

Add to the Scenarios table in `spec.md`:

| Scenario ID | Description | Status |
|-------------|-------------|--------|
| D7-S1 | AdaptiveThreshold + UncertaintyCoord | IMPLEMENTED |
| D7-S2 | Orthogonal IntentKind paths | IMPLEMENTED |
| D7-S3 | Phase 3 Execute | IMPLEMENTED |
| D7-S4 | Phase 3 Channels | IMPLEMENTED |
| D7-S8 | Phase 2 Observe | IMPLEMENTED |
| D7-S9 | Phase 3 Artifact | IMPLEMENTED |
| D7-S10 | Phase 4 Verify | IMPLEMENTED |
| D7-S11 | Phase 5 Learn | IMPLEMENTED |
| D7-S12 | Phase 6 Observe-Learner 跨域闭环 | IMPLEMENTED |
| **D7-S13** | **Phase 7 Verify→Learn Auto-Close + Operator TrackMode + D5 增强** | **IMPLEMENTED** |

## Revision History Update

Add to the revision history table in `spec.md`:

| Version | Date | Change |
|---------|------|--------|
| 4.7.0 | 2026-06-25 | Phase 7: 3 new ADDED Requirements (D7-S13-A47/48/49 — processAutoClose + ProcessRequest.TrackMode + sessionSpan 6 attributes) |

## Archived Changes Update

Add to the archived changes list in `spec.md`:

- `devrix-d7-mups-v4-phase7-verify-auto-close` (DM-20260625-001) — Phase 7 Verify→Learn Auto-Close + Operator TrackMode + D5 可观测化增强 (S7_Archived 2026-06-25)
