---
demand-id: DM-20260704-006
title: "D7 MUPS 不确定性渐进消解可追溯性 — Obs→Execution 统一契约 (ResolutionContract + DecideBinding)"
priority: P0
status: S1_Proposal
dsaft_domain: orchestration
created: 2026-07-04
supersedes_drafts:
  - "Option B (RC-1..RC-4 obs→resolution only) — superseded by Option C unified design"
related:
  - openspec/specs/d7-orchestration/spec.md
  - internal/layers/orchestration/mups/observe/
  - internal/layers/orchestration/mups/plan/
  - internal/layers/orchestration/mups/execute/
  - internal/layers/orchestration/mups/verify/
  - internal/layers/orchestration/workmodel/spawn_policy.go
  - internal/layers/orchestration/workmodel/decompose.go
  - internal/layers/orchestration/orchtypes/uncertainty_report.go
parent_demands:
  - DM-20260704-001  # d7-uncertainty-spawn-decouple — U 驱动拓扑已立
  - DM-20260705-003  # mups-semantics-schema-alignment — locale-neutral 语义层
  - DM-20260705-004  # mups-node-prompt-dedup — 三节点 prompt 净化
trigger_session:
  - "sess_1783239758810_0 (trace c6f2d6910496e2ea63cbcf8f207b2c0a — review d7 plan 目录 → Plan 返回 decompose+2 child_specs → Decide 走 SpawnInline → child_specs 失效)"
---

# D7 MUPS 不确定性渐进消解可追溯性 — Obs→Execution 统一契约

## 1. 原始描述

> MUPS 设计原则是 **不确定性驱动渐进消解**：Observe 发现 obs_uncertainty → Plan 制定消解策略 → Execute 通过 tools 主动消解 → Verify 验证消解覆盖率 → Decide 据此决定 SpawnUserGate / SpawnDecompose / SpawnNone。
>
> 当前实现存在**两层断链**：
>
> **断链 A（Observation → Resolution）**：obs_uncertainty 的"消解"完全靠 LLM 自觉，Execute 把答案混在 artifact prose 里，Verify 用文本正则扫"awaiting your"等关键词判断——既不可靠也不可观测。
>
> **断链 B（Plan → Decide）**：Plan LLM 返回 `execution_mode: "decompose"` + `child_specs[]` 是**叙事意图**，但 `SpawnPolicyEvaluator` 不读 `round.ChildSpecs`，只读 VerdictKind/U/DeliverableStatus。结果 Plan 提议拆 2 个子任务，Decide 却选 SpawnInline retry，整个 `child_specs[]` 被丢弃，子 WorkItem 不创建。trace `c6f2d6910496e2ea63cbcf8f207b2c0a` 即此场景。
>
> 期望：把"渐进式消解"从隐式 LLM 行为升级为 **Obs→Execution 统一契约**——Plan 的 `child_specs[]` 重新定义为 `ResolutionStrategy[].sub_worktree`，每条绑定 `obs_id`；Execute 产出 `ResolutionClaim[]` 对照 obs_id；Verify 计算 `ResolutionCoverage`；Decide 据 `UnresolvedObs` + `sub_worktree` 决定 SpawnDecompose（per-obs 子 WI）或 SpawnUserGate（ask_user_question）。

## 2. 问题陈述

| 断链 | 现状 | 风险 |
|------|------|------|
| **A: Obs→Resolution** | Plan 不声明消解策略；Execute 把答案混在 prose；Verify 用文本正则 | LLM 不写关键词就漏判；无法量化"Plan 承诺消解 3 个 obs，Execute 实际消解了多少" |
| **B: Plan→Decide** | Plan LLM `execution_mode: "decompose"` + `child_specs[]` 是 hint；Decide 不读 ChildSpecs | Plan 提议拆 2 个子任务，Decide 选 SpawnInline retry，子 WI 不创建；Plan 的战略意图完全丢失 |
| **C: SubWorktree→Directive** | `mapRawChildSpecs` 把 `directive = parent_directive + suffix` 但仅在 SpawnDecompose 时使用 | 即使 Decide 走 SpawnDecompose，子 WI 的 directive 与同层 Execute 的 directive 完全分离，无法复用 |

