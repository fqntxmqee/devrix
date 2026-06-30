# Context Engine Specification

**Capability:** context-engine
**Layer:** 2
**Version:** 9.0.0
**Status:** Canonical — source of truth
**Last Updated:** 2026-06-29 (DM-20260629-002 d2-dsaft-restructuring v9.0.0 S7_Archived)
**Domain SoT:** `d2-domain.md` v9.0.0 — North Star / 物理路径 / 实现状态
**D7 Boundary:** `d7-boundary.md` — Leader/Follower 契约 + 跨域漂移清单

> **精简设计契约（Lite-Mode）**：本文档只放当前符合代码的设计契约（v9.0.0）。**过程需求迭代**（66 Requirement / 96 Scenario 详细文本）不进入本文件，留在 `archive/<change-id>/specs/` 各 change 归档目录。详细时间线见 [CHANGELOG.md](CHANGELOG.md)。

---

## Overview

D2 上下文引擎是 **Context Follower**（被 D7 调度的纯执行原语）。职责：**会话级消息历史、Token 预算、七步压缩、工具执行原语（Prepare / ToolRound / Persist）**，通过 `IContextEngine.Process` 向通信层输出 `EngineEvent` 流。**LLM↔Tool 多轮循环归 D7**（`D7-S2-A06 RunTurnLoop`，DM-20260618-010 v8.0.0 物理删除 D2 QueryLoop）。

**现行实现路径（v9.0.0）**：`kernel.ContextEngine.Process()` → D7 `PreparedTurnRunner` 委托 RunTurn/SubTurn → D7 直调 D3 LLM Gateway → 回调 D2 `EnforceExecutionPolicy.ExecuteToolRound` → D2 `PersistSessionState` 落盘。

| ValueFlow | Canonical S | 职责 |
|-----------|-------------|------|
| `D2_Context_Loading_Compression` | D2-S15 PrepareExecutionContext | Turn 前上下文合法、在预算内 |
| `D2_Tool_Permission_Sandbox` | D2-S18 EnforceExecutionPolicy | Tool 权限/沙箱先于执行 |
| `D2_Session_State_Persistence` | D2-S17 PersistSessionState | Turn 后状态 durable + deferred complete |

### 核心设计原则

1. **Context Follower（Leader/Follower）**：D7 拥有 Turn 主循环与 LLM 调用权；D2 仅 Prepare / ToolRound / Persist，**D2→D3 import 禁止**（CI 硬阻断，DM-020 v2.0-d）
2. **不可变 + 纯函数优先**：会话上下文（`SessionContext`）通过 `With*` 返回新副本；`SessionContext` 不可 in-place mutation
3. **七步压缩 + 异步 Autocompact**：占位摘要 + 后台 LLM 摘要 + `OnAutocompactComplete` 异步收口（V4）
4. **Snappy 快照压缩**：魔数头 + legacy JSON 兼容；会话重启 cost ↓（V4）
5. **Deferred Complete Event**：`complete` 仅在 `sc.Messages` + `ContextSnapshot` 持久化完成后发射，避免下游读到不一致状态（V7）
6. **跨域工具注册中心化**：`[]contracts.ToolSurface` 列表 + `[]contracts.ToolFilter` 链，3 入口 (NewContextEngine / buildWithGate / WireDelegate) + 0 package-level global（TOOL-SURFACE-1）
7. **Hard Ban D2→D3 import**：D7 持有 LLM Gateway 调用权；D2 仅消费 `EngineEvent` 流
8. **Trace 树全程贯穿**：每跳节点产生 child span；D7-S16 Layer SubContext 注入 PartitionBase/Above 字段（D2-S16-A20 ContextMaterializer.Materialize）

### S 层职责（canonical S15-S18）

