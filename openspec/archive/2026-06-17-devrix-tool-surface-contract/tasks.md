# Tasks: devrix-tool-surface-contract

**Change ID:** devrix-tool-surface-contract
**Demand ID:** DM-20260617-007
**Status:** S4_Implementation
**估算参考（仅供参考，非承诺）:** 4 Phase × 14 W, ~+1260 LOC (含测试)

---

> **DSAFT Activity 一览**
>
> 本 change 复用既有 DSAFT Activity 节点（free_fork = D4-S11-A02 + D4-S13-A02，
> tracker = D5-S23-A02，verify = D6-S11-A02，lsp = D2-S4-A01，delegate =
> D4-S12-A02/03 等）。W 编号按 Phase 组织，每个 W 标注关联 Activity / 新增
> 拆面契约域 `TOOL-SURFACE-1`。
>
> **两阶段执行**：阶段 1（W1-W9）落 surface + filter + 3 入口收编，**不删
> global**；阶段 2（W10-W12）验证零引用后删除 global + CLI 子命令 +
> 归档。

## Phase 1: 拆面契约 + 单元测试（W1-W2，最先做）

### W1 — TOOL-SURFACE-1 ToolSurface + ToolSpec + ToolResult 契约

- **文件 1:** `internal/shared/contracts/tool_surface.go` (新建, ~60 行)
- **改动:** 4 方法 interface（`Name` / `Tools` / `RiskLevel` / `Execute`）
  + `ToolSpec` struct + `ToolResult` struct
- **文件 2:** `internal/shared/contracts/tool_surface_test.go` (新建, ~60 行)
  - `TestToolSurface_InterfaceCompliance` (用 mock struct 验证 4 方法齐全)
  - `TestToolSpec_Struct` (字段验证)
  - `TestToolResult_Struct` (字段验证)
- **依赖:** 无
- **AC:** AC1
- **T:** TOOL-SURFACE-1-T01
- **估时参考:** 60 min

### W2 — TOOL-SURFACE-1 ToolFilter + FilterCtx + Composite / Allow / Deny

- **文件 1:** `internal/shared/contracts/tool_filter.go` (新建, ~80 行)
- **改动:** 1 方法 interface（`Apply(specs, ctx) []ToolSpec`）
  + `FilterCtx` struct + `Composite(...)` + `Allow(names...)` + `Deny(names...)`
  + `ApplyFilters(surfaces, filters, ctx) []ToolSurface` 辅助函数
- **文件 2:** `internal/shared/contracts/tool_filter_test.go` (新建, ~100 行)
  - `TestComposite_FIFO`
  - `TestComposite_Empty`
  - `TestAllow_Allowlist`
  - `TestDeny_Blocklist`
  - `TestFilterCtx_Zero`
  - `TestApplyFilters_AllPass`
  - `TestApplyFilters_BlockedNotExecutable`
- **依赖:** W1
- **AC:** AC2, AC11 (部分)
- **T:** TOOL-SURFACE-1-T02
- **估时参考:** 90 min

## Phase 2: 7 个 surface 落地（W3-W5）

### W3 — TOOL-SURFACE-1 BuiltinSurface + LSPToolSurface

- **文件 1:** `internal/layers/contextengine/enforce/toolrunner/surface/builtin_surface.go` (新建, ~90 行)
- **改动:** 持有 `*config.ToolConfig`，Tools() 返回 6 个 spec (read_file / write_file / edit_file / bash / grep / glob)，RiskLevel 按名查
- **文件 2:** `internal/layers/contextengine/enforce/toolrunner/surface/lsptool_surface.go` (新建, ~60 行)
- **改动:** 持有 lsp config，Tools() 按 `cfg.LSPEnabled` 条件返回 0/1 个 spec
- **文件 3:** `internal/layers/contextengine/enforce/toolrunner/surface/builtin_surface_test.go` (新建, ~80 行)
- **文件 4:** `internal/layers/contextengine/enforce/toolrunner/surface/lsptool_surface_test.go` (新建, ~50 行)
- **依赖:** W1
- **AC:** AC3 (部分)
- **T:** TOOL-SURFACE-1-T03
- **估时参考:** 120 min

### W4 — TOOL-SURFACE-1 FreeForkSurface + TrackerSurface + VerifySurface

