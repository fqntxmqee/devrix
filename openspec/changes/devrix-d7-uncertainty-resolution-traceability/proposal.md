# Proposal: D7 MUPS Obs→Execution 统一契约（ResolutionContract + DecideBinding）

**Change ID:** `devrix-d7-uncertainty-resolution-traceability`
**Demand ID:** DM-20260704-006
**Created:** 2026-07-04
**Updated:** 2026-07-04 (supersede Option B → adopt Option C unified)
**Status:** S1_Proposal
**Demand:** [`demand.md`](demand.md)

---

## 1. Problem Statement

MUPS 设计原则是 **不确定性驱动渐进消解**，但当前实现存在**两层断链**：

### 断链 A: Observation → Resolution

```
Observe 发现 obs_uncertainty
   ↓
[断链 A] Plan 不声明消解策略 → Execute 把答案混在 prose → Verify 用文本正则扫
```

### 断链 B: Plan → Decide（trace `c6f2d6910496e2ea63cbcf8f207b2c0a` 直接暴露）

```
Plan LLM 返回 execution_mode: "decompose" + child_specs[] (2 children)
   ↓
[断链 B] Decide 选 SpawnInline (因 deliverable incomplete) → child_specs[] 整段失效
   ↓
子 WI 永远不被创建 → 3 个 obs_uncertainty 永远不会被工具消解
```

具体场景（trace `c6f2d6910496e2ea63cbcf8f207b2c0a`, sess_1783239758810_0, wi_d0_s0_goal）：
- Observe 返回 3 个 obs_uncertainty（strength 0.92/0.88/0.82）
- Plan LLM `rationale`: "uncertainty_mean=0.600 ≥ 0.45 必须 decompose"
- Plan LLM 输出 `execution_mode: "decompose"` + 2 个 child_specs，每个带 `directive_suffix`
- **Execute user content 实际收到的指令 = parent directive alone**（"review d7 领域 plan目录下代码"），**没有** directive_suffix
- Execute 工具失败，`stats.files_inspected = 0`，5 个 ReAct iter 跑完输出 findings
- Verify 通过 deliverable_contract（incomplete，因 min_runes=200 不满足 + tool_calls=0）
- **Decide 选 SpawnInline（retry 同一 parent）**，`round.ChildSpecs` 整段被忽略
- trace 结尾只有 1 round，没有 wi_d1_* 子 span

根因：`spawn_policy.go:91-97` 的 `SpawnDecompose` rationale 决策**完全不看** `round.ChildSpecs`——Plan LLM 的"必须 decompose"是叙事意图，被运行时 signal 覆盖。

## 2. Proposed Solution

引入 **Obs→Execution 统一契约**：

| 契约 | 节点 | 义务 |
|------|------|------|
| **RC-1** | Plan | 输出 `ResolutionStrategy[]`，每条绑定 `obs_id` + `planned_tool` + `success_criterion` + **可选 `sub_worktree`**（替代原 `child_specs[]`） |
| **RC-2** | Execute | artifact 携带 `ResolutionClaim[]`，每条对应 `obs_id` |
| **RC-3** | Verify | 新增 `verifyResolutionCoverage()`：对照 strategies vs claims 计算 CoverageRatio + UnresolvedObs[] |
| **RC-4a** | Decide | **强制 SpawnDecompose**：当 UnresolvedObs 中有 `HasSubWorktree=true` 的 obs |
| **RC-4b** | Decide | **强制 SpawnUserGate**：当 UnresolvedObs 全 `HasSubWorktree=false` 且 `MaxUnresolvedStrength ≥ 0.85` |
| **RC-4c** | Decide | **沿用现有 SpawnInline**：其它情况（UnresolvedObs 全 sub_worktree=false 且 strength < 0.85） |
| **RC-5** | safety net | 旧 `execution_mode: "decompose"` + `child_specs[]` 路径保留作 fallback；`detectUserGate` 文本正则保留 |

### 2.1 数据契约

```go
// orchtypes/resolution.go (新增)

// ResolutionStrategy 绑定一个 obs_uncertainty 的消解策略。
// 替代原 child_specs[]：每条必须对应 Observation.ObsID。
type ResolutionStrategy struct {
    ObsID            string          // 必填，与 Observation.ObsID 对齐
    PlannedTool      string          // tool registry 中的 tool name
    SuccessCriterion string          // 自由文本
    SubWorktree      *SubWorktreeSpec // 可选；nil = 同层 Execute 处理
}

// SubWorktreeSpec 描述为单个 obs_uncertainty 创建的子 WorkItem。
type SubWorktreeSpec struct {
    Title           string
    DirectiveSuffix string   // 拼到 parent directive 后
    ExpectedReturn  string
    ScopeIn         []string
    PlannedTool     string
}

type ResolutionClaim struct {
    ObsID              string
    Answer             string
    Confidence         float64
    SupportingEvidence string
}

type ResolutionReport struct {
    TotalStrategies int             `json:"total_strategies"`
    TotalClaims     int             `json:"total_claims"`
    CoverageRatio   float64         `json:"coverage_ratio"`
    UnresolvedObs   []UnresolvedObs `json:"unresolved_obs"`
}

type UnresolvedObs struct {
    ObsID          string  `json:"obs_id"`
    Strength       float64 `json:"strength"`
    Reason         string  `json:"reason"`
    HasSubWorktree bool    `json:"has_sub_worktree"`
}
```

