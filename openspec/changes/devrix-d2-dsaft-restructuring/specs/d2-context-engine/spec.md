# Spec: devrix-d2-dsaft-restructuring (DM-20260629-002)

**Change ID:** `devrix-d2-dsaft-restructuring`
**Demand ID:** DM-20260629-002
**Status:** S3_Spec
**Total AC:** 44
**Template:** `devrix-d7-dsaft-restructuring` spec.md（DM-20260629-001）

---

## §1 Overview

D2 v8.2.0 → v9.0.0 6 子 Change 联动 refactoring。S→A→F 逐层 + 清理 + 跨域。详见 `demand.md` + `proposal.md` + `design.md` + `tasks.md`。

---

## §2 ADDED Requirements（6 子 Change）

### Requirement: Legacy Cleanup（DM-20260629-002 子 Change #0）

D2 MUST 立即删除 P5 Deprecated legacy/ 目录 11 文件（1274 LOC）+ aliases.go（24 LOC），迁移 13 test + 2 cmd + testutil 到 `kernel.ContextEngine` / `prepare.PrepareOrchestrator` / `persist.PersistOrchestrator` 直接 import。

**Priority**: P0
**Rationale**: 双 Agent grep 验证 0 外部调用者；DSAFT 原则 6 "分阶段终态"；v8.2.0 已 P5 Deprecated 标记 + slog.Warn
**L3 映射**: L3-BE-CTX-01..02
**T 映射**: T01-T15

#### Scenario: legacy/ 目录不存在

- **WHEN** PR-1 完成
- **THEN** `test ! -d internal/layers/contextengine/legacy` 0 命中
- **AND** `test ! -f internal/layers/contextengine/aliases.go` 0 命中

#### Scenario: 13 test 文件 import path 改完

- **WHEN** PR-1 完成
- **THEN** 13 test 文件全部改完
- **AND** `go test ./internal/layers/contextengine/... -race -count=1` PASS

#### Scenario: 2 cmd import path 改完

- **WHEN** PR-1 完成
- **THEN** `cmd/devrix/main.go` + `cmd/obs-verify/main.go` 改完
- **AND** `go build ./cmd/devrix/... ./cmd/obs-verify/...` PASS

#### Scenario: tests/testutil/engine_deps.go 改完

- **WHEN** PR-1 完成
- **THEN** `tests/testutil/engine_deps.go` 直接调 `kernel.NewContextEngine(...)`
- **AND** `go test ./tests/... -race` PASS

### Requirement: God Function Split（DM-20260629-002 子 Change #1）

D2 MUST 拆 5 god fn（pipeline.go 109 + assembler.go 55 + materializer.go 30 + analyzer.go 39 + background.go 26 LOC）为 10 文件，每个新文件 <800 行。

**Priority**: P0
**Rationale**: 跨 3 个 S 的 god function 累积；按"职责子模块"拆分；0 函数签名变化
**L3 映射**: L3-BE-CTX-01..02
**T 映射**: T16-T25

#### Scenario: pipeline.go::RunForSession 拆 3 文件

- **WHEN** PR-2 完成
- **THEN** 新建 `compression/compression_steps.go`（7 step helper）+ `compression/compression_budget.go`（token validation）
- **AND** `pipeline.go` 主体 <300 行

#### Scenario: assembler.go::Build 拆 3 文件

- **WHEN** PR-2 完成
- **THEN** 新建 `prepare/prompt/prompt_assembler_layers.go` + `prompt_dynamic_sections.go`
- **AND** `assembler.go` 主体 <300 行

#### Scenario: materializer.go + analyzer.go + background.go 拆 6 文件

- **WHEN** PR-3 完成
- **THEN** 新建 `materialize_prompts.go` + `materialize_compressor.go` + `ast_walker.go` + `ast_rules.go` + `background_registry.go` + `background_notifications.go`
- **AND** 5 god fn 主体文件均 <300 行

#### Scenario: 5 god fn T 全部归属正确

