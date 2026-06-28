# Proposal: devrix-d2-dsaft-restructuring (DM-20260629-002)

**Change ID:** `devrix-d2-dsaft-restructuring`
**Demand ID:** DM-20260629-002
**Priority:** P0
**Sprint:** d2-v8.2.0 维护阶段 → v9.0.0 演进起点
**PR Count:** 8 (6 子 Change 联动)
**Status:** S2_Proposal → S3_Design → S4_Implemented → S5_Accepted → S7_Archived
**DSAFT 阶段:** §阶段 3 North Star 重整 + §阶段 4 v1.1 Traceability + §阶段 6 双锚点对齐 + 原则 1/3/4/5 修复
**Template:** `devrix-d7-dsaft-restructuring` (DM-20260629-001) — 6 子 Change 模式一致

---

## 1. Background

D2 v8.2.0 (2026-06-19 S7_Archived) 已完成 v2.2 结构终态（`enforce/toolrunner/` → `enforce/tools/` + `enforce/sandbox/` 物理迁入 + 根目录 3 生产文件 + `facade/` → `legacy/` P5 Deprecated + `prepare/memory/longterm.go` 拆分 + `MemoryEntry` 提升 shared）。但 2026-06-29 DSAFT Review 暴露 **6 类深度架构债**，对齐 D7 v6.0.x → v7.0 演进节奏（DM-20260629-001 11 PR / 55 T 模板）需要 6 子 Change 联动 refactoring。

1. **S 层语义偏离**（4 canonical S 缺 ValueFlow Alias 列，违反原则 1）
2. **god function 累积**（5 god fn 跨 3 个 S，最长 109 LOC）
3. **Registry 路径漂移**（9 F 路径错误）
4. **~1298 LOC 死代码 + 老链路**（双 Agent grep 验证 0 外部调用者）
5. **文档漂移**（d2-domain.md aliases.go 描述 + f-registry 头部 + t-registry change log 多处）
6. **T↔Span 覆盖率仅 ~20%** + DM-018 slice-c 跨域待收口 + 跨域 fixture type 未标

详见 `demand.md` §1。

---

## 2. Problem Statement

D2 在 DSAFT 方法论 4 轴评估下处于 **"v8.2.0 结构已闭合、god function 累积、registry 路径漂移、~1298 LOC 死代码、T↔Span 覆盖仅 ~20%、DM-018 跨域债遗留"** 的复杂中间状态。

| 维度 | 当前 | 目标 |
|---|---|---|
| S 层合规 | ⭐⭐⭐ | ⭐⭐⭐⭐⭐（加 ValueFlow Alias） |
| A 层合规 | ⭐⭐⭐（god function 拖分） | ⭐⭐⭐⭐（5 god fn 拆 10 文件） |
| F 层合规 | ⭐⭐（9 路径错 + t-registry 子目录前缀缺） | ⭐⭐⭐⭐⭐（9/9 修正） |
| T↔Span 追溯 | ⭐（~20%） | ⭐⭐⭐⭐⭐（≥80%，目标 94%） |
| 跨域边界 | ⭐⭐（DM-018 slice-c + fixture type 未标） | ⭐⭐⭐⭐ |
| 死代码债 | ⭐（~1298 LOC） | ⭐⭐⭐⭐⭐（0） |
| God function 债 | ⭐⭐（5 god fn） | ⭐⭐⭐⭐（0 god fn） |

需要 **6 个子 Change 联动** + **8 个 PR** + **~44 AC** 全闭环。

---

## 3. Goals / Non-Goals

### 3.1 Goals（13 项量化指标）

