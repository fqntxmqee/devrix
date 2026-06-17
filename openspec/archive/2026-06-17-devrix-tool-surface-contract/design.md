# Design: 工具面契约化 — ToolSurface + ToolFilter 拆面

**Change ID:** devrix-tool-surface-contract
**Demand ID:** DM-20260617-007
**Status:** S3_Design
**Parent Proposal:** `proposal.md`
**Parent Demand:** `demand.md`

---

## 0. Grill Review 结论

| Decision | 结论 | 备注 |
|----------|------|------|
| ToolSurface 拆面契约 | **Agreed** | 与既有 IPermissionGate / ITokenCounter / IEngine 同居 shared/contracts |
| ToolFilter 拆面契约 | **Agreed** | 复用既有 `toolpolicy.Filter` (DM-20260614-015)，向上抽取到 shared/contracts |
| 3 入口收编为 1 入口 | **Agreed** | `NewContextEngine` / `buildWithGate` 复用 surface 列表；`WireDelegate` 退化为 post-init hook |
| 6+ global singleton 全删 | **Agreed** | 两阶段：先收敛后删除 |
| per-agent ⊇ main | **Agreed** | AC7 显式守护 |
| 两阶段删除 | **Agreed** | 中间态需 P1 显式 dep 持有 |
| `turn_adapter.ExecuteRound` 走 `ToolSurface.Execute` | **Agreed** | 与 D2 legacy query/executor 同链路 |
| Surface 接口最小化（3 方法） | **Agreed** | Name / Tools / RiskLevel + Execute |

## 1. Root Cause Analysis

### 1.1 工具装配当前形态（4 个观察点）

#### 观察 1：3 入口重复注册 + 6+ global singleton

`internal/bootstrap/context_engine.go:44-132` (`NewContextEngine`) 和
`internal/bootstrap/context_engine_builder.go:94-187` (`buildWithGate`)
是**两份近乎逐行重复**的代码：

```go
// 两份都做：
toolReg, err := contextengine.NewBuiltinToolRegistry(toolCfg)
enforce.RegisterQueryLoopTools(toolReg, ctxCfg)
workmodel.RegisterTaskTools(toolReg, ctxCfg, workmodel.GlobalTaskManager)
enforce.RegisterBackgroundTaskTools(toolReg)
if ctxCfg.TodoWrite.Enabled { toolReg.Register(...) }
if agentToolReg != nil { ... }
toolrunner.RegisterLSPTool(toolReg, nil)
toolrunner.RegisterVerifyTool(toolReg)
toolrunner.RegisterFreeForkTool(toolReg)
wireFreeForkerInjection()
wireTaskNotifDrainer()
tracker.SetGlobalTracker(diagTracker)
startTrackerTick(...)
toolrunner.RegisterTrackerTool(toolReg)
if tdir != "" { transcript.SetGlobalWriter(tw) }
```

注释里甚至自承：

> "Diagnostic tool surface (kept in sync with ContextEngineBuilder.buildWithGate
>  so the leader LLM sees the same tool list as per-agent engines)."

但实际**没保持同步**：buildWithGate 还多注册了 `delegatetools.RegisterTools`
（per-agent 有 delegate，main 没有）— 而 PR #60 / #58 / #55 三次回归都
发生在这两份代码之间。

#### 观察 2：6+ package-level global singleton 详情

| Global | 位置 | 调用方 | 状态 |
|--------|------|--------|------|
| `toolrunner.globalFreeForker` | `internal/layers/contextengine/enforce/toolrunner/freefork_tool.go:49` | `freeforkRunner.Execute` | 隐式 |
| `toolrunner.SetFreeForker` | `freefork_tool.go:54` | `wireFreeForkerInjection` | setter |
| `multiagent.globalBackgroundTaskTools` | `internal/layers/multiagent/background_tools.go` | `enforce.RegisterBackgroundTaskTools` | 隐式 |
| `tracker.GlobalTracker` | `internal/layers/observability/diagnose/tracker/wire.go` | `tracker_tool.go:trackerRunner.Execute` | 隐式 |
| `tracker.SetGlobalTracker` | 同上 | `NewContextEngine:108` / `buildWithGate:162` | setter |
| `transcript.SetGlobalWriter` | `internal/layers/communication/capture/transcript/wire.go` | `NewContextEngine:126` / `buildWithGate:182` | setter |
| `transcript.GlobalWriter` | 同上 | `gateway.ExpireSession` | 隐式 |
| `notify.GlobalHub` | `internal/layers/multiagent/notify/hub.go` | `delegatetools.RegisterTools` | 隐式 |
| `notify.SetGlobalHub` | 同上 | bootstrap | setter |
| `tasks.GlobalTaskManager` | `internal/layers/multiagent/tasks/manager.go` | `workmodel.RegisterTaskTools` | 隐式 |
| `tasks.SetGlobalTaskManager` | 同上 | `workmodel.InitGlobalTaskManager` | setter |
| `tasks.GlobalSessionQueue` | `internal/layers/multiagent/tasks/queue.go` | `contextengine.NewContextEngine` (Deps.SessionCommandQueue) | 隐式 |
| `tasks.SetGlobalSessionQueue` | 同上 | bootstrap | setter |
| `freefork.SetGlobalFreeForker` | `internal/layers/multiagent/provision/freefork/*.go` | `wireFreeForkerInjection` | setter |

`git grep` 验证：**104 处引用** 散落在 `internal/` 全树，测试文件 8 处
`SetGlobalXxx(...) + defer reset` 模式。

#### 观察 3：per-agent ⊉ main（实测）

`buildWithGate` 漏注册 4 个 tool（在 main engine 已注册但 buildWithGate
没注册）：

- `verify_plan_execution` (实际 buildWithGate **有** 注册，line 142) ✅
- `free_fork` (buildWithGate **有** 注册，line 148) ✅
- `query_diagnostics` (buildWithGate **有** 注册，line 164) ✅
- `lsp` (buildWithGate **有** 注册，line 137) ✅

实测全部都已注册 — **当前代码不存在漏注册**。但**没有任何回归测试
守护这个等价性**。下次修改时漏一个就立刻回退。AC7 落地后才真正守住。

#### 观察 4：per-agent visibility 已有但不可组合

`internal/layers/orchestration/toolpolicy/filter.go` (DM-20260614-015)
已经有 per-agent tool filter：

```go
type Filter struct{}
func (f *Filter) Filter(sc *types.SessionContext, tools []toolrunner.ToolSchema) []toolrunner.ToolSchema
```

— 隐藏 delegate_* from workers，约束 explore/plan 为 read-only 子集。

但它有几个**不可组合**的点：
- 输入是 `[]toolrunner.ToolSchema`（D2 内部 type），**不**是中性 `ToolSpec`
- 接受 `*types.SessionContext`（D1 内部 type），**不**是中性 `FilterCtx`
- 没有 `Composite(...)` / `Allow(name)` / `Deny(name)` 组合原语
- per-mode / per-risk / per-session 过滤都没法加

**结论**：现状的 per-agent filter 是 D7 单点的 `AgentRoleToolFilter` 接口，
**未走 shared/contracts 拆面**，与 devrix 既有"横切关注点放 shared/contracts"
原则不符。

### 1.2 Bug 链

