# 任务规划系统设计

**文档类型:** 详细架构设计
**Domain:** D7-S1（`workmodel/`）；PlanAgent 编排面 D7-S5（`decisionplanning/` 消费）
**Change ID:** devrix-task-planning
**版本:** 2.2.0
**状态:** Active — IMPLEMENTED (PlanMode + TaskManager in D7 workmodel)
**Last Updated:** 2026-06-19
**对标:** Claude Code Plan Mode
**关联:** `openspec/specs/d7-orchestration/spec.md`, `design.md`, `demand.md` (DM-20260613-001)

---

## 实现状态（2026-06-19）

| 组件 | 状态 | 代码位置 |
|------|------|----------|
| TaskManager | ✅ | `orchestration/workmodel/task_manager.go` |
| DiskStore (v2) | ✅ | `orchestration/workmodel/task_store.go` |
| PlanMode 状态机 | ✅ | `orchestration/workmodel/plan_mode.go` |
| PlanAgent 只读探索 | ✅ | `orchestration/workmodel/plan_agent.go` |
| VerificationAgent | 🔶 设计完成 | 待独立 change |
| CLI `/task` `/plan` | ✅ | `orchestration/workmodel/cli_commands.go` + `sessionorchestrator/command_handler.go` |
| D7-S5 ClassifyIntent | ✅ | `orchestration/decisionplanning/classifier.go` |
| D7-S1 CreateWorkPlan | ✅ | `sessionorchestrator/workmodel.go` + `workmodel/plan_mode.go` |

> **迁移说明：** Task/Plan 写模型已完全迁入 `internal/layers/orchestration/workmodel/`（DM-012 + DM-20260619-005）。`contextengine/tasks/` 仅为历史 shim。

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

### Plan Mode 工作流

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
approve → TaskManager 批量创建 Task
reject  → 回到 inactive
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

### 任务命令 (`/task`)

```bash
/task create <subject> [description]  # 创建任务
/task list                            # 列出所有任务
/task get <task_id>                  # 获取任务详情
/task update <task_id> [status]      # 更新状态
/task delete <task_id>               # 删除任务
/task ready                          # 显示就绪任务
/task dep <task_id> <blocked_by>   # 添加依赖
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
├── tool_suite.go            # Task/Plan 工具注册
└── cli_commands.go          # /task /plan CLI

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
