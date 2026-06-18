# Design: Unified Work Tree

**Change ID:** devrix-unified-work-tree  
**Demand ID:** DM-20260617-009  
**Status:** S3_Gate_Passed

---

## S3-Gate Design Review

**Reviewer:** Claude (per `review-design.md` v1.0.0)  
**Date:** 2026-06-18  
**Verdict:** **Approved with Suggestions** — 0 blocking issues, 4 non-blocking suggestions

### Architecture Decisions (§2.1)

| Check | Result |
|-------|--------|
| 层归属正确 | ✅ D7 owns WorkTree; D2 keeps tool runners + sc.Todos projection |
| 接口方向正确 | ✅ WorkTree (What) ⊥ RunRegistry (How) via RunRef; no circular deps |
| 不重复造轮子 | ✅ 6→1 model unification; all existing entry points absorbed |
| 跨层依赖最小 | ✅ D2 accesses WorkTree via bootstrap injection; sc.Todos as read projection |
| 设计决策有记录 | ✅ demand.md §10 + gaming-analysis.md + bilateral-consensus.md |
| ⚠️ 命名冲突 | `contracts/worktree.go` (git worktree sandbox) vs proposed `WorkTree` (task model) — suggest rename existing to `sandbox.go` |

### Requirements Completeness (§2.2)

| Check | Result |
|-------|--------|
| 需求可追溯 | ✅ demand → proposal → design → specs → tasks 链路完整 |
| 验收标准覆盖 | ✅ AC1-AC53 (52 ACs) covering all 7 Capabilities, 5 Scenarios, P0-P3 |
| Out of Scope 明确 | ✅ proposal.md §Out of Scope |
| DM ID 无冲突 | ✅ DM-20260617-009 unique; related changes all exist |
| ⚠️ Spec delta 版本覆盖 | `d7-orchestration_delta.md` missing v1.5/v2.1/v3.0 Gherkin scenarios |

### Spec Quality (§2.3)

| Check | Result |
|-------|--------|
| Gherkin 格式正确 | ✅ 11 scenarios with GIVEN/WHEN/THEN |
| Happy path + sad path | ✅ AC37-AC53 cover: state machine violation, circular dep, partial failure, lock enforcement |
| 并发场景 | ✅ AC42 (concurrent terminal), AC53 (atomic write) |
| 错误路径 | ✅ AC37 (invalid transition), AC40 (circular dep), AC41 (retry exhaustion), AC49 (locked item write) |
| T 层映射 | ✅ 33 T-points in .openspec.yaml, all mapped to ACs |
| ⚠️ Sad path Gherkin | Sad path scenarios not yet in spec delta Gherkin form — suggest adding before S4 |

### Risk Assessment (§2.4)

| Check | Result |
|-------|--------|
| 回归风险已评估 | ✅ design.md §13: 7 risks with mitigations |
| 回滚方案可行 | ✅ TaskManager adapter (v1.0), TaskNode adapter (v1.1), feature flags |
| ⚠️ 性能评估 | No performance budget for large WorkTree (>100 items) — suggest adding load/save latency AC |

### Grill Review — Key Design Decisions

| # | Decision | Alternatives Considered | Rationale | Verdict |
|---|----------|------------------------|-----------|---------|
| 1 | WorkTree ⊥ RunRegistry separation | Single unified TaskRegistry | Williamson TCE: WorkTree high asset specificity (make internally), RunRegistry low (market interface via RunRef) | **Agreed** |
| 2 | 8 fixed WorkKind (not open tags) | Open-ended kind tags | Schelling Commitment Device: fixed surface prevents fragmentation; v3.0 allows gated expansion with D7 approval | **Agreed** |
| 3 | Parent-child tree + BlockedBy edges | Full DAG (any→any edges) | Tree simplifies ownership/cascade; cross-branch deps via BlockedBy edges; Wave handles parallel siblings | **Agreed** |
| 4 | v1.5 minimal recursion before v2.0 | Skip to v2.0 directly | Risk mitigation: ~80 line MVP validates decompose→spawn→await→continue loop; v1.5 is independently shippable | **Agreed** |
| 5 | depth ≤ 3 + 24h daily limit 5 | Unlimited recursion | Harsanyi game constraint: prevents recursive cheap talk amplification; depth limit forces LLM accountability on leaf tasks | **Agreed** |
| 6 | 4-tool surface (write/spawn/await/list) | Keep 8+ existing tools | Commitment Device: tool count IS the surface; fewer tools = fewer fragmentation vectors; alias period for migration | **Agreed** |
| 7 | sc.Todos as read projection (not deleted) | Delete sc.Todos entirely | Backward compat: D2 prepare reads sc.Todos; projection sync from WorkTree checklist; v2.0 demotes from authoritative to cache | **Agreed** |
| 8 | v1.0 zero external API change | Change API + model together | "Silent revolution":产权集中无用户感知; TaskManager adapter preserves all existing callers; migration validated before v1.1 | **Agreed** |
| 9 | AC26: rate-limited warn vs block spawn | Block spawn on empty RunRef (Codex) | Claude: block is premature at v1.1; rate-limited warn + dashboard counter → v1.2 hard dependency | **Deferred to v1.2** |

