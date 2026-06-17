---
demand-id: DM-20260617-007
title: 工具面契约化 — ToolSurface + ToolFilter 拆面，消除 3 入口/6+ 单例混乱
priority: P0
status: S1_Proposal
dsaft_domain: multi-domain
created: 2026-06-17
parent_chat: analysis-thread-on-tools-lifecycle
---

# Demand: 工具面契约化 — ToolSurface + ToolFilter 拆面

> **不依赖本次"建议 3+4 hotfix"**（DM-20260617-006），但与之正交互补。hotfix
> 闭合 D2↔D7 单点旁路；本 change 闭合"工具从哪里来、谁能看见、谁来跑"的整条
> 装配链。
>
> **拆分面（Facet Decomposition）原则** 来自 DM-020 D-c：横切关注点通过
> `internal/shared/contracts/` 暴露给上层域，本 change 落地的两条新拆面契约为
> `ToolSurface` 与 `ToolFilter`，与既有 `IPermissionGate` / `ITokenCounter` / `IEngine`
> 同居一系。

## 1. 背景

2026-06-17 与用户做 tools 全生命周期管理深度 review（详见
`openspec/changes/devrix-tool-surface-contract/analysis-tools-lifecycle.md`，将
由本 change 一并产出）后，定位到 5 处系统性混乱：

1. **3 个 bootstrap 入口** 各自独立往 `*ContextEngine` 上注册 tool：
   - `NewContextEngine`（`internal/bootstrap/context_engine_builder.go`）— 主
     engine，buildWithGate 内逐 tool 调用 `toolrunner.RegisterXxxTool`
   - `buildWithGate`（`internal/bootstrap/multiagent.go`）— per-agent 的工具
     子集，逐 agent 重新注册
   - `WireDelegate`（`internal/bootstrap/wire_coordinator.go`）— delegate_*
     工具单独走另一条 wiring
2. **6+ 个 package-level global singleton**：
   - `toolrunner.globalFreeForker`
   - `toolrunner.globalForker`
   - `toolrunner.globalDeps`
   - `multiagent.globalBackgroundTaskTools`
   - `diagnose.tracker.GlobalTracker`
   - `transcript.SetGlobalWriter` / `GlobalWriter`
   - `notify.GlobalHub` / `SetGlobalHub`
   - `tasks.GlobalTaskManager` / `SetGlobalTaskManager`
   - `tasks.GlobalSessionQueue` / `SetGlobalSessionQueue`
   - `freefork.SetGlobalFreeForker` / `globalFreeForker`
3. **主 / per-agent tool set 不等价**：`buildWithGate` 只注册"安全子集"
   （read_file / grep / glob），漏了 free_fork / query_diagnostics / delegate_status
   / task_output / task_list_background 等已在主 engine 注册的 tool，per-agent
   模式下 LLM 看不见这些能力。
4. **D2↔D7 工具旁路**（DM-20260617-006 已修 1 个面）：D7 turn adapter 之前
   不调 `IPermissionGate.Request`，且不传 `RiskLevel` 到 D2 ToolCall。已通过
   hotfix 闭合，但同源的"裸调 `a.tools.Execute`"在 `buildWithGate` 内部
   仍然存在。
5. **可见性策略 (visibility) 缺失**：`buildWithGate` 的硬编码 allowlist 是
   唯一一处"按 agent 类型过滤 tool"的逻辑，没有抽象成可组合的 filter 链
   （per-agent / per-mode / per-risk / per-session 都没法组合）。

**结果**：每加一个 tool 都要在 3 个 bootstrap 入口各加一段 + 1 个 global
singleton + 可能还要在 `buildWithGate` allowlist 维护一份。现场已观测到
3 次回归（PR #60 漏掉 free_fork、PR #58 漏掉 query_diagnostics、PR #55 漏掉
delegate_*）。

## 2. 问题陈述

### 2.1 Tool 装配链路现状（按 "哪里来 → 怎么进 engine → 谁能跑" 三段切片）

