# Design: D1 Communication — 切法 A 信号分层

**Change ID:** devrix-d1-sa-refine
**Demand ID:** DM-20260614-006
**Status:** S3_Approved

---

## Grill Review 记录

| 决策点 | 结论 | 备注 |
|--------|------|------|
| S 切法 | **Agreed 切法 A** | 按用户收到的信号类型 |
| S 编号 | **Agreed 新号段 S13–S18** | 旧 S1–S12 Legacy 冻结 |
| Dispatch 建模 | **Agreed** 单 A + 多 F | 非 RouteMessage/RouteOrchestration 双 A |
| 信号契约 | **Agreed** IMOutboundSignal 三 Kind + 客观锚点 | v1.1；禁止 Agent 自填 Confidence |
| 博弈定位 | **Agreed** Trusted Intermediary | Claude + Cursor 2026-06-14 |
| 完备性边界 | **Agreed** 送达+锚点 ∈ D1；解读+评级 ∈ D5/D6 | 见 § Decision |
| Review | **Approved** | 用户确认 2026-06-14；Phase 1 registry merge |

---

## 1. Root Cause Analysis

| 根因 | 表现 |
|------|------|
| S 按 module 切 | 三类 outbound 无 registry 锚点 |
| A 按平台/包切 | 扩展 IM 污染 T |
| 无 costly signal S | Critical 路径与用户「总结必达」未对齐 |
| 无 Persist A | 「指令存下来」不可验收 |

---

## 2. Solution Design

### 2.1 D1 根本目标（North Star）

> IM 约束下，用户指令不丢；Agent 状态以 **Thinking / Task / Conclusion** 三层信号公开；Conclusion 与 error 为 costly signal，弱网必达；换 IM 只换编码不换语义。

### 2.2 S 层 — 价值流（canonical: D1-S13–S18）

| S | 名称 | 触发 | 用户完成条件 | 关联 Legacy module |
|---|------|------|--------------|-------------------|
| S13 | CaptureUserIntent | 用户发消息/命令 | 指令已 persist 且已 dispatch | gateway, adapters inbound, commands |
| S14 | PresentThinking | Engine thinking | 用户看到思考区更新 | feishu progress/thinking 流 |
| S15 | PresentTaskProgress | tool/Worker/milestone | 用户看到任务/工具/Worker 进度 | worker_card, tool collapse, milestone 展示 |
| S16 | DeliverConclusion | text stream + complete | 用户收到针对指令的终态总结 | complete 卡, summary |
| S17 | ConnectChannel | 进程启动/选 IM | 平台连接健康、入站/outbound 编码可用 | adapters, connection, instance, ratelimit |
| S18 | GuaranteeDelivery | 背压/弱网 | complete/error 仍到达 Adapter | eventbus Critical |

**Domain Kernel（非 S）：** `core.Card`、`types.Session`、`types.InboundMessage`

### Decision: 切法 A vs 切法 B

| 方案 | 优点 | 缺点 |
|------|------|------|
| A: 按收到什么切 S14–S16 | 与三类信号、Priority、UX 1:1 | 需新号段 |
| B: 按操作动线切 Start/Stream/Control | 贴近操作 | outbound 仍混在一个 S |

**选择:** A  
**理由:** 根本目标是「友好 **获取** 信息」，不是「发起对话」；与 EventBus Critical 对齐 Conclusion。

### Decision: S 编号 — 新号段 vs 复用

| 方案 | 优点 | 缺点 |
|------|------|------|
| 复用 S1–S7 新语义 | 号短 | **与 44 T 冲突**（S5 Milestone 等） |
| **S13–S18 新号段** | 零破坏 IMPLEMENTED T | 表变长 |
| 仅改描述不改号 | 无迁移 | 价值流不彻底 |

**选择:** S13–S18 + Legacy S1–S12 冻结  
**理由:** AC5 要求 v1.0 不改测试注释。

### Decision: Milestone 边界

| 方案 | 优点 | 缺点 |
|------|------|------|
| 保留 Legacy S5 独立 S | 零文档变更 | 与切法 A 信号② 重复；T 语义冲突 |
| **S15 语义 + S17 Encode F** | Milestone 卡 = Task 信号；DAG 逻辑归 D7 | v2.0 需迁代码 |

**选择:** IM Milestone **展示**归 **S15-A01-F03**（EmitMilestoneCardProgress）；平台 Card 编解码归 **S17 Encode F**；DAG/TaskFlow 编排源 **v2.0 迁 D7**。Legacy S5 **DEPRECATED 为独立 S**。

