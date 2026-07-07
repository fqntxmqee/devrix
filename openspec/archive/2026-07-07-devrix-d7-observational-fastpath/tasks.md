# Tasks: D7 Observational-Answer Fast-Return

**T-layer registration path**: `openspec/specs/d7-orchestration/t-registry.md`
**Status legend**: PLANNED / IMPLEMENTED

| T ID | Status | Description | Evidence |
|------|--------|-------------|----------|
| D7-S5-A118-T01 | IMPLEMENTED | pickHighStrengthBusinessFact pure function (CatBusiness + strength≥threshold + non-empty Statement) | `internal/layers/orchestration/sessionorchestrator/deliverable_execute.go::pickHighStrengthBusinessFact` |
| D7-S5-A118-T02 | IMPLEMENTED | hasObsUncertainty pure function with source-filter (excludes item_pipeline/verify_signal) | `internal/layers/orchestration/sessionorchestrator/deliverable_execute.go::hasObsUncertainty` |
| D7-S5-A118-T03 | IMPLEMENTED | maybeObservationalAnswer: build Verdict + Artifact + Round, call Learner, persist round, return | `internal/layers/orchestration/sessionorchestrator/item_pipeline.go::maybeObservationalAnswer` |
| D7-S5-A118-T04 | IMPLEMENTED | Run() fork gate at line 285 with 4-condition AND (non-rollup / non-synth / Learner!=nil / no uncertainty) | `internal/layers/orchestration/sessionorchestrator/item_pipeline.go:285-298` |
| D7-S9-A119-T01 | IMPLEMENTED | hardening.EmitMUPSFastPath span wrap | `internal/layers/orchestration/hardening/emitter.go` (existing fast-path span) |
| D7-S9-A119-T02 | IMPLEMENTED | i18n ZH suffix: deterministic Q&A → strength≥0.9 + complete statement | `internal/layers/contextengine/i18n/format_hints_mups.go::observationTaskAppendixZHSuffix` |
| D7-S9-A119-T03 | IMPLEMENTED | i18n EN suffix: deterministic Q&A → strength≥0.9 + complete statement | `internal/layers/contextengine/i18n/format_hints_mups.go::observationTaskAppendixENSuffix` |
| D7-S9-A119-T04 | IMPLEMENTED | i18n golden hash regeneration for DM-20260706-011 suffix (commit a61c1e58) | `internal/layers/contextengine/i18n/format_hints_mups_test.go` |
| D7-S9-A119-T05 | IMPLEMENTED | 9 unit tests in item_pipeline_fastpath_test.go (gate + Learn + persistence + source filter) | `internal/layers/orchestration/sessionorchestrator/item_pipeline_fastpath_test.go` (394 lines) |

**Total**: 9 T-points, all IMPLEMENTED.