# Design: 诊断工具能力差距闭环 — 对齐 clawcode (Claude Code v2.1.88)

**Change ID:** devrix-diagnostic-tools-parity
**Demand ID:** DM-20260616-003
**Status:** S3_Design
**Version:** v1.0（一次性闭环，覆盖原 v1.1/v1.2/v1.3 合并实施）
**Date:** 2026-06-17

---

> **能力别名前缀 (Capability Aliases)**
>
> 本 change 遵循 DSAFT 域-场景-活动-功能-任务五层命名作为权威 ID。G1-G6 / A1-A7 是 S2 阶段为方便对照 `docs/reference/clawcode-diagnostic-tools-analysis.md` 而保留的需求侧别名前缀。一一映射：
>
> | DSAFT Activity | Alias | 域 | 能力 |
> |----------------|-------|----|------|
> | D1-S2-A02-PersistTranscript | A3 | D1 | 会话转录持久化 |
> | D2-S4-A01-ToolRegister | G1 | D2 | LSP 代码智能工具 |
> | D2-S6-A02-TruncateError | A7 | D2 | 共享错误栈截断 |
> | D2-S6-A03-AnalyzeWindow | A5 | D2 | 上下文窗口分析 |
> | D3-S3-A02-ErrorMapping | A6 | D3 | LLM 错误分类 |
> | D4-S11-A02-ForkAgent | G5 | D4 | 自由分叉子代理 |
> | D4-S12-A03-NotifyChild | G3 | D4 | 后台任务完成通知 |
> | D4-S13-A02-IsolateWorktree | G5 | D4 | (G5 worktree 隔离子能力) |
> | D5-S23-A02-TrackDiagnostics | G6 | D5 | 诊断跟踪器 |
> | D5-S23-A03-RunDoctor | A1 | D5 | /doctor 自检命令 |
> | D5-S23-A04-FaultInject | A4 | D5 | 故障注入 |
> | D5-S24-A02-ConfigureDebugFilter | A2 | D5 | Debug 日志分类过滤 |
> | D6-S11-A02-VerifyPlanExec | G4 | D6 | 实现后自动验证 |
> | TOOL-SEC-2-A02-ShellASTPolicy | G2 | tool-security | Bash AST 安全分析器 |

---

## 0. 范围调整说明

原 proposal §3.1 计划 "umbrella + 3 sub-change" 拆分。**本次按用户指令调整为单 change 一次性闭环**：
全部 13 项能力（6 项核心差距 + 7 项附加诊断特性，见 alias 表）在本 change 内完成 S3→S6，**实现能力对标 clawcode**，
不输出 sub-change DM ID。

`.openspec.yaml` 中的 `version_scope` 字段保留作为历史决策痕迹，但不再生成 v1.1/v1.2/v1.3 子目录。

---

## 1. 总体架构

### 1.1 域归属（与 proposal §3.2 一致）

| DSAFT Activity | Alias | 主域 | 实现包 |
|----------------|-------|------|--------|
| D2-S4-A01-ToolRegister | G1 | D2 Context Engine | `internal/layers/contextengine/enforce/toolrunner/lsp/` + `internal/shared/lsp/` |
| TOOL-SEC-2-A02-ShellASTPolicy | G2 | tool-security | `internal/layers/contextengine/enforce/toolrunner/sandboxast/` |
| D4-S12-A03-NotifyChild | G3 | D4 Multi-Agent | `internal/layers/orchestration/workmodel/notify/` |
| D6-S11-A02-VerifyPlanExec | G4 | D6 Evolution | `internal/layers/evolution/verify/` |
| D4-S11-A02-ForkAgent + D4-S13-A02-IsolateWorktree | G5 | D4 Multi-Agent | `internal/layers/multiagent/provision/freefork/` |
| D5-S23-A02-TrackDiagnostics | G6 | D5 Observability | `internal/layers/observability/diagnose/tracker/` |
| D5-S23-A03-RunDoctor | A1 | D5 Observability | `internal/layers/observability/diagnose/doctor/` |
| D5-S24-A02-ConfigureDebugFilter | A2 | D5 Observability | `internal/layers/observability/instrument/logger/debugfilter/` |
| D1-S2-A02-PersistTranscript | A3 | D1 Communication | `internal/layers/communication/capture/transcript/` |
| D5-S23-A04-FaultInject | A4 | D5 Observability | `internal/layers/observability/diagnose/faultinject/` |
| D2-S6-A03-AnalyzeWindow | A5 | D2 Context Engine | `internal/layers/contextengine/token/windowanalyzer/` |
| D3-S3-A02-ErrorMapping | A6 | D3 LLM Gateway | `internal/layers/llmgateway/protect/errorclass/` |
| D2-S6-A02-TruncateError | A7 | shared (横切) | `internal/shared/errors/shortstack.go` |