```
NewContextEngine + buildWithGate 双份重复装配
  ↓ 维护负担
"加 tool 三件套"模式（改 3 处 + 1 global + 1 allowlist）
  ↓ 已 3 次回归
PR #60 (DM-002) 漏掉 free_fork
PR #58 (DM-005) 漏掉 query_diagnostics
PR #55 (DM-005 部分) 漏掉 delegate_*
  ↓
测试 setup 噪音（8 处 SetGlobalXxx + defer reset）
  ↓
D2↔D7 工具旁路（DM-20260617-006 hotfix 修了 1 个面）
  ↓
本次建议 1+2 拆面契约（devrix-tool-surface-contract）做整体收编
```

### 1.3 根因

**工具装配没有走 shared/contracts 拆面**。devrix 既有规范（DM-020 D-c
+ `architecture-design.md §1.1` Facet Decomposition）要求横切关注点
走 `internal/shared/contracts/`。permission / token counter / engine 都
遵守了；**tool 装配没遵守**，所以：

- 装配逻辑硬编码在 3 个 bootstrap 入口
- 依赖通过 package-level var 传递
- 可见性策略用 D2 内部 type（`toolrunner.ToolSchema`）写死

### 1.4 连锁影响

| 下游 | 行为 | 触发条件 |
|------|------|---------|
| per-agent LLM 漏 tool | LLM "I don't know free_fork" | 下次改 buildWithGate 漏一行 |
| 测试 setup 噪音 | `t.Cleanup(SetGlobalXxx(nil))` 漏一个 → 串味 | 多 tool 单测并发跑 |
| D2↔D7 旁路 | surface 改 IPermissionGate 时漏一个面 | 新加 surface 时 |
| 配置不灵活 | devrix.yaml 不能按 surface disable / 调 risk | dev 用户要关 free_fork 时 |

## 2. Solution Design

### 2.1 总体架构：3 拆面契约 + 7 surface + N filter

```
                 ┌─────────────────────────────────────────────┐
                 │   internal/shared/contracts/ (拆面契约层)    │
                 │  ┌──────────────┐  ┌──────────────┐          │
                 │  │ ToolSurface  │  │  ToolFilter  │ (新)     │
                 │  └──────┬───────┘  └──────┬───────┘          │
                 │         │                 │                  │
                 │   (既有) IPermissionGate / ITokenCounter /  │
                 │          IEngine / AgentRoleToolFilter       │
                 └─────────────┬───────────────┬────────────────┘
                               │               │
            ┌──────────────────▼───────────────▼───────────────┐
            │   internal/layers/contextengine/enforce/         │
            │      toolrunner/surface/  (7 个 surface)         │
            │      toolrunner/filter/   (3+ 个 filter)        │
            └──────────────────┬───────────────────────────────┘
                               │
            ┌──────────────────▼───────────────────────────────┐
            │   internal/bootstrap/ (DI 容器)                  │
            │  ┌──────────────────────────────────────────┐    │
            │  │ NewContextEngine(deps, []ToolSurface)   │    │
            │  │   ↳ 主模式: 全 surface                   │    │
            │  │   ↳ per-agent: 同一 surface + filter 链 │    │
            │  │   ↳ delegate: 同一 surface + per-risk  │    │
            │  │ WireDelegate: 退化为 post-init hook     │    │
            │  └──────────────────────────────────────────┘    │
            └──────────────────┬───────────────────────────────┘
                               │
            ┌──────────────────▼───────────────────────────────┐
            │   internal/layers/<domain>/ (library) — 行为零变 │
            │  freefork / tracker / verify / delegate / tasks  │
            │  notify / transcript / multiagent / sandboxast   │
            └──────────────────────────────────────────────────┘
```

**核心原则**：

- 拆面契约仅在 `internal/shared/contracts/`（library 不依赖 contracts）
- surface 文件在 D2 内（`toolrunner/surface/`），import library 不被 library import
- filter 文件在 D2 内（`toolrunner/filter/`），可引用 `shared/contracts` + `toolpolicy` 包
- bootstrap 是唯一 DI 容器，持有所有显式 dep
- 6+ global singleton 全部下线（两阶段）

### 2.2 ToolSurface 接口设计

```go
// internal/shared/contracts/tool_surface.go
package contracts

import (
    "context"
    "github.com/devrix/devrix/internal/shared/types"
)

// ToolSpec 是 LLM tool schema 的中性格式，与 D3 llmgateway.ToolCall 解耦。
type ToolSpec struct {
    Name        string
    Description string
    Parameters  string  // JSON Schema
}

// ToolSurface 是一组相关 tool 的可发现入口。
//
// 设计原则（per architecture-design.md §1.1 + golang-patterns small interfaces）：
//   - 接受接口，返回 structs
//   - 4 个方法，1-3 行实现
//   - 不持有 ctx（Execute 接受 ctx）
//   - 不做 permission 决策（perm 由 turn_adapter.ExecuteRound 在 surface 外做）
type ToolSurface interface {
    // Name 返回 surface 标识（配置 + 日志 + 调试）。
    Name() string
    // Tools 返回当前 surface 在 workDir 下对 sessionID 暴露的 tool 列表。
    // 实现方负责条件筛选（如 lsp 看 cfg.LSPEnabled）。
    Tools(ctx context.Context, workDir, sessionID string) []ToolSpec
    // RiskLevel 查询单个 tool 的风险等级。
    RiskLevel(name string) types.RiskLevel
    // Execute 通过 surface 内部 dispatcher 执行 tool call。
    // 返回 ToolResult{Output, Error}；Error 非空时调用方不阻塞。
    Execute(ctx context.Context, name, input, workDir string) (*ToolResult, error)
}

// ToolResult 是 surface.Execute 的返回类型。
type ToolResult struct {
    Output string
    Error  string
}
```

**关键设计选择：**

- **`ToolSpec` 是 struct 不是 interface**：避免 method 链，让 filter/surface
  接受一致的中性格式
- **4 方法而非 1 方法**（不像 `http.Handler`）：因为 `Name()` 是元信息（配置/日志）、
  `Tools()` 是 schema 查询、`RiskLevel(name)` 是按名查 risk（perm 决策）、
  `Execute(...)` 是执行 — 四种用途语义不同
- **不持有 ctx**：surface 持有 dep 即可，ctx 通过 Execute / Tools 传入
- **不做 permission 决策**：`turn_adapter.ExecuteRound` 已在 surface 之外
  调 `IPermissionGate.Request`（DM-20260617-006 修复点），surface 内部不重复
- **workDir / sessionID 走参数而非 ctx value**：避免 ctx 隐式传参；与
  既有 `toolrunner.ToolRunner.Execute(ctx, workDir, input)` 签名一致

### 2.3 ToolFilter 接口设计

