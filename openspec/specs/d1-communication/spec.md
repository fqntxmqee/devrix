# D1 Communication Domain Specification

**Capability:** communication
**Status:** Active
**Version:** 6.0.0
**Last Updated:** 2026-06-30 (DM-20260629-005 d1-ac-restructuring v6.0.0 S7_Archived)
**Domain SoT:** `d1-domain.md` v1.2.0 — North Star / 6 ValueFlow / DSAFT 资产 / 边界 SoT
**D7 Boundary:** `d7-boundary.md` v1.2.0 — D1↔D7 跨域边界规范 + Boundary Debt Decisions

> **精简设计契约（Lite-Mode）**：本文档只放当前符合代码的设计契约（v6.0.0）。**过程需求迭代**（90 个 Gherkin Scenario 详细文本：happy 30 / sad 24 / boundary 18 / concurrent 9 / timeout 9）不进入本文件，留在 `archive/<change-id>/specs/` 各 change 归档目录。详细时间线见 [CHANGELOG.md](CHANGELOG.md)。

---

## Overview

D1 通信域是 **Trusted Intermediary**（可信送达 + 客观锚点）。负责 IM 双向对话：用户指令捕获（S13）、三类出站信号呈现（S14–S16）、多平台通道（S17）、弱网必达（S18）。作为 Devrix 入口层，所有用户交互经由此域进入系统，**D1→D7 是唯一编排入口**（`IOrchestrationEntry.ProcessMessage`）。

**现行实现路径（v6.0.0）**：`CommunicationGateway` 在适配器与 Context Engine 之间路由消息，适配器实现 `EventHandler` 接口并使用 `GatewayAPI`；D1 capture 通过 `IOrchestrationEntry` 委托 D7 编排，**禁止 D1 capture import `multiagent` / `orchestration/*`**（`lint-d1-imports.sh` CI 守门）。

| ValueFlow | Canonical S | 职责 |
|-----------|-------------|------|
| `D1_Capture_User_Intent` | D1-S13 CaptureUserIntent | 指令不丢、可追、可续聊 |
| `D1_Present_Thinking` | D1-S14 PresentThinking | 思考过程可见（信号① Costly） |
| `D1_Present_Task_Progress` | D1-S15 PresentTaskProgress | 任务/工具/Worker 进度可见（信号②） |
| `D1_Deliver_Conclusion` | D1-S16 DeliverConclusion | 结论/错误必达用户（信号③ Costly） |
| `D1_Connect_Channel` | D1-S17 ConnectChannel | 多 IM 平台结构一致 |
| `D1_Guarantee_Delivery` | D1-S18 GuaranteeDelivery | 背压/弱网下 Critical 不丢 |

### 信号分层博弈论（切法 A）

基于 `devrix-d1-sa-refine` (DM-20260614-006)。D1 信号体系映射到博弈论 4 概念：

| 概念 | 定义 | D1 对应 |
|------|------|---------|
| **Separating Equilibrium** | 不同类型发送者选择不同信号 | D7-S5 ClassifyIntent（D1 仅 Dispatch） |
| **Costly Signal** | 信号发送者承担真实成本，不可伪造 | D1-S14 Thinking · D1-S16 Conclusion |
| **Commitment Device** | 发送者通过信号约束自身未来行为 | D1-S18 Critical 必达 · D1-S16 complete/error |
| **Screening Mechanism** | 接收者设计契约让发送者自我选择 | D1-S13-A04 Permission YOLO |

**禁止约束**：禁止伪造进度信号（anti-fabrication commitment device）。

### 核心设计原则

1. **Trusted Intermediary**（D1 = 客观锚点）：可信送达 + 不可伪造进度信号；不拥有编排、推理与执行
2. **唯一编排入口**：D1 capture → D7 `IOrchestrationEntry.ProcessMessage`（Hard Ban：D1→D2 直连 `IEngine.Process` 退役）
3. **三类出站信号分层**（博弈论 Costly Signal）：D1-S14 Thinking（信号①）/ S15 TaskProgress（信号②）/ S16 Conclusion（信号③）
4. **EventBus 5 态状态机**：`Drain → Compact → Reconnect` 生命周期，5 种状态 (Running/Draining/Compacting/Reconnecting/Closed)；Critical 事件通过 `PublishCritical` 绕过常规通道直接扇出（P0 送达保证）
5. **Permission + YOLO 模式**：权限请求根据风险等级自动审批，CRITICAL 风险永不自动审批（Screening Mechanism 落地）
6. **Card 系统跨平台**：平台无关的 `core.Card` 模型 + `CardBuilder` 链式 API，渲染为平台特定的 JSON/Markdown；飞书适配器支持元素级流式（CardKit 打字机）
7. **Session 持久化**：`FileSessionStore` 原子写入（临时文件 + rename）+ `ResolveSessionByChatID` 重启后恢复
8. **Hard Ban 跨域**：D1 capture 生产代码禁止 import `multiagent` / `orchestration/*`（`lint-d1-imports.sh` CI 守门）

