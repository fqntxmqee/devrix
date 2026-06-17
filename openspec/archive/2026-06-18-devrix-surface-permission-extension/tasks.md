# Tasks: Per-tool CheckPermission hook + IPermissionGate.ToolPolicy

**Change ID:** devrix-surface-permission-extension
**DM ID:** DM-20260618-002
**状态:** S3_Designed → S4_Ready
**估时:** ~12.75h (2 人日)
**PR 拆分:** 3 个 PR (commit 12 个)

---

## Phase 1 — 契约扩展

| ID | 任务 | L4 | L5 | 估行 | 依赖 |
|----|------|-----|-----|------|------|
| T1.1 | `Decision` enum + `PermissionDeniedError` / `PermissionAskRequiredError` 定义 | L4-BE-CTX-CONTRACTS | {T}-TS-26 | ~50 | — |
| T1.2 | `ToolSurface` interface +1 method `CheckPermission` | L4-BE-CTX-CONTRACTS | {T}-TS-26 | ~5 | T1.1 |
| T1.3 | `IPermissionGate` interface +1 method `CheckPermission` | L4-BE-ORCH-PERM | {T}-TS-28 | ~5 | T1.1 |
| T1.4 | compile-time assertion `var _ contracts.ToolSurface = ...` 7 处暂时 fail | — | — | — | T1.2 |

**交付物**：`internal/shared/contracts/permission.go`（新）+ `tool_surface.go` v3

## Phase 2 — 7 surface 各自实现 CheckPermission

| ID | 任务 | L4 | L5 | 估行 | 依赖 |
|----|------|-----|-----|------|------|
| T2.1 | 5 surface (LSP/Tracker/Verify/Delegate/Background) 默认 Allow | L4-BE-CTX-SURFACE | {T}-TS-26 | ~20 | T1.2 |
| T2.2 | `BuiltinSurface.CheckPermission`：read/write/edit/grep/glob Allow + bash 调 BashASTPolicy | L4-BE-CTX-SURFACE | {T}-TS-26, {T}-TS-27 | ~30 | T1.2, T3.1 |
| T2.3 | `FreeForkSurface.CheckPermission`：调 IPermissionGate | L4-BE-CTX-SURFACE | {T}-TS-26 | ~15 | T1.2, T1.3 |
| T2.4 | 7 surface compile-time assertion 全部 PASS | — | {T}-TS-26 | — | T2.1-T2.3 |

**交付物**：7 surface.go 全部 v3

## Phase 3 — BashASTPolicy 解析器

| ID | 任务 | L4 | L5 | 估行 | 依赖 |
|----|------|-----|-----|------|------|
| T3.1 | go.mod 加 `mvdan.cc/sh/v3 v3.5.0` + 跑通 build | L4-BE-CTX-SURFACE | AC11 | ~5 | — |
| T3.2 | `BashASTPolicy` 类型 + `Check(cmd) (Decision, string)` | L4-BE-CTX-SURFACE | {T}-TS-27 | ~30 | T3.1 |
| T3.3 | 5 个默认 deny-list 规则（rm-rf-root, dd, mkfs, sudo, chmod-777-root） | L4-BE-CTX-SURFACE | {T}-TS-27 | ~80 | T3.2 |
| T3.4 | 5 个 deny rule 的 `Match(*syntax.Stmt) bool` 函数 | L4-BE-CTX-SURFACE | {T}-TS-27 | ~60 | T3.3 |
| T3.5 | 单测：10 个危险命令 deny + 5 个安全命令 allow + 2 个 parse error ask | L4-BE-CTX-SURFACE | {T}-TS-27 | ~80 | T3.4 |
| T3.6 | benchmark `BenchmarkBashASTPolicy_Check` 验证 < 5ms p99 | L4-BE-CTX-SURFACE | AC13 | ~30 | T3.5 |

**交付物**：`internal/layers/contextengine/enforce/toolrunner/surface/bash_ast.go`（新）

## Phase 4 — IPermissionGate + PlanMode policy

