---
demand-id: DM-20260619-007
title: D2 v2.2 Structure 终态 — Scenario 编排收敛 + 物理路径双锚点闭合
source: DSAFT Refactoring Playbook v2.0 Structure + D2 领域对焦（2026-06-19）
priority: P0
status: S2_Clarified
dsaft_domain: contextengine
created: 2026-06-19
---

# D2 Context Engine v2.2 Structure 终态

## 1. 背景

D2 已完成 v1.0 Registry（DM-20260614-009：S15–S18 Canonical）、QueryLoop 拆除（DM-20260618-010）、以及 **部分** v2.0 Structure（#104：`facade/` 分包、`prepare/token/`、`kernel/spans.go`、根 3 生产文件）。

但 **规格锚点与运行路径仍未闭合终态**：

| 维度 | 终态期望 | 当前现状 |
|------|----------|----------|
| S15/S17/S18 orchestrator | 生产唯一 SoT | `prepare/persist/enforce/orchestrator.go` **仅单测**；真逻辑在 `facade/engine_*.go` |
| D7 Turn 拆面 | adapter 调 scenario orchestrator | `bootstrap/turn_adapter.go` 部分 duplicate |
| Tools 归属 | S18 `enforce/tools/` | `enforce/toolrunner/` 技术树（49 文件） |
| Worker 沙箱 | S18 `enforce/sandbox/` | `contextengine/sandbox/` 挂域根 |
| Memory 读写 | Recall→S15 / Store→S17 | Store 仍在 `prepare/memory/longterm.go` |
| 根目录 | 仅 `contracts.go` + `aliases.go` | 另有 `tool_context.go` + ~10 个 `*_test.go` |
| a-registry Code Location | 与物理路径一致 | 仍写 `engine_persist.go` 等待更新 |
| Legacy Process | deprecated / 删除 | `facade/engine.go` 仍为主路径之一 |

对标 D7 DM-20260619-005：**目录已 scenario 化，调用链未 scenario 化**。

## 2. Owner 决议（设计缺省 — 本 change 采用）

| # | 议题 | 决议 |
|---|------|------|
| Q1 | 重构范围 | **C** — 编排收敛 + 物理归位 + 根目录清零 + 规格双锚点（不重做 QueryLoop 拆除） |
| Q2 | `facade/` 终态 | 重命名为 `legacy/`，仅保留 `Process()` + builder；D7 100% 走拆面后删除 |
| Q3 | `toolrunner/` | 重命名为 `enforce/tools/`（scenario slug，非技术名） |
| Q4 | `sandbox/` | 迁入 `enforce/sandbox/`（D2-S12 → S18） |
| Q5 | `mock/` | 迁出域 → `tests/testutil/contextengine/` |
| Q6 | 目录最大深度 | scenario 下 **最多 2 层**（例：`enforce/tools/surface/` ✅；更深需 F-registry 登记） |
| Q7 | T 层 | **T ID 不变**；路径变更；新增根目录/深度 layout 守卫 T |
| Q8 | D7 | **本轮不动** D7 场景目录（已在 DM-005 完成） |

## 3. 问题陈述

| # | 问题 | 影响 |
|---|------|------|
| P1 | Scenario orchestrator 与 facade 双轨 | 改 S15 逻辑需改两处；T 绿不等于生产绿 |
| P2 | Tools/Memory 按技术堆叠而非价值流 | onboarding 无法从目录回答「属于哪个 S」 |
| P3 | 根目录测试/mock 污染 | Structure 守卫失效；域边界模糊 |
| P4 | a-registry / design 漂移 | 双锚点断裂；Grill 无法验证 |
| P5 | S18 执行分散在 bootstrap + enforce | `PolicyOrchestrator` stub vs `turn_adapter` 真执行 |

## 4. 终态物理目录（Canonical SoT）

