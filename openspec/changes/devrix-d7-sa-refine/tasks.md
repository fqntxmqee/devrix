# Implementation Tasks: D7 S 层重构（devrix-d7-sa-refine）

**Change ID:** devrix-d7-sa-refine
**Demand ID:** DM-20260614-008
**版本范围:** v1.0（Registry）→ v1.1（T 锚定 + S5 routing）

---

## Phase 0: S3-Gate ✅

- [x] 0.1 S 切法 A 确认（design §2 Decision）
- [x] 0.2 S 编号方案：复用现有 S2/S5
- [x] 0.3 D7-S3/S4 角色修订（Worker = D7-S3+S4 → 调度机制）
- [x] 0.4 Legacy 双轨方案评审
- [x] 0.5 Grill Review 通过
- [x] 0.6 S3-Gate **Approved**（2026-06-14）

---

## Phase 1: v1.0 Registry（无 Go）✅

- [x] 1.1 `a-registry.md` v3.0 — Legacy 双轨建立 + Canonical S2/S3/S4/S5 按用户价值流
- [x] 1.2 `f-registry.md` v3.0 — Canonical F 层（D7-S2/D7-S5）
- [x] 1.3 `t-registry.md` v2.4 — 新增 T03 anti-fabrication + S5-A01-T01/T02/S5-A02-T01
- [x] 1.4 `design.md` — S5 Explore 输入语义补充

**Quality Gate:** S3-Gate Approved ✅

---

## Phase 2: v1.1 T 锚定 + S5 routing（代码）

### 2.1 D7-S2 ProcessMessage T 锚定

- [x] 2.1.1 **D7-S2-A01-T01** — ProcessMessage 端到端延迟 ≤ 2ms（P0）
  - 入口响应承诺
  - 测试：`orchestrator_test.go` 性能测试（TestSessionOrchestrator_FastPath_EndToEnd_Latency）
- [x] 2.1.2 **D7-S2-A01-T02** — FastPath 命中时无 Wave 创建（P0）
  - screening 完整性
  - 测试：`orchestrator_test.go` TestSessionOrchestrator_FastPath_NoWaveScheduled
- [x] 2.1.3 **D7-S2-A01-T03** — 禁止在 Worker terminal FlowEvent 前伪造 Task 进度（P0）
  - anti-fabrication commitment
  - 测试：`orchestrator_test.go` TestSessionOrchestrator_AntiFabrication_NoSyntheticProgress

### 2.2 D7-S5 routing 实现

- [x] 2.2.1 **D7-S5-A01-T01** — 规则分类置信度阈值验证（P0）
  - screening 可重复性
  - 测试：`classifier_test.go` 置信度阈值测试（Skip=100, Command=100, Fast≥70）
- [ ] 2.2.2 **D7-S5-A02-T01** — SynthesizeTaskGraph 产出有效 DAG（P1）
  - 结构决策验证
  - 输入：吸收 Explore Workers FlowEvent
  - 测试：新增 `decomposer_test.go`

### 2.3 D7-S2-A03-T01 ✅（已实现）

- [x] HandleInterrupt 中断顺序正确（Wave→D4→Process→stopped→TaskCancel）
- 状态：t-registry.md 已标记 IMPLEMENTED

### 2.4 D7-S5-A01-T02 ✅（已实现）

- [x] Command-first 优先于 LLM 分类（用户显式策略优先）
- 状态：t-registry.md 已标记 IMPLEMENTED

**Quality Gate:** AC1, AC2, AC3, AC4, AC5, AC6

---

## Phase 3: v1.1 D7-S5-A03 SelectExecutor（规划）

- [ ] 3.1 SelectExecutor 实现：explore→D2 execute→D4
- [ ] 3.2 D7-S5-A03-T01 — executor 选择路由正确性

---

## Phase 4: v2.0 D7-S1 Task 模型归 D7

- [ ] 4.1 Task 模型从 D2 迁入 D7 `orchestration/coordinator/workmodel.go`
- [ ] 4.2 跨 D2 边界消除验证

---

## T 映射（v1.1 PLANNED 新增）

| T ID | 描述 | S | 优先级 | 状态 |
|------|------|---|--------|------|
| D7-S2-A01-T01 | ProcessMessage 端到端延迟 ≤ 2ms | S2 | P0 | PLANNED |
| D7-S2-A01-T02 | FastPath 命中时无 Wave 创建 | S2 | P0 | PLANNED |
| D7-S2-A01-T03 | 禁止在 Worker terminal FlowEvent 前伪造 Task 进度 | S2 | P0 | PLANNED |
| D7-S5-A01-T01 | 规则分类置信度阈值验证 | S5 | P0 | PLANNED |
| D7-S5-A02-T01 | SynthesizeTaskGraph 产出有效 DAG（吸收 Explore 结果） | S5 | P1 | PLANNED |

---

## 下一步清单

- [x] S3-Gate Review 通过
- [x] v1.0 Registry merge
- [x] v1.1 tasks 创建
- [x] v1.1 测试代码（S4 实现）
- [x] S4-Gate Review — Conditionally Approved
- [x] **S5 验收**（go test -race 全量 PASS）
- [x] S6 归档（→ archive/2026-06-14-devrix-d7-sa-refine/）

---

**v1.1 开始日期：** 2026-06-14
