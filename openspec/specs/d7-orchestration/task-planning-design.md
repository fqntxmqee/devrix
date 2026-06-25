# 任务规划系统设计

**文档类型:** 详细架构设计
**Domain:** D7-S1（`workmodel/`）；PlanAgent 编排面 D7-S5（`decisionplanning/` 消费）；**MUPS 5 节点管道** D7-S8 Observe / D7-S8 PR-B1 Plan / D7-S9 Execute / D7-S10 Verify / D7-S11 Learn
**Change ID:** devrix-task-planning
**版本:** 2.3.0
**状态:** Active — IMPLEMENTED (PlanMode + WorkItem + MUPS v4.3)
**Last Updated:** 2026-06-25
**对标:** Claude Code Plan Mode
**关联:** `openspec/specs/d7-orchestration/spec.md`, `design.md`, `demand.md` (DM-20260613-001)

---

## 实现状态（2026-06-25）

| 组件 | 状态 | 代码位置 |
|------|------|----------|
| WorkItemManager（v4.3 起,TaskManager 退化为 Tree() facade） | ✅ | `orchestration/workmodel/work_tree.go` + `task_manager.go` |
| DiskWorkItemStore (schema v2) | ✅ | `orchestration/workmodel/workitem_store.go` |
| PlanMode 状态机 | ✅ | `orchestration/workmodel/plan_mode.go` |
| PlanAgent 只读探索 | ✅ | `orchestration/workmodel/plan_agent.go` |
| VerificationAgent | ✅ | `orchestration/verify/verifier.go`（Phase 4 升格自 D7-S1-A06 ExecutePlanAgent）|
| CLI `/worktree` `/plan` | ✅ | `orchestration/workmodel/cli_commands.go` + `sessionorchestrator/command_handler.go` |
| D7-S5 ClassifyIntent | ✅ | `orchestration/decisionplanning/classifier.go` |
| D7-S1 CreateWorkPlan | ✅ | `sessionorchestrator/workmodel.go` + `workmodel/plan_mode.go` |
| **MUPS 5 节点管道（v4.3）** | ✅ | `orchestration/{observe,execute,verify,learn}/` + `shared/types/` |
| **D7-S12 LP-1 闭环集成** | ✅ | `sessionorchestrator/observe_request.go::buildObserveRequest`（3 层 fail-safe）|
| **D7-S13 Verify Auto-Close** | ✅ | `orchestration/verify/auto_close.go::processAutoClose` |
| **D7-S14 EscapeEngine** | ✅ | `orchestration/escape/{engine,circuit_breaker}.go` |

> **v4.3 迁移说明：** WorkItem 是唯一 canonical 写模型（v4.3 起 Task flat-view + TaskStore + TaskManager conversion helpers 全删）。`contextengine/tasks/` 仅为历史 shim。MUPS 5 节点管道（Observe → Plan → Execute → Verify → Learn）以 D7-S8/S9/S10/S11 升格为顶层场景，Plan 是 5 节点管道的第 2 节点。

---

## ① 设计目标

### 与 Claude Code 对齐

| Claude Code | Devrix | 状态 |
|-------------|--------|------|
| `/plan` 命令 | `/plan` 命令 | ✅ |
| EnterPlanMode Tool | PlanMode.Enter | ✅ |
| ExitPlanMode Tool | PlanMode.Reject / 完成回 inactive | ✅ |
| PlanAgent (只读探索) | PlanAgent | ✅ |
| VerificationAgent | VerificationAgent | 🔶 |
| 任务列表 | TaskManager | ✅ |

### 触发方式

| 方式 | Claude Code | Devrix | 状态 |
|------|-------------|--------|------|
| 显式命令 | `/plan` | `/plan` | ✅ |
| 自动检测 | 无 | 可选 `auto_detect` | ⬜ 默认关闭 |

### 业务目标

- 将"规划"与"执行"阶段显式分离，避免 QueryLoop 内隐式规划
- Task 作为可追踪工作单元，支持依赖 DAG 与持久化
- PlanAgent 只读探索，审批后才创建 Task 并执行

---

## ② 核心流程

### Plan Mode 工作流（CLI 入口，PlanAgent only）