- **WHEN** PR-3 完成
- **THEN** t-registry `D2-S15-A03-T01..T04` + `D2-S15-A04-T01..T03` + `D2-S16-A20-T01..T03` + `D2-S18-A08-T01..T04` + `D2-S18-A09-T01..T02` 路径全部加新文件后缀

### Requirement: Registry Sync（DM-20260629-002 子 Change #2）

D2 MUST 修正 9 F 路径 + t-registry 子目录前缀 + Historical appendix 5 历史 S 全列。

**Priority**: P0
**Rationale**: v8.2.0 P4 split 后 f-registry 路径全漂移；S1/S9/S10/S19/S20 历史债需独立段
**L3 映射**: L3-BE-CTX-01..02
**T 映射**: T26-T35

#### Scenario: 9 F 路径 100% 正确

- **WHEN** PR-4 完成
- **THEN** f-registry 9 F 路径全部修正（D2-S15-A02/A03/A04 + D2-S16-A20 + D2-S17-A02 + D2-S18-A01/A02 + t-registry 子目录前缀）
- **AND** grep 验证 0 漂移

#### Scenario: t-registry §Historical Appendix 完整

- **WHEN** PR-4 完成
- **THEN** t-registry §Historical Appendix 含 D2-S1/S9/S10/S19/S20 5 历史 S
- **AND** a-registry §DISMANTLED/REMOVED 状态行补全

#### Scenario: f-registry 47 → 38 F

- **WHEN** PR-4 完成
- **THEN** f-registry 头部 47 → 38 F（9 历史 F 入 Historical appendix）

### Requirement: ValueFlow Alias（DM-20260629-002 子 Change #3）

D2 MUST 给 4 canonical S 配 ValueFlow Alias（加 `D2_` 前缀避免与 D7 冲突）。

**Priority**: P0
**Rationale**: 对齐 D7 v2.6.0 §North Star 6 行 ValueFlow Alias 模式；DSAFT 原则 1
**L3 映射**: L3-BE-CTX-01..02
**T 映射**: T36-T39

#### Scenario: 4 S ValueFlow Alias 全配

- **WHEN** PR-5 完成
- **THEN** d2-domain.md §North Star 4 S 加 ValueFlow Alias 列
  - S15 → **D2_Context_Loading_Compression**
  - S16 → **D2_LLM_Ready_Assembly**
  - S17 → **D2_Session_State_Persistence**
  - S18 → **D2_Tool_Permission_Sandbox**

#### Scenario: a/f/t-registry ValueFlow Semantic 列完整

- **WHEN** PR-5 完成
- **THEN** a/f/t-registry 加 ValueFlow Semantic 列完整

#### Scenario: layer-delta.md §Canonical 4 S 加别名

- **WHEN** PR-5 完成
- **THEN** layer-delta.md §Canonical 4 S 加别名同步

### Requirement: Span Coverage（DM-20260629-002 子 Change #4）

D2 MUST 删 26 dead span ops + t-registry Span Evidence 列填充 94% 覆盖率。

**Priority**: P0
**Rationale**: 当前 30 ops 中 26 dead（~20% 覆盖率）；对齐 D7 94% 实际达成
**L3 映射**: L3-BE-CTX-01..02
**T 映射**: T40-T44

#### Scenario: 26 dead ops 删完

- **WHEN** PR-6 完成
- **THEN** 6 harness + 3 queryloop + 8 context + 2 tool + 7 task/plan = 26 dead ops 全删
- **AND** `internal/layers/observability/instrument/telemetry/names.go` 26 常量同步删除

#### Scenario: 4 active ops 保留

- **WHEN** PR-6 完成
- **THEN** 保留 `context.process` + `context.compression.run` + `context.compression.step` + `context.materialize` 4 active ops
- **AND** coverage integration test 同步更新

#### Scenario: t-registry Span Evidence 列填充 94%

- **WHEN** PR-7 完成
- **THEN** t-registry 232 T 行 Span Evidence 列填充
- **AND** 220 T 映射到 4 active ops
- **AND** 12 T 标 `—`（compile-time check / internal helper）

#### Scenario: Coverage Guard CI script ≥80% 守门