| Goal | Metric | Target |
|---|---|---|
| **G1**：~1298 LOC 死代码 + legacy 全删 | grep + 22+ -race | 0 死符号 + `legacy/` 目录不存在 |
| **G2**：5 god fn 拆 10 文件 | wc -l | 每个文件 <800 行 |
| **G3**：9 F 路径 + t-registry 子目录前缀全对 | f-registry + t-registry | 9/9 修正 |
| **G4**：4 S 配 ValueFlow Alias（加 `D2_` 前缀） | d2-domain.md §North Star | 4/4 |
| **G5**：T↔Span 覆盖率 ≥80% | t-registry Span Evidence 列 | ≥80%（当前 ~20%）；目标 94% |
| **G6**：跨域越界 Decision 表 + DM-018 slice-c 物理迁移 | d2-domain.md §Out of Scope | DM-018 + fixture type |
| **G7**：Historical Appendix 完整 | t/a-registry appendix 段 | 5 历史 S 全列 |
| **G8**：d2-domain.md v8.2.0 → v9.0.0 | d2-domain.md Version 行 | ✅ |
| **G9**：D2 packages -race PASS | regression | 22+/22+ |
| **G10**：verify-archive.sh | acceptance | 12/12 PASS |
| **G11**：P0 T 100% PASS | acceptance | t-registry P0 子集 |
| **G12**：god fn 5 T 重映射 | t-registry 更新 | 5/5 |
| **G13**：D2 不 import `orchestration` | lint-d2-imports.sh | 0 命中 |

### 3.2 Non-Goals

- 不删 S 旧编号（DSAFT 原则 3）
- 不下沉 `nested/flow_report.go` 之外的 D2→D7 能力（仅 DM-018 slice-c 物理迁移）
- 不动 D2 拆面契约（Prepare / ToolRound / Persist 三个契约稳定）
- 不动 D2→D3 import ban（DM-020 CI 硬阻断）
- 不动 D2 Thin 原则（无 orchestration/multiagent import）
- 不重构 Worktree v2 升级（TD-WT-01..06 单独 change）
- 不拆 S19 DISMANTLED 历史债的旧实现（已物理删除 nested/ 目录）

---

## 4. Solution（6 子 Change 拆分）

### 4.1 子 Change #0：legacy-cleanup（先行 PR，0 业务改动）

**PR-1 范围**：~1298 LOC 死代码 + 老链路删除 + 13 test + 2 cmd + testutil import path 迁移

**修改清单**：

| # | 文件 | 改动 | LOC 变化 |
|---|---|---|---|
| 0.1 | `internal/layers/contextengine/legacy/engine.go` | 删 | -484 |
| 0.2 | `internal/layers/contextengine/legacy/engine_persist_v2.go` | 删 | -220 |
| 0.3 | `internal/layers/contextengine/legacy/engine_builder.go` | 删 | -133 |
| 0.4 | `internal/layers/contextengine/legacy/engine_types.go` | 删 | -104 |
| 0.5 | `internal/layers/contextengine/legacy/persist_adapters.go` | 删 | -80 |
| 0.6 | `internal/layers/contextengine/legacy/engine_events.go` | 删 | -70 |
| 0.7 | `internal/layers/contextengine/legacy/engine_compression.go` | 删 | -61 |
| 0.8 | `internal/layers/contextengine/legacy/commit_window_adapter.go` | 删 | -43 |
| 0.9 | `internal/layers/contextengine/legacy/engine_export.go` | 删 | -46 |
| 0.10 | `internal/layers/contextengine/legacy/prepared_turn_result.go` | 删 | -14 |
| 0.11 | `internal/layers/contextengine/legacy/prepared_turn_wire.go` | 删 | -19 |
| 0.12 | `internal/layers/contextengine/aliases.go` | 删 | -24 |
| 0.13 | 13 个 `*_test.go` 文件 | 改 import path: `contextengine.Process` → `kernel.ContextEngine` / `prepare.PrepareOrchestrator` / `persist.PersistOrchestrator` | 0 |
| 0.14 | `cmd/devrix/main.go` + `cmd/obs-verify/main.go` | 改 import path | 0 |
| 0.15 | `tests/testutil/engine_deps.go` | 改实现：直接调 `kernel.NewContextEngine(...)` | 0 |

**总 LOC 变化**：约 -1298 LOC（1298 死代码全删）

**AC**：grep 验证 0 外部调用者；22+ D2 packages -race PASS；0 test 失败；verify-archive.sh 12/12 PASS。

### 4.2 子 Change #1：god-fn-split（5 god fn 拆 10 文件）

