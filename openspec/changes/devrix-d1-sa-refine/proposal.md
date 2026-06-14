# Proposal: D1 Communication — 切法 A 信号分层（S13–S18）

**Change ID:** devrix-d1-sa-refine
**Demand ID:** DM-20260614-006
**Status:** S3_Approved

---

## 1. Background

D1 根本目标：**让智能体与 IM 的通信对用户更友好**。

用户侧可验证承诺：

1. **入站**：指令不丢、可追、可续聊  
2. **出站三类信号**：  
   - ① 思考信息（Thinking）  
   - ② 任务处理信息（Task）  
   - ③ 基于用户指令的总结反馈（Conclusion）  
3. **通道**：多 IM 可扩展，语义不变、编码可换  
4. **信任**：总结/错误必达（弱网不减损）  
5. **边界**：D1 = Trusted Intermediary — 可信送达 + 客观锚点；质量评级与信誉归 D5/D6（博弈共识 2026-06-14）

本 proposal（S2）定义 **D + S**；design.md（S3）定义 **A + F** 与完备性边界。

## 2. Problem Statement

Registry 将 module 当 Scenario，无法回答「用户收到哪类信息」「指令是否存下来」。切法 B（按用户操作动线切 S）仍会把三类 outbound 混在一个 S 里。**切法 A** 以 **信号类型** 为 S 主轴，与 EventBus Priority、Card 分区、用户心智一致。

## 3. Proposed Solution — 切法 A：六场景 + Legacy 双轨

### 3.1 价值流 Scenario（canonical，D1-S13–S18）

| S ID | Scenario | 用户目标（一句话） | 博弈角色 |
|------|----------|-------------------|----------|
| **D1-S13** | CaptureUserIntent | 我说的指令一定进系统、查得到、能接着聊 | Principal 输入信号 |
| **D1-S14** | PresentThinking | 我能看到它在想什么（可折叠/流式） | Cheap talk 公开 |
| **D1-S15** | PresentTaskProgress | 我能看到它在做什么（工具/Worker/进度） | 过程信号 |
| **D1-S16** | DeliverConclusion | 我能拿到针对我指令的总结结论 | **Costly signal** |
| **D1-S17** | ConnectChannel | 换飞书/钉钉/CLI，三类信息结构一致 | 平台子博弈 |
| **D1-S18** | GuaranteeDelivery | 弱网/背压也不丢结论和错误 | 机制惩罚/承诺 |

**横切门控**（不单独占 S，编入 S13）：

- PermissionConfirm：`ResolvePermissionGate` — tool 执行前用户批准子博弈

### 3.2 Legacy Module Index（冻结，D1-S1–S12）

旧编号 **不重定义语义**，仅作 module/包追溯与现有 44 T 注释锚点。见 design.md §8。

### 3.3 出站信号统一契约（S3 详述，v1.1 代码）

```text
IMOutboundSignal { Kind: Thinking | Task | Conclusion, ... }
```

平台差异下沉为 **Encode* F**（S17 编排），不在 A 层分叉。

### 3.4 分阶段

| 版本 | 范围 |
|------|------|
| v1.0 | S13–S18 registry + Gherkin + Legacy 双轨表 |
| v1.1 | contracts（含客观锚点）+ span + chain_integrity + acceptance |
| v2.0 | 代码按信号 A/F 拆包 |

## 4. Success Metrics

| 指标 | 目标 |
|------|------|
| 价值流 S 注册 | 6/6（S13–S18） |
| 三类 outbound 各有 Scenario | 各 ≥1 happy + 1 sad |
| Legacy T 可追溯 | 44/44 canonical 列 |
| Conclusion P0 span 声明 | 100%（registry） |

## 5. Implementation Plan

| Phase | 内容 |
|-------|------|
| P0 | S3-Gate + 文档 |
| P1 | v1.0 merge openspec/specs |
| P2 | v1.1 contracts/span/tests |

## 6. Risks & Mitigations

| 风险 | 缓解 |
|------|------|
| 双轨 S 混淆 | layering 置顶 Value Stream 表 |
| EngineEvent→Signal 映射遗漏 | design §4 映射表 |
| 与 D7 Worker 进度重复 | S15 只负责 IM 呈现，D7 产事件 |

## 7. Out of Scope

- v1.0 Go 代码
- Auth 实现
- 旧 T 注释批量重命名（v1.1 可选）
- Agent 自报 Confidence / 信誉系统 / 惩罚策略（D5/D6，DM-20260614-007）
- D1 区分好坏 Agent 或保证用户正确解读信号（D5/D6 + 产品层）

## 8. 博弈共识引用

Claude + Cursor 对焦见 `gaming-analysis.md` §最终三方共识。核心：**Task 段为硬信号；Thinking 为 cheap talk；D1 填客观锚点，D6 填质量评级。**
