# Design: 诊断工具能力 E2E 可达性 — 13 项 wiring 闭环

**Change ID:** devrix-diagnostic-tools-wiring
**Demand ID:** DM-20260617-002
**Status:** S3_Design

---

> **能力别名前缀 (Capability Aliases)**
>
> 本 change 复用 DM-016 已确立的 14 个 DSAFT Activity 节点（详见 `openspec/archive/2026-06-17-devrix-diagnostic-tools-parity/spec.md` §0）。G1-G6 / A1-A7 保留为需求侧 alias。
>
> 本文档用 Activity ID（权威）+ alias（便利）双重标注。Activity 命名遵循 `D{N}-S{N}-A{NN}-{FunctionName}`。

## 0. Grill Review 结论

| Decision | 结论 | 备注 |
|----------|------|------|
| L1-L6 wiring 分类 | **Agreed** | 见 proposal §3.2 |
| G5 FreeFork 暴露为 LLM tool | **Agreed** | 默认 worktree 隔离 |
| G6 Tracker 异步 tick | **Agreed** | 1s tick, ctx 控制 |
| G3 Notify consume 走 prompt assembler | **Agreed** | 不动 output_assembler |
| L2 slash command 路由扩展 cli.go | **Agreed** | 与已有 /task /plan 一致 |
| A4 FaultInject 推迟到 P2 | **Agreed** | 仍 build-tag 隔离 |

## 1. Root Cause Analysis

### 1.1 根因：DM-20260616-003 把"library 实现"等同于"功能可达"

DM-016 的 S5 acceptance-report §5 列出了 6 项 Cross-Domain Wiring，但**只声明了接口存在 / publish 触发**，未声明**消费侧调用方**。例如：

```go
// toolrunner/sandbox.go (DM-016 已实现)
type CommandPolicy struct {
    ASTAnalyzer ASTAnalyzer  // G2 接口已声明
}

// 但 NewCommandPolicy() 中:
// ❌ 未注入 sandboxast.PolicyAnalyzer 实例
return &CommandPolicy{
    Enabled:   true,
    Allowlist: allow,
    DenyPatterns: patterns,
    WorkDirLock: true,
    // ASTAnalyzer: <nil>  ← 缺注入
}
```

结果：G2 库存在但生产环境只走 regex denylist；AST 仅在单测中验证。

### 1.2 9 项 library 包有 `NewXxx` 工厂函数但零调用方

通过 `grep -r "freefork\.New\|doctor\.NewDefaultDoctor\|windowanalyzer\.NewTokenAnalyzer"` 在 `cmd/devrix/main.go` 和 `internal/bootstrap/` 中搜索，仅在测试文件中有引用，主路径上零调用。

### 1.3 acceptance-report §5 wiring 表本身具有误导性

```
| `observability.Bridge` | `tracker.Tracker` | D5-S23-A02 (G6) wiring (linter 注册) |
```

实际 `tracker.Tracker` 在 `internal/observability/diagnose/tracker/` 下，但**从未在 `Bridge` 或 bootstrap 中实例化**。"wiring" 实际是"该 package 存在"。S5 验收的自检表把"package 存在"误标为"wiring 完成"，导致 E2E IM 可达率 7.7% 而 acceptance 自报"全 PASS"。

## 2. Solution Design

### 2.1 总体架构：6 Level 接入分层

```
                    ┌─────────────────────────────────────────┐
                    │         cmd/devrix/main.go (entry)       │
                    └────────────────┬────────────────────────┘
                                     │
                    ┌────────────────▼────────────────────────┐
                    │    internal/bootstrap/ (DI container)    │
                    │  ┌──────────────────────────────────┐    │
                    │  │ context_engine_builder.go (L1)   │    │
                    │  │   - RegisterVerifyTool          │    │
                    │  │   - RegisterFreeForkTool        │    │
                    │  │   - RegisterTrackerTool          │    │
                    │  │ observability.go (L4)            │    │
                    │  │   - debugfilter.New wrapper     │    │
                    │  │   - ASTAnalyzer injection       │    │
                    │  │ session_store.go hook (L5)      │    │
                    │  │   - transcript.OnSessionClose   │    │
                    │  └──────────────────────────────────┘    │
                    └─┬──────────┬─────────────┬──────────┬────┘
                      │          │             │          │
            ┌─────────▼─┐  ┌─────▼─────┐  ┌────▼────┐  ┌─▼──────────┐
            │ LLM tool  │  │ CLI slash │  │ Error   │  │ Background │
            │ runner    │  │ command   │  │ inject  │  │ tick/consume│
            │ (L1)      │  │ (L2)      │  │ (L3)    │  │ (L4-L6)    │
            └───────────┘  └───────────┘  └─────────┘  └────────────┘
```

