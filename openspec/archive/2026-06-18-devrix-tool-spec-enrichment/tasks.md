# Tasks: ToolSpec orthogonal flags + InterruptBehavior + BuildSurfaces sort

**Change ID:** devrix-tool-spec-enrichment
**DM ID:** DM-20260618-001
**状态:** S3_Designed → S4_Ready
**估时:** ~10h (1.5 人日)
**PR 拆分:** 3 个 PR (commit 8 个)

---

## Phase 1 — 契约扩展（核心）

| ID | 任务 | L4 | L5 | 估行 | 依赖 |
|----|------|-----|-----|------|------|
| T1.1 | `ToolSpec` 加 4 bool 字段 + 文档注释（默认值全 false） | L4-BE-CTX-CONTRACTS | {T}-TS-22 | ~30 | — |
| T1.2 | `InterruptMode` enum 定义（`cancel` / `block`） | L4-BE-CTX-CONTRACTS | {T}-TS-23 | ~10 | — |
| T1.3 | `ToolSurface` interface +1 method `InterruptBehavior` | L4-BE-CTX-CONTRACTS | {T}-TS-23 | ~5 | T1.1, T1.2 |
| T1.4 | contracts 包 compile-time 通过（7 surface 暂时全部 fail） | L4-BE-CTX-CONTRACTS | {T}-TS-22 | — | T1.3 |

**交付物**：`internal/shared/contracts/tool_surface.go` v2

## Phase 2 — 7 surface 改 Tools() + InterruptBehavior

| ID | 任务 | L4 | L5 | 估行 | 依赖 |
|----|------|-----|-----|------|------|
| T2.1 | `BuiltinSurface.Tools()` 6 tool 全部填 4 bool + `InterruptBehavior="block"` | L4-BE-CTX-SURFACE | {T}-TS-22, {T}-TS-23 | ~50 | T1.3 |
| T2.2 | `LSPToolSurface.Tools()` + `InterruptBehavior="block"` | L4-BE-CTX-SURFACE | {T}-TS-22, {T}-TS-23 | ~15 | T1.3 |
| T2.3 | `FreeForkSurface.Tools()` + `InterruptBehavior="cancel"` + Execute 内部 select ctx.Done() | L4-BE-CTX-SURFACE | {T}-TS-22, {T}-TS-23 | ~30 | T1.3 |
| T2.4 | `TrackerSurface.Tools()` + `InterruptBehavior="block"` | L4-BE-CTX-SURFACE | {T}-TS-22, {T}-TS-23 | ~15 | T1.3 |
| T2.5 | `VerifySurface.Tools()` + `InterruptBehavior="block"` | L4-BE-CTX-SURFACE | {T}-TS-22, {T}-TS-23 | ~15 | T1.3 |
| T2.6 | `DelegateSurface.Tools()` + `InterruptBehavior="block"` | L4-BE-CTX-SURFACE | {T}-TS-22, {T}-TS-23 | ~20 | T1.3 |
| T2.7 | `BackgroundTaskSurface.Tools()` + `InterruptBehavior="block"` | L4-BE-CTX-SURFACE | {T}-TS-22, {T}-TS-23 | ~15 | T1.3 |
| T2.8 | 7 surface compile-time `var _ contracts.ToolSurface = ...` assertion 全部 PASS | L4-BE-CTX-SURFACE | {T}-TS-22, {T}-TS-23 | — | T2.1-T2.7 |

**交付物**：7 surface.go 全部 v2

## Phase 3 — BuildSurfaces sort + 单测

| ID | 任务 | L4 | L5 | 估行 | 依赖 |
|----|------|-----|-----|------|------|
| T3.1 | `BuildSurfaces` 末尾加 `sort.Slice(out, ...)` + import "sort" | L4-BE-CTX-BOOTSTRAP | {T}-TS-24 | ~3 | T2.8 |
| T3.2 | 集成测试：3 套不同 opts 输入，Names() 字符串完全相同 | L4-BE-CTX-BOOTSTRAP | {T}-TS-24 | ~50 | T3.1 |
| T3.3 | 集成测试：sort 顺序 = builtin < freefork < lsp < tracker < verify | L4-BE-CTX-BOOTSTRAP | {T}-TS-24 | ~20 | T3.1 |

**交付物**：`internal/bootstrap/surfaces.go` v2 + `surfaces_test.go` 新增