### Decision: 信誉 / 置信度 / 惩罚 — 跨域边界（D5 + D6）

| 概念 | SoT 域 | D1 职责 | 说明 |
|------|--------|---------|------|
| **置信度（Confidence）** | **D5 测量** + **D6 校准** | 可选展示 D6 只读字段；**禁止** Agent 自填为 SoT | D5：`d1.signal.*` 链路客观指标；D6：Judge/Rubric 离线校准 |
| **信誉（Reputation）** | **D6 存储** + **D5 暴露** | 不提供信誉 S；保留 trace/session 关联 id | 跨 session 聚合状态属 Evolution |
| **惩罚（Punishment）** | **D6 策略** → **D2/D4/D7 执行** | S13 捕获用户反馈入站（如「结论不对」） | 惩罚 = 策略空间收缩，非 IM 新 Scenario |

**选择:** 三概念 **不在 D1 SoT**；D1 v1.1 仅保证信号结构、Critical 必达、**可关联 trace_id / session_id / signal sequence**。  
**理由:** 重复博弈中的信誉更新与策略调整属可观察 + 自我进化域；D1 保持「6 价值流 S」不膨胀。  
**关联需求:** `DM-20260614-007` — `openspec/changes/devrix-reputation-feedback-loop/demand.md`

**D1 预留钩子（v1.1，不含业务逻辑）：**

| 钩子 | 位置 | 用途 |
|------|------|------|
| 用户反馈入站 | S13-A01/A05 扩展 F | 捕获拒结论 / 纠错 turn → D5 span |
| Signal Metadata | `IMOutboundSignal.Metadata` | 承载 D6 只读 badge（非 Agent 自报 Confidence） |
| Trace 关联 | 全链 span | `d1.capture.persist` → `d1.signal.conclusion` → D6 eval 输入 |

### Decision: D1 博弈定位 — Trusted Intermediary（完备性边界）

**Claude + Cursor 共识（2026-06-14）：** D1 是**可信中立通道**，不是信号裁判或 Agent 监督者。

| 能力 | D1 提供？ | v1.1 验证方式 |
|------|----------|---------------|
| 防止 Agent「消失」（complete/error 必达） | ✅ S18 | `eventbus.publish_critical` + T |
| 信号顺序可追溯、不篡改 | ✅ S14–S16 | `d1.signal.chain_integrity` |
| 客观锚点（Agent 不可伪造） | ✅ 契约字段 | source_event_id、elapsed_ms、sequence |
| 注意力成本（背压） | ✅ EventBus | Compact/Drain；IM UX 属 P2 |
| 区分好/坏 Agent | ❌ | D5 metric + D6 Judge |
| 用户即时质量反馈闭环 | ❌ | S13 feedback 钩子 → D5/D6 |
| Agent 自报 Confidence | ❌ **禁止** | cheap talk |

**选择:**  
> D1 只保证 **「信号可信送达」**；**「信号可被用户正确解读」** 与质量评级属 **D5/D6 + 产品层**（DM-20260614-007）。

**D1 可验证承诺（v1.1 操作定义）：**

| 承诺 | 实现 |
|------|------|
| sequence 完整 | IMOutboundSignal.Sequence 单调；span 链 S14→S15→S16 |
| Critical 必达 | S18-A01-F02；Drain 不丢 complete/error |
| source_event_id | 每个 Signal 关联 EngineEvent ID |
| elapsed_ms | 自入站 persist 至该 signal emit 的 D1 侧计时 |

---

## 3. A + F 定义（S3 产出）

### 3.1 D1-S13 CaptureUserIntent

| A ID | Name | Kind | 输入 | 输出 | F 编排 |
|------|------|------|------|------|--------|
| S13-A01 | AcceptInboundMessage | USER | raw IM/CLI | InboundMessage | F01 ParseFeishu/DingTalk/CLI |
| S13-A02 | PersistUserTurn | SYSTEM | InboundMessage | persisted | F01 sessionStore.Update, F02 transcript append |
| S13-A03 | DispatchToAgent | USER | session, content | event_chan | **F01 routeLegacyD2, F02 routeD7, F03 routeAgent** |
| S13-A04 | ResolvePermissionGate | USER | tool, risk | approved | F01 permissionMgr, F02 adapter card |
| S13-A05 | ParseCommand | USER | /new /stop /help | command | F01 command handlers |

**编排序：**

```
A01 Accept → A02 Persist → (A05 if command) → A03 Dispatch
                              ↘ A04 if tool needs approval (interrupt)
```

