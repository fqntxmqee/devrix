# Design: Wave Scheduler

**Demand ID:** DM-20260611-007  
**Status:** Draft

## 1. 架构总览

```
Plan Engine ──► TaskGraph (DAG)
                    │
                    ▼
              WaveScheduler ◄── ConflictGuard
                    │
     ┌──────────────┼──────────────┐
     ▼              ▼              ▼
 call_cursor   call_claude-code   SubQuery×3
     │              │              │
     └──────────────┴──────────────┘
                    │
            ExecutionFlowHub
                    │
         ┌──────────┴──────────┐
         ▼                     ▼
   Leader Queue          Feishu WorkerCard×N
   (汇总/下一波)         (thinking + output)
```

**原则：**

- 编排权仍在 **Leader**；Scheduler 是 **执行层**，不调用 LLM 做调度决策
- Plan Engine **写** TaskGraph；Scheduler **读** ready 节点并 **占槽执行**
- Worker **无** delegate_* / 子 Fork 编排权（继承 D4 约束）

## 2. TaskGraph 扩展（Plan Engine 产出）

```go
type TaskNode struct {
    ID              string
    Title           string
    Directive       string
    WorkerType      WorkerType // cursor | claude_code | subagent
    DependsOn       []string
    ContextPolicy   ContextPolicy
    UpstreamTaskID  string   // policy=upstream 时
    SidechainAgentID string  // policy=resume 时
    FileScope       []string // 调度冲突检测：glob 路径
    ConflictGroup   string   // 同组写任务互斥
    SystemPromptExtra string // fresh 任务补充提示
}

type WorkerType string
const (
    WorkerCursor      WorkerType = "cursor"
    WorkerClaudeCode  WorkerType = "claude_code"
    WorkerSubAgent    WorkerType = "subagent"
)

type ContextPolicy string
const (
    ContextFresh    ContextPolicy = "fresh"     // 无 Leader 历史
    ContextResume   ContextPolicy = "resume"    // Sidechain 续跑
    ContextUpstream ContextPolicy = "upstream"  // 依赖 Task artifact
)
```

Plan Engine 在现有 Milestone/Plan 流程末尾增加 **TaskGraph 物化**（写 TaskManager + 内存 DAG）。

## 3. WaveScheduler

### 3.1 槽位配置（默认）

```yaml
orchestration:
  wave_scheduler:
    enabled: true
    worker_pool:
      cursor:      { max_concurrent: 1, tool: call_cursor }
      claude_code: { max_concurrent: 1, tool: call_claude-code }
      subagent:    { max_concurrent: 3, backend: subquery }
    dispatch: continuous  # 2B：有空槽即派
    workdir_policy: shared # 3B：同目录，调度规避冲突
```

### 3.2 持续调度算法

```
loop:
  ready = graph.ReadyNodes()  // deps 全 completed
  for task in ready (priority order):
    if !conflictGuard.Allow(task, running): continue
    slot = pool.Acquire(task.WorkerType)
    if slot == nil: continue  // 该类型槽满
    go runWorker(task, slot)
  if no running && ready empty: break
  wait: slot released | task completed | ctx cancelled
```

- **非** batch barrier：任一 Worker 完成 → 释放槽 → 立即尝试下一 ready Task
- `running` 上限全局 5（各类型子上限 1/1/3）

### 3.3 WorkerRunner 统一接口

```go
type WorkerRunner interface {
    Run(ctx context.Context, spec WorkerRunSpec) (<-chan WorkerEvent, error)
}

type WorkerRunSpec struct {
    SessionID   string
    TaskID      string
    WorkerID    string
    WorkDir     string
    Directive   string
    Context     ResolvedContext // 由 ContextPolicy 解析
    Emit        func(WorkerEvent)
    Cancel      context.CancelFunc // Scheduler 注入，Worker 必须监听 ctx.Done()
}
```

| WorkerType | Runner 实现 |
|------------|-------------|
| cursor | `AgentToolRunner(call_cursor)` + stream bridge |
| claude_code | `AgentToolRunner(call_claude-code)` |
| subagent | `query.Run(SubQueryParams{...})` |

## 4. ContextPolicy Resolver

| Policy | Messages | SystemPrompt | 其他 |
|--------|----------|--------------|------|
| fresh | `[]` + user directive | base + `SystemPromptExtra` + file_scope 说明 | 新 Sidechain |
| resume | Sidechain.Load(agent_id) | base | QueryDepth+1 |
| upstream | `[]` + directive | base + upstream summary | ArtifactStore.Get(depTaskID) |

**ArtifactStore**：Worker 完成时写入 `{task_id: {summary, files_changed[], exit_code}}`，供 downstream upstream  policy 消费。

## 5. ConflictGuard（3B — 上层规避同目录写冲突）

不启用 worktree；调度前检查：