| ID | 任务 | L4 | L5 | 估行 | 依赖 |
|----|------|-----|-----|------|------|
| T4.1 | `PermissionGateAdapter.CheckPermission` 实现（Risk + PlanMode 链） | L4-BE-ORCH-PERM | {T}-TS-28 | ~40 | T1.3 |
| T4.2 | `PlanModeOpenWorldPolicy` 类型 + `Apply(ctx, spec, current) Decision` | L4-BE-ORCH-POLICY | {T}-TS-29 | ~50 | T1.3 |
| T4.3 | devrix.yaml 解析 `plan_mode.open_world_allowlist` + wildcard 匹配 | L4-BE-ORCH-POLICY | AC6 | ~30 | T4.2 |
| T4.4 | 单测：4 个 Risk 等级 + 2 个 Plan mode 场景 | L4-BE-ORCH-PERM | {T}-TS-28, {T}-PERM-01 | ~60 | T4.1-T4.3 |
| T4.5 | compile-time `var _ IPermissionGate = (*PermissionGateAdapter)(nil)` PASS | L4-BE-ORCH-PERM | {T}-TS-28 | — | T4.4 |

**交付物**：`internal/layers/orchestration/permission/gate.go` v2 + `policy/plan_mode.go`（新）

## Phase 5 — turn_adapter 集成

| ID | 任务 | L4 | L5 | 估行 | 依赖 |
|----|------|-----|-----|------|------|
| T5.1 | `executeOne` 拆 helper：pre-check (CheckPermission) + actual execute | L4-BE-ORCH-TURN | {T}-TS-29 | ~30 | T2.4, T4.5 |
| T5.2 | `ExecuteRound` 重构：Step 1 sequential permission 决策 + Step 2 并行 dispatch | L4-BE-ORCH-TURN | {T}-TS-29 | ~50 | T5.1 |
| T5.3 | PermissionDeniedError / PermissionAskRequiredError 结果填充到 `result.Results[i].Error` | L4-BE-ORCH-TURN | {T}-TS-29 | ~15 | T5.2 |
| T5.4 | 集成测试：Deny 时 surface.Execute 调用计数 = 0 | L4-BE-ORCH-TURN | {T}-TS-29 | ~40 | T5.3 |
| T5.5 | 集成测试：plan_mode + free_fork → Deny | L4-BE-ORCH-TURN | {T}-TS-29 | ~40 | T5.3 |
| T5.6 | 集成测试：allowlist 命中 → 跳过 Deny 进入 Risk 决策 | L4-BE-ORCH-TURN | {T}-PERM-02 | ~30 | T5.3 |
| T5.7 | 集成测试：3 个 Allow read_file 并行（与 T25 不冲突） | L4-BE-ORCH-TURN | {T}-TS-29 | ~30 | T5.3 |
| T5.8 | `go test -race ./...` 必须 100% 绿 | L4-BE-ORCH-TURN | AC10 | — | T5.7 |

**交付物**：`internal/bootstrap/turn_adapter.go` v3

## Phase 6 — 既有 15 T 点回归

| ID | 任务 | L4 | L5 | 估行 | 依赖 |
|----|------|-----|-----|------|------|
| T6.1 | T01-T11 (DM-007/008) 既有 11 P0 T 点重跑 | — | {T}-TS-01~11 | — | T5.8 |
| T6.2 | T22-T25 (DM-001) 既有 4 P0 T 点重跑 | — | {T}-TS-22~25 | — | T5.8 |
| T6.3 | T08 per-agent ⊇ main 测试重跑（policy chain 不影响 filter chain） | L4-BE-ORCH-POLICY | {T}-TS-08 | — | T5.8 |
| T6.4 | library 0 行改动核对（git diff 过滤 freefork/tracker/verify/multiagent） | — | AC11 | — | T5.8 |
| T6.5 | 全量 `go test -race ./...` + `go vet ./...` + `staticcheck` 0 warning | — | AC10 | — | T6.1-T6.4 |

**交付物**：`go test -race ./...` 100% 绿 + library diff = 0

## Phase 7 — T 注册表 + 文档同步

