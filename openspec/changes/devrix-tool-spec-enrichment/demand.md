# S1 需求文档：ToolSpec orthogonal flags + InterruptBehavior + BuildSurfaces sort

**DM ID:** DM-20260618-001
**Change ID:** devrix-tool-spec-enrichment
**状态:** Draft
**创建日期:** 2026-06-18
**父 change:** devrix-tool-surface-contract (DM-007) + devrix-tool-surface-phase2-full (DM-008)
**需求源:** clawcode-tool-design-comparison-2026-06-17 (P0 借鉴清单)

---

## 1. 需求概述

| 字段 | 内容 |
|------|------|
| **功能名称** | ToolSpec orthogonal flags + InterruptBehavior + BuildSurfaces sort |
| **所属域** | 横切契约域 TOOL-SURFACE-1 (D2/D7 复用) |
| **优先级** | **P0** |
| **预计工时** | 2-3 人日 |
| **下游 change** | devrix-surface-permission-extension (DM-002), devrix-surface-lazy-loading (DM-003) |

### 1.1 问题背景

devrix 的 `contracts.ToolSurface` 在 DM-007 拆面契约定型后，已是 4 方法小 interface（Name / Tools / RiskLevel / Execute）。但对照 clawcode (Claude Code v2.1.88) 的 Tool interface 抽象（30+ 方法），发现 3 类信息缺失：

1. **正交关注点挤压到单 RiskLevel enum** —— `ReadOnly / Destructive / OpenWorld / ConcurrencySafe` 4 个独立维度目前全部映射到 `types.RiskLevel` 单字符串 enum，**PerAgentFilter / PerRiskFilter 无法基于这些维度做精细决策**。
   - 实测：explore agent 想自动扩 "ReadOnly tools" 集合（devrix §3 review 提的），但当前 schema 没"ReadOnly"标志，只能用 allowlist 手动登记
   - 实测：plan_mode 想收紧 "OpenWorld tools"（任何会发网络请求的 tool），但当前没有"OpenWorld" 标志

2. **长 run tool 缺 interrupt 协议** —— FreeFork / TaskOutput / LSPTool 可能运行 30s+，用户发新消息时 devrix 不知道该 cancel 还是 block。
   - clawcode 显式有 `interruptBehavior: 'cancel' | 'block'`（Tool.ts:410-416）
   - devrix 当前无：只能等 ctx cancel 或 timeout（最长 60s）

3. **BuildSurfaces 输出顺序不稳定** —— 当前按"添加顺序"返回 surface list，**当 lsp_enabled / tracker 存在 / forker 注入等条件变化时顺序抖动**，破坏 LLM prompt cache。
   - clawcode 显式做 byName sort + uniqBy（tools.ts:362-366，注释："prompt-cache stability, keeping built-ins as a contiguous prefix"）
   - devrix 当前无：env 差异 → LLM 收到不一致的 tool schema 列表 → cache miss

### 1.2 目标

在 1 个 change 内、零侵入、不改 library API 的前提下：

1. **ToolSpec 加 4 个 orthogonal bool flag** —— 让 PerAgentFilter / PerRiskFilter / turn_adapter 能基于精细维度决策
2. **ToolSurface 加 InterruptBehavior 方法** —— 让长 run surface 显式声明 cancel/block 策略
3. **BuildSurfaces sort.Slice by name** —— 1 行代码提升 prompt cache hit rate
4. **turn_adapter 按 ConcurrencySafe 并行 dispatch** —— 2-5x throughput 提升（实测 Bash+Read+Edit 同时无依赖）
5. **7 surface 全部改 Risk 查询 → ToolSpec 填充** —— mechanical 改动，确保 T22 100% 覆盖

---

## 2. DSAFT 结构

### 2.1 D 领域