| Tool 类别 | 入口 | 装配 | 谁来跑 | 状态 |
|----------|------|------|-------|------|
| `read_file` / `write_file` / `edit_file` / `bash` | `NewContextEngine` | `toolrunner.NewBuiltinToolRegistry` (BUILTIN) | `toolrunner.IToolRunner` | ✅ 干净 |
| `grep` / `glob` | `NewContextEngine` | BUILTIN | `toolrunner.IToolRunner` | ✅ 干净 |
| `lsp` | `NewContextEngine` (conditional `cfg.LSPEnabled`) | LSPTOOL | `toolrunner.IToolRunner` | ⚠️ 需 cfg |
| `free_fork` | `NewContextEngine` → `RegisterFreeForkTool` | toolrunner + `toolrunner.globalFreeForker` | `toolrunner.IToolRunner` (via global) | ❌ global singleton |
| `query_diagnostics` | `NewContextEngine` → `RegisterTrackerTool` | toolrunner + `tracker.GlobalTracker` | `toolrunner.IToolRunner` (via global) | ❌ global singleton |
| `verify_plan_execution` | `NewContextEngine` → `RegisterVerifyTool` | toolrunner | `toolrunner.IToolRunner` | ✅ 干净 |
| `delegate_status` / `delegate_*` | `WireDelegate` | `delegate` package | `delegate.Runner` (独立于 D2 IToolRunner) | ❌ 旁路 IToolRunner |
| `task_output` / `task_list_background` | `multiagent.globalBackgroundTaskTools` | `multiagent` package | `multiagent.Runner` (独立于 D2 IToolRunner) | ❌ 旁路 IToolRunner + global |
| per-agent subset | `buildWithGate` | 重新调用各 `RegisterXxxTool` + 硬编码 allowlist | `toolrunner.IToolRunner` (新实例) | ❌ 三入口 + 硬编码 |

### 2.2 6+ global singleton 详情

```go
// 全部在 package-level var 持有，跨 bootstrap 共享，无 ctx 生命周期
var (
    globalFreeForker freefork.Forker              // toolrunner/freefork_register.go
    globalForker     freefork.Forker              // toolrunner/freefork_register.go (重复!)
    globalDeps       *ToolDeps                    // toolrunner/deps.go
    globalBackgroundTaskTools *BackgroundTools    // multiagent/background_tools.go
    GlobalTracker    *tracker.Tracker             // observability/diagnose/tracker/wire.go
    SetGlobalWriter  func(*transcript.Writer)     // communication/capture/transcript/wire.go
    GlobalHub        *notify.Hub                  // multiagent/notify/hub.go
    GlobalTaskManager *tasks.TaskManager          // multiagent/tasks/manager.go
    GlobalSessionQueue *tasks.SessionQueue        // multiagent/tasks/queue.go
)
```

每个 singleton 都是一条隐藏依赖线：测试时 reset 漏一个 → 串味；prod 时
顺序错一个 → nil pointer；改一个的实现 → 4 个 test 文件同时 break。

### 2.3 用户/Agent 痛感

- **「加 tool 三件套」**：开发体验 = 改 3 处 bootstrap + 1 个 global + 1
  份 allowlist，已被 3 次回归（PR #60/#58/#55）反复验证。
- **per-agent 模式盲区**：`buildWithGate` 漏注册的 tool 在 agent 模式下
  LLM 看不见，**目前没有任何回归测试覆盖这个等价性**。
- **测试 setup 噪音**：每个 tool 单测都要 `SetGlobalXxx(...)` + `defer reset()`，
  5 个 toolrunner 测试文件里能找到 8 处 reset 模式。
- **审计困难**：要知道 "LLM 此刻能看见哪些 tool" 必须同时看 3 个入口 +
  allowlist，没有 single source of truth。

## 3. 验收标准

### 3.1 P0（必须达成，否则不交付）

