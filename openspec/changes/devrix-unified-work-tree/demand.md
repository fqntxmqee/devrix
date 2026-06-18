---
demand-id: DM-20260617-009
title: Unified Work Tree — 工作单元树 + 递归求解终态
source: 编排域任务系统深度设计讨论（clawcode QueryLoop 对齐 + 多套 task 简化）
priority: P1
status: S2_Clarified
dsaft_domain: orchestration
created: 2026-06-17
---

# Unified Work Tree — 工作单元树 + 递归求解终态

## 1. 背景

Devrix 编排域存在 **多套「任务」语义混用**，根因是把三种不同概念都叫 task：

| 概念 | 含义 | 现状模块 |
|------|------|----------|
| **工作单元 (What)** | 要解决什么问题、父子关系、状态 | `workmodel.TaskManager`（flat map） |
| **执行句柄 (How/Run)** | 哪个 goroutine/process 在跑、output 在哪 | `BackgroundRegistry` + Wave worker handle |
| **编排图节点 (When)** | Wave 调度槽、依赖、并行度 | `wave.TaskNode` DAG |

此外还有 **`todo_write`**（D2 session scratch checklist）与 **`task_*`**（持久任务板）语义重叠。

OpenSpec 已有 **DM-011 Unified Task Registry** 明确：TaskManager（Plan DAG）与 TaskRegistry（运行时句柄）**应分离**——但只规划了 How 层统一，未统一 What 层工作语义。

clawcode 对齐目标：

> **任务 = 处理不确定性的递归分解单元；RunTurn 是递归引擎；todo 是其中一种轻量表达。**

Devrix canonical 主路径已是 **D7 RunTurn**（`loop_first=true`，DM-20260617-001），终态递归引擎应为 **TurnOrchestrator + WorkTree**，而非再养 D2 QueryLoop 旁路。

## 2. 问题陈述

| 场景 | 现状 | 应有行为 |
|------|------|----------|
| LLM 创建子任务 | `task_create` flat；delegate 可选 task_id | 统一挂 **WorkItem 树**，有 parent_id + kind |
| turn 内 checklist | `todo_write` 写 `sc.Todos` | checklist 是 **父 WorkItem 下 ephemeral 子节点** |
| Wave 并行 | 独立 `TaskNode` DAG，与 task board 无关 | Wave 读 WorkTree ready 子项，**不自有任务类型** |
| 查看 running 输出 | BackgroundRegistry / Wave slog 分散 | RunRegistry（DM-011）+ `WorkItem.RunRef` |
| 子任务完成通知 | notify.Bus 仅覆盖 workmodel 终态 | 所有 spawn terminal → 父节点 bubble → RunTurn 下一 turn |
| 用户/LLM 认知 | 区分 task_id / wave node / bg task_id | 对外 **WorkItem ID** 唯一；Run ID 仅观测层 |

## 3. 澄清记录

### Q1: WorkTree 与 RunRegistry（DM-011）的关系？

**A**: **分离、挂接** — WorkTree 管 What（树、状态、依赖、持久化）；RunRegistry 管 How（running/completed、disk output、cancel）。`WorkItem.RunRef` 指向 RunRegistry 条目；RunRegistry terminal 回调更新 WorkItem.Status 并通知父节点。**不合并数据结构**。

### Q2: todo_write 终态是删还是保留 tool 名？

**A**: **保留 tool 名、改底层** — v1.1 起 `todo_write` 批量 upsert 当前 focus 下的 `kind=checklist` 子节点；v2.0 可选 alias 到 `task_write(mode=checklist)`。默认 **ephemeral=true 不落盘**；plan approve 后可 promote 为 `kind=implement` 持久子项。

### Q3: Wave 是否保留为 LLM 直接调用的 tool？

**A**: **过渡期保留 `delegate_wave` alias**；终态 orchestrator 在「高不确定性 + 多 implement 子项」时自动 batch，LLM 不必须直接调 wave tool。WaveScheduler 降为 **纯基础设施**：读 WorkTree 中 `policy=parallel_ok` 的 ready 子项填 worker slot。

