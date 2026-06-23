# Spec Delta — D7-Orchestration — Phase 6: Observe-Learner 跨域闭环集成

**Change ID:** `devrix-d7-mups-v4-phase6-observe-learner-wiring`
**Target Spec:** `openspec/specs/d7-orchestration/spec.md`
**Target Version:** v4.5.0 → v4.6.0
**Demand ID:** DM-20260624-001
**Created:** 2026-06-24

---

## ADDED Requirements

This delta adds 3 new requirement sections (D7-S12-A41 / D7-S12-A42 / D7-S12-A43) under a new "Scenario D7-S12" for Phase 6 跨域闭环集成.

### D7-S12-A41: Observer 子模块 + WithPrior 变体

The D7 orchestrator shall provide 3 Observer submodules (IntentQuantizer / AnomalyDetector / RuleClassifier) that each expose both a baseline (no prior) method and a `*WithPrior` variant for AdaptivePrior injection.

#### Scenario: IntentQuantizer.QuantizeWithPrior adjusts confidence using prior.PriorBeta.Mean

- **Given** `IntentQuantizer` is constructed with `NewIntentQuantizer(cfg)`
- **When** `QuantizeWithPrior(ctx, "what is the weather?", prior)` is called with `prior.PriorBeta = Beta(8,3)` (Mean=0.727)
- **Then** the returned `IntentPayload.Confidence` shall be `baseline_confidence * 0.727` (clamped to [0, 100])
- **And** `payload.Reason` shall include `[prior.Mean=0.727]`

#### Scenario: AnomalyDetector.HistoricalDetector.DetectWithPrior applies prior.Mean() as threshold multiplier

- **Given** `AnomalyDetector` is constructed with `NewAnomalyDetector()` (threshold=0.5)
- **When** `HistoricalDetector.DetectWithPrior(ctx, anomalies, prior)` is called with `prior.PriorBeta = Beta(8,1)` (Mean=0.889)
- **Then** the effective threshold shall be `0.5 * 0.889 = 0.4445`
- **And** anomalies with `Severity < 0.4445` shall be filtered out

#### Scenario: RuleClassifier.ClassifyWithPrior preserves baseline behavior when prior is nil

- **Given** `RuleClassifier` is constructed with `NewRuleClassifier(cfg)`
- **When** `ClassifyWithPrior(ctx, "/help", nil)` is called
- **Then** the result shall equal `Classify(ctx, "/help")` exactly
- **And** no prior adjustment is applied

#### Scenario: Observer submodules do not mutate prior

- **Given** `prior` is a non-nil `*learn.AdaptivePrior`
- **When** any `*WithPrior` method is called
- **Then** the prior's `Reputation`, `PriorBeta`, and `InjectTargets` shall be unchanged
- **And** the prior is treated as read-only (immutable pattern)

### D7-S12-A42: SessionOrchestrator integrates Learner for LP-1 closed loop

The `SessionOrchestrator` shall integrate a `learn.Learner` for AdaptivePrior injection. The injection happens at ProcessMessage entry, before intent classification.

#### Scenario: SessionOrchestrator.WithLearner wires a Learner

- **Given** `learner learn.Learner` is constructed
- **When** `NewSessionOrchestrator(cfg, exec, WithLearner(learner))` is called
- **Then** the returned orchestrator's `learner` field shall be non-nil
- **And** `ProcessMessage` shall call `learner.Inject` at entry

#### Scenario: buildObserveRequest injects AdaptivePrior before classify

- **Given** orchestrator with `learner != nil`
- **When** `ProcessMessage(ctx, req{SessionID: "sess-1", Message: "hello"})` is called
- **Then** `o.buildObserveRequest(ctx, req)` is called first
- **And** the returned `ObserveRequest.Prior` equals `o.learner.Inject(ctx, "sess-1")`
- **And** classification uses `ObserveRequest.Prior` for confidence adjustment

#### Scenario: Prior injection failure uses DefaultDeveloperPrior fallback