| ID | 标准 | 度量 |
|----|------|------|
| **AC1** | `internal/shared/contracts.ToolSurface` interface 定义完成，含 `Name() / Tools(ctx, workDir) []ToolSpec / RiskLevel(name) RiskLevel` 三方法 | `go doc` 输出 + 单测覆盖 mock 实现 |
| **AC2** | `internal/shared/contracts.ToolFilter` interface 定义完成，含 `Apply(specs []ToolSpec, ctx FilterCtx) []ToolSpec` 一方法 | 同上 |
| **AC3** | 7 个 surface 实现全部就位：BuiltinSurface / FreeForkSurface / TrackerSurface / VerifySurface / DelegateSurface / BackgroundTaskSurface / LSPToolSurface；每个 surface 持有自己需要的依赖（无 global） | 每个 surface 一个 `surface_test.go` 验证 `Tools()` 列表 + `RiskLevel()` 查询 |
| **AC4** | `toolrunner.globalFreeForker` / `globalForker` / `globalDeps` / `multiagent.globalBackgroundTaskTools` / `tracker.GlobalTracker` / `transcript.SetGlobalWriter` / `notify.GlobalHub` / `tasks.GlobalTaskManager` / `tasks.GlobalSessionQueue` 等 6+ global **全部下线**（代码删除 + grep 验证零引用） | `grep -rn "globalFreeForker\|GlobalTracker\|..." internal/` 输出为空 |
| **AC5** | `NewContextEngine` 收编为单 surface 列表构造：`engine := NewContextEngine(deps, []ToolSurface{builtin, freefork, tracker, ...})`；`buildWithGate` 与 `WireDelegate` 复用同一组 surface，只通过 `ToolFilter` 链裁剪可见性 | bootstrap_test 覆盖 3 入口等价性（main 看见的 ⊇ per-agent 看见的 ⊇ delegate 看见的） |
| **AC6** | `ContextPreparer.Prepare` 返回 `VisibleTools []ToolSpec`（替代当前 hardcoded `toolSchemas`）；至少 2 个内置 filter：`PerAgentFilter` (per-agent-type allowlist) + `PerRiskFilter` (per-mode threshold) | `prepare_test` 覆盖 filter 链组合（per-agent + per-risk 串联） |
| **AC7** | per-agent 模式 / 主模式 tool 集合等价性回归测试：100% 主模式注册的 tool 在 per-agent 模式下**至少**出现在 allowlist（per-agent 可选收紧） | `bootstrap_test.go` 新增 `TestBuildWithGate_SupersetOfMainEngineTools` |
| **AC8** | `turn_adapter.ExecuteRound` 不再直接调 `a.tools.Execute` 走 IToolRunner 内部 — 改走 `ToolSurface.Execute(ctx, name, input, workDir)` 路径（与 D2 legacy query/executor 走通同一条链路） | `turn_adapter_test.go` 现有 5 case 全绿 + 新增 "Surface 内部 delegate 给 IToolRunner" 单测 |
| **AC9** | 既有 P0 T 点（`D2-S4-A01-T01` LSP / `D4-S11-A02-T01` FreeFork / `D5-S23-A02-T01` Tracker / `D6-S11-A02-T01` Verifier）全部 PASS，**不修改既有单测期望** | `go test -race ./...` 100% 绿 |
| **AC10** | `go vet ./...` + `staticcheck ./...` 无新增 warning | CI |

### 3.2 P1（本期交付）

| ID | 标准 | 度量 |
|----|------|------|
| **AC11** | `ToolFilter` 链支持 `Composite(Filter, Filter...)` + `Allow(name)` / `Deny(name)` 内置 filter；`FilterCtx` 含 `SessionID / AgentType / Mode / RiskThreshold` 字段 | `filter_test.go` 覆盖 6 个组合 |
| **AC12** | `devrix.yaml` 新增 `tools:` 配置节：每个 surface 一个 `enabled: bool` + `risk_threshold: low|medium|high` 字段；缺省 = 启用 + 全部 risk | `config_test.go` 覆盖缺省 + 显式配置 |
| **AC13** | 新增 `devrix tool list` CLI 子命令，按 surface 分组 dump 当前 session 可见 tool 列表 + risk + description | `cli/tool/list_test.go` 覆盖输出格式 |
| **AC14** | 全部 `SetGlobalXxx` API 删除（不仅 var 删除，setter 函数也删） | `grep -rn "SetGlobal" internal/` 仅命中 `SetGlobalXxx` 注释/旧 doc，无代码调用 |

