# Tasks: devrix-d2-dsaft-restructuring (DM-20260629-002)

**Change ID:** `devrix-d2-dsaft-restructuring`
**Demand ID:** DM-20260629-002
**Status:** S2_Tasks
**Total Tasks:** 44
**Total AC:** 44
**Template:** `devrix-d7-dsaft-restructuring` tasks.md（DM-20260629-001）

---

## §0 任务索引（44 T / 6 子 Change）

| 子 Change | PR | T 范围 | 工作量 |
|---|---|---|---|
| **#0** legacy-cleanup | PR-1 | T01-T15 | 1 PR / 1 天 |
| **#1** god-fn-split pt1 | PR-2 | T16-T20 | 1 PR / 1 天 |
| **#1** god-fn-split pt2 | PR-3 | T21-T25 | 1 PR / 1 天 |
| **#2** registry-sync | PR-4 | T26-T35 | 1 PR / 1 天 |
| **#3** value-flow-rename | PR-5 | T36-T39 | 1 PR / 1 天 |
| **#4** span-coverage pt1 | PR-6 | T40-T43 | 1 PR / 1 天 |
| **#4** span-coverage pt2 | PR-7 | T44 | 1 PR / 1 天 |
| **#5** boundary-decision | PR-8 | T45-T47 | 1 PR / 1 天 |
| **S7_Archive** | — | T48-T50 | 1 PR / 1 天 |
| **总计** | 8 PR + 1 S7 | **44 T** | **7-9 天** |

---

## §1 子 Change #0 legacy-cleanup（PR-1, T01-T15）

### T01 删 `legacy/engine.go` (484 LOC)

- 删除文件 `internal/layers/contextengine/legacy/engine.go`
- 验证 grep 0 命中：`grep -rE "legacy\.Process" --include="*.go" | grep -v "/legacy/"`
- AC: AC-Clean1 + AC-Clean3

### T02 删 `legacy/engine_persist_v2.go` (220 LOC)

- 删除文件 `internal/layers/contextengine/legacy/engine_persist_v2.go`
- 验证 grep 0 命中：`grep -rE "legacy\.persistTurn" --include="*.go" | grep -v "/legacy/"`
- AC: AC-Clean1

### T03 删 `legacy/engine_builder.go` (133 LOC)

- 删除文件 `internal/layers/contextengine/legacy/engine_builder.go`
- 验证 grep 0 命中：`grep -rE "legacy\.NewContextEngine" --include="*.go" | grep -v "/legacy/"`
- AC: AC-Clean1 + AC-Clean3

### T04 删 `legacy/engine_types.go` (104 LOC)

- 删除文件 `internal/layers/contextengine/legacy/engine_types.go`
- 验证 grep 0 命中：`grep -rE "legacy\.EngineDeps" --include="*.go" | grep -v "/legacy/"`
- AC: AC-Clean1 + AC-Clean3

### T05 删 `legacy/persist_adapters.go` (80 LOC)

- 删除文件 `internal/layers/contextengine/legacy/persist_adapters.go`
- 验证 grep 0 命中
- AC: AC-Clean1

### T06 删 `legacy/engine_events.go` (70 LOC)

- 删除文件 `internal/layers/contextengine/legacy/engine_events.go`
- 验证 grep 0 命中
- AC: AC-Clean1

### T07 删 `legacy/engine_compression.go` (61 LOC)

- 删除文件 `internal/layers/contextengine/legacy/engine_compression.go`
- 验证 grep 0 命中
- AC: AC-Clean1

### T08 删 `legacy/commit_window_adapter.go` (43 LOC)

- 删除文件 `internal/layers/contextengine/legacy/commit_window_adapter.go`
- 验证 grep 0 命中
- AC: AC-Clean1

### T09 删 `legacy/engine_export.go` (46 LOC)

- 删除文件 `internal/layers/contextengine/legacy/engine_export.go`
- 验证 grep 0 命中
- AC: AC-Clean1

