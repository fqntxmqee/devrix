# Design: D7 MUPS Obs→Execution 统一契约 (ResolutionContract + DecideBinding)

**Change ID:** `devrix-d7-uncertainty-resolution-traceability`
**Demand ID:** DM-20260704-006
**Status:** S3_Design (Draft)
**Audience:** D7 编排 / MUPS 5 节点 / i18n / Review
**代码锚点:** `orchtypes/resolution.go` (NEW), `mups/plan/plan.go`, `mups/execute/channel.go`, `mups/verify/verify.go`, `workmodel/spawn_policy.go`, `workmodel/decompose.go`, `workmodel/pipeline_round.go`, `sessionorchestrator/strategic_plan_proposer.go`, `i18n/prompttags_semantics_{zh,en}.go`, `i18n/format_hints_mups.go`

**前置:**
- [`d7-uncertainty-spawn-decouple/design.md`](../d7-uncertainty-spawn-decouple/..) — DM-20260704-001 (CC-U1～U6 U 驱动拓扑)
- [`d7-mups-propagation-convergence`](../2026-07-01-devrix-mups-propagation-convergence/) — DM-20260701-001 (DivergenceBudget + Reject 通道)
- `mups-semantics-schema-alignment` (DM-20260705-003) — locale-neutral SemanticRule 语义层
- `mups-node-prompt-dedup` (DM-20260705-004) — 三节点 prompt 净化
- `devrix-d7-taskcontract-unification` (DM-20260629-007/008) — TaskContract (TaskSpec/TaskReport) — **平行不冲突**

---

## ① 架构目标

### 业务目标

**治本双断链：**

| 断链 | 现状 | 治本后 |
|------|------|--------|
| **A: Obs→Resolution** | Plan 不声明消解策略；Execute 把答案混在 prose；Verify 用文本正则扫 | Plan → `ResolutionStrategy[]` 显式声明每条 obs 的消解策略；Execute → `ResolutionClaim[]` 显式声明每条 obs 的答案；Verify → `ResolutionReport{CoverageRatio, UnresolvedObs[]}` 量化 |
| **B: Plan→Decide** | Plan LLM `execution_mode: "decompose"` + `child_specs[]` 是叙事意图；Decide 不读 ChildSpecs | Plan → `ResolutionStrategy[].sub_worktree`（绑定 obs_id）→ Decide 强制 SpawnDecompose，**不再受 deliverable incomplete 压制** |
| **C: SubWorktree→Directive** | 即使 SpawnDecompose，子 WI 的 directive 与同层 Execute 完全分离 | `DecomposeFromSubWorktree` 入口：`child.Directive = parent_directive + "\n\n" + sub_worktree.DirectiveSuffix` |

### 技术目标

| 指标 | 当前 | 目标 |
|------|------|------|
| LLM 战略意图 (Plan) → 运行时决策 (Decide) 转化率 | 0% (Plan `child_specs[]` 整段被忽略) | 100% (sub_worktree 触发 SpawnDecompose) |
| obs_uncertainty 消解可追溯性 | 不可量化 | `CoverageRatio` 在 Jaeger span attribute 可见 |
| `ask_user_question` 触发准确度 | 文本正则误判率高 | `MaxUnresolvedStrength ≥ 0.85` + `HasSubWorktree == false` 双重条件 |
| budget 安全 | 无 sub_worktree 路径上限 | depth/children/daily 三重 gate，超限降级 SpawnInline |

### 约束条件

- **SemVer:** d7-domain minor bump (v6.x → v7.0) — 涉及 artifact schema 扩展
- **灰度:** FeatureFlag `resolution_contract_v1` (default OFF) → staged rollout
- **兼容:** 旧 `execution_mode + child_specs[]` 路径保留作 fallback（safety net）
- **并行 PR 协同:** 与 `mups-semantics-schema-alignment` / `mups-node-prompt-dedup` 三个并行 PR 同步落地，prompt 引导词走 i18n fieldMap guide 而非 prose 块
- **TaskContract 平行:** TaskSpec/TaskReport 与 ResolutionContract 平行存在，不耦合 FF 状态

