# Proposal: D7 Layer SubContext — cohort 域 + 结构化信号 + D2 Materialize

**Change ID:** `devrix-d7-layer-subcontext`  
**Demand ID:** DM-20260627-003  
**Priority:** P1  
**Status:** S4_Development — R1 决议已冻结（2026-06-28，见 `review-r1.md`）  
**Parent SoT:** `workitem-context-graph-design.md` · `workitem-pipeline-unification-design.md` · rollup Phase 1（已归档）

---

## 1. Problem Statement

WorkItem Pipeline 与 ContextGraph（PR #244）已落地 **工作语义树 + Link/Bubble 规则**，但 **LLM 执行上下文仍未分层**：

1. **Execute 仍用 session 单桶**：`WorkItemExecutor` → `ContextPreparer.Prepare(sessionID)`，忽略 `ContextScopeID`、cohort、BlockedBy。  
2. **Materialize 未实现**：设计 §8.4 `MaterializeContext` 无代码；规则与 LLM 输入断层。  
3. **同层协作语义不清**：「同层共享 context」若理解为共享 ReAct 全文，与 CG2 冲突且引入并行污染。  
4. **Goal 缺范围收敛**：开放型任务 decompose 前无 **ScopeContract**，子 WI 边界漂移。  
5. **SubContext 与 Session Context 无差异契约**：无法实现「下层更小 prompt、更窄 tools、更短 budget」。  
6. **Execute 与 Observe 边界模糊**：若通过 context 要求 LLM **每轮**自报 ObsFact/ObsSignal/ObsDeviation/ObsUncertainty，会污染 ReAct transcript、虚高 ObsFact Strength，并违反 G3（SpawnPolicy 须规则裁决）。

**谁受影响：** 所有 multi-WI session（review、架构探索、decompose）；Jaeger 运维；D2/D7 边界维护者。

## 2. 为什么要做（Why）

| 驱动力 | 说明 |
|--------|------|
| **可审计** | 每层 WI 应能回答「LLM 当时看见了什么」——需 partition 级 Materialize 日志 |
| **可收敛** | 大任务需 Goal 范围收敛后再拆，避免 11 轮串行发散（rollup demand 先例） |
| **可协作** | 同层 sibling 需 **信号**（verdict/summary/scope），非全文互灌 |
| **可扩展** | D2 统一 Materialize 后，Wave / delegate / WorkItem 共用物化引擎 |
| **控成本** | SubContext 逐层衰减 token budget，避免 N×ReAct 撑爆 session |
| **MUPS 正交** | Obs* 是 Plan 输入契约；Execute 只产 Signal，Observe 规则映射，避免 LLM 独断 spawn |

**不做的代价：** ContextGraph 永久停留在「metadata 层」；WorkTree 深度增加时 context 噪声线性增长；Obs  taxonomy 若混入 Execute 则 LP-5 血缘不可审计。

## 3. Proposed Solution（摘要）

### 3.1 核心模型：Context ≠ Signal ≠ Observation

- **Context（D2）**：LLM 的 messages + system prompt + tools → 仅经 `Materialize` 产出。  
- **Signal（D7）**：结构化事实 → `LastRound`、Bubble、Link、ScopeContract、ChildDownlink → Materialize **注入**，不默认 append 到 sibling 私有链。  
- **Observation（MUPS Observe）**：`ObsFact / ObsSignal / ObsDeviation / ObsUncertainty` × `CatBusiness/CatSystem` → **UncertaintyReport**，供 Plan `MatchKind` 消费；**仅 Observe 节点（规则为主）产出**，非 Execute 每轮自报。

### 3.2 Partition 模型（修订 CG1）

| Partition | Key | 绑定 | 默认写入 |
|-----------|-----|------|----------|
| SessionContext | `session:<sid>` | L0 Goal / 主 Turn | session transcript + `wi:goal` |
| LayerCohort | `cohort:<sid>:<parent_wi_id>` | 同 Parent 兄弟 **域** | **仅 Signal 元数据**，非 ReAct |
| WorkItemPrivate | `wi:<sid>:<wi_id>` | 单 WI Execute | ReAct append **默认此处** |
| AgentSubContext | `agent:<sid>:<agent_id>` | delegate（Phase 3） | 现有 sidechain |