### 2.2 Decide 决策流（治本断链 B 的关键）

```go
// workmodel/spawn_policy.go 新增逻辑（伪代码）

func DecideFromResolutionReport(round *WorkItemPipelineRound, report *ResolutionReport) SpawnPolicy {
    if report == nil || len(report.UnresolvedObs) == 0 {
        // 无未消解 obs：沿用现有 VerdictKind-based 决策
        return DecideFromVerdict(round)
    }

    // RC-4a：未消解 obs 含 sub_worktree → 强制 SpawnDecompose（治本断链 B）
    var subWorktrees []SubWorktreeSpec
    for _, uo := range report.UnresolvedObs {
        if uo.HasSubWorktree {
            // 从 round.PlanResolutionStrategies 找 SubWorktree
            subWorktrees = append(subWorktrees, ...)
        }
    }
    if len(subWorktrees) > 0 {
        if budgetOK := checkBudgetForSubWorktrees(subWorktrees); budgetOK {
            round.ChildSpecs = subWorktreesToChildSpecs(subWorktrees)
            return SpawnDecompose
        }
        slog.Warn("sub_worktree budget exceeded, falling back to inline", ...)
    }

    // RC-4b：全无 sub_worktree + 高强度 → SpawnUserGate
    maxStrength := maxStrengthOf(report.UnresolvedObs)
    if maxStrength >= UnresolvedStrengthThreshold {
        round.UnresolvedObs = report.UnresolvedObs
        return SpawnUserGate
    }

    // RC-4c：其余沿用现有 SpawnInline 逻辑
    return DecideFromVerdict(round)
}
```

**关键差异**：
- 旧逻辑：`SpawnDecompose` 仅在 `VerdictKind=Indeterminate` 或 `U>threshold` 时触发，且与 `child_specs[]` 无关
- 新逻辑：`SpawnDecompose` 在 `HasSubWorktree=true` 时**直接触发**，且 `round.ChildSpecs` 直接从 `SubWorktreeSpec[]` 生成

### 2.3 Verify 决策表（4 状态，与 ResolutionCoverage 对齐）

| CoverageRatio | AllConfidence ≥ 0.7 | VerdictKind | UnresolvedObs |
|---------------|---------------------|-------------|---------------|
| = 1.0 | true | VerdictPass | [] |
| ∈ [0.5, 1.0) | mixed | VerdictPartial | 未命中 + 低 confidence |
| < 0.5 | — | VerdictFail 或 Partial | 全列 |
| = 0 | — | VerdictFail 或 Partial | 全列，reason="no_resolution_claim" |

### 2.4 兼容性矩阵

| Plan LLM 输出格式 | Decide 路径 | 说明 |
|-------------------|-------------|------|
| 新格式：ResolutionStrategy[] + sub_worktree | **新路径（RC-4a/b/c）** | 主路径 |
| 旧格式：execution_mode + child_specs[] | 旧 R0-R8 路径 | **fallback**（L5-D7-RT-14） |
| 完全没有 Plan 输出 | 旧 R0-R8 路径 | **fallback**（L5-D7-RT-15） |

## 3. Capabilities

| ID | Capability | Owner |
|----|------------|-------|
| **D7-S16-A103** | `ResolutionStrategy` / `SubWorktreeSpec` / `ResolutionClaim` / `ResolutionReport` / `UnresolvedObs` 类型 | `internal/layers/orchestration/orchtypes/resolution.go` |
| **D7-S16-A104** | Plan artifact schema 扩展 ResolutionStrategy[] + sub_worktree 引导词 | `internal/layers/orchestration/mups/plan/plan.go` |
| **D7-S16-A105** | Execute artifact schema 扩展 ResolutionClaim[] + LLM 引导词 | `internal/layers/orchestration/mups/execute/channel.go` |
| **D7-S16-A106** | `verifyResolutionCoverage()` + 4 状态决策表 | `internal/layers/orchestration/mups/verify/verify.go` |
| **D7-S5-A108** | `SpawnDecomposeForUnresolved` 分支 + `SpawnUserGate` 分支 | `internal/layers/orchestration/workmodel/spawn_policy.go` |
| **D7-S15-A109** | `DecomposeFromSubWorktree` 入口（替代 `DecomposeChildren` 处理 sub_worktree） | `internal/layers/orchestration/workmodel/decompose.go` |
| **D7-S15-A110** | `WorkItemPipelineRound.ResolutionReport` 字段 | `internal/layers/orchestration/workmodel/pipeline_round.go` |

