# D1 Communication Domain

**Domain ID:** D1
**Slug:** `communication`
**Type:** Core Domain
**Status:** Active — Canonical S13–S18 (v1.0 registry, DM-20260614-006)
**Version:** 1.1.0
**Last Updated:** 2026-06-30
**Depends On:** D5 (Observability), D7 (`IOrchestrationEntry` — ingress 后唯一编排入口)
**Depended By:** D7 (EngineEvent 消费方展示), D6 (feedback 钩子消费)
**Hard Ban:** D1→D2 直连 `IEngine.Process`（DM-007，`routeLegacyD2` RETIRED）
**Cross-Domain SoT:** `../architecture/cross-domain-boundaries.md` §2.5
**Change:** DM-20260614-006 — 切法 A 双轨 / DM-20260628-003 (devrix-d1-dsaft-refactor) — DSAFT 边界 + Gateway 拆分 + contracts DTO + lint-d1-imports CI / **devrix-d1-ac-restructuring (DM-20260629-005) PR-4 #2 registry-sync — T 56 → 74 全量加 Span Evidence 列 + d1-span-coverage.sh CI 守门 (v1.1.0)**

---

## North Star

**作为 Trusted Intermediary，可靠地完成用户指令入站、三类出站信号呈现与弱网必达——不拥有编排、推理与执行。**

| 可验证承诺 | Canonical S | ValueFlow Alias (PR-5) |
|-----------|-------------|---------------------|
| 指令不丢、可追、可续聊 | D1-S13 CaptureUserIntent | `D1_Capture_User_Intent` |
| 思考过程可见（信号① Costly） | D1-S14 PresentThinking | `D1_Present_Thinking` |
| 任务/工具/Worker 进度可见（信号②） | D1-S15 PresentTaskProgress | `D1_Present_Task_Progress` |
| 结论/错误必达用户（信号③ Costly） | D1-S16 DeliverConclusion | `D1_Deliver_Conclusion` |
| 多 IM 平台结构一致 | D1-S17 ConnectChannel | `D1_Connect_Channel` |
| 背压/弱网下 Critical 不丢 | D1-S18 GuaranteeDelivery | `D1_Guarantee_Delivery` |

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
| T | **74（42 P0，含 Legacy 19 + Canonical 23）** | `t-registry.md` |
| Span | 22 ops | `span-registry.md` |

> **T 数变更说明（DM-20260629-005 PR-4 #2 registry-sync）：** 56 → 74，新增强化显式登记覆盖：S19 Transcript (4) + S5-A07 feishu precheck (4) + S3-A08 error_code (1) + RF-T01..T09 (9) + Legacy S1-S12 真实归档 (44 vs 之前 26) → 总 74。Span Evidence 列覆盖 100% effective（`scripts/d1-span-coverage.sh` 守门）。

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
| `../architecture/d1-flow-architecture.md` | 价值流流图 + Package Map + Legacy 包结构 + 跨域接线 |
| `../../scripts/d1-span-coverage.sh` | Span Evidence 覆盖率守门（PR-4 落地，≥80% effective PASS） |
| `openspec/archive/2026-06-30-devrix-d1-ac-restructuring/legacy-s1-s12.md` | Historical S1–S12 frozen index（PR-4 #2 沉 archive） |

---

## 修订记录

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-06-16 | 初版：North Star、Out of Scope、文档索引；对齐 D2/D4 `*-domain.md` 模式 |
| 1.1.0 | 2026-06-30 | **DM-20260629-005 PR-4 #2 registry-sync**：T 56 → 74 + 全量 Span Evidence 列（覆盖率 100% effective）+ `scripts/d1-span-coverage.sh` CI 守门 + Historical S1–S12 沉 `openspec/archive/2026-06-30-devrix-d1-ac-restructuring/legacy-s1-s12.md` + §Change line |
