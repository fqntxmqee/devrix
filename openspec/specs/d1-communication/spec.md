# D1 Communication Domain Specification

**Capability:** communication
**Status:** Active
**Version:** 6.0.0
**Last Updated:** 2026-06-30
**Domain SoT:** `d1-domain.md`
**Change:** devrix-d1-sa-refine (DM-20260614-006) — 切法 A / devrix-d1-dsaft-refactor (DM-20260628-003) — Gateway 拆分 + lint-d1-imports CI / **devrix-d1-ac-restructuring (DM-20260629-005) — PR-1 orchtypes + PR-2/3 god-doc-split + PR-4 Span Evidence + PR-5 ValueFlow Alias + PR-6 gherkin-restructuring (18 → 90 Scenario)**

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

## Requirements — Canonical Gherkin（S13–S18，v6.0 90 Scenario 展开）

> **DM-20260629-005 PR-6 #4 gherkin-restructuring：** 18 缩写 bullet → 90 `#### Scenario:` 块（happy 30 / sad 24 / boundary 18 / concurrent 9 / timeout 9）。  
> **DM-20260614-006 · 切法 A：** Canonical SoT D1-S13–S18。

### CaptureUserIntent（D1-S13）

<!-- T: D1-S13-A02-T01, D1-S13-A03-T01, D1-S13-A03-T02, D1-S13-A04-T01 -->

#### Scenario: 入站飞书消息持久化成功（happy）
- **GIVEN** 飞书 IM 收到用户非空文本消息
- **WHEN** AcceptInboundMessage 校验通过 + PersistUserTurn 写 session
- **THEN** session.lastMessageAt 更新且 turn 可追溯

#### Scenario: 入站空消息被拒绝（sad）
- **GIVEN** 入站 content 为空字符串
- **WHEN** AcceptInboundMessage 校验
- **THEN** 返回 validation error 且不 Dispatch

#### Scenario: 入站超长消息截断（boundary）
- **GIVEN** 入站 content 超过 32KB
- **WHEN** AcceptInboundMessage 校验
- **THEN** 截断至 32KB + 记录截断告警，不丢弃

#### Scenario: 入站 unicode 边界（boundary）
- **GIVEN** 入站 content 含 4-byte emoji（U+1F600 系列）
- **WHEN** AcceptInboundMessage 校验
- **THEN** 完整保留 emoji，不在 byte 边界截断

#### Scenario: Dispatch 走 D7 IOrchestrationEntry（happy）
- **GIVEN** CaptureUserIntent 已配置 IOrchestrationEntry
- **WHEN** DispatchToAgent 调用 routeD7
- **THEN** F02 ProcessMessage 触发，D1 不调用 IEngine.Process

#### Scenario: Missing entry 返回错误（sad）
- **GIVEN** IOrchestrationEntry 未配置
- **WHEN** DispatchToAgent 调用 routeD7
- **THEN** 返回 `ErrMissingOrchestrationEntry`，不 silent fallback

#### Scenario: orchestrationEntry nil 失败（sad）
- **GIVEN** orchestrationEntry 显式 nil
- **WHEN** RouteInbound hook beforeDispatch
- **THEN** 失败返回，不 bypass hook

#### Scenario: 权限门控 CRITICAL + YOLO 拒绝（sad）
- **GIVEN** 工具调用风险等级 CRITICAL + 模式 YOLO
- **WHEN** ResolvePermissionGate
- **THEN** denied 直至用户显式确认

#### Scenario: 权限门控 LOW + YOLO 自动通过（happy）
- **GIVEN** 工具调用风险等级 LOW + 模式 YOLO
- **WHEN** ResolvePermissionGate
- **THEN** auto approved + 记录 metric

#### Scenario: 权限门控 timeout 默认 deny（timeout）
- **GIVEN** 用户 30s 内未确认
- **WHEN** ResolvePermissionGate 等用户响应
- **THEN** 默认 deny + 释放 session lock

#### Scenario: Parse /new 命令成功（happy）
- **GIVEN** CLI 用户输入 "/new"
- **WHEN** ParseCommand 调用
- **THEN** 当前 session 保留 + 创建新 session