**关键修订：** 「同层共享 context」= 共享 **ScopeContract + cohort 域标签**，**不**共享 WorkItemPrivate 全文。

### 3.3 信号通道

| 方向 | 通道 | 内容 |
|------|------|------|
| **父→子** | ChildDownlink | Directive, ScopeIn/Out, ExpectedReturn, FailureCriteria |
| **子→父** | BubbleStructured（强制）+ BubbleSummary（可选） | 已有 #262 + ContextGraph CB |
| **同层串行** | UpstreamSignal（BlockedBy） | 上游 structured + summary |
| **同层并行** | PeerStatusSignal（可选，P1） | 仅 terminal：wi_id, verdict, 1-line |

### 3.4 Goal ScopeContract

Goal 首轮 Plan **必须**产出（或规则推断）`ScopeContract`；`open_questions` 非空 → 阻断 `SpawnDecompose`。

### 3.5 D2 API

新增 `ContextMaterializer`（或 `PrepareForWorkItem`）：`Materialize(partition, policy, directive) → MaterializedContext`；Execute 后 `Append(wi:private)`。

### 3.6 MUPS Observe 四类 × Execute Context（架构决议）

**问题：** 是否通过 context 要求 LLM 每轮对话将问题收敛到 Obs 四类？

| 策略 | 决议 | 理由 |
|------|------|------|
| Execute **每轮** ReAct 强制 Obs* 标签 | **❌ 拒绝** | 污染 wi 私有链；LLM 自报 ObsFact 无 evidence；与 LC2/G3 冲突 |
| Execute **terminal** 结构化 LastRound / ScopeContract | **✅ 采用** | Signal 进 partition/cohort，不进 Obs taxonomy 原文 |
| **Observe 规则** Signal → Obs* 映射 | **✅ 采用** | 延续 `item_observe.go`；Strength/Evidence 可单测 |
| Goal ScopeContract → ObsUncertainty 门控 | **✅ 采用** | `open_questions` 非空 → 规则 ObsUncertainty → 阻断 SpawnDecompose |
| Execute context **软引导**（`<conclusion>` / `<open_questions>`） | **✅ 可选** | 非 SoT；Observe/Verify 规则升格 |
| Observe 节点 **LLM 提案 + 规则裁决**（PR-A4 方向） | **⏸ Phase 2 登记** | 符合 G3；不在本 change Phase 1 编码 |

**三层边界：**

```text
Execute context  →  结构化 Signal（ScopeContract, LastRound, Bubble）
Observe 规则     →  Observation（Obs* + Strength + Evidence）→ UncertaintyReport
Plan 规则        →  PlanKind（Commitment / Protocol / Scenario / Exploration）
```

## 4. Capabilities（L4 映射）

| L4 ID | 名称 | Phase |
|-------|------|-------|
| D7-S16-A60 | `ScopeContract` Goal 范围收敛 | 1 |
| D7-S16-A61 | `ChildDownlink` 父→子下行契约 | 1 |
| D7-S16-A62 | `LayerCohort` partition 注册 | 1 |
| D2-S16-A20 | `ContextMaterializer.Materialize` | 1 |
| D2-S16-A21 | Partition store（wi private + cohort meta） | 1 |
| D7-S16-A70 | `WorkItemExecutor` → Materialize 接线 | 1 |
| D7-S16-A71 | `ResolvePartitionForWorkItem` | 1 |
| D7-S16-A72 | Signal→Observation 规则映射（Observe 边界） | 1 |
| D7-S16-A73 | Execute 结构化交付模板（非 Obs  taxonomy） | 1 |
| D7-S16-A63 | UpstreamSignal BlockedBy 物化 | 2 |
| D7-S16-A64 | PeerStatusSignal（ParallelExplore） | 2 |
| D5-S23-A06 | Span `D2_Context_Materialize` | 1 |
| D7-S16-A65 | SubTurn → MaterializePolicy 映射 | 3 |

