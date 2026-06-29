# Demand: devrix-d7-6s-observe-merge-cancel (DM-20260626-006)

**Demand ID:** DM-20260626-006
**Status:** S1_Cancelled
**Priority:** P3
**Created:** 2026-06-26
**Change ID:** devrix-d7-6s-observe-merge-cancel
**Related:** devrix-d7-six-s-simplification (DM-20260626-001) · devrix-d7-6s-package-merge (DM-20260626-004) · devrix-d7-6s-verify-promotion (DM-20260626-005)

---

## §1 背景

v6.0.0 域升级 follow-up 序列原始规划（见 `openspec/archive/2026-06-26-devrix-d7-six-s-simplification/tasks.md` §"后续 follow-up"）包含 6 个 follow-up PR：

| # | change-id | 内容 | 状态 |
|---|-----------|------|------|
| #1 | `devrix-d7-six-s-simplification` | 14 S → 6 S 博弈角色对齐 | ✅ S7_Archived (PR #215) |
| #2 | `devrix-d7-mups-package-migration` | execute/ + learn/ → mups/ 子树物理迁移 | ✅ S7_Archived (PR #216+#217) |
| #3 | `devrix-d7-hardening-cross-cutting` | hardening/ 横切包物理落地 | ✅ S7_Archived (PR #218+#219) |
| #4 | `devrix-d7-6s-package-merge` | turn/ → sessionorchestrator/ 整包合并 | ✅ S7_Archived (PR #220+#221) |
| #5 | `devrix-d7-6s-verify-promotion` | exit_reason + verdict_to_exit_reason promote | ✅ S7_Archived (PR #222+#223) |
| **#5'** | **`devrix-d7-6s-observe-merge`** | **`observe/orchtypes/` → `decisionplanning/`** | **❌ S1_Cancelled (本次)** |
| #6 | `devrix-d7-6s-bootstrap-slim` | wire 14 → 6 收口 | ⏳ 待启动 |

## §2 Cancel 原因（核心）

原 follow-up #5 (`devrix-d7-6s-observe-merge`) 的 scope 假设存在一个独立的 `orchestration/observe/orchtypes/` 目录需要被物理合并到 `decisionplanning/`。

**但实际 v6.0.0 实施过程中（具体见 `openspec/archive/2026-06-26-devrix-d7-mups-v4-phase2-observe-plan/` PR-A1 + PR-RF + PR-B1 + PR-C1 + PR-C2 的多 PR 落地路径），Observe 相关类型从一开始就被设计性放置在 `orchestration/orchtypes/` 共享类型包中，而非独立的 `observe/orchtypes/` 目录**。

### 验证证据（2026-06-26 实测）

```bash
# 1. observe/ 目录从未存在
$ ls internal/layers/orchestration/observe/
ls: cannot access 'observe/': No such file or directory

# 2. git history 中也从未有过 observe/orchtypes/
$ git log --all --oneline -- 'internal/layers/orchestration/observe/*'
(空，无任何 commit)

# 3. Observe 类型当前物理位置
$ ls internal/layers/orchestration/orchtypes/ | grep -iE "observe|uncertainty|intent_quantize|anomaly"
anomaly_detector.go
anomaly_detector_test.go
intent_quantizer.go
intent_quantizer_test.go
observation.go
observation_test.go
observe_request.go
observe_request_test.go
system_anomaly_wiring.go
system_anomaly_wiring_test.go
uncertainty_coord.go
uncertainty_coord_test.go
uncertainty_report.go
uncertainty_report_test.go

# 4. Observe 类型在 orchtypes/ 被广泛使用
$ grep -rln "orchtypes\.ObserveRequest\|orchtypes\.UncertaintyReport\|orchtypes\.IntentQuantizer\|orchtypes\.AnomalyDetector" internal/ | wc -l
17 个文件跨 decisionplanning/ + sessionorchestrator/ + executionflow/verify/
```

### 设计意图分析

`orchtypes/` 作为 D7 跨包共享类型包（类比 D2 的 `internal/shared/types/`），其设计原则是 **"类型归属与实现包解耦"**：