具体场景（trace `c6f2d6910496e2ea63cbcf8f207b2c0a`）：review d7 plan 目录后 Observe 返回 3 个 obs_uncertainty。Plan LLM 转 Explore 返回 `execution_mode: "decompose"` + 2 个 child_specs。Execute 工具失败，`files_inspected=0`，但**仍**走完 5 个 ReAct iter 输出 findings。Verify 通过 deliverable_contract（incomplete 因 min_runes 不满足）→ Decide 选 **SpawnInline**（retry 同一 parent）→ `round.ChildSpecs` 被忽略 → 2 个子 WI **永远不会被创建**，3 个 obs_uncertainty 永远不会被工具消解。

## 3. 目标（P0）

1. **统一类型 `ResolutionStrategy`**：每条绑定 `obs_id` + `planned_tool` + `success_criterion` + **可选 `sub_worktree`**（替代原 `child_specs[]`）。`sub_worktree` 含 `{title, directive_suffix, expected_return}`，仅当 Plan 决定针对该 obs 创建独立子 WI 时填。
2. **Plan artifact schema 扩展**：原 `child_specs[]` 字段**重命名/合并**为 `ResolutionStrategy[].sub_worktree`（向后兼容：旧代码读 `child_specs` 时仍可用，但新代码统一走 ResolutionStrategy 路径）。
3. **Execute artifact 携带 `ResolutionClaim[]`**：每条对应 `obs_id`，含 `{answer, confidence, supporting_evidence}`。
4. **Verify 增加 `verifyResolutionCoverage()`**：对照 Plan strategies vs Execute claims 计算 CoverageRatio；输出 `UnresolvedObs[]`（未覆盖 + 低 confidence + obs_id_mismatch）。
5. **Decide 增加 `SpawnDecomposeForUnresolved` 分支**：当 `UnresolvedObs` 非空且每个都有 `sub_worktree` 时，**强制** SpawnDecompose，每个 `sub_worktree` 创建一个 child WorkItem。这是**断链 B 的治本**。
6. **Decide 增加 `SpawnUserGate` 分支**：当 `UnresolvedObs` 非空且 `MaxUnresolvedStrength ≥ 0.85` 且**没有** `sub_worktree` 时，触发 ask_user_question。这是**断链 A 的治本**。
7. **安全网保留**：现有 `detectUserGate` 文本正则与 `execution_mode: "decompose"` 直接强制 SpawnDecompose 的旧路径**保留**作 fallback（Plan/Execute LLM 不按新契约填字段时退化）。

## 4. 数据契约