### Suggested Improvements (non-blocking, can defer to S4+)

1. **S1.** Rename `contracts/worktree.go` → `contracts/sandbox.go` to avoid naming collision with WorkItem/WorkTree task model
2. **S2.** Add v1.5/v2.1/v3.0 Gherkin scenarios to `specs/d7-orchestration_delta.md` (currently only v1.0-v2.0)
3. **S3.** Add sad path Gherkin scenarios for AC37/AC40/AC43/AC49 (state machine violation, circular dep, partial failure, lock enforcement)
4. **S4.** Add performance AC: `DiskWorkItemStore.Load` ≤ 500ms for 100-item WorkTree; `Save` ≤ 200ms

### Next Steps

- S3-Gate passed → proceed to S4 implementation from Phase 0 (`feat/workitem-foundation`)
- Parallel: DM-011 RunRegistry Phase 1 (unlocks v1.2)
- Suggestions S1-S4 can be addressed during S4 or deferred to S6

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

### 1.2 终态（v3.0 完整演进）

> 演进路线见 `version-roadmap.md`。v1.0 产权集中 → v1.5 最小递归 → v2.0 完整递归 → v2.1 跨会话 → v3.0 自演化。

```text
┌──────────────────────────────────────────────────────────────────┐
│  D7 DefaultOrchestrator.RunTurn                                   │
│                                                                    │
│  v1.0–v1.1: 静态树 — focus 由外部设定，树结构手动/规则创建          │
│  v1.5:      单层递归 — focus → decompose? → spawn → await →       │
│             re-resolve (depth=1)                                   │
│  v2.0:      完整递归 — 多层 decompose (depth≤3) + Uncertainty      │
│             Anchor + 4 工具面                                       │
│  v2.1:      跨会话 — 读历史 WorkItem；lock/propose/arbitration      │
│  v3.0:      自演化 — 自适应 uncertainty + 任务模板 + 结构优化       │
└──────────────────────────┬───────────────────────────────────────┘
                           │
         ┌─────────────────┴──────────────────┐
         ▼                                    ▼
   WorkTree (What)                     RunRegistry (How)
   orchestration/workmodel/            orchestration/runregistry/
   - WorkItem, WorkTree                - Register/AppendOutput/
   - DiskWorkItemStore                   SetTerminal/Cancel
   - TaskManager adapter               - 包装 BackgroundRegistry
   - UncertaintyAnchor (v2.0)          - Cross-session RunRef (v2.1)
   - TaskTemplateStore (v3.0)
         │                                    │
         └──── WorkItem.RunRef ───────────────┘
                           │
         ┌─────────────────┴──────────────────┐
         ▼ (v3.0)                             ▼ (v2.1)
   Evolution Engine                    Cross-Session Index
   - Adaptive thresholds               - Historical WorkItem query
   - Pattern extraction                - Lock/propose/arbitration
   - Structure optimization            - Session inheritance
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

## 2. DSAFT 资产变更

> **规范对齐:** 本 change 涉及 D7 域，跨 S1/S2/S3/S5 四个 Scenario。以下按 `architecture-design.md` §1.1 要求定义 A 层活动变更和 F 层功能点编排。

### 2.1 变更总览

| 类型 | DSAFT ID | 名称 | 版本 | 说明 |
|------|---------|------|------|------|
| **修改** | D7-S1-A02 | ManageTask | v1.0 | 底层从 flat map 改为委托 WorkTree |
| **修改** | D7-S1-A03 | QueryWorkPlan | v1.2 | 输出从 flat list 改为树形 WorkPlanSnapshot |
| **修改** | D7-S3-A01 | ScheduleWave | v1.1 | 输入从独立 TaskGraph 改为读 WorkTree ready 子项 |
| **修改** | D7-S5-A02 | SynthesizeTaskGraph | v1.1 | 输出从 []TaskNode 改为 WorkTree 子树 |
| **修改** | D7-S2-A06 | RunTurnLoop | v1.5 | 新增 resolve hook：focus → decompose → spawn → await |
| **新增** | D7-S1-A07 | EnsureGoal | v1.1 | 首条用户消息自动创建 kind=goal WorkItem |
| **新增** | D7-S1-A08 | UpsertChecklist | v1.1 | todo_write → WorkTree checklist 子节点 |
| **新增** | D7-S2-A08 | GetFocus | v1.5 | 优先级调度选择当前应执行的 WorkItem |
| **新增** | D7-S2-A09 | ResolveFocus | v1.5 | 单层递归：focus → decompose/spawn/inline |
| **新增** | D7-S1-A09 | ComputeUncertainty | v2.0 | Uncertainty Anchor 计算（historical+structural+evidence） |
| **新增** | D7-S1-A10 | QueryHistoricalWorkPlan | v2.1 | 跨 Session 只读查询历史 WorkItem |
| **新增** | D7-S1-A11 | InheritWorkContext | v2.1 | 从历史 Session 继承 WorkItem 上下文 |
| **新增** | D7-S6-A01 | AdaptThreshold | v3.0 | 自适应 Uncertainty 阈值（Bayesian 更新） |
| **新增** | D7-S6-A02 | ExtractTemplate | v3.0 | 跨项目任务模板提取 |
| **新增** | D7-S6-A03 | OptimizeStructure | v3.0 | WorkTree 结构自优化（terminal 率反馈） |
| **删除** | — | — | v2.0 | wave.TaskNode 持久化 / flat Task 直写 / sc.Todos 权威 |

### 2.2 修改的 A — 变更详情

#### D7-S1-A02 ManageTask（v1.0 修改）

| 维度 | 变更前 | 变更后 |
|------|--------|--------|
| 存储 | `map[string]*Task` flat map | 委托 `WorkTree`（通过 `TaskManager.Tree()`） |
| task_create | 创建独立 flat Task | 创建 `kind=implement` WorkItem，parent=goal |
| task_list | 返回 flat []Task | 支持 `format=tree`，返回树形结构 |
| task_update | 直接修改 map entry | → `WorkTree.UpdateStatus` |
| 兼容性 | — | `ToTask()` / `WorkItemFromTask()` 互转；外部 API 不变 |

**F 层变更：**
- D7-S1-A02-F01 CreateTask: 修改为 `WorkTree.Create(kind=implement)`（原直写 flat map）
- D7-S1-A02-F04 ListReadyTasks: 修改为 `WorkTree.GetReadyItems()`（tree-aware）
- D7-S1-A02-F05 PersistToDisk: 升级为 DiskWorkItemStore v2（含 v1 自动迁移）

#### D7-S3-A01 ScheduleWave（v1.1 修改）

| 维度 | 变更前 | 变更后 |
|------|--------|--------|
| 输入 | 独立持久化的 `wave.TaskGraph` | `WorkTree.GetReadyItems(session) filtered by subtree` |
| TaskNode | 独立类型，Scheduler 拥有生命周期 | WorkItem runtime 投影（v1.1），v2.0 删除 |
| 写回 | Scheduler 写 TaskGraph 状态 | Scheduler 只读 WorkTree；通过 RunRegistry 更新 |

#### D7-S5-A02 SynthesizeTaskGraph（v1.1 修改）

| 维度 | 变更前 | 变更后 |
|------|--------|--------|
| 输出 | `[]TaskNode`（独立 DAG） | `batchRootID`（WorkTree 子树） |
| TaskNode 创建 | `wave.NewTaskNode(...)` | `workTree.Create(session, {ParentID: batchRoot, Kind: implement, ...})` |

#### D7-S2-A06 RunTurnLoop（v1.5 修改）

| 维度 | 变更前 | 变更后 |
|------|--------|--------|
| 循环内容 | prepare → LLM ↔ tools → persist | 新增 resolve hook：focus → decompose/spawn/await → re-resolve |
| 递归 | 无 | v1.5 单层（depth=1）；v2.0 多层（depth≤3） |

**F 层变更：**
- **新增** D7-S2-A06-F07 ResolveFocus: 单 turn 内的 focus resolve 决策（v1.5）
- **新增** D7-S2-A06-F08 AwaitChildren: 等待子 WorkItem terminal → bubble 通知（v1.5）

### 2.3 新增的 A — 完整 DSAFT 定义

#### v1.1 新增

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| **D7-S1-A07** | **EnsureGoal** | A-BE | session_id, directive | *WorkItem | goal.created | `workmodel/work_tree.go` |
| **D7-S1-A08** | **UpsertChecklist** | A-BE | session_id, parent_id, []ChecklistEntry | — | checklist.upserted | `workmodel/work_tree.go` |

**F 层：**
- D7-S1-A07-F01 CreateGoalItem: 创建 `kind=goal` WorkItem 作为 session 根节点
- D7-S1-A08-F01 ReplaceChecklist: 删除旧 ephemeral checklist → 按序创建新项
- D7-S1-A08-F02 ProjectToTodos: WorkItem checklist → sc.Todos 投影（供 D2 prepare 使用）

#### v1.5 新增

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| **D7-S2-A08** | **GetFocus** | A-BE | session_id | *WorkItem | — | `workmodel/work_tree.go` |
| **D7-S2-A09** | **ResolveFocus** | A-BE | *WorkItem, uncertainty | AWAIT_CHILDREN \| SPAWN \| INLINE | focus.resolved | `turn/orchestrator.go` |

**F 层：**
- D7-S2-A08-F01 SelectFocus: 优先级排序（ready > kind > uncertainty）
- D7-S2-A08-F02 FilterReady: 过滤 status=ready 且 deps satisfied 的 WorkItem
- D7-S2-A09-F01 SingleDecompose: 单层 decompose（depth=1，不递归子项）
- D7-S2-A09-F02 DispatchByKind: 按 kind 路由到 spawn(readonly/write/async) 或 inline

#### v2.0 新增

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| **D7-S1-A09** | **ComputeUncertainty** | A-BE | *WorkItem, *UncertaintyEvidence | float64 | — | `workmodel/uncertainty.go` |

**F 层：**
- D7-S1-A09-F01 ComputeHistoricalFailure: 查询同类 Kind 历史 terminal 率
- D7-S1-A09-F02 ComputeStructuralComplexity: 依赖深度 + FileScope 扩散度
- D7-S1-A09-F03 AnchorLLMClaim: LLM claim × weight + historical × weight，evidence 空时 LLM 权重归零
- D7-S1-A09-F04 CheckDepthConstraint: 验证 DecomposeDepth ≤ MaxDepth（AC20）
- D7-S1-A09-F05 CheckDailyDecomposeLimit: 验证同 Kind 24h decompose ≤ 5（AC22）

#### v2.1 新增

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| **D7-S1-A10** | **QueryHistoricalWorkPlan** | A-BE | session_id | []*WorkItem | — | `workmodel/work_tree.go` |
| **D7-S1-A11** | **InheritWorkContext** | A-BE | session_id, parent_session_id, item_id | *WorkItem | context.inherited | `workmodel/work_tree.go` |

**F 层：**
- D7-S1-A10-F01 LoadHistoricalTree: 加载历史 Session WorkTree（disk v2）
- D7-S1-A10-F02 LockHistoricalItems: 历史 WorkItem 标记 immutable
- D7-S1-A11-F01 CloneSubtree: 复制 WorkItem + 子树到新 Session
- D7-S1-A11-F02 SetSourceSession: 标记继承来源，保留追溯链

#### v3.0 新增（D7-S6 Self-Evolution Scenario）

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| **D7-S6-A01** | **AdaptThreshold** | A-BE | user_id, project_id, kind | float64 | threshold.updated | `workmodel/optimizer.go` |
| **D7-S6-A02** | **ExtractTemplate** | A-BE | kind, min_frequency | *TaskTemplate | template.extracted | `workmodel/template.go` |
| **D7-S6-A03** | **OptimizeStructure** | A-BE | session_id, terminal_rate | — | structure.updated | `workmodel/optimizer.go` |

**F 层：**
- D7-S6-A01-F01 UpdateBayesianPrior: 从历史 terminal 结果更新先验分布
- D7-S6-A01-F02 ComputeAdaptiveThreshold: user_bias × 0.4 + project_bias × 0.3 + global × 0.3
- D7-S6-A02-F01 ClusterWorkTrees: 按 goal 语义相似度 + projectType 聚类
- D7-S6-A02-F02 RankByTerminalRate: 按 terminal 率排序子树结构
- D7-S6-A03-F01 UpdateStructureScore: structure_hash → terminal_rate 评分更新
- D7-S6-A03-F02 SelectOptimalStructure: top-K 结构选择

### 2.4 DSAFT 资产统计（变更后）

| 层 | 变更前 | 变更后 | 变更 |
|----|--------|--------|------|
| S | 5 (S1-S5) | **6** (S1-S6) | +1 (D7-S6 Self-Evolution, v3.0) |
| A | 24 | **32** (+A07-A11, +S6-A01-A03) | +8 新增, 5 修改 |
| F | 51 | **~75** | +~24 新增/修改 |
| T | 66 | **~112** | +~46 (AC20-53 对应 T 点) |

> **T 层登记待 v2.0 完成时更新 `t-registry.md`。**

### 2.5 版本-gated DSAFT 变更

| 版本 | 新增 A | 修改 A | 新增 S |
|------|--------|--------|--------|
| v1.0 | — | D7-S1-A02 | — |
| v1.1 | D7-S1-A07, D7-S1-A08 | D7-S3-A01, D7-S5-A02 | — |
| v1.2 | — | D7-S1-A03 | — |
| v1.5 | D7-S2-A08, D7-S2-A09 | D7-S2-A06 | — |
| v2.0 | D7-S1-A09 | — | — |
| v2.1 | D7-S1-A10, D7-S1-A11 | — | — |
| v3.0 | D7-S6-A01, D7-S6-A02, D7-S6-A03 | — | D7-S6 |

---

## 3. 包与代码位置

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

## 4. WorkTree API（设计）

```go
// workitem.go
type WorkKind string // goal|explore|plan|implement|verify|checklist|shell|agent
type ExecPolicy string // sync|async|readonly|parallel_ok