| 域 | 类型 | 影响 | 理由 |
|---|---|---|---|
| **TOOL-SURFACE-1** | 横切契约 | **新增方法/字段** | ToolSurface 5 方法、ToolSpec +4 bool |
| **D2-ContextEngine** | 核心 | 修改 | turn_adapter.ExecuteRound 并行 dispatch |
| **D7-Orchestration** | 核心 | 修改 | turn 调度决策消费 ConcurrencySafe |
| **D2/D7 library** | 核心 | **零修改** | 本 change 严守 AC21，不动 freefork / tracker / verify 等 library API |

### 2.2 S 场景

| 场景 ID | 名称 | 触发条件 | 用户目标 | 涉及 A |
|---|---|---|---|---|
| **TOOL-SURFACE-1-S2** | Tool Spec 元信息扩展 | LLM 调 tool 前的 schema 决策 | 用精细标志替代单 RiskLevel | A01（ToolSpec 新字段） |
| **TOOL-SURFACE-1-S3** | Long Run Tool Interrupt | 用户在 free_fork 跑时发新消息 | 5s 内 cancel 而非等 60s timeout | A01（InterruptBehavior 方法） |
| **D7-S6** | Parallel Tool Dispatch | turn 有 N 个独立 tool call 同时 | 并发执行提速 2-5x | A03（turn_adapter 并行化） |
| **D2-S7** | Surface List Stability | 不同 env 配置 (lsp / tracker 开关) | prompt cache hit rate 提升 | A05（BuildSurfaces sort） |

### 2.3 A 活动

| 活动 ID | 名称 | 类型 | 输入 | 输出 | 状态变更 |
|---|---|---|---|---|---|
| **TOOL-SURFACE-1-A01** | ToolSurfaceContract v2 | A-BE | 4 方法 v1 | 5 方法 v2（含 InterruptBehavior） | interface breaking change |
| **TOOL-SURFACE-1-A01-F02** | ToolSpec Orthogonal Flags | F-BE | ToolSpec v1 | ToolSpec v2（+4 bool） | struct field addition |
| **TOOL-SURFACE-1-A01-F05** | InterruptBehavior Method | F-BE | toolName string | InterruptMode ('cancel'/'block') | surface method addition |
| **D7-S6-A01** | Parallel Tool Dispatch | A-BE | []ToolCall + ConcurrencySafe flags | []ToolResult（按原顺序） | turn 调度策略变化 |

### 2.4 F 功能点

| F ID | 名称 | 所属 A | 输入 | 输出 |
|---|---|---|---|---|
| TOOL-SURFACE-1-A01-F02-F01 | bool field ReadOnly 填充 | A01-F02 | tool name | bool |
| TOOL-SURFACE-1-A01-F02-F02 | bool field Destructive 填充 | A01-F02 | tool name | bool |
| TOOL-SURFACE-1-A01-F02-F03 | bool field OpenWorld 填充 | A01-F02 | tool name | bool |
| TOOL-SURFACE-1-A01-F02-F04 | bool field ConcurrencySafe 填充 | A01-F02 | tool name | bool |
| TOOL-SURFACE-1-A01-F05-F01 | InterruptMode enum 定义 | A01-F05 | — | `cancel` / `block` |
| TOOL-SURFACE-1-A01-F05-F02 | surface 默认 InterruptBehavior | A01-F05 | toolName | InterruptMode |
| TOOL-SURFACE-1-A05-F02 | BuildSurfaces sort by name | A05 | []ToolSurface（add 顺序） | []ToolSurface（name 字典序） |
| D7-S6-A01-F01 | turn_adapter 并行 dispatch | D7-S6-A01 | req.ToolCalls + ConcurrencySafe 集合 | ToolRoundResult |

---

## 3. 验收标准

### 3.1 P0（必须达成，否则不交付）

