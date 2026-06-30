---
demand-id: DM-20260630-012
title: MUPS 交付收敛 — LLM 战略提案 + Deliverable Gate
priority: P0
status: S1_Demand
dsaft_domain: orchestration
created: 2026-06-30
reporter: 飞书实测 + Jaeger trace（review d2 kernel 目录）
related:
  - DM-20260630-011 (Session 结论完整性 — Turn 路径 LastTextQualityGate)
  - DM-20260630-001 (Observe G3 LLM D2→D3)
  - DM-20260627-001 (WorkItem Rollup Pipeline)
  - DM-20260628-002 (ChildDownlink ExpectedReturn / DefaultDecomposeProposer)
---

# Demand: MUPS 交付收敛

## 1. 原始诉求

用户向 devrix 发送指令 **「review d2 领域 kernel 目录代码」**。目标目录 `internal/layers/contextengine/kernel/` 仅 **14 个 Go 文件**，但实测：

1. **工具调用 ~94 次**（大量重复 `read_file`），session 耗时数分钟；
2. **飞书「任务总结」** 显示 *"Let me continue exploring the directory to understand the structure and contracts."* — 过渡句，非 review 结论；
3. Jaeger trace 显示多 WorkItem MUPS 轮次（Observe → Plan → Execute → …），子任务 `wi_8ad424ee` 等在 Explore 态循环。

用户期望：**一份带 P0/P1 + file:line 的 code review 结论**；是否分解、如何执行应由 **大模型提案**，代码只守不变量。

## 2. 问题陈述（本质）

| 现象 | 根因（架构层） |
|------|----------------|
| 工具次数远超文件数 | 战略由代码写死（Goal→ExplorationPlan、DefaultDecomposeProposer 固定 2 子任务、SpawnInline 重试）；向下无 scope/budget 约束 |
| 总结是探索过渡句 | Execute 在 `max_iters` 终止，最后一轮文本≠交付物；Session complete 取 `lastArtifactSummary` 而非 Goal rollup deliverable |
| 向上无法收敛 | `VerdictPartial → TaskStatusCompleted`；Bubble 只传 verdict 元数据，不传 findings；ExpectedReturn 无 Verify enforcement |
| LLM 未参与战略 | `MatchKind` / `planQuantizedKind` / `DefaultDecomposeProposer` / `SpawnPolicyEvaluator` 替模型决定拆不拆、怎么拆 |

DM-20260630-011 已修复 **Turn 主链路** 的 LastTextQualityGate + D1 fallback；**ItemPipeline / RunSessionTurnLoop** 路径仍缺交付收敛与质量门。

## 3. 业务目标

| ID | 目标 | 可验证承诺 |
|----|------|------------|
| **DC-1** | **LLM 提案战略，代码守门** | Plan 阶段 LLM 输出 `execution_mode` / `scope_in` / `child_specs`；代码只验路径合法、深度/预算/blast radius |
| **DC-2** | **ExpectedReturn 可验证** | review 类 deliverable 必须满足 schema（含 `file:line`）；不满足则不可 `Completed` |
| **DC-3** | **向上传结论载荷** | 子 WorkItem 完成时 Bubble/Rollup 输入含 `StructuredDeliverable`（findings digest），非仅 verdict 元数据 |
| **DC-4** | **Session 出口 = Goal Deliverable** | `complete` 优先 `ExtractSessionDeliverable`；无合格 deliverable → `TaskIncompleteMessage`（非过渡句） |
| **DC-5** | **Item 路径质量门** | RunSessionTurnLoop `complete` 经 LastTextQualityGate + `summary_quality` / `final_quality`（与 DM-20260630-011 对齐） |

## 4. 澄清记录

| Q | A |
|---|---|
| 「小范围 review」是否由代码 `if fileCount < N` 决定？ | **否**。是否 single/decompose 由 **LLM StrategicPlanProposer 提案**；代码不硬编码文件数阈值 |
| 是否删除 DefaultDecomposeProposer / SpawnPolicy？ | **否（Phase 1）**。作为 **LLM 失败时的 rule fallback**；LLM 成功时优先采用 LLM 提案 |
| 与 DM-20260630-011 关系？ | **扩展**。011 覆盖 Turn finalize；012 覆盖 ItemPipeline + WorkTree 收敛 + Plan LLM 化 |
| review 交付 schema 谁定义？ | 注册表 `deliverable_schema`（首期 `p0_p1_file_line`）；Verify 规则校验，可选 LLM verifier 后续 |

## 5. L1–L5 映射（草案）

| 层级 | 映射 |
|------|------|
| **L1** | D7 Orchestration |
| **L2** | Code review via MUPS WorkTree |
| **L3-BE** | RunSessionTurnLoop + ItemPipelineRunner |
| **L3-FE** | 飞书任务总结卡片（D1 EmitComplete） |
| **L4** | StrategicPlanProposer、DeliverableVerifier、SessionDeliverableGate、StructuredBubble payload |
| **L5（草案）** | 见 proposal.md Success Criteria + tasks.md T 点 |

## 6. In Scope / Out of Scope

### In Scope

- LLM `StrategicPlanProposer` @ Plan（G3：提案 → 规则门控）
- Deliverable schema 校验 @ Verify（review: P0/P1 + file:line）
- Partial + `max_iters` / 缺 deliverable → 禁止 `StatusAfterSpawnNone(Partial)→Completed` 用于无 deliverable 的 review 轮
- StructuredDeliverable 写入 `WorkItemPipelineRound` + 向上 Bubble
- RunSessionTurnLoop complete：deliverable 优先 + LastTextQualityGate
- 过渡句 marker 扩展（`let me continue` 等）@ LastTextQualityGate / DetectEmptyConclusion

### Out of Scope

- 删除 WorkTree / MUPS 五节点
- 新 feature flag（默认 LLM proposer wired；nil → rule fallback，同 ObservationProposer）
- 全量 LLM SpawnPolicy（Phase 2）；Phase 1 仅 Plan 战略 LLM 化
- Bug A 类飞书 streaming dedup 阈值重设计

## 7. Demand 级验收标准

- [ ] **P0** 同指令「review `internal/layers/contextengine/kernel/`」：Session complete **不含**探索过渡句；含 P0/P1 或明确 TaskIncompleteMessage
- [ ] **P0** Jaeger：Plan 子树含 StrategicPlan 提案 span；Verify 含 deliverable_check
- [ ] **P0** 子任务 `stop_reason=max_iters` 且无 file:line → WorkItem **非** Completed（或 Parent 触发 rollup 合成）
- [ ] **P1** LLM Plan 提案 `execution_mode=single` 时，不触发 DefaultDecomposeProposer 固定 2 路拆分
- [ ] **P1** t-registry 登记 D7-S5-A22 / D7-S9-A32 / D7-S16-A76 及 T 点
