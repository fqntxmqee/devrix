# Design: devrix-d7-dsaft-restructuring (DM-20260629-001)

**Change ID:** `devrix-d7-dsaft-restructuring`
**Demand ID:** DM-20260629-001
**DSAFT 阶段:** §阶段 3 North Star 重整 + §阶段 4 v1.1 Traceability + §阶段 6 双锚点对齐 + 原则 1/3/4/5 修复

> **本文档结构：S 层 → A 层 → F 层 → 清理 → Span + Boundary 逐层展开。** 每层包含：现状评估 / 关键设计决策 / 详细变更清单 / 验证方式。

---

## §0 总览：S→A→F 逐层 + 清理

```
                  ┌────────────────────────────────────┐
                  │ S 层（价值流语义升级 + S6 双角色分组）│
                  │   #3 value-flow-rename + #5 boundary│
                  └────────────────────────────────────┘
                                     ↓
                  ┌────────────────────────────────────┐
                  │ A 层（god function 拆分 + 36 T 重映射）│
                  │   #1 turn-fn-split (D7-S2-A06→A06-A09) │
                  │   + WorkTree 上行反馈治理 (§2.4)        │
                  └────────────────────────────────────┘
                                     ↓
                  ┌────────────────────────────────────┐
                  │ F 层（路径修正 + 死代码删 + 跨包升格）  │
                  │   #0 dead-code-cleanup + #2 registry │
                  └────────────────────────────────────┘
                                     ↓
                  ┌────────────────────────────────────┐
                  │ 横切层（Span 增补 + Boundary governance）│
                  │   #4 t-span-coverage + #5 governance │
                  └────────────────────────────────────┘
```

**清理贯穿 S/A/F 3 层**（不是单独阶段）：
- S 层清理：`Legacy 41 ghost count` / `d7-domain.md line 147 错配`
- A 层清理：`god function turn_orchestrator.go 1551 行` / `legacy explicit-code path`
- A 层扩展清理（用户 2026-06-29 复盘触发）：WorkTree 上行反馈 3 项治理（RollupReport struct + deterministic sessionRootGoal + 3 T 登记）
- F 层清理：~126 LOC 死代码 + 老链路 + 4 处 doc drift

---

## §1 S 层设计（S1-S6 + 1 横切）

### 1.1 S 层现状（盘点结论）

| S | ValueFlow 别名候选 | 物理包 | 实际散开 | 关键问题 |
|---|---|---|---|---|
| **S1 WorkModel** | Multi-Step Task Coordination | `workmodel/` + `sessionorchestrator/workmodel.go` | 2 位置 | sessionorchestrator 借位 |
| **S2 SessionOrchestrator** | Turn-Based Conversation | `sessionorchestrator/` | 1 包（1551 行 god file）| turn_orchestrator.go 拆分 |
| **S3 WaveScheduler** | Parallel Worktree Execution | `wavescheduler/` | 1 包 | ✅ 对齐 |
| **S4 ExecutionFlow+Verify** | Trustworthy Conclusion Delivery | `executionflow/{hub,bridge,imsink,workplan,verify}/` | **5 子包** | 子包散开 |
| **S5 DecisionPlanning+Observe** | Intent + Uncertainty Quantization | `decisionplanning/` + `orchtypes/` + `plan/` | **3 位置** | Observe 跨包拆 |
| **S6 MUPS Pipeline** | Learn from Outcome | `mups/execute/` + `mups/learn/` + `escape/` + `sessionorchestrator/{autoclose,observe_request,resume}.go` + `executionflow/verify/` | **5 位置** ⚠️ 最严重 | S6 ≠ 1 包 |
| **Hardening** | Discipline Keeper | `hardening/` + `escape/circuit_breaker.go`（KEEP） | 2 包 | circuit_breaker Decision 1 留 escape/ |

### 1.2 S 层关键设计决策

#### Decision S-1: S6 不拆 S6a/S6b，加内部 sub-header

**理由**：v6.0.0 已 S7_Archived（14 S → 6 S），再次重组风险大；内部 sub-header 满足"显式分组"诉求。

**实现**：`d7-domain.md §North Star` S6 行加 sub-header：

```markdown
| 4 Channel + ChannelRouter + C2/W8 1:1 映射 + 4 LearningClass + 3 通道记忆 + ReputationEvidence Bayesian | **D7-S6 MUPS Pipeline** (sub-group: S6a Execute + S6b Learn) | **Learn from Outcome** | Pipeline Coordinator + Memory Curator |
```

#### Decision S-2: 6 S 全部配 ValueFlow Alias（语义层叠加）

**实现**：见 `tasks.md` T30-T34。

#### Decision S-3: 3 项 boundary debt 显式 Decision（不立即下沉）

**实现**：见 §7 + `tasks.md` T45-T48。

### 1.3 S 层验证

| 验证项 | 命令 | 通过条件 |
|---|---|---|
| 6 S ValueFlow Alias 完整 | `rg "ValueFlow Alias" openspec/specs/d7-orchestration/d7-domain.md` | ≥6 |
| S6 sub-header | `rg "S6a Execute.*S6b Learn" openspec/specs/d7-orchestration/d7-domain.md` | ≥1 |
| 3 项 boundary debt 标注 | `rg "boundary-borrowed" openspec/specs/d7-orchestration/` | ≥3 |
| pipeline-architecture v1.2.0 6 S | `head -20 openspec/specs/d7-orchestration/pipeline-architecture.md` | "v1.2.0" + "6 S" |