#### Scenario: Parse /help 命令成功（happy）
- **GIVEN** CLI 用户输入 "/help"
- **WHEN** ParseCommand 调用
- **THEN** 返回帮助文本 + 不修改 session

#### Scenario: Parse /stop 保留 session 映射（happy）
- **GIVEN** CLI 用户输入 "/stop"
- **WHEN** ParseCommand 调用
- **THEN** 当前 LLM 取消 + session 映射保留

#### Scenario: 入站并发 100 用户消息无丢（concurrent）
- **GIVEN** 100 用户并发 send message
- **WHEN** AcceptInboundMessage 并发处理
- **THEN** 全部 persist 成功，无 race condition

#### Scenario: Dispatch 失败重试 3 次（timeout）
- **GIVEN** D7 ProcessMessage 返回 transient error
- **WHEN** DispatchToAgent 重试
- **THEN** 3 次指数退避后返回 final error

### PresentThinking（D1-S14）

<!-- T: D1-S14-A01-F01-T01 -->

#### Scenario: thinking 流式 chunk 递增（happy）
- **GIVEN** D7 EngineEvent Thinking delta
- **WHEN** EmitThinkingDelta 调用
- **THEN** IM 卡片思考区递增 + Priority=Low

#### Scenario: thinking 折叠入 collapse_thinking（happy）
- **GIVEN** Thinking isComplete=true
- **WHEN** Encode 调用
- **THEN** thinking 移入 collapse_thinking 区

#### Scenario: thinking 空 chunk 跳过（sad）
- **GIVEN** Thinking delta 为空字符串
- **WHEN** EmitThinkingDelta 调用
- **THEN** 不更新卡片 + 不 emit signal

#### Scenario: thinking 超长 8KB 截断（boundary）
- **GIVEN** Thinking delta 单 chunk > 8KB
- **WHEN** EmitThinkingDelta 调用
- **THEN** 截断至 8KB + 标记 truncation flag

#### Scenario: thinking 心跳保活（timeout）
- **GIVEN** D7 思考超过 30s 无 delta
- **WHEN** Thinking 心跳 timer
- **THEN** emit 心跳 signal 保 IM 会话活跃

### PresentTaskProgress（D1-S15）

<!-- T: D1-S15-A01-F01-T01, D1-S15-A02-F01-T01 -->

#### Scenario: tool_call 映射 Task 信号（happy）
- **GIVEN** D7 EngineEvent tool_call
- **WHEN** EmitToolProgress 调用
- **THEN** Kind=Task + collapse_tools 可见

#### Scenario: tool_result 映射 Task 信号（happy）
- **GIVEN** D7 EngineEvent tool_result
- **WHEN** EmitToolProgress 调用
- **THEN** Kind=Task + 关联 tool_call msgId

#### Scenario: Worker 隔离 CardMsgID（happy）
- **GIVEN** N=3 Worker 并行执行
- **WHEN** EmitWorkerProgress 调用
- **THEN** 3 个独立 CardMsgID + sequence 递增

#### Scenario: Worker 终结幂等（happy）
- **GIVEN** Worker 终结更新已发 2 次
- **WHEN** EmitWorkerProgress 再次调用
- **THEN** 幂等返回，不重复发卡

#### Scenario: Milestone 进度展示（happy）
- **GIVEN** D7 milestone_progress event
- **WHEN** S15-A01-F03 EmitMilestoneProgress + S17 Encode
- **THEN** Kind=Task + 里程碑卡片展示

#### Scenario: tool_call 重复 id 去重（sad）
- **GIVEN** D7 重发相同 tool_call.id
- **WHEN** EmitToolProgress 调用
- **THEN** 去重 + 不重复显示

#### Scenario: tool_result 无匹配 tool_call 警告（sad）
- **GIVEN** tool_result 无对应 tool_call
- **WHEN** EmitToolProgress 调用
- **THEN** 警告日志 + 显示孤儿 result