```go
// orchtypes/resolution.go (新增)

// ResolutionStrategy 绑定一个 obs_uncertainty 的消解策略。
// 替代原 child_specs[]：每条必须对应 Observation.ObsID，
// 子任务（sub_worktree）可选——仅当 Plan 决定对单 obs 独立 WI 时填。
type ResolutionStrategy struct {
    ObsID            string          // 与 Observation.ObsID 对齐
    PlannedTool      string          // tool registry 中的 tool name
    SuccessCriterion string          // 自由文本："读到 plan 目录的 4 个文件"
    SubWorktree      *SubWorktreeSpec // 可选；nil = 同层 Execute 处理
}

// SubWorktreeSpec 描述为单个 obs_uncertainty 创建的子 WorkItem。
// 当 Plan 决定某 obs 需要独立子 WI 时填，由 Decide→SpawnDecompose 创建。
type SubWorktreeSpec struct {
    Title           string   // 子 WI 标题
    DirectiveSuffix string   // 拼到 parent directive 后
    ExpectedReturn  string   // 子 WI 期望产出
    ScopeIn         []string // 子 WI scope
    PlannedTool     string   // 子 WI 主要 tool（用于 tool_filter hint）
}

type ResolutionClaim struct {
    ObsID              string  // 与 Observation.ObsID 对齐
    Answer             string  // 自由文本或结构化 JSON
    Confidence         float64 // [0, 1]
    SupportingEvidence string  // 引用 tool output 或文件:行号
}

type ResolutionReport struct {
    TotalStrategies int             `json:"total_strategies"`
    TotalClaims     int             `json:"total_claims"`
    CoverageRatio   float64         `json:"coverage_ratio"`
    UnresolvedObs   []UnresolvedObs `json:"unresolved_obs"`
}

type UnresolvedObs struct {
    ObsID    string  `json:"obs_id"`
    Strength float64 `json:"strength"` // 来自 Observation.Strength
    Reason   string  `json:"reason"`   // "no_resolution_claim" | "low_confidence" | "obs_id_mismatch" | "sub_worktree_present"
    HasSubWorktree bool `json:"has_sub_worktree"` // true → Decide 触发 SpawnDecompose
}
```

## 5. Decide 决策流（核心 — 治本断链 B）

```
ResolveReport.UnresolvedObs
   ↓
for each obs in UnresolvedObs:
   if obs.HasSubWorktree:
       → SpawnDecompose: 创建 sub_worktree 数量的子 WI (Directive = parent + obs.SubWorktree.DirectiveSuffix)
   else if MaxUnresolvedStrength ≥ UnresolvedStrengthThreshold (0.85):
       → SpawnUserGate: tool_filter whitelist = [ask_user_question]
   else:
       → SpawnInline retry (现有逻辑)
```

**关键**：当 Plan 填了 `sub_worktree` 时，Decide 看到 `HasSubWorktree=true` **强制**走 SpawnDecompose，**不再受** deliverable incomplete 压制——这是断链 B 的治本。

## 6. L5 测试点（草案）

### 6.1 断链 A 治本（Obs→Resolution）

| ID | Given-When-Then | Priority |
|----|-----------------|----------|
| L5-D7-RT-01 | Given Plan 输出含 3 obs_uncertainty, When Plan LLM 完成, Then `ResolutionStrategy[]` 长度 = 3 且每条 `obs_id` 唯一 | P0 |
| L5-D7-RT-02 | Given Execute 调 read_file 后产生 artifact, When LLM 提交, Then artifact 含 ≥ 1 条 `ResolutionClaim{obs_id, answer, confidence}` | P0 |
| L5-D7-RT-03 | Given Plan 3 strategies、Execute 命中 2/3 claim, When Verify runs, Then VerdictKind=Partial 且 `UnresolvedObs` 长度 = 1 且 `obs_id` 与未命中对齐 | P0 |
| L5-D7-RT-05 | Given 所有 obs 已 claim 且 confidence ≥ 0.7, When Verify runs, Then VerdictKind=Pass 且 `UnresolvedObs = []` | P0 |
| L5-D7-RT-06 | Given ResolveClaim 全缺失, When Verify runs, Then VerdictKind=Fail 或 Partial，且 `VerifyRationale` 含 "no resolution claims" | P1 |
| L5-D7-RT-07 | Given Plan 未给 ResolutionStrategy（fallback 路径），When Verify runs, Then 退化到现有文本正则 detectUserGate，且日志输出 `resolution_strategy_missing` warning | P1 |

### 6.2 断链 B 治本（Plan→Decide 绑定）

