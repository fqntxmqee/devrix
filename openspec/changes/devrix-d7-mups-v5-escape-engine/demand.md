---
demand-id: DM-20260625-003
title: D7 MUPS v5 EscapeEngine — 回路深度统一逃逸机制 (LoopDepthTracker v2 + 3 层仲裁 + 5 类兜底)
priority: P0
status: S2_Proposed
dsaft_domain: orchestration
created: 2026-06-25
sprint: mups-v5
---

# D7 MUPS v5 EscapeEngine — 回路深度统一逃逸机制

## 1. 背景

2026-06-22 完成 MUPS v4.3 完整方法论沉淀（`brain/.../core-concepts/38-mature-uncertainty-methodology.md`，4076 行），其中 §21 提出 **v5 Loop 深度上限与逃逸机制完整设计**。

2026-06-23~25 完成 devrix MUPS v4 7 个 Phase + 1 个 review-fixes 全部 S7_Archived：
- 5 节点数据契约（Observation / Plan / Artifact / Verdict / LearningAsset）
- Verify 4 态 + AggregateVerdicts + 14 ExitReason
- 3 Memory 通道 + ReputationEvidence Bayesian
- LP-1 闭环（Observe 注入 prior）
- Auto-Close + TrackMode 3 层解析

**关键发现**：devrix MUPS v4 已实现 5 节点管道，但**未统一逃逸机制**——4 类深度限制（回路深度 / DenialBudget / CircuitBreaker / CompensationAction）分散在多个沉淀中，且 **LLM 可通过切换 Plan.Kind 绕过回路深度计数**（doc 38 §21.2 关键漏洞）。

## 2. 范围

**本 demand 解决 3 个问题**：
1. **回路深度计数器缺失** → 实现 LoopDepthTracker v2（按"模式 hash"计数）
2. **LLM 切换 Plan.Kind 绕过回路** → 实现 PlanKindSwitchPolicy（3 档：Constrained/Allowed/Forbidden）
3. **多类兜底未统一抽象** → 实现 EscapeEngine（统一逃逸入口 + 3 层仲裁 + 5 类兜底动作）

**SOT 引用**：`brain/01知识探索/项目/20260620-certain-architecture/core-concepts/38-mature-uncertainty-methodology.md §21`

**不**包含：
- 5 节点管道本身（已 S7_Archived）
- 跨 turn Learn 注入（已 S7_Archived）
- Doc 38 §18 P0 盲点 / §19 Clawcode 借鉴（已落地 v4，v5 候选）

## 3. 5 PR 拆分（6.5 天工作量）

| PR | 内容 | 工作量 | 依赖 | AC |
|----|------|--------|------|-----|
| V5.1 | LoopDepthTracker v2（按模式 hash）| 1 天 | doc 38 §19.2 | AC1 |
| V5.2 | PlanKindSwitchPolicy 3 档 + 切换计数 | 0.5 天 | 无 | AC2 |
| V5.3 | ChainedArbitrator (LLM/Rule/Human) | 2 天 | V5.1 | AC3 |
| V5.4 | EscapeEngine 整合 + CircuitBreaker 5 层接线 | 1 天 | V5.3 | AC4 |
| V5.5 | 单元测试 + 集成测试 + 5 节点 EscapeEngine 接线 | 2 天 | 上述全部 | AC5-AC8 |
| **合计** | | **6.5 天** | | 8 AC |

## 4. AC 验收标准

| ID | 验收 | 优先级 |
|----|------|--------|
| **AC1** | LoopDepthTracker 按模式 hash(PlanKind+ObsKind+FailureCriterion+ArtifactType) 计数，同一模式重复 depth++，不同模式 reset；MaxDepth=3 | P0 |
| **AC2** | PlanKindSwitchPolicy 3 档：ExplorationPlan=Constrained (≤4) / ScenarioPlan=Allowed / ProtocolPlan=Constrained (≤4) / CommitmentPlan=Forbidden | P0 |
| **AC3** | ChainedArbitrator 3 层仲裁：LLM 自决 → Rule 强制 → Human 接管 | P0 |
| **AC4** | EscapeEngine 整合 3 类深度限制（回路深度 / DenialBudget / CircuitBreaker）+ 5 类兜底动作（Continue/EscalateToRule/EscalateToHuman/ForceExit/AbortWithAudit） | P0 |
| **AC5** | 5 节点完整接线：每个 Plan→Execute→Verify→[Compensation] 路径都通过 EscapeEngine.Evaluate(ctx) | P0 |
| **AC6** | LLM 切换 Plan.Kind 累计 ≤4（PlanKindSwitchCount），超过 → EscapeForceExit | P0 |
| **AC7** | CircuitBreaker 5 层接线：L0 AnomalyDetector (5 nil) / L2 Verifier (3 > 2s) / L3 Hook (5 fail) → open + fallback | P0 |
| **AC8** | 单元测试 100% PASS + 集成测试覆盖 4 类深度限制 + 3 层仲裁 + 5 类兜底动作 + 0 race | P0 |

## 5. SoT 引用

- **设计 SoT**：`core-concepts/38-mature-uncertainty-methodology.md §21`（行 3621-4025，400 行 v5 完整设计）
- **数据契约**：`core-concepts/37-process-behavior-data-model.md`（10 实体 + 5 维度）
- **方法论**：`core-concepts/35-d7-methodology-upgrade-five-node-pipeline.md`

## 6. devrix 现状对照

详见 `proposal.md` §3 差距分析表（16 行 SoT vs 实际）。

## 7. 风险

- **R1**：PlanKindSwitchPolicy 阈值（>4）是 doc 38 §21.13 诚实声明的"估计值，需要根据实际场景调优"
  - Mitigation：V5.5 集成测试覆盖阈值边界 + 加可配置常量
- **R2**：3 层仲裁可能增加响应延迟（EscalateToHuman 阶段等待用户输入）
  - Mitigation：LLM 阶段超时 5s 兜底为 ForceExit，不阻塞主链路
- **R3**：CircuitBreaker 5 层与现有 5 metrics 重叠（dispatch_loop_wakeups / worker_panics / state.cancels / state.handles / sandbox_exit_failed）
  - Mitigation：V5.4 显式选择哪些 metric 升级为 circuit breaker，保留其余为纯 metric

## 8. 后续 v5+ 候选

- **Doc 38 §18.14 v5 候选**：ToolSurface / Compaction / ReferencePropagation
- **5 类不确定性合并为 4 类**（doc 38 §17 已列入 v5 候选）

## 9. References

- `openspec/changes/devrix-d7-mups-v5-escape-engine/proposal.md`
- `openspec/changes/devrix-d7-mups-v5-escape-engine/design.md`
- `openspec/changes/devrix-d7-mups-v5-escape-engine/tasks.md`
- `brain/.../core-concepts/38-mature-uncertainty-methodology.md §21`
- 9 个 MUPS v4 归档（Phase 1-7 + review-fixes）
- `openspec/specs/d7-orchestration/pipeline-architecture.md`
