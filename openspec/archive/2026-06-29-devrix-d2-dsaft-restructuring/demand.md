# Demand: devrix-d2-dsaft-restructuring (DM-20260629-002)

**Demand ID:** DM-20260629-002
**Status:** S1_Demand
**Priority:** P0 (深度架构重构)
**Created:** 2026-06-29
**Change ID:** devrix-d2-dsaft-restructuring
**Triggered By:** D2 域整体 DSAFT 方法论 Review（2026-06-29 会话）+ 对齐 D7 v6.0.x → v7.0 演进（DM-20260629-001 11 PR / 55 T 模板）
**Related:**
- `devrix-d7-dsaft-restructuring` (DM-20260629-001) — D7 6 子 Change 联动模板
- `devrix-d2-structure-closure` (DM-20260619-007) — v8.2.0 v2.2 结构终态
- `devrix-d1-dsaft-refactor` (DM-20260628-003) — D1 Trusted Intermediary 边界收敛（已 S7_Archived 模板）
- `docs/methodology/dsaft-methodology.md` v4.0.0 — 6 原则
- `docs/methodology/dsaft-refactoring-playbook.md` v1.0.0 — 4 轴 / 6 阶段

---

## §1 背景

D2 v8.2.0（2026-06-19 S7_Archived）完成 v2.2 结构终态（`enforce/toolrunner` → `enforce/tools` + `enforce/sandbox/` 物理迁入 + 根目录 3 生产文件 + `facade/` → `legacy/` P5 Deprecated + `prepare/memory/longterm.go` 拆分 + `MemoryEntry` 提升 shared）。但 2026-06-29 DSAFT Review 暴露 **6 类深度架构债**，对齐 D7 v6.0.x → v7.0 演进节奏需要 6 子 Change 联动 refactoring。

### 1.1 S 层语义偏离 DSAFT 原则 1（P0）

4 canonical S（S15 Prepare / S17 Persist / S18 Enforce + S16 REMOVED）均有 ValueFlow 命名但 `d2-domain.md §North Star` 表**缺 ValueFlow Alias 列**，对齐 D7 v2.6.0 §North Star 6 行 ValueFlow Alias 模式。

**Top 3 真实问题**：
1. **S15 = 4 A 散在 4 物理位置**：`prepare/{memory,compression,prompt}/` + `prepare/conversation/fork*.go`。S15 名称"PrepareExecutionContext"暗示 4 步流水线（Load → Recall → Compress → Assemble），代码按子目录拆。
2. **S17 = 3 A 散在 4 物理位置**：`persist/{snapshot,transcript}/` + `persist/{commit_window,commit,orchestrator,memory/store}.go`。S17 名称"PersistSessionState"暗示单一持久化管道，代码按子系统拆。
3. **S18 = 7 A 散在 5 物理位置**：`enforce/{permission,tools,registry}/` + `enforce/{background,subquery,background_task_tools,planmode_tools,sandbox}.go`。S18 名称"EnforceExecutionPolicy"暗示权限单一契约，代码按子功能拆。

### 1.2 5 个 god function 跨 3 个 S（P0）

| 文件:函数 | LOC | S | 风险等级 |
|---|---|---|---|
| `prepare/compression/pipeline.go::RunForSession()` | 109 | S15 | High（7 步流水线） |
| `prepare/prompt/assembler.go::Build()` | 55 | S15 | Mid（5 step 组装） |
| `materialize/materializer.go::Materialize()` | 30 | S16 | Low（4 step materialize） |
| `enforce/tools/sandboxast/analyzer.go::Analyze()` | 39 | S18 | Low（AST 遍历） |
| `enforce/background.go::RunBackground()` | 26 | S18 | Mid（CRUD + 执行） |

根因：v8.2.0 物理迁移后未做职责子模块拆分。

### 1.3 Registry 路径漂移（P0）