**核心原则**：
- 接入点全部在 `internal/bootstrap/` 和 `internal/cli/`，**不修改** `internal/layers/` 下的 library 实现
- library 包通过 `NewXxx()` 工厂暴露，接入点持有单例
- IM 消息路径：feishu → D1 capture → D7 ingress → LLM tool / CLI slash / 直接调用
- 错误路径：sandbox / llmgateway / agent → errors.Wrap → InjectClassification → ctx propagation

### 2.2 Level 1: LLM Tool 暴露（L1，4 项）

#### 2.2.1 G4 Verifier (D6-S11-A02)

```go
// 文件: internal/layers/contextengine/enforce/toolrunner/verify_tool.go
package toolrunner

type verifyRunner struct{}

func (r *verifyRunner) Name() string { return "verify_plan_execution" }

func (r *verifyRunner) Schema() ToolSchema {
    return ToolSchema{
        Name: "verify_plan_execution",
        Description: "Verify all 'done' items in tasks.md have evidence (file exists, _test.go has func TestXxx)",
        Parameters: `{"type":"object","properties":{"tasks_file":{"type":"string"}}}`,
    }
}

func (r *verifyRunner) RiskLevel() types.RiskLevel { return types.RiskLevelLow }

func (r *verifyRunner) Execute(ctx context.Context, workDir, input string) (*ToolResult, error) {
    var in struct{ TasksFile string `json:"tasks_file"` }
    if err := json.Unmarshal([]byte(input), &in); err != nil {
        return &ToolResult{Error: err.Error()}, nil
    }
    if in.TasksFile == "" {
        in.TasksFile = filepath.Join(workDir, "tasks.md")
    }
    v := verify.NewVerifier()
    report, err := v.Verify(ctx, verify.VerifyInput{TasksFile: in.TasksFile, WorkDir: workDir})
    if err != nil {
        return &ToolResult{Error: err.Error()}, nil
    }
    out, _ := json.Marshal(report)
    return &ToolResult{Output: string(out)}, nil
}

// 文件: internal/layers/contextengine/enforce/toolrunner/verify_register.go
func RegisterVerifyTool(reg *ToolRegistry) error {
    return reg.Register(&verifyRunner{})
}
```

**Bootstrap 注入点** (`context_engine_builder.go`):
```go
// 在 buildWithGate() 中, RegisterLSPTool 之后
if err := toolrunner.RegisterVerifyTool(toolReg); err != nil {
    slog.Error("register verify tool", "error", err)
}
```

#### 2.2.2 G5 FreeFork (D4-S11-A02 + D4-S13-A02)

```go
// 文件: internal/layers/contextengine/enforce/toolrunner/freefork_tool.go
package toolrunner

type freeForkRunner struct {
    forker freefork.Forker
}

func (r *freeForkRunner) Name() string { return "free_fork" }

func (r *freeForkRunner) Schema() ToolSchema {
    return ToolSchema{
        Name: "free_fork",
        Description: "Batch fork N sub-agents with isolated worktrees for parallel investigation",
        Parameters: `{"type":"object","required":["requests"],"properties":{
            "requests": {"type":"array","items":{"type":"object","required":["prompt"],"properties":{
                "prompt": {"type":"string"},
                "isolation": {"enum":["worktree","none"],"default":"worktree"}
            }}}
        }}`,
    }
}

func (r *freeForkRunner) Execute(ctx context.Context, workDir, input string) (*ToolResult, error) {
    var in struct {
        Requests []struct {
            Prompt    string `json:"prompt"`
            Isolation string `json:"isolation"`
        } `json:"requests"`
    }
    if err := json.Unmarshal([]byte(input), &in); err != nil {
        return &ToolResult{Error: err.Error()}, nil
    }
    if len(in.Requests) == 0 || len(in.Requests) > 5 {
        return &ToolResult{Error: "free_fork: requests count must be in [1,5]"}, nil
    }
    sessionID := toolrunner.ToolSessionIDFromContext(ctx)
    reqs := make([]freefork.ForkRequest, len(in.Requests))
    for i, r := range in.Requests {
        reqs[i] = freefork.ForkRequest{
            Prompt:      r.Prompt,
            Isolation:   r.Isolation,
            WorkDir:     workDir,
        }
    }
    handles, err := r.forker.Fork(ctx, sessionID, reqs)
    if err != nil {
        return &ToolResult{Error: fmt.Sprintf("free_fork: %v", err)}, nil
    }
    out, _ := json.Marshal(map[string]any{
        "spawned_count": len(handles),
        "agent_ids":     extractAgentIDs(handles),
    })
    return &ToolResult{Output: string(out)}, nil
}

// Bootstrap 注入: 需要 global forker 实例, 由 multiagent bootstrap 创建
var GlobalForker freefork.Forker

func SetGlobalForker(f freefork.Forker) { GlobalForker = f }
```

