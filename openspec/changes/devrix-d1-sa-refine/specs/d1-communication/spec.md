# D1 Communication Specification — Delta（切法 A）

**Change ID:** devrix-d1-sa-refine
**Demand ID:** DM-20260614-006
**Affects:** D1 价值流 S13–S18；Legacy S1–S12 冻结

---

## ADDED

### Requirement: CaptureUserIntent — 用户指令不丢、可追、可续聊

<!-- T: D1-S13-A02-T01, D1-S13-A03-T01, D1-S13-A03-T02 -->

#### Scenario: 入站消息持久化成功（happy path）

- GIVEN 飞书 Adapter 收到用户非空消息
- WHEN AcceptInboundMessage 与 PersistUserTurn 完成
- THEN Session last_activity 已更新
- AND 用户 turn 可在 session/transcript 追溯
- AND 产生 d1.capture.persist span（v1.1）

#### Scenario: 入站空消息被拒绝（sad path）

- GIVEN Gateway 收到 content 为空的消息
- WHEN AcceptInboundMessage 校验
- THEN 返回错误且不调用 DispatchToAgent
- AND 不写入 transcript

#### Scenario: Dispatch 走 D7 路径

- GIVEN orchestration.d7_enabled=true
- WHEN DispatchToAgent 执行
- THEN 调用 IOrchestrationEntry.ProcessMessage（F02）
- AND 不调用 legacy contextEngine.Process（F01）

<!-- T: D1-S13-A03-T01 -->

#### Scenario: Dispatch 走 Legacy D2 路径

- GIVEN orchestration.d7_enabled=false 且无 AgentFactory
- WHEN DispatchToAgent 执行
- THEN 调用 contextEngine.Process（F01）

<!-- T: D1-S13-A03-T02 -->

#### Scenario: 权限门控阻断未批准工具

- GIVEN tool risk=CRITICAL 且 yolo_mode=true
- WHEN ResolvePermissionGate 执行
- THEN 返回 denied
- AND 不执行工具直至用户确认

<!-- T: D1-S13-A04-T01 -->

---

### Requirement: PresentThinking — 信号① 思考信息

<!-- T: D1-S14-A01-F01-T01 -->

#### Scenario: thinking 增量流式呈现

- GIVEN Engine 产生 thinking 事件
- WHEN EmitThinkingDelta 映射为 IMOutboundSignal(Thinking)
- THEN 用户 IM 思考区 content 递增更新
- AND EventBus 优先级为 Low（可被 Compact）

#### Scenario: thinking 完成折叠到 collapse 区

- GIVEN thinking 事件 isComplete=true
- WHEN 平台 Encode 执行
- THEN 思考内容移入 collapse_thinking
- AND content 区清空准备下一轮

---

### Requirement: PresentTaskProgress — 信号② 任务处理信息

<!-- T: D1-S15-A01-F01-T01, D1-S15-A02-F01-T01 -->

#### Scenario: tool_call 在 collapse_tools 展示

- GIVEN Engine 产生 tool_call
- WHEN EmitToolProgress 执行
- THEN IMOutboundSignal Kind=Task
- AND 用户看到工具名与状态图标

#### Scenario: 多 Worker 独立卡片互不争用 sequence

- GIVEN N 个 Worker 并行 streaming
- WHEN EmitWorkerProgress 执行
- THEN 每个 Worker 独立 CardMsgID 与 sequence
- AND 不争用 Leader 卡 conclusion sequence

---

### Requirement: DeliverConclusion — 信号③ 总结反馈（costly signal）

<!-- T: D1-S16-A02-T01, D1-S16-A02-T02 -->

#### Scenario: complete 终态总结送达

- GIVEN Engine 产生 complete 事件
- WHEN FinalizeReply 映射为 IMOutboundSignal(Conclusion, IsTerminal=true)
- THEN 用户收到终态卡片/消息
- AND streaming_mode 关闭
- AND 走 PublishCritical 路径

#### Scenario: text 流式计入 Conclusion

- GIVEN Engine 产生 text 增量
- WHEN EmitSummaryChunk 执行
- THEN Kind=Conclusion 且 IsTerminal=false
- AND 用户看到答案主内容区流式增长

#### Scenario: error 与 complete 同级必达

- GIVEN EventBus 处于 Draining
- WHEN error 事件 PublishCritical
- THEN 用户仍收到错误结论展示
- AND 不被 Drain 丢弃

<!-- T: D1-S18-A01-F02-T01 -->

---

### Requirement: ConnectChannel — 多 IM 可扩展

<!-- T: D1-S17-A01-T01, D1-S17-A02-T01 -->

#### Scenario: 飞书入站不影响钉钉测试归属

- GIVEN 仅修改 Feishu Parse 逻辑
- WHEN 运行 S17-A01 关联测试
- THEN S17-A02 测试套件不受影响

#### Scenario: 限流拒绝超额 Webhook

- GIVEN 请求超过令牌桶
- WHEN CheckRateLimit 执行
- THEN 返回 429 与 Retry-After

---

### Requirement: GuaranteeDelivery — 弱网不减损 costly signal

<!-- T: D1-S18-A01-F02-T01, D1-S18-A01-F03-T01 -->

#### Scenario: Critical complete 在 Drain 中必达

- GIVEN backlog 超过 HighWatermark 且 Bus Draining
- WHEN complete PublishCritical
- THEN 所有订阅者在返回前收到
- AND complete 不被 Compact 合并丢失

#### Scenario: Drain 只丢弃 Normal/Low

- GIVEN Drain 执行中
- WHEN Normal thinking 与 Low progress 被排空
- THEN Conclusion Critical 队列不受影响

---

### Requirement: IMOutboundSignal 统一契约

<!-- T: v1.1 build constraint -->

#### Scenario: 三类 Kind 枚举完整

- GIVEN contracts 包定义 IMOutboundSignal
- WHEN Kind 取值
- THEN 仅允许 thinking | task | conclusion
- AND Conclusion 的 complete/error 必须 IsTerminal=true

#### Scenario: D1 不依赖 orchestration/wave 类型

- GIVEN Worker 进度编码
- WHEN 编译 adapters 包
- THEN Worker 载荷来自 contracts 或 EngineEvent 映射
- AND 无 import orchestration/wave

---

## MODIFIED

### Requirement: D1 Scenario 双轨注册

Canonical 价值流为 **D1-S13–S18（切法 A）**；**D1-S1–S12** 降级为 Legacy Module Index，仅供包路径与旧 T 追溯。

#### Scenario: 查阅 canonical 价值流

- GIVEN 架构师阅读 layering.md
- WHEN 查看 D1 Scenario 表
- THEN 首屏为 S13–S18 用户目标
- AND Legacy 表单独一节标注 FROZEN

---

## REMOVED

(None — Legacy S7 Metrics 已在 prior spec 移除)

---

## DEPRECATED

### Requirement: D1-S5 Milestone 作为独立 Scenario

Milestone/TaskFlow 的 **IM 展示** 归属 **D1-S15 PresentTaskProgress** 的 F；milestone/ 代码 v2.0 可迁 D7，registry v1.0 仅 DEPRECATED 独立 S 语义。