| ID | Given-When-Then | Priority |
|----|-----------------|----------|
| L5-D7-RT-08 | Given Plan `ResolutionStrategy[].sub_worktree` 非空（每条都有 SubWorktreeSpec）, When Decide runs (即使 deliverable incomplete), Then SpawnPolicy=SpawnDecompose 且 `len(round.ChildSpecs) >= len(UnresolvedObs)` | P0 |
| L5-D7-RT-09 | Given Plan `ResolutionStrategy[]` 中 2/3 有 sub_worktree, When Decide runs, Then SpawnPolicy=SpawnDecompose 且 `len(round.ChildSpecs) = 2`（仅 unresolved 且有 sub_worktree 的） | P0 |
| L5-D7-RT-10 | Given Plan 3 strategies 全无 sub_worktree + MaxUnresolvedStrength ≥ 0.85, When Decide runs, Then SpawnPolicy=SpawnUserGate 且 tool_filter whitelist 仅含 `ask_user_question` | P0 |
| L5-D7-RT-11 | Given Plan 3 strategies 全无 sub_worktree + MaxUnresolvedStrength < 0.85, When Decide runs, Then SpawnPolicy=SpawnInline（沿用现有逻辑） | P1 |
| L5-D7-RT-12 | Given Decide 触发 SpawnDecompose via sub_worktree, When 子 WI 被创建, Then 每个 child.Directive = parent_directive + sub_worktree.DirectiveSuffix 且 child.ScopeIn = sub_worktree.ScopeIn | P0 |
| L5-D7-RT-13 | Given Decide 触发 SpawnDecompose via sub_worktree, When 子 WI 完成 rollup, Then parent 的 CoverageRatio 更新且 Learn 节点把每个 sub_worktree 的 ResolutionClaim 合并到 parent 的 asset | P1 |

### 6.3 回归与安全网

| ID | Given-When-Then | Priority |
|----|-----------------|----------|
| L5-D7-RT-14 | Given Plan LLM 输出旧格式 `execution_mode: "decompose"` + `child_specs[]`（无 obs_id 绑定），When Decide runs, Then 退化到现有逻辑（execution_mode 作为 hint） | P0 |
| L5-D7-RT-15 | Given Plan LLM 完全无 ResolutionStrategy 字段（旧 LLM 输出），When Decide runs, Then 退化到 `execution_mode` hint + detectUserGate 文本正则 | P1 |
| L5-D7-RT-16 | Given Decide 触发 SpawnDecompose via sub_worktree, When budget 检查失败 (depth/children/daily 超限), Then 退化到 SpawnInline 并 log `sub_worktree_budget_exceeded` warning | P0 |

## 7. 不变量

- MUPS **6 节点拓扑**（Observe→Plan→Execute→Verify→Learn→Decide）**不变**。
- Execute ReAct loop（tool call → observation → next action）**不变**。
- Tool registry **不变**；`ask_user_question` 在 SpawnUserGate 模式下强制 whitelist。
- 现有 `detectUserGate` 文本正则作为 **safety net** 保留：Plan 缺 strategy / LLM 不按新契约填字段时退化。
- 现有 `execution_mode: "decompose"` hint 路径**保留**（RT-14/15 fallback）：当 LLM 输出旧格式时仍可用。
- TaskContract（TaskSpec/TaskReport, PR #325/#327）**不冲突**：ResolutionStrategy/Claim 是 artifact schema 扩展，不是新的接口契约。
- `child_specs[]` 字段**保留**作为兼容字段，但新代码统一走 `ResolutionStrategy[].sub_worktree`。

## 8. 域归属与跨域影响