**Bootstrap 注入点** (`multiagent bootstrap`):
```go
// internal/bootstrap/multiagent.go (新建或扩展)
forker := freefork.NewDefaultForker(freefork.ForkerDeps{
    Factory:  agentFactory,
    Worktree: worktree.NewManager(),
})
toolrunner.SetGlobalForker(forker)
```

#### 2.2.3 G6 Tracker (D5-S23-A02)

```go
// 文件: internal/layers/contextengine/enforce/toolrunner/tracker_tool.go
type trackerQueryRunner struct {
    tracker *tracker.Tracker
}

func (r *trackerQueryRunner) Name() string { return "query_diagnostics" }

func (r *trackerQueryRunner) Schema() ToolSchema {
    return ToolSchema{
        Name: "query_diagnostics",
        Description: "Query current file diagnostics diff (new errors since last edit)",
        Parameters: `{"type":"object","properties":{"file":{"type":"string"}}}`,
    }
}

func (r *trackerQueryRunner) Execute(ctx context.Context, workDir, input string) (*ToolResult, error) {
    var in struct{ File string `json:"file"` }
    json.Unmarshal([]byte(input), &in)
    diff := r.tracker.Diff(in.File)
    out, _ := json.Marshal(diff)
    return &ToolResult{Output: string(out)}, nil
}

// 文件: internal/layers/observability/diagnose/tracker/wire.go (新建)
package tracker

var GlobalTracker *Tracker

func SetGlobalTracker(t *Tracker) { GlobalTracker = t }

func TickOnce(ctx context.Context) {
    if GlobalTracker == nil { return }
    GlobalTracker.Tick(ctx)
}

// 文件: internal/layers/contextengine/enforce/toolrunner/tracker_register.go
func RegisterTrackerTool(reg *ToolRegistry, t *tracker.Tracker) error {
    if t != nil {
        tracker.SetGlobalTracker(t)
    }
    return reg.Register(&trackerQueryRunner{tracker: t})
}
```

**Bootstrap 启动 tick goroutine**:
```go
// internal/bootstrap/context_engine_builder.go, buildWithGate() 末尾
tracker := tracker.NewTracker(500)  // 500-file LRU
toolrunner.RegisterTrackerTool(toolReg, tracker)
go func() {
    ticker := time.NewTicker(1 * time.Second)
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            tracker.TickOnce(ctx)
        }
    }
}()
```

#### 2.2.4 A5 WindowAnalyzer (D2-S6-A03)

LLM tool 可选（用户更可能用 CLI `/context`）；若实现，走与 verify 相同 pattern：
```go
func (r *windowanalyzerRunner) Name() string { return "analyze_window" }
// 调用 windowanalyzer.NewTokenAnalyzer().AnalyzeMessages()
```

### 2.3 Level 2: CLI Slash Command 暴露（L2，2 项）

#### 2.3.1 A1 /doctor (D5-S23-A03)

```go
// 文件: internal/cli/doctor/doctor.go (新建)
package doctor

type CLI struct{}

func (c *CLI) Name() string { return "doctor" }
func (c *CLI) Synopsis() string { return "Run 7 self-diagnostics checks" }

func (c *CLI) Run(args []string) error {
    workDir, _ := os.Getwd()
    devrixBin, _ := os.Executable()
    transcriptDir := config.FindConfigFile()  // 复用已有逻辑
    d := doctor.NewDefaultDoctor(workDir, devrixBin, transcriptDir, nil)
    checks := d.Run(context.Background())
    fmt.Println(doctor.FormatTable(checks))
    fmt.Println(doctor.FormatJSON(checks))
    return nil
}

// 文件: cmd/devrix/main.go (修改: 在 debug/eval 之后添加 doctor)
if len(os.Args) >= 2 && os.Args[1] == "doctor" {
    if err := (doctor.CLI{}).Run(os.Args[2:]); err != nil {
        os.Exit(1)
    }
    return
}
```