**PR-2 + PR-3 范围**：5 god fn 按"职责子模块"拆 10 文件

**拆分方案**：

```
S15 PrepareExecutionContext:
  pipeline.go::RunForSession (109 LOC, god)
     ↓ 拆
     ├─ pipeline.go (<300 行, 主入口)
     ├─ compression_steps.go (<500 行, 7 step helper)
     └─ compression_budget.go (<300 行, token validation)
  
  assembler.go::Build (55 LOC, god)
     ↓ 拆
     ├─ assembler.go (<300 行, core)
     ├─ prompt_assembler_layers.go (<300 行, buildCoreLayer+buildLayer3)
     └─ prompt_dynamic_sections.go (<200 行, buildDynamicSections)

S16 Materialize:
  materializer.go::Materialize (30 LOC, god)
     ↓ 拆
     ├─ materializer.go (<200 行, core)
     ├─ materialize_prompts.go (<300 行, buildSystemPrompt+buildWaveSystemPrompt)
     └─ materialize_compressor.go (<300 行, compressMessages)

S18 EnforceExecutionPolicy:
  sandboxast/analyzer.go::Analyze (39 LOC, god)
     ↓ 拆
     ├─ analyzer.go (<200 行, core)
     ├─ ast_walker.go (<300 行, walk function)
     └─ ast_rules.go (<300 行, rule registry)
  
  background.go::RunBackground (26 LOC, god)
     ↓ 拆
     ├─ background_registry.go (<300 行, CRUD)
     └─ background_notifications.go (<200 行, queue integration)
```

**AC**：每个新文件 wc -l <800；5 god fn T 全部归属正确（不再单挂一函数）；22+ packages -race PASS。

### 4.3 子 Change #2：registry-sync（F 路径 + Historical appendix）

**PR-4 范围**：F 路径 + t-registry 子目录前缀 + Historical appendix

**修改清单**：

| # | 文件 | 改动 |
|---|---|---|
| 2.1 | `openspec/specs/d2-context-engine/f-registry.md` D2-S15-A02 | 路径 `prepare/memory/longterm.go` → `prepare/memory/recall.go` + `persist/memory/store.go`（v8.2 P4 split） |
| 2.2 | `openspec/specs/d2-context-engine/f-registry.md` D2-S15-A03 | 路径加 `compression_steps.go`（PR-2 拆出） |
| 2.3 | `openspec/specs/d2-context-engine/f-registry.md` D2-S15-A04 | 路径加 `prompt_assembler_layers.go`（PR-2 拆出） |
| 2.4 | `openspec/specs/d2-context-engine/f-registry.md` D2-S17-A02 | 路径加 `commit_window/` 子目录展开 |
| 2.5 | `openspec/specs/d2-context-engine/f-registry.md` D2-S18-A01 | 路径加 `permission/` 子目录展开 |
| 2.6 | `openspec/specs/d2-context-engine/f-registry.md` D2-S18-A02 | 路径加 `enforce/tools/{surface,sandboxast,builtin}/` 子目录展开 |
| 2.7 | `openspec/specs/d2-context-engine/f-registry.md` D2-S16-A20 | 路径加 `materialize/{prompts,compressor}.go` 子文件展开 |
| 2.8 | `openspec/specs/d2-context-engine/t-registry.md` D2-S15-A02-T01..T04 | 路径加 `prepare/conversation/` 前缀 |
| 2.9 | `openspec/specs/d2-context-engine/t-registry.md` D2-S18-A02-T01..T15 | 路径加 `enforce/tools/` 前缀 |
| 2.10 | `openspec/specs/d2-context-engine/t-registry.md` §Historical Appendix | S1 (PEV, RETIRED) / S9 (Harness, REMOVED) / S10 (QueryLoop, REMOVED) / S19 (NestedExecution, DISMANTLED) / S20 (LegacyHarnessFallback, REMOVED) — 移到独立段 |
| 2.11 | `openspec/specs/d2-context-engine/a-registry.md` §DISMANTLED/REMOVED | 状态行补全 |
| 2.12 | `openspec/specs/d2-context-engine/f-registry.md` 头部 | 47 → 38 F（历史 F 入 Historical appendix） |
| 2.13 | `openspec/specs/d2-context-engine/d2-domain.md` line 89 | aliases.go 描述修正（不纯 type alias） |
| 2.14 | `openspec/specs/d2-context-engine/d2-domain.md` §物理路径映射表 | 11 legacy/ 文件名全列 |
| 2.15 | `openspec/specs/d2-context-engine/d7-boundary.md` | D2 边界描述同步 |