### S 层职责（canonical S13-S18）

| S ID | Scenario | 职责 | Status |
|------|----------|------|--------|
| D1-S13 | CaptureUserIntent | 入站：用户指令捕获 + 校验 + 持久化 | **REGISTRY** |
| D1-S14 | PresentThinking | 出站信号①：思考过程可见 | **REGISTRY** |
| D1-S15 | PresentTaskProgress | 出站信号②：任务/工具/Worker 进度 | **REGISTRY** |
| D1-S16 | DeliverConclusion | 出站信号③：结论/错误必达 | **REGISTRY** |
| D1-S17 | ConnectChannel | 多 IM 接入与编解码（飞书/钉钉/企微） | **REGISTRY** |
| D1-S18 | GuaranteeDelivery | 弱网/背压下 Critical 不丢 | **REGISTRY** |
| D1-S1..S12 | (legacy module index) | RETIRED v2.0（DM-20260614-006）→ `archive/2026-06-30-devrix-d1-ac-restructuring/legacy-s1-s12.md` |

---

## DSAFT 结构

| 层级 | ID | 名称 | 物理路径 |
|------|-----|------|----------|
| D | D1 | Communication | `internal/layers/communication/` |
| S | D1-S13 | Capture User Intent | `capture/` (AcceptInboundMessage + PersistUserTurn) |
| S | D1-S14 | Present Thinking | `adapters/` + `cardkit/` |
| S | D1-S15 | Present Task Progress | `adapters/` + `eventbus/` (Worker 展示经 `contracts.WorkerStreamEvent`) |
| S | D1-S16 | Deliver Conclusion | `adapters/` + `cardkit/` (conclusion.EmitComplete) |
| S | D1-S17 | Connect Channel | `adapters/feishu/` + `adapters/dingtalk/` + `adapters/wecom/` |
| S | D1-S18 | Guarantee Delivery | `eventbus/` (5 态状态机 + PublishCritical) |
| A | A1-A99 | 16 个核心活动 | 见 `a-registry.md` |
| F | F1-F999 | 18 个功能点 | 见 `f-registry.md` |
| T | T1-T200 | 74 个测试点（42 P0） | 见 `t-registry.md` |

**当前计数（v6.0.0）**：D=1, S=6 (canonical: S13-S18) + S=12 (legacy tombstone), A=16, F=18, T=74 (42 P0), Span=22 ops。

---

## Scenarios

| ID | Scenario | Responsibility | Status | 代码位置 |
|----|----------|----------------|--------|----------|
| D1-S13 | CaptureUserIntent | AcceptInboundMessage 校验 + PersistUserTurn + ResolveSessionByChatID | **REGISTRY** | `capture/` + `sessionstore/` |
| D1-S14 | PresentThinking | 流式 Thinking 事件 + Card 元素级渲染 | **REGISTRY** | `adapters/` + `cardkit/` |
| D1-S15 | PresentTaskProgress | Worker 进度经 `contracts.WorkerStreamEvent` 透传 | **REGISTRY** | `eventbus/` + `adapters/` |
| D1-S16 | DeliverConclusion | conclusion.EmitComplete + UpdateCard/FinalizeReplyCard | **REGISTRY** | `conclusion/` + `adapters/feishu/` |
| D1-S17 | ConnectChannel | 飞书/钉钉/企微 多平台编解码 | **REGISTRY** | `adapters/{feishu,dingtalk,wecom}/` |
| D1-S18 | GuaranteeDelivery | EventBus 5 态 + Critical 通道扇出 | **REGISTRY** | `eventbus/` |

---

## Architecture

> **D1↔D7 跨域边界 + Boundary Debt Decisions + 调用链 SoT + 职责矩阵**，详见 `d7-boundary.md` v1.2.0（DM-20260629-005 PR-7 #5 boundary-decision）。