```go
// internal/shared/contracts/tool_filter.go
package contracts

import "github.com/devrix/devrix/internal/shared/types"

// FilterCtx 是 filter 决策的输入（中性，不依赖 D1/D2 内部 type）。
type FilterCtx struct {
    SessionID     string
    AgentType     string  // "main" | "explore" | "fix" | "delegate" | "worker" | ...
    Mode          string  // "plan_mode" | "yolo" | "loop_first" | "rule_orchestrate"
    RiskThreshold types.RiskLevel
}

// ToolFilter 是可见性策略的最小单元。
type ToolFilter interface {
    // Apply 返回 specs 的子集（顺序稳定，per Composite FIFO 语义）。
    Apply(specs []ToolSpec, ctx FilterCtx) []ToolSpec
}

// Composite 串联多个 filter（FIFO 顺序敏感，文档化）。
func Composite(fs ...ToolFilter) ToolFilter {
    return &composite{filters: fs}
}

type composite struct{ filters []ToolFilter }

func (c *composite) Apply(specs []ToolSpec, ctx FilterCtx) []ToolSpec {
    for _, f := range c.filters {
        specs = f.Apply(specs, ctx)
    }
    return specs
}

// Allow(name) 是单点白名单 filter。
func Allow(names ...string) ToolFilter {
    set := make(map[string]bool, len(names))
    for _, n := range names {
        set[n] = true
    }
    return allowFilter{set: set}
}

type allowFilter struct{ set map[string]bool }

func (f allowFilter) Apply(specs []ToolSpec, ctx FilterCtx) []ToolSpec {
    out := make([]ToolSpec, 0, len(specs))
    for _, s := range specs {
        if f.set[s.Name] {
            out = append(out, s)
        }
    }
    return out
}

// Deny(name) 是单点黑名单 filter。
func Deny(names ...string) ToolFilter { /* 对称实现 */ }
```

**关键设计选择：**

- **Apply 返回 `[]ToolSpec` 而非 mutate**：immutability（`coding-style.md` 原则）
- **FilterCtx 是中性 struct**：不持有 `*types.SessionContext`（D1 type），
  不持有 `[]toolrunner.ToolSchema`（D2 type）— 这是与既有 `toolpolicy.Filter`
  的**根本区别**（既有用 D1/D2 内部 type，无法跨域组合）
- **Composite FIFO**：与函数式 pipeline 语义一致；调用方显式控制顺序
- **Allow / Deny 是元 filter**：单点白/黑名单可由 `Allow` 实现，per-agent
  allowlist 是 `Allow(...)` + Composite 的特例

### 2.4 7 个 surface 实现

每个 surface 一个文件（< 150 行），统一签名 `NewXxxSurface(deps...) *XxxSurface`。

| Surface | 文件 | 依赖 | 替代的旧路径 |
|---------|------|------|-------------|
| `BuiltinSurface` | `surface/builtin_surface.go` | `*config.ToolConfig` | `toolrunner.NewBuiltinToolRegistry` |
| `LSPToolSurface` | `surface/lsptool_surface.go` | lsp config | `toolrunner.RegisterLSPTool` (conditional) |
| `FreeForkSurface` | `surface/freefork_surface.go` | `freefork.Forker` (显式 ctor) | `toolrunner.RegisterFreeForkTool` + `globalFreeForker` + `SetFreeForker` |
| `TrackerSurface` | `surface/tracker_surface.go` | `*tracker.Tracker` (显式 ctor) | `toolrunner.RegisterTrackerTool` + `tracker.GlobalTracker` + `SetGlobalTracker` |
| `VerifySurface` | `surface/verify_surface.go` | `verify.Verifier` (显式 ctor) | `toolrunner.RegisterVerifyTool` |
| `DelegateSurface` | `surface/delegate_surface.go` | `delegate.Runner` (显式 ctor) | `WireDelegate` 旁路 + `delegatetools.RegisterTools` |
| `BackgroundTaskSurface` | `surface/background_task_surface.go` | `multiagent.Runner` (显式 ctor) | `enforce.RegisterBackgroundTaskTools` + `globalBackgroundTaskTools` |

**示例：`FreeForkSurface`**

```go
// internal/layers/contextengine/enforce/toolrunner/surface/freefork_surface.go
package surface

import (
    "context"
    "encoding/json"
    "github.com/devrix/devrix/internal/shared/contracts"
    "github.com/devrix/devrix/internal/shared/types"
    "github.com/devrix/devrix/internal/layers/multiagent/provision/freefork"
)

type FreeForkSurface struct {
    forker freefork.Forker  // 显式 dep，无 global
}

func NewFreeForkSurface(fk freefork.Forker) *FreeForkSurface {
    return &FreeForkSurface{forker: fk}
}

func (s *FreeForkSurface) Name() string { return "free_fork" }

func (s *FreeForkSurface) Tools(_ context.Context, _, _ string) []contracts.ToolSpec {
    return []contracts.ToolSpec{{
        Name:        "free_fork",
        Description: "Batch fork N child agents (1..5) under a parent session...",
        Parameters:  `{"parent_session":"<id>","requests":[{"name":"...","prompt":"...","worktree":true,"mode":"default"}]}`,
    }}
}

func (s *FreeForkSurface) RiskLevel(name string) types.RiskLevel {
    if name == "free_fork" {
        return types.RiskLevelHigh
    }
    return types.RiskLevelLow
}

func (s *FreeForkSurface) Execute(ctx context.Context, _, input, workDir string) (*contracts.ToolResult, error) {
    // 内部解 freeforkInput + 调 s.forker.Fork(...) + 返回 JSON
    // 完全不依赖 globalFreeForker
}
```

### 2.5 3 个 filter 实现

| Filter | 文件 | 输入 | 输出 |
|--------|------|------|------|
| `PerAgentFilter` | `filter/per_agent.go` | `FilterCtx.AgentType` | 子集 |
| `PerRiskFilter` | `filter/per_risk.go` | `FilterCtx.RiskThreshold` + `ToolSpec.Risk` | 子集 |
| `PerSessionFilter` | `filter/per_session.go` (P1) | `FilterCtx.SessionID` + 外部 KV | 子集 |

**PerAgentFilter 实现：**

```go
package filter

import "github.com/devrix/devrix/internal/shared/contracts"

type PerAgentFilter struct {
    allowlist map[string]map[string]bool  // agentType → toolNames
}

func NewPerAgentFilter() *PerAgentFilter {
    return &PerAgentFilter{
        allowlist: map[string]map[string]bool{
            "main":     {},  // 全部允许
            "explore":  {"read_file": true, "glob": true, "grep": true, "list_dir": true},
            "plan":     {"read_file": true, "glob": true, "grep": true, "list_dir": true, "enter_plan_mode": true, "exit_plan_mode": true},
            "fix":      {},  // 全部允许
            "delegate": {"delegate_explore": true, "delegate_plan": true, "delegate_implement": true, "delegate_status": true},
            "worker":   {"read_file": true, "glob": true, "grep": true, "list_dir": true, "edit_file": true, "bash": true, "task_create": true, "task_get": true, "task_list": true, "task_update": true, "todo_write": true},
        },
    }
}

func (f *PerAgentFilter) Apply(specs []contracts.ToolSpec, ctx contracts.FilterCtx) []contracts.ToolSpec {
    if ctx.AgentType == "" || ctx.AgentType == "main" || ctx.AgentType == "fix" {
        return specs  // 不过滤
    }
    allow, ok := f.allowlist[ctx.AgentType]
    if !ok {
        return specs  // 未知 agent 不过滤（保守）
    }
    if len(allow) == 0 {
        return specs
    }
    out := make([]contracts.ToolSpec, 0, len(specs))
    for _, s := range specs {
        if allow[s.Name] {
            out = append(out, s)
        }
    }
    return out
}
```

**`toolpolicy.Filter` 适配为 ToolFilter：**