> **遵循约束**：所有跨域调用经 `internal/shared/`；新增 package 不引入新的依赖环。

### 1.2 关键技术决策

| # | 决策 | 选项 | 选择 | 理由 |
|---|------|------|------|------|
| D1 | LSP 协议实现 | (a) 用 `go.lsp.dev/protocol`；(b) 手写 JSON-RPC 子集 | **(b)** | 仅需 3 个 op，手写 ~300 LOC 优于引入大依赖 |
| D2 | LSP server 管理 | (a) 复用 D1 sandbox；(b) 独立进程池 | **(a)** | 与 proposal §8 Decision 一致，沙箱统一 |
| D3 | 文件诊断 linter 来源 | (a) 复用 LSP server diagnostics；(b) 单独跑 `go vet`/`tsc` | **(b)** | linter 与 LSP 解耦；CI 与本地一致 |
| D4 | 文件诊断同步/异步 | (a) 异步 OnEditComplete；(b) 同步 block | **(a)** | proposal §8 Decision 已选 |
| D5 | Bash AST 解析器 | (a) `mvdan.cc/sh/v3/syntax` 纯 Go；(b) Tree-sitter CGO | **(a)** | proposal §6 风险表已选纯 Go |
| D6 | 错误分类匹配方式 | (a) 字符串前缀；(b) 正则 + HTTP 码组合 | **(b)** | 覆盖 anthropic/openai 多种格式 |
| D7 | Free-fork 隔离 | (a) worktree 默认开（D4-S13-A02）；(b) 进程级隔离 | **(a)** | demand §6 已要求 worktree 默认 |
| D8 | /doctor 输出格式 | (a) 仅 JSON；(b) JSON + table | **(b)** | CLI 友好 + CI 友好 |
| D9 | D5-S24-A02 debug 过滤实现 | (a) slog handler 包装；(b) 全局开关 | **(a)** | slog 原生 Attr 路由，零侵入 |
| D10 | 转录格式 | (a) JSONL；(b) SQLite | **(a)** | clawcode 一致；`--continue` 易实现 |

### 1.3 注册路径汇总

新增 tool / 钩子的注册点（按 DSAFT 活动 ID 标注）：

```
internal/bootstrap/context_engine_builder.go
  └─ buildWithGate(): toolReg.Register(...)
       新增: registerLSPTool(...)             # D2-S4-A01
       新增: registerDiagnosticTrackerHook()  # D5-S23-A02 在 edit/write 后调用

internal/bootstrap/wire_observability.go
  └─ 新增: WireDoctorCommand()                # D5-S23-A03
  └─ 新增: WireDebugFilter()                  # D5-S24-A02
  └─ 新增: WireFaultInject()                  # D5-S23-A04

internal/bootstrap/wire_d1.go / capture.go
  └─ 新增: WireTranscript()                   # D1-S2-A02

internal/bootstrap/wire_llm.go
  └─ 新增: WireErrorClassifier()              # D3-S3-A02

internal/bootstrap/wire_multiagent.go
  └─ 新增: WireFreeFork()                     # D4-S11-A02 + D4-S13-A02

internal/bootstrap/wire_orchestration.go
  └─ 新增: WireTaskNotify()                   # D4-S12-A03

internal/bootstrap/cli.go
  └─ 新增: /doctor 命令路由                   # D5-S23-A03
  └─ 新增: --debug=cat1,cat2 flag             # D5-S24-A02
  └─ 新增: --continue flag                    # D1-S2-A02
```

---

## 2. 各能力详细设计

### 2.1 D2-S4-A01 — LSP 代码智能工具 (alias G1)

**对标**: `clawcode/src/tools/LSPTool/` + `clawcode/src/services/lsp/`

#### 2.1.1 公共接口

新增 `internal/shared/lsp/` 包：

