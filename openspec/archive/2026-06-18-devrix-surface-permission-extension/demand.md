# S1 需求文档：Per-tool CheckPermission hook + IPermissionGate.ToolPolicy

**DM ID:** DM-20260618-002
**Change ID:** devrix-surface-permission-extension
**状态:** Draft
**创建日期:** 2026-06-18
**父 change:** devrix-tool-spec-enrichment (DM-20260618-001, S4_Ready)
**需求源:** clawcode-tool-design-comparison-2026-06-17 (P0 借鉴清单)

---

## 1. 需求概述

| 字段 | 内容 |
|------|------|
| **功能名称** | Per-tool CheckPermission hook + IPermissionGate.ToolPolicy |
| **所属域** | 横切契约域 TOOL-SURFACE-1 + 横切 D7-ORCHESTRATION + 支撑 BASH-AST-1 |
| **优先级** | **P0** |
| **预计工时** | 2-3 人日 |
| **下游 change** | devrix-policy-dsl-yaml (DM-005, 自定义 policy 配置) |

### 1.1 问题背景

devrix 当前权限决策分两层：

1. **D7 turn 装配层** — `IPermissionGate.Request(ctx, decision)` 在 turn 开始时同步阻塞决策（DM-006 引入）
2. **D2 surface.Execute** 层 — `IPermissionGate` 在执行时检查（既有）

**实测问题**：

#### 1.1.1 没有 per-tool 决策点

`IPermissionGate.Request` 一次性决定整个 turn 的所有 tool 风险等级；**没有"在单个 tool dispatch 前"再决策一次**的能力。

实测：bash tool 在 plan_mode 下应被严格 policy 拒（如 `rm -rf`），但当前 IPermissionGate 只看 turn-level decision，**plan_mode 不能针对 bash 内容细化**。

clawcode 显式有 `Tool.checkPermissions()` 方法（Tool.ts:404-410），在 `agentLoop` 每次 tool dispatch 前调一次。

#### 1.1.2 Bash 没有 AST-level policy

`BuiltinSurface.bash.Execute` 当前仅校验命令是否在 deny-list 字符串里（粗糙的 substring 匹配）。**无法识别 `rm -rf /` 这种被变量名/路径转义绕过的情况**。

clawcode 用 `mvdan/sh` Go 库做 bash AST 解析（bashParse tool.ts），可对 AST 节点做精确 policy。

#### 1.1.3 Plan mode 无法用 OpenWorld 字段收紧

DM-001（ToolSpec orthogonal flags）已经让 ToolSpec 有 `OpenWorld` 字段，但**当前没有任何 policy 消费这个字段**。plan_mode 仍按 Risk 阈值粗筛。

clawcode 的 `shouldAvoidPermissionPrompts`（hooks/tools.ts）显式用 `isOpenWorld()` 在 plan mode 拒绝网络/副作用 tool。

### 1.2 目标

在 1 个 change 内、零侵入、不改 library API 的前提下：

1. **ToolSurface interface +1 method `CheckPermission(ctx, spec, input) Decision`** —— 每个 tool 在 dispatch 前自己决策
2. **Decision enum 三态** —— `Allow` / `Deny` / `Ask`（与 clawcode 一致；既有 IPermissionGate 二态升级）
3. **BashSurface 内置 AST 解析** —— 用 `mvdan/sh` Go 库（已选型，devrix go.mod 评估后决定是否引入）解析 cmd，对 AST 节点做 deny-list
4. **IPermissionGate 加 `CheckPermission(ctx, spec) Decision` 方法** —— 复用既有的外部决策机制
5. **Plan mode 自动 deny OpenWorld=true 的 tool** —— 通过 Policy 注册器在 layer 3 收编
6. **turn_adapter ExecuteRound 在 dispatch 前调 CheckPermission** —— hook 在 surface.Execute 之前
7. **7 surface 默认实现 CheckPermission=Allow** —— 既有行为不变，向后兼容 DM-001

### 1.3 与 DM-001 关系

| 维度 | DM-001 (ToolSpec Enrichment) | DM-002 (Permission Extension) |
|---|---|---|
| 关注点 | 决策的**输入**（4 bool flags） | 决策的**执行**（CheckPermission method） |
| 改动位置 | contracts.ToolSpec +7 surface.Tools() | contracts.ToolSurface interface +7 surface + IPermissionGate |
| Breaking 程度 | 1 method breaking (InterruptBehavior) | 1 method breaking (CheckPermission) |
| 依赖关系 | — | **强依赖 DM-001**（消费 OpenWorld 字段） |