#### 2.3.2 A5 /context analyze (D2-S6-A03)

```go
// 文件: internal/cli/context_analyze/context_analyze.go (新建)
package context_analyze

type CLI struct{}

func (c *CLI) Run(args []string) error {
    workDir, _ := os.Getwd()
    sessionID := latestSession(workDir)  // 从 capture.FileSessionStore 取最新
    msgs := loadMessages(sessionID)
    b := windowanalyzer.NewTokenAnalyzer().AnalyzeMessages(msgs)
    fmt.Print(windowanalyzer.FormatTable(b))
    return nil
}
```

#### 2.3.3 IM Adapter Slash 路由扩展

```go
// 文件: internal/layers/communication/channel/adapters/cli.go
// 在 handleCommand() switch 中添加:
case types.CommandDoctor:
    return a.handleDoctor(ctx)
case types.CommandContextAnalyze:
    return a.handleContextAnalyze(ctx)

// types/command.go (新建或修改) 添加:
const (
    CommandNew CommandType = iota
    CommandStop
    CommandHelp
    CommandTask
    CommandPlan
    CommandDoctor        // NEW
    CommandContextAnalyze // NEW
)
```

**IM adapter 同步**：feishu adapter 需识别 `/doctor` `/context` 文本，转发到 cli adapter 的同一逻辑。

### 2.4 Level 3: 错误路径接入（L3，3 项）

#### 2.4.1 A6 ErrorClassifier → LLM 网关错误响应 (D3-S3-A02)

```go
// 文件: internal/layers/llmgateway/dispatch/invoke.go (修改)
func DispatchInvoke(ctx context.Context, ...) (*Response, error) {
    resp, err := raw.Invoke(ctx, ...)
    if err != nil {
        // 注入 classification 到 ctx (即使 ctx 即将 cancel)
        c := errorclass.NewDefaultClassifier().Classify(err, resp.HTTPStatus, string(resp.Body))
        ctx = errorclass.InjectClassification(ctx, c)
        // 渲染错误响应时带 class 标签
        return nil, fmt.Errorf("[class=%s] %w", c.Class, errors.WithShortStack(err))
    }
    return resp, nil
}
```

#### 2.4.2 A7 ShortStack → Sandbox 拒绝错误 (D2-S6-A02)

```go
// 文件: internal/layers/contextengine/enforce/toolrunner/sandbox.go (修改)
func (p *CommandPolicy) Validate(command string) error {
    if p == nil || !p.Enabled {
        return nil
    }
    if p.ASTAnalyzer != nil {
        if allow, reason := p.ASTAnalyzer.Analyze(command); !allow {
            wrapped := fmt.Errorf("sandbox: ast block: %s. %s", reason, sandboxPolicyHint)
            return errors.WithShortStack(wrapped)  // ← 过滤 runtime/testing 帧
        }
    }
    // ... existing logic, also wrap errors with WithShortStack
}
```

#### 2.4.3 A7 ShortStack → Agent lifecycle 错误

```go
// 文件: internal/layers/multiagent/agent/engine.go (修改)
func (e *Engine) spawnChild(ctx context.Context, ...) (*Handle, error) {
    handle, err := e.factory.Create(ctx, ...)
    if err != nil {
        return nil, errors.WithShortStack(fmt.Errorf("spawn child: %w", err))
    }
    // ...
}
```

### 2.5 Level 4: AST 注入 + DebugFilter（L4，2 项）

#### 2.5.1 G2 Bash AST 注入 (TOOL-SEC-2-A02)

```go
// 文件: internal/bootstrap/context_engine_builder.go (修改)
import "github.com/devrix/devrix/internal/layers/contextengine/enforce/toolrunner/sandboxast"

func (b *ContextEngineBuilder) buildWithGate(perm contracts.IPermissionGate) contracts.IEngine {
    // ... existing code ...
    
    execCfg := newToolExecConfig(b.toolCfg)
    // G2: 注入 ASTAnalyzer
    if b.toolCfg.Sandbox.ASTEnabled {  // 默认 true
        execCfg.policy.ASTAnalyzer = sandboxast.NewPolicyAnalyzer()
    }
    // ... rest of build
}
```