**AC**：f-registry 路径 100% 正确（grep 验证）；t-registry Historical Appendix 5 历史 S 全列；f-registry 47 → 38 F。

### 4.4 子 Change #3：value-flow-rename（S 层语义升级）

**PR-5 范围**：纯文档

**修改清单**：d2-domain.md §North Star 4 S 加 ValueFlow Alias 列；a/f/t-registry 加 ValueFlow Semantic 列；layer-delta.md §Canonical 4 S 加别名。

| S 名 | ValueFlow Alias | 加 `D2_` 前缀 |
|---|---|---|
| S15 PrepareExecutionContext | **Context Loading & Compression** | `D2_Context_Loading_Compression` |
| S16 Materialize Context（v8.2 rename） | **LLM-Ready Context Assembly** | `D2_LLM_Ready_Assembly` |
| S17 PersistSessionState | **Session State Persistence** | `D2_Session_State_Persistence` |
| S18 EnforceExecutionPolicy | **Tool Permission & Sandbox** | `D2_Tool_Permission_Sandbox` |

**用户故事**：
- S15: 用户想"在不超 token 预算的前提下拿到完整上下文"
- S16: 用户想"按调用场景拿到 LLM-ready 上下文"
- S17: 用户想"会话状态可恢复 / 可审计"
- S18: 用户想"工具调用按权限受控执行"

**AC**：4/4 S 配 ValueFlow Alias；a/f/t-registry ValueFlow Semantic 列完整；不删旧 S 编号。

### 4.5 子 Change #4：span-coverage（26 dead ops 删 + 94% 覆盖）

**PR-6 + PR-7 范围**：26 dead ops span 删 + t-registry Span Evidence 列填充

**26 个 dead ops 删除**：

| 类别 | 数量 | 列表 |
|---|---|---|
| Harness | 6 | `context.harness.bootstrap.run` / `context.harness.bootstrap.stage` / `context.harness.tool_pool` / `context.harness.preflight` / `context.harness.route` / `context.system_prompt.build` |
| QueryLoop | 3 | `query.loop.*` |
| Context | 8 | `context.snapshot.load` / `context.system_prompt.load` / `context.longterm.recall` / `context.longterm.store` / `context.tools.register` / `context.memory.snapshot.save` / 等 |
| Tool | 2 | `tool.execute.single` / `tool.execute.permission` |
| Task/Plan | 7 | `task.plan.*` / `task.plan_mode.*` / `task.manager.*`（已迁 D7 workmodel） |

**4 个 active ops 保留**：`context.process` + `context.compression.run` + `context.compression.step` + `context.materialize`

**PR-6 范围**：
- 删 26 dead ops 常量（`internal/layers/observability/instrument/telemetry/names.go`）
- 同步删除所有引用（D2 域 + observability 集成测试）
- coverage integration test 同步更新

**PR-7 范围**：
- t-registry 232 T 行加 Span Evidence 列
- active ops 映射到 T 行
- dead/未映射 T 标 `—`（compile-time check / internal helper）
- Coverage Guard CI script（`scripts/d2-span-coverage.sh`）≥80% 守门

**期望覆盖率**：94%（220/232 T 映射到 4 active ops；12 个未映射 T 标 `—` 在 T-Without-Span Tracker）。

**AC**：30 → 4 active span ops；names.go 编译过；Coverage Guard ≥80% 守门；observability-guide.md 新增 §T-Without-Span Tracker。

### 4.6 子 Change #5：boundary-decision（DM-018 slice-c + boundary 治理）

**PR-8 范围**：DM-018 slice-c 物理迁移 + boundary_decision.go 治理常量

