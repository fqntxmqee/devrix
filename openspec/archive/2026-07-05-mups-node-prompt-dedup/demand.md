# Demand: MUPS 三节点 Prompt 去冗余与结构优化

**ID:** DM-20260705-004  
**Status:** S4 In Progress  
**Created:** 2026-07-05

## 背景

Observe / Plan / Execute 组装后的 prompt 存在：角色重复、plane 标签三套同名、user guide 列全字段但正文省略、lineframe `[control]`/`[data]` 与前缀 guide 三重重复。详见 `openspec/specs/shared/mups-node-llm-protocols.md` §8。

## 目标

1. **Observe**：user 帧仅暴露分类器所需 data 字段 + `prior_parse_reject`；去掉行前缀与 orchestration-only control 字段。
2. **Plan**：去掉 lineframe 行前缀与 planeGuide；guide 仅列出现字段（已有 fieldMap）。
3. **Execute**：去掉 semantic 中 node_role 与 body intro 重复；字段标签 i18n；System 顺序改为 task → outputHints。
4. **共用**：Observe/Plan phase appendix 不再重复 `observe.node_role` / `plan.node_role`；`BuildLineFrameFromStruct` 默认无 plane 前缀。

## L5 测试点（草案）

| ID | Given-When-Then | P |
|----|-----------------|---|
| L5-MUPS-PROMPT-01 | Given Observe user prompt, When built, Then 不含 work_item_id/prior_mean/incremental_only 行 | P0 |
| L5-MUPS-PROMPT-02 | Given Observe appendix, When rendered, Then 不含 observe.node_role 重复段 | P0 |
| L5-MUPS-PROMPT-03 | Given Plan/Observe lineframe, When serialized, Then 行无 `[control]`/`[data]` 前缀 | P0 |
| L5-MUPS-PROMPT-04 | Given Execute materialize, When assembled, Then workItemBody 在 outputHints 之前 | P1 |
| L5-MUPS-PROMPT-05 | Given Execute ZH locale, When workItem body, Then 字段标签为中文 | P1 |

## 非目标

- observe/plan 轻量 staticBase（PrepareBase 裁剪）— 另 demand
- Plan execution_mode 决策表 JSON 化 — 随 DM-20260705-003 后续