### T10 删 `legacy/prepared_turn_result.go` (14 LOC) + `legacy/prepared_turn_wire.go` (19 LOC)

- 删除文件 `internal/layers/contextengine/legacy/prepared_turn_*.go` (2 文件)
- 验证 grep 0 命中
- AC: AC-Clean1

### T11 删 `aliases.go` (24 LOC)

- 删除文件 `internal/layers/contextengine/aliases.go`
- 验证 grep 0 命中：`grep -rE "contextengine\.(Process|EngineDeps|NewContextEngine|ContextEngine)" --include="*.go" | grep -v "_test.go"` 应仅 main.go / testutil
- AC: AC-Clean2 + AC-Clean3

### T12 改 13 test 文件 import path

- 文件清单：
  - `engine_persist_bridge_test.go`
  - `engine_persist_v2_test.go` (如果存在)
  - `engine_types_test.go` (如果存在)
  - `engine_export_test.go` (如果存在)
  - `engine_compression_test.go` (如果存在)
  - `engine_events_test.go` (如果存在)
  - `commit_window_adapter_test.go` (如果存在)
  - `engine_test.go` (如果存在)
  - `engine_builder_test.go` (如果存在)
  - `persist_adapters_test.go` (如果存在)
  - `prepared_turn_result_test.go` (如果存在)
  - `prepared_turn_wire_test.go` (如果存在)
  - `aliases_test.go` (如果存在)
- 替换规则：
  - `contextengine.Process` → `kernel.ContextEngine`
  - `contextengine.EngineDeps` → `kernel.EngineDeps`
  - `contextengine.NewContextEngine` → `kernel.NewContextEngine`
- AC: AC-Clean4..AC-Clean13

### T13 改 `cmd/devrix/main.go` + `cmd/obs-verify/main.go` import path

- 文件清单：
  - `cmd/devrix/main.go`
  - `cmd/obs-verify/main.go`
- 替换规则：同 T12
- AC: AC-Clean14

### T14 改 `tests/testutil/engine_deps.go` 实现

- 文件：`tests/testutil/engine_deps.go`
- 替换规则：直接调 `kernel.NewContextEngine(...)` 替代 `legacy.NewContextEngine(...)`
- AC: AC-Clean15

### T15 PR-1 集成验证

- `go build ./cmd/devrix/... ./cmd/obs-verify/...`
- `go test ./internal/layers/contextengine/... -race -count=1`
- `test ! -d internal/layers/contextengine/legacy`
- `test ! -f internal/layers/contextengine/aliases.go`
- AC: AC-T1 + AC-Clean1..AC-Clean15

---

## §2 子 Change #1 god-fn-split pt1（PR-2, T16-T20）

### T16 拆 `compression/pipeline.go::RunForSession()` (109 LOC)

- 新建 `compression/compression_steps.go`（7 step helper：deduplicate/snip/fold/expire/maybeAutocompact/assemble/tokenBlock）
- 新建 `compression/compression_budget.go`（token validation + budget validation）
- `pipeline.go` 主体 <300 行
- AC: AC-Split1

### T17 拆 `prepare/prompt/assembler.go::Build()` (55 LOC)

- 新建 `prepare/prompt/prompt_assembler_layers.go`（buildCoreLayer + buildLayer3）
- 新建 `prepare/prompt/prompt_dynamic_sections.go`（buildDynamicSections）
- `assembler.go` 主体 <300 行
- AC: AC-Split1

### T18 t-registry D2-S15-A03 T 重映射

- 更新 t-registry `D2-S15-A03-T01..T04` 路径加 `compression_steps.go` + `compression_budget.go`
- AC: AC-Split5

### T19 t-registry D2-S15-A04 T 重映射

- 更新 t-registry `D2-S15-A04-T01..T03` 路径加 `prompt_assembler_layers.go` + `prompt_dynamic_sections.go`
- AC: AC-Split5