### Q4: flat `task_create` 挂哪？

**A**: 默认挂 **session root goal** 下（`kind=implement`，`parent_id=goal.ID`）。无 goal 时 `EnsureGoal` 先创建。

### Q5: WorkItem ID 与 Run ID？

**A**: **分离** — `wi_*`（工作语义）与 `run_*`（执行句柄）。LLM 面向 WorkItem ID；`task_await` / `task_output` 内部通过 RunRef 解析。

### Q6: 与 agent_tools / multi_agent 配置边界？

**A**: 不变 — **系统 tools 零配置**（task 面 always on）；`multi_agent` 控制 spawn 类（delegate/free_fork）是否暴露；`agent_tools` 是 `task_spawn(kind=agent, executor=claude|cursor)` 的执行器，不是第四套任务系统。

## 4. 终态架构（L1–L5 映射）

```text
D7 SessionOrchestrator
  └─ RunTurn（递归引擎）
       ├─ focus := WorkTree.GetFocus(session)
       ├─ uncertainty > threshold → decompose → 子 WorkItem
       ├─ else spawn（explore/implement/agent）→ RunRegistry
       └─ child terminal → bubble → 下一 turn Prepare 注入

WorkTreeStore（What）          RunRegistry（How，DM-011）
  树 / kind / 依赖 / 状态         goroutine / output / cancel
  ~/.devrix/tasks/*.json        ~/.devrix/runs/*.output
         └──── WorkItem.RunRef ────┘
```

| 层级 | 资产 | 名称 | 状态 |
|------|------|------|------|
| L1 | orchestration | D7 编排域 | 已有 |
| L2 | L2-ORCH-WORK-TREE | 统一工作单元树 | **新增** |
| L3-BE | L3-BE-ORCH-WORK-ITEM | WorkItem + WorkTree CRUD | **新增** |
| L3-BE | L3-BE-ORCH-RUN-REF | WorkItem ↔ RunRegistry 挂接 | **新增（依赖 DM-011）** |
| L4-BE | L4-BE-ORCH-FOCUS | GetFocus + uncertainty decompose | **新增 v2.0** |
| L4-BE | L4-BE-ORCH-TOOL-TASK | task_write/spawn/await/list | **新增 v2.0** |

## 5. 数据模型

### 5.1 WorkItem

```go
WorkItem {
  ID           string       // wi_<uuid>
  ParentID     string       // 空 = 顶层（通常仅 goal）
  Kind         WorkKind     // goal|explore|plan|implement|verify|checklist|shell|agent
  Status       WorkStatus   // pending|in_progress|completed|failed|cancelled
  Title        string
  Directive    string       // 自然语言目标/指令
  Uncertainty  float64      // 0=确定, 1=高度不确定（v2.0 decompose 驱动）
  Policy       ExecPolicy   // sync|async|readonly|parallel_ok
  Owner        string
  BlockedBy    []string
  Blocks       []string
  RunRef       string       // run_<uuid>，可选
  Ephemeral    bool         // checklist 默认 true
  CreatedAt / UpdatedAt
}
```

### 5.2 Kind 吸收映射

| Kind | 吸收现有 | 执行方式 |
|------|----------|----------|
| `goal` | session 根目标 | 分解，不直接 spawn |
| `explore` | delegate_explore, PlanAgent | readonly spawn |
| `plan` | delegate_plan, PlanMode 产出 | readonly spawn |
| `implement` | task_create, delegate_implement, wave worker | write spawn |
| `verify` | verification agent | readonly spawn |
| `checklist` | todo_write | turn 内 inline / ephemeral |
| `shell` | background bash | async RunRegistry |
| `agent` | call_claude/cursor, SubQuery | async RunRegistry |

### 5.3 持久化

