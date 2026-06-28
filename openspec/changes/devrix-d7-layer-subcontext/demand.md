# Demand: D7 Layer SubContext — WorkTree 分层 Context 与结构化信号

**Demand ID:** DM-20260627-003  
**Created:** 2026-06-27  
**Reporter:** 用户 + WorkTree/Context 架构讨论  
**Priority:** P1  
**Sprint:** d7-worktree-context-closure  

---

## 1. 原始诉求

> 1. WorkTree 每一层的 **context 应如何处理**？同层 sibling 如何传递信号？上下层如何传递？  
> 2. LLM 调用时的 context **必须从 D2 领域通过统一接口获取**；Context 与 SubContext 可以有差异。  
> 3. 首次（Goal）WorkTree 是否需要 LLM **确定范围收敛**后再 decompose？  
> 4. 避免「session 单桶 history + 规则写了但 LLM 看不到」的断层（ContextGraph F1–F4 已落地，Execute 仍 session Prepare）。  
> 5. **是否**应通过 context 要求 LLM 每轮对话将问题收敛到 MUPS Observe 四类（ObsFact / ObsSignal / ObsDeviation / ObsUncertainty）？边界应如何划分？

## 2. 现象与根因（现状）

| 现象 | 根因 |
|------|------|
| 所有 WorkItem 看到相同 session transcript | `WorkItemExecutor.prepareContext` 仅 `Prepare(sessionID, directive)` |
| ContextGraph Link/Bubble 不影响 LLM | 缺 `MaterializeContext` → D2 闭环 |
| 并行 sibling 要么全隔离要么全混 | 1:1 `wi_<id>` 与「同层 cohort 协作」未分层建模 |
| decompose 子 WI 边界漂移 | Goal 缺 **ScopeContract** 下行契约 |
| Rollup 与 Execute 上下文策略不一致 | Rollup 走 structured bubble（#262 ✅）；Execute 仍读 session |
| Execute 与 Observe 职责混淆风险 | 若每轮 ReAct 强制 LLM 自报 Obs*，与 G3（规则裁决）及 LC2（Signal≠transcript）冲突 |

**Jaeger / 日志证据（讨论期）：** `~/.devrix/logs/llm/unknown.jsonl` 中 post-i18n 请求已中文化，但 WorkItem 路径仍共享 session 级 tools/messages 语义。

## 3. 业务目标（North Star）

| ID | 目标 | 可验证承诺 |
|----|------|------------|
| **LC1** | **分层 SubContext** | depth≥1 的 WI Execute 前，D2 可 Materialize 出与 session 不同的 SubContext（prompt/budget/tools） |
| **LC2** | **信号与全文分离** | 同层/上下层协作靠 **结构化 Signal**，不靠默认共享 ReAct transcript |
| **LC3** | **D2 唯一 LLM 出参** | D7 只传 partition + policy + directive；messages 仅来自 D2 Materialize |
| **LC4** | **Goal 范围收敛** | 开放型任务 Goal 首轮 Plan 产出 **ScopeContract**；未收敛不 decompose |
| **LC5** | **与 Rollup 一致** | 向上仅 BubbleStructured/Summary；Rollup 不读 layer/wi 全文 |
| **LC6** | **Observe 边界** | Execute 产出 **结构化 Signal**（LastRound/ScopeContract）；**Observe 规则**映射为 Obs*；Execute ReAct **不**每轮自报 Obs  taxonomy |

## 4. L1–L5 映射

| 层级 | 映射 |
|------|------|
| **L1** | D7 Orchestration + D2 Context Engine |
| **L2** | WorkTree 分层执行时的上下文隔离与协作 |
| **L3-BE** | Layer cohort partition、ChildDownlink、ScopeContract、D2 Materializer |
| **L3-FE** | `/task context show`、ResolveHint 扩展（F6 延续） |
| **L4** | 见 `proposal.md` §Capabilities |
| **L5** | 见 `specs/*_delta.md` + `tasks.md` |

## 5. Demand 级验收标准