### 3.3 P2（本期不交付，需求锁定）

| ID | 标准 |
|----|------|
| **AC15** | Surface 动态加载（plugin loader 通过 `.so` 注入 surface） |
| **AC16** | ToolFilter 与 IPermissionGate 合并（permission 是 filter 的子集） |
| **AC17** | Surface 间依赖图（delegate 依赖 task，task 依赖 background）的 DAG 校验 |

### 3.4 质量基线

| ID | 标准 |
|----|------|
| **AC18** | 文件规模 < 800 行；函数 < 50 行；不可变性：surface 内部状态只在 `NewXxxSurface` 构造期固化 |
| **AC19** | 单测覆盖率 ≥ 80%；`go test -race ./...` 全绿；新增 `surface_contract_test.go` 覆盖 interface compliance（编译期 reflection 检查） |
| **AC20** | `verify-security` 闸门：ToolFilter 不能被业务代码绕过（per-agent filter 必须经过 `IPermissionGate` 才到 runner） |
| **AC21** | 不修改 D2/D3/D4/D5/D6 library 的对外 API；只新增 `internal/shared/contracts/` 两条 interface + 每个 surface 一个文件；`internal/bootstrap/` 收编 3 入口为 1 入口 |
| **AC22** | OpenSpec 归档后 `verify-archive.sh` 全部 PASS；新增 `openspec/specs/tool-surface/spec.md` Gherkin Scenario 至少 6 条 |

## 4. 依赖与约束

| 类型 | 内容 |
|------|------|
| 上游（已合并） | DM-20260617-006 (d7-tool-pipeline-permission) — hotfix 修了 D7↔D2 旁路，**不动其代码** |
| 上游（已合并） | DM-20260617-005 (diagnostic-tool-surface registration on main) — PR #58 main engine 修了 3 个缺失 tool，**本 change 把"补登记"模式替换为契约驱动** |
| 上游（已合并） | DM-20260617-002 (d7-turn-history-persist) — `turn_adapter.ExecuteRound` 已可被本 change 改造为走 surface |
| 上游（已合并） | DM-20260616-003 (diagnostic-tools-parity) — 13 个 library package 是 SoT，本 change 不动其对外 API |
| 约束 | library package 行为零变更（13 个 lib + toolrunner + multiagent + diagnose/tracker + notify + tasks + transcript） |
| 约束 | D2/D3/D4/D5/D6 现有 import 路径不变（避免依赖环） |
| 约束 | `internal/shared/contracts/` 不允许 import 任何 `internal/layers/*` |
| 约束 | `internal/bootstrap/` 仍可 import `internal/layers/*`（这是 DI 容器的特权） |
| 约束 | Surface 实现可以 import library，但 library 不可以 import surface（依赖方向：contracts ← surface ← library） |
| 约束 | 6+ global singleton 的删除走"**先收敛后删除**"两阶段：第一阶段所有 surface 持有显式 dep，第二阶段才删 global var（避免一次性爆破） |
| 约束 | `verify-security` 闸门：per-agent filter 不可被 `buildWithGate` 业务代码绕过 |
| 约束 | 文件规模 < 800 行；函数 < 50 行；surface 文件每个不超过 150 行（一个 tool 一个 surface） |

## 5. 变更范围

### 5.1 新增（contracts / surface / filter）

**`internal/shared/contracts/`（拆面契约）:**
- `tool_surface.go` — `ToolSurface` interface + `ToolSpec` struct + `SurfaceDeps` struct
- `tool_filter.go` — `ToolFilter` interface + `FilterCtx` struct + `Composite` / `Allow` / `Deny` helpers

**`internal/layers/contextengine/enforce/toolrunner/surface/`（surface 实现）:**
- `builtin_surface.go` — BUILTIN tools (read_file / write_file / edit_file / bash / grep / glob)
- `freefork_surface.go` — free_fork
- `tracker_surface.go` — query_diagnostics
- `verify_surface.go` — verify_plan_execution
- `lsptool_surface.go` — lsp (conditional on `cfg.LSPEnabled`)
- `delegate_surface.go` — delegate_status / delegate_*
- `background_task_surface.go` — task_output / task_list_background

