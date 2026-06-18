# Design: Unified Work Tree

**Change ID:** devrix-unified-work-tree  
**Demand ID:** DM-20260617-009  
**Status:** S3_Design

---

## 1. 架构概览

### 1.1 现状（多套 task）

```text
todo_write          → sc.Todos（session scratch，D2）
task_*              → TaskManager flat map（D7 workmodel）
wave.TaskNode       → 一次性 DAG（D7 wave）
BackgroundRegistry  → bg_* 异步句柄（D2 enforce）
delegate_*          → hubspoke + 可选 task_id 关联
notify.Bus          → workmodel 终态通知
```

**问题**：三套 ID 语义、无 parent_id、Wave 与 task board 割裂。

### 1.2 终态（两层 + 一树）

```text
┌─────────────────────────────────────────────────────────────┐
│  D7 DefaultOrchestrator.RunTurn                              │
│    focus := WorkTree.GetFocus(sessionID)                     │
│    resolve(focus) → decompose | spawn | inline               │
│    terminal events → bubble → Prepare attachments            │
└──────────────────────────┬──────────────────────────────────┘
                           │
         ┌─────────────────┴─────────────────┐
         ▼                                   ▼
   WorkTree (What)                    RunRegistry (How)
   orchestration/workmodel/           orchestration/runregistry/
   - WorkItem, WorkTree               - Register/AppendOutput/
   - DiskWorkItemStore                  SetTerminal/Cancel
   - TaskManager adapter              - 包装 BackgroundRegistry
         │                                   │
         └──── WorkItem.RunRef ──────────────┘
```

### 1.3 Kind → 现有系统吸收

| Kind | 现有入口 | WorkTree 操作 | RunRegistry |
|------|----------|---------------|-------------|
| goal | 首条用户消息 | EnsureGoal | — |
| explore | delegate_explore | Create(child, explore) | Register(readonly) |
| plan | delegate_plan, PlanMode | Create(child, plan) | Register(readonly) |
| implement | task_create, delegate_implement | Create(child, implement) | Register(write) |
| verify | /verify, verification agent | Create(child, verify) | Register(readonly) |
| checklist | todo_write | UpsertChecklist(ephemeral) | — |
| shell | run_in_background bash | Create(child, shell) | Register(async) |
| agent | call_claude, SubQuery | Create(child, agent) | Register(async) |

Wave 并行：`implement` 子项 + `Policy=parallel_ok` + `BlockedBy` 映射 DependsOn；WaveScheduler **只读** WorkTree，不持久化 TaskNode。

---

## 2. 包与代码位置

| 组件 | 目标路径 | 现状 |
|------|----------|------|
| WorkItem, WorkTree | `internal/layers/orchestration/workmodel/` | 待实现 |
| DiskWorkItemStore | 同上 `workitem_store.go` | 待实现 |
| TaskManager adapter | 同上 `task_manager.go` 委托 | 已有 flat 实现 |
| RunRegistry | `internal/layers/orchestration/runregistry/` | DM-011 待建 |
| todo_write 改底层 | `contextengine/enforce/toolrunner/todo_tool.go` | 写 sc.Todos |
| delegate 写树 | `orchestration/delegatetools/` | 写 TaskManager flat |
| Wave 读树 | `orchestration/wave/scheduler.go` | 读 TaskGraph |
| QueryWorkPlan | `orchestration/coordinator/workmodel.go` | flat List |
| bootstrap 注入 | `bootstrap/wire_coordinator.go` | 单例 TaskManager |

**域归属**：WorkTree **拥有权在 D7**；D2 仅保留 tool runner 与 session context 投影。

---

## 3. WorkTree API（设计）