**修改清单**：

| # | 文件 | 改动 |
|---|---|---|
| 5.1 | `internal/layers/contextengine/nested/flow_report.go` | 物理迁 `internal/layers/orchestration/executionflow/bridge/subquery_bridge.go` |
| 5.2 | `openspec/specs/d2-context-engine/d2-domain.md` §Out of Scope | 加 Pending Boundary Decision 列 + DM-018 slice-c 标注 |
| 5.3 | `openspec/specs/d2-context-engine/d7-boundary.md` | DM-018 slice-c 标注 |
| 5.4 | `internal/layers/contextengine/orchtypes/boundary_decision.go` (NEW) | 治理常量 `BoundaryDM018SliceC = "boundary-debt:dm-018-slice-c-v7.0"` |
| 5.5 | `internal/layers/contextengine/orchtypes/boundary_decision_test.go` (NEW) | 4 单元测试 |
| 5.6 | t-registry `summarizer_fixture.go` + `prepared_turn_fixture.go` | 显式标 fixture type |

**约束**：
- D2→D7 已有 Hard Ban（D2-THIN-T01），反向 import D7→D2 需新建 `internal/shared/bridge/`
- 跨域 fixture 仍留 D2 根（无 import cycle 风险），t-registry 显式标 fixture type
- 不删旧实现（DSAFT 原则 3）

**AC**：DM-018 slice-c 物理迁移完成；D2 不再持有 FlowEvent ownership；boundary_decision.go + test PASS。

---

## 5. PR 序列与依赖图

```
PR-1 (#0 legacy-cleanup, 纯删除 + import path replace)
   ↓ no blocker
   ├─ PR-2 (#1 god-fn-split pt1: pipeline + assembler)
   │  ↓
   │  PR-3 (#1 god-fn-split pt2: materializer + analyzer + background)
   │
   ├─ PR-4 (#2 registry-sync, F 路径 + Historical appendix)
   │  ↓ no blocker
   │  PR-5 (#3 value-flow-rename, 纯文档)
   │  ↓
   │  PR-6 (#4 span-coverage pt1: 26 dead ops 删 + names.go 同步)
   │  ↓
   │  PR-7 (#4 span-coverage pt2: 232 T 行 Span Evidence 94%)
   │
   └─ PR-8 (#5 boundary-decision: DM-018 slice-c + boundary 治理)

verify-archive.sh + S7_Archive
```

**PR 顺序依赖**：
- **PR-1 必须先**（删 legacy/ 后才能进行后续 PR 的代码审计）
- **PR-2 → PR-3 顺序**（先拆 S15，再拆 S16 + S18）
- **PR-4 → PR-5 顺序**（先 registry-sync → value-flow-rename）
- **PR-6 → PR-7 顺序**（先删 dead ops → 后填 Span Evidence 列）
- **PR-1 与 PR-4 可并行**（互不依赖）

---

## 6. Risks

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| legacy/ 删后 13 test 文件编译失败 | High | High | PR-1 前先 `grep -rE` 全量审计，机械替换 import path；`go test -race ./...` 守门 |
| pipeline.go 109 LOC 拆破坏 7 步顺序 | Mid | High | t-registry 加 invariant T 守门；拆后 -race PASS 守门 |
| 26 dead span ops 删后 observability 集成测试断 | Mid | Mid | PR-6 前先 grep 所有引用；coverage integration test 同步更新 |
| DM-018 slice-c 迁移触发 D7 cycle | Low | High | D2→D7 已有 Hard Ban（D2-THIN-T01），反向 import D7→D2 需新建 `internal/shared/bridge/` |
| ValueFlow Alias 与 D7 alias 命名冲突 | Low | Mid | D2 alias 加 `D2_` 前缀区分（`D2_Context_Loading_Compression` 等） |
| 8 PR 联动回归测试成本 | Mid | Mid | 每 PR 后 22+ packages -race PASS 守门；最终 verify-archive.sh 12/12 |
| fixtures 跨域 type 标引发争议 | Low | Mid | t-registry 显式标 fixture type；fixture 物理位置不变（无 import cycle 风险） |
| historical S 移除标破坏 §Canonical S/A 段 | Low | Low | D2-S1/S9/S10/S19/S20 移到独立 §Historical Appendix，不污染 canonical 段 |