**新增 config 字段** (`internal/shared/config/tool_config.go`):
```go
type SandboxConfig struct {
    Enabled         bool     `yaml:"enabled"`
    AllowlistExtra  []string `yaml:"allowlist_extra"`
    DenyPatternsExtra []string `yaml:"deny_patterns_extra"`
    ASTEnabled      bool     `yaml:"ast_enabled"`  // NEW: 默认 true
}
```

#### 2.5.2 A2 DebugFilter 接入 (D5-S24-A02)

```go
// 文件: internal/bootstrap/observability.go (新建)
package bootstrap

import "github.com/devrix/devrix/internal/layers/observability/instrument/logger/debugfilter"

func InstallDebugFilter(categories []string) {
    if len(categories) == 0 { return }
    inner := slog.Default().Handler()
    filtered := debugfilter.New(inner, categories)
    slog.SetDefault(slog.New(filtered))
}
```

**CLI flag 接入** (`cmd/devrix/main.go`):
```go
// 在 os.Args 解析处:
debugCategories := parseDebugFlag(os.Args)
if len(debugCategories) > 0 {
    bootstrap.InstallDebugFilter(debugCategories)
}
```

### 2.6 Level 5: Transcript 持久化接入（L5，1 项）

#### 2.6.1 A3 Transcript OnSessionClose 钩子 (D1-S2-A02)

```go
// 文件: internal/layers/communication/capture/session_store.go (修改)
func (s *FileSessionStore) Close(ctx context.Context, sessionID string) error {
    // 现有 close 逻辑
    if err := s.existingClose(ctx, sessionID); err != nil {
        return err
    }
    // NEW: 写 transcript
    if transcript.GlobalWriter != nil {
        events := s.readEventsForTranscript(ctx, sessionID)
        for _, e := range events {
            transcript.GlobalWriter.Append(sessionID, e)
        }
        slog.Info("transcript written", "session", sessionID, "events", len(events))
    }
    return nil
}

// 文件: internal/layers/communication/capture/transcript/wire.go (新建)
package transcript

var GlobalWriter *Writer

func SetGlobalWriter(w *Writer) { GlobalWriter = w }
```

**Bootstrap 注入**:
```go
// internal/bootstrap/context_engine_builder.go (or session_store bootstrap)
transcriptDir := filepath.Join(configDir, "transcripts")
tw, _ := transcript.NewWriter(transcriptDir)
transcript.SetGlobalWriter(tw)
```

### 2.7 Level 6: Notify Consume 接入（L6，1 项）

#### 2.7.1 G3 Task Notify consume → prompt assembler (D4-S12-A03)

```go
// 文件: internal/layers/contextengine/prepare/prompt/assembler.go (修改)
func (a *Assembler) AssembleReminder(ctx context.Context, ...) string {
    var sb strings.Builder
    sb.WriteString(a.assembleExistingReminder(ctx, ...))
    
    // NEW: drain notify bus
    if bus := notify.GlobalBus(); bus != nil {
        sessionID := sessionIDFromContext(ctx)
        events := bus.Drain(sessionID)
        if len(events) > 0 {
            block := notify.FormatReminder(events)
            sb.WriteString("\n<task_notifications>\n")
            sb.WriteString(block)
            sb.WriteString("\n</task_notifications>\n")
        }
    }
    
    return sb.String()
}
```

### 2.8 接入点汇总表

| Level | Activity | Alias | 接入文件 | 改动类型 |
|-------|----------|-------|----------|---------|
| L1 | D6-S11-A02 | G4 | `toolrunner/verify_tool.go` + `verify_register.go` | 新增 |
| L1 | D4-S11-A02 + D4-S13-A02 | G5 | `toolrunner/freefork_tool.go` + `freefork_register.go` | 新增 |
| L1 | D5-S23-A02 | G6 | `toolrunner/tracker_tool.go` + `tracker_register.go` + `tracker/wire.go` | 新增 |
| L2 | D5-S23-A03 | A1 | `cli/doctor/doctor.go` + `cmd/devrix/main.go` + `cli.go` 路由 | 新增 + 修 |
| L2 | D2-S6-A03 | A5 | `cli/context_analyze/context_analyze.go` + 同上 | 新增 + 修 |
| L3 | D3-S3-A02 | A6 | `llmgateway/dispatch/invoke.go` | 修改 |
| L3 | D2-S6-A02 | A7 | `toolrunner/sandbox.go` + `multiagent/agent/engine.go` | 修改 |
| L4 | TOOL-SEC-2-A02 | G2 | `bootstrap/context_engine_builder.go` + `tool_config.go` | 修改 |
| L4 | D5-S24-A02 | A2 | `bootstrap/observability.go` + `cmd/devrix/main.go` | 新增 + 修 |
| L5 | D1-S2-A02 | A3 | `capture/session_store.go` + `transcript/wire.go` | 修改 + 新增 |
| L6 | D4-S12-A03 | G3 | `prepare/prompt/assembler.go` | 修改 |