| ID | 标准 | 度量 | 关联 T 点 |
|---|---|---|---|
| **AC1** | `ToolSpec` struct 加 4 bool 字段：`ReadOnly / Destructive / OpenWorld / ConcurrencySafe` | `git grep -n "ReadOnly\|Destructive\|OpenWorld\|ConcurrencySafe" internal/shared/contracts/tool_surface.go` 命中 4 个字段定义 | T22 |
| **AC2** | `ToolSurface` interface 加 `InterruptBehavior(name string) InterruptMode` 方法 | compile-time `var _ contracts.ToolSurface = ...` 7 个 surface 全部 PASS | T23 |
| **AC3** | `InterruptMode` enum 定义：`"cancel" \| "block"`，默认 `"block"` | 缺省时 surface 不需要重写方法（interface 提供 default） | T23 |
| **AC4** | 7 个 surface 全部 ToolSpec 填充 4 bool 字段（覆盖度 100%） | 7 个 `surface_test.go` 各 1 个 `TestXxxSurface_ToolSpec_HasOrthogonalFlags` 子测试 | T22 |
| **AC5** | `BuildSurfaces` 输出按 surface name 字典序排序 | `git diff BuildSurfaces(opts1).Names() BuildSurfaces(opts2).Names()` 在 env 变化时（lsp enabled / tracker 存在）输出**完全相同** | T24 |
| **AC6** | `turn_adapter.ExecuteRound` 对 `ConcurrencySafe=true` 的 tool 并行 dispatch（goroutine + errgroup），结果按原顺序返回 | 集成测试：2 个独立 read_file 并行执行时间 < 单个 read_file × 1.5（而非 2.0） | T25 |
| **AC7** | 长 run surface 收到 ctx.Done() 时 ≤ 200ms 内返回 Cancel 错误 | 集成测试：FreeForkSurface.Execute 在 ctx cancel 后 200ms 内返回 ctx.Err() | T23 |
| **AC8** | 既有 11 个 P0 T 点（DM-007 留下的 9 + DM-008 留下的 2）全部 PASS | `go test -race ./...` 100% 绿（除 1 个已登记的 flaky test） | T01-T11 |
| **AC9** | `go vet ./...` + `staticcheck ./...` 无新增 warning | CI | — |
| **AC10** | 文件规模 < 800 行；函数 < 50 行；ToolSpec 仍为不可变 value object | review | — |
| **AC11** | 不修改 D2/D3/D4/D5/D6 library 对外 API；只改 `internal/shared/contracts/` + 7 surface + BuildSurfaces + turn_adapter | `git diff` 0 行 library 改动 | — |

### 3.2 质量基线

| ID | 标准 |
|---|---|
| **AC12** | 覆盖率 ≥ 80% (新 bool 字段填充逻辑全测) |
| **AC13** | OpenSpec 归档后 `verify-archive.sh` 12/12 PASS |
| **AC14** | docs/methodology/dsaft-methodology.md §12 加"ToolSpec orthogonal flags" 案例 |
| **AC15** | DSAFT T 注册表：TOOL-SURFACE-1-T22~T25 4 个新 P0 T 点登记到 `openspec/specs/tool-surface/t-registry.md` |

### 3.3 Out of Scope（明确不做）

- **不修改** `IPermissionGate` 接口（per-tool checkPermission 是 devrix-surface-permission-extension 范围）
- **不引入** Zod-equivalent schema 验证（devrix-surface-lazy-loading 范围）
- **不实现** ToolSearch / SurfaceSearch / Lazy loading（devrix-surface-lazy-loading 范围）
- **不实现** MCP 集成（不在本轮 roadmap）
- **不重构** `ToolSpec.Risk` 字段（保留向后兼容，新 bool 字段是增量）
- **不引入** 新 LLM provider / 新通信协议 / 新持久化后端

---

## 4. 依赖与约束

