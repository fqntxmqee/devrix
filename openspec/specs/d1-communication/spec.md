# D1 Communication Domain Specification

**Capability:** communication
**Status:** Active
**Version:** 5.0.0
**Last Updated:** 2026-06-30
**Domain SoT:** `d1-domain.md`
**Change:** devrix-d1-sa-refine (DM-20260614-006) — 切法 A / devrix-d1-dsaft-refactor (DM-20260628-003) — Gateway 拆分 + lint-d1-imports CI / **devrix-d1-ac-restructuring (DM-20260629-005) — 6 子 Change 联动: PR-2 #1 god-doc-split pt1 (spec.md 176→90 + d1-flow-architecture.md NEW)**

---

## See also

- **Flow architecture**：`../architecture/d1-flow-architecture.md`（价值流流图 + Package Map + Legacy 包结构 + 跨域接线）
- **End-to-end flows**：`terminal-state-guide.md`
- **Observability & Runbook**：`observability-guide.md`

---

## Overview

通信域负责 IM 双向对话：用户指令捕获（S13）、三类出站信号呈现（S14–S16）、多平台通道（S17）、弱网必达（S18）。作为 Devrix 入口层，所有用户交互经由此域进入系统。

> **Canonical SoT（v5.0+）：** 价值流 **D1-S13–S18**。  
> **架构包路径：** 见 `architecture/d1-flow-architecture.md`（PR-2 god-doc-split 拆出）。  
> **博弈定位：** D1 = **Trusted Intermediary** — 可信送达 + 客观锚点；质量评级与信誉见 D5/D6（DM-20260614-007）。

### 信号分层博弈论（切法 A）

> **基于 `devrix-d1-sa-refine` (DM-20260614-006)**

| 概念 | 定义 | D1 对应 |
|------|------|---------|
| **Separating Equilibrium** | 不同类型发送者选择不同信号，使接收者能推断类型 | **D7-S5** ClassifyIntent（D1 仅 Dispatch） |
| **Costly Signal** | 信号发送者承担真实成本，不可伪造 | **D1-S14** Thinking · **D1-S16** Conclusion |
| **Commitment Device** | 发送者通过信号约束自身未来行为 | **D1-S18** Critical 必达 · **D1-S16** complete/error |
| **Screening Mechanism** | 接收者设计契约让发送者自我选择 | **D1-S13-A04** Permission YOLO |

**禁止约束：** 禁止伪造进度信号（anti-fabrication commitment device）。

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

> 历史索引已归档至 `t-registry.md` §Legacy Archive + `openspec/archive/2026-06-14-devrix-d1-sa-refine/legacy-s1-s12.md`（PR-4 #2 registry-sync 沉 archive）。新代码与测试 MUST 使用 D1-S13–S18 canonical ID。

## Cross-Domain Dependencies

| Domain | 依赖内容 | 使用位置 |
|--------|---------|---------|
| D2 Context Engine | `contracts.IEngine`（CLI /task 仍经 `contracts.TaskCLIHandler` 注入） | adapters/cli（composition root 接线） |
| D4 MultiAgent | **禁止** D1 capture import；`bootstrap/sessionagents` 持有 leader 生命周期 | `bootstrap/sessionagents` |
| D5 Observability | `Observability`, `Bridge`, tracer, telemetry | capture, adapters |
| D7 Orchestration | **禁止** channel/adapters import；Worker 展示经 `contracts.WorkerStreamEvent` | D7 `wavescheduler/present.go` → D1 adapter |
| Shared | config, contracts, errors, types | 全子包 |

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
- **Dispatch D7：** GIVEN 已配置 IOrchestrationEntry → WHEN Dispatch → THEN F02 ProcessMessage，D1 不调用 IEngine.Process
- **Missing entry（sad）：** GIVEN 未配置 IOrchestrationEntry → WHEN Dispatch → THEN 返回错误，不 silent fallback
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
- **T 层**: `t-registry.md` — 56 Test Points（全 IMPLEMENTED）
- **Span**: `span-registry.md` — d1.signal.* / capture / critical

## Guides（互补，非登记 SoT）

- **领域 SoT**: `d1-domain.md` — North Star、Out of Scope、文档索引
- **Flow architecture**: `../architecture/d1-flow-architecture.md` — 价值流流图、Package Map、Legacy 包结构、跨域接线
- **终态架构**: `terminal-state-guide.md` — 跨域流程、A→F 编排树、IntentKind 时序、信号映射
- **可观测性**: `observability-guide.md` — Trace 树、EventBus 必达、验收 Runbook

---

## 修订记录

| Version | Date | Changes |
|---------|------|---------|
| 5.0.0 | 2026-06-30 | **DM-20260629-005 S7_Archive ACCEPTED (PR-2 god-doc-split pt1 + PR-6 gherkin-restructuring)**：(1) god-doc-split：176 → 90 行（拆出 `architecture/d1-flow-architecture.md` 80-100 行 NEW，含价值流流图 + Package Map + Legacy 包结构 + 跨域接线 + Legacy 路径索引）；(2) §Cross-Domain Dependencies 保留；(3) §Requirements 24 缩写 bullet → 90 `#### Scenario:` Gherkin 块（PR-6 落地，分布：happy 30 / sad 24 / boundary 18 / concurrent 9 / timeout 9）；(4) §Change line + 修订记录 v5.0.0 row |