```go
package lsp

// Client 抽象 LSP server 的 JSON-RPC 客户端，由 manager 提供生命周期管理。
type Client interface {
    Initialize(ctx context.Context, rootURI string) error
    DidOpen(ctx context.Context, uri, languageID, text string) error
    Definition(ctx context.Context, p Position) ([]Location, error)
    References(ctx context.Context, p Position) ([]Location, error)
    PrepareCallHierarchy(ctx context.Context, p Position) ([]CallHierarchyItem, error)
    IncomingCalls(ctx context.Context, item CallHierarchyItem) ([]CallHierarchyIncomingCall, error)
    Close() error
}

type Position struct {
    URI       string // file://...
    Line      uint32 // 0-based per LSP spec
    Character uint32 // 0-based per LSP spec (UTF-16 code units)
}

type Location struct {
    URI       string
    Range     Range
    Preview   string // 上下文 1-3 行
}

type Range struct {
    Start, End Position
}

type CallHierarchyItem struct {
    Name     string
    Kind     SymbolKind
    URI      string
    Range    Range
    Selection Range
}

type CallHierarchyIncomingCall struct {
    From  CallHierarchyItem
    Lines []uint32 // 调用方代码所在行
}
```

#### 2.1.2 Manager + Server 进程池

```go
// internal/shared/lsp/manager.go
type Manager struct {
    cap      int                       // 最大并发 server 数（默认 4）
    mu       sync.Mutex
    clients  map[string]*entry         // key = languageID + workspace root
    sandbox  SandboxLauncher           // 复用 D1 sandbox
    lru      *list.List                // 双向链表实现 LRU 淘汰
}

func (m *Manager) Acquire(ctx context.Context, langID, root string) (Client, error)
func (m *Manager) Shutdown() error
```

LSP server 通过 sandbox 启动：

```go
// SandboxLauncher 抽象 D1 sandbox.Launch
type SandboxLauncher interface {
    Launch(ctx context.Context, argv []string, env []string) (Process, error)
}
```

**支持的语言**:
- `go` → `gopls`
- `typescript` / `javascript` → `typescript-language-server`（globally installed; fallback to `tsserver`）

#### 2.1.3 JSON-RPC 客户端

`internal/shared/lsp/rpc.go`：手写 LSP base protocol（Content-Length header + JSON body），仅实现需要的请求/通知：
- `initialize`、`initialized`、`shutdown`、`exit`
- `textDocument/didOpen`
- `textDocument/definition`
- `textDocument/references`
- `textDocument/prepareCallHierarchy`
- `callHierarchy/incomingCalls`

无需双向通知（log/progress 丢弃）。

#### 2.1.4 Tool Plugin（D2 ToolPool 入口）

`internal/layers/contextengine/enforce/toolrunner/lsp_tool.go`：

```go
type lspRunner struct {
    mgr *lsp.Manager
    fmt *lspFormatter
}

func (r *lspRunner) Name() string { return "lsp" }
func (r *lspRunner) Schema() ToolSchema {
    return ToolSchema{
        Name:        "lsp",
        Description: "LSP code intelligence. operations: definition | references | incoming_calls",
        Parameters:  lspToolJSONSchema,
    }
}
func (r *lspRunner) RiskLevel() types.RiskLevel { return types.RiskLevelLow }
func (r *lspRunner) Execute(ctx context.Context, workDir, input string) (*ToolResult, error)
```

输入 schema（精简，但与 clawcode `operation` 字段对齐）:

```json
{
  "type": "object",
  "required": ["operation", "file_path", "line", "character"],
  "properties": {
    "operation": {"enum": ["definition", "references", "incoming_calls"]},
    "file_path": {"type": "string"},
    "line": {"type": "integer", "minimum": 1, "description": "1-based"},
    "character": {"type": "integer", "minimum": 1, "description": "1-based"}
  }
}
```

输出格式化（参考 clawcode `formatters.ts`）：每条结果包含 `file:line:col` + 1-3 行上下文，截断超长输出。

#### 2.1.5 配置

`internal/shared/config/tool_config.go` 新增 `LSPConfig`：

```go
type LSPConfig struct {
    Enabled         bool          `yaml:"enabled"`           // 默认 false（v1.0 opt-in）
    MaxServers      int           `yaml:"max_servers"`       // 默认 4
    InitTimeoutSecs int           `yaml:"init_timeout_secs"` // 默认 30
    RequestTimeoutSecs int        `yaml:"request_timeout_secs"` // 默认 10
    Servers         []LSPServer   `yaml:"servers"`
}

type LSPServer struct {
    LanguageID string   `yaml:"language_id"`
    Command    []string `yaml:"command"` // e.g. ["gopls", "serve"]
    FilePatterns []string `yaml:"file_patterns"` // e.g. ["*.go"]
}
```

### 2.2 D5-S23-A02 — 文件诊断追踪 (alias G6)

**对标**: `clawcode/src/services/diagnosticTracking.ts` + `LSPDiagnosticRegistry.ts`