### T20 PR-2 集成验证

- `go test ./internal/layers/contextengine/prepare/compression/... ./prepare/prompt/... -race -count=1`
- 验证每个新文件 wc -l <800
- AC: AC-T1 + AC-Split1 + AC-Split5

---

## §3 子 Change #1 god-fn-split pt2（PR-3, T21-T25）

### T21 拆 `materialize/materializer.go::Materialize()` (30 LOC)

- 新建 `materialize/materialize_prompts.go`（buildSystemPrompt + buildWaveSystemPrompt）
- 新建 `materialize/materialize_compressor.go`（compressMessages）
- `materializer.go` 主体 <200 行
- AC: AC-Split2

### T22 拆 `enforce/tools/sandboxast/analyzer.go::Analyze()` (39 LOC)

- 新建 `enforce/tools/sandboxast/ast_walker.go`（walk function）
- 新建 `enforce/tools/sandboxast/ast_rules.go`（rule registry）
- `analyzer.go` 主体 <200 行
- AC: AC-Split3

### T23 拆 `enforce/background.go::RunBackground()` (26 LOC)

- 新建 `enforce/background_registry.go`（CRUD）
- 新建 `enforce/background_notifications.go`（queue integration）
- `background.go` 主体 <200 行
- AC: AC-Split4

### T24 t-registry S16/S18 god fn T 重映射

- 更新 t-registry `D2-S16-A20-T01..T03` + `D2-S18-A08-T01..T04` + `D2-S18-A09-T01..T02` 路径
- AC: AC-Split5

### T25 PR-3 集成验证

- `go test ./internal/layers/contextengine/materialize/... ./enforce/tools/sandboxast/... ./enforce/... -race -count=1`
- 验证每个新文件 wc -l <800
- AC: AC-T1 + AC-Split2 + AC-Split3 + AC-Split4 + AC-Split5

---

## §4 子 Change #2 registry-sync（PR-4, T26-T35）

### T26 f-registry D2-S15-A02 路径修正

- 路径 `prepare/memory/longterm.go` → `prepare/memory/recall.go` + `persist/memory/store.go`
- AC: AC-Reg1

### T27 f-registry D2-S15-A03 路径修正

- 路径加 `compression/compression_steps.go` + `compression/compression_budget.go`
- AC: AC-Reg2

### T28 f-registry D2-S15-A04 路径修正

- 路径加 `prepare/prompt/prompt_assembler_layers.go` + `prepare/prompt/prompt_dynamic_sections.go`
- AC: AC-Reg3

### T29 f-registry D2-S17-A02 路径修正

- 路径加 `persist/commit_window/` 子目录展开
- AC: AC-Reg4

### T30 f-registry D2-S18-A01 + D2-S18-A02 路径修正

- 路径加 `permission/` + `enforce/tools/{surface,sandboxast,builtin}/` 子目录展开
- AC: AC-Reg5 + AC-Reg6

### T31 f-registry D2-S16-A20 路径修正

- 路径加 `materialize/{prompts,compressor}.go` 子文件展开
- AC: AC-Reg7

### T32 t-registry 子目录前缀

- `D2-S15-A02-T01..T04` 路径加 `prepare/conversation/` 前缀
- `D2-S18-A02-T01..T15` 路径加 `enforce/tools/` 前缀
- AC: AC-Reg8 + AC-Reg9

### T33 t-registry §Historical Appendix

- 移动 D2-S1/S9/S10/S19/S20 到独立 §Historical Appendix 段
- AC: AC-Reg10

### T34 a-registry §DISMANTLED/REMOVED 状态行补全 + f-registry 47 → 38 F

- a-registry §DISMANTLED/REMOVED 状态行补全
- f-registry 47 → 38 F（历史 F 入 Historical appendix）
- AC: AC-Reg11 + AC-Reg12

### T35 PR-4 集成验证