**Dispatch 规则：** 仅 **一个 USER A（A03）**；D7/Agent/Legacy 为 **F**，不可并列 USER A。

### 3.2 D1-S14 PresentThinking

| A ID | Name | Kind | F |
|------|------|------|---|
| S14-A01 | EmitThinkingDelta | SYSTEM | F01 map EngineEvent→Signal(Thinking), F02 encodeFeishu/CLI |

**EventBus Priority:** Low（可 Compact）  
**UI 区：** collapse_thinking / content 流

### 3.3 D1-S15 PresentTaskProgress

| A ID | Name | Kind | F |
|------|------|------|---|
| S15-A01 | EmitToolProgress | SYSTEM | F01 tool_call/result→Signal(Task) |
| S15-A02 | EmitWorkerProgress | SYSTEM | F01 WorkerEvent→Signal(Task), F02 worker card encode |

**EventBus Priority:** Normal  
**UI 区：** collapse_tools / Worker 独立卡

### 3.4 D1-S16 DeliverConclusion

| A ID | Name | Kind | F |
|------|------|------|---|
| S16-A01 | EmitSummaryChunk | SYSTEM | F01 text delta→Signal(Conclusion) |
| S16-A02 | FinalizeReply | SYSTEM | F01 complete/error→Signal(Conclusion), F02 close stream |

**EventBus Priority:** **Critical**（complete/error）  
**UI 区：** footer / complete 卡 — **与 S14/S15 sequence 隔离**

### 3.5 D1-S17 ConnectChannel

| A ID | Name | Kind | F |
|------|------|------|---|
| S17-A01 | ParseFeishuInbound | USER | （也可作为 S13-A01 的 F，此处登记平台入口） |
| S17-A02 | ParseDingTalkInbound | USER | |
| S17-A03 | ParseCLIInbound | USER | |
| S17-A04 | ManageConnection | INTERNAL | heartbeat, reconnect |
| S17-A05 | RegisterInstance | INTERNAL | instance registry |
| S17-A06 | CheckRateLimit | INTERNAL | token bucket |

**扩展 IM 均衡：** 新平台 = 新 Parse* + 新 Encode* F，**不动 S14–S16 的 A**。

### 3.6 D1-S18 GuaranteeDelivery

| A ID | Name | Kind | F |
|------|------|------|---|
| S18-A01 | DeliverOutboundSignal | SYSTEM | F01 Publish, F02 PublishCritical, F03 Drain, F04 Compact, F05 Reconnect |

对用户可见只有 A01；Drain 等为 F。

---

## 4. Key Interfaces / Types

### 4.1 IMOutboundSignal（v1.1 — shared/contracts）

```go
type SignalKind string

const (
    SignalThinking   SignalKind = "thinking"    // ① cheap talk（UX）；硬信号在 Task
    SignalTask       SignalKind = "task"        // ② 工作证明主段
    SignalConclusion SignalKind = "conclusion"  // ③ costly
)

type IMOutboundSignal struct {
    Kind           SignalKind
    SessionID      string
    Sequence       uint64        // 会话内单调序号（D1 分配，防乱序/伪造）
    Delta          string
    IsTerminal     bool          // complete/error 时为 true

    // 客观锚点（D1 填充，Agent 不可伪造 — 博弈共识 AC9）
    SourceEventID  string        // 关联 EngineEvent
    ElapsedMs      int64         // 自 S13 persist 至 emit 的毫秒数
    InboundTurnID  string        // 关联用户入站 turn

    // 扩展；D6 只读 badge 可经 Metadata 展示，非 Agent 自报 Confidence
    Metadata       map[string]string
}
```

**明确不包含（博弈共识）：** `Confidence`、`ReasoningSteps`、`HistoricalAccuracy` 作为 Agent 自填 SoT 字段。

### 4.2 EngineEvent → IMOutboundSignal 映射

| EngineEvent type | SignalKind | S | Priority |
|------------------|------------|---|----------|
| thinking | Thinking | S14 | Low |
| tool_call, tool_result | Task | S15 | Normal |
| worker_* (via WorkerEvent) | Task | S15 | Normal |
| text (stream) | Conclusion | S16 | Normal |
| complete | Conclusion | S16 | **Critical** |
| error | Conclusion | S16 | **Critical** |
| progress/milestone (IM 展示) | Task | S15 | Low |

### 4.3 平台编码 F（S17 编排，示例）

| F | 职责 |
|---|------|
| EncodeFeishuCardKit | Thinking/Conclusion 流式 |
| EncodeFeishuWorkerCard | Task Worker 双卡 |
| EncodeDingTalkMarkdown | 全 Kind |
| EncodeCLIANSI | CLI |