- **v2 schema**: `{ session_id, schema_version: 2, items: [WorkItem...] }`
- **v1 迁移**: 加载 `{ tasks: [Task...] }` 时转为 `kind=implement`、无 parent
- **配置**: 沿用 `context_engine.tasks.mode: v2` + `store_dir`

## 6. 范围

### In Scope

- WorkItem + WorkTree 核心（CRUD、树遍历、依赖、EnsureGoal）
- DiskWorkItemStore v2 + v1 Task JSON 迁移
- TaskManager 委托 WorkTree（legacy Task API 兼容）
- delegate_* / PlanMode / FlowHub 写 WorkItem 树
- todo_write → checklist 子节点（底层统一，tool 名保留）
- Wave decomposer 写 WorkTree；WaveScheduler 读 WorkTree
- WorkItem.RunRef ↔ RunRegistry（与 DM-011 联调）
- QueryWorkPlan 树形读模型
- 终态 tool 面：task_write / task_spawn / task_await / task_list（含 alias 期）
- Uncertainty + GetFocus + 自动 decompose（v2.0）
- RunTurn Prepare 注入子项 terminal 通知（bubble to parent）

### Out of Scope

- 跨 Session WorkItem 可见性
- 重启后 resume 执行 goroutine（RunRegistry P2 persist list only，见 DM-011）
- D2 QueryLoop 新能力（已 LEGACY，DM-20260617-001）
- agent_tools 配置发现机制（另 change）

## 7. 验收标准

### P0 — v1.0 基础模型

| ID | 标准 |
|----|------|
| AC1 | `WorkItem` + `WorkTree` 支持 Create/Get/List/ListChildren/ListSubtree/Ancestors |
| AC2 | `EnsureGoal(session, directive)` 幂等创建 session 根 goal |
| AC3 | `DiskWorkItemStore` 读写 v2；加载 v1 `tasks[]` 自动迁移为 WorkItem |
| AC4 | `TaskManager.Create/Get/List/...` 行为不变，内部委托 WorkTree |
| AC5 | `TaskManager.Tree()` 暴露 WorkTree 供新代码使用 |

### P1 — v1.1 写入路径

| ID | 标准 |
|----|------|
| AC6 | 首条用户消息触发 `EnsureGoal` |
| AC7 | `delegate_*` spawn 创建带 `parent_id` + 正确 `kind` 的 WorkItem |
| AC8 | PlanMode approve 批量创建 goal 下 implement 子项 |
| AC9 | `todo_write` 在当前 focus 下 upsert checklist 子节点（ephemeral） |
| AC10 | Wave decomposer 输出写入 WorkTree 而非独立 TaskGraph 持久化 |

### P2 — v1.2 RunRegistry 挂接

| ID | 标准 |
|----|------|
| AC11 | spawn 时 `WorkItem.RunRef` 注册到 RunRegistry（DM-011 AC1） |
| AC12 | RunRegistry terminal 更新 WorkItem.Status + notify 父节点 |
| AC13 | `QueryWorkPlan` 返回树形 Tasks + RunRegistry Background |

### P3 — v2.0 终态 tool + 递归

| ID | 标准 |
|----|------|
| AC14 | `task_write` 合并 task_create/update + checklist 模式 |
| AC15 | `task_spawn` 合并 delegate_*；multi_agent 配置控制暴露 |
| AC16 | `task_await` 合并 task_output + status（读 RunRegistry delta） |
| AC17 | `GetFocus` 按 kind 优先级 + Uncertainty 选择 ready 节点 |
| AC18 | Uncertainty > threshold 时自动 decompose 子 WorkItem |
| AC19 | legacy `task_create`/`todo_write`/`delegate_*` alias 至少保留 1 版本 |

### 扩展验收标准（博弈论分析后新增，AC20–AC36）

> 完整定义见 `tasks.md` §验收标准汇总。此处仅登记 ID，保持 demand.md 为 AC 索引 SoT。