## 5. Scope

### In Scope

- Partition 契约 + Materialize API（D2）  
- WorkItemExecutor / ItemPipeline Execute 接线（D7）  
- ScopeContract + ChildDownlink 类型与 Plan 模板（D7）  
- **Signal→Observation 边界**（Observe 规则映射；Execute 禁止每轮 Obs* 标签）  
- CG2 语义修订（design + spec delta）  
- 集成测试 + Jaeger 验收  
- Feature flag `FeatureLayerSubContextEnabled`

### Out of Scope

- Execute **每轮** ReAct 强制 ObsFact/ObsSignal/ObsDeviation/ObsUncertainty 自报  
- Observe 节点 LLM Proposer 全量实装（PR-A4，Phase 2 登记）  
- RunParallelExplore Wave 实装（rollup Phase 2）  
- LLM DecomposeProposer 全量（rollup Phase 2）  
- 跨 Session UI（TD-WT-04）  
- delegate SubTurn 统一（Phase 3）  
- D2 主 transcript 存储格式重写

## 6. Success Criteria

- [ ] Flag=on 时，子 WI Jaeger 可见 `D2_Context_Materialize` 且 message_count ≪ session 主 Turn  
- [ ] 同层无 BlockedBy 的 A/B：B 的 materialize payload 不含 A 的 tool result 全文  
- [ ] BlockedBy B→A：B 含 A 的 structured bubble，不含 A 私有链  
- [ ] Goal 开放指令：首轮含 ScopeContract JSON；有 open_questions 时不 spawn  
- [ ] Execute wi 私有链不含 Obs* 强制 taxonomy 块；Obs* 仅 Observe UncertaintyReport  
- [ ] Flag=off：行为与当前 master 一致（回归）

## 7. Risks & Mitigations（摘要，详见 design.md §10）

| Risk | Impact | Mitigation |
|------|--------|------------|
| 与 CG2 语义冲突 | 高 | 修订 CG2：transcript 隔离 vs cohort 域共享 |
| Materialize 性能 | 中 | Execute 内缓存 + 超 budget 再压缩 |
| 迁移/双轨 | 中 | Feature flag；partition 增量上线 |
| sandbox/context 不对齐 | 中 | sandbox_slug → 强制 private |
| 与 rollup 重复读全文 | 低 | Rollup 仅 RollupSynth directive |
| Execute 每轮 Obs 标签污染 transcript | 中 | 规范禁止；Execute 模板仅结构化 Signal |
| LLM 自报 ObsFact 虚高 Strength | 中 | Obs* 仅 Observe 规则产出；PP-1 StrengthMatch 不变 |

## 8. 分 Phase 交付

| Phase | 交付 | 依赖 |
|-------|------|------|
| **1** | ScopeContract + Materialize + WI Execute 接线 + Signal→Obs 边界 + span | rollup Phase 1 |
| **2** | BlockedBy upstream + PeerStatus + cohort CLI show | Phase 1 |
| **3** | SubTurn policy 统一 + Wave ContextResolver 合并 | Phase 2 |

## 9. Review 讨论点（给 Claude）

1. `ScopeContract` 是否应成为 `WorkItem` 持久字段 vs 仅 `LastRound` 扩展？  
2. `ChildSpec` 扩展 `ExpectedReturn` 是否与 rollup Phase 2 DecomposeProposer 合并 PR？  
3. Materialize 是否复用完整 PrepareOrchestrator A01–A04，还是轻量子路径？  
4. CG2 修订是否需 major version bump on ContextGraph design doc？  
5. Execute 软引导块（`<conclusion>` / `<open_questions>`）是否写入 Materialize 默认模板？  
6. Phase 2 是否在 Observe 节点增加 LLM ObservationProposer（提案 + 规则校验）？
