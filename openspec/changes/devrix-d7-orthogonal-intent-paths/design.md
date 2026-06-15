# Design: D7 Intent 路径正交化

**Change ID:** devrix-d7-orthogonal-intent-paths
**Demand ID:** DM-20260615-004
**Status:** S3_Design

## 1. Root Cause Analysis

### 1.1 现象
`coordinator/orchestrator.go:149-162` 的 switch：
```go
switch intent.Kind {
case IntentSkip:       return closed channel
case IntentCommand:    return o.handleCommand(...)   // → fastPath.Run("[command:xxx]")
case IntentFast:       return o.fastPath.Run(...)
case IntentOrchestrate: return o.orchestrate(...)    // → fastPath.Run("[orchestrate:...]")
}
```

### 1.2 根因
v1.0 closure 阶段（2026-06-15）注释（orchestrator.go:222-237）明确说明：
> "v1.0: route to FastPath with a system prompt that asks the LLM to plan internally"

这是 v1.0 阶段让 D7 wired up 起来、让 bootstrap 跑通的**时间盒内的务实妥协**。当时 PlanMode / SynthesizeTaskGraph / WaveScheduler 还没 ready。

### 1.3 为何是债
- v1.1 落地时要在 `fastPath.Run` 内部 `if hint prefix == "[command:" do X / if hint prefix == "[orchestrate:" do Y` —— 把所有真实路径塞进同一个函数
- intent_kind metric 4 类样本不可比
- `IntentKind` 枚举名实不符

### 1.4 为何现在改
v1.1 已 ready：PlanMode + SynthesizeTaskGraph + WaveScheduler + PlanCLICommands 全部 IMPLEMENTED（layer-delta Phase H/K/L/M + DM-020 closure）。

## 2. Solution Design

### 2.1 原则
**IntentKind = 真实路由决策**。识别对了，对应路径就真的被走。system_prompt 不再携带 intent 信息。

### 2.2 4 路径 = 4 真实执行链

```
ProcessMessage (orchestrator.go)
  ├─ IntentSkip        → return closed channel                          (orchestrator.go:150-153)
  ├─ IntentCommand     → CommandHandler.Handle(...)                      (command_handler.go)  ★ 新增
  ├─ IntentFast        → FastPath.Run(...)                               (fastpath.go)         保持
  └─ IntentOrchestrate → OrchestratePath.Run(...)                        (orchestrate_path.go) ★ 新增
```

### 2.3 CommandHandler 设计

```go
// internal/layers/orchestration/coordinator/command_handler.go

// CommandHandler 显式分发 IntentCommand 到 D7 内部的 CLI/PlanMode 处理器。
// 不再走 FastPath，让 LLM "假装"看到命令。
type CommandHandler struct {
    cli     *workmodel.CLICommands
    plan    *workmodel.PlanCLICommands
    sink    EventPublisher
}

// Handle 返回 EngineEvent 流：先把 CLICommands/PlanCLICommands 的字符串结果包成
// text 事件，再 emit complete 事件。Channel 由 caller（D1 gateway）drain。
func (h *CommandHandler) Handle(ctx context.Context, req ProcessRequest, intent IntentClassification) (<-chan *contracts.EngineEvent, error) {
    if h == nil || h.cli == nil {
        return nil, fmt.Errorf("orchestrator: CommandHandler not wired (bootstrap missing CLICommands)")
    }
    out := make(chan *contracts.EngineEvent, 4)

    // 解析命令 token：intent.Command 已由 classifier 提取
    cmd := strings.TrimSpace(intent.Command)
    args := []string{}
    if idx := strings.IndexAny(cmd, " \t\n"); idx >= 0 {
        args = strings.Fields(cmd[idx+1:])
        cmd = cmd[:idx]
    }

    var reply string
    switch cmd {
    case "/plan":
        // PlanMode trigger: /plan enter | /plan <goal> | /plan approve | /plan reject | /plan status | /plan show
        reply = h.plan.Handle(args, req.SessionID, "", workmodel.PlanAgentReadOnlyTools)
    case "/task":
        // Task CRUD
        parsed := workmodel.ParseCommand(req.Message)
        if parsed == nil {
            parsed = &workmodel.Command{Name: "help"}
        }
        reply = h.cli.Handle(parsed, req.SessionID)
    case "/stop":
        // 调用 InterruptHandler（与 ProcessMessage 共享同一 cancel 池）
        if h.interruptHandler != nil {
            _ = h.interruptHandler.Handle(ctx, req.SessionID)
        }
        reply = "stopped"
    case "/status", "/version", "/ping":
        // 状态查询类：直接调用对应方法（v1.1 暂用占位）
        reply = "ok: " + cmd
    case "/help":
        reply = h.cli.Help()  // 新增导出方法（v1.1.0）
    default:
        return nil, fmt.Errorf("orchestrator: unknown command %q (not in whitelist)", intent.Command)
    }

    // 异步 emit 事件
    go func() {
        defer close(out)
        if h.sink != nil {
            h.sink.Publish(ctx, &contracts.EngineEvent{Type: "command_reply", Content: reply, SessionID: req.SessionID})
        }
        out <- &contracts.EngineEvent{Type: "text", Content: reply, SessionID: req.SessionID}
        out <- &contracts.EngineEvent{Type: "complete", SessionID: req.SessionID}
    }()
    return out, nil
}
```