| ID | 标准 | Phase | 优先级 |
|----|------|-------|--------|
| AC20 | 单 WorkItem 递归 decompose 深度 ≤ 3（可配置 `work_tree.max_decompose_depth`） | Phase 5 | P2 |
| AC21 | 深度超限 fallback inline execute（保留 LLM 对 leaf task 的直接责任） | Phase 5 | P2 |
| AC22 | 同 Session 24h 内同 Kind decompose > 5 → 触发 `task_await` 人工 review | Phase 5 | P2 |
| AC23 | CI static analysis 检测 D2 直写 `sc.Todos` + 新增 `*Registry/*Manager` 类 | Phase 0 | P0 |
| AC24 | Code Owner Bot 就绪，新增 task 实体时自动 @ D7 架构师 | Phase 6 | P1 |
| AC25 | 季度 Property Rights Audit 脚本可运行，首次运行输出基准报告 | Phase 6 | P1 |
| AC27 | Uncertainty Anchor 集成测试——LLM 空 evidence 时 uncertainty 回退到 historical+structural | Phase 5 | P0 |
| AC28 | RunTurn 单层递归——LLM decompose 创建子项 → spawn → await → parent 在 children terminal 后继续 | Phase 1.5 | P1 |
| AC29 | 子任务 terminal 后 parent WorkItem uncertainty 自动重评估 | Phase 1.5 | P1 |
| AC30 | 用户在新 Session 问"昨天那个 task 完成了吗"→ 返回历史 WorkItem 状态 | Phase 7 | P2 |
| AC31 | 历史 Session WorkItem 默认不可修改（lock 协议生效） | Phase 7 | P2 |
| AC32 | "继续昨天的第三个任务"→ 创建新 Session，继承 WorkItem 上下文 | Phase 7 | P2 |
| AC33 | 同场景 10 Session 后 auto-decompose 的子任务 terminal 率 ≥ 手动 decompose | Phase 8 | P3 |
| AC34 | LLM 提议新 Kind 时附带 evidence + historical precedent | Phase 8 | P3 |
| AC35 | 新项目启动时系统建议任务模板（基于相似项目） | Phase 8 | P3 |
| AC36 | Uncertainty 阈值在 10 Session 后收敛到用户最优值（不再需要手动调整） | Phase 8 | P3 |

> **AC26** 为 Codex 提议（v1.1 empty RunRef → block spawn），Claude 保留异议——替代方案 rate-limited warn + dashboard 计数器，v1.2 hard dependency。不在当前 AC 列表中。

### 补充验收标准（S3 覆盖审计新增，AC37–AC51）

> 2026-06-18 S3 阶段 DSAFT 覆盖审计发现 5 Critical + 9 High 缺口。以下 AC 按优先级门控：Critical 必须在 Phase 对应版本实现，High 建议在对应版本实现，标注为"应实现"。

#### 状态机与数据完整性（Phase 0，v1.0）

| ID | 优先级 | 标准 |
|----|--------|------|
| **AC37** | **P0** | WorkItem 状态机强制合法转换：`pending→in_progress→completed/failed/cancelled` 为唯一路径；terminal→back to in_progress 必须拒绝并返回 `ErrInvalidTransition` |
| **AC38** | **P1** | v1→v2 迁移边界：(a) 损坏 v1 JSON → 返回 `ErrMigrationFailed` 并保留原文件；(b) 空 v1 文件 → 创建空 v2 WorkTree；(c) v1 和 v2 文件同时存在 → 优先读 v2，warn v1 将被忽略 |
| **AC39** | **P1** | `WorkTree.Remove(itemID)` 级联删除所有子孙节点（硬删除，不软删除）；删除 goal 节点等价于清空整个 session WorkTree |

#### 并发与容错（Phase 0/3）

