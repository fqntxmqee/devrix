---
demand-id: DM-20260614-008
title: D5/D6 — 信誉、置信度与惩罚闭环（Agent 重复博弈）
priority: P2
status: S1_Proposal
dsaft_domain: [observability, evolution]
created: 2026-06-14
related:
  - DM-20260614-006  # devrix-d1-sa-refine — D1 信号链与 trace 钩子
---

# D5/D6 — 信誉、置信度与惩罚闭环

## 1. 背景

### 1.1 领域 North Star

在 **重复博弈** 中，用户（Principal）委托 Agent（Agent）完成任务。D1 已（或将）提供三类 IM 信号（Thinking / Task / Conclusion）与 Critical 必达承诺，但 **不回答**：

- 用户该不该信这次 Conclusion？
- Agent 历史表现如何？
- 结论被用户否定后，系统如何 **收缩策略** 而非继续「刷信号」？

本需求将 **置信度、信誉、惩罚** 的 SoT 明确落在 **D5（可观察）** 与 **D6（自我进化）**，避免 D1 域膨胀。

**与 D1 的分工（Claude + Cursor 共识）：**

| 层 | 职责 |
|----|------|
| D1 | 可信送达 + 客观锚点（source_event_id、elapsed_ms）；S13 捕获用户 feedback |
| D5 | **信号质量客观 metric**（如 `d1.signal.chain_integrity`、假忙碌检测）；暴露信誉 gauge |
| D6 | **质量评级 + 信誉存储 + 惩罚策略**；Judge 可事后，即时 UI 提示属产品层 |
| 产品 | 用户拒结论入口、基于 D5/D6 的即时弱反馈（不阻塞 DM-007 v1.0） |

> Claude 张力点：D6 偏事后 Judge；**即时「该不该信」** 需 D5 实时 metric + 产品 UI 配合，非 D1 通道职责。

### 1.2 博弈论动机

| 概念 | 博弈角色 | 为何不在 D1 |
|------|----------|-------------|
| 置信度 | 对信号类型的可验证估计 | Agent 自报不可信；需 D5 客观 metric + D6 校准 |
| 信誉 | 跨 session 类型声誉 \(R_t\) | 属进化状态，非通道语义 |
| 惩罚 | 错误 Conclusion 后的策略空间收缩 | 执行在 D2/D4/D7，策略在 D6 |

### 1.3 依赖

- **DM-20260614-006**（D1 切法 A）v1.1：`IMOutboundSignal`、`d1.signal.*` span、S13 用户反馈入站钩子
- 现有 **D6-S3** Eval（Probe / Judge / Delta / Gate / Tune）
- 现有 **D5** Tracer / Metrics / Coverage

---

## 2. 问题陈述

| # | 问题 |
|---|------|
| P1 | 无跨 session Agent 信誉存储与更新规则 |
| P2 | 用户拒结论无标准 feedback 事件 → D6 无法 Judge |
| P3 | 信号链「假忙碌」（大量 Thinking、无 Task/Conclusion）无可观测指标 |
| P4 | 惩罚策略未登记，信誉低时无 EvolutionPolicy 联动 D2/D4/D7 |
| P5 | Agent 自报 Confidence 若进 D1 契约，易被博弈（刷分） |

---

## 3. L1–L5 映射草案

| 层级 | 资产 |
|------|------|
| **L1** | D5 Observability + D6 Evolution（跨 D1 消费） |
| **L2** | 信誉反馈价值流：Observe → Judge → UpdateReputation → ApplyPolicy |
| **L3** | D5-S1/S2 观测活动；D6-S3 Judge/Reputation；D6-S4 运行时干预 |
| **L4** | RecordFeedbackEvent(D5)；ComputeSignalQuality(D5)；UpdateReputation(D6)；SuggestEvolutionPolicy(D6) |
| **L5** | 见 §5 验收标准（T 点待 S3 登记） |

---

## 4. 域分工（SoT）

```
用户拒结论 / 纠错
    → D1 S13 入站捕获（钩子，非 SoT）
    → D5 span: user.feedback.conclusion_rejected
    → D6 JudgeResult + ReputationStore.Update
    → D6 EvolutionPolicy（L1–L4 档位）
    → D2/D4/D7 执行（prompt / permission / route）
    → D5 metric: agent.reputation.score（D6 写入，只读暴露）
    → D1 可选展示 D6 badge（非 D1 计算）
```