```go
// internal/layers/orchestration/toolpolicy/filter.go (改)
package toolpolicy

import (
    "github.com/devrix/devrix/internal/shared/contracts"
    "github.com/devrix/devrix/internal/layers/contextengine/enforce/toolrunner"
)

// AsToolFilter 把既有 toolpolicy.Filter 包成 contracts.ToolFilter。
// 复用 FilterToolsForAgentRole 内部逻辑，仅做 type 转换。
func (f *Filter) AsToolFilter() contracts.ToolFilter {
    return toolPolicyFilter{f: f}
}

type toolPolicyFilter struct{ f *Filter }

func (t toolPolicyFilter) Apply(specs []contracts.ToolSpec, ctx contracts.FilterCtx) []contracts.ToolSpec {
    // 把 contracts.ToolSpec 转 toolrunner.ToolSchema
    schemas := specsToSchemas(specs)
    // 构造伪 SessionContext (仅 AgentType 字段被用)
    sc := &types.SessionContext{
        AgentID:    ctx.AgentType,  // hack: 复用 AgentID 字段传 AgentType
        IsWorker:   ctx.AgentType == "worker" || ctx.AgentType == "explore" || ctx.AgentType == "plan",
        WorkerRole: ctx.AgentType,
    }
    filtered := FilterToolsForAgentRole(sc, schemas)
    return schemasToSpecs(filtered)
}
```

**关键决策**：保留 `toolpolicy.Filter`（DM-20260614-015）作为内部实现细节，
**不删除**。`AsToolFilter()` 是适配器，确保新 `ToolFilter` 链能调用既有
per-agent 逻辑。`FilterToolsForAgentRole` 的 allowlist（explore/plan 只读
子集、worker 限制、delegate_* 隐藏）**逻辑零变化**。

### 2.6 3 入口收编为 1 入口 + filter 链

**改造前：**

```go
// cmd/devrix/main.go (3 个 bootstrap 入口)
NewContextEngine(stack, permMgr, ctxCfg, toolCfg, maCfg, obsBridge, agentToolReg)  // 1
// ... 
builder := NewContextEngineBuilder(...)
builder.Build(perm)  // 2 (per-agent)
// ...
WireDelegate(ctxCfg, maCfg, gw, engine, toolReg)  // 3 (post-init)
```

**改造后：**

```go
// cmd/devrix/main.go (1 个 surface 列表 + filter 链)
surfaces := []contracts.ToolSurface{
    surface.NewBuiltinSurface(toolCfg),
    surface.NewLSPToolSurface(lspCfg),
    surface.NewFreeForkSurface(freefork.NewDefaultForker(...)),
    surface.NewTrackerSurface(tracker.New(diagCfg.TrackerLRUCapacity)),
    surface.NewVerifySurface(verify.NewVerifier()),
    surface.NewDelegateSurface(delegateRunner),
    surface.NewBackgroundTaskSurface(taskRunner),
}

filters := []contracts.ToolFilter{
    toolpolicy.NewFilter().AsToolFilter(),  // per-agent (既有)
    filter.NewPerRiskFilter(),                // per-mode (新)
    // filter.NewPerSessionFilter(sidKV),     // per-session (P1)
}

// 主模式
mainEngine := bootstrap.NewContextEngine(deps, surfaces)
// 内部: visibleTools = all surfaces.Tools() (no filter)

// per-agent 模式
agentEngine := bootstrap.NewContextEngine(deps, ApplyFilters(surfaces, filters, FilterCtx{AgentType: agentType}))
// 内部: visibleTools = filter chain(surfaces.Tools())

// delegate 模式
delegateEngine := bootstrap.NewContextEngine(deps, ApplyFilters(surfaces, []contracts.ToolFilter{
    toolpolicy.NewFilter().AsToolFilter(),
}, FilterCtx{AgentType: "delegate"}))

// post-init delegate wiring (保留为 hook)
bootstrap.WireDelegatePostInit(ctxCfg, maCfg, gw, engine, surfaces)  // 内部只做 dispatcher 初始化, 不再 register tool
```

**`ApplyFilters` 辅助函数（`internal/shared/contracts/tool_filter.go`）：**

```go
// ApplyFilters 对每个 surface 跑同一组 filter（FIFO）。
func ApplyFilters(surfaces []ToolSurface, filters []ToolFilter, ctx FilterCtx) []ToolSurface {
    if len(filters) == 0 {
        return surfaces
    }
    out := make([]ToolSurface, len(surfaces))
    for i, s := range surfaces {
        specs := s.Tools(ctxForTools(ctx), "", "")
        for _, f := range filters {
            specs = f.Apply(specs, ctx)
        }
        out[i] = &filteredSurface{surface: s, visible: specs}
    }
    return out
}

type filteredSurface struct {
    surface ToolSurface
    visible []ToolSpec
}

func (f *filteredSurface) Name() string { return f.surface.Name() }
func (f *filteredSurface) Tools(_ context.Context, _, _ string) []ToolSpec { return f.visible }
func (f *filteredSurface) RiskLevel(name string) types.RiskLevel { return f.surface.RiskLevel(name) }
func (f *filteredSurface) Execute(ctx context.Context, name, input, workDir string) (*ToolResult, error) {
    // 检查 visible
    for _, v := range f.visible {
        if v.Name == name {
            return f.surface.Execute(ctx, name, input, workDir)
        }
    }
    return &ToolResult{Error: fmt.Sprintf("tool %q not visible in current context", name)}, nil
}
```

**关键设计选择：**

- **filter 作用于 surface.Tools() 返回的 spec 列表**，不直接过滤 surface 列表
- **filteredSurface 包装原 surface**：Execute 时检查 visible，未通过则返回
  "not visible" 错误，**不会**调用原 surface.Execute
- **filter chain 顺序 FIFO 显式**：默认 `per-agent → per-risk → per-session`，
  Composite 强制 FIFO 语义

### 2.7 `turn_adapter.ExecuteRound` 改造

**改造前**（`internal/bootstrap/turn_adapter.go:142-193`）：

```go
func (a *contextEngineAdapter) ExecuteRound(ctx context.Context, req turn.ToolRoundRequest) (turn.ToolRoundResult, error) {
    if a.tools == nil { return ..., fmt.Errorf("turn adapter: tool runner not available") }
    toolCtx := ctx
    if req.SessionID != "" {
        if prov, ok := a.engine.(sessionContextProvider); ok {
            if sc, ok := prov.SessionContext(req.SessionID); ok && sc != nil {
                toolCtx = contextengine.ToolContextWithGate(toolCtx, sc, a.perm)
            }
        }
    }
    results := make([]turn.ToolResult, len(req.ToolCalls))
    for i, tc := range req.ToolCalls {
        risk := types.RiskLevelLow
        if a.toolsReg != nil { risk = a.toolsReg.RiskLevel(tc.Name) }
        if a.perm != nil && !a.perm.Request(toolCtx, req.SessionID, tc.Name, tc.Input, risk) {
            results[i] = turn.ToolResult{ToolCallID: tc.ID, Error: "permission denied"}
            continue
        }
        result, err := a.tools.Execute(toolCtx, contextengine.ToolCall{
            ID: tc.ID, Name: tc.Name, Input: tc.Input, RiskLevel: risk,
        })
        // ...
    }
    return turn.ToolRoundResult{Results: results}, nil
}
```

**改造后：**