type WorkItem struct {
    // 核心标识
    ID        string
    ParentID  string    // "" = root
    SessionID string

    // 语义
    Kind      WorkKind
    Status    TaskStatus
    Title     string
    Directive string    // LLM 可执行的指令

    // 不确定性 (v1.5: LLM claim only; v2.0: anchored)
    Uncertainty float64               // [0, 1]
    Evidence    *UncertaintyEvidence  // v2.0: 非空时 LLM claim 有效

    // 执行
    Policy   ExecPolicy
    Owner    string
    RunRef   string    // v1.2: RunRegistry entry ID
    Ephemeral bool     // true = checklist 类临时项，不持久化

    // 依赖
    BlockedBy []string
    Blocks    []string

    // 递归约束 (v2.0)
    DecomposeDepth int  // 当前项在树中的深度，用于 AC20 深度上限检查

    // 时间戳
    CreatedAt time.Time
    UpdatedAt time.Time
}

// v2.0: Uncertainty 的证据锚定
type UncertaintyEvidence struct {
    Source     string // tool_output | dependency_unknown | code_smell
    ToolCallID string // 指向发现不确定性的具体 tool call
    Snippet    string // 引用输出片段（不能凭空捏造）
}

// work_tree.go
type WorkTree struct { ... }

// 基础 CRUD（v1.0）
func (t *WorkTree) EnsureGoal(sessionID, directive string) (*WorkItem, error)
func (t *WorkTree) Create(sessionID string, in CreateWorkItemInput) (*WorkItem, error)
func (t *WorkTree) Get(sessionID, itemID string) (*WorkItem, bool)
func (t *WorkTree) List(sessionID string) []*WorkItem
func (t *WorkTree) UpdateStatus(sessionID, itemID string, status TaskStatus) error
func (t *WorkTree) Remove(sessionID, itemID string) error

