# D7 Orchestration — Changelog

> **时间线列表（Lite-Mode）**。每个 change 一行 + 一句话摘要 + 链接到 `archive/`。
>
> - **spec.md 详细 Scenario 演进** = 在 `archive/<change>/specs/` 各 change 目录
> - **当前符合代码的设计契约** = [spec.md](spec.md)（≤ 200 行）
> - **变更类型说明** = IMPLEMENTED（已合入代码）/ PARTIAL（部分合入）/ SUPERSEDED（被替代）/ OBSOLETE（已废弃）
> - **最近 30 天** = 2026-05-31 ~ 2026-06-30，共 46 条 d7 change

---

## 时间线（最近 30 天）

| Date | Change ID | 摘要 | 状态 | 归档 |
|------|-----------|------|------|------|
| 2026-07-01 | devrix-d2-d7-review-hardening | D7 并发硬化 (P0-B PerInvocationEmit + OnReleaseOnce) + D7 错误可观测 (P1-A 4 T 静默吞咽改 slog.Warn / SetRoundPhase span / ResolveRollup warn / DefaultChildExpectedReturn schema tag) + D7 escape ctx cancel (P1-A3 200 cycles no-leak + MUPS ErrChannelCtxCancelled) + D7 规约清理 (P2 arbitrator/strategic_plan_proposer i18n 化 + work_tree.SetStore mu 保护 + decompose_proposer_test 扫描 1→6 文件); 15 T IMPLEMENTED (D7-S1-A80 + D7-S2-A80/A81/A82/A83/A84/A85 + D7-S3-A84 + D7-S9-A33 + D7-S14-A48/A49 + D7-S15-A42 + D7-S16-A77/A78) | IMPLEMENTED | [archive](../../archive/2026-07-01-devrix-d2-d7-review-hardening/) |
| 2026-07-01 | devrix-mups-propagation-convergence | A88 UncertaintyReconcile + A89 RollupTerminationGuard + A90 RollupOutcomeAggregation + A91 AcceptanceCriteriaVisibility + A92 DivergenceBudgetVisibility + A93 SchemaMonotonicNarrowing + A94 ScopeSubdivisionContract + A95 ChildUncertaintyBubble（MUPS 收敛 + Rollup 终止 + 发散范围/验收标准对 LLM 可见） | IMPLEMENTED | [archive](../../archive/2026-07-01-devrix-mups-propagation-convergence/) |
| 2026-06-30 | devrix-mups-deliverable-convergence | A76 StrategicPlanProposer + A32 DeliverableVerifier + Session Deliverable Gate + 战术硬编码清理 | IMPLEMENTED | [archive](../../archive/2026-06-30-devrix-mups-deliverable-convergence/) |
| 2026-06-30 | devrix-d7-observe-unified-llm-path | S16-A75 Observe LLM D2→D3 (SUPERSEDED A74) | IMPLEMENTED | [archive](../../archive/2026-06-30-devrix-d7-observe-unified-llm-path/) |
| 2026-06-29 | devrix-d7-taskcontract-unification-pr-c | S18 CoW VersionChain + Similarity + Hard Evidence | IMPLEMENTED | [archive](../../archive/2026-06-29-devrix-d7-taskcontract-unification-pr-c/) |
| 2026-06-29 | devrix-d7-taskcontract-unification-pr-b | S18-A11/A12 Pessimistic + Rule-based (L3) | IMPLEMENTED | [archive](../../archive/2026-06-29-devrix-d7-taskcontract-unification-pr-b/) |
| 2026-06-29 | devrix-d7-taskcontract-unification-pr-a | S20/S21 TaskSpec + TaskReport 契约 | IMPLEMENTED | [archive](../../archive/2026-06-29-devrix-d7-taskcontract-unification-pr-a/) |
| 2026-06-29 | devrix-d7-taskcontract-unification | v7.0 TaskContract 统一（三件套汇总） | IMPLEMENTED | [archive](../../archive/2026-06-29-devrix-d7-taskcontract-unification/) |
| 2026-06-29 | devrix-d7-mups-v4-5node-coverage-orchestration | 5 节点 Span + root span + 跨包 import 治理 | IMPLEMENTED | [archive](../../archive/2026-06-29-devrix-d7-mups-v4-5node-coverage-orchestration/) |
| 2026-06-29 | devrix-d7-multiturn-session-state | multi-turn session state 安全机制 | IMPLEMENTED | [archive](../../archive/2026-06-29-devrix-d7-multiturn-session-state/) |
| 2026-06-29 | devrix-d7-dsaft-restructuring | D7-S* 重编号 11 PR / 55 T | IMPLEMENTED | [archive](../../archive/2026-06-29-devrix-d7-dsaft-restructuring/) |
| 2026-06-29 | devrix-d7-6s-observe-merge-cancel | Observe 合并 + cancel 路径 | IMPLEMENTED | [archive](../../archive/2026-06-29-devrix-d7-6s-observe-merge-cancel/) |
| 2026-06-28 | devrix-d7-layer-subcontext-phase3 | Layer SubContext Phase 3 Wave ContextResolver→MaterializePolicy | IMPLEMENTED | [archive](../../archive/2026-06-28-devrix-d7-layer-subcontext-phase3/) |
| 2026-06-28 | devrix-d7-layer-subcontext | Layer SubContext Phase 1+2 Per-Layer SubContext + ChildDownlink | IMPLEMENTED | [archive](../../archive/2026-06-28-devrix-d7-layer-subcontext/) |
| 2026-06-27 | devrix-d7-workitem-rollup-pipeline | WorkItem Rollup 闭环（Parent Gate + Root Fallback） | IMPLEMENTED | [archive](../../archive/2026-06-27-devrix-d7-workitem-rollup-pipeline/) |
| 2026-06-27 | devrix-d7-itempipeline-emit-hook | ItemPipelineRunner emit chain 补齐 | IMPLEMENTED | [archive](../../archive/2026-06-27-devrix-d7-itempipeline-emit-hook/) |
| 2026-06-26 | devrix-d7-workitem-context-graph | WorkItem Context Graph 设计 + 物理迁移 | IMPLEMENTED | [archive](../../archive/2026-06-26-devrix-d7-workitem-context-graph/) |
| 2026-06-26 | devrix-d7-six-s-simplification | 14 S → 6 S 简化（v6.0.0 域升级） | IMPLEMENTED | [archive](../../archive/2026-06-26-devrix-d7-six-s-simplification/) |
| 2026-06-26 | devrix-d7-mups-package-migration | execute/ + learn/ → mups/ 子树物理迁移 | IMPLEMENTED | [archive](../../archive/2026-06-26-devrix-d7-mups-package-migration/) |
| 2026-06-26 | devrix-d7-hardening-cross-cutting | 跨切面 hardening 集合 | IMPLEMENTED | [archive](../../archive/2026-06-26-devrix-d7-hardening-cross-cutting/) |
| 2026-06-26 | devrix-d7-6s-verify-promotion | verify 包从 sessionorchestrator/ → executionflow/verify/ 物理 promote | IMPLEMENTED | [archive](../../archive/2026-06-26-devrix-d7-6s-verify-promotion/) |
| 2026-06-26 | devrix-d7-6s-package-merge | turn/ 整包 → sessionorchestrator/ 物理合并 | IMPLEMENTED | [archive](../../archive/2026-06-26-devrix-d7-6s-package-merge/) |
| 2026-06-26 | devrix-d7-6s-bootstrap-slim | InitOrchestration 275 → 140 行瘦身 | IMPLEMENTED | [archive](../../archive/2026-06-26-devrix-d7-6s-bootstrap-slim/) |
| 2026-06-25 | devrix-d7-package-cleanup-sprint | 4 遗留小子包物理合并（runregistry/toolpolicy/d7spans/sessionqueue） | IMPLEMENTED | [archive](../../archive/2026-06-25-devrix-d7-package-cleanup-sprint/) |
| 2026-06-25 | devrix-d7-mups-v5-escape-engine-v5-6-review-fixes | v5 续跑 review fixes（8 applied + 6 deferred） | IMPLEMENTED | [archive](../../archive/2026-06-25-devrix-d7-mups-v5-escape-engine-v5-6-review-fixes/) |
| 2026-06-25 | devrix-d7-mups-v5-escape-engine-v5-6 | v5 续跑入口收口（ResumeSession + applyResumeSession） | IMPLEMENTED | [archive](../../archive/2026-06-25-devrix-d7-mups-v5-escape-engine-v5-6/) |
| 2026-06-25 | devrix-d7-mups-v5-escape-engine | MUPS v5 EscapeEngine 统一逃逸机制（17/17 P0 T） | IMPLEMENTED | [archive](../../archive/2026-06-25-devrix-d7-mups-v5-escape-engine/) |
| 2026-06-25 | devrix-d7-mups-v4-review-fixes-series | v4 review fixes 系列（DM-005 + DM-006 双 PR 收尾） | IMPLEMENTED | [archive](../../archive/2026-06-25-devrix-d7-mups-v4-review-fixes-series/) |
| 2026-06-25 | devrix-d7-mups-v4-review-fixes | v4 hotfix 路径 14 fix（3 Critical + 10 High + 1 doc） | IMPLEMENTED | [archive](../../archive/2026-06-25-devrix-d7-mups-v4-review-fixes/) |
| 2026-06-25 | devrix-d7-mups-v4-phase7-verify-auto-close | Phase 7 运行时 5 节点闭环（processAutoClose + TrackMode） | IMPLEMENTED | [archive](../../archive/2026-06-25-devrix-d7-mups-v4-phase7-verify-auto-close/) |
| 2026-06-24 | devrix-d7-mups-v4-phase6-observe-learner-wiring | Phase 6 Observe-Learner 跨域闭环集成 | IMPLEMENTED | [archive](../../archive/2026-06-24-devrix-d7-mups-v4-phase6-observe-learner-wiring/) |
| 2026-06-23 | devrix-d7-mups-v4-phase5-learn | Phase 5 Learn 节点升格（LearningAsset 5 类 + ReputationEvidence） | IMPLEMENTED | [archive](../../archive/2026-06-23-devrix-d7-mups-v4-phase5-learn/) |
| 2026-06-23 | devrix-d7-mups-v4-phase4-verify-promotion | Phase 4 Verify 节点升格（VerdictKind 4 态 + 14 ExitReason） | IMPLEMENTED | [archive](../../archive/2026-06-23-devrix-d7-mups-v4-phase4-verify-promotion/) |
| 2026-06-23 | devrix-d7-mups-v4-phase3-execute | Phase 3 PR-C1 Execute Artifact 4 类 + SideEffectStatus 5 态 | IMPLEMENTED | [archive](../../archive/2026-06-23-devrix-d7-mups-v4-phase3-execute/) |
| 2026-06-23 | devrix-d7-mups-v4-phase3-channels | Phase 3 PR-C2 Execute 4 Channel + ChannelRouter | IMPLEMENTED | [archive](../../archive/2026-06-23-devrix-d7-mups-v4-phase3-channels/) |
| 2026-06-23 | devrix-d7-mups-v4-phase2-plan | Phase 2 PR-B1 Plan 4 类 + Planner + MatchKind 4 Rules | IMPLEMENTED | [archive](../../archive/2026-06-23-devrix-d7-mups-v4-phase2-plan/) |
| 2026-06-23 | devrix-d7-mups-v4-phase2-observe-plan | Phase 2 PR-A1 + PR-RF Observation 4 类 + UncertaintyReport | IMPLEMENTED | [archive](../../archive/2026-06-23-devrix-d7-mups-v4-phase2-observe-plan/) |
| 2026-06-22 | devrix-d7-metrics-and-concurrency-hardening | 5 P0/P1 metric 命名对齐 + 并发硬化 | IMPLEMENTED | [archive](../../archive/2026-06-22-devrix-d7-metrics-and-concurrency-hardening/) |
| 2026-06-21 | devrix-d7-error-aggregation-and-metrics | 错误聚合 + worktree 全链路 metrics（5 silent failure 模式） | IMPLEMENTED | [archive](../../archive/2026-06-21-devrix-d7-error-aggregation-and-metrics/) |
| 2026-06-19 | devrix-d7-v2-structure | D7 v2.0 Structure（19/19 AC + 7 layout guards） | IMPLEMENTED | [archive](../../archive/2026-06-19-devrix-d7-v2-structure/) |
| 2026-06-17 | devrix-d7-turn-history-persist | Turn 历史持久化 | IMPLEMENTED | [archive](../../archive/2026-06-17-devrix-d7-turn-history-persist/) |
| 2026-06-16 | devrix-d7-uncertainty-gaps | D7 5 uncertainty gaps 修复（PlanAgent sandbox + PlanMode guard） | IMPLEMENTED | [archive](../../archive/2026-06-16-devrix-d7-uncertainty-gaps/) |
| 2026-06-16 | devrix-d7-orthogonal-intent-paths | 4 IntentKind = 4 独立执行链 | IMPLEMENTED | [archive](../../archive/2026-06-16-devrix-d7-orthogonal-intent-paths/) |
| 2026-06-16 | devrix-d7-loop-first-routing | Loop-First 路由（FastPath 优化） | IMPLEMENTED | [archive](../../archive/2026-06-16-devrix-d7-loop-first-routing/) |
| 2026-06-15 | devrix-d7-turn-orchestration | Turn 编排上移（DM-020） | IMPLEMENTED | [archive](../../archive/2026-06-15-devrix-d7-turn-orchestration/) |
| 2026-06-14 | devrix-d7-sa-refine | D7 SA Refine（博弈论角色对齐） | IMPLEMENTED | [archive](../../archive/2026-06-14-devrix-d7-sa-refine/) |
| 2026-06-14 | devrix-d7-s5-t02-planagent-whitelist | S5-T02 PlanAgent 白名单 | IMPLEMENTED | [archive](../../archive/2026-06-14-devrix-d7-s5-t02-planagent-whitelist/) |
| 2026-06-14 | devrix-d7-orchestration-domain | D7 Orchestration 域定义（基线） | IMPLEMENTED | [archive](../../archive/2026-06-14-devrix-d7-orchestration-domain/) |
| 2026-06-14 | devrix-d7-classify-command-first | ClassifyCommandFirst 路径优化 | IMPLEMENTED | [archive](../../archive/2026-06-14-devrix-d7-classify-command-first/) |
| 2026-06-14 | devrix-d1-d7-only-ingress | D1-D7 单一入口收口 | IMPLEMENTED | [archive](../../archive/2026-06-14-devrix-d1-d7-only-ingress/) |

