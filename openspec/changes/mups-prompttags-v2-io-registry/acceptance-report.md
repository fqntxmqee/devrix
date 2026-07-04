---
demand-id: DM-20260704-005
change-id: mups-prompttags-v2-io-registry
status: draft
created: 2026-07-04
---

# Acceptance Report (draft)

| Field | Value |
|-------|-------|
| 需求 ID | DM-20260704-005 |
| Change ID | mups-prompttags-v2-io-registry |
| Verdict | PENDING |

## P0 验收

| AC | Description | Result |
|----|-------------|--------|
| AC1 | MUPSIOCatalog + LineFrameRegistry | PENDING |
| AC2 | Observe max-3 cap | PENDING |
| AC5 | Unit tests PASS | PENDING |

## P1 验收

| AC | Description | Result |
|----|-------------|--------|
| AC3 | Plan uncertainty_mean inject | PENDING |
| AC4 | Observe incremental frame | PENDING |

## T 层验证

| T ID | Evidence | Result |
|------|----------|--------|
| D2-S15-A96-T01 | `registry_test.go` | PENDING |
| D2-S15-A96-T02 | `registry_test.go::LookupLineFrame` | PENDING |
| D7-S16-A96-T01 | `observation_proposer_test.go` | PENDING |
| D7-S16-A96-T02 | `strategic_plan_proposer_test.go` | PENDING |
| D7-S16-A96-T03 | `linefield_test.go` + `llm_observation_proposer` | PENDING |

## 领域文档同步（S5→S6）

- [ ] `openspec/specs/shared/prompttags.md` merge delta
- [ ] `openspec/t-registry.md` + domain t-registries

## 测试命令

```bash
go test ./internal/shared/prompttags/... ./internal/layers/orchestration/sessionorchestrator/... -count=1
```
