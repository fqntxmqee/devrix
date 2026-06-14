---
demand-id: DM-20260614-006
title: D1 Communication — 切法 A 信号分层与 IM 友好通信 registry 对齐
priority: P1
status: S5_Accepted
dsaft_domain: communication
created: 2026-06-14
gaming_consensus: 2026-06-14  # Claude + Cursor，见 gaming-analysis.md §最终三方共识
---

# D1 Communication — 切法 A 信号分层与 IM 友好通信

## 1. 背景

### 1.1 D1 根本目标（领域 North Star）

**在 IM 约束下，建立用户与智能体之间可信赖、可扩展、信息分层清晰的对话通道：**

- 用户指令 **不丢、可追、可续聊**；
- 用户能分层获取 Agent 状态：**思考 → 任务处理 → 总结反馈**；
- **总结与错误** 在弱网/背压下仍必达（costly signal）；
- 换 IM 平台（飞书/钉钉/CLI/未来 Slack）时，**三类信号语义不变**，仅平台编码变。

### 1.2 为何需要本 change

现有 D1 registry 按 Go 包/module 登记 S（Gateway、EventBus、Adapters…），与根本目标不对齐，导致：

- 三类 outbound 信号混在 `StreamPresentation` / `feishu.go` 实现中，无独立 S/T；
- 适配层扩展与 T 回归面耦合；
- EventBus Critical 路径与「总结必达」用户承诺未在 S 层显式表达。

### 1.3 D1 博弈定位（Claude + Cursor 共识）

> **D1 = Trusted Intermediary（可信中立通道）**：保证信号**可信送达**与**客观锚点**；**不**保证用户能正确解读信号质量，**不**区分好坏 Agent，**不**承载 Agent 自报置信度。

| D1 承诺（v1.1 可验证） | 博弈含义 |
|---------------------|----------|
| S14→S16 **sequence 完整** | 信号链顺序不篡改 |
| complete/error **Critical 必达** | S18 commitment device |
| **source_event_id** 可追溯 | 任何 outbound 可关联 EngineEvent |
| **elapsed_ms** 客观计时 | D1 测量，Agent 不可伪造 |
| EventBus **Compact/Drain** Low/Normal | 注意力成本机制（IM UX 产品化属 P2） |

| 非 D1 职责 | 归属 |
|-----------|------|
| 信号质量评级 / 用户是否该信 | D5 metric + D6 Judge |
| 信誉存储 / 惩罚策略 | D6 → D2/D4/D7 |
| Agent 自填 Confidence | **禁止**（cheap talk） |

**完备性边界：** D1 只保证「信号可信送达」；「信号可被用户正确解读」与即时/事后反馈闭环属 **D5/D6 + 产品层**（DM-20260614-007）。

## 2. 问题陈述

| # | 问题 |
|---|------|
| P1 | S 层未按用户 **收到的信息类型** 划分 |
| P2 | 入站「指令存下来」无独立 A/T 锚点 |
| P3 | 出站 thinking/task/conclusion 无统一信号契约 |
| P4 | 旧 S1–S12 与价值流 S 编号冲突（不可复用同号） |
| P5 | D7 路由、Permission 门控无 canonical A |
| P6 | D1「可信通道」完备性边界未声明 — 易与信号质量评级、信誉闭环混淆 |

## 3. 验收标准

| ID | 标准 | 优先级 |
|----|------|--------|
| AC1 | D1 价值流 S 采用 **切法 A**：S13–S18 六场景注册完整，旧 S1–S12 标记 Legacy Module Index 冻结 | P0 |
| AC2 | 三类 outbound 信号（Thinking/Task/Conclusion）各有独立 S + 至少 1 个 A + Gherkin Scenario | P0 |
| AC3 | CaptureUserIntent（S13）含 PersistUserTurn + DispatchToAgent，Dispatch 分支为 F 非并列 USER A | P0 |
| AC4 | `IMOutboundSignal` 三 Kind 契约在 design.md 定义，v1.1 落地 shared/contracts | P1 |
| AC5 | Legacy T（44 条 IMPLEMENTED）通过 canonical→legacy 列追溯，v1.0 不要求改测试注释 | P0 |
| AC6 | P0 costly signal（Conclusion/complete/error）绑定 GuaranteeDelivery（S18）+ span 声明 | P0 |
| AC7 | S3-Gate Approved；v1.0 无 Go 代码变更 | P0 |
| AC8 | design 声明 D1 **完备性边界**（Trusted Intermediary）；信誉/评级不在 D1 SoT | P0 |
| AC9 | v1.1 `IMOutboundSignal` 含 **客观锚点**（source_event_id、elapsed_ms、sequence）；**不含** Agent 自填 Confidence | P0 |
| AC10 | v1.1 D5 span/metric：`d1.signal.chain_integrity`（S14→S15→S16 链）+ E2E journey 测试 | P0 |
| AC11 | S15 Task 工作证明（tool/Worker 与 Conclusion trace 关联）至少在 design + span 登记 | P1 |

### v1.1 实施优先级（博弈共识）

| 优先级 | 项 | AC |
|--------|-----|-----|
| P0 | E2E journey + span 全链 | AC10 |
| P0 | S18 Critical ↔ S16 联动验收 | AC6 |
| P0 | 客观锚点契约 | AC9 |
| P1 | Task 工作证明 | AC11 |
| P2 | 用户注意力控制（IM 折叠/屏蔽 UX） | — |
| P3 | 信誉/置信度闭环 | DM-20260614-007 |

## 4. 依赖与约束

| 类型 | 内容 |
|------|------|
| 依赖 | `docs/methodology/dsaft-methodology.md`；`openspec/specs/project/master.md` |
| 依赖 | D7 `IOrchestrationEntry`（Dispatch F 之一） |
| 约束 | **切法 A**；新 S 号段 **D1-S13–S18**；旧 S1–S12 不重定义语义 |
| 约束 | v1.0 registry-only |

## 5. 变更范围

### 新增

- D1-S13–S18 Scenario 注册表
- `IMOutboundSignal` / `SignalKind` 契约设计（v1.1 代码）
- Legacy Module Index（S1–S12）文档节

### 修改

- `layering.md` D1 表（双轨：Value Stream S13–S18 + Legacy Module S1–S12）
- d1-communication `{a,f,t,span}-registry.md` canonical 列

### 不变更

- v1.0 Go 代码；旧测试 `// T:` 注释
- Agent 自报 Confidence / ReasoningSteps / HistoricalAccuracy 作为 D1 SoT
- D1 内实现信誉系统或惩罚策略（归 D5/D6）

## 6. 风险评估

| 风险 | 缓解 |
|------|------|
| 双轨 S 表增加认知负担 | layering 明确：SoT 价值流 = S13–S18；S1–S12 仅追溯 |
| 信号契约与 EngineEvent 映射遗漏 | design.md Event→Signal 映射表 |
| Milestone 展示归属 | 归 S15 PresentTaskProgress 的 F，非独立 S |
| 信誉/置信度/惩罚 | 归 D5+D6，见 DM-20260614-007；D1 仅预留 trace/feedback 钩子 |

## 7. 关联需求

| Demand ID | 标题 | 关系 |
|-----------|------|------|
| DM-20260614-007 | D5/D6 信誉反馈与进化闭环 | D1 信号链提供可关联锚点；信誉 SoT 不在 D1 |
