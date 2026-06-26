# D7 WorkModel — WorkItem ContextGraph Spec

**Module:** D7 Orchestration / S1 WorkModel (State Authority)
**Change:** `devrix-d7-workitem-context-graph` (DM-20260626-020)
**Status:** S5_Accepted → S7_Archived
**Spec Version:** v1.0
**SoT Design:** `openspec/specs/d7-orchestration/workitem-context-graph-design.md` v0.3.0
**PR:** #244

---

## ADDED

### Requirement: ContextScope 1:1 WorkItem Binding

每个非 Ephemeral WorkItem 在 `D7_WORKITEM_CONTEXT_GRAPH=1` 下 MUST 绑定 `ContextScope`，sidechain 分区键为 `wi_<work_item_id>`。

#### Scenario: CreateWorkItem assigns ContextScope when flag on

- GIVEN `D7_WORKITEM_CONTEXT_GRAPH=1`
- WHEN `TaskManager.CreateWorkItem` 创建非 Ephemeral WorkItem
- THEN `WorkItem.ContextScopeID` 非空
- AND sidechain key 等于 `wi_<id>`

---

### Requirement: ContextLinkEvaluator CL0–CL8

水平透传 MUST 经 `EvaluateContextLinkSpec` 规则裁决；R2 `BlockedBy` 依赖链 MUST 强制 `LinkUpstream`（CL0）。

#### Scenario: BlockedBy auto upstream link

- GIVEN `wi_b.BlockedBy` 含 `wi_a`
- WHEN `ApplyAcceptedContextLinks` 运行
- THEN 产生 `ContextLinkRecord`，`proposed_by=rule:R2_dependency`
- AND `wi_b.ContextPolicy=upstream`

#### Scenario: share_summary only completed to pending

- GIVEN 同层兄弟 R1，`Parent.LastRound.SpawnPolicy=decompose`
- WHEN LLM/default proposer 提案 `share_summary`
- AND from 非 completed 或 to 已 completed
- THEN CL1 拒绝链接

---

### Requirement: ContextBubbleEvaluator CB0–CB6

垂直 bubble MUST 至少 `BubbleStructured`（LP-5）；父 Observe MUST 注入 terminal 子项 structured bubble。

#### Scenario: Parent Observe reads child LP-5

- GIVEN 子 WorkItem terminal 且 `LastRound.ContextBubbleKind=structured`
- WHEN 父 WorkItem 运行 Observe
- THEN Observation 含 `structured_child_bubble` 来源 fact

---

### Requirement: Wave BlockedBy → ContextUpstream Projection

`WaveNodesFromSubtree` 在 flag on 时 MUST 将 BlockedBy 映射为 `ContextUpstream` + `UpstreamTaskID`。

#### Scenario: Dependent implement node upstream policy

- GIVEN `D7_WORKITEM_CONTEXT_GRAPH=1`
- AND implement 节点 `BlockedBy` 非空
- WHEN `WaveNodesFromSubtree` 投影
- THEN `TaskNode.ContextPolicy=upstream`
- AND `TaskNode.UpstreamTaskID` 为首个 blocker

---

### Requirement: Pipeline Decide Order

Decide 阶段顺序 MUST 为：Bubble → Links → Spawn（§8.3）。

#### Scenario: ApplyPipelineDecide ordering

- WHEN `ItemPipelineRunner.Run` 完成 Learn
- THEN `ApplyPipelineDecide` 先于 `ApplyPipelineRound` 写入 spawn
- AND `ContextBubbleKind` 写入 `WorkItemPipelineRound`

---

### Requirement: Context CLI and ResolveHint

运维 MUST 可通过 `/task context show` 与 `ContextResolveHint` 审计 scope/links。

#### Scenario: context show displays sidechain

- GIVEN flag on 且 WorkItem 有 scope
- WHEN `/task context show <id>`
- THEN 输出含 `wi_<id>` sidechain key

---

## MODIFIED

- `workitem-pipeline-unification-design.md` — 交叉引用 ContextGraph
- `d7-domain.md` — 索引 ContextGraph 设计文档

## UNCHANGED

- `SpawnPolicyEvaluator` R0–R8
- WorkTree `BlockedBy` 语义（ContextGraph 只读映射）