```go
// workitem.go
type WorkKind string // goal|explore|plan|implement|verify|checklist|shell|agent
type ExecPolicy string // sync|async|readonly|parallel_ok

type WorkItem struct { ID, ParentID, Kind, Status, Title, Directive,
    Uncertainty, Policy, Owner, BlockedBy, Blocks, RunRef, Ephemeral,
    CreatedAt, UpdatedAt }

// work_tree.go
type WorkTree struct { ... }

func (t *WorkTree) EnsureGoal(sessionID, directive string) (*WorkItem, error)
func (t *WorkTree) Create(sessionID string, in CreateWorkItemInput) (*WorkItem, error)
func (t *WorkTree) UpsertChecklist(sessionID, parentID string, items []ChecklistEntry) error
func (t *WorkTree) Get(sessionID, itemID string) (*WorkItem, bool)
func (t *WorkTree) List(sessionID string) []*WorkItem
func (t *WorkTree) ListChildren(sessionID, parentID string) []*WorkItem
func (t *WorkTree) ListSubtree(sessionID, rootID string) []*WorkItem
func (t *WorkTree) Ancestors(sessionID, itemID string) []*WorkItem
func (t *WorkTree) GetReadyItems(sessionID string) []*WorkItem
func (t *WorkTree) GetFocus(sessionID string) (*WorkItem, error) // v2.0
func (t *WorkTree) UpdateStatus(sessionID, itemID string, status TaskStatus) error
func (t *WorkTree) SetOwner(sessionID, itemID, owner string) error
func (t *WorkTree) AddDependency(sessionID, itemID, blockedByID string) error
func (t *WorkTree) Remove(sessionID, itemID string) error

// task_manager.go — legacy adapter
func (m *TaskManager) Tree() *WorkTree
func (m *TaskManager) Create(...) *Task  // → WorkKindImplement, parent=goal
```

### 3.1 Legacy Task 互操作

```go
func (w *WorkItem) ToTask() *Task           // Subject←Title, Description←Directive
func WorkItemFromTask(t *Task) *WorkItem    // kind=implement, no parent
```

`task_*` tools 继续返回 Task JSON；v1.1 起 `task_list` 可选 `format=tree`。

### 3.2 持久化

```json
{
  "session_id": "sess_abc",
  "schema_version": 2,
  "items": [
    { "id": "wi_001", "kind": "goal", "title": "...", ... },
    { "id": "wi_002", "parent_id": "wi_001", "kind": "implement", ... }
  ]
}
```

v1 文件 `{ "tasks": [...] }` 加载时 `WorkItemsFromTasks()` 迁移，Save 写 v2。

---

## 4. RunRegistry 挂接（依赖 DM-011）

```text
spawn(item):
  runID := registry.Register(item.ID, sessionID, kind)
  item.RunRef = runID
  item.Status = in_progress

terminal(runID, status, summary):
  item := workTree.GetByRunRef(runID)
  workTree.UpdateStatus(item.ID, status)
  if item.ParentID != "":
    notifyParent(item.ParentID)
  bus.Publish(sessionID, CompletionEvent{...})
  registry.SetNotified(runID)
```

**不合并**：RunRegistry 条目含 output offset、cancel func；WorkItem 不含 goroutine 细节。

---

## 5. todo_write 迁移设计

### 5.1 v1.1 行为

```text
todo_write(todos[]):
  focus := workTree.GetFocus(sessionID) ?? goal
  workTree.UpsertChecklist(sessionID, focus.ID, todos)
  sc.Todos := projectChecklistToTodoItems(focus)  // 读缓存，prepare 不变
  verification nudge 逻辑保留（基于 checklist 子项）
```

### 5.2 UpsertChecklist 规则

- 删除 parent 下所有 `kind=checklist && ephemeral=true` 旧项
- 按 todos 数组顺序 Create 新 checklist 子项
- status 映射：pending/in_progress/completed → WorkStatus

### 5.3 Promote（plan approve）

PlanMode approve 时：将 checklist 子项 copy 为 `kind=implement, ephemeral=false` 持久子项。

---

## 6. Wave 迁移设计

### 6.1 Decomposer 输出

