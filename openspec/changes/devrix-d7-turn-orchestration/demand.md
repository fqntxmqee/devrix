---
demand-id: DM-20260614-020
title: D7 Turn 编排上移 — D7 直调 D3，D2 仅上下文组装
source: 架构澄清（D2 不应调用 D3；D7 Leader 应拥有 Turn 循环；D2 专注上下文/策略/持久化）
priority: P0
status: S3_Design
review-round: R1
dsaft_domain: orchestration
created: 2026-06-14
parent: dsaft-refactoring-playbook
related:
  - DM-20260614-009  # D2 SA Refine（D2 Follower + S15–S20）
  - DM-20260614-008  # D7 SA Refine（D7 Leader + Canonical S2–S5）
  - DM-20260614-016  # D3 SA Refine（D3 5+1 价值流）
  - DM-20260614-007  # D1 入站仅路由 D7
  - DM-20260614-018  # D4 Hub-Spoke 归 D7（因果链：DM-020 → DM-018，互补资产 + 双重边际化）
bilateral-consensus: gaming-analysis-bilateral-consensus.md  # G-01~G-12 全部确认；P1~P3 闭合
---

# D7 Turn 编排上移 — D7 直调 D3，D2 仅上下文组装

## 0. Review R1 决议（2026-06-14 — Owner 采纳架构师推荐）

| Decision | 结论 | 影响 |
|----------|------|------|
| **OQ1: D2-S16 命运** | ✅ **保留 Legacy 名**；Canonical 拆到 **D7-S2-A06** + **D2-S18** | S16 冻结追溯；新 Turn SoT 在 D7 |
| **OQ2: QueryLoopExecutor** | ✅ **v2.0 一次拆完**；v1.0 登记目标接口 | `ContextPreparer` + `TurnOrchestrator` + `SessionPersister` |
| **OQ3: SubQuery 内层 LLM** | ✅ **D7 包装 SubQuery Turn** | 与 Hub-Spoke 一致；D2-S19 不直连 D3 |
| **OQ4: Model 路由** | ✅ **D7 InvokeLLM 复用 D3-S1 RouteModel** | Tier 解析在 D7→D3 调用前 |
| **D1: D2→D3** | ❌ **禁止** | D2 移除 `ILLMGateway`；Bridge 消费方迁 D7 |
| **D2: 实施节奏** | v1.0 registry → v2.0 代码 | v1.0 **零 Go 变更** |

---

## 1. 背景

### 1.1 目标 North Star（本 change）

**D7 作为 Turn Leader：在每轮用户消息中，先向 D2 索取合法上下文，再向 D3 发起 LLM 推理，按 tool_calls 回调 D2 执行策略与工具，最后由 D2 持久化——D2 不再直接或间接调用 D3。**

| # | 可验证承诺 | 验收主体 |
|---|-----------|---------|
| C1 | **上下文**：Turn 前 messages/system prompt 合法、在预算内 | D2-S15 |
| C2 | **推理**：LLM 流式调用由 D7 发起，D3 保护链生效 | D7-S2-A06/A07 + D3-S2/S3 |
| C3 | **工具**：tool_calls 经 D2 策略面执行 | D2-S18 |
| C4 | **持久化**：Turn 结束后 snapshot/transcript durable | D2-S17 |
| C5 | **编排**：多轮 tool_use 由 D7 驱动，可取消 | D7-S2-A06 |
| C6 | **边界**：D2 无 `llmgateway` / `bridges/llm` import | import lint |

### 1.2 现状（与目标冲突）

```text
【现行】
D7 → QueryLoopExecutor.RunQueryLoop → D2.Process(S15→S16→S17)
                                          └── S16 内调 D3（ILLMGateway）

【目标】
D7 → TurnOrchestrator.RunTurn
       ├─ D2.PrepareContext
       ├─ D3.StreamChat（D7 直调）
       ├─ D2.ExecuteToolRound
       └─ D2.PersistTurn
```

### 1.3 与已有 SA Refine 的关系

| Change | 关系 |
|--------|------|
| DM-009 D2 | North Star 修订：去掉「D2 执行 LLM 循环」 |
| DM-008 D7 | S2 扩展 A06/A07；删除「D3 不与 D7 交互」 |
| DM-016 D3 | `ILLMGateway` 主消费方 D2→D7 |
| DM-018 D4 | 正交；可并行 v2.0 |

---

## 2. 问题陈述

| # | 问题 | 影响 |
|---|------|------|
| P1 | D2 名义 Follower，实际持有 LLM 循环（S16） | D7 无法调度 Breaker/路由 |
| P2 | `d7-domain.md` 写「D3 不直接与 D7 交互」 | 与 Owner 目标相反 |
| P3 | `cross-domain-boundaries.md` QueryLoop SoT=D2 | Turn 归属错误 |
| P4 | SubQuery / Autocompact 在 D2 内直连 D3 | 边界泄漏 |
| P5 | `QueryLoopExecutor` 黑盒隐藏 LLM 边界 | FastPath 不可观测 |

---

## 3. 验收标准

| ID | 标准 | 优先级 |
|----|------|--------|
| AC1 | 目标调用链写入 d7/d2 domain + cross-domain-boundaries + 新建 d3-boundary | P0 |
| AC2 | 明确禁止 D2→D3；删除 d7-boundary §4.1 `D2 → D3` | P0 |
| AC3 | D7-S2-A06/A07 Canonical A；D2-S16 修订为 Legacy/拆出 | P0 |
| AC4 | 目标接口 `ContextPreparer` + `TurnOrchestrator` v1.0 登记 | P0 |
| AC5 | Legacy T 映射（D2-S16-T* → D7-S2-A06-T*） | P0 |
| AC6 | SubQuery + Autocompact 灰区契约（D7 包装 / D7 调 D3） | P1 |
| AC7 | v1.0 S3-Gate + 零 Go 变更 | P0 |
| AC8 | v2.0 slice a–f 定义 | P1 |

---

## 4. Out of Scope

| 能力 | 归属 |
|------|------|
| D4 Agent 主循环内 LLM | 后续子 change |
| D3 内部 S 重切 | DM-016 已完成 |
| Hub-Spoke slice b–c | DM-018 v2.0 并行 |

---

## 5. L5 测试点（R1 登记，v1.0 PLANNED）

见 `design.md` §6 与 `t-registry` 增量草案。

---

**Revision History**

| 版本 | 日期 | 变更 |
|------|------|------|
| 0.1 | 2026-06-14 | 初稿 |
| 0.2 | 2026-06-14 | R1 决议闭合（OQ1–OQ4 采纳推荐） |
| 0.3 | 2026-06-15 | 双边共识落盘：状态推进 S3_Design；因果链 G-01 互引 DM-018 |