## 3. Key Interfaces / Types

### 3.1 新增 Tool 接口

```go
// internal/layers/contextengine/enforce/toolrunner/contracts.go (沿用)
// 所有新增 tool 必须实现:
type PluginRunner interface {
    Name() string
    Schema() ToolSchema
    RiskLevel() types.RiskLevel
    Execute(ctx context.Context, workDir, input string) (*ToolResult, error)
}
```

### 3.2 全局单例接口

```go
// internal/layers/observability/diagnose/tracker/wire.go
var GlobalTracker *Tracker
func SetGlobalTracker(t *Tracker)

// internal/layers/communication/capture/transcript/wire.go
var GlobalWriter *Writer
func SetGlobalWriter(w *Writer)

// internal/layers/contextengine/enforce/toolrunner/freefork_register.go
var GlobalForker freefork.Forker
func SetGlobalForker(f freefork.Forker)

// 已有: notify.GlobalBus() (workmodel/notify/wire.go)
```

### 3.3 Config 扩展

```go
// internal/shared/config/tool_config.go (新增字段)
type SandboxConfig struct {
    Enabled           bool     `yaml:"enabled"`
    AllowlistExtra    []string `yaml:"allowlist_extra"`
    DenyPatternsExtra []string `yaml:"deny_patterns_extra"`
    ASTEnabled        bool     `yaml:"ast_enabled"`  // NEW
}

// internal/shared/config/contextengine.go (新增)
type DiagnosticsConfig struct {
    TrackerLRUCapacity int      `yaml:"tracker_lru_capacity"`  // 默认 500
    LSPEnabled         bool     `yaml:"lsp_enabled"`            // 默认 false
    LSPServers         []string `yaml:"lsp_servers"`            // 默认空
    DebugCategories    []string `yaml:"debug_categories"`       // 默认空
    TranscriptDir      string   `yaml:"transcript_dir"`         // 默认 ~/.devrix/transcripts
}
```

## 4. Data Flow

### 4.1 IM 触发 A1 /doctor (L2 路径)

```
飞书 IM 用户发 "/doctor"
  → feishu adapter 接收
  → capture.CommunicationGateway.RouteInbound
  → D7 ingress (CommandFirst 路由)
  → 识别为 CommandDoctor
  → adapters/cli.go handleDoctor(ctx)
  → doctor.NewDefaultDoctor(workDir, devrixBin, transcriptDir, nil).Run(ctx)
  → 7 项 Check 顺序执行
  → doctor.FormatTable(checks) 渲染为字符串
  → capture.RenderOutbound → feishu adapter
  → 飞书 IM 收到回复（含 PASS/FAIL/WARN 表）
```

### 4.2 LLM 调用 G5 free_fork (L1 路径)

```
LLM 收到 prompt "想从 3 个方向并行调查 X"
  → LLM 决定调用 free_fork tool
  → toolrunner.Execute("free_fork", {requests: [...]})
  → freeForkRunner.Execute(ctx, workDir, input)
  → ctx 取出 sessionID (来自 ToolSessionIDFromContext)
  → GlobalForker.Fork(ctx, sessionID, reqs)
  → DefaultForker.spawnOne × N (并行 + worktree 隔离)
  → 返回 []Handle
  → JSON marshal 给 LLM
  → LLM 看到 agent_ids, 可继续 SendMessage
```

### 4.3 A6 ErrorClassify 注入 (L3 路径)

```
LLM 网关调用 provider API → HTTP 401
  → llmgateway/dispatch/invoke.go DispatchInvoke
  → raw.Invoke 返回 (resp, err) err != nil
  → errorclass.NewDefaultClassifier().Classify(err, 401, body)
  → 返回 Classification{Class: AuthRequired, Hint: "..."}
  → errorclass.InjectClassification(ctx, c)
  → errors.WithShortStack(fmt.Errorf("[class=AuthRequired] %w", err))
  → capture.RenderError
  → 飞书 IM 收到 "[class=AuthRequired] Invalid API key (filtered stack)"
```