- **文件 1:** `internal/layers/contextengine/enforce/toolrunner/surface/freefork_surface.go` (新建, ~110 行)
- **改动:** 持有 `freefork.Forker` 显式 dep，**不**用 globalFreeForker；Tools() 1 个 spec；Execute 内 freeforkInput 解析 + s.forker.Fork(...) + JSON 序列化
- **文件 2:** `internal/layers/contextengine/enforce/toolrunner/surface/tracker_surface.go` (新建, ~100 行)
- **改动:** 持有 `*tracker.Tracker` 显式 dep，**不**用 GlobalTracker；Execute 内调 s.tracker.Query(...)
- **文件 3:** `internal/layers/contextengine/enforce/toolrunner/surface/verify_surface.go` (新建, ~80 行)
- **改动:** 持有 `verify.Verifier` 显式 dep；Execute 内调 s.verifier.Verify(...)
- **文件 4-6:** 3 个 `*_test.go`，每个 ~80 行
  - `TestFreeForkSurface_Execute_Batch3` / `Rollback` / `TooMany`
  - `TestTrackerSurface_Execute_AfterTick` / `NoTick`
  - `TestVerifySurface_Execute_AllDone` / `MissingFile`
- **依赖:** W1, W3
- **AC:** AC3 (部分), AC4 (阶段 1: 显式 dep 持有, 旧 global 仍可读)
- **T:** TOOL-SURFACE-1-T04
- **估时参考:** 180 min

### W5 — TOOL-SURFACE-1 DelegateSurface + BackgroundTaskSurface

- **文件 1:** `internal/layers/contextengine/enforce/toolrunner/surface/delegate_surface.go` (新建, ~100 行)
- **改动:** 持有 `delegate.Runner` 显式 dep；Tools() 4 个 spec (delegate_explore / plan / implement / status)；Execute 内调 s.runner.Dispatch(...)
- **文件 2:** `internal/layers/contextengine/enforce/toolrunner/surface/background_task_surface.go` (新建, ~100 行)
- **改动:** 持有 `multiagent.Runner` 显式 dep；Tools() 2 个 spec (task_output / task_list_background)；Execute 内调 s.runner.Query(...)
- **文件 3-4:** 2 个 `*_test.go`，每个 ~80 行
  - `TestDelegateSurface_Execute_Explore` / `Status`
  - `TestBackgroundTaskSurface_Execute_TaskOutput` / `ListBackground`
- **依赖:** W1, W4
- **AC:** AC3 (剩余)
- **T:** TOOL-SURFACE-1-T05
- **估时参考:** 150 min

## Phase 3: 3 个 filter 落地 + toolpolicy 适配（W6-W7）

### W6 — TOOL-SURFACE-1 PerAgentFilter + PerRiskFilter + Composite

- **文件 1:** `internal/layers/contextengine/enforce/toolrunner/filter/per_agent.go` (新建, ~50 行)
- **改动:** `allowlist map[agentType]map[toolName]bool`，Apply 按 ctx.AgentType 过滤
- **文件 2:** `internal/layers/contextengine/enforce/toolrunner/filter/per_risk.go` (新建, ~50 行)
- **改动:** Apply 按 `ToolSpec.Risk <= ctx.RiskThreshold` 过滤
- **文件 3:** `internal/layers/contextengine/enforce/toolrunner/filter/composite.go` (新建, ~30 行)
- **改动:** 已经在 W2 落 contracts 里的 Composite；这里是 D2 内的 helper / re-export
- **文件 4:** `internal/layers/contextengine/enforce/toolrunner/filter/per_agent_test.go` (新建, ~70 行)
  - 6 个 case: main / explore / plan / worker / fix / delegate
- **文件 5:** `internal/layers/contextengine/enforce/toolrunner/filter/per_risk_test.go` (新建, ~50 行)
  - 3 个 case: low / medium / high
- **依赖:** W1, W2
- **AC:** AC6 (部分)
- **T:** TOOL-SURFACE-1-T06
- **估时参考:** 120 min

### W7 — TOOL-SURFACE-1 toolpolicy.Filter AsToolFilter 适配器