- **Given** orchestrator with `learner != nil` and `learner.Inject` returns error
- **When** `buildObserveRequest` is called
- **Then** the returned `ObserveRequest.Prior` shall be `learn.BuildAdaptivePrior(nil, learn.TrackModeDeveloper)` (Beta(5,3))
- **And** the error is logged but does not block ProcessMessage

#### Scenario: Nil learner uses DefaultDeveloperPrior

- **Given** orchestrator with `learner == nil`
- **When** `buildObserveRequest` is called
- **Then** the returned `ObserveRequest.Prior` shall be `learn.BuildAdaptivePrior(nil, learn.TrackModeDeveloper)` (Beta(5,3))

### D7-S12-A43: End-to-end LP-1 closed-loop integration test

The 5-node pipeline (Observe → Plan → Execute → Verify → Learn) shall be exercisable end-to-end via an integration test that verifies LP-1 closure (Learn's output injects into the next Observe call).

#### Scenario: E2E LP-1 closure accumulates ReputationStore across sessions

- **Given** an in-memory mock orchestrator with `learner` wired and a fresh `ReputationStore`
- **When** `Learn(VerdictPass) × 3` is called for session "sess-1"
- **Then** `ReputationStore.Get(ctx, "sess-1").Alpha == 3`
- **When** the next `ProcessMessage(ctx, req{SessionID: "sess-1", ...})` is invoked
- **Then** `buildObserveRequest` returns `Prior.PriorBeta = Beta(8,3)` (Developer Beta(5,3) + rep Alpha=3, Beta=0)
- **And** classification uses `Prior.PriorBeta.Mean() = 0.727` to adjust confidence

#### Scenario: E2E INDETERMINATE with verifier_parse_failure does not pollute α/β

- **Given** an in-memory mock orchestrator with `learner` wired
- **When** `Learn(VerdictIndeterminate + "verifier_parse_failure")` is called
- **Then** `ReputationStore.Get` returns Alpha=0, Beta=0 (unchanged)
- **And** `ReputationStore.Get.VerifierFailureCount == 1`
- **When** the next `ProcessMessage` is called
- **Then** `buildObserveRequest` returns `Prior.PriorBeta = Beta(5,3)` (DefaultDeveloperPrior, unchanged)

#### Scenario: E2E PendingAsset path uses ScheduledMemory

- **Given** an in-memory mock orchestrator with `learner` wired
- **When** `Learn(VerdictIndeterminate + "other_reason")` is called
- **Then** a `LearningAsset` with `Class = LearningPending` is stored in `ScheduledMemory`
- **And** the asset's `TriggerAt` equals `ExpiryAt` (24h default)

#### Scenario: E2E 5-node pipeline runs end-to-end

- **Given** an in-memory mock orchestrator with all 5 nodes wired
- **When** `ProcessMessage` is called with a non-trivial message
- **Then** the 5-node pipeline runs once: Observe → Plan → Execute → Verify → Learn
- **And** `Plan.SourceObservationIDs` is set from Observe output
- **And** `Artifact.SourcePlanID` is set from Plan
- **And** `Verdict.SourceArtifactID` is set from Artifact
- **And** `LearningAsset` is created and stored in the correct Memory channel
- **And** `ReputationStore` is updated with BayesianUpdate

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
| **D7-S12** | **Phase 6 Observe-Learner 跨域闭环** | **IMPLEMENTED** |

## Revision History Update

Add to the revision history table in `spec.md`:

| Version | Date | Change |
|---------|------|--------|
| 4.6.0 | 2026-06-24 | Phase 6: 3 new ADDED Requirements (D7-S12-A41/42/43 — Observe submodules + WithPrior variants + Orchestrator LP-1 wiring + E2E test) |

## Archived Changes Update

Add to the archived changes list in `spec.md`:

- `devrix-d7-mups-v4-phase6-observe-learner-wiring` (DM-20260624-001) — Phase 6 Observe-Learner 跨域闭环集成 (S7_Archived 2026-06-24)