```
用户输入 /plan <goal>
    ↓
┌─────────────────────────────────────────────────────────────┐
│                  Plan Mode 激活                             │
│  状态: inactive → active → pending_approval → inactive     │
└─────────────────────────────────────────────────────────────┘
    ↓
PlanAgent 执行（只读模式）
    ↓
探索代码库 + 设计实现方案
    ↓
生成 PlanResult（含任务列表草案）
    ↓
用户审批 (/plan approve / /plan reject)
    ↓
approve → WorkItemManager 批量创建 WorkItem
reject  → 回到 inactive
```

### MUPS 5 节点管道工作流（自动触发，Observe/Plan/Execute/Verify/Learn）

> 与 Plan Mode 的区别：**MUPS 5 节点管道是自动触发的端到端编排主线**，Plan 是其中第 2 节点（不经过 `/plan` CLI 入口）。PlanAgent 只服务于 `/plan` CLI 命令，MUPS Plan 节点走更轻量的 `Planner` 接口。

```
用户消息（无 /plan 前缀）
    ↓
D7-S8 Observe 节点 → UncertaintyReport{Observations, UncertaintyCoord, QuantizedIntent}
    ↓
D7-S8 PR-B1 Plan 节点 → Plan{ID, Kind, Strength, Steps, FailureCriteria, BlastRadius, SourceObservationIDs}
    ↓
D7-S9 Execute 节点 → Artifact{ID, Kind, Payload, Evidence, SourcePlanID}
    ↓
D7-S10 Verify 节点 → Verdict{Kind, Evidence, Reason, SourceArtifactID}
    ↓
D7-S11 Learn 节点 → LearningAsset + ReputationEvidence
    ↓
(下轮) D7-S8 Observe ← ReputationEvidence 注入 AdaptivePrior（D7-S12 跨域闭环）
```

### 状态机

```
┌──────────────┐
│  Inactive    │ ◀─────────────────────────┐
└──────┬───────┘                          │
       │ /plan <goal>                      │
       ▼                                   │
┌──────────────┐                          │
│   Active     │ ── Plan 生成完成 ────────▶│
└──────┬───────┘                          │
       │ 用户审批                           │
       ▼                                   │
┌──────────────┐                          │
│PendingApproval│                          │
└──────┬───────┘                          │
       │ approve/reject                    │
       ▼                                   │
    (返回 Inactive) ───────────────────────┘
```

### Task 与 ExecutionFlow 联动

当 `execution_flow.link_tasks=true` 时：

```
FlowStarted(task_id) → TaskManager.SetOwner + status=in_progress
FlowCompleted       → TaskManager.status=completed
FlowFailed          → TaskManager.status=failed
```

实现：`orchestration/executionflow/hub/hub.go` linkTask()

---

## ③ CLI 命令

### WorkItem 命令 (`/worktree`，v4.3 起取代旧 `/task`)

```bash
/worktree create <subject> [description]  # 创建 WorkItem
/worktree list                            # 列出所有 WorkItem
/worktree get <item_id>                   # 获取 WorkItem 详情
/worktree update <item_id> [status]      # 更新状态
/worktree delete <item_id>               # 删除 WorkItem
/worktree ready                          # 显示就绪 WorkItem
/worktree dep <item_id> <blocked_by>     # 添加依赖
```

**实现：** `orchestration/workmodel/cli_commands.go` + `sessionorchestrator/command_handler.go`

### 规划命令 (`/plan`)

```bash
/plan <goal>   # 进入规划模式，指定目标
/plan enter    # 进入规划模式
/plan approve  # 审批计划，开始执行
/plan reject   # 拒绝计划
/plan status   # 显示当前状态
/plan show     # 显示当前计划
```

**实现：** `orchestration/workmodel/cli_commands.go` + `sessionorchestrator/command_handler.go` + `plan_mode.go`

---

## ④ PlanAgent 设计

### 角色定义

```
角色：软件架构师 + 规划专家
模式：只读（STRICTLY PROHIBITED）
- 不能创建文件
- 不能修改文件
- 不能删除文件
- 不能运行会修改状态的命令
```

### 实现要点

- **包路径：** `contextengine/tasks/plan_agent.go`
- **可观测性：** span `task.plan.generate`（D5 Operation Registry）
- **输出：** `PlanResult` 含 ExplorationFindings、Tasks 列表、CriticalFiles

