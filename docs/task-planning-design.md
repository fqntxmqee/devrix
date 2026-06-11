# 任务规划系统设计

**文档类型:** 详细架构设计
**Change ID:** devrix-task-planning
**版本:** 2.0.0
**状态:** Ready
**对标:** Claude Code Plan Mode

---

## ① 设计目标

### 与 Claude Code 对齐

| Claude Code | Devrix |
|-------------|--------|
| `/plan` 命令 | `/plan` 命令 |
| EnterPlanMode Tool | EnterPlanMode (规划中) |
| ExitPlanMode Tool | ExitPlanMode (规划中) |
| PlanAgent (只读探索) | PlanAgent |
| VerificationAgent | VerificationAgent |
| 任务列表 | TaskManager |

### 触发方式

| 方式 | Claude Code | Devrix |
|------|-------------|--------|
| 显式命令 | `/plan` | `/plan` |
| 自动检测 | 无 | 可选 `auto_detect` |

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
生成任务列表
    ↓
用户审批 (/plan approve / /plan reject)
    ↓
执行 → Verify → VerificationAgent
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

### 规划命令 (`/plan`)

```bash
/plan <goal>   # 进入规划模式，指定目标
/plan enter    # 进入规划模式
/plan approve  # 审批计划，开始执行
/plan reject   # 拒绝计划
/plan status   # 显示当前状态
/plan show     # 显示当前计划
```

### 使用示例

```bash
# 1. 进入规划模式
> /plan Add user authentication
Entered plan mode.
Goal: Add user authentication

# 2. 查看计划
> /plan show
# Implementation Plan

## Exploration Findings
- Found existing auth module at auth/
- Uses JWT for token management
- No existing user model

## Tasks
1. Create User model
   - Add username, email, password_hash fields
2. Add JWT generation
   - Use existing jwt-go package
3. Create login endpoint
   - POST /auth/login

## Critical Files
- auth/handler.go
- models/user.go
---
Use `/plan approve` to proceed or `/plan reject` to cancel.

# 3. 审批执行
> /plan approve
Plan approved. Creating tasks:
✓ task_abc123: Create User model
✓ task_def456: Add JWT generation
✓ task_ghi789: Create login endpoint
```

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

### 提示词模板

```markdown
=== CRITICAL: READ-ONLY MODE ===
You are STRICTLY PROHIBITED from:
- Creating new files
- Modifying existing files
- Deleting files
- Running commands that change system state

You CAN:
- Read files
- Search using grep/find
- Run read-only commands (ls, git status, git log)

## Your Process

1. **Understand Requirements**: Focus on the user's goal
2. **Explore Codebase**: Find patterns, understand architecture
3. **Design Solution**: Consider trade-offs
4. **Detail the Plan**: Break down into tasks
```

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

### 验证探针

```markdown
## Required Adversarial Probes

1. **Boundary values**: 0, -1, empty, very long strings
2. **Idempotency**: same request twice
3. **Orphan operations**: delete/reference non-existent IDs
```

---

## ⑥ 配置

### YAML 配置

```yaml
context_engine:
  plan:
    enabled: true           # 启用规划功能
    auto_detect: false     # 默认关闭，需要显式 /plan
    min_chars_for_plan: 200
    model: "deepseek-v4"
    max_milestones: 10
    timeout: 15s
    on_milestone_fail: "fail_fast"
```

### 默认配置

```go
func DefaultPlanConfig() PlanConfig {
    return PlanConfig{
        Enabled:         false,  // 显式启用
        AutoDetect:      false,  // 默认关闭
        MinCharsForPlan: 200,
        // ...
    }
}
```

---

## ⑦ 文件清单

```
internal/layers/contextengine/tasks/
├── task_manager.go          # 任务管理器
├── task_manager_test.go   # 单元测试
├── plan_agent.go          # PlanAgent
├── verification_agent.go   # VerificationAgent
├── plan_mode.go           # PlanMode 状态机
├── tool_suite.go          # 工具套件
└── cli_commands.go        # CLI 命令处理

internal/shared/types/
└── command.go             # CommandType 更新

internal/layers/communication/adapters/
└── cli.go                # CLI 适配器（集成）
```

---

## ⑧ 验收测试

```bash
# 构建
go build ./...

# CLI 测试
# 启动 devrix 后：
> /plan Add user authentication
> /plan show
> /plan approve
> /task list
```

---

**维护：** 功能变更需同步更新本文档和 `openspec/` 相关规格。