- **文件:** `internal/layers/orchestration/toolpolicy/filter.go` (修改, +30 行)
- **改动:** 新增 `AsToolFilter() contracts.ToolFilter` 方法，**内部复用**
  `FilterToolsForAgentRole` 逻辑，仅做 type 转换 (`[]ToolSpec` ↔
  `[]ToolSchema`)
- **测试:** `internal/layers/orchestration/toolpolicy/filter_test.go` (修改, +30 行)
  - `TestFilter_AsToolFilter_Equivalence` 验证与原 `Filter` 行为一致
- **依赖:** W1, W2
- **AC:** AC5 (部分), AC6 (部分)
- **T:** TOOL-SURFACE-1-T07
- **估时参考:** 45 min

## Phase 4: 3 入口收编为 1 入口（W8-W9，阶段 1 收尾）

### W8 — TOOL-SURFACE-1 NewContextEngine 收编为 surface 列表构造

- **文件 1:** `internal/layers/contextengine/engine_types.go` (修改, +5 行)
- **改动:** `EngineDeps` 新增 `Surfaces []ToolSurface` + `Filters []ToolFilter`
- **文件 2:** `internal/layers/contextengine/tool_context.go` (修改, +10 行)
- **改动:** 新增 `ToolContextWithSurfaces(ctx, surfaces, filters) ctx`，把 surface 列表塞 ctx
- **文件 3:** `internal/bootstrap/context_engine.go` (修改, -50 / +30 行)
- **改动:** `NewContextEngine` 接受 `[]ToolSurface` 参数；删 7 个 `toolrunner.RegisterXxxTool` 分散调用；删 6+ `SetGlobalXxx` 调用
- **文件 4:** `internal/bootstrap/context_engine_builder.go` (修改, -60 / +20 行)
- **改动:** `buildWithGate` 复用同一 surface 列表；增加 `Filters: []ToolFilter{toolpolicy.NewFilter().AsToolFilter()}`
- **文件 5:** `internal/bootstrap/context_engine.go` (修改) — 加
  `TestNewContextEngine_WithSurfaces` 单测覆盖 main 模式 18 个 tool
- **文件 6:** `internal/bootstrap/context_engine_builder_test.go` (修改, +50 行)
  - `TestBuildWithGate_SupersetOfMainEngineTools` (AC7)
  - `TestBuildWithGate_Explore_AppliesFilter`
  - `TestBuildWithGate_Worker_AppliesFilter`
- **依赖:** W3, W4, W5, W6, W7
- **AC:** AC5, AC7, AC8 (部分)
- **T:** TOOL-SURFACE-1-T08
- **估时参考:** 180 min

### W9 — TOOL-SURFACE-1 turn_adapter.ExecuteRound 走 surface.Execute

- **文件 1:** `internal/bootstrap/turn_adapter.go` (修改, -20 / +30 行)
- **改动:**
  - contextEngineAdapter 新增 `surfaces []contracts.ToolSurface` 字段
  - newContextEngineAdapter 注入 surfaces
  - ExecuteRound 改：删 `a.tools.Execute` 直调；改 `a.findSurface(tc.Name) → surf.Execute(...)`
  - findSurface 线性扫 O(7)
  - risk 走 `surf.RiskLevel(name)` 而非 `a.toolsReg.RiskLevel(name)`
- **文件 2:** `internal/bootstrap/turn_adapter_permission_test.go` (修改, +40 行)
  - 现有 5 case 全部 100% 兼容
  - 新增 `TestExecuteRound_GoesThroughSurface_NotThroughIToolRunner`
  - 新增 `TestExecuteRound_FindSurface_NotFound`
- **依赖:** W8
- **AC:** AC8, AC9 (不破既有 5 case)
- **T:** TOOL-SURFACE-1-T09
- **估时参考:** 90 min

## Phase 5: 阶段 1 全量回归 + 灰度（W10，里程碑）

### W10 — TOOL-SURFACE-1 阶段 1 全量回归 + E2E IM 验证

- **文件 1:** `tests/integration/tool_surface_test.go` (新建, ~200 行)
  - `TestNewContextEngine_MainMode_AllTools`
  - `TestNewContextEngine_ExploreMode_ReadOnly`
  - `TestNewContextEngine_WorkerMode_Full`
  - `TestBuildWithGate_SupersetOfMainEngineTools` (已在 W8 写)
  - `TestFilterChain_TwoFilters`
  - `TestTurnAdapter_GoesThroughSurface`
  - `TestTurnAdapter_PermissionDenied` (已在 W9 写)