---

## §2 A 层设计（49 A → 52 A）

### 2.1 A 层现状（盘点结论）

| S | 声明 A 数 | 实际 | 关键问题 |
|---|---|---|---|
| S1 | 4 | 4 + PlanMode 附加 3 | A04/A05/A06 是补丁 |
| S2 | 7 | 7 | **A06 god activity（36 T 单挂）** |
| S3 | 4 | 4 | ✅ |
| S4 | 9 | 9 | verify-promotion A50 PLANNED 待决 |
| S5 | 8 | 8 | 跨 3 物理位置 |
| S6 | 15 | 15 | 散 5 物理位置 |
| Hardening | 2 | 2 | ✅ |

### 2.2 A 层关键设计决策

#### Decision A-1: D7-S2-A06 RunTurnLoop 拆 4 子活动（最重要）

**问题**：D7-S2-A06 RunTurnLoop 是典型 god activity，36 T 全部挂此单一函数（`session_turn_loop.go::RunSessionTurnLoop`）。

**拆分方案**：

| 原 A ID | 新 A ID | 归属文件 | T 数 | A 职责 |
|---|---|---|---|---|
| **D7-S2-A06** | D7-S2-A06 RunTurn 主入口 | `turn_orchestrator.go` | ~10 | ProcessMessage 入口 + IntentKind 路由 + WaitTurn/BeginTurn/EndTurn |
| **D7-S2-A07** | D7-S2-A07 SessionTurnLoop | `turn_loop.go` | ~10 | SessionTurnLoop + iter loop + escape 检查 + ItemPipelineRunner 调用 |
| **D7-S2-A08** | D7-S2-A08 LLM Invoke + ReAct | `turn_invoke.go` | ~8 | LLMInvoke + ToolRound + ReAct iter + 子对话 emit |
| **D7-S2-A09** | D7-S2-A09 Error Recovery + Resume | `turn_recovery.go` | ~8 | EscapeEngine 接入 + ResumeSession 3 决策 + Error Recovery |

**约束**：
- **0 函数签名变化**（pure physical split）
- **D7-S15 TurnState 接口不变**（DM-20260628-003 刚闭环）
- 36 T 全部归属正确（不再单挂一函数）

#### Decision A-2: D7-S6 = 15 A 保留，加 sub-header

**理由**：v6.0.0 已 S7_Archived，再次重组风险大；sub-header 满足显式分组诉求。

**实现**：`a-registry.md §D7-S6` 段落加 sub-header：

```markdown
## D7-S6: MUPS Pipeline ✅ IMPLEMENTED (v6.0.0)

### S6a Execute (Pipeline Coordinator, A01-A09)
- A01 ChannelRouter
- A02-A05 4 Channel (Commit/Protocol/Scenario/Exploration)
- ...

### S6b Learn (Memory Curator, A10-A15)
- A10 RunLearner
- A11-A14 4 LearningClass (SOP/Protocol/Knowledge/Conclusion)
- A15 Pending Asset (LP-2 隔离)
```

#### Decision A-3: D7-S4-A50 verify-promotion PLANNED → 决策

**当前**：t-registry D7-S4-A50-T01..T04 PLANNED（verify-promotion 包迁移 v6.0.0 follow-up #5）。

**决策**：
- **方案 A**：PLANNED → IMPLEMENTED（v6.0.0 已部分落地，PR #222/#223 已合 verify 包 promote，需补 T）
- **方案 B**：CANCEL（v6.0.0 已 S7_Archived，verify-promotion 已完成 spec 侧，code 侧复用 v2.3.0 物理迁移）

**选择**：方案 A（T01-T04 IMPLEMENTED）。

### 2.3 A 层验证

| 验证项 | 命令 | 通过条件 |
|---|---|---|
| D7-S2-A06-A09 子活动 | `rg "^D7-S2-A0[6-9]" openspec/specs/d7-orchestration/a-registry.md` | ≥4 |
| D7-S6 sub-header | `rg "^### S6[ab]" openspec/specs/d7-orchestration/a-registry.md` | ≥2 |
| D7-S4-A50 IMPLEMENTED | `rg "D7-S4-A50.*IMPLEMENTED\|D7-S4-A50.*✅" openspec/specs/d7-orchestration/t-registry.md` | ≥4 |
| A 总数 49 → 52 | `rg "^D7-S" openspec/specs/d7-orchestration/a-registry.md \| wc -l` | ≥52 |
| 36 T 重映射 | `rg "D7-S2-A0[6-9]" openspec/specs/d7-orchestration/t-registry.md \| wc -l` | ≥36 |

### 2.4 A 层扩展：WorkTree 上行反馈治理（用户 2026-06-29 复盘触发）

#### 2.4.1 现状问题

**向下传播**：✅ 清晰（单入口 `ApplyPipelineDecide` + 强类型 `ChildDownlink` + 自动作用域绑定）。