- **WHEN** PR-7 完成
- **THEN** `scripts/d2-span-coverage.sh` ≥80% 守门
- **AND** `verify-archive.sh` 第 13 项 PASS（期望 12/12 → 13/13）

### Requirement: Boundary Decision（DM-20260629-002 子 Change #5）

D2 MUST 物理迁移 DM-018 slice-c + boundary_decision.go 治理常量 + fixture type 标。

**Priority**: P0
**Rationale**: D2-S19 DISMANTLED 残留 `nested/flow_report.go` 应迁 D7-S4 ExecutionFlow + Verify；D2 不应持有 FlowEvent ownership
**L3 映射**: L3-BE-CTX-01..02
**T 映射**: T45-T47

#### Scenario: DM-018 slice-c 物理迁移

- **WHEN** PR-8 完成
- **THEN** `internal/layers/contextengine/nested/flow_report.go` 物理迁 `internal/layers/orchestration/executionflow/bridge/subquery_bridge.go`
- **AND** 新建 `internal/shared/bridge/` 打破反向 import cycle

#### Scenario: d2-domain.md §Out of Scope 加 Pending Boundary Decision

- **WHEN** PR-8 完成
- **THEN** d2-domain.md §Out of Scope 加 Pending Boundary Decision 列
- **AND** DM-018 slice-c 标注 `boundary-debt:dm-018-slice-c-v7.0`

#### Scenario: orchtypes/boundary_decision.go + 4 单元测试

- **WHEN** PR-8 完成
- **THEN** 新建 `internal/layers/contextengine/orchtypes/boundary_decision.go`
- **AND** 4 单元测试 PASS

#### Scenario: t-registry fixture type 标

- **WHEN** PR-8 完成
- **THEN** t-registry `summarizer_fixture.go` + `prepared_turn_fixture.go` 显式标 `fixture:cross-domain`

---

## §3 MODIFIED Requirements

### Requirement: D2-S15 PrepareExecutionContext（v9.0.0）

D2-S15 MUST 包含 8 A（god fn 拆分后 4 A → 8 A），ValueFlow Alias 为 **D2_Context_Loading_Compression**。

**Priority**: P0
**L4 映射**: L4-CTX-PREPARE

#### Scenario: S15 4 → 8 A 拆分

- **WHEN** v9.0.0 落地
- **THEN** a-registry D2-S15-A01..A08（4 旧 A + 4 新 A：compression_steps / compression_budget / prompt_assembler_layers / prompt_dynamic_sections）
- **AND** 每个 A 均有 t-registry T 编号归属

### Requirement: D2-S18 EnforceExecutionPolicy（v9.0.0）

D2-S18 MUST 包含 9 A（god fn 拆分后 7 A → 9 A），ValueFlow Alias 为 **D2_Tool_Permission_Sandbox**。

**Priority**: P0
**L4 映射**: L4-CTX-ENFORCE

#### Scenario: S18 7 → 9 A 拆分

- **WHEN** v9.0.0 落地
- **THEN** a-registry D2-S18-A01..A09（7 旧 A + 2 新 A：ast_walker / background_registry）
- **AND** 每个 A 均有 t-registry T 编号归属

---

## §4 REMOVED Requirements

### Requirement: legacy/ + aliases.go（DM-20260629-002 子 Change #0）

`internal/layers/contextengine/legacy/` 目录 11 文件 + `aliases.go` 24 LOC 必须删除；所有调用方迁移到 `kernel.ContextEngine` / `prepare.PrepareOrchestrator` / `persist.PersistOrchestrator` 直接 import。

**Status:** REMOVED (DM-20260629-002 PR-1)

### Requirement: 26 dead span ops（DM-20260629-002 子 Change #4）

6 harness + 3 queryloop + 8 context + 2 tool + 7 task/plan = 26 dead span ops 必须删除；保留 4 active ops（`context.process` + `context.compression.run` + `context.compression.step` + `context.materialize`）。

**Status:** REMOVED (DM-20260629-002 PR-6)

---

## §5 ACCEPTANCE CRITERIA（44 AC 全表）

### 5.1 清理 AC（AC-Clean，15 项，子 Change #0）