**`internal/layers/contextengine/enforce/toolrunner/filter/`（filter 实现）:**
- `per_agent.go` — per-agent-type allowlist
- `per_risk.go` — per-mode risk threshold
- `per_session.go` — per-session override (P1)
- `composite.go` — 串联

**`openspec/specs/tool-surface/`:**
- `spec.md` — Gherkin Scenario（ToolSurface / ToolFilter / Composite / PerAgent / PerRisk / 3 入口等价性）
- `t-registry.md` — 6 个 P0 T 点 + 4 个 P1 T 点

### 5.2 修改（收编 3 入口为 1 入口）

- `internal/bootstrap/context_engine_builder.go` — `NewContextEngine` 改为接受 `[]ToolSurface`，删 `RegisterFreeForkTool` / `RegisterTrackerTool` / `RegisterVerifyTool` 等分散调用
- `internal/bootstrap/multiagent.go` — `buildWithGate` 复用同一组 surface，仅追加 `PerAgentFilter`
- `internal/bootstrap/wire_coordinator.go` — `WireDelegate` 复用同一组 surface，仅追加 `PerRiskFilter`
- `internal/layers/orchestration/turn/orchestrator.go` — `ContextPreparer.Prepare` 返回增加 `VisibleTools []ToolSpec`
- `internal/bootstrap/turn_adapter.go` — `ExecuteRound` 走 `Surface.Execute(...)` 而非 `a.tools.Execute` 裸调
- `internal/shared/config/contextengine.go` — 新增 `ToolsConfig` 结构（`Surfaces map[string]SurfaceConfig`）
- `cmd/devrix/main.go` — 新增 `tool list` 子命令；surface 装配从 yml 读

### 5.3 删除

- `toolrunner.globalFreeForker` / `globalForker` / `globalDeps`（var + setter + 全部 reset 模式）
- `multiagent.globalBackgroundTaskTools`（var + setter）
- `tracker.GlobalTracker` / `tracker.SetGlobalTracker`（var + setter）
- `transcript.SetGlobalWriter` / `transcript.GlobalWriter`（var + setter）
- `notify.GlobalHub` / `notify.SetGlobalHub`（var + setter）
- `tasks.GlobalTaskManager` / `tasks.SetGlobalTaskManager`（var + setter）
- `tasks.GlobalSessionQueue` / `tasks.SetGlobalSessionQueue`（var + setter）
- `freefork.SetGlobalFreeForker`（setter + var）

### 5.4 不变更

- 13 个 diagnostic-tools-parity library package（library 行为是 SoT）
- `toolrunner.PluginRunner` 4-method interface
- `IPermissionGate` / `ITokenCounter` / `IEngine` 既有 contract
- D1/D2/D3/D4/D5/D6 各 domain 的对外 API
- Feishu / IM 适配器
- `rule_orchestrate` 路由路径
- 既有 P0 T 测试点期望值

## 6. 风险评估

| 风险 | 影响 | 缓解 |
|------|------|------|
| **2 阶段删除 global 的中间态引入 nil 风险** | bootstrap 顺序错一个 → panic | 阶段 1 显式 dep 持有（global 仍可读，但只用于兼容）；阶段 2 才 `git grep` 验证零引用后删除；每阶段独立单测 |
| **`buildWithGate` 复用主 surface 后某些 per-agent 模式漏 tool** | per-agent LLM 看不见主 engine 可见的 tool | AC7 显式测等价性（`⊇`），per-agent 只收紧不放宽；新增 P0 T `TOOL-SURFACE-1-T01` 守护 |
| **Surface 接口设计过度抽象（"interfaces should be small" vs "领域表达力"）** | 后期添加 surface 累赘 | `ToolSpec` 用 struct（不要 method 链）；`RiskLevel(name)` 单独一方法而非 `Tools()` 内嵌 |
| **filter 链顺序敏感** | `(PerAgent, PerRisk)` 与 `(PerRisk, PerAgent)` 行为不同 → 测试难写 | `FilterCtx` 标注 `Order` 字段；`Composite` 强制 FIFO；显式 `PerAgentFirst()` / `PerRiskFirst()` helper |
| **`turn_adapter.ExecuteRound` 改造期间回归现有 5 个 P0 单测** | DM-006 hotfix 被改坏 | 改造前先 snapshot 5 个 case 的行为，改造后 case 必须 100% 兼容（input → output 字节级一致） |
| **delegate / background task surface 改造期间破坏 WireDelegate 调用图** | D7 路由回到 stub 行为 | 阶段 1 保留 WireDelegate 旧路径（adapter 模式），阶段 2 才切；新旧并行 2 周 |
| **D2 library 持有 surface 引用 → 依赖方向反转** | 触发 layering 规则破坏 | 明确 surface 是 `internal/layers/contextengine/enforce/toolrunner/surface/`（在 D2 内），library 不 import surface；surface import library |