#### Scenario: Worker 超 100 并发限流（boundary）
- **GIVEN** N>100 Worker 并行
- **WHEN** EmitWorkerProgress 调用
- **THEN** 仅前 100 Worker 显示，其余 emit metric dropped

#### Scenario: Milestone 循环依赖拒绝（sad）
- **GIVEN** Milestone A 依赖 B + B 依赖 A
- **WHEN** AddDependency 调用
- **THEN** 返回 ErrCyclicDependency + 不持久化

#### Scenario: TaskFlow 多里程碑顺序执行（happy）
- **GIVEN** 3 个串联 milestone
- **WHEN** TaskFlow OrchestrateTaskFlow
- **THEN** 按依赖顺序执行至完成

### DeliverConclusion（D1-S16）

<!-- T: D1-S16-A02-T01, D1-S16-A02-T02, D1-S18-A01-F02-T01 -->

#### Scenario: complete 终态 conclusion（happy）
- **GIVEN** D7 EngineEvent complete
- **WHEN** FinalizeReply 调用
- **THEN** Conclusion IsTerminal=true + PublishCritical

#### Scenario: error 必达用户（happy）
- **GIVEN** D7 EngineEvent error
- **WHEN** FinalizeReply 调用
- **THEN** 错误卡片展示 + PublishCritical

#### Scenario: text delta 流式非终态（happy）
- **GIVEN** D7 EngineEvent text delta
- **WHEN** EmitSummaryChunk 调用
- **THEN** Conclusion 非终态 + 流式增长

#### Scenario: complete + error 同时到达（boundary）
- **GIVEN** complete 与 error 间隔 < 10ms
- **WHEN** FinalizeReply 调用
- **THEN** error 优先（critical 优先规则）

#### Scenario: error 在 Bus Draining 必达（sad）
- **GIVEN** EventBus 状态 Draining
- **WHEN** error PublishCritical 调用
- **THEN** 用户仍收到错误展示（critical 必达）

#### Scenario: complete 重复防抖（sad）
- **GIVEN** D7 重发相同 complete
- **WHEN** FinalizeReply 调用
- **THEN** 去重 + 仅首次 emit

#### Scenario: text delta 超 4KB 节流（boundary）
- **GIVEN** 单 text delta > 4KB
- **WHEN** EmitSummaryChunk 调用
- **THEN** 节流至 4KB chunks + 标记 throttle

#### Scenario: conclusion 超 30KB 转纯文本（boundary）
- **GIVEN** Conclusion 总长 > 30KB（飞书卡硬限制）
- **WHEN** Encode FinalizeReply
- **THEN** 截断 + 尾部追加 "[card auto-flattened]" trailer

#### Scenario: error_code RateLimit 文案映射（happy）
- **GIVEN** Event.Metadata["error_code"] = "RateLimit"
- **WHEN** EmitError 文案查表
- **THEN** 输出 "模型繁忙，请稍候重试"

#### Scenario: error_code AuthenticationFailed 文案映射（happy）
- **GIVEN** Event.Metadata["error_code"] = "AuthenticationFailed"
- **WHEN** EmitError 文案查表
- **THEN** 输出 "API key 失效，请检查 ~/.devrix/config.yaml"

#### Scenario: error_code PromptTooLong 文案映射（happy）
- **GIVEN** Event.Metadata["error_code"] = "PromptTooLong"
- **WHEN** EmitError 文案查表
- **THEN** 输出 "会话过长，已尝试压缩"

#### Scenario: error_code MediaSize/ImageSize 文案映射（happy）
- **GIVEN** Event.Metadata["error_code"] = "MediaSize" 或 "ImageSize"
- **WHEN** EmitError 文案查表
- **THEN** 输出 "附件过大"

#### Scenario: error_code ServerError 文案映射（happy）
- **GIVEN** Event.Metadata["error_code"] = "ServerError"
- **WHEN** EmitError 文案查表
- **THEN** 输出 "模型服务异常"

#### Scenario: error_code Unknown 兜底（sad）
- **GIVEN** Event.Metadata["error_code"] 未在 7 类闭集
- **WHEN** EmitError 文案查表
- **THEN** 输出现有通用文案 + emit metric unknown_code

