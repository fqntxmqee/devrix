# Demand: devrix-d4-dsaft-restructuring (DM-20260629-004)

**Demand ID:** DM-20260629-004
**Status:** S1_Demand
**Priority:** P0 (深度架构重构)
**Created:** 2026-06-30
**Change ID:** devrix-d4-dsaft-restructuring
**Triggered By:** D4 域整体 DSAFT 方法论 Review（2026-06-29 会话）+ 对齐 D7 v6.0.x → v7.0 (DM-20260629-001) + D2 DSAFT (DM-20260629-002) + D3 DSAFT (DM-20260629-003) 6 子 Change 模板
**Related:**
- `devrix-d7-dsaft-restructuring` (DM-20260629-001) — D7 6 子 Change 联动模板 (S7_Archived 2026-06-29)
- `devrix-d2-dsaft-restructuring` (DM-20260629-002) — D2 6 子 Change 联动 (S7_Archived 2026-06-29)
- `devrix-d3-dsaft-restructuring` (DM-20260629-003) — D3 6 子 Change 联动 (S7_Archived 2026-06-29)
- `devrix-d4-sa-refine` (DM-20260614-018) — D4 S/A 重切 v1.0 → v2.0 (S7_Archived 2026-06-15)
- `docs/methodology/dsaft-methodology.md` v4.0.0 — 6 原则
- `docs/methodology/dsaft-refactoring-playbook.md` v1.0.0 — 4 轴 / 6 阶段

---

## §1 背景

D4 v1.0.0（2026-06-15 d4-domain.md）已达"9 子包 / 4108 LOC / 6 A / 18 F / 77 T / 6 active span ops（OpD4_S4_*）+ 7 EngineEvent 字面量" 的稳定 SoT 状态。但 2026-06-29 DSAFT Review 暴露 **5 类深度架构债**，对齐 D7 v6.0.x → v7.0 + D2/D3 DSAFT (DM-20260629-002/003) 联动模板需要 6 子 Change 联动 refactoring。

### 1.1 1 类 god function 集中 external/ (P0)

| 文件:函数 | LOC | S | 风险等级 |
|---|---|---|---|
| `external/cli_adapter.go` | 466 | S15 | High（CLI session lifecycle + execute + 7 helper methods） |
| `external/cursor_adapter.go` | 410 | S15 | High（cursor session lifecycle + streaming + 6 handler methods） |

### 1.2 F 路径漂移 18 处（P0, v2.0-d 已迁代码但 f-registry 未跟）

- D4-S11 ProvisionAgent: `agent/agent.go` + `factory/factory.go` → `provision/factory.go`
- D4-S12 RunAgentLoop: `agent/lifecycle.go` + `agent/agent.go` + `agent/perm_gate.go` → `run/{lifecycle,agent,perm_gate}.go`
- D4-S13 IsolateAndMerge: `agent/forkjoin.go` + `sessionview/sessionview.go` + `agent/worker_engine.go` → `isolate/{forkjoin,sessionview,worker_engine}.go`
- D4-S15 InvokeExternalAgent: `tool/{registry,cli_adapter,cursor_adapter,stream_json}.go` → `external/{session,execute,...}.go`
- D4-S5 kernel: `contracts.go` + `observer/noop.go` → `kernel/{contracts,observer/noop}.go`

### 1.3 ValueFlow Alias 缺失 (P1)

`d4-domain.md §North Star` 表**缺 ValueFlow Alias 列**，对齐 D2 v9.0.0 + D7 v2.6.0 + D3 v1.6.0 §North Star ValueFlow Alias 模式。

5 canonical S + 1 横切 = 6 ValueFlow Alias 待加：
- S11 ProvisionAgent → `D4_Provision_Agent`
- S12 RunAgentLoop → `D4_Run_Agent_Loop`
- S13 IsolateAndMerge → `D4_Isolate_Merge`
- S14 ExecuteWorker → `D4_Execute_Worker`
- S15 InvokeExternalAgent → `D4_External_Agent_Tool`
- S16 ConfigureAgents → `D4_Configure_Agents`（横切）

### 1.4 Span Evidence 缺失 (P1)

D4 现状 6 active span ops（OpD4_S4_Agent_Run/Tool_Call/Fork/Join/Terminate/State_Transition） + 7 EngineEvent 字面量（`agent.started/error/terminated/iterating/forked/joined` + `permission_required`）**双轨 SoT，无单一治理**：

- t-registry 77 T 行**无 Span Evidence 列**（D3 已加、D2 已加、D7 已加）
- 7 EngineEvent 字面量在 `run/lifecycle.go::emit()` 5 处 + `run/forkjoin.go::emit()` 2 处
- `agent_bridge.go` (D7 orchestration) `L142-154` 6 case switch 走字面量
- `evolution/guard/observer.go` `L52-86` 2 case switch 走字面量
- 期望覆盖率：≥80% effective（60+/77 T 映射到 6 OpD4_S4_* 或 7 EventAgent* const）

### 1.5 Boundary Debt 3 项未治理 (P0)

D4 跨域边界债**登记缺失**（对齐 D2/D3/D7 4-Constant 模板）：

- `D4 emit agent.{started,error,terminated,iterating,forked,joined} → D7 FlowEvent` — 常量化后稳定
- `D4 emit agent.{forked,joined} + permission_required → D6 evolution/guard/observer` — 常量化后稳定
- `D4 forbidden flow.Hub.Publish` (D7 v2.0-b 后 lint 强制) — 经 PR-6 常量化后 governance