**向上反馈**：⚠️ 不清晰（3 个具体问题）：
1. **`ReevaluateParentAfterChild` 3 调用点缺统一治理**：`sessionorchestrator/session_turn_loop.go:194` + `workmodel/run_spawn.go:51` + `workmodel/cli_commands.go:329` 共享 per-parent `sync.Mutex`，但**无 T 守护 3 调用点幂等性**。
2. **`child.LastRound` 作隐式 envelope**：`context_bubble_apply.go:67/188` + `rollup_gate.go:36/135/154` 共 5 处自由取字段（`ContextBubbleKind / ArtifactSummary / SpawnPolicy / VerdictKind / UncertaintyMean`），无强类型约束。
3. **`sessionRootGoal` 非确定性遍历**：`rollup_gate.go:122-129` 直接 `for range map` 返回"first"——DM-20260628-003 修了 locked-root priority bug 但**根选择逻辑本身仍非确定**。

#### 2.4.2 改进 A：RollupReport struct（typed envelope）

**位置**：`internal/layers/orchestration/workmodel/rollup_report.go`（NEW）

**设计**：

```go
// RollupReport 强类型聚合 child.LastRound → 上行反馈 envelope (DM-20260629-001-A).
// 替代 context_bubble_apply.go / rollup_gate.go 多处 child.LastRound.* 直接取字段。
// 保持 child.LastRound source-of-truth 不变，RollupReport 仅作为读取侧聚合。
type RollupReport struct {
    ChildID          string    `json:"child_id"`
    VerdictKind      string    `json:"verdict_kind"`       // pass / partial / fail / indeterminate
    ArtifactSummary  string    `json:"artifact_summary"`   // 与 rollup_gate.go:154 一致
    UncertaintyMean  float64   `json:"uncertainty_mean"`   // LP-1 Bayesian 后验均值
    SpawnPolicy      string    `json:"spawn_policy"`       // SpawnDecompose/SpawnAwait/SpawnNone
    BubbleKind       string    `json:"bubble_kind"`        // context_bubble_apply.go:67
    GeneratedAt      time.Time `json:"generated_at"`
}

// NewRollupReportFromRound 构造器（保持 child.LastRound 不变，RollupReport 仅作为读取侧聚合）。
func NewRollupReportFromRound(childID string, round *WorkItemPipelineRound) *RollupReport {
    if round == nil {
        return &RollupReport{ChildID: childID, GeneratedAt: time.Now()}
    }
    return &RollupReport{
        ChildID:         childID,
        VerdictKind:     string(round.VerdictKind),
        ArtifactSummary: round.ArtifactSummary,
        UncertaintyMean: round.UncertaintyMean,
        SpawnPolicy:     string(round.SpawnPolicy),
        BubbleKind:      string(round.ContextBubbleKind),
        GeneratedAt:     time.Now(),
    }
}
```

**调用方改造**（5 处 `child.LastRound.*` → `NewRollupReportFromRound(...).*`，覆盖 3 distinct fields：ContextBubbleKind / ArtifactSummary / SpawnPolicy）：

| 位置 | 原 | 改后 | 字段 |
|---|---|---|---|
| `context_bubble_apply.go:67` | `kind := child.LastRound.ContextBubbleKind` | `kind := NewRollupReportFromRound(child.ID, child.LastRound).BubbleKind` | BubbleKind |
| `context_bubble_apply.go:188` | `summary := strings.TrimSpace(child.LastRound.ArtifactSummary)` | `summary := NewRollupReportFromRound(child.ID, child.LastRound).ArtifactSummary` | ArtifactSummary |
| `rollup_gate.go:36` | `parent.LastRound.SpawnPolicy` | `NewRollupReportFromRound(parent.ID, parent.LastRound).SpawnPolicy` | SpawnPolicy |
| `rollup_gate.go:135` | `root.LastRound.SpawnPolicy` | `NewRollupReportFromRound(root.ID, root.LastRound).SpawnPolicy` | SpawnPolicy |
| `rollup_gate.go:154` | `root.LastRound.ArtifactSummary` | `NewRollupReportFromRound(root.ID, root.LastRound).ArtifactSummary` | ArtifactSummary |

**RollupReport 7 字段 vs 5 调用点 3 字段的对应关系**：
- **当前已使用**（3 字段）：`BubbleKind` + `ArtifactSummary` + `SpawnPolicy`
- **新增未用**（4 字段）：`VerdictKind` + `UncertaintyMean` + `ChildID` + `GeneratedAt`
- **为什么新增**：LP-5 反向追溯（`D7-S12-A42`）需要 `VerdictKind`；LP-1 长期 Bayesian 后验（`D7-S13-A47`）需要 `UncertaintyMean`；`ChildID` 是 envelope 标识；`GeneratedAt` 是 trace 元数据
- **未来使用场景**：v2.6.x 维护期会被 LP-5/LP-1 acceptance test 引用（PR-7 `d7_acceptance_lp1_test.go` + `d7_acceptance_lp5_test.go`）

#### 2.4.3 改进 B：`sessionRootGoal` 确定性排序

**位置**：`internal/layers/orchestration/workmodel/rollup_gate.go:122-129`

**当前**：
```go
func sessionRootGoal(tm *TaskManager, sessionID string) *WorkItem {
    for _, item := range tm.Tree().List(sessionID) {  // ← map 遍历，顺序不定
        if item != nil && item.Kind == WorkKindGoal && item.ParentID == "" {
            return item
        }
    }
    return nil
}
```

**改后**：
```go
func sessionRootGoal(tm *TaskManager, sessionID string) *WorkItem {
    var roots []*WorkItem
    for _, item := range tm.Tree().List(sessionID) {
        if item != nil && item.Kind == WorkKindGoal && item.ParentID == "" {
            roots = append(roots, item)
        }
    }
    if len(roots) == 0 {
        return nil
    }
    sort.Slice(roots, func(i, j int) bool {
        return roots[i].ID < roots[j].ID
    })
    return roots[0]
}
```

