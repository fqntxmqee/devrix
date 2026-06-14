# D1 Communication Domain Specification

**Capability:** communication
**Status:** Active
**Version:** 4.0.0
**Last Updated:** 2026-06-14

---

## Overview

通信域负责 IM 双向对话：用户指令捕获（S13）、三类出站信号呈现（S14–S16）、多平台通道（S17）、弱网必达（S18）。作为 Devrix 入口层，所有用户交互经由此域进入系统。

> **Canonical SoT（v4.0+）：** 价值流 **D1-S13–S18**。  
> **代码路径 SoT：** `openspec/specs/architecture/code-layout.md` §4.1（scenario-slug → 目录）。  
> 当前实现仍部分位于 legacy 技术目录，见 §6 迁移表。
> **博弈定位：** D1 = **Trusted Intermediary** — 可信送达 + 客观锚点；质量评级与信誉见 D5/D6（DM-20260614-007）。

## Scenarios — Canonical（价值流）

| ID | Scenario | 用户目标 | Status |
|----|----------|----------|--------|
| D1-S13 | CaptureUserIntent | 指令不丢、可追、可续聊 | IMPLEMENTED |
| D1-S14 | PresentThinking | 信号① 思考信息 | IMPLEMENTED |
| D1-S15 | PresentTaskProgress | 信号② 任务/工具/Worker/里程碑 | IMPLEMENTED |
| D1-S16 | DeliverConclusion | 信号③ 总结（costly signal） | IMPLEMENTED |
| D1-S17 | ConnectChannel | 多 IM 接入与编解码 | IMPLEMENTED |
| D1-S18 | GuaranteeDelivery | 弱网/背压下结论必达 | IMPLEMENTED |

## Scenarios — Legacy Module Index（RETIRED v2.0）

> 历史索引已归档至 `t-registry.md` §Legacy Archive。新代码与测试 MUST 使用 D1-S13–S18 canonical ID。

## Architecture（价值流 — v2.0 实现）

```
[User IM]
  → S17 Parse* → S13 Accept → Persist → Dispatch (D7|Agent|D2)
  → Agent events → S18 EventBus → present/ (S14|S15|S16)
  → S17 Encode* → [User IM]
  S18 overlay: Critical Conclusion 永不 Drain
```

## Package Map

> **目标布局** 见 `../architecture/code-layout.md` §5–§6。下表为当前路径速查。

| scenario-slug | 当前路径 | Canonical S |
|---------------|----------|-------------|
| `capture` | `gateway/`, `signal/` | S13 |
| `thinking` / `taskprogress` / `conclusion` | `present/`（已拆分） | S14–S16 |
| `channel` | `adapters/`, `connection/`, `instance/`, `ratelimit/`, `renderers/` | S17 |
| `delivery` | `eventbus/` | S18 |
| `kernel` | `core/` | Domain Kernel |

## Architecture（Legacy 包结构 — RETIRED v2.0）
User/IM ──→ Adapters (D1-S2) ──→ Gateway (D1-S1) ──→ Context Engine (D2)
  │            │  │                    │                    │
  │  Feishu    │  ├─ CardKit 流式      ├─ AgentRoute ──→ D4 MultiAgent
  │  DingTalk  │  ├─ Worker 卡片       ├─ EventDispatcher
  │  CLI       │  └─ Session Resolve   ├─ PermissionManager
  │            │                       ├─ SessionStore (File)
  └─ Renderers (D1-S8) ←── EventBus (D1-S9) ←─┘
       │                         │
       ├─ CLIRenderer            ├─ Drain/Compact/Reconnect
       ├─ DingTalkCardRenderer   └─ Priority (Critical/Normal/Low)
       └─ Components
