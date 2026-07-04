# Implementation Tasks: MUPS prompttags framework

**Change ID:** `mups-prompttags`  
**Demand ID:** DM-20260704-004  
**Status:** S5_Acceptance  
**Design:** [`design.md`](design.md)

---

## Phase P0 — Registry + envelope API

- [x] Create `internal/shared/prompttags/registry.go` — TagName, TagSpec, MUPSRegistry
- [x] Create `internal/shared/prompttags/envelope.go` — Wrap, ExtractOne, ExtractAll
- [x] Create `internal/shared/prompttags/wholebody.go` — ParseWholeBody
- [x] Add round-trip + golden tests

## Phase P0 — Call-site migration

- [x] `materialize/phase_prompts.go` — scope_contract, deliverable_schema, prior_verify_reason
- [x] `workmodel/deliverable_contract.go` — DeliverableContractTag / ParseDeliverableContractTag
- [x] `workmodel/expected_return.go` — DeliverableSchemaTag legacy scalar path
- [x] `workmodel/scope_contract_parse.go` — ParseScopeContractBlock, open_questions

## Phase P0 — OpenSpec

- [x] `openspec/changes/mups-prompttags/` change package
- [x] Delta spec `specs/shared/prompttags.md`

## Verification (P0)

- [x] `go test ./internal/shared/prompttags/...`
- [x] `go test ./internal/layers/contextengine/materialize/...`
- [x] `go test ./internal/layers/orchestration/workmodel/...`

## Phase P1 — LineField user frames

- [x] Create `internal/shared/prompttags/linefield.go` — FrameSpec, ObserveUserFrame, PlanUserFrame, BuildLineFrame
- [x] Add `linefield_test.go` golden snapshots for Observe and Plan frames
- [x] Migrate `llm_observation_proposer.go` — `buildLLMObservationUserPrompt` via BuildLineFrame
- [x] Migrate `strategic_plan_proposer.go` — `buildStrategicPlanUserPrompt` via BuildLineFrame

## Verification (P1)

- [x] `go test ./internal/shared/prompttags/...`
- [x] `go test ./internal/layers/orchestration/sessionorchestrator/...`

## Phase P2 — DocBlock (i18n appendix)

- [x] Create `internal/shared/prompttags/docblock.go` — ExecuteOutputTagDoc, DocBlock, DocBlockObserveSchema, DocBlockPlanSchema
- [x] Wire `i18n/workitem_execute.go` — compose locale prose + DocBlock tag syntax
- [x] Wire `i18n/format_hints_mups.go` — Observe appendix schema from DocBlockObserveSchema
- [x] Wire `i18n/prompt_dynamic.go` — Plan appendix schema from DocBlockPlanSchema
- [x] Add `docblock_test.go`

## Phase P3 — ParseWholeBody adoption

- [x] `parseObservationProposalsJSON` — ParseWholeBody[[]rawObsProposal]
- [x] `parseStrategicPlanJSON` — ParseWholeBody[rawStrategicPlan]
- [x] `deliverable_findings_parse.go` — tryParseWholeBodyFindingsObject fast path + comment
- [x] `ExtractAll` — phase filter via TagAppliesToPhase + tests
- [x] Registry comments for Observe/Plan wholebody response shapes

## Phase S5 — OpenSpec closure

- [x] Update `demand.md` AC for P1–P3, status S5
- [x] Mark all tasks done
- [x] Register T points in t-registry
- [x] Create `acceptance-report.md`
- [x] Update delta spec with P2/P3 behavior

## Verification (final)

- [x] `go test ./internal/shared/prompttags/...`
- [x] `go test ./internal/layers/contextengine/materialize/...`
- [x] `go test ./internal/layers/orchestration/workmodel/...`
- [x] `go test ./internal/layers/orchestration/sessionorchestrator/... -count=1` (1 pre-existing failure excluded)