```go
func (a *contextEngineAdapter) ExecuteRound(ctx context.Context, req turn.ToolRoundRequest) (turn.ToolRoundResult, error) {
    if a.surfaces == nil { return ..., fmt.Errorf("turn adapter: surfaces not available") }
    toolCtx := ctx
    if req.SessionID != "" {
        if prov, ok := a.engine.(sessionContextProvider); ok {
            if sc, ok := prov.SessionContext(req.SessionID); ok && sc != nil {
                toolCtx = contextengine.ToolContextWithGate(toolCtx, sc, a.perm)
            }
        }
    }
    results := make([]turn.ToolResult, len(req.ToolCalls))
    for i, tc := range req.ToolCalls {
        // 1. 查 risk
        risk := a.lookupRisk(tc.Name)
        // 2. perm gate
        if a.perm != nil && !a.perm.Request(toolCtx, req.SessionID, tc.Name, tc.Input, risk) {
            results[i] = turn.ToolResult{ToolCallID: tc.ID, Error: "permission denied"}
            continue
        }
        // 3. 找 surface
        surf, ok := a.findSurface(tc.Name)
        if !ok {
            results[i] = turn.ToolResult{ToolCallID: tc.ID, Error: fmt.Sprintf("tool %q not found in any surface", tc.Name)}
            continue
        }
        // 4. 调 surface.Execute
        workDir := a.workDirFor(req.SessionID)
        result, err := surf.Execute(toolCtx, tc.Name, tc.Input, workDir)
        // ... handle err same as before
    }
    return turn.ToolRoundResult{Results: results}, nil
}

func (a *contextEngineAdapter) findSurface(name string) (contracts.ToolSurface, bool) {
    for _, s := range a.surfaces {
        for _, spec := range s.Tools(context.Background(), "", "") {
            if spec.Name == name {
                return s, true
            }
        }
    }
    return nil, false
}
```

**关键设计选择：**

- **a.tools (IToolRunner) 不再被 ExecuteRound 直接调用** — 走 surface.Execute
- **findSurface 线性扫**：surface 数 ≤ 7，O(7) 实际开销 < 1µs；不需要 hash
- **risk 查询走 surface.RiskLevel(name)**，删除 a.toolsReg.RiskLevel 路径
  （a.toolsReg 可保留用于向后兼容，但不再是 SoT）

### 2.8 6+ global singleton 两阶段删除

**阶段 1（PR #63，4-5 天）**：

- 7 个 surface 全部就位（持有显式 dep）
- `NewContextEngine` / `buildWithGate` 改造为接受 `[]ToolSurface`
- 6+ global var 保留（仍可读），但**所有新代码走 surface 路径**
- 全量单测 + E2E IM 验证
- **不删除** global var

**阶段 2（PR #64，2-3 天）**：

- `git grep` 验证 6+ global var 零引用
- 删除 global var + setter 函数
- 全量单测 + E2E IM 验证
- 灰度 1 周（保持 devrix binary 旧版 + 新版并行 1 天）

**回滚点**：

- 阶段 1 回滚：revert PR #63，global var 仍可工作
- 阶段 2 回滚：revert PR #64，但 global var 已被删 — 需用 `git revert` 然后
  从 git history 恢复；**不推荐阶段 2 后回滚**

### 2.9 `devrix tool list` CLI 子命令

```go
// internal/cli/tool/list.go
package tool

import "github.com/devrix/devrix/internal/shared/contracts"

type ListCmd struct {
    Surfaces []contracts.ToolSurface
    Filters  []contracts.ToolFilter
    AgentType string  // --agent flag
    Format    string  // --format json|text
}

func (c *ListCmd) Run() error {
    ctx := contracts.FilterCtx{AgentType: c.AgentType}
    surfaces := contracts.ApplyFilters(c.Surfaces, c.Filters, ctx)
    // 按 surface name 字母序分组 dump
    for _, s := range surfaces {
        specs := s.Tools(context.Background(), "", "")
        // output: surface name, then for each spec: name + risk + description
    }
}
```

**输出格式（text）**：

```
=== main engine tool list (7 surfaces, 18 tools) ===

[builtin] 6 tools
  read_file      low     Read a file from disk
  write_file     medium  Write a file to disk
  ...

[free_fork] 1 tool
  free_fork      high    Batch fork N child agents (1..5) under a parent session
  ...
```

### 2.10 数据契约

#### 2.10.1 ContextEngine 改造

```go
// 既有
type EngineDeps struct {
    Tools               IToolRunner
    ToolsReg            IToolRegistry
    Permission          IPermissionGate
    TokenCounter        ITokenCounter
    ...
    AgentRoleToolFilter AgentRoleToolFilter  // 既有
    ...
}

// 改造后
type EngineDeps struct {
    // 保留向后兼容（如有调用方依赖），但不再 SoT
    Tools               IToolRunner  // DEPRECATED
    ToolsReg            IToolRegistry  // DEPRECATED
    Permission          IPermissionGate
    TokenCounter        ITokenCounter
    Surfaces            []ToolSurface  // 新增 SoT
    Filters             []ToolFilter   // 新增（per-agent 等）
    ...
    AgentRoleToolFilter AgentRoleToolFilter  // 保留（已内部转 ToolFilter）
    ...
}
```

**关键决策**：`Tools` / `ToolsReg` 标记为 `DEPRECATED` 但保留，**不删**
— 5 个 toolrunner 单测（DM-006 hotfix 加的）需要它。新代码走 `Surfaces`。

#### 2.10.2 PreparedContext 改造

```go
// 既有 (turn.PreparedContext)
type PreparedContext struct {
    Tools       []ToolSchema  // D2 内部 type
    Messages    []types.Message
    Model       string
    MaxContextTokens int
    CompressHint *CompressHint
}

// 改造后
type PreparedContext struct {
    Tools        []contracts.ToolSpec  // 中性 type
    VisibleTools []contracts.ToolSpec  // filter chain 之后的可见子集
    Messages     []types.Message
    Model        string
    MaxContextTokens int
    CompressHint *CompressHint
}
```

**关键决策**：`Tools` 保留为 `[]ToolSchema`（D2 内部 type，向后兼容），
**新增** `VisibleTools []contracts.ToolSpec`（中性 type，filter 后）。
两个字段冗余但保持兼容性；`Tools` 标记 DEPRECATED。

## 3. 关键文件变更