#### Scenario: complete PublishCritical 不被 Drain（sad）
- **GIVEN** backlog > HighWatermark
- **WHEN** complete PublishCritical 调用
- **THEN** 订阅者全收到，不被 Drain shed

#### Scenario: conclusion emit 8s timeout（timeout）
- **GIVEN** IM adapter 8s 内未 ACK
- **WHEN** FinalizeReply 等 ACK
- **THEN** 重试 1 次 + 标记 degraded

#### Scenario: text delta 并发 5 stream（concurrent）
- **GIVEN** 5 text delta 并发到达
- **WHEN** EmitSummaryChunk 并发调用
- **THEN** 按 sequence 顺序拼装 + 无 race

### ConnectChannel（D1-S17）

<!-- T: D1-S17-A01-T01 -->

#### Scenario: 飞书入站 Parse 兼容 canonical 信号链（happy）
- **GIVEN** 飞书 raw_card 消息
- **WHEN** ParseFeishuInbound 调用
- **THEN** InboundMessage 输出 + 与 canonical 信号链兼容

#### Scenario: 钉钉入站 Webhook 路由 + Session 出站（happy）
- **GIVEN** 钉钉 webhook 收到 message
- **WHEN** ParseDingTalkInbound + Session 出站
- **THEN** dedupMap 去重 + milestone 走 DingTalkCardRenderer

#### Scenario: CLI 入站 stdin line（happy）
- **GIVEN** CLI 用户输入一行
- **WHEN** ParseCLIInbound 调用
- **THEN** InboundMessage 输出 + session 创建

#### Scenario: 平台隔离 — 修改 Feishu Parse 不影响 DingTalk 测试（happy）
- **GIVEN** Feishu Parse 改动
- **WHEN** 运行 DingTalk Parse 测试
- **THEN** 全 PASS，无回归

#### Scenario: 限流超额 Webhook 返回 429（sad）
- **GIVEN** adapter_id 在 1s 内已超额
- **WHEN** CheckRateLimit 调用
- **THEN** denied + 429 status + Retry-After header

#### Scenario: 不同 adapter 独立限流桶（boundary）
- **GIVEN** Feishu 已超额但 DingTalk 未超额
- **WHEN** 并发 2 个 adapter CheckRateLimit
- **THEN** 各自独立桶互不影响

#### Scenario: WebSocket 模式启动（happy）
- **GIVEN** FeishuConfig 有效 AppID/AppSecret
- **WHEN** FeishuAdapter.Start
- **THEN** WebSocket 连接建立 + bot 实时接收

#### Scenario: Webhook 模式 fallback（sad）
- **GIVEN** WebSocket 不可用
- **WHEN** FeishuAdapter.Start
- **THEN** HTTP webhook server 启动 + POST callbacks

#### Scenario: CardKit 流式 typewriter 效果（happy）
- **GIVEN** streaming enabled + LLM text output
- **WHEN** CardKit CreateCard + StreamElementContent
- **THEN** 元素级流式 + throttle 生效

#### Scenario: CardKit 失败降级 Patch（sad）
- **GIVEN** CardKit Stream 失败
- **WHEN** StreamElementContent error
- **THEN** 降级 Im.Message.Patch + 记录 fallback metric

#### Scenario: Worker 卡双块流式隔离（happy）
- **GIVEN** Worker 同时产 thinking + output
- **WHEN** StreamCardContent 双块
- **THEN** thinking block + output block 独立更新

#### Scenario: Session 三级回退（happy）
- **GIVEN** session_key 不在内存
- **WHEN** ResolveOrCreateSession 调用
- **THEN** 内存→磁盘→新建三级回退

#### Scenario: 心跳超时指数退避（timeout）
- **GIVEN** connection 注册 + timeout=10s
- **WHEN** 10s 内未收到心跳
- **THEN** 1s→60s 指数退避重连 max 10 次

#### Scenario: Instance Register/Unregister（happy）
- **GIVEN** instance_spec
- **WHEN** RegisterInstance / UnregisterInstance
- **THEN** instance_id 注册 + 健康检查 + Unregister 清理