### 1.6 Root Re-export Shim 待退役 (P2)

`internal/layers/multiagent/contracts.go` 47 LOC re-export `kernel.*` 类型，被 `sessionagents/manager.go` + 5+ 测试文件依赖。Spec v2.0-e 标记清理（不强制立即删，渐进迁移）。

### 1.7 Historical S 强残留 (P2)

D4-S1~S10 已退役但仍写进 4 spec 文件 (`a-registry.md`/`f-registry.md`/`t-registry.md`/`d4-domain.md`) 6 个 section 章节，代码已 0 caller。需下沉到 `openspec/archive/2026-06-15-devrix-d4-sa-refine/legacy-s1-s10.md`。

---

## §2 治理目标

D4 域从 v1.0.0 演进到 v1.5.0（v1.1-1.4 5 步），对齐 D2 v9.0.0 / D3 v1.6.0 / D7 v6.0.0 模板：

```
v1.1 orchtypes 包建立 (PR-1)
  + spans.go move
  + events.go + boundary_decision.go 前置治理常量
  + WorkerEngine inline (~44 LOC)
  + ExecutorMetricsSnapshot/ForkerMetricsSnapshot 类型清理
v1.2 god-fn-split pt1 (PR-2)
  + external/cli_adapter.go 466→2 文件 (session + execute)
v1.2 god-fn-split pt2 (PR-3)
  + external/cursor_adapter.go 410→2 文件 (session + execute)
v1.3 registry-sync (PR-4)
  + 18 F 路径全替换
  + Historical S 沉 archive (legacy-s1-s10.md)
v1.4 value-flow-rename (PR-5)
  + 6 ValueFlow Alias (D4_* 前缀)
  + a/f/t/layer-delta 同步
v1.4 span-coverage (PR-6)
  + 7 EngineEvent 常量化 → const switch
  + t-registry 77 T 行加 Span Evidence 列
  + d4-span-coverage.sh CI 守门 ≥80%
v1.5 boundary-decision (PR-7)
  + 3 boundary debt 全部 RESOLVED
  + orchtypes/boundary_decision.go 治理常量
  + 3 单元测试
```

---

## §3 验收目标

| 维度 | 现状 (v1.0.0) | 目标 (v1.5.0) | 验证 |
|------|--------------|--------------|------|
| 子包数 | 9 + 1 root shim | 10 (含 orchtypes) | `ls internal/layers/multiagent/` |
| LOC | 4108 | ~4200（god fn 拆 +120 + 治理常量 -100） | `wc -l` 守门 |
| 死代码 | 15 exported claim → 实际 8 (WorkerEngine/Creator/2 Snapshot) | 0 | `grep -rE "dead-symbol" internal/` |
| God fn | 2 (cli_adapter + cursor_adapter) | 4 文件拆解 | `wc -l external/*_adapter*.go` |
| F 路径漂移 | 18 处 | 0 | `grep -E "f-registry.md" path-mismatch` |
| ValueFlow Alias | 0 | 6 | grep `D4_*_Agent` |
| Span Coverage | 0% (无 evidence) | ≥80% effective | `scripts/d4-span-coverage.sh` |
| Boundary Debt | 3 未登记 | 3 RESOLVED | `grep "RESOLVED" d4-domain.md` |
| d4-domain Version | 1.0.0 | 1.5.0 | git diff |
| T-registry | 77 行无 Span Evidence | 77 行全标注 | `grep "Span Evidence"` |

---

## §4 风险与缓解

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| contracts.go 退役破坏 D4 跨包引用 | High | High | PR-1 不删，PR-4 + PR-5 渐进迁移；新增 `orchtypes/contracts.go` 作为新 canonical |
| 18 F 路径漂移破坏 D3/D5/D7 跨包引用 | Mid | High | grep 0 漂移守门；`go test ./internal/... -race` 全量 PASS |
| 7 EngineEvent 字面量常量化破坏 D7/D6 consumer | Mid | Mid | 同步迁移 consumer（agent_bridge.go + observer.go）+ 端到端 race 守门 |
| Historical S 沉 archive 丢失追溯信息 | Low | Mid | archive dir 已存在 (2026-06-15-devrix-d4-sa-refine/)；新文件 `legacy-s1-s10.md` 含 frozen index |
| cli_adapter / cursor_adapter 拆分破坏 streaming protocol | Mid | High | 拆前后 `-race` PASS 守门；端到端 cursor/CLI 模拟测试 |
| 8 子 Change 跨 7-9 天 baseline 不稳 | Mid | Mid | 每日 1 PR squash auto-merge；CI gate 即时验证 |

---

## §5 与 D7/D2/D3 模板对齐

| 维度 | D7 (DM-001) | D2 (DM-002) | D3 (DM-003) | D4 (本 DM) |
|------|------------|------------|------------|------------|
| LOC | ~15000+ | ~9000 | 4826 | **4108** |
| PR | 11 | 8 | 8 | **8** |
| T | 55 | 44 | 40 | **~34** |
| Span Coverage | 94% | 88% | 100% | **≥80%** |
| 死代码 | ~775 LOC | ~1298 LOC | 0 (审计) | **8 exported + orchtypes 化** |
| God fn | 4 个拆 4 文件 | 5 个拆 5 文件 | 4 个拆 4 文件 | **2 个拆 4 文件** |
| Boundary Debt | 3 RESOLVED | 2 (1+1 待定) | 4 RESOLVED | **3 RESOLVED** |

---

**END of Demand**