| 文件 | 操作 | 行数估算 |
|------|------|---------|
| `internal/shared/contracts/tool_surface.go` | **新增** | +60 行 |
| `internal/shared/contracts/tool_filter.go` | **新增** | +80 行 |
| `internal/shared/contracts/tool_surface_test.go` | **新增** | +60 行（mock surface + interface compliance） |
| `internal/shared/contracts/tool_filter_test.go` | **新增** | +100 行（Composite / Allow / Deny / 6 组合） |
| `internal/layers/contextengine/enforce/toolrunner/surface/builtin_surface.go` | **新增** | +90 行 |
| `internal/layers/contextengine/enforce/toolrunner/surface/lsptool_surface.go` | **新增** | +60 行 |
| `internal/layers/contextengine/enforce/toolrunner/surface/freefork_surface.go` | **新增** | +110 行 |
| `internal/layers/contextengine/enforce/toolrunner/surface/tracker_surface.go` | **新增** | +100 行 |
| `internal/layers/contextengine/enforce/toolrunner/surface/verify_surface.go` | **新增** | +80 行 |
| `internal/layers/contextengine/enforce/toolrunner/surface/delegate_surface.go` | **新增** | +100 行 |
| `internal/layers/contextengine/enforce/toolrunner/surface/background_task_surface.go` | **新增** | +100 行 |
| `internal/layers/contextengine/enforce/toolrunner/filter/per_agent.go` | **新增** | +50 行 |
| `internal/layers/contextengine/enforce/toolrunner/filter/per_risk.go` | **新增** | +50 行 |
| `internal/layers/contextengine/enforce/toolrunner/filter/per_session.go` | **新增** (P1) | +60 行 |
| `internal/layers/contextengine/enforce/toolrunner/filter/composite.go` | **新增** | +30 行 |
| `internal/layers/orchestration/toolpolicy/filter.go` | **修改** | +30 行（AsToolFilter 适配器） |
| `internal/layers/contextengine/engine_types.go` | **修改** | +5 行（EngineDeps.Surfaces / Filters） |
| `internal/layers/contextengine/tool_context.go` | **修改** | +10 行（ToolContextWithSurfaces） |
| `internal/bootstrap/context_engine.go` | **修改** | -50 行 / +30 行（接受 surfaces，去掉 RegisterXxxTool 分散调用） |
| `internal/bootstrap/context_engine_builder.go` | **修改** | -60 行 / +20 行（同上，buildWithGate 复用 surface 列表） |
| `internal/bootstrap/delegate.go` | **修改** | -10 行 / +5 行（WireDelegate 退化为 hook） |
| `internal/bootstrap/turn_adapter.go` | **修改** | -20 行 / +30 行（走 surface.Execute） |
| `internal/shared/config/contextengine.go` | **修改** | +25 行（ToolsConfig + Surfaces map） |
| `internal/cli/tool/list.go` | **新增** | +80 行 |
| `internal/cli/root.go` | **修改** | +10 行（tool list 子命令注册） |
| **删除** 6+ global var + setter | **删除** | -100 行 |
| `openspec/specs/tool-surface/spec.md` | **新增** | +150 行（6 个 Gherkin Scenario） |
| `openspec/specs/tool-surface/t-registry.md` | **新增** | +50 行（6 P0 + 4 P1 T 点） |
| `tests/integration/tool_surface_test.go` | **新增** | +200 行（3 入口等价性 + filter 链 + E2E IM） |

**总计：~+1500 行 / -240 行 = +1260 行**（含测试）

## 4. 接口与数据契约

### 4.1 ToolSurface（中性 contract）

```go
// internal/shared/contracts/tool_surface.go
package contracts

type ToolSpec struct {
    Name        string
    Description string
    Parameters  string  // JSON Schema
}

type ToolResult struct {
    Output string
    Error  string
}

type ToolSurface interface {
    Name() string
    Tools(ctx context.Context, workDir, sessionID string) []ToolSpec
    RiskLevel(name string) types.RiskLevel
    Execute(ctx context.Context, name, input, workDir string) (*ToolResult, error)
}
```

### 4.2 ToolFilter（中性 contract）

```go
// internal/shared/contracts/tool_filter.go
package contracts

type FilterCtx struct {
    SessionID     string
    AgentType     string
    Mode          string
    RiskThreshold types.RiskLevel
}

type ToolFilter interface {
    Apply(specs []ToolSpec, ctx FilterCtx) []ToolSpec
}
```

### 4.3 既有契约保持不变

- `IPermissionGate` (`internal/shared/contracts/permission.go`)
- `ITokenCounter` (`internal/shared/contracts/tokencounter.go`)
- `IEngine` (`internal/shared/contracts/engine.go`)
- `AgentRoleToolFilter` (`internal/layers/contextengine/enforce/agent_role_filter.go`)
  — 保留；新增 `AsToolFilter()` 适配器

## 5. 数据流

### 5.1 装配流（启动期，一次）

```
cmd/devrix/main.go
  ↓
constructors (显式 dep, 无 global)
  ├─ freefork.NewDefaultForker(...) → FreeForkSurface
  ├─ tracker.New(...) → TrackerSurface
  ├─ verify.NewVerifier() → VerifySurface
  ├─ delegateRunner := execute.NewExecutor(...) → DelegateSurface
  ├─ taskRunner := workmodel.NewTaskManager(...) → BackgroundTaskSurface
  └─ ...
  ↓
surfaces := []ToolSurface{builtin, lsp, freefork, tracker, verify, delegate, background}
filters := []ToolFilter{toolpolicy.AsToolFilter(), NewPerRiskFilter()}
  ↓
mainEngine := bootstrap.NewContextEngine(deps, surfaces)  // 不过 filter
agentEngine := bootstrap.NewContextEngine(deps, ApplyFilters(surfaces, filters, {AgentType: "explore"}))
  ↓
所有 surface.Tools() 调用 (在 turn_adapter.ExecuteRound / Prepare 内) 都是声明式、无副作用
```

### 5.2 工具调用流（运行期，每条 LLM tool call）

```
turn.DefaultOrchestrator.runLoop
  ↓ tool_round (DM-006 修复后)
contextEngineAdapter.ExecuteRound (turn_adapter.go)
  ↓ 1. 查 risk
risk := surface.RiskLevel(name)
  ↓ 2. perm gate
perm.Request(ctx, sessionID, name, input, risk) == true
  ↓ 3. 找 surface
surf := findSurface(name)  // O(7) 线性扫
  ↓ 4. 调 surface.Execute
surf.Execute(ctx, name, input, workDir)
  ↓ surface 内部:
  ├─ FreeForkSurface.Execute → s.forker.Fork(...)  // 显式 dep
  ├─ TrackerSurface.Execute → s.tracker.Query(...)
  ├─ VerifySurface.Execute → s.verifier.Verify(...)
  └─ ...
  ↓
ToolResult{Output, Error} → D2 ToolCall result → LLM 下一轮 prompt
```

**关键不变性**：

- `findSurface` 走 `surface.Tools(ctx, "", "")`（无副作用，调用 `Tools()` 仅
  返回 spec 列表，不做 I/O）
- `surf.Execute` 持有显式 dep（构造期固化），不查任何 global

## 6. 回归风险评估

| 风险 | 等级 | 触发条件 | 缓解 |
|------|------|---------|------|
| **`buildWithGate` 复用主 surface 后某些 per-agent 模式漏 tool** | H | 下次改 surface 列表漏一行 | AC7 `TestBuildWithGate_SupersetOfMainEngineTools` 守护；P0 T `TOOL-SURFACE-1-T01` |
| **2 阶段删除 global 的中间态引入 nil 风险** | H | 阶段 1 期间某个调用方既走 surface 路径又读 global | 阶段 1 grep 验证所有 global read 都被 surface 路径覆盖；阶段 2 才 `git grep` 验证零引用 |
| **`turn_adapter.ExecuteRound` 改造期间回归现有 5 个 P0 单测** | H | `a.surfaces` 没注入；`findSurface` 找不到；`risk` 走错 | 改造前先 snapshot 5 个 case 的行为（input → output 字节级）；改造后 case 必须 100% 兼容 |
| **delegate / background task surface 改造期间破坏 WireDelegate 调用图** | M | WireDelegate 旧路径被删太快 | 阶段 1 保留 WireDelegate 旧路径（adapter 模式），阶段 2 才切；新旧并行 2 周 |
| **filter 链顺序敏感** | M | `Composite(PerAgent, PerRisk)` vs `Composite(PerRisk, PerAgent)` 行为不同 | `FilterCtx` 标注 `Order` 字段（debug log）；`Composite` 强制 FIFO；显式 helper `PerAgentFirst()` / `PerRiskFirst()` |
| **D2 library 持有 surface 引用 → 依赖方向反转** | M | surface 文件 import library 但 library 反向 import surface | surface 在 `toolrunner/surface/`（D2 内），library import D2 是允许的；显式 grep 验证 library 不 import surface |
| **Surface 接口设计过度抽象** | L | 后期添加 surface 时累赘 | `ToolSpec` 用 struct（不要 method 链）；`RiskLevel(name)` 单独方法 |
| **`tools:` 配置节引入新 schema 与旧 config 兼容** | L | 旧 yml 文件 break | 缺省 = 启用 + 全部 risk；旧 yml 文件零修改即可工作 |
| **`devrix tool list` CLI 输出格式破坏 shell 解析** | L | 用户 pipe 到 `jq` | 显式 `--format json|text` flag |
| **OpenSpec S3-Gate review 拒绝** | L | 缺 Gherkin Scenario / 缺 Decision 记录 | 严格遵循 `review-design.md` 5 段式 |
| **P1 PerSessionFilter 引入新 dep（session KV）** | L | AC11 P1 锁定，本期不做 | 留 S6 backlog |
| **D7 既有 `AgentRoleToolFilter` 适配器破坏 DM-20260614-015 单测** | M | `AsToolFilter` 与原 `Filter` 行为不一致 | 复用 `FilterToolsForAgentRole` 内部逻辑；新增 `AsToolFilterTest` 覆盖等价性 |
| **`freefork.NewDefaultForker` 创建期阻塞** | L | 初始化时做网络 / 文件 I/O | 已有 freefork library 单测；本 change 仅改注入路径 |