| S ID | Scenario | Responsibility | Status | ValueFlow |
|------|----------|----------------|--------|-----------|
| D2-S15 | PrepareExecutionContext | Load, repair, compress, assemble prompt | **REGISTRY** | `D2_Context_Loading_Compression` |
| D2-S16 | RunQueryLoop | ~~LLM↔Tool 执行原语~~ → D7-S2-A06 RunTurnLoop | **REMOVED (v8.0.0)** | (归 D7) |
| D2-S17 | PersistSessionState | Snapshot, transcript, commit window | **REGISTRY** | `D2_Session_State_Persistence` |
| D2-S18 | EnforceExecutionPolicy | Permission, sandbox, tool surface | **REGISTRY** | `D2_Tool_Permission_Sandbox` |
| D2-S19 | NestedExecution | SubQuery/background/fork/sidechain → S15+S18 | **DISMANTLED (v6.4.0)** | (历史) |
| D2-S20 | LegacyHarnessFallback | ~~`query_loop.enabled=false` 路径~~ | **REMOVED (v6.5.0)** | (历史) |

> Legacy S1-S14（PEV/Compression/Memory/Token/Registry/Snapshot/Prompt/Sandbox/Harness/QueryLoop/Queue/WorkerDirSandbox/Conversation/Mock）映射详见 [d2-domain.md §Legacy Module Index](d2-domain.md)。

---

## DSAFT 结构

| 层级 | ID | 名称 | 物理路径 |
|------|-----|------|----------|
| D | D2 | Context Engine | `internal/layers/contextengine/` |
| S | D2-S15 | Prepare Execution Context | `prepare/` (`conversation/` + `pipeline/` + `assembler/`) |
| S | D2-S17 | Persist Session State | `persist/` (`memory/`, `transcript/`, `snapshot/`) |
| S | D2-S18 | Enforce Execution Policy | `enforce/` (`tools/`, `sandbox/`, `permission/`) |
| S | D2-S16 | ~~Run Query Loop~~ | **REMOVED → D7-S2-A06 RunTurnLoop** |
| A | A1-A99 | 22 个核心活动 | 见 `a-registry.md` |
| F | F1-F999 | 功能点编排 | 见 `f-registry.md` |
| T | T1-T200 | 测试点（IMPLEMENTED） | 见 `t-registry.md` |

**当前计数（v9.0.0）**：D=1, S=4 (canonical: S15/S17/S18 + S16→D7) + S=10 (legacy tombstone), A=22, F=120, T=180 (IMPLEMENTED)。

---

## Scenarios

| ID | Scenario | Responsibility | Status | 代码位置 |
|----|----------|----------------|--------|----------|
| D2-S15 | PrepareExecutionContext | LoadOrInit + Bootstrap + LongTerm recall + compress + SystemPromptAssembler.Build + Materialize (SubTurn/Wave) | **REGISTRY** | `prepare/` + `materialize/` |
| D2-S17 | PersistSessionState | Snappy 快照 + 双轨 transcript (主线程+ worker overlay) + commit window + deferred complete | **REGISTRY** | `persist/` + `kernel/spans.go` |
| D2-S18 | EnforceExecutionPolicy | `IPermissionGate.Request` 一次/调用 + `findSurface` 5 类 (Builtin/LSP/FreeFork/Tracker/Verify) + fallback ToolReg.Execute | **REGISTRY** | `enforce/` + `surface/` + `filter/` |
| D2-S16 | RunQueryLoop | ~~LLM↔Tool 执行原语 (QueryLLMCaller / query_loop.enabled)~~ | **REMOVED** | (D7-S2-A06 接管) |

---

## Architecture

> **D7 Leader ↔ D2 Follower 端到端 + 跨域边界 + 5 节点管道**，详见 `d7-boundary.md`（Leader/Follower 契约 + 跨域漂移清单 v2.0）。

