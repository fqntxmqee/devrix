# D1 ↔ D7 跨域边界规范

**Capability:** d1-d7-boundary
**Status:** Active
**Version:** 1.0.0
**Last Updated:** 2026-06-30
**Change ID:** devrix-d1-ac-restructuring
**Demand ID:** DM-20260629-005
**Parent (D1):** `openspec/specs/d1-communication/d1-domain.md`
**Parent (D7):** `openspec/specs/d7-orchestration/d7-domain.md`
**Related (D4):** `openspec/specs/d4-multi-agent/d7-boundary.md`
**Related (D2):** `openspec/specs/d2-context-engine/d7-boundary.md`

---

## 1. 关系摘要

| 域 | 角色 | 一句话 |
|----|------|--------|
| **D1** | Trusted Intermediary + **入站唯一入口** | 捕获 IM/CLI 用户指令 + 三类出站信号呈现 + EventBus Critical 必达 |
| **D7** | Orchestration Leader + **Hub-Spoke SoT** | 决定派哪个 Spoke、发布 FlowEvent、聚合 WorkPlan |

**ingress：** D1 → D7 `IOrchestrationEntry.ProcessMessage` only。D1 capture 是**唯一**用户入站路径。  
**egress：** D7 → D1 `IMOutboundSignal(Thinking/Task/Conclusion)` 经 EventBus fanout，D1 adapter 渲染。

---

## 2. Boundary Debt Decisions

> **治理常量位置：** `internal/layers/communication/orchtypes/boundary_decision.go`  
> **单测：** `internal/layers/communication/orchtypes/boundary_decision_test.go`（3 单测 PASS，DM-20260629-005 PR-1 落地）  
> **重新评估触发：** D1 域升级 v3.0+ 或 D7 编排 v7.0+ 跨域契约变化时。

| Boundary Decision ID | 描述 | SoT | Status | 解决路径 |
|----------------------|------|-----|--------|----------|
| `boundary-debt:d1-to-d7-orchestration-entry-v1.0` | D1 capture → D7 `IOrchestrationEntry.ProcessMessage` 唯一入口契约（v2.0+ 后 `routeLegacyD2` RETIRED，DM-007） | `d1-domain.md` §North Star / `cross-domain-boundaries.md` §2.5 | **RESOLVED** | 治理常量 + 启动期校验（`bootstrap/sessionagents`）+ lint-d1-imports CI 守门 |
| `boundary-debt:d1-to-d4-permission-gate-v1.0` | D1 → D4 `sessionagents/manager.ResolveAgentPermission`（CRITICAL 永不 YOLO） | `d1-domain.md` §Out of Scope + `d4-multi-agent/d7-boundary.md` §5 | **RESOLVED** | 治理常量 + DM-20260628-003 (devrix-d1-dsaft-refactor) 后 bootstrap 接线 |
| `boundary-debt:d1-forbidden-orchestration-import-v2.0` | D1 capture 生产代码禁止 import `multiagent` / `orchestration/*`（lint-d1-imports.sh CI 强制） | `d4-multi-agent/d7-boundary.md` §5.1 + DM-20260628-003 | **RESOLVED** | `scripts/lint-d1-imports.sh` 守门 + `capture/import_boundary_test.go` 单测 |

---

## 3. 调用链 SoT

```text
User IM/CLI
    └── D1.S17.ParseInbound (feishu/dingtalk/cli)
            └── D1.S13.AcceptInboundMessage + PersistUserTurn
                    └── D1.S13.DispatchToAgent (routeD7)
                            └── D7.IOrchestrationEntry.ProcessMessage
                                    ├── D7-S2 delegatetools / DispatchWorker
                                    │       ├── Spoke=D4Worker → D4.WorkerExecutor.Execute
                                    │       └── Spoke=D2SubQuery → D2.NestedExecutor.Run
                                    ├── D7-S4 SpokeBridge.Publish(FlowEvent)
                                    │       └── WorkPlan + executionflow + imsink
                                    └── EngineEvent 流 → D1.S18 EventBus
                                            ├── D1.S14 EmitThinkingDelta
                                            ├── D1.S15 EmitToolProgress / EmitWorkerProgress
                                            └── D1.S16 FinalizeReply (PublishCritical)
                                                    └── D1.S17 Encode* → IM adapter
```

**代码锚点（v2.0 实际）：**
- D1 capture: `internal/layers/communication/capture/{coordinator,ingress,dispatch,session}.go`
- D1 present: `internal/layers/communication/present/{thinking,taskprogress,conclusion}.go`
- D1 EventBus: `internal/layers/communication/delivery/eventbus/bus.go`
- D7 IOrchestrationEntry: `internal/layers/orchestration/bootstrap/sessionagents/manager.go`