1. **ConflictGroup**：同组 Task 最多 1 个 `running`
2. **FileScope 交集**：若两 running Task 的写 scope（非 read-only）glob 交集非空 → 后者排队
3. **Worker 类型 hint**：Plan 标注 `read_only: true` 的 explore Task 可与写 Task 并行

```go
func (g *ConflictGuard) Allow(candidate TaskNode, running []RunningTask) bool
```

Plan Engine 负责产出合理的 `file_scope` / `conflict_group`；Guard 为硬约束兜底。

## 6. Worker 生命周期与 Shutdown（clawcode Swarm 参照）

**参考：** clawcode `teammateMailbox.ts` shutdown request/approve、`TaskStopTool`、`spawnInProcess` cleanup

Wave Worker 必须有 **可取消、可观测、可释放槽位** 的完整生命周期，不能只有 Start 没有 Stop。

### 6.1 状态机

```
pending → running → { completed | failed | cancelled }
```

| 状态 | 触发 |
|------|------|
| running | Scheduler dispatch 占槽 |
| completed | Worker 正常结束 + artifact 写入 |
| failed | 非 cancel 错误 |
| cancelled | Leader/Wave/User 主动 cancel |

### 6.2 Cancel 协议（依赖 DM-009）

```go
type WorkerHandle struct {
    TaskID      string
    WorkerType  WorkerType
    SlotID      string
    Cancel      context.CancelFunc
    StartedAt   time.Time
}

func (s *WaveScheduler) CancelWorker(taskID string) error      // 单 Worker
func (s *WaveScheduler) CancelAll(sessionID string) int        // session 结束 / /new
func (s *WaveScheduler) CancelByConflictGroup(group string) int // ConflictGuard 升级
```

**实现要点：**

1. **SubAgent 槽：** `context.WithCancel` 传入 `query.Run`；cancel → synthetic failed + 释放槽
2. **CLI 槽（cursor/claude-code）：** AgentToolRunner 持有 OS process / session handle；cancel 发 SIGTERM → 超时 SIGKILL
3. **槽位释放：** 无论 terminal 原因，`<-done` 后 **必须** `pool.Release(workerType)` 并 `conflictGuard.Unregister`
4. **IM 卡：** cancelled/failed 时 Worker 卡 footer 显示原因，关闭 streaming

### 6.3 触发源

| 触发源 | 动作 |
|--------|------|
| Leader 调用 `task_stop(task_id)` | `CancelWorker`（DM-009） |
| 用户飞书 `/new` / session 过期 | `CancelAll(sessionID)` |
| Plan 重入（新 wave 覆盖） | `CancelAll` + 新 graph |
| ConflictGuard 死锁检测（P2） | 可选 cancel 低优先级 running |

### 6.4 clawcode 差异

| clawcode | Devrix Wave |
|----------|-------------|
| mailbox shutdown 握手（approve/reject） | **简化：** Scheduler 单向 cancel，不等待 Worker 回复 |
| tmux pane kill | CLI process SIGTERM/KILL |
| in-process AsyncLocalStorage cleanup | SubQuery ctx cancel + ProcessOverlay 释放 |

### 6.5 T 层增补

| T ID | Given-When-Then | 优先级 |
|-------|-----------------|--------|
| {T}-ORCH-19 | Given running SubAgent Worker When CancelWorker Then 30s 内槽位释放且 status=cancelled | P0 |
| {T}-ORCH-20 | Given 5 running When CancelAll Then 全部 terminal 且 pool 全释放 | P0 |
| {T}-ORCH-21 | Given CLI Worker running When cancel Then 进程终止且 IM 卡显示 cancelled | P1 |

## 7. IM — Worker 双区块卡片

### 7.1 数据模型

```go
type WorkerCardSession struct {
    TaskID         string
    WorkerID       string
    WorkerType     string
    CardMsgID      string
    ThinkingMsgID  string  // 区块 A
    OutputMsgID    string  // 区块 B
    thinkingBuf    strings.Builder
    outputBuf      strings.Builder
}
```

 keyed by `sessionID + taskID`（非 session 级单卡）。

### 7.2 事件映射

| EngineEvent / FlowEvent | 区块 |
|-------------------------|------|
| thinking | 思考区 patch |
| text / tool_result (worker) | 输出区 patch |
| complete / failed / cancelled | 输出区 footer + 卡状态 |

FlowEvent 扩展 metadata：`task_id`, `worker_type`, `render=worker_card`.

### 7.3 与 devrix-feishu-streaming 关系

- 复用 cardkit 元素流式（若 enabled）
- Worker 卡为 **新 Renderer**，不挤占 Leader 主回复卡

## 8. Leader 集成

1. Plan 完成 → `waveScheduler.Start(ctx, sessionID, graph)`
2. Scheduler 发 `FlowEvent` → Hub → Leader Queue（delegate-progress 风格摘要）
3. 全部 Task terminal → Scheduler 回调 Leader：`wave_completed` + artifact 汇总
4. Leader 可选继续下一 Plan 轮或回复用户