9 个 F 路径与 code 100% 漂移：
- `D2-S15-A02` 标 `prepare/memory/longterm.go` → 实际 `prepare/memory/recall.go` + `persist/memory/store.go`（v8.2 P4 split）
- `D2-S15-A03` 标 `prepare/compression/` 但缺 `compression_steps.go`（待 PR-2 拆出）
- `D2-S15-A04` 标 `prepare/prompt/assembler.go` 但缺 `assembler_layers.go`（待 PR-2 拆出）
- `D2-S17-A02` 标 `persist/commit_window.go` 但实际在 `persist/commit_window/` 子目录
- `D2-S18-A01` 标 `permission/` 路径 OK 但缺子目录展开
- `D2-S18-A02` 标 `tools/` 但实际 `enforce/tools/surface/` + `sandboxast/` + `builtin/`
- `D2-S16-A20` Materialize 子活动标 `materialize/` 但缺子文件展开
- t-registry `D2-S15-A02-T01..T04` 路径加 `prepare/conversation/` 前缀缺失
- t-registry `D2-S18-A02-T01..T15` 路径加 `enforce/tools/` 前缀缺失

### 1.4 ~1274 LOC 死代码（P0）

**双 Agent grep 验证 0 外部调用者的 legacy/ 目录**：

| 文件 | LOC | P5 状态 | 备注 |
|---|---|---|---|
| `legacy/engine.go` | 484 | P5 Deprecated | `ContextEngine.Process()` 入口（D7 turn_runner 已接管） |
| `legacy/engine_persist_v2.go` | 220 | P5 Deprecated | `persistTurn()` 委托（D7 直接调 persist 层） |
| `legacy/engine_builder.go` | 133 | P5 Deprecated | `NewContextEngine()`（已被 kernel/ 替代） |
| `legacy/engine_types.go` | 104 | P5 Deprecated | `EngineDeps` struct |
| `legacy/persist_adapters.go` | 80 | P5 Deprecated | snapshot/transcript/longTerm adapter |
| `legacy/engine_events.go` | 70 | P5 Deprecated | `infoEvent`/`errorEvent` helper |
| `legacy/engine_compression.go` | 61 | P5 Deprecated | `compressionPipeline()` helper |
| `legacy/commit_window_adapter.go` | 43 | P5 Deprecated | adapter bridge |
| `legacy/engine_export.go` | 46 | P5 Deprecated | `ToolRegistry()`/`Surfaces()` accessors |
| `legacy/prepared_turn_result.go` | 14 | P5 Deprecated | test helper |
| `legacy/prepared_turn_wire.go` | 19 | P5 Deprecated | test helper |
| `aliases.go`（根） | 24 | P5 Deprecated | `contextengine.Process`/`EngineDeps`/`NewContextEngine` 别名 |
| **总计** | **1274 + 24 = 1298** | | |

**唯一外部调用者**：`cmd/devrix/main.go` + `cmd/obs-verify/main.go` + `tests/testutil/engine_deps.go` + 13 test 文件 — 全部可直接改为 `kernel.ContextEngine` / `prepare.PrepareOrchestrator` / `persist.PersistOrchestrator` import。

### 1.5 文档漂移（P1）

| 文件:行 | 漂移 |
|---|---|
| `d2-domain.md §DSAFT 双轨` line 89 | "D2 root | Public API re-exports | `contracts.go`, `aliases.go`, `tool_context.go`（3 生产文件，type alias only）" — `aliases.go` 实际 24 LOC 含 `Process`/`EngineDeps` 重导出（不纯 type alias） |
| `d2-domain.md §物理路径映射表` line 86 | "**P5 Deprecated** (slog.Warn + T07 guard)" 状态描述 OK；但 11 文件名未全列 |
| `f-registry.md` header line 7 | "Last Updated: 2026-06-16" v3.1.0 — 实际 v8.2.0 物理迁移后未更新 |
| `t-registry.md` line 1-18 | Change 注释未含 `devrix-api-error-classification` 等 v8.2 后续 |
| `a-registry.md` §Canonical S/A | S15/S17/S18 物理路径 4/4/7 拆分后未列子文件 |

### 1.6 T↔Span 覆盖率仅 ~20%（P1）