---

## 4. 职责矩阵

| 能力 | D1 | D7 | D4 | D2 | D5 |
|------|----|----|----|----|-----|
| IM/CLI Parse | ✅ S17 | ❌ | ❌ | — | — |
| Session 持久化 | ✅ S13-A02 | ❌ | ❌ | — | — |
| 路由分发到 D7 | ✅ S13-A03 (routeD7) | 接收方 | ❌ | ❌ | — |
| Permission YOLO | UI/CRITICAL gate | — | 决策权 | — | — |
| Turn 主循环 / ClassifyIntent | ❌ | ✅ S2 / S5 | ❌ | — | — |
| Worker fork/run/join | ❌ | 派发 | ✅ S12/S13/S14 | ❌ | — |
| 三类出站信号呈现 | ✅ S14/S15/S16 | 数据源 | — | — | — |
| EngineEvent Critical 必达 | ✅ S18 (PublishCritical) | 订阅 | — | — | — |
| FlowEvent 发布 | ❌ | ✅ S4 | ❌ | ❌ | — |
| IM 出站渲染（飞书卡片 / 钉钉 / CLI） | ✅ S17 Encode | ❌ | ❌ | ❌ | — |
| 限流（adapter 隔离） | ✅ S17-A06 | ❌ | ❌ | — | — |
| D1 ↔ D7 跨域 metric | ✅ capture.persist span | ✅ SpokeBridge | — | — | ✅ SoT |

图例：✅ SoT · ❌ Out of Scope · — 无相关

---

## 5. 契约接口

| 接口 | 定义位置 | 实现 | 消费 |
|------|----------|------|------|
| `IOrchestrationEntry` | `shared/contracts` | `bootstrap/sessionagents/manager.go` | D1.S13-A03 routeD7 |
| `IMOutboundSignal` | `internal/layers/communication/present/` | D1 S14/S15/S16 Emit* | D1.S17 Encode* |
| `EngineEvent` | D7 编排产出 | D7.S4 SpokeBridge | D1.S18 EventBus |
| `WorkerStreamEvent` | `shared/contracts`（v2.0，DM-20260628-003） | D7 `wavescheduler/present.go` | D1 S15-A02 EmitWorkerProgress |

### 5.1 依赖规则

```text
✅ D1 → D7 (IOrchestrationEntry.ProcessMessage)
✅ D1 → D4 (ResolveAgentPermission via sessionagents/manager)
✅ D1 → shared/contracts, D5 observability
❌ D1 capture → D2 (IEngine.Process 直连, v2.0+ 后禁止；routeLegacyD2 RETIRED)
❌ D1 capture → D4 multiagent (DM-20260628-003 后禁止；lint-d1-imports.sh CI)
❌ D1 channel/adapters → orchestration/ (D7 import forbidden)
❌ D7 channel/adapters import (D7 owns orchestration, not IM adapter)
```

---

## 6. Canonical S 对照

| D1 Canonical S | 与 D7 关系 |
|----------------|-----------|
| D1-S13 CaptureUserIntent | ingress → D7 `ProcessMessage` |
| D1-S14 PresentThinking | egress ← D7 EngineEvent(Thinking) |
| D1-S15 PresentTaskProgress | egress ← D7 EngineEvent(tool/worker) + D4 WorkerEvent (via WorkerStreamEvent DTO) |
| D1-S16 DeliverConclusion | egress ← D7 EngineEvent(complete/error) + PublishCritical |
| D1-S17 ConnectChannel | 横切 — 多 IM 接入与编解码 |
| D1-S18 GuaranteeDelivery | 横切 — EventBus Critical 路径（D7 EngineEvent 流 → D1 出站） |

| D7 Canonical S | 与 D1 关系 |
|----------------|-----------|
| D7-S2 Session Orchestrator | 接收 D1 入站 + beforeDispatch hook |
| D7-S4 Execution Flow | EngineEvent 流 → D1 EventBus 消费 |
| D7-S5 Decision & Planning | ClassifyIntent 后处理 D1 入站 turn |

---

## 7. 跨域迁移表（v2.0）