| ID | 任务 | L4 | L5 | 估行 | 依赖 |
|----|------|-----|-----|------|------|
| T7.1 | `openspec/specs/tool-surface/t-registry.md` 加 T26-T29 4 个新 P0 T 点 | L5-DOCS | {T}-TS-26~29 | ~50 | T5.8 |
| T7.2 | `openspec/specs/permission-gate/t-registry.md` 新建 + 加 T01-T02 | L5-DOCS | {T}-PERM-01~02 | ~50 | T5.8 |
| T7.3 | `docs/reference/bash-default-denylist.md` 新建 | L5-DOCS | AC15 | ~40 | T3.5 |
| T7.4 | `docs/reference/bash-ast-library-selection.md` 新建（mvdan/sh vs tree-sitter vs 自实现） | L5-DOCS | AC14 | ~50 | T3.1 |
| T7.5 | `openspec/specs/tool-surface/spec.md` 主 spec 增量 REQ-TS-12~16 | L5-DOCS | REQ-TS-12~16 | ~80 | T5.8 |
| T7.6 | `docs/methodology/dsaft-methodology.md` §12 加"per-tool CheckPermission" 案例 | L5-DOCS | AC18 | ~40 | T7.1 |

**交付物**：4 个新文档 + 1 个 spec 增量 + 2 个 t-registry 更新

## Phase 8 — S5 验收 + S6 归档

| ID | 任务 | L4 | L5 | 估行 | 依赖 |
|----|------|-----|-----|------|------|
| T8.1 | `verify-archive.sh` 12/12 PASS | — | AC16 | — | T7.6 |
| T8.2 | PR-1 创建（契约+permission.go+BashAST+7 surface） + auto-merge | — | — | — | T8.1 |
| T8.3 | PR-2 创建（IPermissionGate + PlanMode policy + turn_adapter 集成） + auto-merge | — | — | — | T8.2 |
| T8.4 | PR-3 创建（T26-T29 测试 + 文档） + auto-merge | — | — | — | T8.3 |
| T8.5 | 归档到 `openspec/archive/2026-06-18-devrix-surface-permission-extension/` | — | S6 | — | T8.4 |

**交付物**：3 个 PR auto-merged + 归档目录

---

## 依赖顺序

```
T1.1 → T1.2 → T1.3 → T1.4 (compile fail 预期)
   ↓         ↓
  T3.1-T3.6 (BashAST, 可与 T2.x 并行)
   ↓         ↓
T2.1-T2.4   T4.1-T4.5 (Permission Gate)
   ↓         ↓
   └────┬────┘
        ↓
   T5.1-T5.8 (turn_adapter 集成)
        ↓
   T6.1-T6.5 (既有 15 T 点回归)
        ↓
   T7.1-T7.6 (文档)
        ↓
   T8.1-T8.5 (PR + 归档)
```

---

## 建议 PR 拆分

### PR-1: contracts + BashAST + 7 surface（最大改动 + 1 个 breaking interface 集中 commit）

**包含 T1.1-T1.4, T2.1-T2.4, T3.1-T3.6**
- `internal/shared/contracts/permission.go`（新）
- `internal/shared/contracts/tool_surface.go` v3
- `internal/layers/contextengine/enforce/toolrunner/surface/bash_ast.go`（新 ~200 行）
- 7 surface.go 全部 v3
- 7 surface_test.go 各加 1 个子测试
- `tests/integration/tool_surface_test.go` T26+T27
- go.mod + go.sum（mvdan/sh）

**Review 重点**：
- 5 个 deny-list 规则（mvdan/sh AST 匹配）
- 7 surface 5 默认 + 1 Bash override + 1 FreeFork override
- compile-time `var _ contracts.ToolSurface = ...` 7 处 PASS
- benchmark < 5ms p99

**预估 review 时间**：45-60 min（5 段式）

### PR-2: IPermissionGate + PlanMode policy + turn_adapter 集成

**包含 T4.1-T4.5, T5.1-T5.8**
- `internal/layers/orchestration/permission/gate.go` v2
- `internal/layers/orchestration/policy/plan_mode.go`（新）
- `internal/bootstrap/turn_adapter.go` v3
- `tests/integration/permission_test.go` T28+T29
- `tests/integration/turn_adapter_test.go` T29 集成部分

**Review 重点**：
- PlanMode policy chain + allowlist wildcard 匹配
- Step 1 sequential 决策 → Step 2 并行 dispatch 的 race 保证
- Deny 时 surface.Execute 调用计数 = 0

**预估 review 时间**：30-45 min

### PR-3: 文档同步 + 既有 15 T 点回归

