# D1 Communication Domain

**Domain ID:** D1
**Slug:** `communication`
**Type:** Core Domain
**Status:** Active — Canonical S13–S18 (v1.0 registry, DM-20260614-006)
**Version:** 1.0.0
**Last Updated:** 2026-06-16
**Depends On:** D5 (Observability), D7 (`IOrchestrationEntry` — ingress 后唯一编排入口)
**Depended By:** D7 (EngineEvent 消费方展示), D6 (feedback 钩子消费)
**Hard Ban:** D1→D2 直连 `IEngine.Process`（DM-007，`routeLegacyD2` RETIRED）
**Cross-Domain SoT:** `../architecture/cross-domain-boundaries.md` §2.5

---

## North Star

**作为 Trusted Intermediary，可靠地完成用户指令入站、三类出站信号呈现与弱网必达——不拥有编排、推理与执行。**

| 可验证承诺 | Canonical S |
|-----------|-------------|
| 指令不丢、可追、可续聊 | D1-S13 CaptureUserIntent |
| 思考过程可见（信号① Costly） | D1-S14 PresentThinking |
| 任务/工具/Worker 进度可见（信号②） | D1-S15 PresentTaskProgress |
| 结论/错误必达用户（信号③ Costly） | D1-S16 DeliverConclusion |
| 多 IM 平台结构一致 | D1-S17 ConnectChannel |
| 背压/弱网下 Critical 不丢 | D1-S18 GuaranteeDelivery |

---

## Out of Scope

| 能力 | 归属 | 备注 |
|------|------|------|
| Turn 主循环 / 意图分类 | D7 | `ProcessMessage` + `ClassifyIntent` |
| 上下文准备 / 工具执行 | D2 | D7 调度 Follower |
| LLM 调用 / 内容过滤 | D3 | D7 直调 |
| Worker 派发 / FlowEvent 写侧 | D7-S4 | `GatewaySink` 只读展示 |
| 结论质量 / 信誉计算 | D6 | D1 仅 feedback 钩子 + 客观锚点 |
| Task/Plan 写模型 | D7-S1 | milestone 展示数据来自 D7 |

---

## DSAFT 资产

### Canonical 价值流 — D1-S13–S18

| S ID | Scenario | Status |
|------|----------|--------|
| D1-S13 | CaptureUserIntent | IMPLEMENTED |
| D1-S14 | PresentThinking | IMPLEMENTED |
| D1-S15 | PresentTaskProgress | IMPLEMENTED |
| D1-S16 | DeliverConclusion | IMPLEMENTED |
| D1-S17 | ConnectChannel | IMPLEMENTED |
| D1-S18 | GuaranteeDelivery | IMPLEMENTED |

Legacy D1-S1–S12 已退役（DM-20260614-006 Phase 3）。

### 登记规模（Canonical）

| 层 | 数量 | SoT 文件 |
|----|------|----------|
| A | 16 | `a-registry.md` |
| F | 18 | `f-registry.md` |
| T | 56（26 P0） | `t-registry.md` |
| Span | 22 ops | `span-registry.md` |

---

## 规格文档索引

| 文档 | 用途 |
|------|------|
| `spec.md` | Gherkin 验收规格 |
| `terminal-state-guide.md` | 终态流程、A→F 编排树、IntentKind 时序、信号映射 |
| `observability-guide.md` | Span↔T、Trace 树、EventBus 必达、验收 Runbook |
| `design.md` | 六段式详细设计（EventBus、CardKit、Permission 等） |
| `dsaft-architecture.md` | Stub — DSAFT 五层计数；明细见本文件与 Guides |
| `a-registry.md` / `f-registry.md` / `t-registry.md` | A/F/T 登记 SoT |
| `span-registry.md` | Span operation 登记 SoT |
| `layer-delta.md` | V1→V2 演进 Delta |
| `../architecture/code-layout.md` §4.1 | scenario-slug 物理路径 |

---

## 修订记录

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-06-16 | 初版：North Star、Out of Scope、文档索引；对齐 D2/D4 `*-domain.md` 模式 |