#### 2.2.1 设计

`internal/layers/observability/diagnose/tracker/`：

```go
type Tracker struct {
    mu       sync.Mutex
    cap      int                       // 500（LRU 上限）
    lru      *list.List
    by       map[string]*list.Element
    linter   LinterFunc                // 由语言注入（go vet、tsc --noEmit）
}

type LinterFunc func(ctx context.Context, file string) ([]Diagnostic, error)

type Diagnostic struct {
    File     string
    Line     int
    Severity string // "error"|"warning"|"info"
    Message  string
    Source   string // 工具名（go-vet, tsc, ...）
}

// SnapshotBefore 在编辑前缓存快照；Diff 在编辑后产出新增 diagnostic 列表。
func (t *Tracker) SnapshotBefore(ctx context.Context, file string) error
func (t *Tracker) Diff(ctx context.Context, file string) ([]Diagnostic, error)
```

#### 2.2.2 集成点

`enforce/toolrunner/edit_tool.go` 和 `write_file_tool.go` 在 Execute 完成前后调用 Tracker：

```go
func (r *editFileRunner) Execute(ctx, workDir, input) (*ToolResult, error) {
    // 1. 解析参数 → target file
    // 2. tracker.SnapshotBefore(ctx, target)   // 不阻塞失败
    // 3. 写文件
    // 4. result := ...
    // 5. 异步: go func() { diags := tracker.Diff(...); attach to ToolResult.Extra }()
    //    — 实际由 ToolResult 加 DiagnosticsAsync chan，或在下一回合 prepare 阶段附加
}
```

异步策略：编辑主路径返回时**不等**linter；linter 结果通过 `sessionDiagnosticAppender`（D2 prepareTurn 钩子）在下次请求 LLM 前 attach 到 system reminder。

#### 2.2.3 默认 Linter

| 语言 | 命令 | 解析 |
|------|------|------|
| Go | `go vet -json ./pkg/...` 或单文件 `go vet ./<dir>` | 标准 vet 输出 |
| TypeScript | `tsc --noEmit --pretty false` | `path(L,C): error TS_NNNN: msg` |
| Shell | `shellcheck -f json <file>` (如可用) | JSON 数组 |

LinterFunc 注册表按文件扩展名路由；无匹配 linter 时 `Diff` 返回空。

#### 2.2.4 LRU 去重

500 文件容量，超出时淘汰最久未访问的快照；同 file 多次 SnapshotBefore 替换旧值。

### 2.3 D3-S3-A02 — 错误分类引擎 (alias A6)

**对标**: clawcode 错误分类（散落于 `utils/errors`）+ devrix 现有 sentinel。

#### 2.3.1 设计

`internal/layers/llmgateway/protect/errorclass/classifier.go`：

```go
type Class string

const (
    ClassRateLimit       Class = "rate_limit"
    ClassQuota           Class = "quota_exceeded"
    ClassAuth            Class = "auth_failed"
    ClassPermission      Class = "permission_denied"
    ClassNotFound        Class = "model_not_found"
    ClassInvalidRequest  Class = "invalid_request"
    ClassContextOverflow Class = "context_overflow"
    ClassTimeout         Class = "timeout"
    ClassNetwork         Class = "network"
    ClassUpstreamDown    Class = "upstream_unavailable"
    ClassOverloaded      Class = "overloaded"
    ClassContentFilter   Class = "content_filter"
    ClassToolUseError    Class = "tool_use_error"
    ClassPromptTooLong   Class = "prompt_too_long"
    ClassResponseTooLong Class = "response_too_long"
    ClassStreamError     Class = "stream_error"
    ClassParseError      Class = "parse_error"
    ClassCircuitOpen     Class = "circuit_open"
    ClassBudgetExceeded  Class = "budget_exceeded"
    ClassCancelled       Class = "cancelled"
    ClassUnknown         Class = "unknown"
)

type Classification struct {
    Class    Class
    Retry    bool
    UserHint string // 一句可操作提示
    Detail   string // 原始 error message tail
}

type Classifier interface {
    Classify(err error, httpStatus int, raw string) Classification
}
```

实现：基于 (a) `errors.Is` 检测既有 sentinel、(b) HTTP 状态码、(c) 正则匹配 provider 错误 body 三层。

#### 2.3.2 接入点

`internal/layers/llmgateway/protect/` 现有重试/熔断逻辑在产生 error 时调用 `Classifier.Classify(...)`，将结果存入 ctx (`errorclass.WithClassification(ctx, c)`)。Span attribute 附加 `error.class`。