| 域 | SoT 职责 | 非职责 |
|----|----------|--------|
| **D1** | feedback 入站、signal 关联 id、可选展示 badge | 信誉计算、惩罚执行 |
| **D5** | 客观 metric、链路完整性、feedback 事件、信誉 gauge 暴露 | 判对错、改 Agent 策略 |
| **D6** | 信誉存储、Judge、Delta、Tune、EvolutionPolicy | IM 编解码 |
| **D2/D4/D7** | 消费 EvolutionPolicy 落地 | 信誉 SoT |

---

## 5. 验收标准（草案）

| ID | 标准 | 优先级 |
|----|------|--------|
| AC1 | D5 登记 `user.feedback.conclusion_rejected` span + `d1.signal.chain_integrity` metric | P0 |
| AC2 | D6 `ReputationStore` 支持按 agent_id/session 聚合更新与查询 | P0 |
| AC3 | 用户 feedback 入站 → D6 JudgeResult 可追溯（trace_id 关联 D1 conclusion span） | P0 |
| AC4 | 信誉低于阈值 θ 时，D6 输出 EvolutionPolicy（至少 L1/L2 两档） | P1 |
| AC5 | D5 暴露 `agent.reputation.score` Gauge（D6 写入）；D1 **不**将 Agent 自报 Confidence 作 SoT | P0 |
| AC6 | D6 Eval Probe 覆盖「Conclusion 被拒」回归场景 + Delta Gate | P1 |
| AC7 | 惩罚档位文档化：L1 信号质量 → L2 Permission → L3 D7 保守路由 → L4 CI gate | P1 |

---

## 6. 惩罚档位（EvolutionPolicy 草案）

| 档位 | 触发 | 动作 | 执行域 |
|------|------|------|--------|
| L1 | 单次拒结论 / 链断裂 | 提高 Task 工作证明要求；Thinking Compact 更激进 | D1 EventBus + D4 |
| L2 | R < θ₁ | 关闭 YOLO；CRITICAL 工具强制 Permission | D1 S13-A04 + D4 |
| L3 | R < θ₂ | D7 路由偏 conservative / 少 fork | D7 |
| L4 | R < θ₃ 或 Delta 回归 | 触发 `devrix eval run` + CI gate 阻断 | D6 |

---

## 7. 契约草案（v1.1 参考）

```go
// shared/contracts — D6 写，D5 读，D1 可选展示
type ReputationSnapshot struct {
    AgentID    string
    Score      float64   // [0,1]，D6 聚合，非 Agent 自报
    SampleSize int
    UpdatedAt  time.Time
}

type EvolutionPolicy struct {
    Level      int       // L1–L4
    Reason     string
    ExpiresAt  time.Time
}
```

**明确不做：** `IMOutboundSignal.Confidence` 由 LLM 自填作为信任依据。

---

## 8. 变更范围

### 新增（预期 change 包）

- D5 span/metric registry delta
- D6-S3 或新 S：Reputation / FeedbackJudge
- `shared/contracts/reputation.go`（草案见 §7）
- Cross-domain EvolutionPolicy 消费点（D2/D4/D7 design delta）

### 不变更

- D1 六价值流 S 数量（S13–S18）
- D1 作为信誉 SoT

---

## 9. 风险

| 风险 | 缓解 |
|------|------|
| 信誉分被误用为「自动化法庭」 | Judge 可解释 + 用户可 override |
| D6/D5 与 D1 v1.1 并行开发 | 依赖 DM-20260614-006 Phase 2 trace 钩子 |
| 惩罚过重导致 Agent 不可用 | 分档 + ExpiresAt + 人工复核路径 |

---

## 10. 下一步（S2 澄清）

- [ ] 确认信誉粒度：per-agent / per-session / per-tool
- [ ] 确认用户 feedback UI：IM 文本 / 命令 / reaction
- [ ] 与 D6-S4 Orchestration 校验边界对齐
- [ ] 登记 L5 T 点至 `openspec/specs/d5-observability/t-registry.md` 与 `d6-evolution/t-registry.md`