**Unit Test**：`workmodel/rollup_gate_test.go::TestSessionRootGoal_DeterministicOrder`
- 构造 3 root（ID 顺序打乱：root_c / root_a / root_b）
- 多次调用 `sessionRootGoal`，**始终返回 root_a**（ID 最小）
- 排序前后行为对比 PASS

#### 2.4.4 改进 C：3 T 登记（D7-S0-A07）

**位置**：`openspec/specs/d7-orchestration/t-registry.md`

| T ID | 描述 | 触发场景 | Status |
|---|---|---|---|
| **D7-S0-A07-T01** | ApplyPipelineDecide 4 步顺序不变式（ContextBubbleDecision → AcceptedContextLinks → SpawnPolicy → ScopeContractSpawnGate）—— 对应实现：`workmodel/context_decide.go:4` `ApplyPipelineDecide` | 任意 turn 调用 ProcessMessage | PENDING (S4) |
| **D7-S0-A07-T02** | ReevaluateParentAfterChild 3 调用点幂等性（同一 child 多次 terminal 仅触发 1 次 rollup）—— 对应实现：`workmodel/resolve.go:7` + 3 调用点（`session_turn_loop.go:194` + `run_spawn.go:51` + `cli_commands.go:329`）| 并发场景下 3 调用点同时触发 | PENDING (S4) |
| **D7-S0-A07-T03** | Path A vs Path B rollup trigger 选择矩阵：<br>• **Path A**（eager rollup）：`rollup_gate.go:26` `ShouldRollupAfterChildren(parent, policy, stats)` — 3 policies × 2 needs_rollup = 6 组合<br>• **Path B**（root fallback）：`rollup_gate.go:89` `MaybeRootRollupFallback(sessionID, tm)` — 2 has_ephemeral × 2 needs_rollup = 4 组合<br>• **合计**：6 + 4 = 10 组合 unit test 覆盖 | Path A (eager rollup) vs Path B (root fallback) 路由 | PENDING (S4) |

#### 2.4.5 ReevaluateParentAfterChild 函数签名变化（向后兼容）

**当前签名**：
```go
func ReevaluateParentAfterChild(sessionID, childID string, tm *TaskManager) (struct{}, error)
```

**新签名**：
```go
func ReevaluateParentAfterChild(sessionID, childID string, tm *TaskManager) (*RollupReport, error)
```

**3 调用点迁移**（全部丢弃返回值即可，0 业务代码改动）：
- `session_turn_loop.go:194`: `workmodel.ReevaluateParentAfterChild(...)` → 保持现状（Go 允许忽略返回值）
- `run_spawn.go:51`: 同上
- `cli_commands.go:329`: 同上

**关键约束**：
- **D7-S15 TurnState 接口不变**（DM-20260628-003 刚闭环）
- RollupReport **不参与** TurnState 序列化（仅 process 内传递）
- 0 业务逻辑改动（仅返回类型）

#### 2.4.6 WorkTree 治理验证

| 验证项 | 命令 | 通过条件 |
|---|---|---|
| RollupReport struct | `rg "type RollupReport" internal/layers/orchestration/workmodel/rollup_report.go` | =1 |
| 7 字段完整 | `rg "json:\"[a-z_]+\"" internal/layers/orchestration/workmodel/rollup_report.go \| wc -l` | ≥7（5 数据 + ChildID + GeneratedAt）|
| 构造器 | `rg "func NewRollupReportFromRound" internal/layers/orchestration/workmodel/rollup_report.go` | =1 |
| 5 处 child.LastRound 替换 | `rg "child\.LastRound\.(ContextBubbleKind\|ArtifactSummary\|SpawnPolicy)" internal/layers/orchestration/workmodel/` | = 0（除构造器内）|
| sessionRootGoal 排序 | `rg "sort\.Slice" internal/layers/orchestration/workmodel/rollup_gate.go` | ≥1 |
| 多 root unit test | `go test -race ./internal/layers/orchestration/workmodel/ -run TestSessionRootGoal` | PASS |
| 3 T 登记 | `rg "^D7-S0-A07-T0[1-3]" openspec/specs/d7-orchestration/t-registry.md \| wc -l` | = 3 |
| 22/22 -race | `go test -race ./internal/layers/orchestration/...` | PASS |

---

## §3 F 层设计（68 F → 66 F，路径修正 + dead code 删 + 跨包升格）

### 3.1 F 层现状（盘点结论）

| 问题 | 数量 | 严重性 |
|---|---|---|
| 路径错误（指向不存在位置） | 6 F | 高 |
| 命名漂移（orchestrate vs orchestratePath） | 2 F | 中 |
| Legacy 41 ghost count | 41 F | 中（仅 2 F 实际标 legacy）|
| 跨包 F 应升格 orchtypes/ | 3 F | 低 |
| 死代码 | ~126 LOC | 中 |

### 3.2 F 层关键设计决策

#### Decision F-1: 6 F 路径全部修正