### 2.4 D2-S6-A02 — 堆栈截断 (alias A7)

`internal/shared/errors/shortstack.go`：

```go
// ShortStack 截取调用栈前 N 帧，去掉 runtime/testing 噪声。
func ShortStack(err error, maxFrames int) string

// WithShortStack 在错误包装链上挂上截短的栈，可被 fmt.Sprintf("%+v", err) 渲染。
func WithShortStack(err error, maxFrames int) error
```

策略：`runtime.Callers` + 过滤 `runtime.`/`testing.`/`reflect.` 前缀帧；输出 `file:line func()` 一行一帧。

### 2.5 TOOL-SEC-2-A02 — Bash AST 安全分析 (alias G2)

**对标**: `clawcode/src/tools/BashTool/bashSecurity.ts`（2592 行）。

#### 2.5.1 依赖

新增 `mvdan.cc/sh/v3 v3.x.x`（纯 Go shell parser）。

#### 2.5.2 设计

`internal/layers/contextengine/enforce/toolrunner/sandboxast/analyzer.go`：

```go
type Analyzer struct {
    parser      *syntax.Parser
    forbidden   map[string]struct{}   // 默认禁用命令集合
    zshSurface  []*regexp.Regexp      // zmodload / sysopen / =cmd 等
}

type Verdict struct {
    Allow       bool
    Reason      string
    Findings    []Finding
}

type Finding struct {
    Kind     string  // "heredoc_injection" | "zsh_attack_surface" | "process_substitution" | ...
    Snippet  string  // 触发节点的源码片段
    Position syntax.Pos
}

func (a *Analyzer) Analyze(cmd string) Verdict
```

#### 2.5.3 检查项（与 clawcode 对齐）

| 检查项 | 实现 |
|--------|------|
| Heredoc body 单独审计 | 遍历 AST `*syntax.Redirect{Op: Hdoc/DashHdoc}`，对 body 递归 Analyze |
| Process substitution `<()`, `>()` | AST `*syntax.ProcSubst` |
| Command substitution `$()`, ` `` ` | AST `*syntax.CmdSubst` |
| Zsh 攻击面 | 正则 `\b(zmodload|sysopen|syswrite)\b`、`=\(.+\)`、`(\?[*])` 等 ≥20 模式 |
| Shebang 注入 | 检查 `#!` 行内引号嵌套 |
| 转义异常 | 引号深度 + 反斜杠链 |
| 危险重定向 | `>/dev/(s|h|null)`、`>&` 跨进程 |

返回 `Verdict.Allow=false` 时记录原因到 audit log；fallback 到现有 `CommandPolicy`（正则） 双层防护。

#### 2.5.4 接入

`enforce/toolrunner/sandbox.go` 现有 `CommandPolicy.Check` 增加 AST 前置：

```go
func (p *CommandPolicy) Check(cmd string) error {
    if a := getASTAnalyzer(); a != nil {
        v := a.Analyze(cmd)
        if !v.Allow {
            return fmt.Errorf("ast block: %s", v.Reason)
        }
    }
    // 现有正则层 fallback / 加固
    ...
}
```

### 2.6 D6-S11-A02 — 实现后自动验证 (alias G4)

**对标**: `clawcode/src/tools/VerifyPlanExecutionTool/`。

#### 2.6.1 设计

`internal/layers/evolution/verify/plan.go`：

```go
type PlanItem struct {
    ID       string // 如 "W1.1"
    Title    string
    Done     bool   // 来自 tasks.md 表格的 "done"|"pending" 字段
    Evidence []Evidence
}

type Evidence struct {
    Kind  string // "file"|"test"|"command"
    Path  string
    Match string
}

type Verifier interface {
    LoadPlan(taskFile string) ([]PlanItem, error)
    Verify(ctx context.Context, items []PlanItem, repoRoot string) (Report, error)
}

type Report struct {
    Total    int
    Verified int
    Unverified []UnverifiedItem
}

type UnverifiedItem struct {
    Item   PlanItem
    Reason string
}
```

#### 2.6.2 工作流

1. 解析 `tasks.md` 中 `| W{N}.{M} | desc | file | done|pending |` 表格
2. 对每条 Done 的 item：
   - 若有 `File:` 字段 → 检查文件存在且包含关键 token
   - 若有 `Test:` 字段 → 检查文件包含 `func Test...`
3. 输出 JSON 报告 `verification_report.json`
4. CLI：`devrix verify-plan <change-id>` 返回 exit 0/1