## Phase 4 — turn_adapter 并行 dispatch

| ID | 任务 | L4 | L5 | 估行 | 依赖 |
|----|------|-----|-----|------|------|
| T4.1 | `isConcurrencySafe(ctx, name)` 辅助方法（surface.Tools 查表） | L4-BE-ORCH-TURN | {T}-TS-25 | ~15 | T2.8 |
| T4.2 | `executeOne(ctx, tc)` 提取单 tool 执行逻辑（找 surface + InterruptBehavior timeout） | L4-BE-ORCH-TURN | {T}-TS-25 | ~25 | T2.8 |
| T4.3 | `ExecuteRound` 重构：sequential 先 + errgroup 并行 + indexed slice 写回 | L4-BE-ORCH-TURN | {T}-TS-25 | ~40 | T4.1, T4.2 |
| T4.4 | 集成测试：2 个 read_file 并行时间 < 2x 单个 + 顺序保持 | L4-BE-ORCH-TURN | {T}-TS-25 | ~60 | T4.3 |
| T4.5 | 集成测试：mixed safe/unsafe 工具混合调度（顺序+并行组合） | L4-BE-ORCH-TURN | {T}-TS-25 | ~40 | T4.3 |
| T4.6 | `go test -race ./...` 必须 100% 绿 | L4-BE-ORCH-TURN | {T}-TS-25 | — | T4.3 |

**交付物**：`internal/bootstrap/turn_adapter.go` v2

## Phase 5 — T22-T25 单测覆盖

| ID | 任务 | L4 | L5 | 估行 | 依赖 |
|----|------|-----|-----|------|------|
| T5.1 | T22: 7 surface 各 1 个 `TestXxxSurface_ToolSpec_HasOrthogonalFlags` 子测试 | L4-BE-CTX-SURFACE | {T}-TS-22 | ~30 | T2.8 |
| T5.2 | T22: 集成测试 `TestIntegration_AllSurfaces_HaveCompleteOrthogonalFlags` | L4-BE-CTX-SURFACE | {T}-TS-22 | ~30 | T2.8 |
| T5.3 | T23: 单测 `TestFreeForkSurface_InterruptBehavior_ReturnsCancel` | L4-BE-CTX-SURFACE | {T}-TS-23 | ~15 | T2.3 |
| T5.4 | T23: 单测 6 个 block surface 断言 | L4-BE-CTX-SURFACE | {T}-TS-23 | ~30 | T2.1, T2.2, T2.4-T2.7 |
| T5.5 | T23: 集成测试 cancel 200ms 内返回 | L4-BE-CTX-SURFACE | {T}-TS-23 | ~40 | T2.3 |
| T5.6 | T24: 集成测试 3 套 opts 顺序稳定 | L4-BE-CTX-BOOTSTRAP | {T}-TS-24 | (T3.2) | T3.1 |
| T5.7 | T25: 集成测试 2 个 read_file 并行时间 < 2x + 顺序保持 | L4-BE-ORCH-TURN | {T}-TS-25 | (T4.4) | T4.3 |

**交付物**：7 surface_test.go + tests/integration/tool_surface_test.go + tests/turn_adapter_test.go

## Phase 6 — 既有 T01-T11 回归 + 兼容性

| ID | 任务 | L4 | L5 | 估行 | 依赖 |
|----|------|-----|-----|------|------|
| T6.1 | T11 devrix tool list CLI 输出顺序改 name 序断言 | L4-BE-CTX-BOOTSTRAP | {T}-TS-11 | ~10 | T3.1 |
| T6.2 | T08 per-agent ⊇ main 测试重跑（应自动 PASS） | L4-BE-ORCH-POLICY | {T}-TS-08 | — | T2.8 |
| T6.3 | T09 turn_adapter.findSurface 测试重跑（dispatch 路径不变） | L4-BE-ORCH-TURN | {T}-TS-09 | — | T4.3 |
| T6.4 | T10 IPermissionGate 集成测试重跑（Risk 字段不变） | L4-BE-ORCH-PERM | {T}-TS-10 | — | T2.8 |
| T6.5 | 全量 `go test -race ./...` + `go vet ./...` + `staticcheck` 必须 0 warning | — | ALL | — | T6.1-T6.4 |
| T6.6 | library 0 行改动核对（git diff 过滤 freefork/tracker/verify/multiagent/orchestration） | — | AC11 | — | T6.5 |

