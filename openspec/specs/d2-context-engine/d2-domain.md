# D2 Context Engine Domain

**Domain ID:** D2
**Slug:** `contextengine`
**Type:** Core Domain
**Status:** Active — Canonical S15–S18 (v2.2 final, S19 dismantled, S20 removed)
**Version:** 8.3.0
**Last Updated:** 2026-06-29
**Depends On:** ~~D3 (ILLMGateway)~~ → **D7 消费（DM-020）**, D5 (Observability), **D7 (invocation only — Leader)**
**Hard Ban:** D2→D3 import 禁止（DM-020 v1.0 Registry，v2.0-d CI 硬阻断）
**Depended By:** D1 (EngineEvent consumer), **D7 (TurnExecutor / PreparedTurnRunner consumer)**
**Cross-Domain SoT:** `d7-boundary.md`

---

## North Star

**在会话边界内，可靠地准备上下文、执行工具轮次，并持久化会话状态——作为被 D7 调度的纯执行原语（Context Follower）。D7 拥有 LLM 调用权与 Turn 主循环；D2 仅负责 Prepare / ToolRound / Persist。**

| 可验证承诺 | Canonical S |
|-----------|-------------|
| Turn 前上下文合法、在预算内 | D2-S15 PrepareExecutionContext |
| LLM↔Tool 多轮 | ~~D2-S16 RunQueryLoop~~ → **D7-S2-A06 RunTurnLoop**（DM-20260618-010 REMOVED） |
| Tool 权限/沙箱先于执行 | D2-S18 EnforceExecutionPolicy |
| Turn 后状态 durable + deferred complete | D2-S17 PersistSessionState |
| 嵌套 SubQuery/Background 有边界 | ~~D2-S19 NestedExecution~~ → S15+S18 拆分 |
| ~~Legacy Harness 仅显式配置~~ | D2-S20 LegacyHarnessFallback（**REMOVED v6.5.0**） |

---

## Out of Scope

| 能力 | 归属 | 备注 |
|------|------|------|
| IM ingress / 信号语义 | D1 | EngineEvent 产出，展示归 D1 |
| ProcessMessage / Wave / ClassifyIntent | D7 | Leader |
| Task 写模型 / PlanMode | D7-S1/S5 | ✅ 已迁入 `internal/layers/orchestration/workmodel/`（DM-20260614-009 v1.1 closure） |
| delegate_* 路由 | ✅ D7 `delegatetools/` | ~~`delegate_tools.go`~~ 已迁出 |
| FlowEvent / WorkPlan | D7-S4 | `queue/` delegate-progress |
| 结论质量 / 信誉 | D6 | Judge |

---

## DSAFT 双轨

### Canonical 价值流（SoT）— D2-S15–S20

| S ID | Scenario | Responsibility | Status |
|------|----------|----------------|--------|
| D2-S15 | PrepareExecutionContext | Load, repair, compress, assemble prompt | REGISTRY |
| D2-S16 | RunQueryLoop | ~~LLM↔Tool 执行原语~~ | **REMOVED（v8.0.0，DM-20260618-010）→ D7-S2-A06** |
| D2-S17 | PersistSessionState | Snapshot, transcript, commit window | REGISTRY |
| D2-S18 | EnforceExecutionPolicy | Permission, sandbox, tool surface | **REGISTRY（自 S16 拆出 tool 面，DM-020）** |
| D2-S19 | NestedExecution | ~~SubQuery, background, fork, sidechain~~ → S15+S18 拆分 | **DISMANTLED（v6.4.0）** |
| D2-S20 | LegacyHarnessFallback | ~~`query_loop.enabled=false` 路径~~ | **REMOVED（v6.5.0）**: harness 完全移除 |

### Legacy Module Index（冻结追溯）— D2-S1–S14

| Module ID | Scenario | Status | Canonical 映射 |
|-----------|----------|--------|----------------|
| D2-S1 | PEV | RETIRED | — |
| D2-S2 | Compression | Legacy | → S15 |
| D2-S3 | Memory | Legacy | → S15, S17 |
| D2-S4 | Token | Legacy | → S15 |
| D2-S5 | Registry | Legacy | → S18 |
| D2-S6 | Snapshot | Legacy | → S17 |
| D2-S7 | Prompt | Legacy | → S15 |
| D2-S8 | Sandbox | Legacy | → S18 |
| D2-S9 | Harness | Legacy | → **REMOVED** |
| D2-S10 | QueryLoop | Legacy | **REMOVED → D7-S2-A06** |
| D2-S11 | Queue | Legacy | → **D7-S4** |
| D2-S12 | WorkerDirSandbox | Legacy | → S18 (`contextengine/sandbox/`) |
| D2-S13 | Conversation | Legacy | → S15 |
| D2-S14 | Mock | Legacy | 测试辅助 |