| ID | 优先级 | 标准 |
|----|--------|------|
| **AC40** | **P0** | 循环依赖检测：`WorkTree.AddDependency(A, B)` 若 B 的传递依赖中包含 A → 返回 `ErrCircularDependency`；`GetReadyItems` 启动时全量检测并 log 所有成环项 |
| **AC41** | **P1** | RunRegistry terminal callback 失败重试：更新 WorkItem.Status 失败时指数退避重试（100ms→200ms→400ms，最多 3 次）；3 次均失败 → 写 error log + 标记 RunRegistry 条目 `notified=failed` |
| **AC42** | **P1** | 并发 terminal 通知幂等：多个子任务同一时刻 terminal → parent 只 re-resolve 一次（`sync.Once` 或等效 CAS）；后续 terminal 事件检测到 parent 已在 resolving 状态时跳过 |

#### 递归求解正确性（Phase 1.5/5，v1.5–v2.0）

| ID | 优先级 | 标准 |
|----|--------|------|
| **AC43** | **P0** | 子任务部分失败处理：父 WorkItem 的子节点 terminal 后状态混合（部分 completed + 部分 failed）→ 父节点标记 `failed`，`Directive` 追加子任务失败摘要；用户可通过 `task_spawn` 手动重试失败子项 |
| **AC44** | **P1** | GetFocus 确定性 tiebreak：同 kind + 同 uncertainty → 按 `CreatedAt` 升序（FIFO）；同 kind + 同 uncertainty + 同 CreatedAt → 按 ID 字典序。确保同一状态下 LLM 每次看到相同 focus |
| **AC45** | **P1** | ResolveFocus dispatch 路由：goal→仅 decompose；explore/plan/verify→spawn(readonly)；implement→spawn(write)；checklist→inline 当前 turn；shell/agent→spawn(async)。路由错误（如 goal spawn）→ 返回 `ErrInvalidDispatch` |

#### Wave 迁移（Phase 2，v1.1）

| ID | 优先级 | 标准 |
|----|--------|------|
| **AC46** | **P1** | WaveScheduler 读 WorkTree 正确性：(a) `GetReadyItems(session)` 返回 deps satisfied 的项；(b) `filtered by subtree(batchRootID)` 排除非本 batch 项；(c) TaskNode 投影 adapter 往返一致（WorkItem→TaskNode→WorkItem 关键字段不丢） |

#### Checklist Promote（Phase 2，v1.1）

| ID | 优先级 | 标准 |
|----|--------|------|
| **AC47** | **P1** | PlanMode approve checklist promote：ephemeral checklist 子项 → `kind=implement, ephemeral=false` 持久子项；(a) 原 checklist 项标记 `completed`；(b) 新 implement 项保留原 Title+Directive；(c) 新项 BlockedBy 继承 checklist 项的依赖关系 |

#### Tool 面（Phase 4，v2.0）

| ID | 优先级 | 标准 |
|----|--------|------|
| **AC48** | **P1** | Alias 行为等价性：`task_write(mode=checklist, items=[...])` 输出（WorkItem 树状态 + sc.Todos 投影）与旧 `todo_write(todos=[...])` 完全一致；`task_spawn(kind=explore, ...)` 与旧 `delegate_explore` 行为等价 |

#### 跨会话（Phase 7，v2.1）