```text
internal/layers/contextengine/
│
├── contracts.go                         # kernel 类型 re-export
├── aliases.go                           # ContextEngine / EngineDeps / NewContextEngine / tool aliases
│
├── kernel/                              # 域内核（无业务编排）
│   ├── contracts.go
│   └── spans.go
│
├── prepare/                             # ★ D2-S15 PrepareExecutionContext
│   ├── orchestrator.go                  # 生产 wired 唯一入口
│   ├── contracts.go
│   ├── memory/
│   │   ├── manager.go                   # A01 LoadSession
│   │   └── recall.go                    # A02 RecallLongTerm（只读）
│   ├── compression/                     # A03
│   ├── token/                           # A03 辅助
│   ├── prompt/                          # A04
│   ├── conversation/
│   ├── attachments/
│   ├── usercontext/
│   └── tools_list.go                    # 调用 enforce 构建本轮 tools[]（无执行）
│
├── persist/                             # ★ D2-S17 PersistSessionState
│   ├── orchestrator.go                  # 生产 wired 唯一入口
│   ├── contracts.go
│   ├── snapshot/                        # A01
│   ├── transcript/                      # A02
│   ├── memory/
│   │   └── store.go                     # A03 StoreLongTerm（从 prepare 迁出）
│   └── commit.go                        # A04 CommitWindow / TrimMessages
│
├── enforce/                             # ★ D2-S18 EnforceExecutionPolicy
│   ├── orchestrator.go                  # 与 turn_adapter 对齐（非 stub）
│   ├── contracts.go
│   ├── permission/                      # A01
│   ├── sandbox/                         # A03 WorkerDirSandbox（自根迁入）
│   ├── subquery.go                      # A06/A08
│   ├── background.go                    # A07
│   ├── background_task_tools.go
│   ├── planmode_tools.go
│   ├── tool_filter.go
│   ├── agent_role_filter.go
│   ├── registry/                        # A04
│   └── tools/                           # 原 toolrunner/
│       ├── runner.go                    # A05 Execute 入口
│       ├── context.go                   # 原 tool_context.go 主体
│       ├── surface/
│       ├── filter/
│       ├── builtin/
│       ├── bash/
│       ├── sandboxast/
│       └── zodgen/
│
└── legacy/                              # Deprecated（D7 100% 后删除）
    ├── process.go                       # 原 facade/engine.go runProcess
    ├── builder.go                       # NewContextEngine
    └── wire_prepared_turn.go

tests/
├── integration/d2/                      # 原根目录 integration tests
└── testutil/contextengine/              # 原 mock/
```

### 4.1 根目录现有文件 → 终态映射

| 当前路径 | 终态 |
|----------|------|
| `contracts.go` | ✅ 保留 |
| `aliases.go` | ✅ 保留（扩展 tool alias） |
| `tool_context.go` | → `enforce/tools/context.go` + 根 alias |
| `facade/*` | → `legacy/*`（Process deprecated） |
| `sandbox/*` | → `enforce/sandbox/*` |
| `mock/*` | → `tests/testutil/contextengine/*` |
| `queryloop_removed_test.go` | → `internal/lint/layer/d2_layout_test.go` |
| `engine_accessor_test.go` | → `legacy/builder_test.go` 或 `tests/integration/d2/` |
| `engine_persist_bridge_test.go` | → `persist/commit_test.go` |
| `compression_unified_test.go` | → `tests/integration/d2/` |
| `plan_mode_tools_*.go` | → `enforce/planmode_tools_test.go` |
| `prepared_turn_integration_test.go` | → `tests/integration/d2/` |
| `path_regression_integration_test.go` | → `tests/integration/d2/` |
| `tool_stream_test.go` | → `enforce/tools/context_test.go` |
| `contextengine_test_helper_test.go` | → `tests/testutil/contextengine/` |

## 5. 终态调用链

```text
D7 RunTurnLoop
  ├── turn_adapter → prepare.Orchestrator.Prepare()
  ├── D7 InvokeLLM → D3
  ├── turn_adapter → enforce.* ExecuteToolRound()
  └── turn_adapter → persist.Orchestrator.Persist()

legacy.Process() → @Deprecated → 仅兼容测试；lint 禁止新增生产引用
```

## 6. Concern → Scenario 归属（速查）

| Concern | Scenario | 目录 |
|---------|----------|------|
| Session Load / Recall / Compress / Prompt | S15 | `prepare/` |
| Token 计数 / Window 分析 | S15 | `prepare/token/` |
| Snapshot / Transcript / Store / Trim | S17 | `persist/` |
| Tool 注册 / 执行 / 权限 / 沙箱 / Surface | S18 | `enforce/tools/` 等 |
| Worker 目录隔离 | S18 | `enforce/sandbox/` |
| 域契约 / Span | kernel | `kernel/` |
| 对外稳定 API | 根 | `contracts.go` + `aliases.go` |