// 树遍历（v1.0）
func (t *WorkTree) ListChildren(sessionID, parentID string) []*WorkItem
func (t *WorkTree) ListSubtree(sessionID, rootID string) []*WorkItem
func (t *WorkTree) Ancestors(sessionID, itemID string) []*WorkItem

// Checklist（v1.1）
func (t *WorkTree) UpsertChecklist(sessionID, parentID string, items []ChecklistEntry) error

// 调度（v1.1）
func (t *WorkTree) GetReadyItems(sessionID string) []*WorkItem
func (t *WorkTree) AddDependency(sessionID, itemID, blockedByID string) error

// 递归求解（v1.5）
func (t *WorkTree) GetFocus(sessionID string) (*WorkItem, error)

// RunRef 索引（v1.2）
func (t *WorkTree) GetByRunRef(runID string) (*WorkItem, bool)

// 递归约束（v2.0）
func (t *WorkTree) SetOwner(sessionID, itemID, owner string) error
func (t *WorkTree) MaxDepth() int                     // 可配置，默认 3
func (t *WorkTree) DecomposeCount(kind WorkKind, since time.Time) int  // AC22

// 跨会话（v2.1）
func (t *WorkTree) QueryHistorical(sessionID string) ([]*WorkItem, error)
func (t *WorkTree) LockHistorical(sessionID string) error
func (t *WorkTree) InheritContext(sessionID, parentSessionID, itemID string) (*WorkItem, error)

