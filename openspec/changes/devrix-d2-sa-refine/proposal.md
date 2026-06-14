# D2 上下文引擎 S 层重构 — S2 Proposal

**Change ID:** devrix-d2-sa-refine
**Demand ID:** DM-20260614-009
**阶段:** S2 Proposal
**版本:** v1.0

---

## 1. 现状快照

| 指标 | 数值 | 说明 |
|------|------|------|
| Legacy Module S | 14 | S1–S14（S1 RETIRED） |
| Canonical 价值流 S | 0 | 无按执行生命周期切的 S |
| IMPLEMENTED T | ~80+ | 分散在 module S 下 |
| 跨域边界漂移 | 4+ | tasks/, delegate_tools, queue/, plan_agent |
| loop.go 行数 | ~268 | 含 D7 编排 Hook 字段 |

---

## 2. North Star 确认

**D2 = 会话内 LLM↔Tool 执行原语（D7 的 Follower）：准备上下文 → 跑 QueryLoop → 持久化状态。**

### 2.1 与 D7 的关系（Leader / Follower）

| | D7 Leader | D2 Follower |
|---|-----------|-------------|
| 入口 | D1 → `ProcessMessage` | 仅经 `d2Executor` |
| 决策 | Classify / Wave / Executor | 无 |
| 执行 | 调用 QueryLoopExecutor | S15→S16→S17 |
| 进度 | FlowEvent → D1（S4） | EngineEvent → D7 |

**对称原则（对 DM-008）：** D7「S2 入口确定性 > S5 决策」；D2「S16 机制确定性 > S10 功能堆叠」。

跨域 SoT：`openspec/specs/d2-context-engine/d7-boundary.md`

| 承诺 | 可验证方式 |
|------|-----------|
| 会话可恢复 | Snapshot Deserialize + Process 继续 |
| 长对话可继续 | 压缩后 token ≤ budget |
| 工具回合有序 | tool_result 与 tool_use 配对 |
| 完成可信赖 | complete 在 snapshot 写入后 |
| 权限先于执行 | write 被拒时不落盘 |

**Out of Scope（明确不归 D2 SoT）：**

| 能力 | 归属 |
|------|------|
| 消息 ingress / IM 信号 | D1 |
| ProcessMessage / Wave / ClassifyIntent | D7 |
| Task 写模型 / PlanMode / PlanAgent | D7-S1/S5 |
| Agent 委托 / delegate_* 路由 | D7 + D4 |
| FlowEvent / WorkPlan 聚合 | D7-S4 |
| 结论质量 / 信誉 | D6 |

---

## 3. Proposed Solution — 切法 A：六场景 + Legacy 双轨

### 3.1 价值流 Scenario（Canonical，D2-S15–S20）

| S ID | Scenario | 系统/用户目标（一句话） | 博弈角色 |
|------|----------|------------------------|----------|
| **D2-S15** | PrepareExecutionContext | 每次 turn 前，会话上下文已加载、修复、压缩、Prompt 已组装 | Pre-play setup |
| **D2-S16** | RunQueryLoop | LLM↔Tool 多轮执行直到无 pending tool 或达 max_turns | **Follower 核心机制** |
| **D2-S17** | PersistSessionState | turn 成功后快照/transcript/记忆 durable 写入 | Costly completion signal |
| **D2-S18** | EnforceExecutionPolicy | 工具权限、沙箱、工具面、Plan 写路径约束 | Mechanism constraint |
| **D2-S19** | NestedExecution | SubQuery / Background / Sidechain / Fork 嵌套执行 | Nested sub-game |
| **D2-S20** | LegacyHarnessFallback | 显式 `query_loop.enabled=false` 时的 Harness 路径 | Legacy equilibrium |

**横切（不单独占 S）：**

- Token 计数 → 编入 S15 Prepare（F 层 `CountTokens`）
- Mock Engine → 测试辅助，保留 Legacy S14

### 3.2 Legacy Module Index（冻结，D2-S1–S14）

旧编号 **不重定义语义**，仅作 module/包追溯与现有 T 注释锚点。见 design.md §8。

| Legacy S | Module | Canonical 映射 |
|----------|--------|----------------|
| D2-S2 | Compression | → S15 |
| D2-S3, S6 | Memory, Snapshot | → S15 + S17 |
| D2-S4 | Token | → S15 |
| D2-S5, S8 | Registry, Sandbox | → S18 |
| D2-S7 | Prompt | → S15 |
| D2-S9 | Harness | → S20 |
| D2-S10 | QueryLoop（整体） | → S16 + S19 + 部分 S18 |
| D2-S11 | Queue | → **Out of Scope → D7-S4** |
| D2-S12 | Worktree | → S18（沙箱策略 F） |
| D2-S13 | Conversation | → S15 |
| D2-S14 | Mock | Legacy 保留 |

### 3.3 与 D7 的接口契约（v1.0 声明，v1.1 测试）

```text
D7.ProcessMessage
    └── d2Executor.RunQueryLoop  (D2-S16-A01)
            ├── PrepareExecutionContext (D2-S15) — engine.go 编排
            ├── RunQueryLoop (D2-S16) — query/loop.go
            └── PersistSessionState (D2-S17) — engine.go 编排
```

D7 **注入** LoopHooks / SessionQueue；D2 **不** import D7。

### 3.4 分阶段

| 版本 | 范围 |
|------|------|
| v1.0 | S15–S20 registry + Gherkin + Legacy 双轨 + 跨域清单 |
| v1.1 | Canonical span 名 + D2 Thin 契约测试（loop 无 D4 import） |
| v2.0 | tasks/ → D7；delegate_tools 移除；scenario 物理路径 |

---

## 4. Success Metrics

| 指标 | 目标 |
|------|------|
| Canonical S 注册 | 6/6（S15–S20） |
| Legacy S 冻结 | 14/14 有 canonical 映射或 Out of Scope |
| Gherkin Scenario | 每 Canonical S ≥1 |
| IMPLEMENTED T 可追溯 | 100% canonical 列 |
| 跨域漂移登记 | 4/4 有 D7 目标 |

---

## 5. Implementation Plan

| Phase | 内容 | OpenSpec |
|-------|------|----------|
| P0 | Registry + design + S3-Gate | S2–S3 |
| P1 | v1.0 merge 至 `openspec/specs/` | S7 前 delta |
| P2 | v1.1 span + 边界测试 | 独立 change |
| P3 | v2.0 物理迁移 | 与 D7 v2.0 联动 |

---

## 6. Open Questions

| # | 问题 | 默认决策 |
|---|------|----------|
| Q1 | S11 Queue 是否保留 D2 Legacy？ | 保留 Legacy 追溯；Canonical → D7-S4 |
| Q2 | worktree 归 S18 还是 S19？ | S18（执行策略/隔离） |
| Q3 | TaskTools 何时迁 D7？ | v2.0，v1.0 仅 Out of Scope |