| 域 | 影响 | 说明 |
|----|------|------|
| **D7 mups/observe/** | 不变 | Observation 已含 obs_id + strength |
| **D7 mups/plan/** | **修改** | artifact schema 增加 `ResolutionStrategy[]`；`child_specs[]` 字段标记 deprecated |
| **D7 mups/execute/** | **修改** | artifact schema 增加 `ResolutionClaim[]`；tool call 完成后引导 LLM 声明 claim |
| **D7 mups/verify/** | **修改** | 新增 `verifyResolutionCoverage()` + 决策表 |
| **D7 workmodel/spawn_policy.go** | **修改** | 新增 `SpawnDecomposeForUnresolved` + `SpawnUserGate` 两个分支；旧 R0-R8 逻辑保留 |
| **D7 workmodel/decompose.go** | **修改** | `DecomposeChildren` 接受 `SubWorktreeSpec[]` 而非 `ChildSpec[]`；**或**新增 `DecomposeFromSubWorktree` 入口 |
| **D7 workmodel/pipeline_round.go** | **修改** | `WorkItemPipelineRound` 增加 `ResolutionReport` 字段 |
| **D2 contextengine/** | 不变 | MaterializeForMUPS 仍按 phase 暴露 tool |
| **D1 communication/** | 不变 | ask_user_question 既有 tool |

## 9. 非目标

- 替换或废除现有 `detectUserGate` 文本正则（safety net 保留）。
- 删除 `child_specs[]` 字段（兼容保留）。
- 替换 `execution_mode: "decompose"` hint 路径（fallback 保留）。
- 新增 tool 类型。
- 修改 MUPS 6 节点顺序或 ReAct loop。
- 改动 TaskContract（TaskSpec/TaskReport）FF 状态。

## 10. 验收口径

### L5-D7-RT-08 — sub_worktree 强制 SpawnDecompose（P0 — 断链 B 治本）
- **GIVEN** Plan `ResolutionStrategy[]` 含 3 条，其中 2 条 `SubWorktree != nil`（针对 obs_u1, obs_u2）
- **AND** Execute `ResolutionClaim` 命中 1 条（仅 obs_u3，confidence=0.85）
- **WHEN** Verify → Decide runs
- **THEN** Verify 输出 `UnresolvedObs = [obs_u1(strength=0.92, HasSubWorktree=true), obs_u2(strength=0.88, HasSubWorktree=true)]`
- **AND** Decide 选 SpawnPolicy = SpawnDecompose
- **AND** `round.ChildSpecs` 长度 = 2（仅 unresolved 且有 sub_worktree 的）
- **AND** 即使 `deliverableStatus=incomplete`，**仍然** SpawnDecompose（不因 deliverable 退化为 SpawnInline）

### L5-D7-RT-09 — 部分 sub_worktree 触发部分 decompose（P0）
- **GIVEN** Plan `ResolutionStrategy[]` 含 3 条，其中仅 1 条 `SubWorktree != nil`（针对 obs_u2）
- **WHEN** Verify → Decide runs
- **THEN** `UnresolvedObs = [obs_u1(no_sub_worktree), obs_u2(has_sub_worktree), obs_u3(no_sub_worktree)]`
- **AND** Decide 选 SpawnPolicy = SpawnDecompose（至少 1 个 sub_worktree）
- **AND** `round.ChildSpecs` 长度 = 1（仅 obs_u2 的 sub_worktree）

### L5-D7-RT-10 — 无 sub_worktree + 高强度 → SpawnUserGate（P0）
- **GIVEN** Plan `ResolutionStrategy[]` 含 3 条，全 `SubWorktree == nil`
- **AND** `MaxUnresolvedStrength = 0.92 ≥ 0.85`
- **WHEN** Decide runs
- **THEN** SpawnPolicy = SpawnUserGate
- **AND** 下一轮 WI.Prompt.Input.ToolFilter = `[ask_user_question]`
- **AND** Prompt.Input.UnresolvedObs = UnresolvedObs list

### L5-D7-RT-12 — sub_worktree 创建子 WI（P0）
- **GIVEN** Decide 选 SpawnDecompose via sub_worktree
- **WHEN** `tm.DecomposeFromSubWorktree` runs
- **THEN** 每个 child.Directive = parent_directive + "\n\n" + sub_worktree.DirectiveSuffix
- **AND** child.ScopeIn = sub_worktree.ScopeIn
- **AND** child.Title = sub_worktree.Title
- **AND** child.ExpectedReturn = sub_worktree.ExpectedReturn

### L5-D7-RT-14 — 旧格式 execution_mode + child_specs fallback（P0）
- **GIVEN** Plan LLM 输出 `execution_mode: "decompose"` + `child_specs[]`，**无** ResolutionStrategy
- **WHEN** Decide runs
- **THEN** 退化到现有 `execution_mode` hint 路径
- **AND** log `resolution_strategy_missing_fallback_to_execution_mode`
- **AND** SpawnPolicy 由 R0-R8 决定（不受本提案 RC-4 逻辑影响）

## 11. 关联变更

| 变更 | 关系 |
|------|------|
| DM-20260704-001 (d7-uncertainty-spawn-decouple) | **本 change 直接扩展**：本 change 加 RC-1..RC-4 + Decide binding；DM-001 立 U 驱动拓扑 |
| DM-20260705-003 (mups-semantics-schema-alignment) | 并行：locale-neutral 语义层可复用为 ResolutionClaim 的 render |
| DM-20260705-004 (mups-node-prompt-dedup) | 并行：Execute prompt 净化同步 |
| DM-20260629-007/008 (devrix-d7-taskcontract-unification PR-A/B) | 独立：TaskContract 与 ResolutionContract 平行 |
| sess_1783239758810_0 / trace c6f2d6910496e2ea63cbcf8f207b2c0a | 触发场景（断链 A + B 同时暴露） |

## 12. 风险评估

| 风险 | 概率 | 影响 | 缓解 |
|------|------|------|------|
| LLM 不按新契约填 ResolutionStrategy[].sub_worktree 字段 | High | Med | safety net：RT-14/15 退化路径保留；ParseRejectRecord 反馈注入下一轮 |
| sub_worktree 强制 SpawnDecompose 后单 session 子 WI 暴增 | Med | High | Budget gate (RT-16)：depth/children/daily 超限时退化 SpawnInline + log warning |
| threshold 0.85 误触发用户门控 | Med | High | Configurable `UnresolvedStrengthThreshold`；先观察飞书反馈 |
| ResolutionClaim schema 漂移 | Low | Med | Schema 单一来源（orchtypes/resolution.go）；golden test 覆盖 |
| child_specs[] 兼容字段双写 | Low | Low | 标注 deprecated；CI guard 警告旧字段使用 |
| 与 DM-20260705-004 (prompt dedup) 冲突 | Low | Low | 引导词走 execute.fieldMap guide 而非 prose 块 |

## 13. 追溯

```
DM-20260704-006 (demand.md)
  → proposal.md (Option C unified Obs→Execution)
  → design.md (S3)
  → specs/d7-orchestration_resolution_contract_delta.md (OpenSpec delta)
  → tasks.md
  → code:
      - orchtypes/resolution.go: 5 types (Strategy/SubWorktreeSpec/Claim/Report/UnresolvedObs)
      - mups/plan/plan.go: Plan schema + ResolutionStrategy[] + sub_worktree 引导
      - mups/execute/channel.go: Artifact schema + ResolutionClaim[] 引导
      - mups/verify/verify.go: verifyResolutionCoverage() + 4 状态决策表
      - workmodel/spawn_policy.go: SpawnDecomposeForUnresolved + SpawnUserGate 分支
      - workmodel/decompose.go: DecomposeFromSubWorktree 入口
      - mups-node-llm-protocols.md: §8 Resolution 引导词规范
  → acceptance-report.md (S5)
```

## 14. 检查清单（S1 完成）

- [x] DM ID 已分配且无冲突（DM-20260704-006）
- [x] demand.md 包含背景、问题、验收标准、范围
- [x] 至少 1 个 P0 验收标准（11 个 P0 + 5 个 P1）
- [x] Out of Scope 已明确声明
- [x] DSAFT 域标注正确（orchestration）
- [x] 与 parent demands / related demands 关联明确
- [x] 治本双断链（A: Obs→Resolution, B: Plan→Decide）均覆盖