| 类型 | 内容 |
|---|---|
| 上游（已合并） | DM-20260617-007（拆面契约定型）— 4 方法 interface 基线 |
| 上游（已合并） | DM-20260617-008（删 5 global）— ctor 注入模式基线 |
| 上游（已合并） | DM-20260617-003+005+006（D7 turn 装配 + SessionContext + perm gate）— turn_adapter.ExecuteRound 行为基线 |
| 上游（已合并） | DM-20260617-002（13 diagnostic tools wiring）— surface list 基线 |
| 下游（待启动） | DM-20260618-002（per-tool checkPermission）— 本 change 是前置依赖 |
| 下游（待启动） | DM-20260618-003（lazy loading + Zod schema）— 本 change 是前置依赖（ToolSpec 字段已扩，DM-003 可加 DeferLoading） |
| 约束 | 不修改 D2/D3/D4/D5/D6 library 对外 API |
| 约束 | 7 surface 全部 ≥ 1 个单测覆盖新 bool 字段填充 |
| 约束 | InterruptBehavior 默认 `"block"`，向后兼容既有 7 surface 行为 |
| 约束 | BuildSurfaces sort.Slice 必须**保持** name 稳定（D2 legacy path 不受影响） |
| 约束 | turn_adapter 并行 dispatch 必须**保持** ToolRoundResult 顺序与 ToolCalls 顺序一致 |

---

## 5. 风险评估

| 风险 | 等级 | 缓解 |
|---|---|---|
| **ToolSurface interface 加方法 = breaking change** | H | 7 surface 同时改；加 default 实现（`func (s *XxxSurface) InterruptBehavior(string) InterruptMode { return "block" }`）保持向后兼容；S3-Gate 走严格 review |
| **turn_adapter 并行 dispatch 顺序错乱** | H | 集成测试断言 ToolRoundResult.Results 顺序 == req.ToolCalls 顺序；用 errgroup 收集 + indexed slice 写回 |
| **4 bool 字段填充口径不一致（7 surface 各自解读）** | M | S3 design.md 给出"填充决策表"（bash: ReadOnly=false, Destructive=true, OpenWorld=true, ConcurrencySafe=true）；7 surface 改完后开 spec.md 集中 review |
| **BuildSurfaces sort 后 LLM 工具顺序变了** | L | 既有 T08（per-agent ⊇ main）保持 PASS；新增 T24 显式守护 |
| **InterruptBehavior 与 ctx cancel 冲突** | L | InterruptMode='cancel' 时 surface.Execute 必须 select ctx.Done()；测试 ctx cancel 后 200ms 内返回 |
| **D7 turn 与 D2 library 的 ctx cancel 协议不统一** | M | T23 覆盖：D7 turn 触发 cancel → D2 surface.Execute 收到 ctx.Done() → 内部清理资源 |

---

## 6. 关联参考

- 上游 change：`openspec/archive/2026-06-17-devrix-tool-surface-contract/` (DM-007)
- 上游 change：`openspec/archive/2026-06-17-devrix-tool-surface-phase2-full/` (DM-008)
- 借鉴源：`docs/reference/clawcode-tool-design-comparison.md` (DM-20260618-001 followup)
- clawcode 参考：`clawcode/src/Tool.ts:402-447` (isReadOnly / isDestructive / isOpenWorld / isConcurrencySafe / shouldDefer / interruptBehavior)
- DSAFT 方法论：`docs/methodology/dsaft-methodology.md` §12 (Facet Decomposition)
- 域归档：`openspec/specs/d2-context-engine/`, `openspec/specs/d7-orchestration/`
- T 注册表：`openspec/specs/tool-surface/t-registry.md`

---

## 7. 检查清单（S1 完成确认）

- [x] DM ID 已分配：`DM-20260618-001`
- [x] demand.md 包含背景、问题、目标、DSAFT 结构、验收标准
- [x] 11 个 P0 验收标准 (AC1-AC11) + 4 个质量基线 (AC12-AC15)
- [x] 3 个 A 节点 (TOOL-SURFACE-1-A01 / D7-S6-A01 / TOOL-SURFACE-1-A01-F02)
- [x] 8 个 F 节点
- [x] Out of Scope 已明确（§3.3）
- [x] DSAFT 域标注正确（横切 TOOL-SURFACE-1 + D2/D7）
- [x] 风险评估含影响与缓解（§5）
- [x] 上下游 change 显式登记（§4）
- [x] 借鉴源（clawcode-tool-design-comparison）显式标注
- [x] 不动 13 个 diagnostic-tools-parity library
- [x] 不动 DM-007/008 已归档的 22 AC