// task_manager.go — legacy adapter (v1.0)
func (m *TaskManager) Tree() *WorkTree
func (m *TaskManager) Create(...) *Task  // → WorkKindImplement, parent=goal
```

### 4.1 Legacy Task 互操作

```go
func (w *WorkItem) ToTask() *Task           // Subject←Title, Description←Directive
func WorkItemFromTask(t *Task) *WorkItem    // kind=implement, no parent
```

`task_*` tools 继续返回 Task JSON；v1.1 起 `task_list` 可选 `format=tree`。

### 4.2 持久化

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

## 5. RunRegistry 挂接（依赖 DM-011）

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

## 6. todo_write 迁移设计

### 6.1 v1.1 行为

```text
todo_write(todos[]):
  focus := workTree.GetFocus(sessionID) ?? goal
  workTree.UpsertChecklist(sessionID, focus.ID, todos)
  sc.Todos := projectChecklistToTodoItems(focus)  // 读缓存，prepare 不变
  verification nudge 逻辑保留（基于 checklist 子项）
```

### 6.2 UpsertChecklist 规则

- 删除 parent 下所有 `kind=checklist && ephemeral=true` 旧项
- 按 todos 数组顺序 Create 新 checklist 子项
- status 映射：pending/in_progress/completed → WorkStatus

### 6.3 Promote（plan approve）

PlanMode approve 时：将 checklist 子项 copy 为 `kind=implement, ephemeral=false` 持久子项。

---

## 7. Wave 迁移设计

### 7.1 Decomposer 输出

```text
SynthesizeTaskGraph(goal) → 旧: []TaskNode
                         → 新: batchRootID（WorkTree 子树）