---

## 变更类型分布

| 状态 | 数量 | 说明 |
|------|------|------|
| **IMPLEMENTED** | 46 | 已合入代码，需求态转代码态 |
| **PARTIAL** | 0 | 当前所有 d7 change 都已完整合入 |
| **SUPERSEDED** | 1 | 2026-06-30 observe-unified-llm-path SUPERSEDED A74 裸 D3 |
| **OBSOLETE** | 0 | 当前所有 d7 change 仍可追溯 |
| **S1_CANCELLED** | 0 | 无取消的 d7 change（devrix-d7-spec-split 不在此处） |

---

## 历史归档（早于 30 天）

如需查阅 30 天前的 d7 历史，访问 `openspec/archive/` 目录，命名格式 `YYYY-MM-DD-devrix-d7-*`。`openspec/demand-archive-index.md` 包含全部归档记录的元信息（Demand ID / 标题 / 归档日期 / PR / Verdict）。

---

## 状态映射（spec.md 索引）

| spec.md 段 | 描述 | 对应 archive 历史 |
|-----------|------|-----------------|
| Overview | D7 域职责 + 5 节点管道 + v4.3 post-cleanup 物理路径 | 全部 IMPLEMENTED 汇总 |
| DSAFT 结构 | 1 D + 6 S + 22 A + 120 F + 180 T | devrix-d7-dsaft-restructuring 完整重编号 |
| Scenarios 表 | S1-S21 17 个 S 层 | 每个 S 层对应多个 archive change |
| Architecture | 5 节点管道 + D1/D2/D3/D4 上下游接口 | devrix-d7-v2-structure + devrix-d7-mups-v4-* (Phase 1-7) |
| 关键 Scenario 范式 | 1-2 canonical Gherkin 范式 | 完整 174 个 Scenario 在 archive/<change>/specs/ |
| 关键链路口 | 6 端到端路径 | 全部 archive change 累积形成 |

---

## 维护规则

- **新增 change 时**：归档时（`changes/<id>/` → `archive/<date>-<id>/`）追加一行，按 `Date | Change ID | 摘要 | 状态 | 归档` 格式
- **架构级变更时**：修订 [spec.md](spec.md) 主体段（Overview / DSAFT / Architecture / 关键 Scenario 范式）
- **超 300 行时**：精简为一行摘要 + 归档链接；超期条目（> 30 天）折叠到「历史归档」段
- **禁止**：复制 Requirement/Scenario 详细文本到本文件；创建子文件（lite-mode 不需要）
- **S3-Gate 检查**：本文件 ≤ 300 行（硬上限）