**包含 T6.1-T6.5, T7.1-T7.6, T8.1**
- 2 个 t-registry 更新
- 2 个 docs/reference 新建
- 1 个 spec.md 增量
- 1 个 dsaft-methodology 案例
- T08 既有测试重跑

**Review 重点**：
- 文档与代码 1:1 对齐
- 既有 15 T 点全部 PASS
- library 0 改动

**预估 review 时间**：15-20 min

---

## 风险与回滚

| 风险 | 触发条件 | 回滚策略 |
|---|---|---|
| 7 surface 改 CheckPermission 不一致 | PR-1 review 发现 | PR-1 修该 surface；不 merge PR-2 |
| Bash AST 解析 > 5ms p99 | T3.6 benchmark fail | 降级 substring 匹配；T27 改宽松断言 |
| Plan mode 误 deny | 集成测试 fail | yaml allowlist 调整；devrix.yaml 默认加 `web_fetch, git_*` |
| mvdan/sh 引入 cgo 风险 | `CGO_ENABLED=0` 失败 | 换 tree-sitter-bash 或自实现 |
| IPermissionGate stub 没实现 | compile fail | 修 stub；compile-time assertion 守护 |
| turn_adapter CheckPermission 引入 race | `go test -race` fail | 改 sequential decision；T29 重跑 |
| library 0 改动被破坏 | T6.4 git diff 非 0 | 该 PR reject；定位误改 library 的 commit 并 revert |

每个 PR 独立可回滚（git revert）。

---

## 与下游 change 的接口

### 留给 DM-005 (policy DSL) 的 hook 点

本 change 之后，DM-005 可以：
- 替换 `DefaultBashDenyRules` 硬编码为 `devrix.yaml.bash_policy.deny_list` 加载
- 替换 `PlanMode.OpenWorldAllowList` 为 `devrix.yaml.plan_mode.allowlist` 更细粒度（per-agent-type）
- 加 `PermissionAuditLog`：所有 Deny/Ask 决策写入 `~/.devrix/audit/permission-YYYY-MM-DD.log`
- 加 `InteractivePrompt`：Ask 决策可发 IM 消息给用户确认（v1 简化是错误返回）

### 留给 DM-003 (lazy loading) 的 hook 点

本 change 之后，DM-003 可以：
- `ToolSurface` interface 已有 6 method，DM-003 加 `ShouldDefer() bool` 是 7th method
- 探索 agent filter 预分类时，可消费 `Decision=Allow` 的 spec 优先加载
- 拒决策（`Decision=Deny`）的 tool 不进入 schema 列表（节省 LLM tokens）

---

## 工作量汇总

| Phase | 估时 | 任务数 |
|---|---|---|
| Phase 1 — 契约扩展 | 1h | 4 |
| Phase 2 — 7 surface 改 | 1.5h | 4 |
| Phase 3 — BashASTPolicy | 3h | 6 |
| Phase 4 — IPermissionGate + PlanMode | 2h | 5 |
| Phase 5 — turn_adapter 集成 | 2.5h | 8 |
| Phase 6 — 既有 15 T 点回归 | 0.5h | 5 |
| Phase 7 — 文档同步 | 1h | 6 |
| Phase 8 — S5+S6 | 1h | 5 |
| **总计** | **~12.5h (2 人日)** | **43** |

---

## 检查清单（S4 完成确认）

- [x] 8 Phase 任务拆分（机械到 0.5h 级别）
- [x] 每个任务标注 L4 / L5 / 估行 / 依赖
- [x] 依赖图清晰（1.x → 2.x || 3.x → 4.x → 5.x → 6.x → 7.x → 8.x）
- [x] 3 个 PR 拆分（按"风险集中度"切：1=breaking 集中 + Bash / 2=policy + 集成 / 3=文档+回归）
- [x] 既有 15 T 点回归任务显式列出（T6.1-T6.5）
- [x] 风险与回滚策略（7 项）
- [x] 与下游 DM-005/DM-003 的接口预留
- [x] 工作量汇总（12.5h / 43 tasks）
- [x] S5 验收条件（verify-archive.sh 12/12 PASS）
- [x] S6 归档路径（`openspec/archive/2026-06-18-devrix-surface-permission-extension/`）
- [x] Bash AST 库选型说明任务（T7.4）
- [x] deny-list 默认值文档化任务（T7.3）