for each planned step:
  workTree.Create(session, { ParentID: batchRoot, Kind: implement,
    Policy: parallel_ok, BlockedBy: dependsOn })
```

### 7.2 Scheduler 输入

```text
WaveScheduler.Start(batchRootID):
  ready := workTree.GetReadyItems(session) filtered by subtree(batchRootID)
  for each ready item: dispatch worker(item.Directive, item.Policy, ...)
```

`wave.TaskNode` 类型 **v1.1 保留为 runtime 投影**（从 WorkItem 映射），v2.0 删除。

---

## 8. RunTurn 递归求解

### 8.1 v1.5 — 最小递归（MVP）

> **设计目标：** 用最小改动让 WorkTree "活起来"。单层递归，验证 decompose→spawn→await→continue 核心循环。depth=1 硬编码，无 Uncertainty Anchor（LLM claim 直接使用）。

```text
// v1.5: 单层递归（depth=1 硬编码）
function resolve_v1_5(item):
  if item.DecomposeDepth >= 1:
    // 达到最大深度，直接执行
    return dispatchByKind(item)

  if item.Uncertainty > 0.7 && decomposable(item.Kind):
    // 单层 decompose：创建子 WorkItem
    children := Decomposer.Decompose(item)
    for c in children:
      c.DecomposeDepth = item.DecomposeDepth + 1  // children depth = 1
      workTree.Create(session, c, parent=item)
    return AWAIT_CHILDREN

  return dispatchByKind(item)

function dispatchByKind(item):
  switch item.Kind:
    explore, plan, verify → spawn(readonly)
    implement, agent, shell → spawn(write|async)
    checklist → inline（当前 turn tool round）
    goal → decompose only（不直接 spawn）

on child terminal:
  if all siblings terminal:
    // 聚合子结果，re-resolve parent
    parent.Status = ready
    // v1.5: uncertainty 简单重评估（LLM re-claim）
    re-resolve(parent)
```

**v1.5 明确不做：**
- 多层递归（child 不再 decompose）
- Uncertainty Anchor（LLM claim 直接使用，等 v2.0）
- 深度/宽度约束（depth=1 不需要）
- Tool rename（保留 8 个工具名）
- 删 legacy

**v1.5 软依赖 DM-011 Phase 1**（Register + SetTerminal）。未就绪时降级使用 Legacy BackgroundRegistry。

### 8.2 v2.0 — 完整递归

> **设计目标：** 多层递归 + Uncertainty Anchor + 深度约束。v1.5 已验证核心循环，v2.0 加复杂度。

```text
// v2.0: 多层递归 + Anchor
function resolve_v2_0(item):
  // 深度硬约束（AC20）
  if item.DecomposeDepth >= workTree.MaxDepth():
    return dispatchByKind(item)

  // 计算 Anchored Uncertainty（AC27）
  anchored := ComputeUncertainty(item, item.Evidence)
  // evidence 为空 → LLM claim 权重强制归零 → 回退 historical+structural

  if anchored > cfg.Threshold && decomposable(item.Kind):
    // 多层 decompose（depth 递增）
    children := Decomposer.Decompose(item)
    for c in children:
      c.DecomposeDepth = item.DecomposeDepth + 1
      workTree.Create(session, c, parent=item)
    return AWAIT_CHILDREN

  return dispatchByKind(item)