> **Change:** `openspec/archive/2026-06-14-devrix-d2-sa-refine/` (DM-20260614-009)

### 物理路径映射表（Canonical S → 代码目录）

| Canonical S | Scenario | 物理路径 |
|-------------|----------|---------|
| D2-S15 | PrepareExecutionContext | `prepare/` (memory/recall.go, compression/, prompt/, conversation/fork.go+fork_worker.go, attachments/, usercontext/) |
| D2-S16 | RunQueryLoop | **REMOVED** — loop 归 D7 `turn/orchestrator.go` |
| D2-S17 | PersistSessionState | `persist/` (snapshot/, transcript/, commit.go, commit_window.go, orchestrator.go, **memory/store.go**) |
| D2-S18 | EnforceExecutionPolicy | `enforce/` (permission/, **tools/** (was toolrunner/), registry/, tool_filter.go, agent_role_filter.go, background.go, subquery.go, background_task_tools.go, planmode_tools.go, **sandbox/**) |
| — | Process legacy (S15→D7→S17 glue) | **`legacy/`** (`engine.go`, `engine_builder.go`, `engine_persist_v2.go`, `prepared_turn_*.go`, `engine_*.go`) — **P5 Deprecated** (slog.Warn + T07 guard) |
| D2-S4 (Legacy Token) | Token counting | `prepare/token/` (+ `windowanalyzer/`) |
| D2 kernel | Domain contracts + span registry | `kernel/` (`contracts.go`, `spans.go`) |
| D2 root | Public API re-exports | `contracts.go`, `aliases.go`, `tool_context.go`（3 生产文件，type alias only） |
| D2-S19 | ~~NestedExecution~~ → S15+S18 | **DISMANTLED**: fork→`prepare/conversation/`, subquery+background→`enforce/` |
| D2-S20 | ~~LegacyHarnessFallback~~ | **REMOVED（v6.5.0）**: `fallback/` 目录已删除，所有 harness 相关代码已清理 |

> **v8.2 重命名记录（DM-20260619-007 devrix-d2-structure-closure）：**
> - `enforce/toolrunner/` → `enforce/tools/`（P3-T2，scenario slug 重命名，package 名同步 `package tools`）
> - `enforce/sandbox/` 物理迁入 `enforce/sandbox/`（P3-T1，逻辑归属 `enforce/`，与 D1 capture/channel/delivery 模式对齐）
> - `enforce/orchestrator.go`（92 行 stub）**删除**（P3-T4，dispatch 由 `turn_adapter.ExecuteRound` 接管）
> - `prepare/memory/longterm.go` 拆分为 `prepare/memory/recall.go`（S15 端口）+ `persist/memory/store.go`（S17 实现）（P4 split）
> - `MemoryEntry` 提升至 `internal/shared/types/memory.go`（共享类型，无域拥有）
> - `LongTermRecaller` / `LongTermStore` 提升至 `internal/shared/contracts/memory.go`（端口接口，D2-STRUCT-T04 解耦）
> - `facade/` → `legacy/`（P5 retirement，Deprecated + slog.Warn + T07 layer-lint 硬阻断）
>
> **v2.0 历史重命名：**
> - `policy/` → `enforce/`（2026-06-16）
> - `harness/` → `fallback/`（2026-06-16，已删除 v6.5.0）
> - `attachments/` → `prepare/attachments/`（2026-06-16）
> - `usercontext/` → `prepare/usercontext/`（2026-06-16）

---

## 与 D7 关系（Leader / Follower）

> 完整矩阵见 [`d7-boundary.md`](./d7-boundary.md)。

### 角色

| | D7 | D2 |
|---|----|----|
| Stackelberg | **Leader** — 先选路径与 Executor | **Follower** — 后执行 Prepare/ToolRound/Persist |
| 回答 | 做什么、顺序、谁做、进度广播 | 上下文如何准备、工具如何执行、状态如何持久化 |
| 不保证 | 结论质量（D6） | 编排决策正确（D7-S5） |

### 调用链（DM-020 修订）

```text
D1 → D7.ProcessMessage → D7.RunTurnLoop（D7-S2-A06）
                              ├── D2.Prepare（D2-S15）
                              ├── D7.InvokeLLM → D3.StreamChat（D7 直调，D2 不参与）
                              ├── D2.ExecuteToolRound（D2-S18）
                              └── D2.PersistTurn（D2-S17）
                              ↓
                         EngineEvent → D7 → D1
                         FlowEvent（SubQuery）→ D7-S4 → D1
```

> **DM-020 修订：** D7 不再通过黑盒 `d2Executor.RunQueryLoop` 调用 D2。Turn 主循环归 D7-S2-A06，LLM 调用归 D7-S2-A07（D7→D3），D2 拆面为 Prepare / ToolRound / Persist 三个独立契约。

### D7 路径 × D2 参与

| D7 路径 | D2 Canonical |
|---------|--------------|
| FastPath | S15→D7 RunTurn→S17 |
| Wave Worker (D2) | S18 per worker（Turn 由 D7 调度） |
| SubQuery / Background | S15（fork）+ S18（subquery/background） |
| PlanMode（决策在 D7-S5） | S18 机制执行 |

### 注入点（D7 → D2，非 D2 编排）

| 注入 | 用途 | v2.0 |
|------|------|------|
| `PreparedTurnRequest` | session、message、system、tools | 保持 |
| `SessionQueue` | progress drain | 迁 D7-S4 |

### 跨域漂移（v1.0 登记 / v2.0 迁移）

| 组件 | 目标 D7 |
|------|---------|
| `tasks/` | S1 / S5 |
| `delegate_tools.go` | S2/S5 F |
| `queue/` delegate-progress | S4 |

---

## 与 D7 接口（实现锚点）

```text
v2.0（DM-020 + DM-20260618-010）:
  wire_coordinator.go: ContextPreparer / ToolRoundExecutor / SessionPersister → D2
  wire_coordinator.go: turnOrchExecutor (TurnExecutor) → D7 RunTurn
  engine.Process: PreparedTurnRunner → D7 RunTurn / SubTurn
  D7-S2-A07 InvokeLLM → bridges/llm → D3（D7 直调，D2 不参与）
```

D2 **不** import `orchestration` 包。**D2→D3 import 禁止**（CI 硬阻断）。

---

## 实现状态（2026-06-19, v8.0.0）

| 项 | 状态 |
|----|------|
| D7 Turn 主路径（PreparedTurnRunner） | ✅ IMPLEMENTED |
| Per-turn compression (`turn_runtime.compress_per_turn`) | ✅ IMPLEMENTED |
| Deferred complete | ✅ IMPLEMENTED |
| D2 Thin（无 orchestration/multiagent import） | ✅ IMPLEMENTED |
| D7 ingress（capture 无 contextengine） | ✅ IMPLEMENTED |
| QueryLoop 物理删除（S16 REMOVED） | ✅ DM-20260618-010 |
| tasks/ 归 D7 | ✅ 已迁入 `orchestration/workmodel/` |
| scenario 物理路径 S15/S17/S18 | ✅ IMPLEMENTED |
| **D2-S16 Legacy Freeze（→ D7-S2-A06）** | ✅ v1.0 Registry（DM-020） |
| **D2→D3 import lint（CI 硬阻断）** | ✅ v2.0-d（DM-020） |
| **S18 ExecuteToolRound 拆面** | ✅ v2.0-d（DM-020） |
| **S20 LegacyHarnessFallback 移除** | ✅ 所有 harness 代码、类型、测试已删除 |
| **S19 NestedExecution 拆解** | ✅ fork→prepare/conversation/, subquery+background→enforce/ |
| **v2.0 根目录瘦身** | ✅ 根 3 生产文件 + `facade/` 编排包（`engine.go` Process glue） |
| **P5 legacy 退役** | ✅ `facade/` → `legacy/` → **kernel/** (DM-20260629-002 PR-1 closed P5 window)；`legacy.Process()` 已删除；D2-STRUCT-T07 layout guard 硬阻断改为 `kernel.ContextEngine.Process()` |
| **F registry 重映射** | ✅ DM-20260629-002 PR-4 — F ID D2-S1..S5 → canonical D2-S15..S18；S1/S5/S9/S10/S19/S20 移入 Historical Appendix tombstone |
| **工具注册迁入 enforce/** | ✅ `background_task_tools.go` + `queryloop_tools.go` → `enforce/` |

---

## 修订记录

| Version | Date | Changes |
|---------|------|---------|
| 8.2.0 | 2026-06-19 | **v2.2 closure (DM-20260619-007)**: (1) `enforce/toolrunner/` → `enforce/tools/` (package `tools`); (2) `enforce/sandbox/` 物理迁入 enforce/; (3) 删除 `enforce/orchestrator.go` (92 行 stub); (4) `prepare/memory/longterm.go` 拆分 recall.go + persist/memory/store.go; (5) `MemoryEntry` 提升至 shared/types; (6) `facade/` → `legacy/` (Deprecated + slog.Warn + T07 guard) |
| 8.3.0 | 2026-06-29 | **DM-20260629-002 PR-4 registry-sync**: (1) F-registry D2-S1..S5 → canonical D2-S15..S18 重映射；(2) S1 PEV / S5 Nested / S9 Harness / S10 QueryLoop / S19 NestedExecution / S20 LegacyHarness 统一移入 Historical Appendix；(3) PR-1 legacy/→kernel/ 路径落地反映到实现状态表 |
| 8.1.0 | 2026-06-19 | **DSAFT Structure 重构**: (1) `engine_*.go` + `prepared_turn_*.go` → `facade/`; (2) 根包仅 `contracts.go` + `aliases.go` + `tool_context.go`; (3) `token/` → `prepare/token/`; (4) `spans.go` → `kernel/spans.go` |
| 6.1.0 | 2026-06-15 | DM-020 拆面闭合状态同步：实现状态表 3 项 ⬜ PLANNED → ✅ IMPLEMENTED（D2-S16 Legacy Freeze / D2→D3 import lint / S18 ExecuteToolRound 拆面），引用 commit 41aec47 与 `TestD2_D3Ban` 实测通过 |
| 7.0.0 | 2026-06-16 | **v2.0 终态**: (1) `background_task_tools.go` + `queryloop_tools.go` → `enforce/`; (2) 根目录合并 `tool_register.go`→`tool_context.go`; (3) `spans.go` 清理 harness span 死引用; (4) 根目录 11 生产文件 `engine.go` 212行 Facade |
| 6.5.1 | 2026-06-16 | **根目录瘦身**: `process_overlay.go` → `prepare/conversation/fork_worker.go`; 删除死代码 `tool_messages.go`; `tool_context.go` 移除 `parseToolInput`/`toolInputString` 包装函数 |
| 6.5.0 | 2026-06-16 | **S20 移除**: `fallback/` 目录删除，`engine_harness.go` + `harness_adapter.go` 删除，`engine.go` 去 harness 分支，`assembler.go` 清理 harness 字段，`types/harness.go` 删除，`SessionContext.Harness`/`.Transcript` + `Session.HarnessInitialized` 移除 |
| 6.4.0 | 2026-06-16 | **S19 拆解（DISMANTLED）**: fork.go → `prepare/conversation/`（S15），subquery.go + background.go → `enforce/`（S18），所有外部 import `nested.` → `enforce.`，`nested/` 目录删除 |
| 6.3.0 | 2026-06-16 | **v2.0 物理重构同步**：(1) 新增物理路径映射表；(2) 实现状态 `scenario 物理路径 S15/S17/S18` ⬜ v2.0 → ✅ IMPLEMENTED；(3) 记录 `policy/`→`enforce/`、`harness/`→`fallback/`、`attachments/`→`prepare/`、`usercontext/`→`prepare/` 重命名 |
| 6.2.0 | 2026-06-15 | **D7 Real-Closure Spec Sync (D2 侧)**：(1) 实现状态表 `tasks/ 归 D7` ⬜ v2.0 → ✅ 已迁入 `orchestration/workmodel/`（DM-20260614-009 v1.1 closure，commit 41aec47 后）；(2) Out of Scope 表 `Task 写模型 / PlanMode` 备注从 `代码暂在 tasks/` 改为 `✅ 已迁入 orchestration/workmodel/` |

---

## 规格文档索引

| 文档 | 用途 |
|------|------|
| `spec.md` | Gherkin 验收规格 |
| `terminal-state-guide.md` | D7 拆面时序、S15–S18 A 树、硬约束 |
| `observability-guide.md` | Span↔T、DM-020 Trace 树、P0 Runbook |
| `design.md` | 六段式架构设计 |
| `a-registry.md` / `f-registry.md` / `t-registry.md` | A/F/T 注册表 |
| `span-registry.md` | Span operation 登记 SoT |
| `d7-boundary.md` | **D2↔D7 跨域 SoT** |
| `dsaft-architecture.md` | Stub — DSAFT 五层计数 |
| `../d1-communication/d1-domain.md` | D1 入口与展示域 SoT |
| `layer-delta.md` | Delta SoT |
| `docs/methodology/dsaft-refactoring-playbook.md` | 重构方法论 |
