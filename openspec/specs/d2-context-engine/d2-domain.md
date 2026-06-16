# D2 Context Engine Domain

**Domain ID:** D2
**Slug:** `contextengine`
**Type:** Core Domain
**Status:** Active — Canonical S15–S20 (v1.0 registry, DM-20260614-009)
**Version:** 6.1.0
**Last Updated:** 2026-06-15
**Depends On:** ~~D3 (ILLMGateway)~~ → **D7 消费（DM-020）**, D5 (Observability), **D7 (invocation only — Leader)**
**Hard Ban:** D2→D3 import 禁止（DM-020 v1.0 Registry，v2.0-d CI 硬阻断）
**Depended By:** D1 (EngineEvent consumer), **D7 (QueryLoopExecutor consumer)**
**Cross-Domain SoT:** `d7-boundary.md`

---

## North Star

**在会话边界内，可靠地准备上下文、执行工具轮次，并持久化会话状态——作为被 D7 调度的纯执行原语（Context Follower）。D7 拥有 LLM 调用权与 Turn 主循环；D2 仅负责 Prepare / ToolRound / Persist。**

| 可验证承诺 | Canonical S |
|-----------|-------------|
| Turn 前上下文合法、在预算内 | D2-S15 PrepareExecutionContext |
| LLM↔Tool 多轮（Legacy 冻结 → D7-S2-A06） | ~~D2-S16 RunQueryLoop~~ |
| Tool 权限/沙箱先于执行 | D2-S18 EnforceExecutionPolicy |
| Turn 后状态 durable + deferred complete | D2-S17 PersistSessionState |
| 嵌套 SubQuery/Background 有边界 | D2-S19 NestedExecution |
| Legacy Harness 仅显式配置 | D2-S20 LegacyHarnessFallback |

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
| D2-S16 | RunQueryLoop | LLM↔Tool 执行原语（Thin Loop） | **LEGACY FREEZE（DM-020）** |
| D2-S17 | PersistSessionState | Snapshot, transcript, commit window | REGISTRY |
| D2-S18 | EnforceExecutionPolicy | Permission, sandbox, tool surface | **REGISTRY（自 S16 拆出 tool 面，DM-020）** |
| D2-S19 | NestedExecution | SubQuery, background, fork, sidechain | REGISTRY |
| D2-S20 | LegacyHarnessFallback | `query_loop.enabled=false` 路径 | REGISTRY |

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
| D2-S9 | Harness | Legacy | → S20 |
| D2-S10 | QueryLoop | Legacy | → S16, S18, S19 |
| D2-S11 | Queue | Legacy | → **D7-S4** |
| D2-S12 | Worktree | Legacy | → S18 |
| D2-S13 | Conversation | Legacy | → S15 |
| D2-S14 | Mock | Legacy | 测试辅助 |

> **Change:** `openspec/archive/2026-06-14-devrix-d2-sa-refine/` (DM-20260614-009)

---

## 与 D7 关系（Leader / Follower）

> 完整矩阵见 [`d7-boundary.md`](./d7-boundary.md)。

### 角色

| | D7 | D2 |
|---|----|----|
| Stackelberg | **Leader** — 先选路径与 Executor | **Follower** — 后执行 Loop |
| 回答 | 做什么、顺序、谁做、进度广播 | 上下文如何准备、Loop 如何跑、状态如何持久化 |
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
| FastPath | S15→S16→S17 |
| Wave Worker (D2) | S16 + S18 per worker |
| SubQuery / Background | S19 |
| PlanMode（决策在 D7-S5） | S18 机制执行 |

### 注入点（D7 → D2，非 D2 编排）

| 注入 | 用途 | v2.0 |
|------|------|------|
| `QueryRequest` | session、message、system | 保持 |
| `LoopHooks` | 编排回调 | D7 定义策略 |
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
v2.0 目标（DM-020）:
  wire_coordinator.go: ContextPreparer / ToolRoundExecutor / SessionPersister → D2
  D7-S2-A07 InvokeLLM → bridges/llm → D3（D7 直调，D2 不参与）

v1.0 Legacy（现行）:
  wire_coordinator.go: d2Executor.RunQueryLoop → engine.Process
```

D7 注入 `LoopHooks`、`SessionQueue`；D2 **不** import `orchestration` 包。
**D2→D3 import 禁止（DM-020 v1.0 Registry，v2.0-d CI 硬阻断）。**

---

## 实现状态（2026-06-15）

| 项 | 状态 |
|----|------|
| QueryLoop 主路径 | ✅ IMPLEMENTED |
| Per-turn compression | ✅ IMPLEMENTED |
| Deferred complete | ✅ IMPLEMENTED |
| D2 Thin（query 无 orchestration/multiagent） | ✅ IMPLEMENTED | `d2_thin_test.go` |
| D7 ingress（capture 无 contextengine） | ✅ IMPLEMENTED | `d7_boundary_test.go` |
| tasks/ 归 D7 | ✅ 已迁入 `orchestration/workmodel/`（DM-20260614-009 v1.1 closure） |
| ~~delegate_tools 移除~~ | ✅ `orchestration/delegatetools/` (DM-011) |
| scenario 物理路径 S15/S17/S18 | ⬜ v2.0 |
| **D2-S16 Legacy Freeze（→ D7-S2-A06）** | ✅ v1.0 Registry（DM-020, commit 41aec47） |
| **D2→D3 import lint（CI 硬阻断）** | ✅ v2.0-d（DM-020, `lint/layer::TestD2_D3Ban` 4 whitelist = 4 fallback 路径） |
| **S18 ExecuteToolRound 拆面** | ✅ v2.0-d（DM-020, D7 → `ContextPreparer` / `ToolRoundExecutor` / `SessionPersister`） |

---

## 修订记录

| Version | Date | Changes |
|---------|------|---------|
| 6.1.0 | 2026-06-15 | DM-020 拆面闭合状态同步：实现状态表 3 项 ⬜ PLANNED → ✅ IMPLEMENTED（D2-S16 Legacy Freeze / D2→D3 import lint / S18 ExecuteToolRound 拆面），引用 commit 41aec47 与 `TestD2_D3Ban` 实测通过 |
| 6.2.0 | 2026-06-15 | **D7 Real-Closure Spec Sync (D2 侧)**：(1) 实现状态表 `tasks/ 归 D7` ⬜ v2.0 → ✅ 已迁入 `orchestration/workmodel/`（DM-20260614-009 v1.1 closure，commit 41aec47 后）；(2) Out of Scope 表 `Task 写模型 / PlanMode` 备注从 `代码暂在 tasks/` 改为 `✅ 已迁入 orchestration/workmodel/` |

---

## 相关文档

| 文档 | 用途 |
|------|------|
| `spec.md` | Gherkin 验收规格 |
| `design.md` | 六段式架构设计 |
| `a-registry.md` / `f-registry.md` / `t-registry.md` | A/F/T 注册表 |
| `d7-boundary.md` | **D2↔D7 跨域 SoT** |
| `../d1-communication/d1-domain.md` | D1 入口与展示域 SoT |
| `layer-delta.md` | Delta SoT |
| `docs/methodology/dsaft-refactoring-playbook.md` | 重构方法论 |