**交付物**：`go test -race ./...` 100% 绿 + library diff = 0

## Phase 7 — T 注册表更新 + 文档同步

| ID | 任务 | L4 | L5 | 估行 | 依赖 |
|----|------|-----|-----|------|------|
| T7.1 | `openspec/specs/tool-surface/t-registry.md` 加 T22-T25 4 个新 P0 T 点 | L5-DOCS | {T}-TS-22~25 | ~50 | T5.7 |
| T7.2 | `docs/methodology/dsaft-methodology.md` §12 加"ToolSpec orthogonal flags" 案例 | L5-DOCS | AC14 | ~40 | T7.1 |
| T7.3 | `openspec/specs/tool-surface/spec.md` 主 spec 同步 v2 字段（REQ-TS-07~11 增量） | L5-DOCS | REQ-TS-07~11 | ~80 | T5.7 |

**交付物**：3 个文档同步

## Phase 8 — S5 验收 + S6 归档

| ID | 任务 | L4 | L5 | 估行 | 依赖 |
|----|------|-----|-----|------|------|
| T8.1 | `verify-archive.sh` 12/12 PASS（OpenSpec S6 归档前置） | — | AC13 | — | T7.3 |
| T8.2 | PR-1 创建（契约+7 surface）+ auto-merge | — | — | — | T8.1 |
| T8.3 | PR-2 创建（BuildSurfaces sort + turn_adapter 并行） + auto-merge | — | — | — | T8.2 |
| T8.4 | PR-3 创建（T22-T25 集成测试 + 文档同步） + auto-merge | — | — | — | T8.3 |
| T8.5 | 归档到 `openspec/archive/2026-06-18-devrix-tool-spec-enrichment/` | — | S6 | — | T8.4 |

**交付物**：3 个 PR auto-merged + 归档目录

---

## 依赖顺序

```
T1.1 → T1.2 → T1.3 → T1.4 (compile fail 是预期)
                   ↓
              T2.1-T2.7 (7 surface 并行)
                   ↓
              T2.8 (compile-time assertion PASS)
                   ↓
        ┌──────────┴──────────┐
        ↓                     ↓
       T3.1-T3.3            T4.1-T4.6
    (BuildSurfaces sort)   (turn_adapter 并行)
        ↓                     ↓
        └──────────┬──────────┘
                   ↓
            T5.1-T5.7 (T22-T25 集成)
                   ↓
            T6.1-T6.6 (既有 T01-T11 回归)
                   ↓
              T7.1-T7.3 (文档)
                   ↓
              T8.1-T8.5 (PR + 归档)
```

---

## 建议 PR 拆分

### PR-1: contracts + 7 surface（最大改动 + 1 个 breaking interface 集中 commit）

**包含 T1.1-T1.4, T2.1-T2.8, T5.1-T5.5**
- `internal/shared/contracts/tool_surface.go` v2
- 7 surface.go 全部 v2
- 7 surface_test.go 各加 1 个子测试
- `tests/integration/tool_surface_test.go` T22+T23 部分

**Review 重点**：
- 7 surface 4 bool 字段填充对照 design.md §2.3
- compile-time `var _ contracts.ToolSurface = ...` 7 处 PASS
- InterruptBehavior 实现：FreeForkSurface=cancel, 其余 6=block

**预估 review 时间**：30-45 min（5 段式）

### PR-2: BuildSurfaces sort + turn_adapter 并行（性能优化）

**包含 T3.1-T3.3, T4.1-T4.6**
- `internal/bootstrap/surfaces.go` +1 行 sort
- `internal/bootstrap/turn_adapter.go` ExecuteRound 重构
- `internal/bootstrap/surfaces_test.go` T24
- `tests/integration/turn_adapter_test.go` T25

**Review 重点**：
- errgroup + indexed slice 写回的顺序保证
- isConcurrencySafe 查表逻辑（surface.Tools 缓存？每次都查？）
- 并行超时兜底（5min）

**预估 review 时间**：30 min

### PR-3: 文档同步 + 既有 T01-T11 回归（验收闭环）