### 提示词约束

PlanAgent system prompt 强制只读模式，允许 read/grep/ls/git status 等只读操作。

---

## ⑤ VerificationAgent 设计

### 角色定义

```
角色：验证专家（破坏性测试）
目标：尝试 BREAK 实现，不只是确认它工作
```

### 两大失败模式

| 模式 | 对抗策略 |
|------|----------|
| 验证规避 | 每个检查必须有 Command run |
| 80% 陷阱 | 必须尝试对抗性探测 |

> **状态：** 设计完成，集成测试覆盖待补全（D7-S5-T02 PLANNED）。

---

## ⑥ 配置

### YAML 配置（现行）

```yaml
context_engine:
  tasks:
    mode: v2                  # v1=todo, v2=task（带 DiskStore）
    store_dir: "~/.devrix/tasks/"
  plan:
    enabled: false            # 默认关闭，需显式启用
    auto_detect: false        # 默认关闭
    min_chars_for_plan: 200
    model: "deepseek-v4"
    max_milestones: 10
    timeout: 15s
    on_milestone_fail: "fail_fast"
  execution_flow:
    enabled: false
    link_tasks: true          # Task-Flow 状态联动
```

### 默认配置

```go
// contextengine/tasks — plan 默认关闭
func DefaultPlanConfig() PlanConfig {
    return PlanConfig{
        Enabled:         false,
        AutoDetect:      false,
        MinCharsForPlan: 200,
    }
}
```

---

## ⑦ 文件清单（现行）

```
internal/layers/contextengine/tasks/
├── task_manager.go          # D7-S1-A02 ManageTask
├── task_manager_test.go     # D7-S1-T01/T02/T04
├── disk_store.go            # D7-S1-A02-F05 持久化
├── disk_store_test.go       # D7-S1-T03
├── plan_mode.go             # D7-S1-A04/A05 PlanMode 状态机
├── plan_agent.go            # D7-S5-A04 RunPlanAgent
├── verification_agent.go      # VerificationAgent（若存在）
├── tool_suite.go            # WorkItem 工具注册
└── cli_commands.go          # /worktree /plan CLI（v4.3 起 /task 已并入 /worktree）

internal/layers/orchestration/executionflow/hub/
└── hub.go                   # linkTask → TaskManager 联动

internal/layers/orchestration/sessionorchestrator/
└── command_handler.go       # /task /plan CLI dispatch

internal/shared/config/
├── queryloop.go             # TasksConfig
└── execution_flow.go        # ExecutionFlowConfig
```

### v2.0 结构（DM-20260619-005，已落地）

```
internal/layers/orchestration/sessionorchestrator/
└── workmodel.go             # CreateWorkPlan facade

internal/layers/orchestration/workmodel/
├── task_manager.go          # Task CRUD + DAG
├── plan_mode.go             # PlanMode 状态机
└── plan_agent.go            # PlanAgent 只读探索
```

### 目标迁移（D7 v1.1，历史）

---

## ⑧ DSAFT 映射

| DSAFT ID | 组件 | 状态 |
|----------|------|------|
| D7-S1-A02 | ManageTask | ✅ |
| D7-S1-A04 | EnterPlanMode | ✅ |
| D7-S1-A05 | ApprovePlan | ✅ |
| D7-S5-A04 | RunPlanAgent | ✅ |
| D7-S1-A01 | CreateWorkPlan | ⬜ |
| D7-S5-A01 | ClassifyIntent | ⬜ |

---

## ⑨ 验收测试

```bash
# 单元测试
go test ./internal/layers/contextengine/tasks/...
go test ./internal/layers/orchestration/workmodel/...
go test ./internal/layers/orchestration/sessionorchestrator/...
go test ./internal/layers/orchestration/executionflow/...

# CLI 手动验收
# 启动 devrix 后：
> /plan Add user authentication
> /plan show
> /plan approve
> /task list
```

关联 T 测试点：`t-registry.md` D7-S1-T* / D7-S5-T*

---

**维护：** 功能变更需同步更新 `spec.md`、`a-registry.md`、`f-registry.md`、`t-registry.md` 与本文档。

---