| 类型类别 | 物理位置 | 消费者 |
|----------|----------|--------|
| Config / Routing / ProcessRequest | `orchtypes/` | 14+ 调用方 |
| ObserveRequest / UncertaintyReport / IntentQuantizer / AnomalyDetector | `orchtypes/` | 17+ 调用方（sessionorchestrator/ + decisionplanning/ + executionflow/verify/） |
| LLMInvoker / ToolSchema | `orchtypes/llm_invoker.go` | sessionorchestrator/ + bootstrap/ |
| Intent types (IntentKind, IntentClassification) | `orchtypes/intent.go` | sessionorchestrator/ + decisionplanning/ |
| ArtifactKind / SideEffectStatus | `orchtypes/` | mups/ + wavescheduler/ |

**关键观察：Observe 类型被 17+ 文件跨 3 个 S 层（S2 sessionorchestrator + S5 decisionplanning + S4 executionflow/verify）使用，天然属于"共享类型"范畴，不应被归属到任一 S 层的实现包**。

如果把 Observe 类型从 `orchtypes/` 移到 `decisionplanning/`：
1. `sessionorchestrator/` 必须 import `decisionplanning/` 来获取 `ObserveRequest`（**破坏当前 sessionorchestrator/ 0 依赖 decisionplanning/ 的清洁边界**）
2. `executionflow/verify/` 必须 import `decisionplanning/` 来获取 `UncertaintyReport` 和 `AnomalyDetector`（**产生 S4 → S5 反向依赖**）
3. `decisionplanning/` 自身使用 `UncertaintyReport.ComputeOverallStrength` 但其作为共享类型被 S2 + S4 共同消费 — 类型归属应保持中性

**结论**：原 follow-up #5 的 scope 设计与 v6.0.0 实施实际状态不一致，**真实状态就是 Observe 类型正确地归属在 `orchtypes/` 共享包**，无需迁移。

## §3 取消方式

按 OpenSpec 规范，DM-20260626-006 状态标记为 `S1_Cancelled`，遵循以下处理：

1. **不进入 S2 提案阶段**（无需 proposal.md + design.md + tasks.md）
2. **不创建 S6 归档**（Cancel 状态不入 `archive/` 目录）
3. **仅维护 `demand.md` + `.openspec.yaml` 两个文件** 留存决策痕迹
4. **`demand-archive-index.md` 添加一行** 标记 Cancelled
5. **后续 follow-up #6 (bootstrap-slim) 不再依赖本 change**

## §4 后续 follow-up 序列状态

| # | change-id | 状态 | PR |
|---|-----------|------|-----|
| #1 | `devrix-d7-six-s-simplification` | ✅ S7_Archived | #215 |
| #2 | `devrix-d7-mups-package-migration` | ✅ S7_Archived | #216+#217 |
| #3 | `devrix-d7-hardening-cross-cutting` | ✅ S7_Archived | #218+#219 |
| #4 | `devrix-d7-6s-package-merge` | ✅ S7_Archived | #220+#221 |
| #5 | `devrix-d7-6s-verify-promotion` | ✅ S7_Archived | #222+#223 |
| **#5'** | **`devrix-d7-6s-observe-merge`** | **❌ S1_Cancelled** (本次, DM-20260626-006) | — |
| #6 | `devrix-d7-6s-bootstrap-slim` | ⏳ 启动中 (DM-20260626-007) | — |

**结论**：v6.0.0 follow-up 6 个 PR 中 5 个 S7_Archived + 1 个 S1_Cancelled + 1 个 S1_Activating（#6 bootstrap-slim）。完整 follow-up 序列即将收官。

## §5 Cancel 文档用途

本文件作为 OpenSpec R&D 流程中 **"变更被主动取消"** 的标准模板，未来可被以下场景复用：
- v6.0.1+ 任何变更在调研阶段发现 scope 假设与实际状态不一致
- 任何"按原计划应迁移但实际未发生"的 follow-up
- 任何"调研后决定不实施"的变更

---

## 修订记录

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-06-26 | 初版：observe/orchtypes/ 从未存在 + Observe 类型正确归属在 orchtypes/ 共享包, S1_Cancelled 状态文档化 |