## 4. Scope

### In Scope
- 5 个新类型（ResolutionStrategy/SubWorktreeSpec/ResolutionClaim/ResolutionReport/UnresolvedObs）
- Plan artifact schema 扩展：`ResolutionStrategy[]`（含 `sub_worktree` 可选字段）
- Execute artifact schema 扩展：`ResolutionClaim[]`
- Verify 新增 `verifyResolutionCoverage()` + 4 状态决策表
- Decide 新增 `SpawnDecomposeForUnresolved` + `SpawnUserGate` 两个分支
- `DecomposeFromSubWorktree` 入口替代原 `DecomposeChildren` 处理 sub_worktree 路径
- 兼容性：旧 `execution_mode + child_specs[]` 路径保留作 fallback
- safety net：现有 `detectUserGate` 文本正则保留
- L5 单测 + 集成测试 + OpenSpec delta

### Out of Scope
- 替换或废除 `detectUserGate`（safety net 保留）
- 删除 `child_specs[]` 字段（兼容保留，标记 deprecated）
- 替换 `execution_mode: "decompose"` hint 路径（fallback 保留）
- 新增 tool 类型
- 修改 MUPS 6 节点拓扑或 ReAct loop
- TaskContract（TaskSpec/TaskReport）FF 状态
- orchtypes.TaskSpec（D2 legacy, dead code）

## 5. Impact Analysis

| Component | Change | Details |
|-----------|--------|---------|
| `orchtypes/resolution.go` | **新增** | 5 类型 |
| `mups/plan/plan.go` | **修改** | artifact schema `ResolutionStrategy[]` + LLM 引导词 |
| `mups/execute/channel.go` | **修改** | artifact schema `ResolutionClaim[]` + LLM 引导词 |
| `mups/verify/verify.go` | **修改** | `verifyResolutionCoverage()` + 4 状态决策表 |
| `workmodel/spawn_policy.go` | **修改** | `SpawnDecomposeForUnresolved` + `SpawnUserGate` 分支 |
| `workmodel/decompose.go` | **修改** | `DecomposeFromSubWorktree` 入口 |
| `workmodel/pipeline_round.go` | **修改** | `WorkItemPipelineRound.ResolutionReport` 字段 |
| `mups/item_observe.go` | **不变** | Observation 已含 obs_id + strength |
| `strategic_plan_proposer.go` | **修改** | 解析 `sub_worktree` 字段；旧 `child_specs[]` 仍解析但标 deprecated |
| `detectUserGate` regex | **保留** | safety net fallback |
| `mups-node-llm-protocols.md` | **修改** | §8 增加 Resolution + sub_worktree 引导词规范 |
| D1 Feishu | **Indirect** | SpawnUserGate 触发 ask_user_question 在 IM 表现更聚焦 |

## 6. Architecture Considerations

- **与 DM-20260704-001 互补**：DM-001 让 SpawnPolicy 主信号回归 U；本 change 让 Decide 据 obs resolution 状态**直接强制** SpawnDecompose（sub_worktree 路径），不再受 deliverable incomplete 压制。
- **与 DM-20260705-003/004 协同**：本 change 在 prompt 层引入 ResolutionStrategy/Claim 引导词，可与 D2 语义层和 Execute prompt 净化**同步落地**。
- **不与 TaskContract 冲突**：TaskContract 是节点间**接口契约**（PR #325/#327），ResolutionContract 是节点**内部 artifact schema 扩展**。两者平行存在。
- **safety net 必要性**：LLM 不按契约填字段是预期内的，所以 `execution_mode + child_specs` 旧路径**必须保留**。新增的 RC-1..RC-4 是**主路径**，旧路径是**兜底**。
- **断链 B 治本**：把 `child_specs[]` 重新定义为 `sub_worktree`（绑定 obs_id），Decide 据 HasSubWorktree 触发 SpawnDecompose——**这是 Plan 战略意图变成运行时决策的关键转换**。
- **budget safety**：RT-16 防止 sub_worktree 强制 SpawnDecompose 后子 WI 暴增；depth/children/daily 超限时退化 SpawnInline。

## 7. Success Metrics