**30 D2 span ops 中 26 个 dead**（harness 6 + queryloop 3 + context 8 + tool 2 + task/plan 7 + 历史路径 2 - 实际生产 emit = 仅 `context.process` + `context.compression.run` + `context.compression.step` + `context.materialize` 4 active）。t-registry **缺 Span Evidence 列**，对齐 D7 v2.6.0 94% 覆盖率。

### 1.7 DM-018 slice-c 待 D7 跨域（P1）

`nested/flow_report.go`（D2-S19 DISMANTLED 残留）应迁 `orchestration/executionflow/bridge/subquery_bridge.go`（DM-018 slice-c 收口），但当前未迁（D2→D7 import cycle 风险待解）。

### 1.8 跨域 fixture 归属未明示（P1）

`summarizer_fixture.go` + `prepared_turn_fixture.go`（D2 根）跨 D2 + D7 使用，但 `t-registry.md` 未标 fixture type，可能影响未来跨域 fixture 重新分配决策。

---

## §2 范围

### 2.1 In Scope（本次 Change 解决）

**6 个子 Change（S→A→F 逐层 + 清理）**：

| 子 Change | 层级 | 内容 | 工作量 |
|---|---|---|---|
| **#0** legacy-cleanup | 横切 | 11 legacy/ + aliases.go 全删（1274 + 24 LOC）+ 13 test + 2 cmd + testutil import path 迁移 | 1 PR / 1 天 |
| **#1** god-fn-split | S15/S18 + F | 5 god fn 拆 10 文件（pipeline 7 步 + assembler 5 步 + materializer 4 步 + analyzer AST + background CRUD） | 2 PR / 2 天 |
| **#2** registry-sync | F + 横切 | 9 F 路径修正 + Historical appendix（S1/S9/S10/S19/S20）+ t-registry 子目录前缀 | 1 PR / 1 天 |
| **#3** value-flow-rename | S | 4 S 配 ValueFlow Alias（加 `D2_` 前缀避免与 D7 冲突）+ 用户感知层 | 1 PR / 1 天 |
| **#4** span-coverage | A + F | 26 dead ops 删 + names.go 同步 + t-registry Span Evidence 列填充 94% | 2 PR / 2 天 |
| **#5** boundary-decision | S + 横切 | DM-018 slice-c 迁 D7 + boundary_decision.go 治理常量 + fixture type 标 | 1 PR / 1 天 |

**总计 8 PR / ~44 T / ~7-9 天**。

### 2.2 Out of Scope

- 不删 S 旧编号（DSAFT 原则 3：T 是安全网）
- 不下沉 `nested/flow_report.go` 之外的 D2→D7 能力（保留实现，仅 Decision 标注）
- 不动 D2 拆面契约（Prepare / ToolRound / Persist 三个契约稳定）
- 不动 D2→D3 import ban（DM-020 CI 硬阻断）
- 不动 D2 Thin（无 orchestration/multiagent import）原则
- 不下沉 `MemoryEntry` 到 shared/types 之外的跨域类型（已 S7_Archived DM-20260619-007 P4 split）
- 不重构 Worktree v2 升级（TD-WT-01..06 单独 change）
- 不拆 S19 DISMANTLED 历史债的旧实现（已物理删除 nested/ 目录）

---

## §3 Goals

| Goal | Metric | Target |
|---|---|---|
| **G1**：~1298 LOC 死代码 + legacy 全删 | grep + test PASS | 0 死符号 + `legacy/` 目录不存在 |
| **G2**：5 god fn 拆完 | wc -l | 每个文件 <800 行；共 10 文件 |
| **G3**：9 F 路径全对 | f-registry + grep | 9/9 修正 + t-registry 子目录前缀同步 |
| **G4**：4 S 配 ValueFlow Alias | d2-domain.md §North Star | 4/4 |
| **G5**：T↔Span 覆盖率 ≥80% | t-registry Span Evidence 列 | ≥80%（当前 ~20%）；目标 94%（对齐 D7） |
| **G6**：跨域越界 Decision 表 | d2-domain.md §Out of Scope + boundary_decision.go | DM-018 slice-c + fixture type 标 |
| **G7**：Historical Appendix 完整 | t/a-registry appendix 段 | S1/S9/S10/S19/S20 5 历史 S 全列 |
| **G8**：d2-domain.md v8.2.0 → v9.0.0 | d2-domain.md Version 行 | ✅ |
| **G9**：D2 packages -race PASS | regression | 22+/22+ packages |
| **G10**：verify-archive.sh | acceptance | 12/12 PASS |
| **G11**：P0 T 100% PASS | acceptance | t-registry P0 子集 |
| **G12**：god fn 5 T 重映射 | t-registry 更新 | 5/5 归属正确 |
| **G13**：D2 不 import `orchestration` | lint-d2-imports.sh | 0 命中 |