- **文件 2:** `tests/e2e/im_tool_surface_test.go` (新建, ~150 行)
  - 5 步骤 E2E（飞书发 free_fork / query_diagnostics / verify / delegate / worker-隐藏-delegate）
- **文件 3:** `openspec/specs/tool-surface/spec.md` (新建, ~150 行)
  - 6 个 Gherkin Scenario
- **文件 4:** `openspec/specs/tool-surface/t-registry.md` (新建, ~50 行)
  - 6 P0 T 点 + 4 P1 T 点
- **验证:**
  - `go test -race ./...` 100% 绿
  - `go vet ./...` + `staticcheck ./...` 无新增 warning
  - 6+ global var 全部仍可读（阶段 1 保留）
  - 飞书 IM E2E 5 步全部通过
- **AC:** AC9, AC10, AC19, AC22
- **依赖:** W1-W9
- **估时参考:** 240 min

## Phase 6: 阶段 2 — 全局 singleton 删除 + CLI 子命令（W11-W12）

### W11 — TOOL-SURFACE-1 删除 6+ global var + setter

- **操作 1:** `git grep` 验证 6+ global var (`globalFreeForker` / `SetFreeForker`
  / `globalForker` / `globalDeps` / `globalBackgroundTaskTools` /
  `GlobalTracker` / `SetGlobalTracker` / `GlobalWriter` / `SetGlobalWriter` /
  `GlobalHub` / `SetGlobalHub` / `GlobalTaskManager` / `SetGlobalTaskManager` /
  / `GlobalSessionQueue` / `SetGlobalSessionQueue` / `SetGlobalFreeForker`)
  零引用（除注释外）
- **操作 2:** 删除上述 6+ global var 声明
- **操作 3:** 删除所有 `SetGlobalXxx` setter 函数
- **操作 4:** 删除 8 处测试 setup 的 `defer reset()` 模式
- **验证:** `go test -race ./...` 100% 绿（**重要**：阶段 2 不能 break 任何既有 case）
- **AC:** AC4, AC14
- **T:** TOOL-SURFACE-1-T10
- **估时参考:** 120 min

### W12 — TOOL-SURFACE-1 devrix tool list CLI + config 节

- **文件 1:** `internal/shared/config/contextengine.go` (修改, +25 行)
  - `ToolsConfig` struct + `Surfaces map[string]SurfaceConfig`
  - `SurfaceConfig{Enabled bool, RiskThreshold types.RiskLevel}`
- **文件 2:** `internal/cli/tool/list.go` (新建, ~80 行)
  - `devrix tool list` 子命令
  - `--agent main|explore|...` flag
  - `--format json|text` flag
- **文件 3:** `internal/cli/root.go` (修改, +10 行)
  - `tool list` 注册
- **文件 4:** `internal/cli/tool/list_test.go` (新建, ~50 行)
  - 覆盖输出格式
- **文件 5:** `internal/shared/config/contextengine_test.go` (修改, +30 行)
  - 缺省 + 显式配置 case
- **依赖:** W8, W11
- **AC:** AC12, AC13
- **T:** TOOL-SURFACE-1-T11
- **估时参考:** 90 min

## Phase 7: 集成测试 + 验收 + 归档（W13-W14）

### W13 — TOOL-SURFACE-1 集成 + 验收

- **文件 1:** `openspec/changes/devrix-tool-surface-contract/acceptance-report.md` (新建, ~80 行)
  - 22 个 AC 全部 PASS / FAIL 状态
  - 4 张表：14 个既有 P0 T 点 + 6 个新 P0 T 点 + 4 个 P1 T 点
  - 6+ global 引用数: 104 → 0
  - 3 入口数: 3 → 1
  - per-agent 漏 tool 数: 7 (理论) → 0 (实测)
- **文件 2:** `docs/methodology/dsaft-methodology.md` (修改, +30 行)
  - 补充"Facet Decomposition" 案例: ToolSurface + ToolFilter
  - 引用本次 change 的 contracts/tool_surface.go / tool_filter.go
- **依赖:** W1-W12
- **AC:** AC18, AC20, AC21
- **估时参考:** 60 min