## 7. Out of Scope

- **不修改** 13 个 diagnostic-tools-parity library 的对外 API
- **不重写** `toolrunner.PluginRunner` 4-method interface
- **不引入** 新 LLM provider / 新通信协议
- **不实现** Surface 动态 plugin loader（P2 AC15 锁定）
- **不合并** ToolFilter 与 IPermissionGate（P2 AC16 锁定）
- **不重构** `buildWithGate` 的 agent-type allowlist 配置文件位置（仍 `multiagent.yaml`）
- **不补** 2026-06-16 19:18 之后历史丢失的 session 数据
- **不动** `cmd/devrix/main.go` 已有的 `doctor` / `context-analyze` 子命令路由
- **不实现** tool 调用链的 OTEL span（与 `devrix-queryloop-spans-v1.1` 无关，本 change 跑在它之下）

## 8. 关联参考

- 上游 change：`openspec/changes/devrix-d7-turn-history-persist/`（DM-20260617-003+005+006，含 hotfix 闭合）
- 上游 change：`openspec/archive/2026-06-17-devrix-diagnostic-tools-wiring/`（DM-002，13 项 wiring）
- 上游 change：`openspec/archive/2026-06-17-devrix-diagnostic-tools-parity/`（DM-20260616-003，13 个 library）
- 上游 change：`openspec/archive/2026-06-17-devrix-queryloop-legacy-decommission/`（DM-017，QueryLoop 退役）
- DSAFT 方法论：`openspec/specs/project/master.md` + `docs/methodology/dsaft-methodology.md`
- 拆面契约参考：`openspec/specs/project/architecture-design.md` §1.1（Facet Decomposition）
- 域归档：`openspec/specs/d1-communication/` `d2-context-engine/` `d3-llm-gateway/` `d4-multi-agent/` `d5-observability/` `d6-evolution/`
- T 注册表：`openspec/t-registry.md`（根索引）+ 各域 `openspec/specs/d{N}-*/t-registry.md`

## 9. 检查清单（S1 完成确认）

- [x] DM ID 已分配：`DM-20260617-007`（当日序号 007）
- [x] demand.md 包含背景、问题、验收标准、范围
- [x] 10 个 P0 验收标准（AC1-AC10）+ 4 个 P1（AC11-AC14）+ 3 个 P2（AC15-AC17）
- [x] 5 个质量基线（AC18-AC22）
- [x] Out of Scope 已明确（§7）
- [x] DSAFT 域标注正确（multi-domain，覆盖 D1/D2/D3/D4/D5 + tool-surface 新增横切）
- [x] 风险评估含影响与缓解（§6）
- [x] 跨域边界已声明（§4 约束 + §5.4 不变更）
- [x] 不动 DM-20260617-003+005+006 hotfix 代码
- [x] 不动 13 个 diagnostic-tools-parity library

---

> **S1 → S2 接力**: 用户确认后，将按 S2 (`proposal.md`) 走方案对比与决策，
> S3 (`design.md`) 落接口与文件清单，S4 (`tasks.md`) 列 W 编号与估时。
> 由于本 change 涉及 D2 核心装配链改造，建议 S3-Gate 走严格 review
> （按 `openspec/specs/project/review-design.md` 5 段式审查清单）。