- `grep -rE "D2-S15-A02.*longterm" openspec/specs/d2-context-engine/f-registry.md` → 0 命中
- `grep -rE "D2-S18-A02.*enforce/tools" openspec/specs/d2-context-engine/t-registry.md` ≥ 15 命中
- AC: AC-T1 + AC-Reg1..AC-Reg15

---

## §5 子 Change #3 value-flow-rename（PR-5, T36-T39）

### T36 d2-domain.md §North Star 4 S ValueFlow Alias

- S15 PrepareExecutionContext → **D2_Context_Loading_Compression**
- S16 Materialize Context → **D2_LLM_Ready_Assembly**
- S17 PersistSessionState → **D2_Session_State_Persistence**
- S18 EnforceExecutionPolicy → **D2_Tool_Permission_Sandbox**
- AC: AC-VF1

### T37 a-registry ValueFlow Semantic 列

- a-registry 加 ValueFlow Semantic 列完整
- AC: AC-VF2

### T38 f-registry ValueFlow Semantic 列

- f-registry 加 ValueFlow Semantic 列完整
- AC: AC-VF3

### T39 t-registry ValueFlow Semantic 列 + layer-delta.md 同步

- t-registry 加 ValueFlow Semantic 列完整
- layer-delta.md §Canonical 4 S 加别名
- AC: AC-VF4

---

## §6 子 Change #4 span-coverage pt1（PR-6, T40-T43）

### T40 删 26 dead span ops + names.go 同步

- 删 6 harness ops + 3 queryloop ops + 8 context ops + 2 tool ops + 7 task/plan ops
- `internal/layers/observability/instrument/telemetry/names.go` 26 常量同步删除
- AC: AC-Span1..AC-Span6

### T41 coverage integration test 同步更新

- `internal/layers/observability/integration/coverage/*_test.go` 同步删除 26 引用
- AC: AC-Span6

### T42 observability-guide.md §T-Without-Span Tracker

- observability-guide.md 新增 §T-Without-Span Tracker 段
- 列出 12 个未映射 T
- AC: AC-Span7

### T43 PR-6 集成验证

- `grep -c "context.harness\|query.loop\|context.snapshot\|context.system_prompt\|context.longterm\|context.tools\|context.memory\|context.token\|context.fork\|tool.execute\|task.plan\|task.plan_mode\|task.manager" internal/layers/observability/instrument/telemetry/names.go` → 0 命中
- AC: AC-T1 + AC-Span1..AC-Span7

---

## §7 子 Change #4 span-coverage pt2（PR-7, T44）

### T44 t-registry Span Evidence 列填充 + Coverage Guard

- t-registry 232 T 行加 Span Evidence 列
- active ops 映射到 T 行（220 T → 4 active ops）
- dead/未映射 T 标 `—`（12 T）
- `scripts/d2-span-coverage.sh` ≥80% 守门
- AC: AC-Span7 + AC-Span8

---

## §8 子 Change #5 boundary-decision（PR-8, T45-T47）

### T45 DM-018 slice-c 物理迁移

- `internal/layers/contextengine/nested/flow_report.go` 物理迁 `internal/layers/orchestration/executionflow/bridge/subquery_bridge.go`
- 新建 `internal/shared/bridge/` 打破反向 import cycle
- AC: AC-Bound1

### T46 d2-domain.md §Out of Scope + d7-boundary.md

- d2-domain.md §Out of Scope 加 Pending Boundary Decision 列 + DM-018 slice-c 标注
- d7-boundary.md 同步
- AC: AC-Bound2

### T47 orchtypes/boundary_decision.go + fixture type 标

- 新建 `internal/layers/contextengine/orchtypes/boundary_decision.go` + test（4 单元测试）
- t-registry `summarizer_fixture.go` + `prepared_turn_fixture.go` 显式标 fixture type
- AC: AC-Bound3 + AC-Bound4

---

## §9 S7_Archive（T48-T50）