**关键设计：**
- 不调 FastPath，不调 D2 LLM，command 路径**零 LLM 成本**
- `/plan` 走 PlanCLICommands → PlanMode 状态机
- `/task` 走 CLICommands → TaskManager CRUD
- 事件类型 `command_reply` 让 observability 区分 command 路径 vs LLM 路径

### 2.4 OrchestratePath 设计

```go
// internal/layers/orchestration/coordinator/orchestrate_path.go

// OrchestratePath 显式编排多步任务：SynthesizeTaskGraph → WaveScheduler → 流式回放
type OrchestratePath struct {
    decomposer *TaskDecomposer
    scheduler  WaveSchedulerRunner  // 接口
    sink       EventPublisher
}

// WaveSchedulerRunner 抽象 WaveScheduler 依赖，便于测试 mock
type WaveSchedulerRunner interface {
    Start(ctx context.Context, sessionID string, graph *wave.TaskGraph) error
    WaitForCompletion(ctx context.Context, sessionID string) ([]wave.Artifact, error)
}

// Run 真正调用 D7-S5-A02 + D7-S3-A01。
func (op *OrchestratePath) Run(ctx context.Context, req ProcessRequest, intent IntentClassification) (<-chan *contracts.EngineEvent, error) {
    if op == nil || op.decomposer == nil || op.scheduler == nil {
        return nil, fmt.Errorf("orchestrator: OrchestratePath not wired (bootstrap missing decomposer/wave)")
    }
    out := make(chan *contracts.EngineEvent, 16)

    go func() {
        defer close(out)

        // 1) SynthesizeTaskGraph（D7-S5-A02）
        result, err := op.decomposer.SynthesizeTaskGraph(ctx, req.SessionID, req.Message)
        if err != nil {
            out <- &contracts.EngineEvent{Type: "error", Content: "synthesize: " + err.Error(), SessionID: req.SessionID}
            return
        }
        // emit plan formation
        out <- &contracts.EngineEvent{Type: "plan_formed", Content: fmt.Sprintf("%d tasks", len(result.Nodes)), SessionID: req.SessionID}
        if op.sink != nil { op.sink.Publish(ctx, ...) }

        // 2) Build TaskGraph
        graph := buildTaskGraph(result.Nodes)

        // 3) Start Wave (D7-S3-A01)
        if err := op.scheduler.Start(ctx, req.SessionID, graph); err != nil {
            out <- &contracts.EngineEvent{Type: "error", Content: "wave start: " + err.Error(), SessionID: req.SessionID}
            return
        }
        out <- &contracts.EngineEvent{Type: "wave_started", SessionID: req.SessionID}

        // 4) Wait for completion
        artifacts, err := op.scheduler.WaitForCompletion(ctx, req.SessionID)
        if err != nil {
            out <- &contracts.EngineEvent{Type: "error", Content: "wave wait: " + err.Error(), SessionID: req.SessionID}
            return
        }

        // 5) 汇总输出
        summary := summarizeArtifacts(artifacts)
        out <- &contracts.EngineEvent{Type: "text", Content: summary, SessionID: req.SessionID}
        out <- &contracts.EngineEvent{Type: "complete", SessionID: req.SessionID}
    }()
    return out, nil
}
```

**关键设计：**
- 异步 goroutine 包出 stream（与 FastPath.Run 的 sink mirror 模式一致）
- 真正调 `SynthesizeTaskGraph` + `WaveScheduler.Start`（D7-S5-A02 + D7-S3-A01）
- 单步消息也会创建 Wave（1 个 TaskNode，不创建新资源）
- 事件流透明：`plan_formed` → `wave_started` → 聚合 `text` → `complete`