---

## 7. Acceptance Criteria（~44 AC 全表）

### 7.1 AC 分类

| 类别 | AC 数 | 子 Change |
|---|---|---|
| **清理 AC**（AC-Clean） | 15 | #0 |
| **拆分 AC**（AC-Split） | 5 | #1 |
| **Registry AC**（AC-Reg） | 15 | #2 |
| **ValueFlow AC**（AC-VF） | 4 | #3 |
| **Span AC**（AC-Span） | 8 | #4 |
| **Boundary AC**（AC-Bound） | 4 | #5 |
| **总体 AC**（AC-Total） | 4 | 全部 |
| **总计** | **~44 AC** | - |

### 7.2 总体 AC（AC-Total）

| AC | 验收方式 | 必过 |
|---|---|---|
| AC-T1 | 22+/22+ D2 packages -race PASS | ✅ |
| AC-T2 | verify-archive.sh 12/12 PASS | ✅ |
| AC-T3 | t-registry P0 子集 100% PASS | ✅ |
| AC-T4 | 8 PR 全部 squash merge + auto-merge | ✅ |

### 7.3 清理 AC（AC-Clean，15 项，子 Change #0）

| AC | 验收方式 | 必过 |
|---|---|---|
| **AC-Clean1** | `test ! -d internal/layers/contextengine/legacy` | ✅ |
| **AC-Clean2** | `test ! -f internal/layers/contextengine/aliases.go` | ✅ |
| **AC-Clean3** | `grep -rE "legacy\.(Process|ContextEngine|EngineDeps|NewContextEngine)" --include="*.go" | grep -v "/legacy/"` → 0 命中 | ✅ |
| **AC-Clean4-AC-Clean13** | 13 test 文件 import path 改完；`go test ./internal/layers/contextengine/... -race` PASS | ✅ |
| **AC-Clean14** | `cmd/devrix/main.go` + `cmd/obs-verify/main.go` import path 改完；`go build ./cmd/...` PASS | ✅ |
| **AC-Clean15** | `tests/testutil/engine_deps.go` 改完；`go test ./tests/... -race` PASS | ✅ |

### 7.4 拆分 AC（AC-Split，5 项，子 Change #1）

| AC | 验收方式 | 必过 |
|---|---|---|
| **AC-Split1** | `compression_steps.go` + `compression_budget.go` + `assembler.go` + `prompt_assembler_layers.go` + `prompt_dynamic_sections.go` 5 文件 wc -l <800 | ✅ |
| **AC-Split2** | `materialize_prompts.go` + `materialize_compressor.go` 2 文件 wc -l <800 | ✅ |
| **AC-Split3** | `ast_walker.go` + `ast_rules.go` 2 文件 wc -l <800 | ✅ |
| **AC-Split4** | `background_registry.go` + `background_notifications.go` 2 文件 wc -l <800 | ✅ |
| **AC-Split5** | 5 god fn T 全部归属正确（不再单挂一函数）；22+ packages -race PASS | ✅ |

### 7.5 Registry AC（AC-Reg，15 项，子 Change #2）

| AC | 验收方式 | 必过 |
|---|---|---|
| **AC-Reg1-AC-Reg9** | 9 F 路径 100% 正确（grep 验证） | ✅ |
| **AC-Reg10** | t-registry D2-S15-A02-T01..T04 路径加 `prepare/conversation/` 前缀 | ✅ |
| **AC-Reg11** | t-registry D2-S18-A02-T01..T15 路径加 `enforce/tools/` 前缀 | ✅ |
| **AC-Reg12** | t-registry §Historical Appendix 含 5 历史 S 全列 | ✅ |
| **AC-Reg13** | a-registry §DISMANTLED/REMOVED 状态行补全 | ✅ |
| **AC-Reg14** | f-registry 47 → 38 F | ✅ |
| **AC-Reg15** | d2-domain.md aliases.go 描述修正 + d7-boundary.md 同步 | ✅ |