### W14 — TOOL-SURFACE-1 S6 归档

- **操作 1:** `git add` + `git commit`（独立 commit，按 Phase 组织）
- **操作 2:** `bash scripts/verify-archive.sh` 全部通过
- **操作 3:** 归档到 `openspec/archive/2026-06-17-devrix-tool-surface-contract/`
- **操作 4:** 开 PR（squash merge + auto-merge）
- **操作 5:** S7_Archived 状态
- **依赖:** W13
- **AC:** AC22
- **估时参考:** 30 min

## 总览

| Phase | W 编号 | 主题 | 估时参考 | AC |
|-------|--------|------|---------|----|
| P1 契约 | W1-W2 | ToolSurface + ToolFilter | 150 min | AC1, AC2, AC11 |
| P2 7 surface | W3-W5 | Builtin/LSP/FreeFork/Tracker/Verify/Delegate/Background | 450 min | AC3, AC4 (阶段 1) |
| P3 3 filter | W6-W7 | PerAgent/PerRisk/Composite/toolpolicy 适配 | 165 min | AC5, AC6 |
| P4 收编入口 | W8-W9 | NewContextEngine + buildWithGate + turn_adapter | 270 min | AC5, AC7, AC8, AC9 |
| P5 阶段 1 验证 | W10 | 全量回归 + E2E + spec.md | 240 min | AC9, AC10, AC19, AC22 |
| P6 阶段 2 删 global | W11-W12 | 6+ global 删除 + CLI | 210 min | AC4, AC12, AC13, AC14 |
| P7 集成 + 归档 | W13-W14 | acceptance-report + verify-archive.sh | 90 min | AC18, AC20, AC21, AC22 |
| **合计** | **14** | **7 surface + 3 filter + 2 契约 + 1 入口收编 + 1 turn 改造** | **~25.8 h** | **AC1-AC22** |

> **注意：** 估时仅供参考，非承诺。实际进度按 Phase 推进。

## 执行顺序

1. **W1 → W2**（Phase 1 — 先有契约）
2. **W3 → W4 → W5**（Phase 2 — surface 可与 W6 并行）
3. **W6 → W7**（Phase 3 — filter 依赖 W1-W2）
4. **W8 → W9**（Phase 4 — 收编 3 入口，依赖 W3-W7 全绿）
5. **W10**（Phase 5 — 阶段 1 验证，依赖 W1-W9）
6. **W11 → W12**（Phase 6 — 阶段 2 删 global，依赖 W10 全绿）
7. **W13 → W14**（Phase 7 — 集成 + 归档，依赖 W1-W12）

每个 W 完成后立即 `git add` + `git commit`（独立 commit），便于回滚
与 review。

## 风险与缓解

| 风险 | 等级 | 缓解 |
|------|------|------|
| 阶段 1 期间 6+ global 仍可读，新代码可能混用新旧路径 | M | W10 全量 E2E 验证；W11 阶段 2 之前 `git grep` 验证零引用 |
| 阶段 2 删 global 时漏一个调用方 | H | W11 第一步就是 `git grep` 验证零引用；零引用后才动手 |
| `turn_adapter.ExecuteRound` 改造回归 DM-006 5 个 P0 单测 | H | W9 改造前先 snapshot 5 case 行为；改造后 100% 兼容 |
| `toolpolicy.AsToolFilter` 适配器破坏 DM-20260614-015 单测 | M | W7 复用 `FilterToolsForAgentRole` 内部逻辑；新增 `TestFilter_AsToolFilter_Equivalence` 验证 |
| `findSurface` 性能（O(7 × spec) 每次 tool call） | L | 实际 surface 数 ≤ 7，spec ≤ 3/surface；总开销 < 1µs；无需 hash |
| W10 阶段 1 E2E 失败 | H | 阶段 1 完整回滚方案：revert W1-W9，6+ global 仍可工作 |
| OpenSpec S3-Gate review 拒绝 | L | 严格遵循 `review-design.md` 5 段式；design.md 已含 5 Decision |

## 文件交付清单（按 W 汇总）

详见 design.md §3 文件清单 + §10 测试矩阵。

### 新增文件 (~25 个)