```text
SynthesizeTaskGraph(goal) → 旧: []TaskNode
                         → 新: batchRootID（WorkTree 子树）

for each planned step:
  workTree.Create(session, { ParentID: batchRoot, Kind: implement,
    Policy: parallel_ok, BlockedBy: dependsOn })
```

### 6.2 Scheduler 输入

```text
WaveScheduler.Start(batchRootID):
  ready := workTree.GetReadyItems(session) filtered by subtree(batchRootID)
  for each ready item: dispatch worker(item.Directive, item.Policy, ...)
```

`wave.TaskNode` 类型 **v1.1 保留为 runtime 投影**（从 WorkItem 映射），v2.0 删除。

---

## 7. RunTurn 递归求解（v2.0）

```text
function resolve(item):
  if item.Uncertainty > cfg.Threshold && decomposable(item.Kind):
    children := Decomposer.Decompose(item)
    for c in children: workTree.Create(c, parent=item)
    return AWAIT

  switch item.Kind:
    explore, plan, verify → task_spawn(readonly)
    implement, agent, shell → task_spawn(write|async)
    checklist → inline（当前 turn tool round，不 spawn）
    goal → decompose only

on child terminal:
  if all siblings terminal or ready:
    parent.Uncertainty *= decay  // 可选
    re-resolve(parent)
```

### 7.1 GetFocus 优先级

1. `status=ready`（pending + deps satisfied）
2. kind 顺序：verify > implement > explore > checklist > plan
3. 同 kind：`Uncertainty` 降序

---

## 8. Tool 面终态（v2.0）

| Tool | 参数要点 | 合并来源 |
|------|----------|----------|
| `task_write` | item_id?, parent_id?, kind, title, directive, mode=checklist | task_create/update, todo_write |
| `task_spawn` | item_id, executor?, block? | delegate_*, wave trigger |
| `task_await` | item_id, offset?, block=false | task_output, status poll |
| `task_list` | subtree?, filter=running\|all | task_list, task_get, task_list_background |

**Alias 期**：旧名注册为 thin wrapper，slog.Warn once per session。

**配置**：
- 系统 tools：always on（task_write/list/await）
- `multi_agent.enabled`：控制 task_spawn 是否含 delegate 策略
- `agent_tools`：task_spawn executor=claude|cursor

---

## 9. 与 DM-011 边界

| 层 | Change | 职责 |
|----|--------|------|
| What | **devrix-unified-work-tree** (本 change) | WorkItem 树、kind、依赖、持久化 |
| How | **devrix-unified-task-registry** (DM-011) | output delta、cancel、notified |

联调点：`WorkItem.RunRef`、terminal 双向同步、`QueryWorkPlan.Background` 读 RunRegistry。

**禁止**：把 Wave 调度器塞进 WorkTree；把 WorkItem 树塞进 RunRegistry。

---

## 10. 风险与缓解

| 风险 | 影响 | 缓解 |
|------|------|------|
| task_id 前缀变更 wi_ vs task_ | 集成测试 / 飞书卡片 | legacy Create 可配置 ID 前缀；alias 映射表 |
| todo_write 改底层破坏 prepare | IM 多轮 | sc.Todos 保留为投影缓存 |
| Wave 迁移破坏并行调度 | delegate_wave | TaskNode adapter 过渡期；feature flag |
| DM-011 未就绪阻塞 RunRef | spawn 观测 | v1.0–v1.1 RunRef 可空；v1.2 硬依赖 |
| tool rename 破坏 prompt | LLM 行为 | alias ≥1 版本 + schema 文档 |

---

## 11. 参考

- `openspec/changes/devrix-unified-work-tree/demand.md`
- `openspec/changes/devrix-unified-task-registry/demand.md` (DM-011)
- `openspec/specs/d7-orchestration/task-planning-design.md`
- `openspec/specs/d7-orchestration/spec.md` § D7-S1 Work Model
