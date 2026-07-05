---
demand-id: DM-20260705-003
title: "MUPS semantics schema alignment — Structured SemanticBlock vs i18n prose"
source: 架构评审（prompttags v3 语义层后续）
priority: P0
status: S4_Development
l1-domain: shared, contextengine
created: 2026-07-05
related:
  - openspec/specs/shared/prompttags.md
  - internal/shared/prompttags/semantics.go
  - internal/layers/contextengine/i18n/prompttags_semantics_render.go
parent_demands:
  - DM-20260705-001  # tag semantics layer
  - DM-20260705-002  # parse reject feedback
---

# MUPS semantics schema alignment

## 1. 原始描述

> DM-20260705-001 引入 `TagSemanticsRegistry` 与 i18n 短 bullet appendix，但语义仍以 `prompttagsSemantics_{zh,en}.go` 中 **per-target prose key** 为 SoT，与 machine DocBlock / LineFrameRegistry 存在双写。需要将语义层重构为 **locale-neutral SemanticRule + SemanticCondition**，i18n 仅保留 glossary overlay。

## 2. 问题陈述

| 现状 | 风险 |
|------|------|
| i18n map key 形如 `observe.kind.obs_uncertainty.when_use` | 与 `SemanticsForPhase.OutputRules` 双写；新增 field 需改两处 |
| `PhaseSemantics.InputRules` 手工列表 | 与 `LineFrameRegistry` 字段顺序可能漂移 |
| bullet prose 无 machine-readable 形态 | LLM 难以对齐 Go gate JSON（ParseRejectRecord 已有先例） |

## 3. 目标（P0）

1. `SemanticCondition` 枚举 — locale-neutral machine codes
2. `SemanticRule` → `MachineLine()` 输出 compact JSON（同 ParseRejectRecord profile）
3. `SemanticBlock(phase)` — locale-neutral JSON-lines
4. `InputRulesForFrame()` — 从 LineFrameRegistry 派生 input semantics
5. i18n = thin glossary（condition code → zh/en label）+ node role only

## 4. L5 测试点

| ID | Given-When-Then | Priority |
|----|-----------------|----------|
| L5-MUPS-TAG-01 | Given Observe appendix, When rendered, Then contains `obs_uncertainty` machine rule + scope_unclear glossary | P0 |
| L5-MUPS-TAG-02 | Given Plan appendix, When rendered, Then contains execution_mode machine rule + uncertainty_mean/decompose glossary | P0 |
| L5-MUPS-TAG-03 | Given Execute hints, When rendered, Then contains deliverable_contract/findings_json machine rules + Required/Optional glossary | P0 |
| L5-MUPS-TAG-04 | Given zh/en appendix, When hashed, Then golden snapshot stable | P0 |

## 5. 非目标

- DocBlock schema 语法变更
- D7 proposer 行为变更
- 新增 envelope tag