| F ID | 当前（错） | 目标（对） |
|---|---|---|
| `D7-S1-A02-F01` | `contextengine/tasks/task_manager.go` | `workmodel/task_manager.go` |
| `D7-S1-A02-F02` | 同上 | 同上 |
| `D7-S1-A02-F03` | 同上 | 同上 |
| `D7-S1-A02-F04` | 同上 | 同上 |
| `D7-S1-A02-F05` | 同上 | 同上 |
| `D7-S1-A02-F06` | 同上 | 同上 |
| `D7-S2-A01-F03` | `orchestrate` | `orchestratePath` |
| `D7-S2-A01-F04` | `orchestrator.go + EventPublisher` | `turn_orchestrator.go` |
| `D7-S5-A01-F02` | `decisionplanning/classifier.go` | `classifier_fallback.go` |
| `D7-S5-A01-F03` | `classifier.go + classifier_fallback.go` | `classifier_fallback.go` |

#### Decision F-2: Legacy 41 ghost → "deprecated 2 + canonical 66 = 68"

**理由**：f-registry Statistics 写 "Legacy 41 + Canonical 27" 但**无 F 条目标注 Legacy 列**，只有 2 个 F 标 deprecated（`D7-S3-A03-F01/F02`）。修正为真实数。

#### Decision F-3: 3 跨包 F 候选升格 orchtypes/

| F ID | 当前 | 推荐升格 |
|---|---|---|
| `D7-S2-A01-F01 RouteByIntent` | `orchestrator.go` 内联 | `orchtypes/routing.go::RouteByIntent`（被 a-registry 提及但未列为 F）|
| `D7-S2-A01-F02 ExecuteFastPath` | `sessionorchestrator/fastpath.go` | `orchtypes/routing.go::FastPath` |
| `D7-S11-A40-F01 RunLearner` | `mups/learn/learner.go` | `orchtypes/learner_iface.go::Learner interface` |

**实施**：本次不动实现（避免 v6.0.0 已闭合迁移被打破）；仅在 f-registry 标注"跨包候选"。

### 3.3 F 层死代码清理清单（~126 LOC）

**详细清单见 `tasks.md` T01-T13。汇总**：

| 类别 | 文件 | LOC | 死符号 |
|---|---|---|---|
| **F 死符号** | `orchtypes/uncertainty_coord.go` | 4 | `ErrInvalidVerdictKind` |
| **F 死字段** | `orchtypes/config.go` | 13 | `LLMFallback` / `ShadowLLMClassify` / `ShadowLLMTimeoutMs` / `FastPathThreshold` / `RuleOrchestrateConfig` |
| **F 死枚举** | `orchtypes/routing.go` | 6 | `RoutingModeRuleOrchestrate` + 死 switch arm |
| **F 死别名** | `orchtypes/artifact_kind_alias.go` | 6 | 4 alias constants |
| **F 死函数** | `decisionplanning/prompts.go` | 26 | `TurnSystemPrompt` + `loopFirstSystemPrompt` |
| **F 死函数** | `delegatetools/subquery_fallback.go` | 4 | `BuildSubQueryRunner` |
| **F 死代码** | `escape/engine.go` | 1 | `var _ = time.Now` stub |
| **A 老链路** | `sessionorchestrator/turn_orchestrator.go:917-922` | 6 | legacy explicit-code path |
| **A 死函数** | `workmodel/aggregate_verdicts.go:42-97,179` | 60 | `AggregateVerdicts` + 4 serialization helpers |
| **总计** | - | **~126 LOC** | 12 项 |

### 3.4 F 层验证

| 验证项 | 命令 | 通过条件 |
|---|---|---|
| 6 F 路径修正 | `rg "contextengine/tasks/task_manager" openspec/` | =0 |
| F 命名修正 | `rg "orchestrator\.go.*EventPublisher.*orchestrate[^P]" openspec/` | =0 |
| Legacy ghost 修复 | `rg "Legacy 41" openspec/` | =0 |
| 死代码全删 | `rg "ErrInvalidVerdictKind\|LLMFallback\|ShadowLLMClassify\|TurnSystemPrompt\|BuildSubQueryRunner\|AggregateVerdicts\|RoutingModeRuleOrchestrate" internal/` | =0（除 test fixture）|
| 死代码 doc 修正 | `rg "v2.0 Slice plan\|FastPath calls TurnOrchestrator" internal/layers/orchestration/sessionorchestrator/turn_doc.go` | =0 |

---

## §4 S→A→F 拆分 vs 清理：决策表

| 类别 | 改 S 名？ | 拆 A？ | 修 F 路径？ | 删死代码？ | 升格 F？ |
|---|---|---|---|---|---|
| **#0 dead-code-cleanup** | - | - | - | ✅ 14 项 | - |
| **#1 turn-fn-split** | - | ✅ D7-S2-A06→A09 | - | ✅ legacy explicit-code | - |
| **#2 registry-sync** | - | - | ✅ 6 F | - | - |
| **#3 value-flow-rename** | ✅ 6 S Alias | - | - | - | - |
| **#4 t-span-coverage** | - | - | - | - | - |
| **#5 boundary-decision** | - | - | - | - | - |
| **#6 verify-archive** | - | - | - | - | - |

**关键设计原则**：
- **每个子 Change 都有清理动作**（非单独清理阶段）
- **S 层不删旧编号**（DSAFT 原则 3）
- **A 层只拆 god activity**（D7-S2-A06），不重组其他 48 A
- **F 层只修路径 + 删死代码**（不重新分类 68 F）

---

## §5 横切 1：Span 增补（5 ops + 4 acceptance）

### 5.1 5 个新 ops span 设计