### 2.7 D4-S12-A03 — 后台任务完成通知 (alias G3)

**对标**: `clawcode/src/tools/TaskOutputTool/TaskOutputTool.tsx`。

#### 2.7.1 设计

`internal/layers/orchestration/workmodel/notify/`：

```go
type CompletionEvent struct {
    TaskID    string
    Kind      string // "bash"|"agent"|"remote"
    ExitCode  int
    Duration  time.Duration
    TailLines []string
    Error     string
}

type Bus interface {
    Subscribe(sessionID string) <-chan CompletionEvent
    Publish(sessionID string, evt CompletionEvent)
}
```

`workmodel.TaskManager` 在任务完成时调用 `bus.Publish(...)`。

`task_output` tool 在 `block=true` 时除轮询外，监听 `Subscribe` channel 提前返回。

下一回合 prepareTurn 阶段，附加未消费的 completion event 到 system reminder。

### 2.8 D4-S11-A02 + D4-S13-A02 — 自由分叉子代理 (alias G5)

**对标**: `clawcode/src/tools/AgentTool/ForkSubagent`。

#### 2.8.1 设计

`internal/layers/multiagent/provision/freefork/`：

```go
type ForkRequest struct {
    Name     string
    Prompt   string
    Worktree bool // 默认 true (D4-S13-A02)
}

type Forker interface {
    Fork(ctx context.Context, parentSession string, reqs []ForkRequest) ([]Handle, error)
}

type Handle struct {
    ChildSessionID string
    Inbox          chan Message
    Wait           func(ctx context.Context) (Result, error)
}
```

每个 Fork child 在独立 worktree 启动（D4-S13-A02 默认开启）；通过 `multiagent/external` Registry 注册 child agent；child→child SendMessage 经 `Inbox` 直接路由（绕过 DAG）。

新增 tool `fork_subagent`（PluginRunner）参数：`children: [{name, prompt}], worktree: bool, n: int`。

### 2.9 D5-S23-A03 — /doctor (alias A1)

**对标**: clawcode `/doctor` slash command。

#### 2.9.1 设计

`internal/layers/observability/diagnose/doctor/`：

```go
type Check struct {
    Name   string
    Status string // "pass"|"warn"|"fail"
    Detail string
}

type Doctor interface {
    Run(ctx context.Context) []Check
}
```

内置 checks：
- `install_paths` — devrix 二进制可达；go/gopls/tsserver 可寻
- `config_yaml_valid` — `devrix.yaml` 解析成功
- `lsp_servers_reachable` — 每个 LSPServer.Command 存在
- `workdir_writable` — 当前 workdir 可写
- `observability_ready` — slog / tracer initialized
- `tool_count` — 注册工具数 > 0
- `transcript_dir_ok` — 转录目录可写（D1-S2-A02）

CLI：`devrix doctor [--json|--table]` 输出。

### 2.10 D5-S24-A02 — Debug 日志分类过滤 (alias A2)

`internal/layers/observability/instrument/logger/debugfilter/`：

```go
// Filter 包装现有 slog Handler，按 category attr 过滤。
type Filter struct {
    inner slog.Handler
    enabled map[string]bool
}

func New(inner slog.Handler, categories []string) *Filter
```

启用方式：CLI `--debug=api,hooks,telemetry`；handler 在 `Enabled()` 检查 record attrs 或 logger group。

### 2.11 D1-S2-A02 — 会话转录持久化 (alias A3)

`internal/layers/communication/capture/transcript/`：

```go
type Writer struct {
    dir string // ~/.devrix/transcripts/
}

func (w *Writer) Append(sessionID string, event Event) error

type Event struct {
    Time  time.Time
    Kind  string // "user"|"assistant"|"tool_call"|"tool_result"|"system"
    Role  string
    Body  string
}
```

文件 `<sessionID>.jsonl` 每行一条事件。

`--continue` CLI flag：读取最近 session 文件 → 重建 D2 context window → 进入新回合。

### 2.12 D5-S23-A04 — 故障注入 (alias A4)

`internal/layers/observability/diagnose/faultinject/`：

```go
type Injector struct {
    enabled bool
    rules []Rule
}

type Rule struct {
    Target string // 如 "llmgateway.dispatch.invoke"
    Mode   string // "error"|"latency"|"truncate"
    Param  string // e.g. error message or latency_ms
    Once   bool
}

func (i *Injector) Hook(target string) error // 返回注入错误或 nil
```