---

## 5. Data Flow（切法 A 主编排序）

```
[User IM]
  → S17 Parse* (F of S13-A01)
  → S13 Accept → Persist → Dispatch (F: D7|Agent|D2)
        ↘ S13 PermissionGate (optional)

[Agent events]
  → S18 DeliverOutboundSignal (EventBus)
  → map to IMOutboundSignal Kind
  → S14 EmitThinking | S15 EmitTask | S16 EmitConclusion
  → S17 Encode* → [User IM]

[S18 overlay] Critical Conclusion 永不 Drain
```

---

## 6. File Manifest

### v1.0（registry）

- `openspec/specs/architecture/layering.md` — 双轨表
- `openspec/specs/d1-communication/{a,f,t,span,spec}.md`
- `openspec/t-registry.md`

### v1.1（代码）

- `internal/shared/contracts/im_outbound_signal.go`
- `internal/layers/communication/...` signal mapper + span
- `tests/acceptance/p0/d1_signal_journey_test.go`

---

## 7. Regression Risk Assessment

| 风险 | 检测 |
|------|------|
| 双轨 S 误用 Legacy 作 SoT | layering 置顶 banner |
| Signal 映射漏 event type | 映射表 + acceptance |
| WorkerEvent 字段丢失 | contracts diff vs wave |

---

## 8. Rollback Plan

v1.0 revert OpenSpec；Legacy S1–S12 未动；S13–S18 整段删除即可。

---

## 9. Legacy Module Index（D1-S1–S12 冻结）

| Legacy S | Module | canonical 价值流 |
|----------|--------|-----------------|
| S1 Gateway | gateway/ | S13, S18 |
| S2 Adapters | adapters/ | S13, S14–S16, S17 |
| S3 Commands | commands | S13-A05 |
| S4 Auth | auth/ | S13-A04 扩展 |
| S5 Milestone | milestone/ | **S15 展示 F**（DEPRECATED 为独立 S） |
| S6 RateLimit | ratelimit/ | S17-A06 |
| S7 Metrics | metrics/ | REMOVED → D5 |
| S8 Renderers | renderers/ | S14–S16 的 Encode F |
| S9 EventBus | eventbus/ | S18 |
| S10 Connection | connection/ | S17-A04 |
| S11 Core | core/ | Domain Kernel |
| S12 Instance | instance/ | S17-A05 |

### 9.1 Legacy T → Canonical 全表（44 行）

| Legacy T | Canonical S/A/F |
|----------|-----------------|
| D1-S1-A01-T01 | S13-A02-F01 CreateSession |
| D1-S1-A01-T02 | S17-A05 RegisterInstance |
| D1-S1-A01-T03 | S16-A02-F buildCompletionSummary |
| D1-S1-A01-T04 | S16-A02-F ComputeCtxPct |
| D1-S1-A01-T05 | S13-A02-F sessionStore lifecycle |
| D1-S1-A02-T01 | S13-A03 + S14/S15/S16 |
| D1-S1-A03-T01 | S13-A04 ResolvePermissionGate |
| D1-S1-A03-T02 | S13-A04 ResolvePermissionGate |
| D1-S1-A04-T01 | S13-A03-F03 routeAgent |
| D1-S5-A01-T01 | S15-A01-F03 + D7（v2.0） |
| D1-S5-A01-T02 | S15-A01-F03 + D7 TaskFlow |
| D1-S5-A01-T03 | S15-A01-F03 EmitMilestoneProgress |
| D1-S3-A01-T01 | S13-A05 ParseCommand |
| D1-S3-A01-T02 | S13-A05 ParseCommand |
| D1-S3-A01-T03 | S13-A05 ParseCommand |
| D1-S8-A01-T01 | S17 Encode F |
| D1-S8-A01-T02 | S14–S16 Encode F |
| D1-S8-A01-T03 | S17-A03 + Encode CLI |
| D1-S2-A01-T01 | S17-A01 / S13-A01-F01 |
| D1-S2-A01-T02 | S17-A02 ParseDingTalkInbound |
| D1-S2-A02-T03 | S15-A02 + S17 Encode |
| D1-S2-A02-T04 | S16-A01 + S17-F01 EncodeFeishuCardKit |
| D1-S2-A02-T05 | S16-A01 + S17-F01 |
| D1-S2-A02-T06 | S16-A01 + S17-F01 |
| D1-S2-A02-T07 | S16-A02 + S17-F01 |
| D1-S2-A02-T08 | S16-A01 + S17 Encode throttle |
| D1-S2-A02-T09 | S13-A02 + S16-A02 |
| D1-S2-A03-T01 | S15-A02 + S17-F02 |
| D1-S2-A03-T02 | S15-A02 + S17-F02 |
| D1-S2-A04-T01 | S17-F01 EncodeFeishuCardKit |
| D1-S2-A04-T02 | S17-F01 |
| D1-S2-A05-T01 | S13-A02-F ResolveSession |
| D1-S9-A01-T01 | S18-A01-F01 Publish |
| D1-S9-A02-T02 | S18-A01-F03 Drain |
| D1-S9-A02-T03 | S18-A01-F04 Compact |
| D1-S9-A02-T04 | S18-A01-F05 Reconnect |
| D1-S9-A01-T05 | S18-A01-F02 PublishCritical |
| D1-S9-A01-T06 | S18-A01-F02 PublishCritical |
| D1-S9-A01-T07 | S18-A01-F01 Publish backpressure |
| D1-S10-A01-T01 | S17-A04 ManageConnection |
| D1-S10-A01-T02 | S17-A04 ManageConnection |
| D1-S6-A01-T01 | S17-A06 CheckRateLimit |
| D1-S6-A01-T02 | S17-A06 CheckRateLimit |
| D1-S6-A01-T03 | S17-A06 CheckRateLimit |