## ⑩ MUPS Plan 节点（Phase 2 PR-B1, 2026-06-23 落地）

> MUPS 5 节点管道的 **第 2 节点**，与 `/plan` CLI 入口的 PlanAgent 是**两个不同的 Plan 实现**：CLI 入口的 PlanAgent 是只读探索 + 用户审批的人工流程；MUPS Plan 节点是自动触发的算法流程（Planner + Plan.Validate + MatchKind）。

### 1. 节点定位

```
Observe (S8) ── UncertaintyReport ──▶ Plan (S8 PR-B1) ── Plan ──▶ Execute (S9)
   ↑                                          │
   └── ReputationEvidence (D7-S12 LP-1) ─────┘
```

- **上游契约（D7-S8 Observe → D7-S8 Plan）：** `UncertaintyReport{Observations, UncertaintyCoord, QuantizedIntent, Anomalies}` 是 Plan 节点的输入。
- **下游契约（D7-S8 Plan → D7-S9 Execute）：** `Plan{ID, Kind, Strength, Steps, FailureCriteria, BlastRadius, SourceObservationIDs}` 是 Execute 节点的输入。

### 2. 4 类 Plan

| PlanKind | 触发条件（ObsKind） | 强度范围 | 配套 FailureCriteria | 爆炸半径 |
|----------|--------------------|---------|---------------------|---------|
| **CommitmentPlan** | ObsFact strength ≥ ★★★ | 严格（4 步硬约束）| Artifact.Hash 必须匹配预期 | 小（无副作用）|
| **ProtocolPlan** | ObsSignal strength ≥ ★★ | 中（3 步软约束）| Tool 调用序列必须符合 protocol | 中（依赖 Tool 副作用）|
| **ScenarioPlan** | ObsAnomaly strength ≥ ★ | 弱（2 步弹性约束）| 必须触发兜底分支 | 大（可能产生外部副作用）|
| **ExplorationPlan** | ObsUser strength < ★ | 探索（无约束）| N/A（只读探索）| 零（只读）|

### 3. 3 项强制约束

| 约束 | 含义 | 代码实现 |
|------|------|---------|
| **强度匹配** | Plan.Strength ≤ min(Observations.Strength) | `Plan.Validate.PP-1` |
| **可证伪性** | Plan.FailureCriteria 至少含 1 条可观测判定 | `Plan.Validate.PP-2` |
| **爆炸半径** | Plan.BlastRadius 与 Plan.Kind 配对 | `Plan.Validate.PP-3` |

### 4. Kind 匹配规则（MatchKind）

```go
// MatchKind: 从 Observation 推导 Plan.Kind
func MatchKind(obs Observation) PlanKind {
    switch obs.Kind {
    case ObsFact:    return CommitmentPlan
    case ObsSignal:  return ProtocolPlan
    case ObsAnomaly: return ScenarioPlan
    case ObsUser:    return ExplorationPlan
    }
}
```

### 5. 代码入口

- **Planner interface：** `orchestration/observe/plan/planner.go::Plan`
- **Plan.Validate：** `orchestration/observe/plan/plan.go::Validate`（PP-1/PP-2/PP-3 三项约束）
- **MatchKind：** `orchestration/observe/plan/kind.go::MatchKind`（4 条规则）
- **BlastRadiusCalculator：** `orchestration/observe/plan/blast_radius.go::Calculate`

### 6. T 点绑定（Phase 2 PR-B1）

- D7-S8-A22-T01：4 类 Plan 落地 + Validate 触发
- D7-S8-A22-T02：MatchKind 4 条规则覆盖率 100%
- D7-S8-A22-T03：PP-1/PP-2/PP-3 三项约束检测

---

## 关联文档

- `d7-domain.md`：MUPS 5 节点管道 SoT
- `a-registry.md` §D7-S8：MUPS Plan 节点 A 活动登记
- `f-registry.md` §D7-S8-A15..A22：MUPS Plan 节点 F 层登记
- `span-registry.md` §Operations：MUPS 5 节点 span 登记
- `terminal-state-guide.md` §3：D7-S8 D7-S11 节点博弈角色与职责
- `../d7-orchestration/observability-guide.md` §1：D7-S8-A22-T01..T03 T 层绑定