由环境变量 `DEVRIX_FAULT_INJECT=target=mode:param[,target=...]` 启用，仅在 `tests/` build tag 下生效；生产 binary `nil` 实现。

### 2.13 D2-S6-A03 — 上下文窗口分析 (alias A5)

`internal/layers/contextengine/token/windowanalyzer/`：

```go
type Breakdown struct {
    System    int
    Tools     int
    Messages  int
    Thinking  int
    Reminders int
    Total     int
}

type Analyzer interface {
    Analyze(history []Message) Breakdown
}
```

输出表格 + percentage 条形（ASCII），由 CLI `devrix context analyze --session <id>` 触发。

---

## 3. 跨域 import 矩阵

| from → to | 允许 | 说明 |
|-----------|------|------|
| `contextengine/enforce/toolrunner/lsp_tool.go` → `shared/lsp` | ✓ | shared 包横切 |
| `observability/diagnose/tracker` → `shared/lsp` | ✗ | 不耦合，linter 独立 |
| `evolution/verify` → `shared/...` | ✓ | shared 工具横切 |
| `multiagent/provision/freefork` → `multiagent/external` | ✓ | 同域 |
| `communication/capture/transcript` → `shared/types` | ✓ | shared 类型 |
| `llmgateway/protect/errorclass` → `shared/errors` | ✓ | shared sentinel |
| `bootstrap` → 任意 | ✓ | bootstrap 是唯一允许调用所有层的入口 |

通过现有 `internal/lint/` 层级检查器验证。

---

## 4. 测试矩阵

| ID | 描述 | 实现 (DSAFT Activity, alias) | 测试位置 |
|----|------|------------------------------|----------|
| D2-S4-A01-T01 | LSP ToolPool 注册 | D2-S4-A01 (G1) | `enforce/toolrunner/lsp_tool_test.go` |
| D2-S4-A01-T02 | LSP definition 返回 location + context | D2-S4-A01 (G1) | `shared/lsp/manager_test.go` |
| D2-S4-A01-T03 | LSP references 跨文件 | D2-S4-A01 (G1) | `shared/lsp/manager_test.go` |
| D2-S4-A01-T04 | LSP incomingCalls 列出调用方 | D2-S4-A01 (G1) | `shared/lsp/manager_test.go` |
| D2-S4-A01-T05 | LSP LRU 淘汰 (cap=2 → 3rd 触发 evict) | D2-S4-A01 (G1) | `shared/lsp/manager_test.go` |
| D5-S23-A02-T01 | Tracker SnapshotBefore + Diff 报新错 | D5-S23-A02 (G6) | `diagnose/tracker/tracker_test.go` |
| D5-S23-A02-T02 | Tracker 无关编辑 → Diff 为空 | D5-S23-A02 (G6) | `diagnose/tracker/tracker_test.go` |
| D5-S23-A02-T03 | Tracker 500 LRU 去重 | D5-S23-A02 (G6) | `diagnose/tracker/tracker_test.go` |
| TOOL-SEC-2-A02-T01 | Bash AST 检出 heredoc 注入 | TOOL-SEC-2-A02 (G2) | `sandboxast/analyzer_test.go` |
| TOOL-SEC-2-A02-T02 | Bash AST 检出 zsh sysopen | TOOL-SEC-2-A02 (G2) | `sandboxast/analyzer_test.go` |
| TOOL-SEC-2-A02-T03 | AST 解析失败 → regex fallback | TOOL-SEC-2-A02 (G2) | `sandboxast/analyzer_test.go` |
| D6-S11-A02-T01 | Verifier 全部 plan item 通过 | D6-S11-A02 (G4) | `evolution/verify/plan_test.go` |
| D6-S11-A02-T02 | Verifier 缺失 evidence → unverified | D6-S11-A02 (G4) | `evolution/verify/plan_test.go` |
| D4-S12-A03-T01 | TaskNotify Bus 推送 completion event | D4-S12-A03 (G3) | `notify/bus_test.go` |
| D4-S11-A02-T01 | FreeFork 启动 N 子代理 worktree 隔离 (D4-S13-A02) | D4-S11-A02 + D4-S13-A02 (G5) | `provision/freefork/forker_test.go` |
| D4-S11-A02-T02 | FreeFork SendMessage 路由 | D4-S11-A02 (G5) | `provision/freefork/forker_test.go` |
| D5-S23-A03-T01 | Doctor 报告 healthy | D5-S23-A03 (A1) | `doctor/doctor_test.go` |
| D5-S23-A03-T02 | Doctor 检出 missing lsp server | D5-S23-A03 (A1) | `doctor/doctor_test.go` |
| D5-S24-A02-T01 | DebugFilter `api` 类启用 | D5-S24-A02 (A2) | `debugfilter/filter_test.go` |
| D1-S2-A02-T01 | Transcript writer append JSONL | D1-S2-A02 (A3) | `transcript/writer_test.go` |
| D1-S2-A02-T02 | Transcript --continue 恢复 session | D1-S2-A02 (A3) | `transcript/replay_test.go` |
| D5-S23-A04-T01 | FaultInject 触发 error 一次 | D5-S23-A04 (A4) | `faultinject/injector_test.go` |
| D2-S6-A03-T01 | WindowAnalyzer 分解 token 类别 | D2-S6-A03 (A5) | `windowanalyzer/analyzer_test.go` |
| D3-S3-A02-T01 | ErrorClass map rate_limit 429 | D3-S3-A02 (A6) | `errorclass/classifier_test.go` |
| D3-S3-A02-T02 | ErrorClass ≥20 类映射 | D3-S3-A02 (A6) | `errorclass/classifier_test.go` |
| TOOL-SEC-2-A03-T01 | ShortStack 截取 N 帧 | D2-S6-A02 (A7) | `shared/errors/shortstack_test.go` |