### 2.5 Orchestrator 改造

```go
// orchestrator.go ProcessMessage (修改后)
switch intent.Kind {
case IntentSkip:
    ch := make(chan *contracts.EngineEvent)
    close(ch)
    return ch, nil
case IntentCommand:
    return o.commandHandler.Handle(ctx, req, intent)   // ★ 不再调 fastPath
case IntentFast:
    return o.fastPath.Run(ctx, req, "")
case IntentOrchestrate:
    return o.orchestratePath.Run(ctx, req, intent)      // ★ 不再调 fastPath
}
```

`handleCommand` 与 `orchestrate` 两个旧函数**完全删除**（inlining 完毕）。

### 2.6 SessionOrchestrator 注入

```go
// NewSessionOrchestrator 新增 option
type OrchestratorOption func(*SessionOrchestrator)

func WithCommandHandler(h *CommandHandler) OrchestratorOption {
    return func(o *SessionOrchestrator) { o.commandHandler = h }
}

func WithOrchestratePath(p *OrchestratePath) OrchestratorOption {
    return func(o *SessionOrchestrator) { o.orchestratePath = p }
}
```

`NewSessionOrchestrator` 在 `o.commandHandler == nil` 时构造 default（用 `workmodel.NewCLICommands(manager)` + `workmodel.NewPlanCLICommands(...)`，`manager` 从 `o.workModel` 反射获取）。nil guard 在 switch 进入时统一报错。

## 3. Key Interfaces / Types

```go
// 新增：coordinator/command_handler.go
type CommandHandler struct { ... }
func NewCommandHandler(cli *workmodel.CLICommands, plan *workmodel.PlanCLICommands, sink EventPublisher) *CommandHandler
func (h *CommandHandler) Handle(ctx, req, intent) (<-chan *contracts.EngineEvent, error)

// 新增：coordinator/orchestrate_path.go
type WaveSchedulerRunner interface {
    Start(ctx, sessionID, *wave.TaskGraph) error
    WaitForCompletion(ctx, sessionID) ([]wave.Artifact, error)
}
type OrchestratePath struct { ... }
func NewOrchestratePath(d *TaskDecomposer, s WaveSchedulerRunner, sink EventPublisher) *OrchestratePath
func (op *OrchestratePath) Run(ctx, req, intent) (<-chan *contracts.EngineEvent, error)

// 修改：coordinator/orchestrator.go
type SessionOrchestrator struct {
    // ... 既有字段
    commandHandler  *CommandHandler       // ★ 新增
    orchestratePath *OrchestratePath      // ★ 新增
}
```

## 4. Data Flow

### 4.1 Command path
```
D1 message "/plan add auth"
  → D7-S2 ProcessMessage
    → classifier.Classify → Intent{IntentCommand, Command:"/plan", ...}
    → switch IntentCommand
      → commandHandler.Handle
        → parse: cmd="/plan", args=["add","auth"]
        → plan.Handle(args, sessionID, ...) → "Plan mode: Active..."
        → emit command_reply → text → complete
  → D1 drain channel → IM 渲染
```

### 4.2 Orchestrate path
```
D1 message "fix bug in auth.go && add tests"
  → D7-S2 ProcessMessage
    → classifier.Classify → Intent{IntentOrchestrate, ...}
    → switch IntentOrchestrate
      → orchestratePath.Run
        → decomposer.SynthesizeTaskGraph → [node1, node2]
        → emit plan_formed
        → scheduler.Start(graph)
        → emit wave_started
        → scheduler.WaitForCompletion
        → emit text (summary) → complete
  → D1 drain channel
```

## 5. File Manifest

### 新增
- `internal/layers/orchestration/coordinator/command_handler.go` (~150 行)
- `internal/layers/orchestration/coordinator/orchestrate_path.go` (~180 行)
- `internal/layers/orchestration/coordinator/command_handler_test.go` (~120 行, 3 P0 AC)
- `internal/layers/orchestration/coordinator/orchestrate_path_test.go` (~150 行, 3 P0 AC)
- `openspec/changes/devrix-d7-orthogonal-intent-paths/specs/d7-orchestration/spec.md` (Gherkin)

### 修改
- `internal/layers/orchestration/coordinator/orchestrator.go` (ProcessMessage switch 4 行 + 删 2 个旧函数 + 加 2 个 option + nil guard)
- `internal/layers/orchestration/coordinator/orchestrator_test.go` (Command / Orchestrate / AntiFabrication 3 测试更新)
- `openspec/specs/d7-orchestration/spec.md` (D7-S2-A01 Requirement 状态 + Revision History)
- `openspec/specs/d7-orchestration/a-registry.md` (D7-S2-A01 标注 ✅ v1.1.0)
- `openspec/specs/d7-orchestration/t-registry.md` (新增 3 P0 T: T04 / T05 / T06)