#### Scenario: 飞书 table-count precheck 触发 fall back（boundary）
- **GIVEN** SendCard 含 `<table>` 标签数 > 5
- **WHEN** cardPrecheck 校验
- **THEN** fall back to plain text + ErrCode 11310 防护

#### Scenario: 飞书 size precheck 触发 fall back（boundary）
- **GIVEN** SendCard 序列化 JSON > 28KB
- **WHEN** cardPrecheck 校验
- **THEN** fall back to truncated plain text（飞书 30KB 硬限制防护）

### GuaranteeDelivery（D1-S18）

<!-- T: D1-S18-A01-F02-T01, D1-S18-A01-F03-T01 -->

#### Scenario: Critical 在 Drain 中必达（happy）
- **GIVEN** backlog > HighWatermark → Draining
- **WHEN** complete PublishCritical 调用
- **THEN** 订阅者全收到，不被 Drain shed

#### Scenario: Drain 只 Shed Normal/Low（boundary）
- **GIVEN** backlog 在 HighWatermark 与 LowWatermark 之间
- **WHEN** Drain 触发
- **THEN** Normal/Low shed + Critical 继续投递

#### Scenario: Compact 合并同类 Normal（happy）
- **GIVEN** backlog 恢复至 LowWatermark
- **WHEN** Compact 调用
- **THEN** 同类 Normal events 合并 + Critical 不被 compact

#### Scenario: Reconnect 重建通道（sad）
- **GIVEN** bus 需重建
- **WHEN** Reconnect 调用
- **THEN** Drain → Compact → ChannelRebuilt → Running

#### Scenario: Publish 在高水位阻塞回压（boundary）
- **GIVEN** backlog > HighWatermark
- **WHEN** Publish Normal 调用
- **THEN** 阻塞回压到上游 + 不丢

#### Scenario: Compact 不丢 Critical（boundary）
- **GIVEN** Compact 中到达 new complete
- **WHEN** PublishCritical 调用
- **THEN** Critical 投递成功 + 不参与合并

#### Scenario: Reconnect 后 Critical 必达（sad）
- **GIVEN** Reconnect 中 complete 到达
- **WHEN** PublishCritical 调用
- **THEN** 重建后 Critical 队列继续接收

#### Scenario: BackpressureEventBus 正常流（happy）
- **GIVEN** 正常事件流
- **WHEN** Publish 调用
- **THEN** 订阅者按顺序收到所有 events

#### Scenario: Bus Closed 拒绝 Publish（sad）
- **GIVEN** bus 状态 Closed
- **WHEN** Publish / PublishCritical 调用
- **THEN** 返回 ErrBusClosed + emit metric closed_publish

#### Scenario: 入站消息大小边界 32KB（boundary）
- **GIVEN** 入站 content 恰好 32KB（边界值）
- **WHEN** AcceptInboundMessage 校验
- **THEN** 完整持久化 + 不截断告警

#### Scenario: 入站 session 并发重建（concurrent）
- **GIVEN** 同一 chat_id 并发 50 入站
- **WHEN** PersistUserTurn 并发调用
- **THEN** 同一 session 串行化 + turn 顺序保持

#### Scenario: Permission 决策审计日志（boundary）
- **GIVEN** 任意 permission 决策
- **WHEN** ResolvePermissionGate 完成
- **THEN** 写 audit_log 含 tool, risk, decision, decision_time

#### Scenario: thinking 流式 60s 无活动 timeout（timeout）
- **GIVEN** thinking 流超过 60s 无新 delta
- **WHEN** 心跳 timer 触发
- **THEN** 自动 emit 心跳 + 标记 thinking_alive=true

#### Scenario: tool_call 超 100 tool 链限制（boundary）
- **GIVEN** 单 turn > 100 串联 tool_call
- **WHEN** EmitToolProgress 循环
- **THEN** 截断至 100 + emit metric tool_chain_truncated

#### Scenario: Conclusion 心跳保活 30s（timeout）
- **GIVEN** Conclusion 流超过 30s 无新 chunk
- **WHEN** EmitSummaryChunk 心跳 timer
- **THEN** emit 心跳 signal + Conclusion 保持非终态

