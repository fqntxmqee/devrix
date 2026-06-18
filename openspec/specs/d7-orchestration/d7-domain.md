# D7 Orchestration Domain

**Domain ID:** D7
**Slug:** `orchestration`
**Type:** Core Domain
**Status:** Active — Canonical S1–S5（v1.0+ value-stream，无需像 D1 重编号）
**Version:** 1.1.0
**Last Updated:** 2026-06-18
**Depends On:** D1（ingress `ProcessMessage`）、D2（Follower 拆面）、D3（`IGateway`，D7 直调）、D4（Delegate Follower）
**Depended By:** D1（EngineEvent / Flow 展示）、D6（`ValidateOrchestration` advisory）
**Hard Ban:** D1→D2 直连 `IEngine.Process`（DM-007）；D2→D3 import（DM-020）；D4 直 Publish FlowEvent（DM-018）
**Cross-Domain SoT:** `../architecture/cross-domain-boundaries.md` §2.4 / §3.1 · `../d2-context-engine/d7-boundary.md`

---

## North Star

**作为 Orchestration Mediator / Turn Leader，决定做什么、按什么顺序、谁来做，并把执行进度信号化送达 D1——不拥有 Session 上下文与 Agent 生命周期。**

| 可验证承诺 | Canonical S |
|-----------|-------------|
| Task/Plan 事实与状态机单一权威 | D7-S1 Work Model（**WorkItem + WorkTree**，`run_ref` → RunRegistry） |
| 用户消息统一入口 + Turn 主循环 + LLM 调用权 + **RunTurn resolve/decompose/await** | D7-S2 Session Orchestrator |
| 多 Worker 并行 DAG，冲突与上下文隔离 | D7-S3 Wave Scheduler |
| FlowEvent / WorkPlan 聚合，进度广播 | D7-S4 Execution Flow |
| 意图分类、任务拆解、执行器选择 | D7-S5 Decision & Planning |

---

## Out of Scope

| 能力 | 归属 | 备注 |
|------|------|------|
| IM ingress / 卡片呈现 | D1 | D7 只消费 `InboundMessage`，产出 `EngineEvent` |
| Session 上下文 / 工具沙箱 / Persist | D2 | D7 拆面调用 Prepare / ToolRound / Persist |
| LLM Gateway 实现 / Breaker 执行 | D3 | D7 **拥有调用决策权**（DM-020），经 `InvokeLLM` |
| Worker 执行体 / Agent 生命周期 | D4 | D7 Dispatch → D4 RunAgent |
| 结论质量 / 信誉 | D6 | D7 可被 advisory 校验，不阻塞 |
| 可观测性基础设施 | D5 | D7 发 span，D5 聚合 |

---

## DSAFT 资产

### Canonical 价值流 — D7-S1–S5

| S ID | Scenario | 博弈角色 | Status |
|------|----------|----------|--------|
| D7-S1 | Work Model | State Authority | IMPLEMENTED |
| D7-S2 | Session Orchestrator | Screening + Turn Leader | IMPLEMENTED |
| D7-S3 | Wave Scheduler | Mechanism Designer | IMPLEMENTED |
| D7-S4 | Execution Flow | Costly Signaler | IMPLEMENTED |
| D7-S5 | Decision & Planning | Information Producer | IMPLEMENTED |

### 登记规模（Canonical）

| 层 | 数量 | SoT 文件 |
|----|------|----------|
| A | 24（S1:6 · S2:6 · S3:3 · S4:5 · S5:4） | `a-registry.md` |
| F | 51（Legacy 44 + Canonical 7） | `f-registry.md` |
| T | 69（47 P0） | `t-registry.md` |
| Span | 9 ops | `span-registry.md` |

---

## 规格文档索引

| 文档 | 用途 |
|------|------|
| `spec.md` | Gherkin 验收规格 |
| `terminal-state-guide.md` | 终态流程、IntentKind 四链、A→F 编排树、跨域时序 |
| `observability-guide.md` | Span↔T、Trace 树、FastPath SLA、P0 Runbook |
| `design.md` | 六段式详细设计（Wave、Hub、PlanMode 等） |
| `d7-requirements-clarifications.md` | Review R1/R2 完整澄清（历史归档） |
| `dsaft-architecture.md` | Stub — DSAFT 五层计数 |
| `a-registry.md` / `f-registry.md` / `t-registry.md` | A/F/T 登记 SoT |
| `span-registry.md` | Span operation 登记 SoT |
| `layer-delta.md` | V1→V2 演进 Delta |
| `../../tech-debt/worktree-v2-deferred.md` | WorkTree v2.1+ 技术债务（TD-WT-01..06） |
| `../architecture/code-layout.md` §4.2 | scenario-slug 物理路径 |

---

## 修订记录

| Version | Date | Changes |
|---------|------|---------|
| 1.1.0 | 2026-06-18 | DM-20260617-009 闭环：WorkItem/WorkTree 写入 North Star；RunTurn resolve/decompose/await；tech-debt 索引 |
| 1.0.0 | 2026-06-16 | 初版：薄领域 SoT；厚版迁至 `d7-requirements-clarifications.md`；对齐 D1 `*-domain.md` 模式 |