### 删除
- `internal/layers/orchestration/coordinator/orchestrator.go::handleCommand` (inlined)
- `internal/layers/orchestration/coordinator/orchestrator.go::orchestrate` (inlined)

## 6. Regression Risk Assessment

| 风险 | 严重度 | 缓解 |
|------|--------|------|
| Command 路径不再调 D2 → `TestSessionOrchestrator_ProcessMessage_Command` 断言 "D2 call = 1" 失败 | 中 | 更新测试为 "CommandHandler.Handle called once" |
| Orchestrate 路径不再调 D2 → `TestSessionOrchestrator_AntiFabrication_NoSyntheticProgress` 失败 | 中 | fakeWaveScheduler 注入完整 FlowEvent 序列 |
| Orchestrate 路径异步 goroutine，channel 关闭时机变化 | 低 | 镜像 FastPath.Run 的 sink mirror 模式 |
| PlanCLICommands.Handle 返回字符串格式可能与 D2 LLM 输出差异大 | 低 | 不修改 PlanCLICommands 内部；D1 渲染层已抽象 |
| `NewCommandHandler` / `NewOrchestratePath` 的 sink 字段 nil panic | 中 | nil guard 放在 `Handle` / `Run` 入口 |
| `interruptHandler` 在 CommandHandler 中通过 nil interface 调用 panic | 低 | 加 nil guard；保留 fastpath nil 行为 |

## 7. Rollback Plan

每 commit 可独立 revert：
1. revert commit 3（switch 改回 fastPath）→ command/orchestrate path 退化为占位（v1.0 行为）
2. revert commit 2（删 orchestrate_path.go）→ 同上
3. revert commit 1（删 command_handler.go）→ 同上

无数据迁移（无 DDL / 无配置变更），rollback = `git revert`。

## 8. 决策记录

### Decision: CommandHandler 路径走 `workmodel.CLICommands` / `workmodel.PlanCLICommands`，不调 FastPath

**选项:**
| 方案 | 优点 | 缺点 |
|------|------|------|
| A. 调 workmodel CLI（推荐） | 零 LLM 成本；PlanMode 状态机原生支持 | 行为与 v1.0 不同（v1.0 让 LLM 解释命令）|
| B. 调 FastPath.Run（v1.0 行为）| 零改动 | 不正交；债未还 |
| C. 新建独立 command DSL | 概念最清晰 | 重写成本高，workmodel CLI 已存在 |

**选择:** A
**理由:** workmodel.CLICommands / PlanCLICommands 已是 v1.1 落地的标准 command 处理器；Command 路径应零 LLM 成本（不浪费 LLM 解释 `/plan` 这种显式命令）；A 是真正"v1.1 真实落地"。

### Decision: OrchestratePath 走 SynthesizeTaskGraph + WaveScheduler，不调 FastPath

**选项:**
| 方案 | 优点 | 缺点 |
|------|------|------|
| A. SynthesizeTaskGraph → Wave（推荐）| 真实多步编排；v1.1 已 ready | 单步消息会创建 1-TaskNode Wave |
| B. 调 FastPath + system_prompt hint（v1.0）| 单步快 | 不正交；v1.0 妥协 |
| C. 调 D2.RunQueryLoop 让 LLM 自拆 | 不创建 Wave | 实际等同 v1.0 |

**选择:** A
**理由:** v1.1 已实现 SynthesizeTaskGraph（5s timeout LLM-augmented + rule fallback）+ WaveScheduler（5-slot pool + ConflictGuard + DAG）。单步消息产生 1-TaskNode Wave 不增加资源成本（仍走 5-slot 池）。"orchestrate" 语义本就指多步，v1.0 让 LLM 自拆是把决策成本推给 LLM，违反 DM-020 D7 拥有 LLM 调用权但任务结构由 D7 决定的原则。

### Decision: 保留 orchestrator.go::ProcessMessage 外部签名

**理由:** 契约稳定。`contracts.IOrchestrationEntry.ProcessMessage(ctx, sessionID, message) (<-chan *contracts.EngineEvent, error)` 是 D1 入口的对外契约，改签名意味着改 D1 → D7 的 wiring。switch 内部实现变化对 D1 透明。