- [ ] L5-D7-RT-01..16 单测/集成全绿
- [ ] 复现 trace `c6f2d6910496e2ea63cbcf8f207b2c0a` 场景：review d7 plan 目录 → 3 obs → Plan 3 strategies (含 sub_worktree) → Execute 消解 2/3 → Verify 1 unresolved → Decide SpawnDecompose via sub_worktree → 2 子 WI 创建
- [ ] `ResolutionCoverage` 在 Jaeger span attribute 中可见
- [ ] 现有 22/22 orchestration packages `go test -race` PASS
- [ ] OpenSpec delta 合入 `openspec/specs/d7-orchestration/`

## 8. Risks & Mitigations

| Risk | Prob | Impact | Mitigation |
|------|------|--------|------------|
| LLM 不按新契约填字段 | High | Med | safety net RT-14/15 退化路径保留 |
| sub_worktree 强制 SpawnDecompose 后单 session 子 WI 暴增 | Med | High | RT-16 budget gate：depth/children/daily 超限退化 SpawnInline |
| threshold 0.85 误触发用户门控 | Med | High | Configurable `UnresolvedStrengthThreshold`；先观察飞书反馈 |
| ResolutionClaim schema 漂移 | Low | Med | Schema 单一来源（orchtypes/resolution.go）；golden test 覆盖 |
| child_specs[] 兼容字段双写 | Low | Low | 标 deprecated；CI guard 警告旧字段使用 |
| 与 DM-20260705-004 (prompt dedup) 冲突 | Low | Low | 引导词走 execute.fieldMap guide 而非 prose 块 |

## 9. Migration Path

| Step | Action |
|------|--------|
| 1 | 新增 `orchtypes/resolution.go`（5 类型） |
| 2 | Plan artifact schema 扩展 `ResolutionStrategy[]` + LLM 引导词 append |
| 3 | Execute artifact schema 扩展 `ResolutionClaim[]` + LLM 引导词 append |
| 4 | Verify 新增 `verifyResolutionCoverage()` + 4 状态决策表 |
| 5 | Decide 新增 `SpawnDecomposeForUnresolved` + `SpawnUserGate` 分支 |
| 6 | `DecomposeFromSubWorktree` 入口 + budget gate |
| 7 | 全量单测 + 集成测试 + safety net 验证 |
| 8 | OpenSpec delta + t-registry 预登记 |

每步 PR 可独立 merge，无强顺序依赖（除 1→2/3/4/5/6 外）。

## 10. Open Questions (待 S2 澄清)

1. ResolutionClaim.Answer 字段：自由文本 vs 结构化 JSON？（倾向 JSON，统一 schema）
2. confidence < 0.7 算 UnresolvedObs 还是 PartialCoverage？（倾向 UnresolvedObs）
3. depth > 0 时是否允许 SpawnUserGate？（倾向仅 depth=0）
4. ResolutionStrategy 与 TaskContract.TaskSpec.Constraints 复用？独立（避免 FF 耦合）
5. threshold 0.85 是否按 obs_kind 差异化？（倾向统一，先观察）
6. sub_worktree 数量上限？（倾向 ≤ ResolutionStrategy 总数；与 Budget 联动）
7. 旧 `child_specs[]` 字段何时移除？（倾向 1 个版本 deprecated 后再删）

## 11. 关联变更

| 变更 | 关系 |
|------|------|
| DM-20260704-001 (d7-uncertainty-spawn-decouple) | **本 change 直接扩展** |
| DM-20260705-003 (mups-semantics-schema-alignment) | 并行：locale-neutral 语义层可复用 |
| DM-20260705-004 (mups-node-prompt-dedup) | 并行：Execute prompt 净化同步 |
| DM-20260629-007/008 (devrix-d7-taskcontract-unification PR-A/B) | 独立：TaskContract 与 ResolutionContract 平行 |
| sess_1783239758810_0 / trace c6f2d6910496e2ea63cbcf8f207b2c0a | 触发场景（断链 A + B 同时暴露） |

## 12. 追溯

```
DM-20260704-006 (demand.md)
  → proposal.md (本文 Option C unified Obs→Execution)
  → design.md (S3)
  → specs/d7-orchestration_resolution_contract_delta.md (OpenSpec delta)
  → tasks.md
  → code:
      - orchtypes/resolution.go: 5 types
      - mups/plan/plan.go: Plan schema + LLM 引导
      - mups/execute/channel.go: Artifact schema + ResolutionClaim 引导
      - mups/verify/verify.go: verifyResolutionCoverage()
      - workmodel/spawn_policy.go: SpawnDecomposeForUnresolved + SpawnUserGate 分支
      - workmodel/decompose.go: DecomposeFromSubWorktree 入口
      - mups-node-llm-protocols.md: §8 Resolution 引导词规范
  → acceptance-report.md (S5)
```