**P0 测试点**: D2-S4-A01-T01..T05、D5-S23-A02-T01..T03、D3-S3-A02-T01..T02、TOOL-SEC-2-A03-T01 共 **11 项**。

---

## 5. 风险与缓解（实施层补充）

| # | 风险 | 缓解 |
|---|------|------|
| R1 | gopls/tsserver 未安装导致 LSP tool 调用失败 | `LSPConfig.Enabled` 默认 `false`；Manager 在 Acquire 时返回明确错误（`lsp: server "gopls" not found in PATH`）；Doctor check 提前告警 |
| R2 | `go vet` 在不完整 package 上失败导致 tracker 误报 | tracker 仅 diff 新增 diagnostic（旧+新 → 新 -= 旧），不报告"消失"；vet exit≠0 时记录但不暴露给模型 |
| R3 | mvdan.cc/sh AST 解析 panic | Analyzer 内 `defer recover` → 返回 `Verdict.Allow=true`（fallback 到 regex） |
| R4 | FreeFork worktree 数量爆炸 | `MaxConcurrentFork=8`（config）；超出 → 拒绝并 hint（D4-S13-A02 隔离计数） |
| R5 | D1-S2-A02 转录目录 disk full | Writer 在 ENOSPC 时降级为 in-memory ring buffer，记录 metric |
| R6 | D5-S23-A04 Injector 误触发到生产 | 仅在 `testbuild` build tag + `DEVRIX_FAULT_INJECT` env 同时存在时启用 |
| R7 | D3-S3-A02 漏分类导致 metric 噪声 | 未匹配返回 `ClassUnknown`，配 metric `llm_error_unclassified_total`，阈值告警 |

---

## 6. 实施顺序

按依赖关系排序，每项独立可 merge（以 DSAFT Activity 标识）：

1. D2-S6-A02 (alias A7) ShortStack（shared 横切，零依赖）
2. D3-S3-A02 (alias A6) ErrorClassifier（依赖 shared/errors）
3. D5-S23-A02 (alias G6) Tracker（独立）
4. D2-S4-A01 (alias G1) LSP (shared/lsp → toolrunner/lsp_tool)
5. TOOL-SEC-2-A02 (alias G2) Bash AST (go get + analyzer)
6. D6-S11-A02 (alias G4) Verifier
7. D4-S12-A03 (alias G3) TaskNotify
8. D4-S11-A02 + D4-S13-A02 (alias G5) FreeFork（依赖 multiagent/external）
9. D5-S23-A03 (alias A1) Doctor
10. D5-S24-A02 (alias A2) DebugFilter
11. D1-S2-A02 (alias A3) Transcript
12. D5-S23-A04 (alias A4) FaultInject
13. D2-S6-A03 (alias A5) WindowAnalyzer

完成后 `bootstrap` 串联 + 集成测试。

---

## 7. spec.md 输出

按域分别在 `openspec/changes/devrix-diagnostic-tools-parity/specs/<domain>/spec.md` 输出 ADDED Requirements + Gherkin 场景；S6 归档时合并到 `openspec/specs/<domain>/spec.md`。

涉及域：D1 / D2 / D3 / D4 / D5 / D6 / tool-security。

---

## 8. S6 归档清单

参见 `tasks.md` W6.* 项。