| AC | 验收方式 | 必过 |
|---|---|---|
| AC-Clean1 | `test ! -d internal/layers/contextengine/legacy` | ✅ |
| AC-Clean2 | `test ! -f internal/layers/contextengine/aliases.go` | ✅ |
| AC-Clean3 | `grep -rE "legacy\.(Process|ContextEngine|EngineDeps|NewContextEngine)" --include="*.go" \| grep -v "/legacy/"` → 0 命中 | ✅ |
| AC-Clean4 | engine_persist_bridge_test.go import path 改完 | ✅ |
| AC-Clean5 | engine_persist_v2_test.go import path 改完（如果存在） | ✅ |
| AC-Clean6 | engine_types_test.go import path 改完（如果存在） | ✅ |
| AC-Clean7 | engine_export_test.go import path 改完（如果存在） | ✅ |
| AC-Clean8 | engine_compression_test.go import path 改完（如果存在） | ✅ |
| AC-Clean9 | engine_events_test.go import path 改完（如果存在） | ✅ |
| AC-Clean10 | commit_window_adapter_test.go import path 改完（如果存在） | ✅ |
| AC-Clean11 | engine_test.go import path 改完（如果存在） | ✅ |
| AC-Clean12 | engine_builder_test.go import path 改完（如果存在） | ✅ |
| AC-Clean13 | persist_adapters_test.go + prepared_turn_*_test.go + aliases_test.go import path 改完 | ✅ |
| AC-Clean14 | cmd/devrix/main.go + cmd/obs-verify/main.go import path 改完 | ✅ |
| AC-Clean15 | tests/testutil/engine_deps.go 改完 | ✅ |

### 5.2 拆分 AC（AC-Split，5 项，子 Change #1）

| AC | 验收方式 | 必过 |
|---|---|---|
| AC-Split1 | compression_steps.go + compression_budget.go + prompt_assembler_layers.go + prompt_dynamic_sections.go 4 文件 wc -l <800 | ✅ |
| AC-Split2 | materialize_prompts.go + materialize_compressor.go 2 文件 wc -l <800 | ✅ |
| AC-Split3 | ast_walker.go + ast_rules.go 2 文件 wc -l <800 | ✅ |
| AC-Split4 | background_registry.go + background_notifications.go 2 文件 wc -l <800 | ✅ |
| AC-Split5 | 5 god fn T 全部归属正确（不再单挂一函数）；22+ packages -race PASS | ✅ |

### 5.3 Registry AC（AC-Reg，15 项，子 Change #2）

| AC | 验收方式 | 必过 |
|---|---|---|
| AC-Reg1 | f-registry D2-S15-A02 路径修正 | ✅ |
| AC-Reg2 | f-registry D2-S15-A03 路径修正 | ✅ |
| AC-Reg3 | f-registry D2-S15-A04 路径修正 | ✅ |
| AC-Reg4 | f-registry D2-S17-A02 路径修正 | ✅ |
| AC-Reg5 | f-registry D2-S18-A01 路径修正 | ✅ |
| AC-Reg6 | f-registry D2-S18-A02 路径修正 | ✅ |
| AC-Reg7 | f-registry D2-S16-A20 路径修正 | ✅ |
| AC-Reg8 | t-registry D2-S15-A02-T01..T04 路径加 `prepare/conversation/` 前缀 | ✅ |
| AC-Reg9 | t-registry D2-S18-A02-T01..T15 路径加 `enforce/tools/` 前缀 | ✅ |
| AC-Reg10 | t-registry §Historical Appendix 含 5 历史 S 全列 | ✅ |
| AC-Reg11 | a-registry §DISMANTLED/REMOVED 状态行补全 | ✅ |
| AC-Reg12 | f-registry 47 → 38 F | ✅ |
| AC-Reg13 | d2-domain.md aliases.go 描述修正 | ✅ |
| AC-Reg14 | d2-domain.md §物理路径映射表 11 legacy/ 文件名全列 | ✅ |
| AC-Reg15 | d7-boundary.md D2 边界描述同步 | ✅ |

### 5.4 ValueFlow AC（AC-VF，4 项，子 Change #3）