---

## 2. DSAFT 结构

### 2.1 D 领域

| 域 | 类型 | 影响 | 理由 |
|---|---|---|---|
| **TOOL-SURFACE-1** | 横切契约 | **新增方法** | ToolSurface 6 方法（+1 CheckPermission） |
| **PERMISSION-GATE-1** | 横切权限 | **扩展方法** | IPermissionGate + CheckPermission |
| **D2-ContextEngine** | 核心 | 修改 | turn_adapter dispatch 前调 CheckPermission |
| **D7-Orchestration** | 核心 | 修改 | Plan mode policy 注册 OpenWorld deny |
| **BASH-AST-1** | 支撑 | 新增 | BashAST 解析器 + deny-list 配置 |
| **D2/D7 library** | 核心 | **零修改** | 严守 AC |

### 2.2 S 场景

| 场景 ID | 名称 | 触发条件 | 用户目标 | 涉及 A |
|---|---|---|---|---|
| **TOOL-SURFACE-1-S4** | Per-Tool Permission Hook | LLM 调 tool 前的精细决策 | 用工具自己的逻辑替代 turn-level 粗筛 | A02（CheckPermission） |
| **PERMISSION-GATE-1-S1** | Open World Plan Mode | plan_mode + OpenWorld tool | 自动 deny 不需用户确认 | A03（Policy 注册） |
| **BASH-AST-1-S1** | Bash Dangerous Cmd Detection | bash tool 收到 `rm -rf /` | AST 拒危险命令 | A04（AST 解析） |
| **D7-S7** | Policy Decision Path | turn 启动 + plan_mode | 决策 < 5ms 返回（in-process） | A05（permission overhead 优化） |

### 2.3 A 活动

| 活动 ID | 名称 | 类型 | 输入 | 输出 | 状态变更 |
|---|---|---|---|---|---|
| **TOOL-SURFACE-1-A02** | CheckPermission Method | A-BE | 5 method v2 | 6 method v3（+CheckPermission） | interface breaking change |
| **PERMISSION-GATE-1-A01** | IPermissionGate.ToolPolicy | A-BE | IPermissionGate 1 method | IPermissionGate 2 method（+CheckPermission） | interface breaking change |
| **BASH-AST-1-A01** | BashASTPolicy | A-BE | bash cmd string | Allow / Deny / Ask | 新增解析器 |
| **D7-S7-A01** | PlanMode OpenWorld Deny | A-BE | ToolSpec.OpenWorld | Decision.Deny | policy 注册 |

### 2.4 F 功能点

| F ID | 名称 | 所属 A | 输入 | 输出 |
|---|---|---|---|---|
| TOOL-SURFACE-1-A02-F01 | Decision enum (Allow/Deny/Ask) | A02 | — | Decision string |
| TOOL-SURFACE-1-A02-F02 | 7 surface 默认 CheckPermission=Allow | A02 | spec, input | Allow |
| PERMISSION-GATE-1-A01-F01 | IPermissionGate.CheckPermission(ctx, spec) | A01 | spec | Decision |
| PERMISSION-GATE-1-A01-F02 | Plan mode policy 注册 OpenWorld deny | A01 | spec | Decision.Deny |
| BASH-AST-1-A01-F01 | bash AST 解析（mvdan/sh） | A01 | cmd string | AST |
| BASH-AST-1-A01-F02 | deny-list 配置（rm -rf /, dd, mkfs） | A01 | AST | Decision.Deny |
| D7-S7-A01-F01 | turn_adapter dispatch 前 CheckPermission 调用 | A01 | tc | Decision |

---

## 3. 验收标准

### 3.1 P0（必须达成，否则不交付）