### T48 S7_Archive 6 artifacts + verify-archive 12/12

- 6 artifacts: demand.md / proposal.md / design.md / tasks.md / spec.md / acceptance-report.md
- `verify-archive.sh devrix-d2-dsaft-restructuring` → 12/12 PASS
- AC: AC-T2

### T49 spec v8.2.0 → v9.0.0

- d2-domain.md Version 8.2.0 → **9.0.0**
- d2-domain.md Last Updated 2026-06-29
- d2-domain.md 修订记录新增 v9.0.0 row
- AC: AC-T1

### T50 demand-archive-index.md 更新

- DM-20260629-002 row 加 Verdict S7_Archived + PR 链接
- AC: AC-T2

---

## §10 验证总览

### 10.1 每 PR 后 -race PASS 守门

```bash
go test ./internal/layers/contextengine/... -race -count=1
```

期望：22+/22+ packages PASS（每 PR 后守门）。

### 10.2 端到端验证（PR-1..PR-8 + S7_Archive）

```bash
# 1. 全量 Go 编译
go build ./cmd/devrix/... ./cmd/obs-verify/...

# 2. D2 域全量 -race
go test ./internal/layers/contextengine/... -race -count=1

# 3. Span Evidence 覆盖率
./scripts/d2-span-coverage.sh
# 期望：94% (220/232 T 行映射)

# 4. 跨域 import 门禁
go test ./internal/lint/layer/... -v
# 期望：D2→D3 ban 0 命中；D2→D7 0 命中

# 5. legacy/ 目录清理验证
test ! -d internal/layers/contextengine/legacy
test ! -f internal/layers/contextengine/aliases.go

# 6. verify-archive
./scripts/verify-archive.sh devrix-d2-dsaft-restructuring
# 期望：12/12 PASS

# 7. 飞书 E2E 飞书实测
# - 启动 devrix (./scripts/devrix.sh restart)
# - 发送消息验证 Prepare/Enforce/Persist 链路
# - Jaeger 验证 4 active span ops emit
```

### 10.3 期望 AC 全过

| AC 类别 | AC 数 | 期望 |
|---|---|---|
| AC-Clean | 15 | 15/15 PASS |
| AC-Split | 5 | 5/5 PASS |
| AC-Reg | 15 | 15/15 PASS |
| AC-VF | 4 | 4/4 PASS |
| AC-Span | 8 | 8/8 PASS |
| AC-Bound | 4 | 4/4 PASS |
| AC-Total | 4 | 4/4 PASS |
| **总计** | **44 AC** | **44/44 PASS** |

---

## §11 PR 落地序列

| Day | PR | T 范围 | 验收 |
|---|---|---|---|
| 1 | PR-1 #0 legacy-cleanup | T01-T15 (1298 LOC 删 + 13 test 迁移) | D2 全量 -race PASS |
| 2 | PR-2 #1 god-fn pt1 | T16-T20 (pipeline + assembler) | 文件 <800 行 |
| 3 | PR-3 #1 god-fn pt2 | T21-T25 (materializer + analyzer + background) | 文件 <800 行 |
| 4 | PR-4 #2 registry-sync | T26-T35 (9 F path + Historical appendix) | t-registry 对齐 |
| 5 | PR-5 #3 value-flow-rename | T36-T39 (4 S + 用户感知层) | d2-domain v8.2.0 |
| 6 | PR-6 #4 span pt1 | T40-T43 (26 dead ops 删) | telemetry 编译过 |
| 7 | PR-7 #4 span pt2 | T44 (232 T 行 Span Evidence 94%) | coverage check ≥80% |
| 8 | PR-8 #5 boundary-decision | T45-T47 (DM-018 slice-c + boundary 治理) | 4 单元测试 PASS |
| 9 | S7_Archive | T48-T50 (6 artifacts + verify-archive 12/12) | spec v8.2.0 → v9.0.0 |