```
D7 Leader (Orchestrator)
    └── PreparedTurnRunner → D7 RunTurn / SubTurn
            ├── D7-S2-A07 InvokeLLM → bridges/llm → D3 (D7 直调, D2 不参与)
            └── D7→D2 turn_adapter.ExecuteRound
                    ├── D2-S18 IPermissionGate.Request (1×/call)
                    ├── D2-S18 findSurface (5 Surface + 3 Filter chain)
                    └── D2-S18 fallback ToolReg.Execute (delegate_* → D4)

D2 Follower (Process glue)
    └── kernel.ContextEngine.Process
            ├── D2-S15 PrepareExecutionContext (LoadOrInit → Bootstrap → LongTerm → compress → Assembler → Materialize)
            ├── D2-S18 EnforceExecutionPolicy (per-tool ExecuteRound)
            └── D2-S17 PersistSessionState (Snappy snapshot + transcript + deferred complete)

D2 触发跨域:
    delegate_explore/plan/implement/status → D4 hubspoke.Dispatcher
    tracker.* → D5 LRU
    verify.* → D6
```

### 域边界

| D2 拥有 | D2 编排（不拥有） | D2 不拥有 |
|---------|------------------|----------|
| ContextEngine.Process | D7 RunTurn / SubTurn | LLM 调用（D3） |
| Prepare/ToolRound/Persist 三原语 | D4 Delegate RunAgent | Agent 生命周期（D4） |
| Tool Surface + Filter 列表 | — | IM 入口（D1） |
| EngineEvent 契约 | | |

**Hard Ban**：D2→D3 import 禁止（`internal/layers/contextengine/` 不允许 import `internal/layers/llmgateway/`，CI 硬阻断）。

---

## 关键 Scenario 范式

> **1 个 canonical Gherkin 示例**。完整 96 Scenario 分布在 `archive/<change>/specs/` 各 change 目录。

### 范式：S15 PrepareExecutionContext 三阶段编排（DSAFT S15）

#### Scenario: Materialize SubTurn 路径（D2-S16-A20 + D2-S16-A22）

- GIVEN a WorkItem with `SubTurn` policy and `brief` mode
- WHEN `ContextMaterializer.Materialize(ctx, sc, policy)` is called
- THEN execution order is `LoadBasePrompt` → `InjectSignals` → `LoadPrivateChain` → `Compress(token_budget)`
- AND `D2_Context_Materialize` span is emitted on completion
- AND the returned `PreparedContext` is consumed by D7-S2-A06 RunTurnLoop without reading `sc.Messages` directly
- AND `Compress` enforces `token_budget` via `MaxSubagentDepth=3` cap (Phase B)

---

## 关键链路口

1. **Leader → Follower 主链**：D7-S2-A06 RunTurnLoop → `turn_adapter.ExecuteRound` → D2-S18 `EnforceExecutionPolicy` → Tool 结果回灌 D7
2. **Prepare → Materialize 链**：D7-S16 Layer SubContext → D2-S16-A20 `ContextMaterializer.Materialize` → D7-S2-A07 InvokeLLM → D3
3. **Persist 链**：D7 Verify 完成 → D2-S17 `PersistSessionState` → Snappy snapshot + transcript 双轨 + deferred complete
4. **跨域消费**：D2-S18 `tracker.*` → D5 LRU / D2-S18 `verify.*` → D6 / D2-S18 `delegate_*` → D4 hubspoke
5. **Hard Ban 链**：D2→D3 import 路径 = 0（CI 硬阻断）；D7 独享 LLM Gateway 调用权
6. **Surface → Runner → 域 拓扑**：5 Surface (Builtin/LSP/FreeFork/Tracker/Verify) × 跨域 (D2内/D2+D4/D2+D5/D2+D6)，1 Surface = 1 跨域边界

---

## 附录：总览

- **当前活跃 Requirement 数**：0（已合入代码，需求态转为代码态）
- **当前活跃 Scenario 数**：1 canonical 范式（详细 96 个留 archive/）
- **历史 Scenario 详细文本**：96 个，分布在 `archive/<change>/specs/` 各 change 目录（详见 CHANGELOG.md）
- **当前 spec 版本**：v9.0.0
- **下一次架构级变更触发**：D7-D2 跨域 fixture 治理（`boundary-debt:cross-domain-fixtures-v9.0` 待定）