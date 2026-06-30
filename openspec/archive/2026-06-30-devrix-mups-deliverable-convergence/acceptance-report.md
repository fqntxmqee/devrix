---
demand-id: DM-20260630-012
change-id: devrix-mups-deliverable-convergence
status: ACCEPTED
verified-at: 2026-06-30
---

# Acceptance Report: MUPS Deliverable Convergence

## Summary

P0 + P1 implementation complete: DeliverableVerifier gates review rounds, session `complete` prefers Goal rollup deliverable with LastTextQualityGate, LLM StrategicPlanProposer wired at Plan (with rule fallback), StructuredDeliverable flows upward via bubble and rollup Materialize signals.

## P0 Acceptance (DC-2, DC-4, DC-5)

| AC | Result | Evidence |
|----|--------|----------|
| Partial + missing deliverable → not Completed | PASS | `StatusAfterSpawnNone` + `deliverable_verify_test.go` |
| Session complete prefers rollup over transition text | PASS | `buildSessionCompleteEvent` + `session_complete_test.go` |
| LastTextQualityGate meta on complete | PASS | `summary_quality` / `final_quality` metadata |
| Transition phrase markers EN/ZH | PASS | `last_text_quality_gate.go`, `deliverable_verify.go` |
| TaskIncompleteMessage when both bad | PASS | `session_complete_test.go` |

## P1 Acceptance (DC-1, DC-3)

| AC | Result | Evidence |
|----|--------|----------|
| StrategicPlanProposer D2→D3 JSON | PASS | `strategic_plan_proposer.go` + tests |
| Bootstrap wire symmetric to Observe proposer | PASS | `wire_item_pipeline.go` + test |
| StructuredDeliverable on round + bubble digest | PASS | `pipeline_round.go`, `context_bubble_apply.go` |
| Rollup child findings in Materialize signals | PASS | `workitem_exec_context.go` |
| Execute deliverable JSON appendix (executor) | PASS | `workitem_executor.go` + `deliverable_execute.go` |

## Test Execution

```text
go test ./internal/layers/orchestration/sessionorchestrator/... -count=1  → PASS
go test ./internal/layers/orchestration/workmodel/... -count=1           → PASS
go test ./internal/bootstrap/... -count=1                                  → PASS
```

Integration (requires `-tags='integration d7'`): `d7_deliverable_convergence_test.go`

## 领域文档同步清单

- [x] `openspec/specs/d7-orchestration/t-registry.md` — D7-S9-A32, D7-S5-A22, D7-S15-A41, D7-S16-A76, D7-S2-A73-T03/T04
- [x] `openspec/specs/d7-orchestration/spec.md` — v4.20.0 lite-mode 契约 + 范式 2 交付收敛（S6 合入）
- [x] `openspec/specs/d7-orchestration/CHANGELOG.md` — 时间线条目
- [x] `openspec/demand-archive-index.md` — S7 archive entry

## Notes

- Custom `ItemPipelineRunner.Verify` test hook bypasses deliverable status gate (intentional for existing tests).
- Rollup rounds use `verifyRollupArtifact`; deliverable schema gate skipped via `DeliverableStatusNotApplicable`.
- Jaeger manual tree verification (design §4) recommended on next staging deploy.