## 7. 验收标准（AC）

### Phase P1 — 编排收敛

| AC | 描述 | 优先级 |
|----|------|--------|
| AC-P1-1 | `facade/legacy` Prepare 路径改调 `prepare.Orchestrator` | P0 |
| AC-P1-2 | Persist 路径改调 `persist.Orchestrator` + `commit.go` | P0 |
| AC-P1-3 | `turn_adapter` Prepare/PersistTurn 与 orchestrator 对齐，无 duplicate inline | P0 |
| AC-P1-4 | 关联 T 全绿 + D7 turn 集成测试全绿 | P0 |

### Phase P2 — 根目录清零

| AC | 描述 | 优先级 |
|----|------|--------|
| AC-P2-1 | 根生产文件仅 `contracts.go` + `aliases.go` | P0 |
| AC-P2-2 | 根 `*_test.go` 全部迁出 | P0 |
| AC-P2-3 | `mock/` 迁 `tests/testutil/contextengine/` | P1 |
| AC-P2-4 | `TestD2_RootProductionFiles` + layout lint 守卫 | P0 |

### Phase P3 — enforce 归位

| AC | 描述 | 优先级 |
|----|------|--------|
| AC-P3-1 | `sandbox/` → `enforce/sandbox/` | P0 |
| AC-P3-2 | `toolrunner/` → `enforce/tools/` | P0 |
| AC-P3-3 | `enforce/orchestrator` 与 ExecuteRound 语义对齐 | P1 |
| AC-P3-4 | `code-layout.md` 深度规则 ≤2 层写入 | P0 |

### Phase P4 — Memory 读写分离

| AC | 描述 | 优先级 |
|----|------|--------|
| AC-P4-1 | `StoreLongTerm` 迁 `persist/memory/store.go` | P0 |
| AC-P4-2 | a-registry S17-A03/A04 Code Location 更新 | P0 |

### Phase P5 — Legacy 退役

| AC | 描述 | 优先级 |
|----|------|--------|
| AC-P5-1 | `facade/` → `legacy/`；`Process()` Deprecated | P1 |
| AC-P5-2 | lint 禁止新增 `Process()` 生产引用 | P1 |

### Phase P6 — 规格双锚点

| AC | 描述 | 优先级 |
|----|------|--------|
| AC-P6-1 | `d2-domain.md` v8.2 终态路径表 | P0 |
| AC-P6-2 | `a-registry` / `f-registry` Code Location 全量同步 | P0 |
| AC-P6-3 | `design.md` / `layering.md` 目录树更新 | P1 |
| AC-P6-4 | S5 验收后 `openspec/specs/d2-context-engine/` delta 合入 | P0 |

## 8. 非目标

- 不修改 D7 场景物理路径（DM-20260619-005 已闭合）
- 不重新引入 QueryLoop / S16
- 不在本 change 内实现 Anthropic native client
- 不改变 `shared/contracts` 跨域接口签名（仅 D2 内部重组）

## 9. L1–L5 映射草案

| L1 | L2 | L3 示例 | L4 示例 |
|----|-----|---------|---------|
| D2 contextengine | S15 prepare | A03 CompressContext | `prepare/compression/pipeline.go` |
| D2 | S17 persist | A04 CommitWindow | `persist/commit.go` |
| D2 | S18 enforce | A05 ExecuteToolRound | `enforce/tools/runner.go` |

## 10. 依赖与顺序

```text
#104 facade 分包（已合 master）
  → P1 编排收敛
  → P2 根清零 ∥ P3 enforce 归位
  → P4 memory 分离
  → P5 legacy 退役
  → P6 规格合入 specs/
```

## 11. 风险

| 风险 | 缓解 |
|------|------|
| 编排收敛改动面大 | 分 2 PR；每 PR <400 行；T 先行 |
| import 路径 churn | git mv + 机械替换；layer-lint |
| bootstrap 与 D2 边界 | turn_adapter 只做 wiring，逻辑留 enforce |