---

## §4 解决思路（6 子 Change 拆分）

### 4.1 #0 legacy-cleanup（先行 PR）

**核心动作**：1298 LOC 死代码 + 老链路全删，0 业务逻辑改动。

**清理清单**：
1. 删 `internal/layers/contextengine/legacy/*.go` 11 文件
2. 删 `internal/layers/contextengine/aliases.go` 24 LOC
3. 改 13 test 文件 import path：`contextengine.{Process,EngineDeps,NewContextEngine}` → `kernel.ContextEngine` / `prepare.PrepareOrchestrator` / `persist.PersistOrchestrator`
4. 改 `cmd/devrix/main.go` + `cmd/obs-verify/main.go` import path
5. 改 `tests/testutil/engine_deps.go` 实现：直接调 `kernel.NewContextEngine(...)` 替代 `legacy.NewContextEngine(...)`

**AC**：grep 验证 0 外部调用者；`go test ./internal/layers/contextengine/... ./cmd/devrix/... ./tests/... -race` PASS；22+ packages -race PASS。

### 4.2 #1 god-fn-split（5 god fn 拆 10 文件）

**核心动作**：5 god fn 按"职责子模块"拆 10 文件（每 god fn 拆 2 文件）。

| 新文件 | 职责 | 估算 LOC |
|---|---|---|
| `compression_steps.go` (S15) | 7 step helper（deduplicate/snip/fold/expire/[autocompact]/assemble/token_block） | <500 |
| `compression_budget.go` (S15) | token validation + budget validation | <300 |
| `prompt_assembler_layers.go` (S15) | buildCoreLayer + buildLayer3 | <300 |
| `prompt_dynamic_sections.go` (S15) | buildDynamicSections | <200 |
| `materialize_prompts.go` (S16) | buildSystemPrompt + buildWaveSystemPrompt | <300 |
| `materialize_compressor.go` (S16) | compressMessages | <300 |
| `ast_walker.go` (S18) | walk function | <300 |
| `ast_rules.go` (S18) | rule registry | <300 |
| `background_registry.go` (S18) | CRUD | <300 |
| `background_notifications.go` (S18) | queue integration | <200 |

**AC**：每个新文件 wc -l <800；拆前后 t-registry T 编号归属一致；22+ packages -race PASS。

### 4.3 #2 registry-sync（F 路径 + Historical appendix）

**核心动作**：
1. 9 个 F 路径修正（详见 §1.3）
2. t-registry Historical appendix：S1 (PEV, RETIRED) / S9 (Harness, REMOVED) / S10 (QueryLoop, REMOVED) / S19 (NestedExecution, DISMANTLED) / S20 (LegacyHarnessFallback, REMOVED) — 移到独立 §Historical Appendix
3. a-registry §DISMANTLED/REMOVED 状态行补全
4. f-registry 47 → 38 F（历史 F 入 Historical appendix）
5. d2-domain.md 物理路径表与 code 100% 对齐
6. 同步 `d7-boundary.md` 中 D2 边界描述

**AC**：f-registry 路径 100% 正确（grep 验证）；t-registry Historical Appendix 5 历史 S 全列。

### 4.4 #3 value-flow-rename（S 层语义升级）

**核心动作**：4 canonical S 加 ValueFlow Alias 列（同 D7 v2.6.0 模式）。