## 9. 包结构（草案）

```
internal/layers/orchestration/wave/
  scheduler.go      # WaveScheduler
  pool.go           # WorkerPool slots
  conflict.go       # ConflictGuard
  context.go        # ContextPolicy resolver
  artifact.go       # ArtifactStore
  runners/
    subagent.go
    agent_tool.go

internal/layers/communication/adapters/
  feishu_worker_card.go  # 双区块 Renderer
```

## 10. T 层测试点（草拟 — 实施前登记 t-registry）

| T ID | Given-When-Then | 优先级 |
|-------|-----------------|--------|
| {T}-ORCH-10 | Given DAG 6 ready subagent + 1 cursor When 持续调度 Then 峰值并发=5且cursor≤1 sub≤3 | P0 |
| {T}-ORCH-11 | Given Task A completes When B depends A and policy=upstream Then B 收到 A artifact 无 Leader 全量 | P0 |
| {T}-ORCH-12 | Given policy=fresh When SubAgent 启动 Then Messages 仅含 directive | P0 |
| {T}-ORCH-13 | Given 两 Task 同 conflict_group When 调度 Then 不并行 | P0 |
| {T}-ORCH-14 | Given Worker 流式事件 When IM 渲染 Then 每 Task 独立双区块卡 | P0 |
| {T}-ORCH-15 | Given 槽位释放 When 队列仍有 ready Then 立即派发下一 Task | P0 |
| {T}-ORCH-16 | Given call_cursor + call_claude-code 并行 When 不同 file_scope Then 可同时 running | P1 |
| {T}-ORCH-17 | Given Plan 产出 DAG When Scheduler Start Then 仅 ready 节点被派发 | P0 |
| {T}-ORCH-18 | Given wave 全完成 When Leader 回调 Then 收到 wave_completed 汇总 | P1 |
| {T}-ORCH-19 | Given running SubAgent When CancelWorker Then 槽位释放 status=cancelled | P0 |
| {T}-ORCH-20 | Given 5 running When CancelAll Then 全释放 | P0 |
| {T}-ORCH-21 | Given CLI Worker When cancel Then 进程终止 IM 卡 cancelled | P1 |

## 11. 与现有设计文档关系

- **扩展** `design-d4-v2.md` §1「CLI 不参与委派」→ v3 Wave 池纳入 CLI Agent Tool
- **对齐** `design-orchestration-v3.md` D7 方向，本变更先在 v2 `orchestration/` 包落地 Scheduler，不升格顶层 D7
- **承接** DM-012 未交付 v3 任务 T24–T27（自 `openspec/archive/2026-06-10-devrix-queryloop-context/tasks.md` 迁移）
- **依赖** DM-006 cardkit 基础设施（T13 复用 `feishu_cardkit.go`）
- **依赖** DM-009 Background Cancel 协议（Worker shutdown）

## 12. ADR-007: CLI Agent 进入 Wave Worker 池

**状态：** Accepted（2026-06-11）  
**决策者：** 产品 / 架构（用户确认 1B/2B/3B）

### 背景

`design-d4-v2.md` 规定 Delegate 路径 **不含** `call_cursor` / `call_claude-code`，CLI Agent 仅作 Leader 按需工具调用。Wave Scheduler 需将 CLI 纳入固定 Worker 池以实现 DAG 并行。

### 决策

1. **D4 Delegate 约束不变** — Leader 仍不可通过 `delegate_*` 嵌套编排 CLI
2. **Wave Scheduler 为独立编排层** — 由 Plan Engine 产出 TaskGraph，Scheduler 直接调用 AgentTool，**不经过** D4 Delegate
3. **同 WorkDir（3B）** — CLI 与 SubAgent 默认共享 Leader WorkDir；写冲突由 **ConflictGuard**（`conflict_group` + `file_scope`）在调度层规避，不强制 worktree
4. **上下文策略** — CLI Worker 使用 `ContextPolicy=fresh` + Task directive，**不**继承 Leader 全量 Messages

### 后果

| 正面 | 负面 / 缓解 |
|------|-------------|
| 复杂任务可 cursor + claude-code + SubAgent 真并行 | 同目录写冲突 → ConflictGuard P0 |
| IM 5 卡独立展示 CLI 输出 | 需新 `feishu_worker_card.go`，sequence 域与 DM-006 回复卡隔离 |
| Plan Engine 统一 DAG | Plan 产出需含 `worker_type` + `file_scope` 字段（T1） |

### 替代方案（已拒绝）

- **A: worktree 隔离** — 用户选 3B，调度层规避而非物理隔离
- **B: CLI 仅 Leader 串行** — 无法满足 5 槽并行产品目标
- **C: 扩 D4 Delegate 含 CLI** — 破坏 d4-v2 Sidechain/Resume 语义，Worker 编排权失控