| ID | 标准 | 度量 | 关联 T 点 |
|---|---|---|---|
| **AC1** | `ToolSurface` interface +1 method `CheckPermission(ctx, spec, input) Decision` | compile-time `var _ contracts.ToolSurface = ...` 7 个 surface 全部 PASS | T26 |
| **AC2** | `Decision` enum 三态：`Allow` / `Deny` / `Ask` | `git grep "Decision = " internal/shared/contracts/` 命中 3 个常量 | T26 |
| **AC3** | 7 surface 默认实现 CheckPermission=Allow（向后兼容 DM-001） | 7 个 `TestXxxSurface_CheckPermission_DefaultAllow` 子测试 PASS | T26 |
| **AC4** | `BashSurface.CheckPermission` 内置 AST 解析，拒 `rm -rf /`、`dd if=`、`mkfs` 等危险命令 | T27 集成测试：10 个危险命令 100% 拒 | T27 |
| **AC5** | `IPermissionGate` interface +1 method `CheckPermission(ctx, spec) Decision` | compile-time assertion `var _ IPermissionGate = ...` PASS（既有 stub 1 个） | T28 |
| **AC6** | Plan mode（mode=plan）+ ToolSpec.OpenWorld=true → Decision.Deny | T29 集成测试：plan_mode + free_fork → Deny，自动路径 | T29 |
| **AC7** | turn_adapter ExecuteRound 在 surface.Execute 前调 CheckPermission，Deny 直接返回不 Execute | T29 集成测试：mock Deny → surface.Execute 调用计数 = 0 | T29 |
| **AC8** | Ask 决策 = 抛 `PermissionAskRequiredError` 错误（含 spec + input 元信息） | 单测：mock Ask → 错误含 spec.Name | T29 |
| **AC9** | 既有 15 个 P0 T 点（DM-007/008 留下的 11 + DM-001 加的 4）全部 PASS | `go test -race ./...` 100% 绿 | T01-T11, T22-T25 |
| **AC10** | `go vet ./...` + `staticcheck ./...` 无新增 warning | CI | — |
| **AC11** | 不修改 D2/D3/D4/D5/D6 library 对外 API；只改 `internal/shared/contracts/` + 7 surface + IPermissionGate + turn_adapter + BashAST（新增包） | `git diff` 0 行 library 改动 | — |
| **AC12** | 单测覆盖率（新代码）≥ 80% | `go test -cover` | — |
| **AC13** | CheckPermission p99 overhead < 5ms（in-process，不 RPC） | benchmark | T26 |

### 3.2 质量基线

| ID | 标准 |
|---|---|
| **AC14** | Bash AST 解析库选型说明文档：`docs/reference/bash-ast-library-selection.md`（mvdan/sh vs tree-sitter-bash vs 自实现） |
| **AC15** | deny-list 默认值文档化：`docs/reference/bash-default-denylist.md`（10+ 条：rm -rf /, dd, mkfs, chmod 777 /, sudo, ...） |
| **AC16** | OpenSpec 归档后 `verify-archive.sh` 12/12 PASS |
| **AC17** | DSAFT T 注册表：TOOL-SURFACE-1-T26~T29 + PERMISSION-GATE-1-T01~T02 6 个新 P0 T 点登记到 `openspec/specs/tool-surface/t-registry.md` 和 `openspec/specs/permission-gate/t-registry.md` |
| **AC18** | docs/methodology/dsaft-methodology.md §12 加"per-tool CheckPermission" 案例 |

### 3.3 Out of Scope（明确不做）

- **不实现** Per-tool 自定义 policy DSL（YAML/JSON 配置）—— DM-005 范围
- **不实现** Permission audit log —— DM-005 范围
- **不引入** Zod-equivalent schema 验证（DM-003 范围）
- **不实现** ToolSearch / SurfaceSearch / Lazy loading（DM-003 范围）
- **不实现** MCP 集成（不在本轮 roadmap）
- **不重构** 既有的 IPermissionGate.Request 方法（保留向后兼容；新 CheckPermission 是增量）
- **不修改** ToolSpec 4 bool 字段（DM-001 已是 v2；本 change 不再扩字段）
- **不引入** 新 LLM provider / 新通信协议 / 新持久化后端

---

## 4. 依赖与约束

| 类型 | 内容 |
|---|---|
| 上游（已合并） | DM-007 (ToolSurface 4 method 基线) |
| 上游（已合并） | DM-008 (0 global 基线) |
| 上游（DM-001, S4_Ready） | **强依赖**：ToolSpec 4 bool 字段 + InterruptBehavior 5 method 已是基础 |
| 上游（已合并） | DM-006 (IPermissionGate.Request turn-level 决策) |
| 下游（待启动） | DM-005 (per-tool policy DSL + audit log) |
| 约束 | 不修改 D2/D3/D4/D5/D6 library 对外 API |
| 约束 | 7 surface 全部 ≥ 1 个单测覆盖新 CheckPermission |
| 约束 | CheckPermission 默认 `Allow`（既有 7 surface 行为零变化） |
| 约束 | Plan mode 自动 deny OpenWorld=true 的 tool **必须可被用户配置覆盖**（devrix.yaml 加 `plan_mode.open_world_allowlist: [...]`） |
| 约束 | Bash AST 解析 overhead < 5ms p99 |
| 约束 | turn_adapter CheckPermission 调用 overhead 不应使 T25 并行加速失效 |