| S 名 | ValueFlow Alias | 加 `D2_` 前缀 |
|---|---|---|
| S15 PrepareExecutionContext | Context Loading & Compression | D2_Context_Loading_Compression |
| S16 Materialize Context（v8.2 rename） | LLM-Ready Context Assembly | D2_LLM_Ready_Assembly |
| S17 PersistSessionState | Session State Persistence | D2_Session_State_Persistence |
| S18 EnforceExecutionPolicy | Tool Permission & Sandbox | D2_Tool_Permission_Sandbox |

**AC**：4/4 S 配 ValueFlow Alias；a/f/t-registry ValueFlow Semantic 列完整；不删旧 S 编号。

### 4.5 #4 span-coverage（26 dead ops 删 + 94% 覆盖）

**核心动作**：
1. 删 26 dead ops span（harness 6 + queryloop 3 + context 8 + tool 2 + task/plan 7）
2. `internal/layers/observability/instrument/telemetry/names.go` 同步删除
3. t-registry 232 T 行加 Span Evidence 列
4. Coverage Guard CI script（`scripts/d2-span-coverage.sh`）≥80% 守门
5. observability-guide.md 新增 §T-Without-Span Tracker

**期望覆盖率**：94%（220/232 T 映射到 4 active ops；12 个未映射 T 标 `—` 在 T-Without-Span Tracker）。

**AC**：30 → 4 active span ops；names.go 编译过；Coverage Guard ≥80% 守门。

### 4.6 #5 boundary-decision（DM-018 slice-c + boundary 治理）

**核心动作**：
1. `nested/flow_report.go` 物理迁 `orchestration/executionflow/bridge/subquery_bridge.go`（DM-018 slice-c 收口）
2. d2-domain.md §Out of Scope 加 Pending Boundary Decision 列 + DM-018 slice-c 标注
3. 跨域 fixture 决策：`summarizer_fixture.go` + `prepared_turn_fixture.go` 仍留 D2 根（无 import cycle 风险），t-registry 显式标 fixture type
4. `internal/layers/contextengine/orchtypes/boundary_decision.go` 治理常量（`BoundaryDM018SliceC = "boundary-debt:dm-018-slice-c-v7.0"`） + 4 单元测试

**AC**：DM-018 slice-c 物理迁移完成；D2 不再持有 FlowEvent ownership；boundary_decision.go + test PASS。

---

## §5 Risks

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| legacy/ 删后 13 test 文件编译失败 | High | High | PR-1 前先 `grep -rE` 全量审计，机械替换 import path；`go test -race ./...` 守门 |
| pipeline.go 109 LOC 拆破坏 7 步顺序 | Mid | High | t-registry 加 invariant T 守门；拆后 -race PASS 守门 |
| 26 dead span ops 删后 observability 集成测试断 | Mid | Mid | PR-6 前先 grep 所有引用；coverage integration test 同步更新 |
| DM-018 slice-c 迁移触发 D7 cycle | Low | High | D2→D7 已有 Hard Ban（D2-THIN-T01），反向 import D7→D2 需新建 `internal/shared/bridge/` |
| ValueFlow Alias 与 D7 alias 命名冲突 | Low | Mid | D2 alias 加 `D2_` 前缀区分（`D2_Context_Loading_Compression` 等） |
| 8 PR 联动回归测试成本 | Mid | Mid | 每 PR 后 22+ packages -race PASS 守门；最终 verify-archive.sh 12/12 |

---

## §6 相关

- `docs/methodology/dsaft-methodology.md` v4.0.0
- `docs/methodology/dsaft-refactoring-playbook.md` v1.0.0
- `openspec/specs/d2-context-engine/d2-domain.md` v8.2.0
- `openspec/specs/d2-context-engine/{a,f,t,span}-registry.md`
- `openspec/specs/d2-context-engine/observability-guide.md`
- `openspec/specs/d7-orchestration/d7-domain.md` v2.6.0（ValueFlow Alias 模式 SoT）
- `openspec/archive/2026-06-29-devrix-d7-dsaft-restructuring/` — DM-20260629-001 S7_Archive 模板
- `openspec/archive/2026-06-28-devrix-d1-dsaft-refactor/` — DM-20260628-003 S7_Archive 模板
- `internal/layers/contextengine/` — 8 包 / 41 prod / 36 test / ~12K LOC（含 legacy/）