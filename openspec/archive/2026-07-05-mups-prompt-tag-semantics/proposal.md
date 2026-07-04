# Proposal: MUPS 标签语义层 — Observe/Plan/Execute 动态提示词

**Change ID:** `mups-prompt-tag-semantics`  
**Demand ID:** DM-20260705-001  
**Created:** 2026-07-05  
**Status:** S3_Planning  
**Demand:** [`demand.md`](demand.md)

---

## 1. Problem Statement

DM-004/005 建立了 `prompttags` **形态层**（registry、DocBlock 语法行、lineframe、wholebody parse、Observe max-3 enforce）。三节点动态 prompt 仍主要向 LLM 暴露：

- 一行 JSON schema（Observe/Plan 输出）
- envelope tag 语法列表（Execute 输出）
- 无注释的 `key: value` user frame（Observe/Plan 输入）

**后果**：模型难以选用正确的 `obs_*` kind、`execution_mode`、Execute envelope 块；Verify 侧 incomplete 增多；不确定性拆解依赖 Go 确定性 Obs 兜底而非 LLM 提案质量。

## 2. Proposed Solution

在 `internal/shared/prompttags/` 新增 **TagSemanticsRegistry**（语义 SoT），D2 i18n/materialize 组装时注入 **PhaseSemanticAppendix** 与 **UserFrameFieldDoc**，不修改 D7 编排 Go 战术散文。

```text
TagSemanticsRegistry (Go, locale-neutral keys)
        ↓
i18n.SemanticAppendix(phase, locale)   ← zh/en 短 bullet
        ↓
BuildPhaseAppendix / BuildLineFrameHeader
        ↓
AssembleMUPSSystemPrompt / user message
```

### 2.1 设计原则

| 原则 | 说明 |
|------|------|
| **Syntax vs semantics split** | DocBlock 保留 machine syntax；语义进 SemanticsRegistry |
| **Data vs control plane** | 每个 tag/field 标注 `plane: data \| control` |
| **Enforce-aligned** | prompt 声明与 Go gate 一致；advisory-only 须标注 |
| **Token budget** | 每节点语义段 ≤ ~400 tokens（zh）；表格/单行 bullet |
| **六节点边界** | LLM 仅 Observe/Plan/Execute；Verify/Learn/Decide 在语义 doc 中一笔带过 |

## 3. Capabilities

| ID | Capability | Layer | Owner |
|----|------------|-------|-------|
| **shared-A97** | TagSemanticsRegistry | L4 shared | `internal/shared/prompttags/semantics.go` |
| **D2-S15-A97** | PhaseSemanticAppendix 组装 | L4 D2 | `materialize/phase_prompts.go`, `i18n/format_hints_mups.go`, `i18n/prompt_dynamic.go`, `i18n/workitem_execute.go` |
| **D2-S15-A97** | UserFrameFieldDoc 注入 | L4 D2 | `prompttags/linefield.go` + i18n |
| **D7-S5-A97** | （无 D7 代码变更）proposer 消费 D2 prepared prompt | L3 D7 | 仅测试断言 appendix 含语义段 |

## 4. Deliverables by node

### Observe

| 面 | 增量 |
|----|------|
| **输出 wholebody** | 四种 `obs_*` When/When-not；`strength`/`question`/`evidence` 字段说明 |
| **输入 lineframe** | `prior_mean`, `scope_open_question`, `incremental_only` 控制/数据面说明 |
| **边界** | 不重复 Go 已产 Obs；`incremental_only` 与 appendix 规则对齐 |

### Plan

| 面 | 增量 |
|----|------|
| **输出 wholebody** | `execution_mode` 决策树；`deliverable_contract` 合法组合示例；与 budget 对齐 |
| **输入 lineframe** | budget 字段 = control；`observation_summary` = data；`uncertainty_mean` → 禁 single（与 CC-U4 对齐） |
| **边界** | child_specs max 2 与 enforce 一致 |

### Execute

| 面 | 增量 |
|----|------|
| **输出 envelope** | 各 tag Required/Optional/HumanProse；findings_json vs contract 分段 |
| **输入** | wiBody 字段 + 实例 tag 为 **输入契约**；输出 tag 为 **交付契约** |
| **边界** | 禁止 Obs  taxonomy（已有）；说明 Obs 已在 Observe 完成 |

## 5. Migration plan

| Phase | Scope |
|-------|-------|
| **P0** | SemanticsRegistry + Observe/Plan/Execute appendix 语义段 + unit tests |
| **P1** | User frame field doc header + zh/en golden hash tests |
| **P2** | defer — parse reject → next-round prompt（DM-005 P2 或子 change） |

## 6. Non-goals

- devrix_core 静态 base 重写
- 同一轮 LLM format-hint retry
- 新 envelope tag 类型（除非设计评审确认必需）
- Verify/Learn/Decide 实现变更

## 7. Success metrics

- P0 L5 三项（Observe kind / Plan mode / Execute required tags）测试 PASS
- `deliverable_incomplete` 中 `planning_meta` / `findings_json_incomplete` 占比下降（staging 对比，S5 记录）
- Appendix token 增量可测量（Materialize token est 不突破现有 budget 告警线）