---

## 5. 风险评估

| 风险 | 等级 | 缓解 |
|---|---|---|
| **ToolSurface interface +1 method = breaking（继 DM-001 之后第 2 次 breaking）** | H | 7 surface 集中 commit；compile-time `var _ contracts.ToolSurface = ...` 7 处 assertion 必须 PASS；DM-001 已是 v2，本 change 是 v3 |
| **Plan mode 自动 deny OpenWorld tool 影响 LLM 工作流** | M | devrix.yaml `plan_mode.open_world_allowlist: ["web_fetch", "git_*"]` 可覆盖；T29 集成测试既覆盖 deny 也覆盖 allowlist 命中 |
| **Bash AST 解析 overhead > 5ms** | M | 选 mvdan/sh（Go 库，无 cgo）；benchmark 在 S4 阶段必须出数；不行则降级为 string-based 启发式（devrix 0.1 时期的方案） |
| **IPermissionGate 既有 stub 改造不彻底** | M | compile-time `var _ IPermissionGate = ...` 守护；既有 tests/integration/permission_gate_test.go 调通 2 method 路径 |
| **Bash AST 解析库选型错误** | L | 选 mvdan/sh：纯 Go、5MB binary、bash 语法 100% 覆盖；备选 tree-sitter-bash（cgo 依赖，binary +15MB） |
| **per-tool CheckPermission 引入新 race** | M | `go test -race ./...` 必须 100% 绿；Decision 不携带可变状态 |
| **deny-list 漏判（绕过手段）** | M | 0 阶段 deny-list 仅覆盖"教科书"危险命令（rm -rf /, dd, mkfs）；用户可在 devrix.yaml 扩；DM-005 引入 DSL 后可加正则 |

---

## 6. 关联参考

- 上游 change：`openspec/archive/2026-06-17-devrix-tool-surface-contract/` (DM-007)
- 上游 change：`openspec/archive/2026-06-17-devrix-tool-surface-phase2-full/` (DM-008)
- 上游 change：`openspec/changes/devrix-tool-spec-enrichment/` (DM-001, S4_Ready)
- 借鉴源：`docs/reference/clawcode-tool-design-comparison.md` §8.2 P0-(2)
- clawcode 参考实现：
  - `clawcode/src/Tool.ts:404-410` (`checkPermissions(aiInput, toolUse, context, parentMemo)`)
  - `clawcode/src/Tool.ts:101-110` (`PermissionResult` enum: allow/deny/ask)
  - `clawcode/src/Tool.ts:201-260` (`buildTool` factory + checkPermissions 调用)
  - `clawcode/src/hooks/tools.ts:checkPermissionRequest` (policy decision path)
  - `clawcode/src/tools/BashTool/bashParse.ts` (mvdan/sh integration)
  - `clawcode/src/utils/permissions/permissionDefaults.ts` (default deny-list)
- DSAFT 方法论：`docs/methodology/dsaft-methodology.md` §12 (Facet Decomposition)
- 域归档：`openspec/specs/d2-context-engine/`, `openspec/specs/d7-orchestration/`
- T 注册表：`openspec/specs/tool-surface/t-registry.md`, `openspec/specs/permission-gate/t-registry.md`

---

## 7. 检查清单（S1 完成确认）

- [x] DM ID 已分配：`DM-20260618-002`
- [x] demand.md 包含背景、问题、目标、DSAFT 结构、验收标准
- [x] 13 个 P0 验收标准 (AC1-AC13) + 5 个质量基线 (AC14-AC18)
- [x] 4 个 A 节点 (TOOL-SURFACE-1-A02 / PERMISSION-GATE-1-A01 / BASH-AST-1-A01 / D7-S7-A01)
- [x] 7 个 F 节点
- [x] Out of Scope 已明确（§3.3）
- [x] DSAFT 域标注正确（横切 TOOL-SURFACE-1 + PERMISSION-GATE-1 + D2/D7）
- [x] 风险评估含影响与缓解（§5）
- [x] 上下游 change 显式登记（§4）
- [x] 借鉴源（clawcode-tool-design-comparison）显式标注
- [x] 不动 13 个 diagnostic-tools-parity library
- [x] 不动 DM-007/008/001 已归档/已设计的 AC