### 7.6 ValueFlow AC（AC-VF，4 项，子 Change #3）

| AC | 验收方式 | 必过 |
|---|---|---|
| **AC-VF1** | d2-domain.md §North Star 4 S 加 ValueFlow Alias 列（加 `D2_` 前缀） | ✅ |
| **AC-VF2** | a-registry ValueFlow Semantic 列完整 | ✅ |
| **AC-VF3** | f-registry ValueFlow Semantic 列完整 | ✅ |
| **AC-VF4** | t-registry ValueFlow Semantic 列完整 + layer-delta.md 同步 | ✅ |

### 7.7 Span AC（AC-Span，8 项，子 Change #4）

| AC | 验收方式 | 必过 |
|---|---|---|
| **AC-Span1** | 6 harness ops 删（`context.harness.*` + `context.system_prompt.build`） | ✅ |
| **AC-Span2** | 3 queryloop ops 删（`query.loop.*`） | ✅ |
| **AC-Span3** | 8 context ops 删（snapshot/system_prompt/longterm/tools/memory 等） | ✅ |
| **AC-Span4** | 2 tool ops 删（`tool.execute.*`） | ✅ |
| **AC-Span5** | 7 task/plan ops 删（已迁 D7 workmodel） | ✅ |
| **AC-Span6** | `names.go` 26 常量同步删除；编译过 | ✅ |
| **AC-Span7** | t-registry 232 T 行 Span Evidence 列填充 94% | ✅ |
| **AC-Span8** | Coverage Guard CI script ≥80% 守门 | ✅ |

### 7.8 Boundary AC（AC-Bound，4 项，子 Change #5）

| AC | 验收方式 | 必过 |
|---|---|---|
| **AC-Bound1** | `nested/flow_report.go` 物理迁 `orchestration/executionflow/bridge/subquery_bridge.go` | ✅ |
| **AC-Bound2** | d2-domain.md §Out of Scope 加 Pending Boundary Decision 列 + DM-018 slice-c 标注 | ✅ |
| **AC-Bound3** | t-registry `summarizer_fixture.go` + `prepared_turn_fixture.go` 显式标 fixture type | ✅ |
| **AC-Bound4** | `internal/layers/contextengine/orchtypes/boundary_decision.go` + test PASS（4 单元测试） | ✅ |

---

## 8. Decision 记录

### Decision 1: legacy/ 策略

| 方案 | 优点 | 缺点 |
|---|---|---|
| A: 立即删除（推荐） | 0 LOC + 0 认知负担 | 未来可能需要（但 0 外部调用者） |
| B: 保留 + 注释 | 向后兼容 | 累计 1298 LOC 永久债 |

**选择:** A
**理由:** 双 Agent grep 验证 0 外部调用者；DSAFT 原则 6 "分阶段终态"——v8.2.0 已 P5 Deprecated 标记 + slog.Warn，未启用则删；git history 完整保留可回溯。
**影响:** 1298 LOC 删除；13 test + 2 cmd + testutil import path 迁移；0 业务逻辑改动。

### Decision 2: god fn 拆分粒度（5 god fn 拆 10 文件）

| 方案 | 优点 | 缺点 |
|---|---|---|
| A: 5 god fn 各自拆 2 文件 | 改动可控 | 10 文件，scope 略大 |
| **B: 5 god fn 各自拆 2 文件（推荐）** | 同 A | - |
| C: 5 god fn 全部留原文件 | 0 改动 | god function 持续累积 |

**选择:** B
**理由:** 每个 god fn 拆 2 文件（主+helper 或 core+register）后均 <800 行；t-registry T 编号归属一致；-race PASS 守门；0 函数签名变化。
**影响:** 5 god fn → 10 文件；5 T 重映射；0 函数签名变化。

### Decision 3: ValueFlow Alias 命名（`D2_` 前缀）

