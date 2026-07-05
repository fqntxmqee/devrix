# Acceptance Report: MUPS semantics schema alignment

**Change ID:** `mups-semantics-schema-alignment`
**Demand ID:** DM-20260705-003 (注: 同一 DM-ID 此前曾被 `mups-go-struct-driven` 使用, 见下方 §6)
**Date:** 2026-07-05
**Verdict:** ACCEPTED

---

## 1. Summary

将 prompttags 语义层从「i18n per-target prose + `PhaseSemantics.InputRules` 双写」重构为「locale-neutral `SemanticRule` JSON-lines + thin glossary overlay」。LLM 现在可见与 Go gate JSON 形态一致的 machine-readable rules (`{"target":"obs_uncertainty","plane":"data","when":"scope_unclear",...}`), 而非散文 bullet。

---

## 2. L5 Verification

| L5 ID | Given-When-Then | Result |
|-------|-----------------|--------|
| **L5-MUPS-TAG-01** | Observe appendix contains `obs_uncertainty` machine rule + scope_unclear glossary | PASS — `TestObservationTaskAppendix_IncludesObserveKindSemantics` |
| **L5-MUPS-TAG-02** | Plan appendix contains execution_mode machine rule + uncertainty_mean/decompose glossary | PASS — `TestStrategicPlanAppendix_IncludesExecutionModeSemantics` |
| **L5-MUPS-TAG-03** | Execute hints contain deliverable_contract/findings_json machine rules + Required/Optional glossary | PASS — `TestWorkItemExecuteOutputHints_IncludesRequiredOptionalMatrix` |
| **L5-MUPS-TAG-04** | zh/en appendix golden hash stable | PASS — `TestMUPSSemanticAppendix_GoldenHash` |

---

## 3. T-Layer Evidence

| T ID | Status | Notes |
|------|--------|-------|
| shared-A97-T01 (`SemanticCondition` enum) | IMPLEMENTED | `internal/shared/prompttags/semantic_condition.go` |
| shared-A97-T02 (`SemanticRule` + `MachineLine()` + `InputRulesForFrame()`) | IMPLEMENTED | `internal/shared/prompttags/semantic_rule.go` |
| shared-A97-T03 (`SemanticBlock(phase)`) | IMPLEMENTED | `internal/shared/prompttags/semantic_block.go` |
| shared-A97-T04 (rewrite `semantics.go` — output rules only) | IMPLEMENTED | `internal/shared/prompttags/semantics.go` |
| D2-S15-A97-T01 (shrink i18n to glossary maps) | IMPLEMENTED | `prompttags_semantics_{zh,en}.go` |
| D2-S15-A97-T02 (rewrite `RenderSemanticAppendix`) | IMPLEMENTED | `prompttags_semantics_render.go` |
| D2-S15-A97-T03 (fix `prompttags_semantics_init.go`) | IMPLEMENTED | uses `InputRulesForFrame` |
| D2-S15-A97-T04 (update unit + golden tests) | IMPLEMENTED | `*_test.go` |

> **T-Registry note:** 上述 T-ID 是本 change 内部追踪用，与 `mups-prompt-tag-semantics` (DM-20260705-001) 共享 A97 编号；后者已在 `openspec/specs/d2-context-engine/t-registry.md` §D2-S15-A97 登记并涵盖相同 capability (output rules + glossary + golden)。本 change 是同一 capability 的 schema 演进 (prose → machine JSON), 实质上是 S5 增量的延伸归档，不新增 t-registry 段以避免重复登记。

---

## 4. Test Commands

```bash
go test -count=1 ./internal/shared/prompttags/... \
                    ./internal/layers/contextengine/i18n/... \
                    ./internal/layers/orchestration/sessionorchestrator/...
# All PASS
```

---

## 5. Token Budget Note

Machine JSON-lines 比散文 bullet 略密 (`{"target":"obs_uncertainty","plane":"data","when":"scope_unclear"}` ≈ 70 字符 vs. ZH "obs_uncertainty: 范围/目标不清时使用" ≈ 25 字符). 整体 Observe appendix token 数估计 +30%, 但获得了机器可校验性 (与 Go gate JSON 形态一致, ParseRejectRecord 先例). Staging A/B 对比 deferred to post-merge monitoring.

---

## 6. DM-ID Conflict Note (重要)

DM-20260705-003 此前已被 `mups-go-struct-driven` 使用 (PR #403, 已 S7_Archived 2026-07-05, 详见 `openspec/archive/2026-07-05-mups-go-struct-driven/acceptance-report.md`).

**两者实质不同**：
- 旧 `mups-go-struct-driven`: prompttags 反射驱动的 I/O contract (Observe M1)
- 本 `mups-semantics-schema-alignment`: 语义层 prose → machine JSON-lines 的 schema 演进

DM-ID 在 OpenSpec 规范中本应唯一；本 change 误用 DM-20260705-003 属 metadata error。建议未来 change-id 命名遵循 `mups-` 前缀且 DM-ID 强唯一性 (或在 demand.md 顶部明确 parent_demands 链)。

---

## 7. 关联变更

| 变更 | 关系 |
|------|------|
| DM-20260705-001 (mups-prompt-tag-semantics) | parent: 引入 `TagSemanticsRegistry` 但用 prose |
| DM-20260705-002 (mups-parse-reject-feedback) | parent: 提供 `MachineLine()` JSON profile 范式 |
| DM-20260705-009 (d7-observe-closed-classifier-prompt) | 后续: 同步封闭式分类器定位到新 schema |

---

## 8. 领域文档同步

| 文档 | 状态 |
|------|------|
| `openspec/specs/shared/prompttags.md` v3 → v3.1 | SYNCED (delta: `specs/shared/prompttags-semantics-schema.md`) |
| `openspec/specs/d2-context-engine/t-registry.md` §D2-S15-A97 | EXISTING (与 DM-001 共用) |
| `openspec/specs/d7-orchestration/t-registry.md` | EXISTING (no new section) |