#### Span #1: D7_Orchestration_LongTerm_Reputation_Update（S6 LP-1）

**触发点**：`mups/learn/reputation/store.go::BayesianUpdate`

**Key Attributes**：

| Attribute | 类型 | 说明 |
|---|---|---|
| `session.id` | string | session ID |
| `reputation.asset_class` | string | SOP / Protocol / Knowledge / Conclusion / Pending |
| `reputation.prior_alpha` | int | 上一轮 Alpha |
| `reputation.prior_beta` | int | 上一轮 Beta |
| `reputation.posterior_alpha` | int | 本轮 Alpha |
| `reputation.posterior_beta` | int | 本轮 Beta |
| `reputation.wilson_lower` | float | Wilson Lower Bound |
| `reputation.wilson_upper` | float | Wilson Upper Bound |
| `reputation.verifier_failure_count` | int | 累计失败次数 |
| `reputation.track_mode` | string | TrackMode enum |

**覆盖 T 点**：D7-S12-A42-T01..T04（LP-1 长期信誉 4 T）

#### Span #2: D7_Orchestration_Anomaly_Trigger（S4 SystemAnomaly）

**触发点**：`executionflow/verify/anomaly.go::SystemAnomalyDetector.Trigger`

**Key Attributes**：

| Attribute | 类型 | 说明 |
|---|---|---|
| `session.id` | string | session ID |
| `anomaly.kind` | string | AnomalyKind enum |
| `anomaly.severity` | string | low / medium / high / critical |
| `anomaly.threshold` | float | 触发阈值 |
| `anomaly.evidence_id` | string | 异常证据 ID |

**覆盖 T 点**：D7-S4-A08/A09（SystemAnomaly 8 T）

#### Span #3: D7_Orchestration_AdaptivePrior_Inject（跨 S5/S6）

**触发点**：`sessionorchestrator/observe.go::buildObserveRequest`

**Key Attributes**：

| Attribute | 类型 | 说明 |
|---|---|---|
| `session.id` | string | session ID |
| `prior.adaptive_kind` | string | developer / operator / uniform |
| `prior.beta_alpha` | int | 注入的 Alpha |
| `prior.beta_beta` | int | 注入的 Beta |
| `prior.source_learner` | string | learner 实例 ID |

**覆盖 T 点**：D7-S5-A15/A16 + D7-S6-A30/A31（跨 S 数据契约 6 T）

#### Span #4: D7_Orchestration_Resume_Decision_Path（S2 ResumeSession 3 决策）

**触发点**：`sessionorchestrator/orchestrator.go::ApplyResumeSession`

**Key Attributes**：

| Attribute | 类型 | 说明 |
|---|---|---|
| `session.id` | string | session ID |
| `resume.decision` | string | fall_through / user_accept / user_cancel |
| `resume.user_choice` | string | continue / abort / force_exit |
| `resume.circuit_level` | string | L0..L5 |
| `resume.exit_reason` | string | 14 ExitReason 中的 1 个 |

**覆盖 T 点**：D7-S14-A57/A58/A59（ResumeSession 3 决策 6 T）

#### Span #5: D7_Orchestration_Feishu_Card_Render（跨域 D7→D1）

**触发点**：`internal/layers/communication/feishu/progress.go::finalizeReplyCardStreaming`

**Key Attributes**：

| Attribute | 类型 | 说明 |
|---|---|---|
| `session.id` | string | session ID |
| `feishu.card_type` | string | text / tool_call / complete / error |
| `feishu.update_method` | string | streaming / final |
| `d7.last_verdict` | string | Pass / Partial / Fail / Indeterminate |
| `d7.last_exit_reason` | string | 14 ExitReason 中的 1 个 |

**覆盖 T 点**：D7→D1 决策可观测性（跨域 T 点）

### 5.2 4 个 acceptance test 设计

#### Test #1: `tests/integration/d7/d7_acceptance_lp1_test.go`

**测试目标**：LP-1 长期 Bayesian 信誉累积在生产可观测

**测试流程**：

```go
func TestAcceptanceLP1_LongTermReputation(t *testing.T) {
    // 1. 启动 D7 test stack
    stack := testutil.NewD7TestStack(t, testutil.D7StackOptions{...})
    
    // 2. 多轮对话（5 轮），每轮失败 → 累积信誉
    for i := 0; i < 5; i++ {
        req := orchtypes.ProcessRequest{
            SessionID: "sess_lp1_test",
            Message:   "failing message",
            UserID:    "user_1",
        }
        out, _ := stack.Gateway.ProcessMessage(ctx, req)
        // 等待 complete 事件
        ...
    }
    
    // 3. 验证 ReputationStore 状态
    evidence := stack.TaskManager.GetReputationEvidence("sess_lp1_test", asset_class_knowledge)
    require.Greater(t, evidence.Alpha, 0)
    require.Greater(t, evidence.VerifierFailureCount, 0)
    
    // 4. 验证 5 个 LongTerm_Reputation_Update span 被发
    traces := obsBridge.GetTracesByOp("D7_LongTerm_Reputation_Update")
    require.Equal(t, 5, len(traces))
}
```

**AC**：5 轮对话 + 5 个 span + 1 个 ReputationEvidence 状态。

#### Test #2: `tests/integration/d7/d7_acceptance_lp2_test.go`

**测试目标**：LP-2 Pending 隔离（不污染主知识库）

**测试流程**：