- [ ] **P0** Goal 开放型指令：Plan 产出可解析 `ScopeContract`；`open_questions` 非空时 **不** `SpawnDecompose`（规则门控）。
- [ ] **P0** 子 WI Execute：`Materialize` 的 `system_prompt`/`messages` **≠** 同 session 主 Turn 的完整 transcript（Jaeger `D2_Context_Materialize` 可证）。
- [ ] **P0** 同层 sibling（无 BlockedBy）：A 的 ReAct 全文 **不出现在** B 的 Materialize payload。
- [ ] **P0** `BlockedBy`：下游 WI Materialize 含上游 **structured bubble**，不含上游 wi 私有全文。
- [ ] **P0** 子 terminal：父 Observe 仍含 `BubbleStructured`（与 #262 兼容，不回归）。
- [ ] **P1** `SpawnParallelExplore`：可选 **PeerStatusSignal**（terminal 后 1 行）；默认 **无** live tail 共享。
- [ ] **P1** `FeatureLayerSubContextEnabled=1` 时集成测试全绿；=0 时行为与现网一致。
- [ ] **P0** Execute ReAct transcript **不含** ObsFact/ObsSignal/ObsDeviation/ObsUncertainty 强制标签块（Obs* 仅出现在 Observe 阶段 UncertaintyReport）。
- [ ] **P0** Goal `ScopeContract.open_questions` 非空 → Observe 产出 ObsUncertainty（规则映射）→ SpawnPolicy 阻断 decompose。
- [ ] **P0** 子 WI terminal BubbleStructured → 父 Observe ObsFact（已有 #262 路径，不回归）。

## 6. 关联文档

- ContextGraph SoT：`openspec/specs/d7-orchestration/workitem-context-graph-design.md`（**本 change 修订 CG2 默认语义**）
- Rollup SoT：`openspec/archive/2026-06-27-devrix-d7-workitem-rollup-pipeline/design.md`
- Pipeline SoT：`openspec/specs/d7-orchestration/workitem-pipeline-unification-design.md`
- MUPS Observe SoT：`openspec/specs/d7-orchestration/spec.md` D7-S8-A15（Observation 4 类 × 2 Category）
- Plan 路由 SoT：`openspec/specs/d7-orchestration/pipeline-architecture.md` §4 MatchKind
- D2 边界：`openspec/specs/d2-context-engine/d7-boundary.md`
- 技术债：`openspec/tech-debt/worktree-v2-deferred.md`（TD-WT-02 Wave 投影可复用 Materializer）

## 7. 澄清 Q&A（供 Claude / Review 继续讨论）

| # | 问题 | 当前决议（Draft） |
|---|------|-------------------|
| Q1 | 同层是否共享 ReAct 全文？ | **否（默认）**；仅共享 **ScopeContract cohort 域** + 可选 terminal **PeerStatus** |
| Q2 | 与 CG2「默认隔离」是否冲突？ | **修订 CG2**：隔离指 **LLM transcript**；cohort **域/契约** 可共享 |
| Q3 | Materialize 放 D2 还是 D7？ | **D2**（存储+压缩+prompt）；D7 只解析 partition/policy |
| Q4 | Goal ScopeContract 是否强制 LLM 一轮？ | **结构化产出强制**；极具体指令可 **规则推断** 单文件 scope |
| Q5 | delegate SubTurn 是否 Phase 1 统一？ | **否** — Phase 3 映射 brief/fork/full → MaterializePolicy |
| Q6 | sandbox_slug 与 cohort 关系？ | 有 sandbox 的 WI **强制 WorkItemPrivate**，禁止 peer layer 注入 |
| Q7 | 每轮 LLM 对话是否强制收敛到 Obs 四类？ | **否（Execute 每轮）**；Execute 产出 **结构化 Signal**；**Observe 规则**映射 Obs*；Goal ScopeContract ≈ ObsUncertainty 门控输入 |
| Q8 | Execute context 能否软引导结构化块？ | **可以**（`<conclusion>` / `<open_questions>`），但 **非 SoT**；Observe/Verify 规则升格 |