```

## Cross-Domain Dependencies

| Domain | 依赖内容 | 使用位置 |
|--------|---------|---------|
| D2 Context Engine | `contracts.IEngine`, tasks (CLICommands, PlanMode) | gateway, adapters/cli |
| D4 MultiAgent | `IAgentFactory`, `Agent`, `AgentObserver` | gateway/agent_route |
| D5 Observability | `Observability`, `Bridge`, tracer, telemetry | gateway, adapters |
| D7 Orchestration | `wave.WorkerType`, `wave.WorkerEvent` | adapters/feishu_worker_card |
| Shared | config, contracts, errors, types | 全子包 |

## Legacy 路径索引（RETIRED — 仅追溯）

> 已废弃的 D1-S1–S12 包映射见 `t-registry.md` §Legacy Archive。新代码禁止使用下表路径。

| 旧子包 | 原 Legacy S | 目标 scenario-slug |
|--------|-------------|-------------------|
| `gateway/` | S1 | `capture/` |
| `adapters/` | S2 | `channel/` |
| `eventbus/` | S9 | `delivery/` |
| `core/` | S11 | `kernel/` |

## Key Design Patterns

1. **Gateway-Adapter**: `CommunicationGateway` 在适配器与 Context Engine 之间路由消息。适配器实现 `EventHandler` 接口并使用 `GatewayAPI`。
2. **EventBus 状态机**: `eventbus.Bus` 实现 Drain → Compact → Reconnect 生命周期，5 种状态 (Running/Draining/Compacting/Reconnecting/Closed)。Critical 事件通过 `PublishCritical` 绕过常规通道直接扇出（P0 送达保证）。
3. **PermissionManager + YOLO 模式**: 权限请求根据风险等级自动审批，CRITICAL 风险永不自动审批。
4. **Card 系统**: 平台无关的 `core.Card` 模型 + `CardBuilder` 链式 API，渲染为平台特定的 JSON/Markdown。
5. **Session 持久化**: `FileSessionStore` 使用原子写入（临时文件 + rename），支持 `ResolveSessionByChatID` 实现重启后恢复。
6. **CardKit 流式**: 飞书适配器支持元素级流式输出（打字机效果），带节流配置，CardKit 不可用时降级为 `Im.Message.Patch`。

## Requirements — Canonical Gherkin（S13–S18）

> DM-20260614-006 · 切法 A

### CaptureUserIntent（D1-S13）

<!-- T: D1-S13-A02-T01, D1-S13-A03-T01, D1-S13-A03-T02, D1-S13-A04-T01 -->

- **入站持久化（happy）：** GIVEN 飞书非空消息 → WHEN Accept+Persist → THEN session 更新且 turn 可追溯
- **入站空消息（sad）：** GIVEN 空 content → WHEN Accept 校验 → THEN 错误且不 Dispatch
- **Dispatch D7：** GIVEN d7_enabled → WHEN Dispatch → THEN F02 ProcessMessage，不走 Legacy D2
- **Dispatch Legacy：** GIVEN d7 关闭且无 AgentFactory → WHEN Dispatch → THEN F01 contextEngine.Process
- **权限门控：** GIVEN CRITICAL + yolo → WHEN ResolvePermissionGate → THEN denied 直至用户确认

### PresentThinking（D1-S14）

<!-- T: D1-S14-A01-F01-T01 -->

- **thinking 流式：** GIVEN thinking 事件 → WHEN EmitThinkingDelta → THEN 思考区递增，Priority=Low
- **thinking 折叠：** GIVEN isComplete → WHEN Encode → THEN 移入 collapse_thinking

### PresentTaskProgress（D1-S15）

<!-- T: D1-S15-A01-F01-T01, D1-S15-A02-F01-T01 -->

- **tool 展示：** GIVEN tool_call → WHEN EmitToolProgress → THEN Kind=Task，collapse_tools 可见
- **Worker 隔离：** GIVEN N Worker 并行 → WHEN EmitWorkerProgress → THEN 独立 CardMsgID/sequence
- **Milestone 卡：** GIVEN milestone 进度 → WHEN S15-A01-F03 + S17 Encode → THEN Kind=Task 里程碑展示

### DeliverConclusion（D1-S16）

<!-- T: D1-S16-A02-T01, D1-S16-A02-T02, D1-S18-A01-F02-T01 -->

- **complete 终态：** GIVEN complete → WHEN FinalizeReply → THEN Conclusion IsTerminal=true，PublishCritical
- **text 流式：** GIVEN text delta → WHEN EmitSummaryChunk → THEN Conclusion 非终态流式增长
- **error 必达：** GIVEN Bus Draining → WHEN error PublishCritical → THEN 用户仍收到错误展示

### ConnectChannel（D1-S17）

<!-- T: D1-S17-A01-T01 -->

- **平台隔离：** 修改 Feishu Parse 不影响 DingTalk 测试归属
- **限流：** 超额 Webhook → CheckRateLimit → 429 + Retry-After

### GuaranteeDelivery（D1-S18）

<!-- T: D1-S18-A01-F02-T01, D1-S18-A01-F03-T01 -->

- **Critical 在 Drain 中必达：** complete PublishCritical → 订阅者全收到，不被 Compact 丢
- **Drain 只丢 Normal/Low：** thinking/progress 可排空，Critical 队列不受影响

## Registries

- **A 层**: `a-registry.md` — Canonical 16 + Legacy 21 Activities
- **F 层**: `f-registry.md` — Canonical 18 + Legacy 43 Function Points
- **T 层**: `t-registry.md` — 56 Test Points (44 IMPLEMENTED Legacy + 12 PLANNED Canonical)
- **Span**: `span-registry.md` — d1.signal.* / capture / critical