**`internal/shared/contracts/` (+4):**
- `tool_surface.go` (W1)
- `tool_filter.go` (W2)
- `tool_surface_test.go` (W1)
- `tool_filter_test.go` (W2)

**`internal/layers/contextengine/enforce/toolrunner/surface/` (+14):**
- 7 个 `*_surface.go` (W3-W5)
- 7 个 `*_surface_test.go` (W3-W5)

**`internal/layers/contextengine/enforce/toolrunner/filter/` (+5):**
- 3 个 `*.go` (W6)
- 2 个 `*_test.go` (W6)

**`internal/cli/tool/` (+2):**
- `list.go` (W12)
- `list_test.go` (W12)

**`openspec/specs/tool-surface/` (+2):**
- `spec.md` (W10)
- `t-registry.md` (W10)

**`tests/integration/` (+1):**
- `tool_surface_test.go` (W10)

**`tests/e2e/` (+1):**
- `im_tool_surface_test.go` (W10)

**`openspec/changes/devrix-tool-surface-contract/` (+1):**
- `acceptance-report.md` (W13)

### 修改文件 (~10 个)

- `internal/layers/orchestration/toolpolicy/filter.go` (W7, +30 行)
- `internal/layers/orchestration/toolpolicy/filter_test.go` (W7, +30 行)
- `internal/layers/contextengine/engine_types.go` (W8, +5 行)
- `internal/layers/contextengine/tool_context.go` (W8, +10 行)
- `internal/bootstrap/context_engine.go` (W8, -50/+30 行)
- `internal/bootstrap/context_engine_builder.go` (W8, -60/+20 行)
- `internal/bootstrap/context_engine_builder_test.go` (W8, +50 行)
- `internal/bootstrap/turn_adapter.go` (W9, -20/+30 行)
- `internal/bootstrap/turn_adapter_permission_test.go` (W9, +40 行)
- `internal/shared/config/contextengine.go` (W12, +25 行)
- `internal/shared/config/contextengine_test.go` (W12, +30 行)
- `internal/cli/root.go` (W12, +10 行)
- `docs/methodology/dsaft-methodology.md` (W13, +30 行)

### 删除文件/代码 (~100 行)

- 6+ global var 声明 + setter 函数 (W11)
- 8 处 `defer reset()` 测试 setup 模式 (W11)

## T 层测试点登记

| T ID | 描述 | 阶段 | W |
|------|------|------|---|
| TOOL-SURFACE-1-T01 | ToolSurface 4 方法 interface compliance | P0 | W1 |
| TOOL-SURFACE-1-T02 | ToolFilter + Composite + Allow + Deny | P0 | W2 |
| TOOL-SURFACE-1-T03 | 7 个 surface.Tools() + RiskLevel 查询 | P0 | W3-W5 |
| TOOL-SURFACE-1-T04 | 7 个 surface.Execute 行为 | P0 | W3-W5 |
| TOOL-SURFACE-1-T05 | PerAgentFilter + PerRiskFilter | P0 | W6 |
| TOOL-SURFACE-1-T06 | toolpolicy.AsToolFilter 行为等价 | P0 | W7 |
| TOOL-SURFACE-1-T07 | NewContextEngine 接受 surface 列表 | P0 | W8 |
| TOOL-SURFACE-1-T08 | buildWithGate ⊇ main (AC7) | P0 | W8 |
| TOOL-SURFACE-1-T09 | turn_adapter.ExecuteRound 走 surface.Execute | P0 | W9 |
| TOOL-SURFACE-1-T10 | 6+ global 零引用 (阶段 2) | P1 | W11 |
| TOOL-SURFACE-1-T11 | devrix tool list CLI 输出 | P1 | W12 |
| TOOL-SURFACE-1-T12 | PerSessionFilter (P1) | P1 | (S6 backlog) |
| TOOL-SURFACE-1-T13 | Surface 动态 plugin loader (P2) | P2 | (S6 backlog) |
| TOOL-SURFACE-1-T14 | ToolFilter + IPermissionGate 合并 (P2) | P2 | (S6 backlog) |

---

> **S4 → S5 接力**: 14 个 W 全部 PASS 后，进入 S5 验收（按
> `openspec/specs/project/testing.md` + `acceptance-report.md` §22 AC）。
> S5 通过后 S6 归档（PR + `verify-archive.sh`）。