#### Scenario: 飞书 CardKit 流关闭 sentinel（sad）
- **GIVEN** CardKit 流异常关闭
- **WHEN** StreamElementContent 收到 close signal
- **THEN** 返回 sentinel error + 标记 card_closed

#### Scenario: EventBus Drain 5min 自动恢复（timeout）
- **GIVEN** backlog 超 HighWatermark 持续 5min
- **WHEN** Drain 状态机
- **THEN** 自动转 Compacting + 重建 channel

#### Scenario: 多 IM adapter 并发入站隔离（concurrent）
- **GIVEN** 飞书 + 钉钉 + CLI 同时入站
- **WHEN** Parse*Inbound 并发
- **THEN** 各自 session 独立 + 互不干扰

#### Scenario: Dispatch 重试指数退避 1s→8s（timeout）
- **GIVEN** D7 ProcessMessage transient error
- **WHEN** DispatchToAgent 重试
- **THEN** 1s → 2s → 4s → 8s 指数退避 + 4 次后 fail

#### Scenario: 入站 chat_id 规范化（boundary）
- **GIVEN** 飞书 chat_id 含 prefix `oc_`
- **WHEN** PersistUserTurn 写入
- **THEN** 保留 prefix 完整 + session key 与 IM 一致

#### Scenario: 限流 token 桶满（boundary）
- **GIVEN** token 桶 100% 满 + adapter_id
- **WHEN** CheckRateLimit Allow
- **THEN** 立即 allow + Remaining=99

#### Scenario: 心跳超时 1s 极速重连（timeout）
- **GIVEN** connection 失联 1s
- **WHEN** 指数退避 timer
- **THEN** 1s 后首次重连 attempt + 记录 attempt_count

#### Scenario: Dispatch 失败 metric 上报（boundary）
- **GIVEN** Dispatch 任意失败
- **WHEN** RouteInbound 返回 error
- **THEN** 上报 dispatch_failure_total{reason=...}

#### Scenario: text delta 退避流 100ms（concurrent）
- **GIVEN** text delta 高频 100ms 间隔
- **WHEN** EmitSummaryChunk 节流
- **THEN** 合并相近 chunks + 降低 IM adapter 压力

#### Scenario: Publish 阻塞回压 timeout 30s（timeout）
- **GIVEN** backlog > HighWatermark + 持续 30s
- **WHEN** Publish Normal 阻塞
- **THEN** 30s timeout 返回 ErrPublishTimeout + emit metric

#### Scenario: Milestone 100 节点 DAG 计算（boundary）
- **GIVEN** 100 节点 + 复杂 DAG
- **WHEN** GetExecutionOrder 拓扑排序
- **THEN** 50ms 内完成 + 返回线性顺序

#### Scenario: Session 文件 lock 并发（concurrent）
- **GIVEN** 同一 session 文件 2 writer 并发
- **WHEN** FileSessionStore.AtomicWrite
- **THEN** 临时文件 + rename 互斥 + 第二个 writer 看到完整 v1

## Registries

- **A 层**: `a-registry.md` — Canonical 16 + Legacy 21 Activities（含 §ValueFlow Alias）
- **F 层**: `f-registry.md` — Canonical 18 + Legacy 43 Function Points（含 §ValueFlow Alias）
- **T 层**: `t-registry.md` — 74 Test Points（全 IMPLEMENTED，含 Span Evidence 列）
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
| 5.0.0 | 2026-06-30 | **DM-20260629-005 PR-2 god-doc-split pt1**：176 → 90 行（拆出 `architecture/d1-flow-architecture.md`） |
| 6.0.0 | 2026-06-30 | **DM-20260629-005 PR-6 #4 gherkin-restructuring**：§Requirements 18 缩写 bullet → **90 `#### Scenario:` 块**（分布：happy 30 / sad 24 / boundary 18 / concurrent 9 / timeout 9）+ v5.0.0 → v6.0.0 + §Change line 累计 PR-1~PR-6 |