on child terminal:
  if all siblings terminal:
    // v2.0: uncertainty 重评估走完整 Anchor
    parent.Status = ready
    if parent.DecomposeDepth < workTree.MaxDepth():
      re-resolve(parent)  // 递归继续
    else:
      // 达到深度上限，inline 执行（AC21）
      inlineExecute(parent)
```

**v2.0 新增约束：**
- `maxDecomposeDepth = 3`（可配置，AC20）
- `maxChildrenPerDecompose = 7`（可配置）
- `maxDailyDecomposePerKind = 5`（AC22）
- 超限 fallback inline execute（AC21）

### 8.3 GetFocus 优先级（v1.5+）

1. `status=ready`（pending + deps satisfied）
2. kind 顺序：verify > implement > explore > checklist > plan
3. 同 kind：`Uncertainty` 降序

### 8.4 Uncertainty Anchor（v2.0）

```go
func ComputeUncertainty(wi *WorkItem, ev *UncertaintyEvidence) float64 {
    histFail := reputation.FailureRate(wi.Kind)
    structComp := structuralComplexity(wi)
    
    // LLM claim 权重随历史样本量动态调整
    sampleSize := reputation.SampleSize(wi.Kind)
    llmWeight := lerp(0.50, 0.15, min(sampleSize/100.0, 1.0))
    
    // evidence 为空 → LLM claim 权重强制归零
    if ev == nil || ev.ToolCallID == "" {
        llmWeight = 0
    }
    
    anchorWeight := 1.0 - llmWeight
    return llmWeight*wi.Uncertainty + anchorWeight*(0.6*histFail + 0.4*structComp)
}
```

---

## 9. Tool 面终态（v2.0）

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

## 10. 与 DM-011 边界

| 层 | Change | 职责 |
|----|--------|------|
| What | **devrix-unified-work-tree** (本 change) | WorkItem 树、kind、依赖、持久化 |
| How | **devrix-unified-task-registry** (DM-011) | output delta、cancel、notified |

联调点：`WorkItem.RunRef`、terminal 双向同步、`QueryWorkPlan.Background` 读 RunRegistry。

**禁止**：把 Wave 调度器塞进 WorkTree；把 WorkItem 树塞进 RunRegistry。

---

## 11. v2.1 跨会话设计

> **目标:** WorkTree 生命周期超越单 Session。**依赖:** DM-011 完整交付。

### 11.1 跨 Session 只读查询

```text
QueryWorkPlan(historicalSessionID):
  1. 加载历史 Session 的 WorkTree (disk v2)
  2. 所有 WorkItem 默认 lock（immutable）
  3. 返回树形 WorkPlanSnapshot
  4. IM 卡片展示："上次 3 个 task，2 完成 1 未完成"
```

### 11.2 Mutable 引用协议

```text
"继续昨天的第三个任务":
  1. 用户请求引用历史 WorkItem（session_A, item_003）
  2. D7 创建新 Session（session_B）
  3. session_B 调用 InheritContext(session_A, item_003)
     → 复制 WorkItem + 子树到 session_B
     → 标记 SourceSession = session_A
     → 历史 WorkItem 保持 lock
  4. session_B 正常递归求解
```

### 11.3 RunRegistry 跨 Session 索引

```text
GetByRunRef(runID) 支持跨 Session 查询:
  - DM-011 RunRegistry 维护 session_id → runIDs 索引
  - terminal 状态 = lock 信号
  - 历史 run 可读但不可 cancel
```

---

## 12. v3.0 自演化设计

> **目标:** WorkTree 的结构和使用模式通过历史数据自我优化。**依赖:** v2.1 交付 + 10+ Session 数据积累。

### 12.1 自适应 Uncertainty 阈值

```text
v2.0: 全局固定阈值 (0.3 / 0.7)
v3.0: 每用户/每项目的 Bayesian 更新阈值

AdaptiveThreshold(userID, projectID, kind):
  base = globalDefault[kind]
  userBias = learnedFrom(userID, kind, last30days)
  projectBias = learnedFrom(projectID, kind, ...)
  return base * 0.3 + userBias * 0.4 + projectBias * 0.3

learnedFrom 基于:
  - 历史 decompose 的 terminal 率
  - 用户手动 override 的频率
  - 同类项目的模式