---

## ② 架构原则

### 设计原则（5 条）

1. **观测先行，决策后随** (Observe→Plan→Execute→Verify→Decide 5 节点顺序不变；ResolutionContract 是节点内部 artifact schema 扩展，不是新接口)
2. **冗余双写但单点权威** (`child_specs[]` 字段保留兼容，但新代码统一走 `ResolutionStrategy[].sub_worktree`；orchtypes/resolution.go 是类型单一来源)
3. **safety net 必留** (Plan LLM 不按新契约填字段是预期内的，所以旧 `execution_mode + child_specs` + `detectUserGate` 文本正则保留作 fallback)
4. **budget 闸门先于决策** (sub_worktree 触发 SpawnDecompose 前必须先过 depth/children/daily gate；超限降级 SpawnInline + warning log)
5. **跨域类型上提 orchtypes/** (5 类型统一在 `internal/layers/orchestration/orchtypes/resolution.go`，跨域 import 时 D2/D7 都引用同一来源，避免循环依赖)

### 命名规范

| 类别 | 模板 | 示例 |
|------|------|------|
| 类型 | `Resolution{Strategy,Claim,Report}` + `SubWorktreeSpec` + `UnresolvedObs` | `ResolutionStrategy{ObsID, PlannedTool, SuccessCriterion, SubWorktree}` |
| 字段 | `ObsID` (string, 必填) / `PlannedTool` (string) / `Confidence` (float64 [0,1]) | `claim.ObsID = "obs_u1"` |
| 错误 | `ErrResolutionStrategyMissing` / `ErrObsIDMismatch` | sentinel error |
| Span | `d7.mups.resolution_coverage` (CoverageRatio + UnresolvedObs 长度) | sessionSpan attribute |

### 代码风格

- 函数 < 50 行（`DecideFromResolutionReport` 主函数 + 3 个子函数 `pickSpawnDecompose`/`pickSpawnUserGate`/`pickSpawnInline`）
- 文件 < 800 行（orchtypes/resolution.go 5 类型 + 4 验证方法 ≈ 200 行）
- 不可变值对象（`With*` 返回新副本），实体通过 method 加锁变更状态

---

## ③ 业务流程

### 核心用例时序图：复现 trace `c6f2d6910496e2ea63cbcf8f207b2c0a`

```
Parent WI (depth=0)  directive: "review d7 领域 plan目录下代码"
   │
   ▼ Observe ──────────▶ 3 obs_uncertainty:
   │                       obs_u1 (strength=0.92, scope: "读 plan/*.go")
   │                       obs_u2 (strength=0.88, scope: "读 strategy.go")
   │                       obs_u3 (strength=0.82, scope: "读 rollup_directive.go")
   │
   ▼ Plan ──────────────▶ ResolutionStrategy[] (3 条):
   │                       obs_u1 → { PlannedTool: read_file, SubWorktree: { Title: "读 plan 目录", DirectiveSuffix: "请读 plan/*.go", ScopeIn: ["plan/"] } }
   │                       obs_u2 → { PlannedTool: read_file, SubWorktree: { Title: "读 strategy", DirectiveSuffix: "请读 strategy.go", ScopeIn: ["workmodel/"] } }
   │                       obs_u3 → { PlannedTool: read_file, SubWorktree: nil }  // 同层 Execute 处理
   │
   ▼ Execute ───────────▶ ResolutionClaim[] (1 条):
   │                       obs_u3 → { Answer: "...", Confidence: 0.85, SupportingEvidence: "rollup_directive.go:120" }
   │                       // obs_u1, obs_u2 未被消解（tools 失败，files_inspected=0）
   │
   ▼ Verify ────────────▶ verifyResolutionCoverage():
   │                       TotalStrategies=3, TotalClaims=1
   │                       CoverageRatio = 1/3 = 0.333
   │                       UnresolvedObs = [
   │                         {ObsID: "obs_u1", Strength: 0.92, HasSubWorktree: true, Reason: "no_resolution_claim"},
   │                         {ObsID: "obs_u2", Strength: 0.88, HasSubWorktree: true, Reason: "no_resolution_claim"}
   │                       ]
   │
   ▼ Decide ─────────────▶ DecideFromResolutionReport():
   │                       // RC-4a: HasSubWorktree=true → SpawnDecompose
   │                       round.ChildSpecs = [
   │                         {Directive: parent + "\n\n请读 plan/*.go", ScopeIn: ["plan/"], Title: "读 plan 目录", ExpectedReturn: "..."},
   │                         {Directive: parent + "\n\n请读 strategy.go", ScopeIn: ["workmodel/"], Title: "读 strategy", ExpectedReturn: "..."}
   │                       ]
   │                       SpawnPolicy = SpawnDecompose   // ← 治本！旧逻辑会因 deliverable incomplete 走 SpawnInline
   │
   ▼ Decompose ──────────▶ DecomposeFromSubWorktree(round.ChildSpecs):
   │                       // 创建 2 个 child WI (wi_d1_s0_child_u1, wi_d1_s0_child_u2)
   │                       // 每个 child.Directive = parent_directive + "\n\n" + sub_worktree.DirectiveSuffix
   │                       // child.ScopeIn = sub_worktree.ScopeIn
   │
   ▼ Child Execute (parallel)
   │   child_u1 → read_file plan/*.go → ResolutionClaim obs_u1
   │   child_u2 → read_file strategy.go → ResolutionClaim obs_u2
   │
   ▼ Child Verify + Rollup → Parent
```

### 异常补偿（Fallback 路径表）

| 异常 | Fallback 路径 | L5 测试点 |
|------|---------------|-----------|
| Plan LLM 不填 `ResolutionStrategy[]` (旧格式) | 退化到 `execution_mode + child_specs[]` 路径（R0-R8） | RT-14, RT-15 |
| Plan LLM 完全无输出 | 退化到 `execution_mode` hint + `detectUserGate` 文本正则 | RT-15 |
| sub_worktree budget 超限 (depth/children/daily) | 退化到 SpawnInline + `sub_worktree_budget_exceeded` warning log | RT-16 |
| Execute LLM 不填 `ResolutionClaim[]` | Verify 用 CoverageRatio=0 触发 Partial/Fail + "no resolution claims" rationale | RT-06 |
| Plan LLM 填 `ResolutionStrategy[]` 但 obs_id 与 Observe 不对齐 | Verify 计算 CoverageRatio 时 obs_id_mismatch → UnresolvedObs | RT-03 |
| confidence < 0.7 | 算 UnresolvedObs（reason="low_confidence"），走 RC-4a/b/c | RT-03 |

### 分支处理决策树

```
Verify 输出 ResolutionReport
   │
   ├─ UnresolvedObs 全空 ─────────────────▶ 沿用现有 VerdictKind-based 决策 (CC-U1)
   │
   └─ UnresolvedObs 非空
         │
         ├─ 任一 HasSubWorktree=true ─────▶ RC-4a: checkBudget → SpawnDecompose
         │                                  │
         │                                  └─ budget 超限 ──▶ SpawnInline + warning
         │
         ├─ 全 HasSubWorktree=false + MaxUnresolvedStrength ≥ 0.85
         │                                  ──────▶ RC-4b: SpawnUserGate (tool_filter=[ask_user_question])
         │
         └─ 其它 ─────────────────────────▶ RC-4c: 沿用 SpawnInline (现有 R0-R8 逻辑)
```

---

## ④ 领域模型

### 聚合根：`ResolutionContract`（orchtypes/resolution.go 5 类型）

```
ResolutionContract (聚合根)
   ├─ ResolutionStrategy[]   ← Plan 产出
   │    ├─ ObsID            (string, 必填, 与 Observation.ObsID 对齐)
   │    ├─ PlannedTool      (string, tool registry 中的 tool name)
   │    ├─ SuccessCriterion (string, 自由文本)
   │    └─ SubWorktree      (*SubWorktreeSpec, 可选; nil = 同层 Execute 处理)
   │
   ├─ ResolutionClaim[]     ← Execute 产出
   │    ├─ ObsID              (string)
   │    ├─ Answer             (string 或 JSON)
   │    ├─ Confidence         (float64, [0, 1])
   │    └─ SupportingEvidence (string, 引用 tool output 或文件:行号)
   │
   └─ ResolutionReport      ← Verify 产出
        ├─ TotalStrategies    (int)
        ├─ TotalClaims        (int)
        ├─ CoverageRatio      (float64, TotalClaims/TotalStrategies)
        ├─ UnresolvedObs[]    (UnresolvedObs 列表)
        └─ Reason             (string, "no_resolution_claim" | "low_confidence" | "obs_id_mismatch")

UnresolvedObs {
   ObsID          string
   Strength       float64
   Reason         string
   HasSubWorktree bool
}
```

### 限界上下文（包边界图）

```
internal/layers/orchestration/
   orchtypes/
      resolution.go           ← 5 类型 (ResolutionStrategy/SubWorktreeSpec/ResolutionClaim/ResolutionReport/UnresolvedObs) + 4 验证方法
   workmodel/
      spawn_policy.go         ← 新增 DecideFromResolutionReport() + RC-4a/b/c 分支
      decompose.go            ← 新增 DecomposeFromSubWorktree() 入口
      pipeline_round.go       ← 新增 WorkItemPipelineRound.ResolutionReport 字段
   mups/
      plan/plan.go            ← artifact schema 扩展 ResolutionStrategy[] + 引导词
      execute/channel.go      ← artifact schema 扩展 ResolutionClaim[] + 引导词
      verify/verify.go        ← 新增 verifyResolutionCoverage() + 4 状态决策表
   sessionorchestrator/
      strategic_plan_proposer.go ← 解析 sub_worktree 字段; 保留 child_specs[] 兼容路径

internal/layers/contextengine/i18n/
   format_hints_mups.go       ← 新增 ResolutionStrategy/Claim fieldMap guide (ZH/EN)
   prompttags_semantics_{zh,en}.go ← observe/plan/execute appendix 含 ResolutionContract 引导词
```

### 领域事件（Span/Metric 列表）

| Span/Metric | 节点 | 说明 |
|-------------|------|------|
| `d7.mups.resolution_coverage` | Verify | CoverageRatio + TotalStrategies + TotalClaims + UnresolvedObsLen |
| `d7.mups.resolution_claim` | Execute | claim.ObsID + claim.Confidence |
| `d7.mups.resolution_strategy` | Plan | strategy.ObsID + strategy.HasSubWorktree |
| `d7.spawn.from_resolution` | Decide | decision_action (decompose/user_gate/inline) + unresolved_obs_count + max_strength |

### 跨域消费模型

- **D2 contextengine/i18n**: ResolutionContract 引导词走 fieldMap guide（不写 prose），与 DM-20260705-003 SemanticRule 共享 schema
- **D1 communication/feishu**: SpawnUserGate 触发 ask_user_question 时，IM 卡片显示聚焦的 unresolved_obs 列表（而不是模糊的"task incomplete"）
- **D7 orchtypes**: 跨域类型上提（ResolutionReport 被 Observe/Plan/Execute/Verify/Decide 5 节点共享，单一来源）

---

## ⑤ 核心链路图

### 端到端路径

```
[Parent WI depth=0]
   │
   ▼ Observe 节点 ─────────────────────────────────────────────
   │   mups/observe/item_observe.go::ObserveWorkItem
   │   产出: []Observation (含 obs_id + strength)
   │
   ▼ Plan 节点 ────────────────────────────────────────────────
   │   mups/plan/plan.go::PlanWorkItem
   │   产出: ResolutionStrategy[] (含可选 sub_worktree)
   │   + 旧 child_specs[] (兼容字段, 标 deprecated)
   │
   ▼ Execute 节点 ─────────────────────────────────────────────
   │   mups/execute/channel.go::Channel.Execute
   │   产出: Artifact + ResolutionClaim[]
   │
   ▼ Verify 节点 ──────────────────────────────────────────────
   │   mups/verify/verify.go::verifyResolutionCoverage
   │   产出: ResolutionReport {TotalStrategies, TotalClaims, CoverageRatio, UnresolvedObs[]}
   │   + 旧 deliverable verify (沿用)
   │
   ▼ Decide 节点 ──────────────────────────────────────────────
   │   workmodel/spawn_policy.go::DecideFromResolutionReport
   │   决策: SpawnDecompose | SpawnUserGate | SpawnInline
   │
   ▼ Decompose (若 SpawnDecompose) ────────────────────────────
   │   workmodel/decompose.go::DecomposeFromSubWorktree
   │   产出: []WorkItem (每个 child.Directive = parent + sub_worktree.DirectiveSuffix)
   │
   ▼ [Child WI Execute + Verify + Rollup → Parent]
```

### 时序标注

| 节点 | P99 RT (目标) | 单点风险 | 缓解 |
|------|---------------|----------|------|
| Observe | < 50ms | obs_id 唯一性 | Existing validate |
| Plan | < 200ms (含 LLM) | LLM 不填 sub_worktree | safety net RT-14/15 |
| Execute | < 5s (含 LLM + tools) | LLM 不填 claim | ParseRejectRecord 反馈注入下一轮 |
| Verify | < 10ms (纯函数) | obs_id 对齐 | Set 校验 |
| Decide | < 5ms | budget gate | 早退 |
| Decompose | < 20ms | sub_worktree 数量暴增 | budget gate RT-16 |

### 单点风险与缓解

| 单点 | 风险 | 缓解 |
|------|------|------|
| orchtypes/resolution.go | 5 类型定义演化 | SemVer 守住；旧字段 deprecated 不删 |
| spawn_policy.go RC-4a 路径 | sub_worktree 触发 SpawnDecompose 后子 WI 暴增 | RT-16 budget gate |
| prompttags_semantics_*.go | LLM 不按新契约填字段 | safety net + ParseRejectRecord |
| coverage ratio 计算 | obs_id 集合对齐性能 | Set O(1) 查找 |

---

## ⑥ 接口/API 设计

### 风格

**Pure types + Builder + With*:**

```go
// 单一来源: internal/layers/orchestration/orchtypes/resolution.go

type ResolutionStrategy struct {
    ObsID            string
    PlannedTool      string
    SuccessCriterion string
    SubWorktree      *SubWorktreeSpec
}

func NewResolutionStrategy(obsID, tool, criterion string, sub *SubWorktreeSpec) (*ResolutionStrategy, error) {
    if obsID == "" {
        return nil, ErrResolutionStrategyObsIDEmpty
    }
    return &ResolutionStrategy{ObsID: obsID, PlannedTool: tool, SuccessCriterion: criterion, SubWorktree: sub}, nil
}

func (s ResolutionStrategy) WithSubWorktree(sub SubWorktreeSpec) ResolutionStrategy {
    s.SubWorktree = &sub
    return s
}

// 不可变值对象
type ResolutionClaim struct {
    ObsID              string
    Answer             string
    Confidence         float64
    SupportingEvidence string
}

func (c ResolutionClaim) WithConfidence(conf float64) ResolutionClaim {
    c.Confidence = clamp01Float(conf)
    return c
}

type ResolutionReport struct {
    TotalStrategies int
    TotalClaims     int
    CoverageRatio   float64
    UnresolvedObs   []UnresolvedObs
    Reason          string
}
```

### 契约（错误码三元组 + TraceID）

```go
// orchtypes/resolution.go
var (
    ErrResolutionStrategyObsIDEmpty  = errors.New("resolution strategy: obs_id empty")
    ErrResolutionStrategyInvalid     = errors.New("resolution strategy: invalid")
    ErrResolutionClaimObsIDEmpty     = errors.New("resolution claim: obs_id empty")
    ErrResolutionClaimConfidenceOutOfRange = errors.New("resolution claim: confidence out of [0,1]")
    ErrObsIDMismatch                 = errors.New("obs_id mismatch between plan and observation")
)

// 4 错误码: 4 类可恢复错误
// + 5 类验证错误: empty/duplicate/format/import cycle/cycle
```

### 幂等

- `verifyResolutionCoverage()` 是纯函数（输入 plan strategies + execute claims → 输出 report），可重入
- `DecideFromResolutionReport()` 仅读 round state，不修改；修改发生在 DecideFromVerdict
- `DecomposeFromSubWorktree` 创建 child WI 用 IdempotencyKey 保证不重复创建

### 版本演进

- **v1.0 (本 change):** ResolutionStrategy/Claim/Report/UnresolvedObs 4 类型，RC-1..RC-4 完整契约
- **v1.1 (backlog):** ResolutionClaim.Answer 支持结构化 JSON（与 DM-001 ParseRejectRecord 复用）
- **v2.0 (next minor):** `child_specs[]` 字段标记 deprecated 后删除（1 版本后）
- **breaking change:** 删 `child_specs[]` 时需要 2 版本 deprecation period

### API 暴露面

| 层 | API | 说明 |
|----|-----|------|
| **D7 内部** | `orchtypes.NewResolutionStrategy/Claim/Report` | 构造函数 |
| **D7 内部** | `workmodel.DecideFromResolutionReport(round, report) SpawnPolicy` | 决策函数 |
| **D7 内部** | `workmodel.DecomposeFromSubWorktree(subWorktrees) []WorkItem` | 拆解函数 |
| **D2 i18n** | `i18n.AppendResolutionStrategyGuide(strategy)` | fieldMap guide ZH/EN |
| **LLM prompt** | Plan/Execute LLM 输出含 `resolution_strategies` / `resolution_claims` 字段 | JSON 结构化输出 |

---

## 附录 A：File Manifest（Phase 1-5 实施分解）

| Phase | 文件 | 描述 | 增量 |
|-------|------|------|------|
| Phase 1 | `orchtypes/resolution.go` (NEW) | 5 类型 + 4 验证方法 | +200 行 |
| Phase 1 | `mups/plan/plan.go` (MOD) | artifact schema + 引导词 | +50 行 |
| Phase 1 | `mups/execute/channel.go` (MOD) | artifact schema + 引导词 | +50 行 |
| Phase 1 | `strategic_plan_proposer.go` (MOD) | 解析 sub_worktree + 兼容 child_specs | +30 行 |
| Phase 1 | `i18n/format_hints_mups.go` (MOD) | ResolutionStrategy/Claim fieldMap guide | +40 行 |
| Phase 2 | `mups/verify/verify.go` (MOD) | verifyResolutionCoverage() + 4 状态决策表 | +80 行 |
| Phase 2 | `workmodel/pipeline_round.go` (MOD) | ResolutionReport 字段 | +10 行 |
| Phase 3 | `workmodel/spawn_policy.go` (MOD) | DecideFromResolutionReport + RC-4a/b/c | +60 行 |
| Phase 4 | `workmodel/decompose.go` (MOD) | DecomposeFromSubWorktree + budget gate | +40 行 |
| Phase 5 | 各文件 (MOD) | safety net + 全量测试 + OpenSpec delta | +200 行 |

**总增量:** ~760 行, 跨 7-9 文件, 0 函数签名变化 (除 Decide 新分支外)

---

## 附录 B：Rollback Plan

| 阶段 | 回滚方式 |
|------|----------|
| Phase 1 (orchtypes/resolution.go) | 直接 `git revert` PR；类型未引用时 0 影响 |
| Phase 2-4 | FeatureFlag `resolution_contract_v1 = OFF`；旧 `execution_mode + child_specs[]` 路径仍可用 |
| Phase 5 (safety net) | 旧 `detectUserGate` 文本正则保留；fallback 路径不删 |

**回滚触发条件:**
- CoverageRatio 计算性能回归 (> 10ms)
- sub_worktree 触发 SpawnDecompose 后单 session 子 WI > 50（实际生产触发 budget gate）
- LLM 不按新契约填字段率 > 30%（safety net fallback 命中率）

---

## 附录 C：回归风险

| 风险 | 概率 | 影响 | 缓解 |
|------|------|------|------|
| LLM 不按新契约填 `ResolutionStrategy[].sub_worktree` | High | Med | safety net RT-14/15 退化路径保留 |
| sub_worktree 强制 SpawnDecompose 后单 session 子 WI 暴增 | Med | High | RT-16 budget gate (depth/children/daily) |
| threshold 0.85 误触发 SpawnUserGate | Med | High | Configurable `UnresolvedStrengthThreshold`；先观察飞书反馈 |
| ResolutionClaim schema 漂移 | Low | Med | Schema 单一来源（orchtypes/resolution.go）；golden test 覆盖 |
| `child_specs[]` 兼容字段双写 | Low | Low | 标 deprecated；CI guard 警告旧字段使用 |
| 与 `mups-node-prompt-dedup` (DM-20260705-004) 冲突 | Low | Low | 引导词走 execute.fieldMap guide 而非 prose 块 |

---

## 附录 D：S3 Checklist

- [x] ① 架构目标（业务目标 + 技术指标 + 约束条件）
- [x] ② 架构原则（5 条 + 命名规范 + 代码风格）
- [x] ③ 业务流程（时序图 + Fallback 路径表 + 分支决策树）
- [x] ④ 领域模型（聚合根 + 限界上下文 + 领域事件 + 跨域消费）
- [x] ⑤ 核心链路图（端到端 + 时序标注 + 单点风险）
- [x] ⑥ 接口/API 设计（风格 + 契约 + 幂等 + 版本演进）
- [x] File Manifest（Phase 1-5 实施分解）
- [x] Rollback Plan
- [x] 回归风险
- [x] 引用 detail-design-framework.md 六段式（禁止旧式 7 段）
- [ ] d7-orchestration/t-registry.md 预登记 T points (PLANNED)
- [ ] d7-orchestration/spec_delta.md (S3 spec delta) 已创建
- [ ] S3-Gate Review 通过 → 进入 S4 实现

---

## 附录 E：下一步

| 步骤 | 行动 | 期望产出 |
|------|------|----------|
| S3-Gate | 提交 Review | design.md 通过审核 |
| S4 实施 Phase 1 | orchtypes/resolution.go + Plan/Execute schema + i18n guide | 1 PR (本 change v1.0) |
| S4 实施 Phase 2-3 | Verify + Decide binding | 1-2 PR (含 RC-4a/b/c) |
| S4 实施 Phase 4 | DecomposeFromSubWorktree + budget gate | 1 PR |
| S4 实施 Phase 5 | safety net + 全量测试 + OpenSpec delta | 1 PR |
| S5 验收 | L5-D7-RT-01..16 单测/集成全绿 | acceptance-report.md |
| S6 归档 | git mv changes/ → archive/ + demand-archive-index.md | S7_Archived |

**Phase 1 PR 优先级: P0**（治本断链 B 治本 + 数据契约就位）
**Phase 2-3 PR 优先级: P0**（Verify decision table + Decide binding）
**Phase 4-5 PR 优先级: P1**（Decompose + safety net）