## 7. Rollback Plan

### 7.1 阶段 1（PR #63）

- **回滚方式**：`git revert` PR #63
- **回滚影响**：所有 surface 文件删除，filter 文件删除；`NewContextEngine`
  恢复 3 入口重复装配；6+ global 仍可工作（未删）
- **回滚时间**：< 5 分钟（git revert + go build）
- **回滚验证**：`go test -race ./...` 全绿 + 飞书 IM 1 轮对话

### 7.2 阶段 2（PR #64）

- **回滚方式**：`git revert` PR #64，然后从 git history 恢复 6+ global var
  代码（`git show <commit>:path/to/file`）
- **回滚影响**：所有 surface/filter 保留（阶段 1 已就位），仅 global var
  恢复
- **回滚时间**：15-30 分钟（人工恢复 global var）
- **回滚验证**：同阶段 1

### 7.3 完全回滚

- `git reset --hard <commit-before-tool-surface-contract>`
- 不推荐（丢失所有 surface/filter 改造）

## 8. 关键决策记录

### Decision: ToolSurface 接口方法数（3 vs 4）

**选项：**
| 方案 | 方法 | 优点 | 缺点 |
|------|------|------|------|
| A. 1 方法（Execute only） | `Execute(ctx, name, input, workDir) (*ToolResult, error)` | 极简 | 拿不到 schema（LLM 看不见），拿不到 risk（perm 决策失败） |
| B. 3 方法 | `Name` / `Tools` / `Execute` | 中等粒度 | RiskLevel 走 Tools() 内嵌 risk 字段 → filter chain 拿不到 |
| **C. 4 方法（推荐）** | `Name` / `Tools` / `RiskLevel` / `Execute` | 4 个语义清晰 | 略多 |

**选择：** C
- 4 方法是必要的最小集：metadata / discovery / policy / dispatch 四种用途
- `RiskLevel(name)` 单独一方法 → perm 决策（DM-006 修复点）和 filter chain
  （per-risk filter）都能直接调用
- 与既有 `toolrunner.PluginRunner` 4-method interface (Name/Schema/RiskLevel/Execute) **对齐**

**理由：** 沿用 devrix 既有 tool runner 接口设计模式，最小化学习成本。

### Decision: ToolFilter 链顺序（FIFO vs 自动）

**选项：**
| 方案 | 顺序 | 优点 | 缺点 |
|------|------|------|------|
| A. 自动（per-agent 优先） | 内部固定 | 调用方无需关心 | 难扩展（per-session / per-risk 加进来时易乱） |
| **B. FIFO 显式（推荐）** | `Composite(f1, f2, f3)` | 透明 | 调用方需理解顺序敏感性 |
| C. 全部 filter 同时跑 | AND/OR 组合 | 灵活 | 实现复杂，难调试 |

**选择：** B
- 既有 `toolpolicy.Filter` 已经是单 filter（per-agent），扩展为链自然走 FIFO
- 调用方显式 `Composite(perAgent, perRisk)` 可读
- 与函数式 pipeline 语义一致

**理由：** 简单 + 可读 + 与既有风格一致。

### Decision: surface 内部是否做 perm 决策

**选项：**
| 方案 | 位置 | 优点 | 缺点 |
|------|------|------|------|
| A. surface 内部 | Execute 内 | 单点 | DM-006 hotfix 的"调 perm.Request"被复制 7 次 |
| **B. surface 之外（推荐）** | turn_adapter.ExecuteRound | 1 处 | surface 依赖调用方做 perm |
| C. 双重 | surface 内部 + turn_adapter | 防御 | 重复 |

**选择：** B
- DM-20260617-006 hotfix 已经把 perm gate 放在 turn_adapter.ExecuteRound
- 7 个 surface 各自重复 = 7 个 if 1 个 bug
- surface 是"执行"，不是"决策"

**理由：** 单一职责。

### Decision: 6+ global 一次删还是两阶段

**选项：**
| 方案 | 风险 | 回滚 | 推荐 |
|------|------|------|------|
| A. 一次删 | 高（爆破） | 难 | 否 |
| **B. 两阶段（推荐）** | 中 | 易 | 是 |
| C. 保留（仅注释） | 低 | n/a | 否（用户目标是消除） |

**选择：** B
- 阶段 1：所有新代码走 surface 路径；global 仍可读（保留 1-2 周）
- 阶段 2：grep 验证零引用后删除

**理由：** 大规模重构的标准做法。

### Decision: 既有 `toolpolicy.Filter` 改还是包

**选项：**
| 方案 | 优点 | 缺点 |
|------|------|------|
| A. 改（用 contracts.ToolSpec 替代 toolrunner.ToolSchema） | 类型统一 | DM-20260614-015 单测全 break |
| **B. 包（AsToolFilter 适配器，推荐）** | 既有逻辑零变化 | 多一层 type 转换 |
| C. 删除（合并到 PerAgentFilter） | 单一实现 | 既有 7 个 test 文件要重写 |

**选择：** B
- `toolpolicy.Filter` 已被 D7 用了 1 周，5+ 处调用
- 适配器模式最小侵入
- 既有过滤逻辑（explore/plan read-only、worker 限制、delegate 隐藏）
  **不**重写

**理由：** 重构时尊重既有约定。

## 9. 性能预算

| 操作 | 当前 | 改造后 | 差异 |
|------|------|--------|------|
| 装配期（启动一次） | O(n_tool × n_register_call) | O(n_surface) | -80% 调用次数 |
| `Tools()` 查询（每次 LLM round） | O(1) (从 IToolRunner 拿 schema) | O(n_surface × n_spec_per_surface) ≈ O(20) | +20× |
| `findSurface` (每次 tool call) | O(1) (从 IToolRunner 拿 runner) | O(n_surface × n_spec) ≈ O(20) | +20× |
| `ExecuteRound` (10 tool call) | O(10) | O(10 × 20) = O(200) | +20× |
| 实际延迟 | ~10µs/turn | ~50µs/turn | +40µs（不可感知） |

