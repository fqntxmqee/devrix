# Implementation Tasks: D1 切法 A 信号分层

**Change ID:** devrix-d1-sa-refine
**Demand ID:** DM-20260614-006

---

## Phase 0: S3-Gate

- [x] 0.1 确认切法 A + S13–S18 号段（design §2 Decision）
- [x] 0.2 三类信号 Scenario 覆盖 review
- [x] 0.3 Legacy 双轨表评审
- [x] 0.4 Draft PR + Reviewer **Approved**（2026-06-14）

---

## Phase 1: v1.0 Registry（无 Go）

- [x] 1.1 `layering.md` v3.4 — 价值流表 S13–S18 + Legacy FROZEN 表
- [x] 1.2 `a-registry.md` — canonical A（S13–S18）+ Legacy 映射列
- [x] 1.3 `f-registry.md` — Dispatch F01/F02/F03；Encode* F；EventBus F
- [x] 1.4 `t-registry.md` — PLANNED 新 T + **Canonical 列**（44 Legacy 行）
- [x] 1.5 `span-registry.md` — d1.signal.* / capture / critical
- [x] 1.6 `spec.md` + 根 `t-registry.md` 计数
- [x] 1.7 补全 design §9.1 Legacy T→canonical **44 行全表**

**Quality Gate:** AC1, AC2, AC3, AC5, AC7

---

## Phase 2: v1.1 契约与观测（代码）— 博弈共识 P0/P1

- [x] 2.1 `shared/contracts/im_outbound_signal.go`
  - Kind + **SourceEventID / ElapsedMs / InboundTurnID / Sequence**（AC9）
  - **禁止** Agent 自填 Confidence
- [x] 2.2 EngineEvent→Signal mapper（`signal/turn.go` + `MapEngineEventToSignal`）
- [x] 2.3 span 落地 + coverage registry
  - `d1.signal.chain_integrity`（AC10）
  - `d1.signal.task.work_proof`（AC11）
- [x] 2.4 `tests/acceptance/p0/d1_signal_journey_test.go`
  - Capture → Thinking → Task → Conclusion 全链 + Critical 必达
- [x] 2.5 S13 feedback 入站钩子（`/feedback ` → D5 span，不 Dispatch）
- [ ] 2.6 测试 `// T:` 可选迁移 canonical ID

**Quality Gate:** AC4, AC6, AC9, AC10, AC11

---

## Phase 3: v2.0 结构

- [x] 3.1 按 S14/S15/S16 拆 outbound → `present/` 包
- [x] 3.2 Milestone 迁 D7 → `orchestration/milestone/`
- [x] 3.3 Legacy Module Index 退役（layering v3.5）
- [x] 2.6 关键测试 `// T:` 迁移 canonical ID + error journey

---

## v2.0 完成（2026-06-14）

| 检查项 | 状态 |
|--------|------|
| present/ S14–S16 | ✅ |
| milestone → D7 | ✅ |
| layering v3.5 Package Map | ✅ |
| canonical T 12/12 IMPLEMENTED | ✅ |

---

## T 映射（canonical 新增）

| T ID | S | 描述 |
|------|---|------|
| D1-S13-A02-T01 | S13 | PersistUserTurn 成功 |
| D1-S13-A03-T01 | S13 | Dispatch F02 D7 |
| D1-S13-A03-T02 | S13 | Dispatch F01 Legacy |
| D1-S13-A04-T01 | S13 | Permission CRITICAL YOLO |
| D1-S14-A01-F01-T01 | S14 | thinking 首 chunk P99 |
| D1-S15-A02-F01-T01 | S15 | Worker 卡隔离 |
| D1-S16-A02-T01 | S16 | complete costly 送达 |
| D1-S16-A02-T02 | S16 | error costly 送达 |
| D1-S18-A01-F02-T01 | S18 | PublishCritical |
| D1-S18-A01-F03-T01 | S18 | Drain 不丢 Critical |

---

## Completion Checklist

- [x] S3-Gate Approved
- [x] Phase 1 merge（registry 已写入 openspec/specs）
- [x] Phase 2 v1.1 代码（S4）— 待 S5 验收
- [x] acceptance-report（S5）— ACCEPTED 2026-06-14

---

## S5 验收（2026-06-14）

| 检查项 | 状态 |
|--------|------|
| AC1–AC11 P0/P1 | ✅ 全绿 |
| acceptance-report.md | ✅ |
| **可进入 S6/S7** | **✅** 合 main 后归档 |
