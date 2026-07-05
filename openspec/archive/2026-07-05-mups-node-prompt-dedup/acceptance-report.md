# Acceptance Report: MUPS 三节点 Prompt 去冗余

**Change ID:** `mups-node-prompt-dedup`
**Demand ID:** DM-20260705-004 (注: 同一 DM-ID 此前曾被 `mups-plan-structbind` 使用, 见 §6)
**Date:** 2026-07-05
**Verdict:** ACCEPTED

---

## 1. Summary

MUPS Observe/Plan/Execute 三节点 prompt 组装存在 4 类冗余（角色重复 / plane 标签三套同名 / guide 列全字段正文省略 / Execute 字段标签未走 i18n）。本 change 全部消除：净减 LLM 可见行；Execute 字段标签 i18n 化；`BuildLineFrameFromStruct` 默认无 plane 前缀；appendix 不再重复 `node_role`。

---

## 2. L5 Verification

| L5 ID | Given-When-Then | Result |
|-------|-----------------|--------|
| **L5-MUPS-PROMPT-01** | Observe user prompt 不含 work_item_id/prior_mean/incremental_only 行 | PASS — `TestBuildObserveUserPrompt_NoControlPlaneFields` |
| **L5-MUPS-PROMPT-02** | Observe appendix 不含 observe.node_role 重复段 | PASS — `TestObservationTaskAppendix_NoNodeRoleDuplicate` |
| **L5-MUPS-PROMPT-03** | Plan/Observe lineframe 行无 `[control]`/`[data]` 前缀 | PASS — `TestBuildLineFrameFromStruct_NoPlanePrefix` |
| **L5-MUPS-PROMPT-04** | Execute materialize 中 workItemBody 在 outputHints 之前 | PASS — `TestExecuteMaterialize_TaskBeforeHints` |
| **L5-MUPS-PROMPT-05** | Execute ZH locale 下 workItem body 字段标签为中文 | PASS — `TestWorkItemExecuteOutputHints_ZHLabels` |

---

## 3. T-Layer Evidence

| T ID | L5 | Status | Test 位置 |
|------|-----|--------|-----------|
| shared-A97-T05 | L5-MUPS-PROMPT-03 | IMPLEMENTED | `prompttags/structbind_test.go::TestBuildLineFrameFromStruct_NoPlanePrefix` |
| shared-A97-T06 | L5-MUPS-PROMPT-01 | IMPLEMENTED | `sessionorchestrator/observe_structbind_test.go::TestBuildObserveUserPrompt_NoControlPlaneFields` |
| shared-A97-T07 | L5-MUPS-PROMPT-02 | IMPLEMENTED | `i18n/prompttags_semantics_render_test.go::TestObservationTaskAppendix_NoNodeRoleDuplicate` |
| shared-A97-T08 | L5-MUPS-PROMPT-04 | IMPLEMENTED | `materialize/phase_prompts_test.go::TestExecuteMaterialize_TaskBeforeHints` |
| shared-A97-T09 | L5-MUPS-PROMPT-05 | IMPLEMENTED | `i18n/format_hints_mups_test.go::TestWorkItemExecuteOutputHints_ZHLabels` |

> **T-Registry note:** 上述 T-ID 是本 change 内部追踪用 (继续 A97 序列)；实质能力归属在 `shared/prompttags` (shared) + `contextengine/i18n` (D2) + `sessionorchestrator` (D7)。本 change 是 §D2-S15-A97 / §shared-A97 同一 capability 的 prompt 净化增量，不新增 t-registry 段。

---

## 4. Test Commands

```bash
go test -count=1 \
  ./internal/shared/prompttags/... \
  ./internal/layers/contextengine/i18n/... \
  ./internal/layers/contextengine/materialize/... \
  ./internal/layers/orchestration/sessionorchestrator/...
# All PASS
```

PR #418 (commit `0bc02410`) MERGED 2026-07-05 09:11:12；CI `unit tests` + `layer-lint (warn)` 双绿；22/22 orchestration packages `go test -race` PASS。

---

## 5. Token Budget Note

净减行：
- Observe user frame：5 行 → 2-3 行 (~40% 减)
- Plan user frame：~7 行 → ~6 行 (去掉 plane 前缀)
- Execute system：3 段重复 → 1 段 (重复 role 删除 ~33% 减)

整体 MUPS pipeline LLM 可见 token 估计 -10%~-15%。Execute 字段标签 i18n 切换无 token 影响。

---

## 6. DM-ID Conflict Note (重要)

DM-20260705-004 此前已被 `mups-plan-structbind` 使用 (PR #405, 已 S7_Archived 2026-07-05, 详见 `openspec/archive/2026-07-05-mups-plan-structbind/acceptance-report.md`).

**两者实质不同**：
- 旧 `mups-plan-structbind`: Plan 节点反射驱动 struct I/O (M2 kernel 复用)
- 本 `mups-node-prompt-dedup`: 三节点 prompt 文本层去冗余 + structbind 对齐

DM-ID 误用属 metadata error。建议未来 change-id 命名遵循 `mups-` 前缀且 DM-ID 强唯一性。

---

## 7. 关联变更

| 变更 | 关系 |
|------|------|
| DM-20260705-003 (mups-semantics-schema-alignment) | 同步: 结构化 SemanticRule 替代 prose 重复 |
| DM-20260705-002 (mups-parse-reject-feedback) | 同步: `prior_parse_reject` 在 Observe user frame 保留 |
| DM-20260705-009 (d7-observe-closed-classifier-prompt) | 后续: 封闭式分类器定位在 Observe body 同步强化 |
| DM-20260705-008 (d7-mups-strategy-injection) | 后续: 5 节点重构总图 M3 依赖本 change 的 prompt 净化基线 |

---

## 8. 领域文档同步

| 文档 | 状态 |
|------|------|
| `openspec/specs/shared/mups-node-llm-protocols.md` §8 | SYNCED (delta: `specs/shared/mups-node-prompt-dedup.md`) |
| `openspec/specs/shared/prompttags.md` | SYNCED (+30 lines schema 对齐) |
| `openspec/specs/d2-context-engine/t-registry.md` | EXISTING (no new section) |
| `openspec/specs/d7-orchestration/t-registry.md` | EXISTING (no new section) |