```go
func TestAcceptanceLP2_PendingIsolation(t *testing.T) {
    req := orchtypes.ProcessRequest{
        SessionID: "sess_lp2_test",
        Message:   "trigger parse failure",
        ...
    }
    out, _ := stack.Gateway.ProcessMessage(ctx, req)
    
    // 验证 PendingAsset 进 ScheduledMemory 而非 SkillMemory
    pendingAssets := stack.Memory.ScheduledMemory.List(ctx, "sess_lp2_test")
    require.NotEmpty(t, pendingAssets)
    require.Empty(t, stack.Memory.SkillMemory.List(ctx, "sess_lp2_test"))
    
    // 验证 Asset.SourceSessionIDs 链可追溯
    for _, asset := range pendingAssets {
        require.Equal(t, "sess_lp2_test", asset.SourceSessionIDs[0])
    }
}
```

**AC**：Pending 进 ScheduledMemory 而非 SkillMemory。

#### Test #3: `tests/integration/d7/d7_acceptance_lp5_test.go`

**测试目标**：LP-5 反向追溯链

**测试流程**：

```go
func TestAcceptanceLP5_ReverseTraceability(t *testing.T) {
    req := orchtypes.ProcessRequest{...}
    out, _ := stack.Gateway.ProcessMessage(ctx, req)
    
    // 验证 Asset.SourceSessionIDs → Plan.SourceObservationIDs → Observation 链闭合
    plan, _ := stack.TaskManager.GetLastPlan("sess_lp5_test")
    asset, _ := stack.Memory.GetAssetByPlanID(plan.ID)
    observation, _ := stack.TaskManager.GetObservationByID(plan.SourceObservationIDs[0])
    
    require.NotNil(t, plan)
    require.NotNil(t, asset)
    require.NotNil(t, observation)
    require.Equal(t, "sess_lp5_test", observation.SessionID)
}
```

**AC**：5 节点链路闭合。

#### Test #4: `tests/integration/d7/d7_acceptance_resume_test.go`

**测试目标**：ResumeSession 3 决策路径（A/B/C）

**测试流程**：

```go
func TestAcceptanceResume_3Decisions(t *testing.T) {
    testPath("sess_resume_a", resume.DecisionFallThrough)
    testPath("sess_resume_b", resume.DecisionUserAccept)
    testPath("sess_resume_c", resume.DecisionUserCancel)
}

func testPath(t *testing.T, sessionID string, decision resume.Decision) {
    // 强制 circuit breaker 触发
    // 调用 ProcessMessage → 触发 ApplyResumeSession
    // 验证 D7_Resume_Decision_Path span 被发
    traces := obsBridge.GetTracesByOpAndSession("D7_Resume_Decision_Path", sessionID)
    require.Equal(t, 1, len(traces))
    require.Equal(t, string(decision), traces[0].Attributes["resume.decision"])
}
```

**AC**：3 路径各 1 个 span。

### 5.3 Span 验证

| 验证项 | 命令 | 通过条件 |
|---|---|---|
| 5 Op 常量注册 | `rg "OpD7_Orchestration_(LongTerm\|Anomaly\|AdaptivePrior\|Resume\|Feishu)" internal/layers/observability/instrument/telemetry/names.go` | =5 |
| 5 SpanMeta | `rg "SinceVersion.*2\.6\.0.*Instrumented.*true" internal/layers/orchestration/sessionorchestrator/spans.go` | ≥5 |
| coverage registry 期望 | `rg "D7_LongTerm_Reputation_Update\|D7_Anomaly_Trigger\|D7_AdaptivePrior_Inject\|D7_Resume_Decision_Path\|D7_Feishu_Card_Render" internal/layers/observability/diagnose/coverage/registry_test.go` | =5 |
| 4 acceptance test PASS | `go test -tags=acceptance ./tests/integration/d7/` | PASS |
| T↔Span 覆盖率 | t-registry.md Span Evidence 列 | ≥80% |

---

## §6 横切 2：Boundary Governance（3 项 Decision）

### 6.1 3 项 boundary debt Decision 表

#### Decision 1: ReputationEvidence / BayesianPrior

