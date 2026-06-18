# Proposal: D5/D6 — 信誉、置信度与惩罚闭环（Agent 重复博弈）

**Change ID:** devrix-reputation-feedback-loop
**Demand ID:** DM-20260614-008
**Status:** S7_Archived (2026-06-18; S1_Cancelled; not implemented)
**Author:** Devrix Team
**Date:** 2026-06-14 → Cancelled 2026-06-18

> **取消原因 (2026-06-18):** 创建 4 天未推进；未进入 S2 提案。依赖项 `devrix-d1-sa-refine` v1.1（更上游的 S/A/F 层注册表重命名）也未实施，故信誉/置信度模块缺乏可挂载的 S/A 节点。归档为 S7_Archived（S1_Cancelled → Archived；不再重开）。

## 1. Background

Devrix 多智能体协作中存在"Agent 重复博弈"——同一任务可能被不同 Agent 多次尝试，期望通过**信誉（reputation）+ 置信度（confidence）+ 惩罚（penalty）** 闭环机制，让高信誉 Agent 的结果优先采纳。

## 2. Problem Statement

| 问题 | 影响 |
|------|------|
| 多 Agent 结果冲突时无优先级 | 需手动选择；效率低 |
| 无信誉记录 | 低质量 Agent 持续产生噪声 |
| 无置信度量化 | 无法区分"高置信度答案"与"猜测" |
| 无惩罚机制 | 低质量 Agent 无改进动力 |

## 3. 提案范围（未实施）

预期模块（位于 D5/D6）：
- `internal/layers/observability/reputation/` — 信誉记录与查询
- `internal/layers/observability/confidence/` — 置信度评分
- `internal/layers/evolution/penalty/` — 惩罚策略

跨域集成：
- D1 Communication：信号携带信誉/置信度
- D2 Context Engine：上下文聚合按信誉加权
- D4 Multi-Agent：Agent 选择按信誉排序

## 4. Non-Goals（仍记录）

- 不替代人工审核（最终决策仍归人）
- 不引入外部信誉系统（如区块链）
- 不修改 LLM 调用接口

## 5. 取消决策

**Decision (2026-06-18):** 取消本 change；理由：
1. 4 天未推进（自 2026-06-14 创建以来无 commit）
2. 依赖项 `devrix-d1-sa-refine` v1.1 未实施，缺少可挂载节点
3. 重复博弈场景在当前 devrix 流量下尚未成为痛点
4. 后续若需要，可基于 `devrix-d1-sa-refine` v1.1 重开

## 6. 归档

**Status:** S7_Archived (2026-06-18)
**Verdict:** S1_Cancelled → Archived；不实施；未来按需重开。