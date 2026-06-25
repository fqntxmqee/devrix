# Proposal — devrix-d7-six-s-simplification (DM-20260626-001)

**Change ID:** `devrix-d7-six-s-simplification`
**Demand ID:** DM-20260626-001
**Priority:** P0
**Sprint:** d7-v6
**Estimated Effort:** 5 天
**PR Count:** 1
**Status:** S2_Proposal → S3_Design → S4_Implemented → S5_Accepted → S7_Archived (2026-06-26)
**SoT:** 用户反馈 "D7 理想的架构应该不需要 14 S 场景，定位不要变"

---

## 1. 目标

把 D7 编排层从 14 S 博弈角色场景精简到 6 S + 1 横切，对齐 6 大博弈角色，并补齐 5 个核心链路缺失的 P0/P1 Span op。

**用户硬约束（定位不变）：**
- D7 = Orchestration Mediator / Turn Leader / 5-node Pipeline Owner
- 不动 D1/D2/D3/D4/D5/D6 边界
- 不动 IntentKind 四链、14 ExitReason、Verdict 4 态、sessionSpan 9 attr
- 不删除 doc 35/36/37/41（项目知识沉淀保持原貌）

## 2. 14 S 的 4 类冗余

| 冗余类型 | 案例 | 解决方案 |
|---------|------|----------|
| 角色重合 | S4 + S9 都叫 "Costly Signaler"（ExecutionFlow 聚合 vs Execute 通道） | 合并到 S4 |
| 代码同址 | S7 = S2 自身，S13 = S2 内部文件，S12 散落 S2 内 | 全部并入 S2 |
| 跨切不该独立成 S | S6 Hardening 是观测基础设施 | 改为 cross-cutting（`orchestration/hardening/`） |
| 粒度过细 | S5 决策 + S8 Observe Quantize 都属"信息生产+量化" | 合并到 S5 |

## 3. 6 S + 1 横切博弈角色映射

| 6 S | 角色 | 范围 | A 数 |
|-----|------|------|------|
| **S1 WorkModel** | State Authority | WorkItem + ReputationEvidence + UncertaintyCoord | 4 |
| **S2 SessionOrchestrator** | Mediator + Turn Leader + Error Recovery | Turn 主循环 + ResumeSession + EscapeEngine 调度 + AutoClose | 7 |
| **S3 WaveScheduler** | Mechanism Designer | 调度策略 + 资源管理 | 4 |
| **S4 ExecutionFlow+Verify** | Costly Signaler + Certifier | FlowEvent 聚合 + Verifier 验证 + 14 ExitReason | 9 |
| **S5 DecisionPlanning+Observe** | Info Producer + Quantizer | ClassifyIntent + UncertaintyReport + IntentQuantize | 8 |
| **S6 MUPS Pipeline** | Pipeline Coord + Memory Curator | Execute 4 Channel + Learn 3 通道 + ChannelRouter + Memory | 15 |
| **Cross-cutting: Hardening** | Discipline Keeper | `orchestration/hardening/`（non-S） | 2 |

**A 编号变化：** 56 → 49（保留 49 个 A 编号，去掉 7 个冗余 A）
**F 编号变化：** 75 → 68（按新 S 重归类；Legacy 41 + Canonical 27）
**T 编号变化：** 180 持平（重归类不删，已有测试点 0 损失）
**Span op 变化：** 18 → 23（**+5 个 P0/P1**）

## 4. 5 个新 P0/P1 Span

| Span op | S | 文件 | 关键属性 | 优先级 |
|---------|---|------|----------|--------|
| `D7_Channel_Route` | S6-A48 | `execute/channel.go::ChannelRouter.Route` | channel.kind / plan.kind / score / fallback | **P0** |
| `D7_Memory_Persist` | S6-A49 | `learn/memory.go::SkillMemory.Store` | channel / asset.kind / ttl_ms / payload_size | **P0** |
| `D7_System_Anomaly_Detect` | S4-A47 | NEW `executionflow/verify/anomaly.go::DetectSystemAnomaly` | anomaly.kind / severity / threshold / evidence_id | **P0** |
| `D7_TaskGraph_Synthesize` | S5-A33 | `decisionplanning/decomposer.go::SynthesizeTaskGraph` | node_count / edge_count / dag_depth / cycle_detected | **P1** |
| `D7_Executor_Select` | S5-A34 | `wavescheduler/scheduler.go::dispatchOne` | candidates_count / selected_kind / score / policy | **P1** |

**未推进的 4 个 Span（P2/P3）：**
- `bridge.agent_spoke`（D4 跨域）
- `bridge.subquery_spoke`（D2 SubQuery）
- `subtask.decompose`
- `subtask.merge`

## 5. 范围

### 5.1 包含

**Spec 层（9 个文档）：**
- `d7-domain.md`（v1.2.0 → v2.0.0）
- `a-registry.md`（v4.0.0 → v5.0.0）
- `f-registry.md`（v4.0.0 → v5.0.0）
- `span-registry.md`（v3.0.0 → v4.0.0）
- `t-registry.md`（v3.18.0 → v4.0.0）
- `terminal-state-guide.md`（v1.2.0 → v2.0.0）
- `observability-guide.md`（v1.2.0 → v2.0.0）
- `layer-delta.md`（v5.0.0 → v6.0.0）
- `design.md`（v3.3.0 → v4.0.0）

**Code 层（5 个 Span emit + 2 个 NEW 包）：**
- `orchestration/d7spans/`（NEW package）：5 emit helpers + SetBridge setter
- `orchestration/executionflow/verify/`（NEW directory）：S4 Certifier role
- 5 个 Span emit 点：见 §4
- 1 个 wrapper 函数：`DetectSystemAnomaly`（保持 LP-1 兼容）
- 1 个 helper：`dagDepth`（最长路径 BFS）

**测试层（20 个新 T 点）：**
- 10 d7spans emit/fail-safe（T01-T10）
- 6 executionflow/verify anomaly（T11-T16）
- 4 decisionplanning dagDepth + Span emit fail-safe（T17-T20）

### 5.2 不包含

- 14 S 文档中 v1.x 历史保留（a-registry.md §V1..V5 历史段不删）
- 已有 PR 编号引用（V4.3 + V5 + V5.6 + V5.6 review fixes）保持原貌
- 5 个未推进的 Span（bridge.agent_spoke / bridge.subquery_spoke / subtask.decompose / subtask.merge）留作 P2/P3
- Phase 4 PR-D4 的 UncertaintyCoord Value=0.95 路径（保持 LP-1 兼容，0 改动）
- orchtypes.EvaluateSystemAnomaly 已有实现（新增 executionflow/verify/anomaly.go 仅是 wrapper，0 改原函数）
- 14 → 6 S 的"代码包路径迁移"（Step 2）：本 PR 仅做 spec 文档精简 + 5 Span 落地；包路径合并（execute/ + learn/ → mups/）留作后续 PR

## 6. 工作量与拆分

5 天工作量，1 个 PR：

| 步骤 | 工作量 | 输出 |
|------|--------|------|
| Step 1: 精简 9 个 spec 文档 | 0.5 天 | +1621/-193 行 |
| Step 2: 代码层包路径迁移 | 1 天 | **deferred to follow-up PR** |
| Step 3a: 3 个 P0 Span | 1.5 天 | channel.route + memory.persist + system.anomaly_detect |
| Step 3b: 2 个 P1 Span | 1 天 | taskgraph.synthesize + executor.select |
| Step 4: 验证 + 归档 | 1 天 | 22/22 包 PASS + verify-archive.sh 11/11 PASS |