| 维度 | 当前 | 推荐 |
|---|---|---|
| 归属 D | D7 | 保留 D7（advisory 与 D6 并行） |
| 跨域性质 | 信誉累积本质是"跨域质量信号" | boundary-borrowed |
| 当前状态 | 已 S7_Archived (PR #235/#236) | stable |
| 推荐 | 保留 | 不下沉，避免推翻 v6.0.0 共识 |
| Future | re-evaluate at v7.0 | 与 D6 重新协商归属 |

#### Decision 2: SystemAnomaly / 异常检测

| 维度 | 当前 | 推荐 |
|---|---|---|
| 归属 D | D7-S4 | 保留 D7 |
| 跨域性质 | 系统异常本质是 observability infra | boundary-borrowed |
| 当前状态 | 已 S7_Archived (v6.0.0) | stable |
| 推荐 | 保留 | D5 暂不接管异常检测，由 D7 在 Verify 节点同步 |
| Future | re-evaluate at v7.0 | 与 D5 协商异常信号共享 |

#### Decision 3: AdaptivePrior / 跨 S5/S6 数据契约

| 维度 | 当前 | 推荐 |
|---|---|---|
| 归属 S | D7-S5 + D7-S6 | 保留 |
| 跨 S 性质 | 同一类型被两个 S 层使用 | boundary-borrowed |
| 当前状态 | 已 S7_Archived (Phase 6) | stable |
| 推荐 | 保留 | 跨 S 契约由 S5/S6 共享，contract 文件放 orchtypes/ |
| Future | re-evaluate at v7.0 | 视 MUPS 演进决定是否拆分 |

### 6.2 orchtypes/boundary_decision.go governance 文件

```go
// Package orchtypes provides shared types for D7 orchestration.
//
// boundary_decision.go (DM-20260629-001) — DSAFT 原则 4 governance:
// 显式标注 D7 域内"boundary-borrowed"能力的归属决策，避免跨域漂移。
//
// 3 项 boundary debt 在 v2.6.0 (PR #9 of devrix-d7-dsaft-restructuring)
// 显式登记，由 v7.0 重新评估。
package orchtypes

const (
    // BoundaryDebt: ReputationEvidence 信誉累积跨域质量信号
    // 当前归属: D7-S6 MUPS Pipeline
    // 越域性质: 信誉累积本质是"跨域质量"信号，D6 advisory 校验更合理
    // 决策: 保留当前归属（D7-S6），避免推翻 v6.0.0 S7_Archived 共识
    // Future: re-evaluate at v7.0 (与 D6 重新协商)
    BoundaryReputationEvidence = "boundary-debt:reputation-evidence-v7.0"

    // BoundaryDebt: SystemAnomaly 系统异常 observability infra
    // 当前归属: D7-S4 ExecutionFlow + Verify
    // 越域性质: 系统异常本质是 observability infra，D5 暂未接管
    // 决策: 保留当前归属（D7-S4），D5 暂不接管异常检测
    // Future: re-evaluate at v7.0 (与 D5 协商异常信号共享)
    BoundarySystemAnomaly = "boundary-debt:system-anomaly-v7.0"

    // BoundaryDebt: AdaptivePrior 跨 S5/S6 数据契约
    // 当前归属: D7-S5 Observe 注入 + D7-S6 Learn 更新
    // 越域性质: 同一类型被两个 S 层使用，跨 S 契约
    // 决策: 保留当前归属（跨 S 共享），contract 文件放 orchtypes/
    // Future: re-evaluate at v7.0 (视 MUPS 演进决定是否拆分)
    BoundaryAdaptivePrior = "boundary-debt:adaptive-prior-v7.0"
)
```

### 6.3 orchtypes/boundary_decision_test.go governance 测试

```go
package orchtypes

import (
    "testing"
    "github.com/stretchr/testify/require"
)

func TestBoundaryDecisionConstants(t *testing.T) {
    require.Equal(t, "boundary-debt:reputation-evidence-v7.0", BoundaryReputationEvidence)
    require.Equal(t, "boundary-debt:system-anomaly-v7.0", BoundarySystemAnomaly)
    require.Equal(t, "boundary-debt:adaptive-prior-v7.0", BoundaryAdaptivePrior)
}

func TestBoundaryDecisionVersioning(t *testing.T) {
    debts := []string{
        BoundaryReputationEvidence,
        BoundarySystemAnomaly,
        BoundaryAdaptivePrior,
    }
    for _, debt := range debts {
        require.Regexp(t, `^boundary-debt:[a-z-]+-v\d+\.\d+$`, debt)
    }
}
```

### 6.4 Boundary 验证

| 验证项 | 命令 | 通过条件 |
|---|---|---|
| 3 boundary debt 常量 | `rg "Boundary.*boundary-debt" internal/layers/orchestration/orchtypes/boundary_decision.go` | =3 |
| Boundary governance test | `go test ./internal/layers/orchestration/orchtypes/` | PASS |
| Decision 表 3 项 | `rg "Decision [123]:" openspec/specs/d7-orchestration/design.md` | ≥3 |

---

## §7 与现有 Change 的关系

### 7.1 前置 Change（已 S7_Archived）

| Change | DM ID | 关系 |
|---|---|---|
| `devrix-d7-six-s-simplification` | DM-20260626-001 | v6.0.0 6 S + 1 横切（本次 ValueFlow Alias 在 v6.0.0 之上叠加） |
| `devrix-d7-mups-v4-5node-coverage-orchestration` | DM-20260625-019 | 5-node Span + 目录结构治理（本次 5 ops span 增量） |
| `devrix-d7-multiturn-session-state` | DM-20260628-003 | turn 串行化（D7-S15，本次拆分必须保持 TurnState 接口不变） |

### 7.2 后置 Change（v7.0 候选）

| 候选 Change | 内容 |
|---|---|
| `devrix-d7-v7-boundary-migration` | 3 项 boundary debt 重新评估 |
| `devrix-d7-v7-t-span-coverage-final` | 剩余 ~46 T 缺口 Tracker 闭合 |
| `devrix-d7-v7-cross-pkg-promotion` | 3 个跨包 F 升格 orchtypes/ |

---

## §8 相关文档

- `demand.md` — 6 类架构债 + 范围 + 风险
- `proposal.md` — 6 子 Change + 5 Decision
- `tasks.md` — 51 任务清单
- `specs/d7-orchestration/spec.md` — ~54 AC 全表
- `docs/methodology/dsaft-methodology.md` v4.0.0
- `docs/methodology/dsaft-refactoring-playbook.md` v1.0.0
- `openspec/specs/d7-orchestration/d7-domain.md` v2.5.1 → v2.6.0