**包含 T6.1-T6.6, T7.1-T7.3, T8.1**
- `openspec/specs/tool-surface/t-registry.md` 加 T22-T25
- `docs/methodology/dsaft-methodology.md` §12 加案例
- `openspec/specs/tool-surface/spec.md` 主 spec 增量
- T11 devrix tool list CLI 顺序断言更新
- `go test -race ./...` + `go vet` + `staticcheck` 全绿

**Review 重点**：
- 文档与代码 1:1 对齐
- 既有 11 T 点全部 PASS

**预估 review 时间**：15-20 min

---

## 风险与回滚

| 风险 | 触发条件 | 回滚策略 |
|---|---|---|
| 7 surface 改 4 bool 字段填充口径不一致 | PR-1 review 发现 | PR-1 修该 surface；不 merge PR-2 |
| turn_adapter 并行结果顺序错乱 | T25 fail | PR-2 改 errgroup → 串行 fallback；T25 改宽松断言 |
| BuildSurfaces sort 顺序与既有 T11 不一致 | T11 fail | T6.1 同步更新断言（不视为回滚） |
| InterruptBehavior='cancel' 200ms 不达标 | T23 集成 fail | FreeForkSurface.Execute 内部 select ctx.Done() 加 select；T23 重跑 |
| errgroup race | `go test -race` fail | 显式锁或改回 WaitGroup；T25 重跑 |
| library 0 行改动被破坏 | T6.6 git diff 非 0 | 该 PR reject；定位误改 library 的 commit 并 revert |

每个 PR 独立可回滚（git revert）。`master` 永远保持 0 global + 0 library 改动 + 4 bool 字段实现 + BuildSurfaces sort + 并行 dispatch 中**至少有"既有的 11 T 点"全部 PASS**。

---

## 与下游 change 的接口

### 留给 DM-20260618-002 (per-tool checkPermission) 的 hook 点

本 change 之后，ToolSpec 已有 4 bool 字段，DM-002 可以：
- 在 `IPermissionGate` 加 `CheckPermission(ctx, spec ToolSpec, input json.RawMessage) Decision` 方法
- 决策消费 `spec.OpenWorld` / `spec.Destructive` 决定是否需要更严格 prompt
- 与 `Risk` 字段联动：Risk=high 强制 OpenWorld=true；Risk=low 强制 OpenWorld=false

### 留给 DM-20260618-003 (lazy loading + Zod) 的 hook 点

本 change 之后，DM-003 可以：
- `ToolSurface` interface + 1 method `ShouldDefer() bool`（已是 6 method）
- `Tools(ctx, workDir, sessionID)` 返回的 spec 标 `DeferLoading: true` 时，LLM 端用 `defer_loading` beta header
- 探索 agent filter 用 `spec.ReadOnly` 预分类，只发"必须"的 tool schema
- Zod schema 仍与 ToolSpec.Parameters (string JSON Schema) 共存

---

## 工作量汇总

| Phase | 估时 | 任务数 |
|---|---|---|
| Phase 1 — 契约扩展 | 1h | 4 |
| Phase 2 — 7 surface 改 | 3h | 8 |
| Phase 3 — BuildSurfaces sort | 0.5h | 3 |
| Phase 4 — turn_adapter 并行 | 2h | 6 |
| Phase 5 — T22-T25 单测 | 1.5h | 7 |
| Phase 6 — 既有 T01-T11 回归 | 0.5h | 6 |
| Phase 7 — 文档同步 | 0.5h | 3 |
| Phase 8 — S5+S6 | 1h | 5 |
| **总计** | **10h (1.5 人日)** | **42** |

---

## 检查清单（S4 完成确认）

- [x] 8 Phase 任务拆分（机械到 0.5h 级别）
- [x] 每个任务标注 L4 / L5 / 估行 / 依赖
- [x] 依赖图清晰（1.3 → 2.x → 3.x+4.x → 5.x → 6.x → 7.x → 8.x）
- [x] 3 个 PR 拆分（按"风险集中度"切：1=breaking 集中 / 2=性能 / 3=文档+回归）
- [x] 既有 11 T 点回归任务显式列出（T6.1-T6.6）
- [x] 风险与回滚策略（6 项）
- [x] 与下游 DM-002/DM-003 的接口预留
- [x] 工作量汇总（10h / 42 tasks）
- [x] S5 验收条件（verify-archive.sh 12/12 PASS）
- [x] S6 归档路径（`openspec/archive/2026-06-18-devrix-tool-spec-enrichment/`）