| ID | 优先级 | 标准 |
|----|--------|------|
| **AC49** | **P0** | Lock 运行时 enforcement：对 `LockHistorical()` 标记的 WorkItem 调用 `UpdateStatus/Remove/UpsertChecklist` → 返回 `ErrWorkItemLocked`；只读操作（Get/List/ListChildren）不受 lock 影响 |
| **AC50** | **P1** | 链式继承追溯：Session A→B 继承（B.InheritContext(A, item)）→ Session B→C 继承（C.InheritContext(B, item')）→ C 中 WorkItem.SourceSession = A（追溯链不中断） |

#### 自演化（Phase 8，v3.0）

| ID | 优先级 | 标准 |
|----|--------|------|
| **AC51** | **P1** | 冷启动降级：(a) < 3 Session 历史数据时 `AdaptThreshold` 返回全局默认值（0.3/0.7）；(b) < 5 Session 时 `ExtractTemplate` 返回 nil（不建议模板）；(c) 降级行为对用户透明，不报错 |
| **AC52** | **P1** | 阈值振荡防止：`AdaptThreshold` 使用 hysteresis——只有当新阈值与当前值差异 > 0.1 且连续 3 个 Session 同方向时才更新；防止 terminal 率正常波动导致阈值频繁跳变 |

#### 系统韧性（Phase 0，v1.0）

| ID | 优先级 | 标准 |
|----|--------|------|
| **AC53** | **P1** | 磁盘写入原子性：`DiskWorkItemStore.Save` 先写临时文件（`*.tmp`），写入完成后 `os.Rename` 原子替换；写入中断时原文件保持完整（不产生半写损坏文件） |

> **统计：** 补充 AC37–AC53（17 条）：P0 4 条（AC37/AC40/AC43/AC49），P1 13 条。全量 AC 从 36 条扩展到 **52 条**（AC26 除外）。

### T 层测试点（草案，S3 登记 t-registry）

| T ID | Given-When-Then | 优先级 |
|------|-----------------|--------|
| D7-S1-A01-T01 | Given v1 tasks.json When Load Then 迁移为 WorkItem implement | P0 |
| D7-S1-A01-T02 | Given goal+child When ListSubtree Then DFS 含父子 | P0 |
| D7-S1-A01-T03 | Given blocked dependency When GetReadyItems Then 阻塞项 excluded | P0 |
| D7-S1-A02-T01 | Given delegate_explore When spawn Then parent=goal kind=explore | P1 |
| D7-S1-A02-T02 | Given todo_write snapshot When execute Then checklist 子项 upsert | P1 |
| D7-S2-A06-T11 | Given child terminal When RunTurn Prepare Then 父节点通知注入 | P2 |

## 8. 迁移分期（与 tasks.md 对齐）

| 阶段 | 版本 | 用户可见变化 |
|------|------|--------------|
| **Phase 0** | v1.0 | 无（纯模型 + 兼容层） |
| **Phase 1** | v1.1 | task_list 可树形；delegate 有 parent |
| **Phase 2** | v1.2 | task_output 可靠（DM-011）；RunRef 可见 |
| **Phase 3** | v2.0 | tool 名简化；递归 decompose |

## 9. 依赖

| 方向 | 需求 | 说明 |
|------|------|------|
| 前置 | DM-20260617-008 workmodel 显式注入 | 已完成 |
| 前置 | DM-20260617-001 loop_first RunTurn | 已完成 |
| 并行 | DM-011 RunRegistry | How 层；v1.2 挂接 |
| 下游 | DM-007 wave_completed 附件 | 数据源从 RunRegistry + WorkTree |
| 关联 | devrix-task-planning | PlanMode 写 WorkTree |
| 关联 | devrix-wave-scheduler | Scheduler 读 WorkTree |

## 10. 设计决策摘要（已拍板）

1. **先统一 WorkItem 模型，后改 tool 名** — 避免 API 先行、底层分裂
2. **WorkTree 与 RunRegistry 分离** — What / How 两层
3. **todo_write 保留名、改底层** — checklist 子节点，默认 ephemeral
4. **Wave 终态降为基础设施** — LLM 不必须直接调 delegate_wave
5. **Task legacy API 保留至 v2.0 删除** — TaskManager 作 adapter

## 11. clawcode 参照

- `clawcode/src/utils/task/framework.ts` — registerTask, pollTasks, generateTaskAttachments
- `clawcode/src/utils/task/diskOutput.ts` — appendTaskOutput, getTaskOutputDelta
- `clawcode/src/tasks/LocalAgentTask/LocalAgentTask.tsx` — enqueueAgentNotification
- Devrix 对齐：**WorkTree = tasks 语义；RunRegistry = framework 执行层；RunTurn = QueryLoop 递归**