```

### 12.2 WorkItem Kind 自动检测

```text
LLM 遇到无法归类的新场景:
  1. LLM 调用 propose_kind(name, evidence, examples[])
  2. D7 S3-Gate review 审批或拒绝
  3. 审批通过 → Kind 枚举扩展
  4. 拒绝 → LLM 用现有 Kind 重新归类

防止 Kind 膨胀:
  - 新 Kind 需 ≥3 个独立 Session 的 evidence
  - 季度 Kind 审计（合并低使用率 Kind）
  - max 20 Kind 硬上限
```

### 12.3 任务模板提取

```text
Template = 高频子树模式:
  输入: 跨项目、跨用户的 WorkTree 历史数据
  步骤:
    1. 按 (goalDirective 语义相似度, projectType) 聚类
    2. 提取每类的典型子树结构
    3. 计算每类的 terminal 率
    4. terminal 率 top-10 的子树 → TaskTemplate
  输出: TaskTemplateStore

新项目使用:
  用户说"添加暗色模式"
  → D7 发现 projectType=frontend + goal="暗色模式" 匹配 Template#42
  → "建议任务结构: explore 现有主题 → implement CSS 变量 → verify 视觉检查"
  → 用户确认 → 按模板创建 WorkItem 子树
```

### 12.4 WorkTree 结构自优化

```text
反馈循环:
  for each session:
    for each decompose:
      observe: structure → terminal_rate
      update: structure_score[structure_hash] += terminal_rate - prior

  每 10 Session:
    for each kind:
      top_structures = topK(structure_score, k=5)
      update decompose_strategy[kind] = top_structures[0]

效果: 同场景 terminal 率在 10 Session 后显著提升
```

### 12.5 v3.0 数据流

```text
┌──────────────────────────────────────────┐
│  D7 RunTurn (v3.0)                        │
│                                            │
│  focus := WorkTree.GetFocus(session)       │
│                                            │
│  // v3.0: 检查模板建议                     │
│  template := TemplateStore.Match(focus)    │
│  if template != nil && confidence > 0.8:  │
│    suggest(template)  → 用户确认           │
│                                            │
│  // v3.0: 自适应阈值                       │
│  threshold := AdaptiveThreshold(user, proj)│
│  if anchored > threshold:                  │
│    decompose → spawn → await               │
│                                            │
│  // v3.0: 反馈学习                         │
│  on terminal:                              │
│    EvolutionEngine.Observe(structure,      │
│      terminal_rate, user_override)         │
│    EvolutionEngine.UpdateModels()          │
└──────────────────────────────────────────┘
         │                    │
         ▼                    ▼
  TemplateStore         EvolutionEngine
  - 跨项目子树模式       - 自适应阈值模型
  - 语义相似度匹配       - 结构评分更新
  - terminal 率排名      - Bayesian 先验更新
```

---

## 13. 风险与缓解

| 风险 | 影响 | 缓解 |
|------|------|------|
| task_id 前缀变更 wi_ vs task_ | 集成测试 / 飞书卡片 | legacy Create 可配置 ID 前缀；alias 映射表 |
| todo_write 改底层破坏 prepare | IM 多轮 | sc.Todos 保留为投影缓存 |
| Wave 迁移破坏并行调度 | delegate_wave | TaskNode adapter 过渡期；feature flag |
| DM-011 未就绪阻塞 RunRef | spawn 观测 | v1.0–v1.1 RunRef 可空；v1.2 硬依赖；v1.5 软依赖（可降级 Legacy） |
| tool rename 破坏 prompt | LLM 行为 | alias ≥1 版本 + schema 文档 |
| **v2.0 大爆炸** | 递归引擎延迟 | **v1.5 最小递归 MVP 独立交付**（单层递归，验证核心循环） |
| **DM-011 完全失败** | v1.5+ 全阻塞 | **v1.0–v1.1 WorkTree 独立价值**（统一真相源）；备选 Legacy BackgroundRegistry |
| **产权过渡期阳奉阴违** | D2/D4 绕过 WorkTree | **CI static analysis (AC23)** + Code Owner Bot (AC24) + 季度审计 (AC25) |

> 完整演进路线和风险分析见 `version-roadmap.md`。

---

## 14. 参考

- `openspec/changes/devrix-unified-work-tree/demand.md`
- `openspec/changes/devrix-unified-work-tree/version-roadmap.md` — v1.0→v3.0 演进路线
- `openspec/changes/devrix-unified-work-tree/gaming-analysis.md` — 博弈论分析 v2
- `openspec/changes/devrix-unified-task-registry/demand.md` (DM-011)
- `openspec/specs/d7-orchestration/task-planning-design.md`
- `openspec/specs/d7-orchestration/spec.md` § D7-S1 Work Model