### 4.4 G3 Notify Consume (L6 路径)

```
用户通过 /task create 添加任务
  → workmodel.TaskManager.UpdateStatus(task, completed)
  → task_manager.go:218 publishCompletion
  → notify.GlobalBus().Publish(sessionID, CompletionEvent{...})
  → event 进入 channel (或 overflow)
  
下一个 LLM 请求到来
  → prepare/prompt/assembler.go AssembleReminder(ctx, ...)
  → bus.Drain(sessionID) 返回所有 pending events
  → notify.FormatReminder(events) 渲染为 XML
  → 注入到 <reminder> 段
  → system prompt 包含 <task_notifications>...</task_notifications>
  → LLM 看到 task 状态变化
```

## 5. File Manifest

### 5.1 新增文件（约 14 个）

| 文件 | 行数估算 | Activity |
|------|---------|----------|
| `internal/layers/contextengine/enforce/toolrunner/verify_tool.go` | 60 | G4 |
| `internal/layers/contextengine/enforce/toolrunner/verify_register.go` | 15 | G4 |
| `internal/layers/contextengine/enforce/toolrunner/freefork_tool.go` | 80 | G5 |
| `internal/layers/contextengine/enforce/toolrunner/freefork_register.go` | 25 | G5 |
| `internal/layers/contextengine/enforce/toolrunner/tracker_tool.go` | 50 | G6 |
| `internal/layers/contextengine/enforce/toolrunner/tracker_register.go` | 25 | G6 |
| `internal/layers/observability/diagnose/tracker/wire.go` | 20 | G6 |
| `internal/cli/doctor/doctor.go` | 80 | A1 |
| `internal/cli/context_analyze/context_analyze.go` | 60 | A5 |
| `internal/layers/communication/capture/transcript/wire.go` | 15 | A3 |
| `internal/bootstrap/observability.go` | 30 | A2 |
| `internal/layers/communication/channel/adapters/cli_commands.go` (slash 注册) | 40 | A1/A5 |
| `openspec/changes/devrix-diagnostic-tools-wiring/specs/diagnostic-tools-wiring/spec.md` | 400 | ALL |
| `openspec/changes/devrix-diagnostic-tools-wiring/tasks.md` | 200 | ALL |
| **小计** | **~1100 行** | |

### 5.2 修改文件（约 8 个）

| 文件 | 改动行数 | Activity |
|------|---------|----------|
| `cmd/devrix/main.go` | +20 | A1/A5/A2 入口 |
| `internal/bootstrap/context_engine_builder.go` | +60 | G2/G4/G5/G6 注册 |
| `internal/layers/communication/channel/adapters/cli.go` | +40 | A1/A5 路由 |
| `internal/layers/communication/capture/session_store.go` | +20 | A3 |
| `internal/layers/contextengine/prepare/prompt/assembler.go` | +20 | G3 |
| `internal/layers/llmgateway/dispatch/invoke.go` | +15 | A6 |
| `internal/layers/contextengine/enforce/toolrunner/sandbox.go` | +10 | A7 |
| `internal/layers/multiagent/agent/engine.go` | +10 | A7 |
| `internal/shared/config/tool_config.go` | +5 | G2 |
| `internal/shared/config/contextengine.go` | +15 | ALL |
| **小计** | **~215 行** | |

### 5.3 不变更

- DM-016 全部 21 个 library 文件
- D2 QueryLoop / D7 Turn 编排主路径
- D3 LLM Gateway 现有熔断/重试/超时逻辑
- D1 Communication 现有协议层
- A4 FaultInject 的 build-tag 行为（AC13 锁定 P2）

## 6. Regression Risk Assessment

