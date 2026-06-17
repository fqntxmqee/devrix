# Proposal: 工具面契约化 — ToolSurface + ToolFilter 拆面

**Change ID:** devrix-tool-surface-contract
**Demand ID:** DM-20260617-007
**Status:** S7_Archived (PR #63 merged 2026-06-17; PR #64 followup W11 phase 2 full split)
**Priority:** P0
**DSAFT:** D2-S4 (Tool Registration 场景) + D7-S5 (Per-agent Tool Visibility 场景) + 新增横切契约域 TOOL-SURFACE

---

## 1. Background

2026-06-17 与用户对 devrix tools 全生命周期做深度 review（analysis-thread
"tools 全生命周期管理设计"），定位到 5 处系统性混乱：

1. **3 个 bootstrap 入口** 各自注册 tool（`NewContextEngine` / `buildWithGate`
   / `WireDelegate`）
2. **6+ 个 package-level global singleton**（`globalFreeForker` / `GlobalTracker`
   / `GlobalHub` / `GlobalTaskManager` / `GlobalSessionQueue` / `globalDeps`
   / `globalBackgroundTaskTools` / `GlobalWriter` 等）
3. **主 / per-agent tool set 不等价**：per-agent 模式 LLM 看不见 free_fork /
   query_diagnostics / delegate_status / task_output / task_list_background
4. **D2↔D7 工具旁路**（DM-20260617-006 hotfix 已修 1 个面）— 剩余同类
   旁路在 `buildWithGate` / `WireDelegate` 内部仍存在
5. **可见性策略无抽象**：per-agent 硬编码 allowlist 是唯一过滤手段，per-mode
   / per-risk / per-session 不可组合

现场已观测到 3 次回归：
- PR #60 (DM-002) 漏掉 free_fork 注册
- PR #58 (DM-005) 漏掉 query_diagnostics 注册
- PR #55 (DM-005 部分) 漏掉 delegate_* 路径

根因都是"加 tool 三件套"模式（改 3 处 bootstrap + 1 个 global + 1 份
allowlist）的脆弱性。

## 2. Problem Statement

### 2.1 现状对照（7 个 surface 维度）

| Surface | 入口 | 装配方式 | 依赖 | 状态 |
|---------|------|---------|------|------|
| BUILTIN (read_file / write_file / edit_file / bash / grep / glob) | `NewContextEngine` | `toolrunner.NewBuiltinToolRegistry` | 无 | ✅ 干净 |
| LSPTOOL (`lsp`) | `NewContextEngine` (conditional) | LSPTOOL | lsp 配置 | ⚠️ 条件依赖 |
| FreeFork (`free_fork`) | `NewContextEngine` | toolrunner + `globalFreeForker` | freefork.Forker | ❌ global |
| Tracker (`query_diagnostics`) | `NewContextEngine` | toolrunner + `GlobalTracker` | tracker.Tracker | ❌ global |
| Verify (`verify_plan_execution`) | `NewContextEngine` | toolrunner | verify.Verifier | ✅ 干净 |
| Delegate (`delegate_status` / `delegate_*`) | `WireDelegate` | `delegate` package | delegate.Runner | ❌ 旁路 IToolRunner |
| BackgroundTask (`task_output` / `task_list_background`) | `multiagent.globalBackgroundTaskTools` | `multiagent` package | multiagent.Runner | ❌ 旁路 + global |

### 2.2 三入口不对等

```
NewContextEngine (主)
  ↓ 注册 13 个 tool
  ↓ 内含 free_fork, query_diagnostics, verify_plan_execution, lsp, builtin (6)

buildWithGate (per-agent)
  ↓ 重新注册硬编码子集：read_file, write_file, edit_file, bash, grep, glob (6)
  ↓ ❌ 缺 free_fork, query_diagnostics, verify_plan_execution, lsp, delegate_*, task_output, task_list_background (7)
  ↓ hardcoded allowlist

WireDelegate (delegate_*)  ← 单独走另一条 wiring
  ↓ ❌ 旁路 IToolRunner，无风险查询，无 LLM tool 暴露
```

**实测含义**：per-agent 模式下用户发"用 free_fork 查一下 X"→ LLM 收到
tool schema 列表里没有 free_fork → 答"我不知道 free_fork"。**目前没有
任何回归测试覆盖这个等价性**。

### 2.3 6+ global singleton 详情

```go
// 全部 package-level var，跨 bootstrap 共享，无 ctx 生命周期
// 测试时 reset 漏一个 → 串味；prod 顺序错一个 → nil pointer
var (
    globalFreeForker          freefork.Forker      // toolrunner/freefork_register.go
    globalForker              freefork.Forker      // 重复! 应合并
    globalDeps                *ToolDeps            // toolrunner/deps.go
    globalBackgroundTaskTools *BackgroundTools     // multiagent/background_tools.go
    GlobalTracker             *tracker.Tracker     // observability/diagnose/tracker/wire.go
    SetGlobalWriter           func(*transcript.Writer)
    GlobalHub                 *notify.Hub
    GlobalTaskManager         *tasks.TaskManager
    GlobalSessionQueue        *tasks.SessionQueue
    SetGlobalFreeForker       func(freefork.Forker)
)
```

每个 singleton 都是一条隐藏依赖线。

## 3. Proposed Solution

### 3.1 方案 A（推荐）：ToolSurface + ToolFilter 拆面契约

**核心思想**：把"tool 装配"从过程式（3 入口 + 1 allowlist + 6 global）改为
声明式（surface 列表 + filter 链）。

#### 3.1.1 两条新拆面契约

```go
// internal/shared/contracts/tool_surface.go
package contracts

// ToolSurface 是一组相关 tool 的可发现入口。
// 每个 tool 类目（builtin / free_fork / delegate_* / ...）一个 surface。
type ToolSurface interface {
    // Name 返回 surface 标识（用于 devrix.yaml 配置 + 日志 + 调试）
    Name() string
    // Tools 返回当前 surface 在 workDir 下对 sessionID 暴露的 tool 列表。
    // 实现方负责按需做条件筛选（如 lsp 看 cfg.LSPEnabled）。
    Tools(ctx context.Context, workDir, sessionID string) []ToolSpec
    // RiskLevel 查询单个 tool 的风险等级；用于 IPermissionGate.Request。
    RiskLevel(name string) types.RiskLevel
    // Execute 通过 surface 内部 dispatcher 执行 tool call。
    // 返回 ToolResult{Output, Error}；Error 非空时不阻塞调用方。
    Execute(ctx context.Context, name, input, workDir string) (*ToolResult, error)
}

// ToolSpec 是 LLM tool schema 的中性格式（与 D3 llmgateway.ToolCall 解耦）
type ToolSpec struct {
    Name        string
    Description string
    Parameters  string  // JSON Schema 字符串
}

// internal/shared/contracts/tool_filter.go
package contracts

// ToolFilter 是可见性策略的最小单元（per-agent / per-risk / per-session / ...）
type ToolFilter interface {
    Apply(specs []ToolSpec, ctx FilterCtx) []ToolSpec
}

type FilterCtx struct {
    SessionID      string
    AgentType      string  // "main" | "explore" | "fix" | "delegate" | ...
    Mode           string  // "plan_mode" | "yolo" | "loop_first" | "rule_orchestrate"
    RiskThreshold  types.RiskLevel
}
```

#### 3.1.2 7 个 surface 实现

| Surface | 文件 | 依赖（无 global） | 替代的旧路径 |
|---------|------|------------------|-------------|
| `BuiltinSurface` | `surface/builtin_surface.go` | 无 | `toolrunner.NewBuiltinToolRegistry` |
| `LSPToolSurface` | `surface/lsptool_surface.go` | lsp config | conditional registration |
| `FreeForkSurface` | `surface/freefork_surface.go` | `freefork.Forker` (显式 ctor) | `globalFreeForker` |
| `TrackerSurface` | `surface/tracker_surface.go` | `*tracker.Tracker` (显式 ctor) | `GlobalTracker` |
| `VerifySurface` | `surface/verify_surface.go` | `verify.Verifier` (显式 ctor) | `RegisterVerifyTool` |
| `DelegateSurface` | `surface/delegate_surface.go` | `delegate.Runner` (显式 ctor) | `WireDelegate` 旁路 |
| `BackgroundTaskSurface` | `surface/background_task_surface.go` | `multiagent.Runner` (显式 ctor) | `globalBackgroundTaskTools` |

**所有 surface 持有依赖通过构造期固化（`NewXxxSurface(deps...)`）**，
**不再有 package-level var**。

#### 3.1.3 3 入口收编为 1 入口 + filter 链

```go
// 旧（3 入口，3 套重复装配）
NewContextEngine(deps)              // 主
buildWithGate(agentType, deps)      // per-agent（重复注册）
WireDelegate(deps)                  // delegate（旁路 IToolRunner）

// 新（1 入口 + 1 filter 链）
surfaces := []ToolSurface{
    NewBuiltinSurface(),
    NewLSPToolSurface(cfg.LSP),
    NewFreeForkSurface(forker),
    NewTrackerSurface(tracker),
    NewVerifySurface(verifier),
    NewDelegateSurface(delegateRunner),
    NewBackgroundTaskSurface(taskRunner),
}

filters := []ToolFilter{
    NewPerAgentFilter(agentType),    // per-agent allowlist
    NewPerRiskFilter(mode),          // per-mode risk threshold
    // NewPerSessionFilter(sid),     // P1
}

// 主：不过 filter
engine := NewContextEngine(deps, surfaces)

// per-agent：经过 filter 链
filteredSurfaces := ApplyFilters(surfaces, filters)
agentEngine := NewContextEngine(deps, filteredSurfaces)

// delegate：经过 PerAgentFilter("delegate")
delegateSurfaces := ApplyFilters(surfaces, []ToolFilter{
    NewPerAgentFilter("delegate"),
})
delegateEngine := NewContextEngine(deps, delegateSurfaces)
```

**关键不变性**：per-agent 看见的 ⊇ 主看见的（per-agent 可选收紧；不允许
**新增** 主没有的 tool）— 由 `TestBuildWithGate_SupersetOfMainEngineTools`
显式守护。

#### 3.1.4 ToolFilter 链实现

```go
// filter/per_agent.go
type perAgentFilter struct {
    agentType string
    allowlist map[string]bool  // agentType → toolNames
}

func (f *perAgentFilter) Apply(specs []ToolSpec, ctx FilterCtx) []ToolSpec {
    if ctx.AgentType != f.agentType {
        return specs  // 不适用本 filter
    }
    out := make([]ToolSpec, 0, len(specs))
    for _, s := range specs {
        if f.allowlist[s.Name] {
            out = append(out, s)
        }
    }
    return out
}

// filter/per_risk.go
type perRiskFilter struct {
    threshold types.RiskLevel
}

func (f *perRiskFilter) Apply(specs []ToolSpec, ctx FilterCtx) []ToolSpec {
    out := make([]ToolSpec, 0, len(specs))
    for _, s := range specs {
        if riskAtMost(s.Risk, f.threshold) {
            out = append(out, s)
        }
    }
    return out
}

// filter/composite.go
type composite struct {
    filters []ToolFilter
}

func (c *composite) Apply(specs []ToolSpec, ctx FilterCtx) []ToolSpec {
    for _, f := range c.filters {
        specs = f.Apply(specs, ctx)
    }
    return specs
}

func Composite(fs ...ToolFilter) ToolFilter {
    return &composite{filters: fs}
}
```

### 3.2 方案 B（不推荐）：仅删除 global，保留 3 入口

只做"global singleton → 显式 dep"的迁移，不引入 surface 抽象。

- ✅ 改动量最小（~200 行）
- ❌ **不解决** 3 入口不对等（per-agent 漏 tool 仍会发生）
- ❌ **不解决** D2↔D7 旁路（WireDelegate 仍旁路 IToolRunner）
- ❌ **不提供** per-mode / per-session 过滤能力
- ❌ **下次** 加新 tool 仍要改 3 处 bootstrap

### 3.3 方案 C（备选）：在 `toolrunner` 内引入 Service Locator

把所有 tool 注册到 `toolrunner.Service.Locator`，`WireDelegate` / `buildWithGate`
按需查询。

- ✅ 单点注册
- ❌ **重蹈** global singleton 覆辙（Service Locator 本身就是 global）
- ❌ 与本次"消除 global"目标**正交**
- ❌ 测试 setup 噪音不降反升

### 3.4 决策

**选择方案 A**。理由：

1. **拆面契约**（Facet Decomposition，DM-020 D-c）是 devrix 既有模式
   （`IPermissionGate` / `ITokenCounter` / `IEngine`），本 change 把同一
   模式应用到 tool 装配层，**架构一致性 > 单次改动量**
2. 方案 B 留下"per-agent 漏 tool"问题，**回归可能 100% 重现**
3. 方案 C 与本 change 目标**正交对立**

## 4. Success Metrics

| Metric | Baseline | Target | 测量 |
|--------|----------|--------|------|
| package-level global var (tool 相关) | 6+ | 0 | `git grep "var global\|var Global" internal/layers/.../toolrunner/ internal/layers/multiagent/ internal/layers/observability/diagnose/tracker/ internal/layers/communication/capture/transcript/"` 输出为空 |
| bootstrap 入口 (tool 注册) | 3 | 1 | `git grep "RegisterFreeForkTool\|RegisterTrackerTool\|RegisterVerifyTool\|WireDelegate" internal/bootstrap/` 仅命中 `context_engine_builder.go` 1 处 |
| per-agent 模式漏 tool 数 | 7 (free_fork / query_diagnostics / verify / lsp / delegate_* / task_output / task_list_background) | 0 | `TestBuildWithGate_SupersetOfMainEngineTools` PASS |
| `turn_adapter.ExecuteRound` 走 IToolRunner 裸调点 | 1 (`a.tools.Execute` 旁路 perm) | 0 | 走 `ToolSurface.Execute` |
| `ToolSurface` 实现数 | 0 | 7 | `ls internal/layers/contextengine/enforce/toolrunner/surface/` |
| `ToolFilter` 实现数 | 0 | 3+ (P0) / 4+ (P1) | `ls internal/layers/contextengine/enforce/toolrunner/filter/` |
| 单测覆盖率 (新增 surface / filter) | n/a | ≥ 80% | `go test -cover` |
| `go test -race ./...` | ✓ | ✓ | CI |
| `go vet ./...` + `staticcheck ./...` 无新增 warning | ✓ | ✓ | CI |
| 既有 P0 T 点（13 项 DM-002 wiring T） | PASS | PASS | `go test -race ./...` 100% 绿 |

## 5. Implementation Plan

| Step | 任务 | 估时（仅供参考） | 交付物 |
|------|------|----------------|--------|
| 1 | 创建 `feat/devrix-tool-surface-contract` 分支 | 1 min | git branch |
| 2 | S3 review + S4 启动：写 `internal/shared/contracts/tool_surface.go` + `tool_filter.go` | 1.5 h | 2 个 interface 文件 + 各 3 个单测 |
| 3 | 7 个 surface 落地（每个 < 150 行） | 8 h | 7 个 `surface/*.go` + 7 个 `surface_test.go` |
| 4 | 3 个 filter 落地（P0：PerAgent + PerRisk + Composite；P1：PerSession） | 3 h | 3-4 个 `filter/*.go` + filter 链单测 |
| 5 | `NewContextEngine` 收编为单 surface 列表构造 | 2 h | `context_engine_builder.go` 重构 + 等价性单测 |
| 6 | `buildWithGate` / `WireDelegate` 改造为复用 surface 列表 + filter 链 | 3 h | `multiagent.go` + `wire_coordinator.go` + 回归单测 |
| 7 | `turn_adapter.ExecuteRound` 改造走 `ToolSurface.Execute` | 1 h | `turn_adapter.go` + 5 个既有 case 重跑 |
| 8 | 删除 6+ global var + setter（两阶段：先收敛后删除） | 2 h | grep 验证零引用 + 全量单测 |
| 9 | `devrix.yaml` 新增 `tools:` 配置节 + `devrix tool list` CLI 子命令 | 2 h | `config/contextengine.go` + `cli/tool/list.go` + 单测 |
| 10 | 全量回归 + E2E IM 验证（per-agent 模式漏 tool 检查） | 2 h | `tests/integration/tool_surface_test.go` |
| 11 | `openspec/specs/tool-surface/spec.md` Gherkin Scenario + `t-registry.md` | 1 h | spec + t-registry |
| 12 | S5 验收 + S6 归档（PR + auto-merge） | 1 h | PR + `verify-archive.sh` |

**总计：约 26.5 小时**（仅供参考，非承诺）

### 执行顺序

1. **Step 2 → Step 3**（先有契约，后有 surface）
2. **Step 4**（filter 可与 step 3 并行）
3. **Step 5 → Step 6 → Step 7**（收编 3 入口，依赖 Step 3-4）
4. **Step 8**（删 global，最激进，依赖 Step 5-7 全绿）
5. **Step 9-12**（配置 + 验证 + 归档）

每个 Step 完成后立即 `git add` + `git commit`（独立 commit），便于回滚
与 review。

## 6. Risks & Mitigations

| 风险 | 等级 | 缓解 |
|------|------|------|
| **`buildWithGate` 复用主 surface 后某些 per-agent 模式漏 tool** | H | AC7 显式测等价性（`⊇`），per-agent 只收紧不放宽；新增 P0 T `TOOL-SURFACE-1-T01` 守护 |
| **2 阶段删除 global 的中间态引入 nil 风险** | H | 阶段 1 显式 dep 持有（global 仍可读，但只用于兼容）；阶段 2 才 `git grep` 验证零引用后删除；每阶段独立单测 |
| **`turn_adapter.ExecuteRound` 改造期间回归现有 5 个 P0 单测** | H | 改造前先 snapshot 5 个 case 的行为，改造后 case 必须 100% 兼容（input → output 字节级一致） |
| **delegate / background task surface 改造期间破坏 WireDelegate 调用图** | M | 阶段 1 保留 WireDelegate 旧路径（adapter 模式），阶段 2 才切；新旧并行 2 周 |
| **filter 链顺序敏感** | M | `FilterCtx` 标注 `Order` 字段；`Composite` 强制 FIFO；显式 `PerAgentFirst()` / `PerRiskFirst()` helper |
| **D2 library 持有 surface 引用 → 依赖方向反转** | M | 明确 surface 是 `internal/layers/contextengine/enforce/toolrunner/surface/`（在 D2 内），library 不 import surface；surface import library |
| **Surface 接口设计过度抽象** | L | `ToolSpec` 用 struct（不要 method 链）；`RiskLevel(name)` 单独一方法而非 `Tools()` 内嵌 |
| **`tools:` 配置节引入新 schema 与旧 config 兼容** | L | 缺省 = 启用 + 全部 risk；旧 yml 文件零修改即可工作 |
| **`devrix tool list` CLI 输出格式破坏 shell 解析** | L | 显式 `--format json|text` flag；缺省 text |
| **OpenSpec S3-Gate review 拒绝** | L | 严格遵循 `review-design.md` 5 段式；提前请 Cursor 预审 |

## 7. Out of Scope

- **不修改** 13 个 diagnostic-tools-parity library 的对外 API
- **不重写** `toolrunner.PluginRunner` 4-method interface
- **不引入** 新 LLM provider / 新通信协议 / 新持久化后端
- **不实现** Surface 动态 plugin loader（P2 AC15 锁定）
- **不合并** ToolFilter 与 IPermissionGate（P2 AC16 锁定）
- **不重构** `buildWithGate` 的 agent-type allowlist 配置文件位置（仍 `multiagent.yaml`）
- **不补** 2026-06-16 19:18 之后历史丢失的 session 数据
- **不动** `cmd/devrix/main.go` 已有的 `doctor` / `context-analyze` 子命令路由
- **不实现** tool 调用链的 OTEL span（与 `devrix-queryloop-spans-v1.1` 无关）
- **不动** `dm-20260617-006` hotfix 闭合的 5 个 turn_adapter case（改造后必须 100% 兼容）

## 8. Open Questions

| Q | 状态 | 决策 |
|---|------|------|
| `ToolSurface.Execute` 是否要返回 `RiskLevel`（用于 perm 决策）？ | S3 决 | 否，perm 决策在 `turn_adapter.ExecuteRound` 已做（DM-006）；surface 内部不再做 |
| `ToolFilter` 是否要支持 session 级别 override（per-session allow/deny 单条 tool）？ | S3 决 | 是，AC11 P1 列入 |
| `WireDelegate` 改造期是 1 周还是 2 周？ | S4 决 | 2 周保守（旧路径 + 新路径并行） |
| Surface 之间有依赖关系（delegate 依赖 task）需要 DAG 校验？ | S3 决 | 否，P2 AC17 锁定 |
| `devrix tool list` 是否要按 surface 分组？ | S3 决 | 是（按 surface name 字母序） |