**关键不变性**：`Tools()` 在同一 surface 实例上**不重复 I/O**（spec 列表
是 cache 友好的）；filter chain 在 `ApplyFilters` 时跑一次，结果缓存在
`filteredSurface.visible` 字段。

## 10. 测试矩阵

### 10.1 Surface 单元测试（P0）

每个 surface 一个 `surface_test.go`：

| Test ID | 场景 | 预期 |
|---------|------|------|
| `TestBuiltinSurface_Tools` | NewBuiltinSurface 调 Tools() | 6 个 spec（read_file / write_file / edit_file / bash / grep / glob） |
| `TestBuiltinSurface_RiskLevel` | RiskLevel("read_file") | types.RiskLevelLow |
| `TestLSPToolSurface_Tools_Disabled` | lsp.Enabled=false | spec 列表空 |
| `TestLSPToolSurface_Tools_Enabled` | lsp.Enabled=true + 1 server | 1 个 spec |
| `TestFreeForkSurface_Execute_Batch3` | 3 个 ForkRequest | spawned_count=3, agent_ids=[3] |
| `TestFreeForkSurface_Execute_Rollback` | 1 个 factory 失败 | error 透传，0 spawned |
| `TestFreeForkSurface_Execute_TooMany` | 6 个 ForkRequest | "requests count must be in [1,5]" |
| `TestTrackerSurface_Execute_AfterTick` | tick 后调 | 返回新错误 |
| `TestTrackerSurface_Execute_NoTick` | 无 tick | 返回空 |
| `TestVerifySurface_Execute_AllDone` | tasks.md 全 done | verified=count, unverified=0 |
| `TestVerifySurface_Execute_MissingFile` | tasks.md 不存在 | error 透传 |
| `TestDelegateSurface_Execute_Explore` | delegate_explore | 返回 status |
| `TestBackgroundTaskSurface_Execute_TaskOutput` | task_output | 返回 task result |

### 10.2 Filter 单元测试（P0）

| Test ID | 场景 | 预期 |
|---------|------|------|
| `TestPerAgentFilter_Main` | AgentType=main | 全部返回 |
| `TestPerAgentFilter_Explore` | AgentType=explore | 只 read_file / glob / grep / list_dir |
| `TestPerAgentFilter_Plan` | AgentType=plan | 同 explore + enter/exit_plan_mode |
| `TestPerAgentFilter_Worker` | AgentType=worker | explore 子集 + edit_file / bash / task_* |
| `TestPerAgentFilter_Delegate` | AgentType=delegate | 只 delegate_* |
| `TestPerRiskFilter_Low` | Threshold=Low | 全部返回 |
| `TestPerRiskFilter_High` | Threshold=High | 只 High |
| `TestPerRiskFilter_Between` | Threshold=Medium | Low+Medium |
| `TestComposite_FIFO` | PerAgent + PerRisk | 先 per-agent 再 per-risk |
| `TestComposite_OrderSensitive` | PerRisk + PerAgent | 顺序不同结果不同（显式记录差异） |
| `TestAllow_Allowlist` | Allow("read_file", "bash") | 只 2 个 |
| `TestDeny_Blocklist` | Deny("free_fork") | 全部 - 1 |
| `TestToolPolicyFilter_Equivalence` | toolpolicy.Filter vs toolpolicy.AsToolFilter | 行为一致（DM-20260614-015 单测全绿） |

### 10.3 集成测试（P0）

**`tests/integration/tool_surface_test.go`**：

| Test ID | 场景 | 预期 |
|---------|------|------|
| `TestNewContextEngine_MainMode_AllTools` | 主模式：surfaces 不带 filter | Tools() 返回 18 个 tool |
| `TestNewContextEngine_ExploreMode_ReadOnly` | explore 模式：per-agent + per-risk(Low) | 只 4 个 tool（read_file / glob / grep / list_dir） |
| `TestNewContextEngine_WorkerMode_Full` | worker 模式：per-agent(worker) | read + edit + task_* 子集 |
| `TestBuildWithGate_SupersetOfMainEngineTools` | 主模式 vs buildWithGate 模式 | buildWithGate ⊇ main（AC7） |
| `TestFilterChain_TwoFilters` | Composite(per-agent, per-risk) | 先 per-agent 再 per-risk |
| `TestTurnAdapter_GoesThroughSurface` | turn_adapter.ExecuteRound 调 1 个 tool | findSurface 被调 + surface.Execute 被调，**不**调 a.tools.Execute |
| `TestTurnAdapter_PermissionDenied` | perm.Request=false | ToolResult.Error="permission denied"，surface.Execute 不被调 |
| `TestTurnAdapter_RiskLevelPropagated` | free_fork risk=High | perm.Request 的 risk 参数是 High |

### 10.4 端到端测试（P0）

**`tests/e2e/im_tool_surface_test.go`**：

| 步骤 | 操作 | 期望 |
|------|------|------|
| 1 | 飞书发"用 free_fork 查 X" | LLM 看到 free_fork schema，调通，返回 agent_ids |
| 2 | 飞书发"查 query_diagnostics" | LLM 看到 query_diagnostics，调通，返回 errors |
| 3 | 飞书发"verify_plan_execution" | LLM 看到 verify，调通，返回 verified 统计 |
| 4 | 飞书发"delegate_explore"（per-agent=main） | LLM 看到 delegate_explore，**调通**（不是 hidden） |
| 5 | 飞书发"delegate_explore"（per-agent=worker） | LLM **看不见** delegate_explore（toolpolicy 隐藏） |

### 10.5 现有单测不破

`go test -race ./...` 100% 绿，包括：
- `internal/bootstrap/turn_adapter_permission_test.go` (5 case, DM-006)
- `internal/layers/contextengine/enforce/toolrunner/*_test.go` (10+ 文件)
- `internal/layers/orchestration/toolpolicy/filter_test.go` (DM-20260614-015)
- `internal/layers/orchestration/turn/orchestrator_test.go` (10+ case)
- `internal/layers/multiagent/notify/*_test.go` (5+ case)
- `internal/layers/observability/diagnose/tracker/*_test.go` (5+ case)
- `internal/layers/communication/capture/transcript/*_test.go` (5+ case)

## 11. 检查清单（S3 完成确认）

- [x] demand.md（DM-20260617-007）已写
- [x] proposal.md 已写（方案对比 + 决策记录）
- [x] design.md 已写（本文件，根因 + 方案 + 文件清单 + 回归风险）
- [x] 5 个 Decision 记录（Surface 方法数 / Filter 顺序 / perm 位置 / global 删除 / toolpolicy 适配）
- [x] dsaft_activities 已标注
- [x] 文件清单 + 行数估算
- [x] 接口契约（ToolSurface / ToolFilter / FilterCtx / ToolSpec / ToolResult）
- [x] 数据流图（装配 + 运行期）
- [x] 回归风险评估（13 条）
- [x] Rollback plan（两阶段）
- [x] 性能预算
- [x] 测试矩阵（surface 单测 + filter 单测 + 集成 + E2E）
- [x] Gherkin Scenario（openspec/specs/tool-surface/spec.md）— 留 S4 写
- [x] T 层测试点（TOOL-SURFACE-1-T01..T06 P0；TOOL-SURFACE-1-T07..T10 P1）— 留 S4 写
- [x] Draft PR 已创建（PR #63）

---

> **S3 → S4 接力**: design.md 通过 review 后进入 S4 实现，按 `tasks.md`
> 列出的 12 个 W 顺序推进。每个 W 独立 commit，便于回滚与 review。