| 方案 | 优点 | 缺点 |
|---|---|---|
| A: 不加前缀 | 简洁 | 与 D7 alias 命名冲突（`Context_Loading_Compression` vs D7 `Multi-Step Task Coordination`） |
| **B: 加 `D2_` 前缀（推荐）** | 与 D7 alias 区分 | 前缀冗余 |
| C: 完全不同命名 | 完全区分 | 用户感知层断裂 |

**选择:** B
**理由:** 对齐 D7 v2.6.0 ValueFlow Alias 模式；D2 alias 加 `D2_` 前缀避免跨域 alias 冲突；用户感知层保留 D2 原语义。
**影响:** d2-domain.md §North Star 4 S 加 ValueFlow Alias 列（加 `D2_` 前缀）。

### Decision 4: 26 dead span ops 删 vs 保留

| 方案 | 优点 | 缺点 |
|---|---|---|
| A: 全部删除 | 0 LOC + 0 认知负担 | 未来可能需要（但 0 production emit） |
| **B: 全部删除（推荐）** | 同 A + git history 保留 | - |
| C: 全部保留 | 向后兼容 | 26 dead ops 永久债 |

**选择:** B
**理由:** 双 Agent grep 验证 0 production emit（harness 6 + queryloop 3 + context 8 + tool 2 + task/plan 7）；t-registry 缺 Span Evidence 列 = 当前覆盖率 ~20%；删后覆盖率 94%。
**影响:** 30 → 4 active span ops；t-registry Span Evidence 列填充 94%。

### Decision 5: DM-018 slice-c 迁移目标

| 方案 | 优点 | 缺点 |
|---|---|---|
| A: 物理迁 `orchestration/executionflow/bridge/subquery_bridge.go` | D2 不再持有 FlowEvent ownership | D2→D7 已有 Hard Ban（D2-THIN-T01） |
| **B: 物理迁 + 新建 `internal/shared/bridge/`（推荐）** | D7→D2 反向 import 通过 shared bridge | 1 新建目录 |
| C: 不迁，标 Decision | 0 改动 | 跨域债遗留 |

**选择:** B
**理由:** DM-018 slice-c 是 D2-S19 DISMANTLED 残留；D7-S4 ExecutionFlow + Verify 已 14 ExitReason 落地；D2 不应再持有 FlowEvent ownership；新建 `internal/shared/bridge/` 打破反向 import cycle。
**影响:** `nested/flow_report.go` 物理迁；d2-domain.md §Out of Scope 加 Pending Boundary Decision 列；boundary_decision.go 治理常量 `BoundaryDM018SliceC`。

### Decision 6: 跨域 fixture 处理

| 方案 | 优点 | 缺点 |
|---|---|---|
| A: 物理迁 D7 | 严格跨域分离 | 5 文件 import path 迁移 + cycle 风险 |
| **B: 物理留 D2 根 + t-registry 标 fixture type（推荐）** | 无 import cycle 风险 | fixture 跨域标记 |
| C: 不迁，标 Decision | 0 改动 | 跨域 fixture type 未明示 |

**选择:** B
**理由:** `summarizer_fixture.go` + `prepared_turn_fixture.go` 在 D2 根无 import cycle 风险（仅 D2 内 import）；t-registry 显式标 fixture type 给未来跨域 fixture 重新分配留决策空间。
**影响:** 0 代码改动；t-registry 加 fixture type 列。

---

## 9. 相关文档

- `demand.md` — 背景 + 6 类架构债 + 范围
- `tasks.md` — 44 任务清单（按 6 子 Change 拆分）
- `design.md` — S→A→F 逐层详细设计 + §Boundary Decision
- `specs/d2-context-engine/spec.md` — ~44 AC 全表 + Gherkin
- `docs/methodology/dsaft-methodology.md` v4.0.0
- `docs/methodology/dsaft-refactoring-playbook.md` v1.0.0
- `openspec/specs/d2-context-engine/d2-domain.md` v8.2.0 → v9.0.0
- `openspec/specs/d7-orchestration/d7-domain.md` v2.6.0 — ValueFlow Alias 模式 SoT
- `openspec/archive/2026-06-29-devrix-d7-dsaft-restructuring/` — DM-20260629-001 S7_Archive 模板