| AC | 验收方式 | 必过 |
|---|---|---|
| AC-VF1 | d2-domain.md §North Star 4 S 加 ValueFlow Alias 列 | ✅ |
| AC-VF2 | a-registry ValueFlow Semantic 列完整 | ✅ |
| AC-VF3 | f-registry ValueFlow Semantic 列完整 | ✅ |
| AC-VF4 | t-registry ValueFlow Semantic 列完整 + layer-delta.md 同步 | ✅ |

### 5.5 Span AC（AC-Span，8 项，子 Change #4）

| AC | 验收方式 | 必过 |
|---|---|---|
| AC-Span1 | 6 harness ops 删 | ✅ |
| AC-Span2 | 3 queryloop ops 删 | ✅ |
| AC-Span3 | 8 context ops 删 | ✅ |
| AC-Span4 | 2 tool ops 删 | ✅ |
| AC-Span5 | 7 task/plan ops 删 | ✅ |
| AC-Span6 | names.go 26 常量同步删除；编译过 | ✅ |
| AC-Span7 | t-registry 232 T 行 Span Evidence 列填充 94% | ✅ |
| AC-Span8 | Coverage Guard CI script ≥80% 守门 | ✅ |

### 5.6 Boundary AC（AC-Bound，4 项，子 Change #5）

| AC | 验收方式 | 必过 |
|---|---|---|
| AC-Bound1 | nested/flow_report.go 物理迁 orchestration/executionflow/bridge/subquery_bridge.go | ✅ |
| AC-Bound2 | d2-domain.md §Out of Scope 加 Pending Boundary Decision 列 + DM-018 slice-c 标注 | ✅ |
| AC-Bound3 | t-registry summarizer_fixture.go + prepared_turn_fixture.go 显式标 fixture type | ✅ |
| AC-Bound4 | internal/layers/contextengine/orchtypes/boundary_decision.go + test PASS（4 单元测试） | ✅ |

### 5.7 总体 AC（AC-Total，4 项，全部子 Change）

| AC | 验收方式 | 必过 |
|---|---|---|
| AC-T1 | 22+/22+ D2 packages -race PASS | ✅ |
| AC-T2 | verify-archive.sh 12/12 PASS | ✅ |
| AC-T3 | t-registry P0 子集 100% PASS | ✅ |
| AC-T4 | 8 PR 全部 squash merge + auto-merge | ✅ |

---

## §6 Gherkin 验收场景

### 6.1 子 Change #0 — legacy/ 清理

```gherkin
Scenario: legacy/ 目录不存在
  Given PR-1 完成
  When 检查 internal/layers/contextengine/legacy 目录
  Then 目录不存在
  And 13 test 文件全部 import path 改完
  And go test -race ./internal/layers/contextengine/... PASS
```

### 6.2 子 Change #1 — god fn 拆分

```gherkin
Scenario: pipeline.go::RunForSession 拆 3 文件
  Given PR-2 完成
  When 验证 compression_steps.go + compression_budget.go 文件存在
  And pipeline.go 主体 <300 行
  Then 7 step helper 全部就位
  And go test ./internal/layers/contextengine/prepare/compression/... -race PASS
```

### 6.3 子 Change #4 — Span Evidence 覆盖率

```gherkin
Scenario: Span Evidence 94% 覆盖率
  Given PR-7 完成
  When 运行 scripts/d2-span-coverage.sh
  Then 输出 "D2 Span Evidence Coverage: 94% (220/232 T rows)"
  And exit code 0
```

### 6.4 子 Change #5 — DM-018 slice-c 物理迁移

```gherkin
Scenario: DM-018 slice-c 物理迁移
  Given PR-8 完成
  When 验证 internal/layers/contextengine/nested/flow_report.go 不存在
  And 验证 internal/layers/orchestration/executionflow/bridge/subquery_bridge.go 存在
  Then D2 不再持有 FlowEvent ownership
  And boundary_decision.go + 4 单元测试 PASS
```

---

## §7 修订记录

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-06-29 | 初版：6 子 Change 联动 Gherkin + 44 AC 全表 + D2 v8.2.0 → v9.0.0 |