**v1.0 规则：** 测试文件注释 **保留 Legacy T**；`t-registry.md` **Canonical** 列已同步（Phase 1.7 ✓）。

---

## 10. Span 矩阵（P0）

| Operation | Kind | S | T |
|-----------|------|---|---|
| d1.capture.persist | INTERNAL | S13 | S13-A02-T01 |
| d1.dispatch.route | INTERNAL | S13 | S13-A03-T01/T02 |
| d1.signal.thinking | INTERNAL | S14 | S14-A01-F01-T01 |
| d1.signal.task | INTERNAL | S15 | S15-A02-F01-T01 |
| d1.signal.conclusion | INTERNAL | S16 | S16-A02-T01 |
| d1.signal.chain_integrity | INTERNAL | S14–S16 | D1-S16-A02-T01 |
| d1.signal.task.work_proof | INTERNAL | S15 | D1-S15-A02-F01-T01 |
| eventbus.publish_critical | INTERNAL | S18 | S18-A01-F02-T01 |
| eventbus.drain | INTERNAL | S18 | S18-A01-F03-T01 |

---

## 11. S3-Gate 自检

- [x] 根本目标驱动 S 划分（切法 A）
- [x] 新号段避免 T 冲突
- [x] Dispatch 单 A + F 分支
- [x] 三类信号 + Guarantee 覆盖
- [x] Reviewer Approved（2026-06-14）
- [x] 博弈三方共识落盘（gaming-analysis.md §最终三方共识；AC8–AC11）

---

## 12. Cross-Domain — D5 / D6 信誉闭环（Out of Scope for D1）

本 change **不实现** 信誉、置信度、惩罚；域分工如下：

```
D1  S13–S18     信号 + 必达 + 关联 id
         │ spans / feedback events
         ▼
D5  客观 metric   链路完整性、送达窗口、user.feedback.*
         │ eval inputs
         ▼
D6  Reputation   Judge、信誉存储、Delta Gate、Tune、运行时干预
         │ EvolutionPolicy
         ▼
D2/D4/D7        prompt / 权限 / 路由策略落地
```

详见 `openspec/changes/devrix-reputation-feedback-loop/demand.md`（DM-20260614-007）。

---

## 13. v1.1 路线图（博弈共识优先级）

| 序 | 工作项 | AC | Phase |
|----|--------|-----|-------|
| 1 | `IMOutboundSignal` + 客观锚点 | AC9 | 2.1 |
| 2 | EngineEvent→Signal mapper | AC4 | 2.2 |
| 3 | span：`d1.signal.*` + `chain_integrity` + coverage gate | AC10 | 2.3 |
| 4 | `d1_signal_journey_test.go` 全链 | AC10, AC6 | 2.4 |
| 5 | Task 工作证明 span（tool/Worker↔Conclusion 关联） | AC11 | 2.3 |
| 6 | S13 feedback 入站钩子（仅 capture，不 Judge） | DM-007 依赖 | 2.x |
| — | 信誉/评级/惩罚 | DM-007 | 独立 change |
| — | IM 注意力 UX（折叠/屏蔽） | P2 | 产品迭代 |