| # | 路径 | 行为 | 目标 | Status |
|---|------|------|------|--------|
| 1 | `capture/agent_route.go` (D1-S1-A04) | AgentFactory 路由 | `bootstrap/sessionagents` (D7 owns) | RESOLVED (DM-20260628-003) |
| 2 | `gateway/routeLegacyD2` (D1-S13-A03-F01) | IEngine.Process 直连 | 删除 — routeD7 唯一 | RESOLVED (DM-007 + DM-20260614-007) |
| 3 | `capture/event_dispatcher.go` (D1-S9-A01) | Event dispatch | `capture/dispatch.go` + `delivery/eventbus/` | RESOLVED (DM-20260614-006) |
| 4 | `adapter.WorkerEvent` (D7 `wavescheduler.WorkerEvent` 直引) | Worker 卡 DTO | `contracts.WorkerStreamEvent` | RESOLVED (DM-20260628-003) |
| 5 | `capture.IContextEngine` alias | D2 跨层契约 | 删除 — 用 `contracts.IEngine` | RESOLVED (DM-20260628-003) |

---

## 8. 影子编排风险（双边共识）

| 风险路径 | 影子编排方式 | 检测方式 | 缓解 |
|---------|------------|---------|------|
| **EngineEvent 字面量漂移** | D1 capture 消费 EngineEvent 类型时硬编码 string（"thinking"/"tool"/"complete"） | D1 capture 用 const switch 而非字符串比较 | `orchtypes/events.go` 治理常量 + 测试覆盖 |
| **EventBus Critical 路径绕过** | D1 capture 直接调 IM adapter，跳过 PublishCritical | `d1.dispatch.route` span attribute + Critical path lint | EventBus D1.S18 必走 PublishCritical，`delivery/eventbus/bus_test.go` 覆盖 |
| **Session 状态污染** | D1 capture 接收跨 turn orphan EngineEvent | `bootstrap/sessionagents` DeliverOrphanEngineEvent sink | sessionagents D1-RF-T04 测试 |
| **Permission 决策漂移** | D1 capture 改 YOLO 风险等级 | `d1.capture.persist` span attribute risk_level + D4 ResolveAgentPermission audit | CRITICAL 永不 YOLO（v2.0 强约束） |
| **D7 import 反向渗透** | D1 capture 新代码误 import `orchestration/*` | `scripts/lint-d1-imports.sh` CI | CI 守门 + `capture/import_boundary_test.go` |

---

## 9. Follower 对称性声明（与 D4 一致）

> D2 和 D4 作为 Stackelberg Follower 享有对等角色约束（D4 `d7-boundary.md` §10）。D1 不是 Follower，是 **Trusted Intermediary**——与 D7 是横向协作而非纵向追随。

| 对称轴 | D7 Orchestration Leader | D1 Trusted Intermediary |
|--------|------------------------|--------------------------|
| 拥有决策权 | 编排（Hub-Spoke） | 入站路由 + 出站呈现 + Critical 必达 |
| 直接发布 Event | ✅ FlowEvent Publish | ✅ IM Outbound + EventBus Critical |
| 不被另一方反向调用 | ❌ 接收 D1 入站 | ❌ 接收 D7 EngineEvent |
| 硬约束 | 不直接 import D1 channel/adapters | 不直接 import D7 orchestration |

---

## 10. Boundary Decision 治理常量

```go
// internal/layers/communication/orchtypes/boundary_decision.go
package orchtypes

const (
    BoundaryD1ToD7OrchestrationEntry      = "boundary-debt:d1-to-d7-orchestration-entry-v1.0"
    BoundaryD1ToD4PermissionGate          = "boundary-debt:d1-to-d4-permission-gate-v1.0"
    BoundaryD1ForbiddenOrchestrationImport = "boundary-debt:d1-forbidden-orchestration-import-v2.0"
)

func AllBoundaryDecisions() [3]string { ... }
```

**单测覆盖（DM-20260629-005 PR-1 orchtypes-bootstrap 落地）：**

```go
// internal/layers/communication/orchtypes/boundary_decision_test.go
func TestBoundaryDecisions_Exist(t *testing.T)       { /* 3 ID 存在 */ }
func TestBoundaryDecisions_VersionFormat(t *testing.T) { /* regex ^boundary-debt:[a-z0-9\-]+-v\d+\.\d+$ */ }
func TestBoundaryDecisions_Unique(t *testing.T)      { /* 3 ID 唯一 */ }
```

**3/3 PASS**（`go test ./internal/layers/communication/orchtypes/... -race -count=1`）。

---

## 11. Revision History

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-06-30 | **DM-20260629-005 PR-7 #5 boundary-decision**：初版 — D1 是 D7 入站唯一入口 + Boundary Debt Decisions 3 row 治理表 + 调用链 SoT + 职责矩阵 + 契约接口 + 依赖规则 + Canonical S 对照 + 跨域迁移表（5 条 RESOLVED）+ 影子编排风险（5 路径）+ Follower 对称性声明 + 治理常量 + 3 单测 PASS |