```
IM Adapter (Feishu/DingTalk/WeCom)
    └── CommunicationGateway (composition root 接线)
            ├── D1-S13 AcceptInboundMessage 校验
            │   └→ D1 capture (Hard Ban: 禁止 import multiagent/orchestration)
            │       └→ D7 IOrchestrationEntry.ProcessMessage 唯一入口
            │           ├→ D7 ClassifyIntent
            │           ├→ D7 RunTurnLoop → D3 LLM Gateway + D2 ContextEngine
            │           └→ D7 SessionOrchestrator → D4 Worker / D2 SubQuery
            │
            ├── D1-S14/15/16 出站信号呈现
            │   └→ D7 EngineEvent 流 → 适配器渲染
            │       ├→ D1-S14 Thinking (信号①)
            │       ├→ D1-S15 TaskProgress (信号②, 经 WorkerStreamEvent)
            │       └→ D1-S16 Conclusion (信号③)
            │
            └── D1-S18 GuaranteeDelivery
                └→ EventBus 5 态 + PublishCritical 扇出
```

### 域边界

| D1 拥有 | D1 编排（不拥有） | D1 不拥有 |
|---------|------------------|----------|
| IM 适配器（飞书/钉钉/企微） | D7 ProcessMessage / ClassifyIntent | Turn 主循环（D7） |
| EventBus + Card 系统 | D2 Context Engine | 上下文准备（D2） |
| PermissionManager + YOLO | D4 Worker | LLM 调用（D3） |
| Session 持久化 | D6 信誉 | 编排/推理/执行 |

**Hard Ban**：
- D1→D2 直连 `IEngine.Process` 退役（DM-007 `routeLegacyD2` RETIRED）
- D1 capture 禁止 import `multiagent` / `orchestration/*`（`lint-d1-imports.sh` CI 守门，DM-20260628-003 落地）

**Boundary Debt**（3 项 RESOLVED，治理常量 in `orchtypes/boundary_decision.go`）：
- `boundary-debt:d1-to-d7-orchestration-entry-v1.0` — D1→D7 唯一入口契约
- `boundary-debt:d1-to-d4-permission-gate-v1.0` — D1→D4 Permission Manager（CRITICAL 永不 YOLO）
- `boundary-debt:d1-forbidden-orchestration-import-v2.0` — D1 capture 禁止 import orchestration

---

## 关键 Scenario 范式

> **1 个 canonical Gherkin 示例**。完整 90 个 Scenario 分布在 `archive/<change>/specs/` 各 change 目录。

### 范式：S13 CaptureUserIntent happy 路径（DSAFT S13-A02/A03）

#### Scenario: 入站飞书消息持久化成功（happy）

- **GIVEN** 飞书 IM 收到用户非空文本消息
- **WHEN** AcceptInboundMessage 校验通过 + PersistUserTurn 写 session
- **THEN** session.lastMessageAt 更新且 turn 可追溯
- **AND** D7 `IOrchestrationEntry.ProcessMessage` 被调用（唯一入口契约）
- **AND** 适配器收到 D7 EngineEvent 流（D1-S14 Thinking / D1-S15 TaskProgress / D1-S16 Conclusion 顺序呈现）

---

## 关键链路口

1. **入口主链**：User → IM Adapter (飞书/钉钉/企微) → `CommunicationGateway.RouteInbound` → D1-S13 AcceptInboundMessage → D7 `IOrchestrationEntry.ProcessMessage` 唯一入口
2. **出站信号链**：D7 RunTurnLoop → D7 EngineEvent 流 → D1 adapter 渲染（D1-S14 Thinking 流式 + D1-S15 TaskProgress 进度 + D1-S16 Conclusion 必达）
3. **Critical 必达链**：D1-S18 EventBus 5 态 + `PublishCritical` 扇出；Drain/Compact/Reconnect 生命周期；背压/弱网下结论必达
4. **跨域消费**：D1 → D4 `sessionagents/manager.ResolveAgentPermission`（CRITICAL 永不 YOLO）+ D1 → D5 Observability（tracer / telemetry）+ D7 → D1 适配器（EngineEvent 消费方展示）
5. **Session 持久化链**：D1 `FileSessionStore` 原子写入（tmp + rename）+ `ResolveSessionByChatID` 重启后恢复
6. **Hard Ban 链**：D1→D2 直连 `IEngine.Process` = 0（`routeLegacyD2` RETIRED）；D1→D7 是唯一编排入口契约（`boundary-debt:d1-to-d7-orchestration-entry-v1.0`）

---

## 附录：总览

- **当前活跃 Requirement 数**：0（已合入代码，需求态转为代码态）
- **当前活跃 Scenario 数**：1 canonical 范式（详细 90 个留 archive/，分布：happy 30 / sad 24 / boundary 18 / concurrent 9 / timeout 9）
- **历史 Scenario 详细文本**：90 个，分布在 `archive/<change>/specs/` 各 change 目录（详见 CHANGELOG.md）
- **当前 spec 版本**：v6.0.0
- **下一次架构级变更触发**：D1 域升级 v3.0+ 或 D7 编排 v7.0+ 跨域契约变化时重新审计 Boundary Debt Decisions