| 改动 | 风险 | 缓解 |
|------|------|------|
| `cmd/devrix/main.go` 增加 doctor / context 子命令 | 与 debug/eval 子命令解析冲突 | flag 严格匹配 `os.Args[1] == "doctor"`；fallback 到原行为 |
| `cli.go` 增加 slash 路由 | 现有 /task /plan 路由破坏 | 在 switch default 之前添加；不修改现有 case |
| `bootstrap/context_engine_builder.go` 注册新 tool | 与现有 tool 命名冲突 | 每个 tool name 唯一 (`verify_plan_execution` / `free_fork` / `query_diagnostics` / `analyze_window`)；Register 返回 err 时 log + skip |
| `sandbox.go` 注入 errors.WithShortStack | 现有测试断言错误字符串时失败 | 短栈格式 `msg\nstack\n`；现有 sandbox_test.go 检查 err.Error() 含子串即可兼容 |
| `invoke.go` 注入 InjectClassification | LLM 网关返回错误时多 100ns 开销 | NewDefaultClassifier 是 sync.Once 注册 + map lookup；可忽略 |
| `session_store.go` 增加 transcript 写 | session close 慢 50ms | 异步写：sync.WaitGroup + goroutine |
| `assembler.go` drain notify bus | system prompt 变长 | notify FormatReminder 限制最多 5 个 event |
| 新增 tool (4 个) | LLM 误用工具 | tool description 详细；RiskLevel=Low；execute 内部限速 |
| tracker tick goroutine | 启动 goroutine 泄漏 | ctx 控制 + WaitGroup + shutdown hook |

### 6.1 性能影响

| 改动 | P50 影响 | P99 影响 |
|------|---------|---------|
| tracker tick 1s | 0 (异步) | +5ms per tick |
| InjectClassification | +50ns | +200ns |
| WithShortStack | +5μs | +20μs (过滤 runtime 帧) |
| transcript write | +30ms/close | +80ms/close (异步后 0) |
| notify drain | +10μs/reminder | +50μs |
| 4 个新 tool schema 渲染 | +200μs | +500μs (ListTools 一次性) |

### 6.2 layer-lint 影响

- `bootstrap/context_engine_builder.go` 引用 `sandboxast`、`tracker`、`freefork` — **已合规**（bootstrap 是合法依赖容器）
- `toolrunner/verify_tool.go` 引用 `verify` (`evolution/`) — 跨域，但 toolrunner 是 D2 核心，`evolution/verify` 是 D6 支撑；需检查是否有 layer-lint 规则禁止 D2→D6。如有，需把 verify 改为 bridge 模式
- `toolrunner/freefork_tool.go` 引用 `freefork` (`multiagent/`) — D2→D4 跨域；同上
- `toolrunner/tracker_tool.go` 引用 `tracker` (`observability/`) — D2→D5；同上

**对策**：在 layer-lint 检查时，如果禁止跨域调用，则改为：
- 在 `internal/shared/contracts/` 添加 `DiagnosticsHook` 接口
- `evolution/verify`、`multiagent/freefork`、`observability/tracker` 实现该接口
- bootstrap 把实现注入到 toolrunner

详细方案见 §7 Rollback。

## 7. Rollback Plan

每个 wiring 独立可回滚：

| Wiring | 回滚命令 |
|--------|---------|
| G4/G5/G6 新 tool | revert `bootstrap/context_engine_builder.go` 的 Register 行 |
| A1/A5 CLI 子命令 | revert `cmd/devrix/main.go` + delete `cli/doctor/` `cli/context_analyze/` |
| A6 ErrorClassify | revert `invoke.go` 的 4 行修改 |
| A7 ShortStack | revert `sandbox.go` 和 `agent/engine.go` 的 errors.WithShortStack 行 |
| G2 AST 注入 | `toolCfg.Sandbox.ASTEnabled = false`（无需 revert） |
| A2 DebugFilter | 不传 `--debug=` flag（无需 revert） |
| A3 Transcript | revert `session_store.go` + delete `transcript/wire.go` |
| G3 Notify consume | revert `assembler.go` 的 drain 段 |

**Layer-lint 不合规的应急方案**：
- 把 `verify/freefork/tracker` 包移到 `internal/bridges/diagnostics/`（横切桥接）
- toolrunner 仅依赖 `bridges/diagnostics` 接口
- 三方实现注入到接口
- 详细设计推迟到 S4-Gate 之后视 layer-lint 实测结果决定

## 8. Open Questions

1. **A4 FaultInject 推迟到 P2** — 是否同意保留 build-tag 隔离，不引入 IM 注入？
2. **A5 WindowAnalyzer 是否同时暴露 LLM tool** — L1 + L2 双接入 vs 仅 L2？
3. **tracker tick goroutine 频率** — 1s vs 用户可配置？
4. **transcript 与 session_store 是否要合并** — 共存 6 个月观察 vs 立即合并？

---

**S3 完成，下一步 S3-Gate 自审 + S4 实现。**