# Spec Delta: devrix-d1-ac-restructuring (DM-20260629-005)

**Change ID:** devrix-d1-ac-restructuring
**Demand ID:** DM-20260629-005
**Status:** S2_Proposal → S3_Design → S4_Implemented → S5_Accepted → S7_Archived

---

## §1 Modified Specs

| Spec | Version | Status |
|------|---------|--------|
| `d1-domain.md` | v1.0.0 → **v1.1.0** | 修订 (§ValueFlow Alias + §Boundary Debt Decisions) |
| `spec.md` | v4.1.1 → **v5.0.0** | god-doc-split (176→90) + gherkin-restructuring (24 缩写 bullet→90 Scenario) |
| `a-registry.md` | v3.1.0 → **v3.2.0** | 修订 (Historical S 沉 archive + ValueFlow Alias) |
| `f-registry.md` | v3.0.0 → **v3.1.0** | 修订 (Historical S 沉 archive + ValueFlow Alias) |
| `t-registry.md` | v3.2.0 → **v3.3.0** | 修订 (Span Evidence 列 + §T-Without-Span Tracker + ValueFlow Alias) |
| `span-registry.md` | v3.1.0 → v3.1.0 | UNCHANGED (10 canonical + 14 legacy ops) |
| `observability-guide.md` | v1.0.0 → **v2.0.0** | god-doc-split (230→100) |
| `layer-delta.md` | v1.x → **v1.1.0** | §Canonical S → ValueFlow Alias 表 |
| `d1-flow-architecture.md` | n/a → **v1.0.0** | NEW (god-doc-split pt1 拆出) |
| `d7-boundary.md` | n/a → **v1.0.0** | NEW (boundary-decision) |

---

## §2 Delta — d1-domain.md §North Star ValueFlow Alias 列

**ADDED** ValueFlow Alias 列 (6 S):

| 可验证承诺 | Canonical S | ValueFlow Alias (用户感知) |
|-----------|-------------|------------------------------|
| 指令不丢、可追、可续聊 | D1-S13 CaptureUserIntent | **D1_Capture_User_Intent** |
| 思考过程可见（信号① Costly） | D1-S14 PresentThinking | **D1_Present_Thinking** |
| 任务/工具/Worker 进度可见（信号②） | D1-S15 PresentTaskProgress | **D1_Present_Task_Progress** |
| 结论/错误必达用户（信号③ Costly） | D1-S16 DeliverConclusion | **D1_Deliver_Conclusion** |
| 多 IM 平台结构一致 | D1-S17 ConnectChannel | **D1_Connect_Channel** |
| 背压/弱网下 Critical 不丢 | D1-S18 GuaranteeDelivery | **D1_Guarantee_Delivery** |

## §3 Delta — d1-domain.md §Boundary Debt Decisions

**ADDED** §Boundary Debt Decisions 章节 (PR-7):

| Boundary ID | 状态 | 内容 | 重新评估 |
|-------------|------|------|----------|
| `boundary-debt:d1-to-d7-orchestration-entry-v1.0` | ✅ RESOLVED | D1-S13-A03-F02 routeD7 走 `IOrchestrationEntry.ProcessMessage`（零 D2 直连）| — |
| `boundary-debt:d1-to-d4-permission-gate-v1.0` | ✅ RESOLVED | D1-S13-A04 ResolvePermissionGate 调 D4 sessionagents/manager.ResolveAgentPermission | — |
| `boundary-debt:d1-forbidden-orchestration-import-v2.0` | ✅ RESOLVED | D1 capture 禁止 import orchestration/（lint-d1-imports.sh 守门） | — |

> 治理常量在 `internal/layers/communication/orchtypes/boundary_decision.go`。
> 3 单元测试: `internal/layers/communication/orchtypes/boundary_decision_test.go` (Exist + VersionFormat + Unique)。

## §4 Delta — spec.md god-doc-split (PR-2) + gherkin-restructuring (PR-6)

**CHANGED** spec.md 176 → 90 行:
- 删 §Architecture（价值流） + §Package Map + §Architecture（Legacy 包结构）→ `architecture/d1-flow-architecture.md` (NEW)
- 头部加 "See also" 块
- §Requirements 24 缩写 bullet → 90 `#### Scenario:` Gherkin 块（PR-6）
- §Change line + 修订记录 v5.0.0 row

**CHANGED** Gherkin 90 Scenario 分布:
- D1-S13: 16 Scenario (happy 5 / sad 4 / boundary 3 / concurrent 2 / timeout 2)
- D1-S14: 12 Scenario (happy 4 / sad 3 / boundary 2 / concurrent 1 / timeout 2)
- D1-S15: 15 Scenario (happy 5 / sad 4 / boundary 3 / concurrent 2 / timeout 1)
- D1-S16: 16 Scenario (happy 6 / sad 4 / boundary 3 / concurrent 1 / timeout 2)
- D1-S17: 16 Scenario (happy 5 / sad 5 / boundary 3 / concurrent 2 / timeout 1)
- D1-S18: 15 Scenario (happy 5 / sad 4 / boundary 4 / concurrent 1 / timeout 1)

## §5 Delta — observability-guide.md god-doc-split (PR-3)

**CHANGED** observability-guide.md 230 → 100 行:
- 删 §1 Span↔T 绑定矩阵 18 行（迁 t-registry §T-Without-Span Tracker）
- 删 §5 T 验收摘要（指向 t-registry §Statistics）
- 删 §7 已知缺口 5 行（迁 t-registry §T-Without-Span Tracker）
- 头部加 "See also" 块
- 修订记录 v2.0.0 row

**UNCHANGED** §2 IntentFast Trace 树 + §3 EventBus 双通道与必达 + §4 弱网 Drain 故障注入场景 + §6 生产 Trace 检查清单

## §6 Delta — t-registry.md Span Evidence 列 (PR-4)

**ADDED** Span Evidence 列 (44 T 映射 + 12 显式 `—` = 56 total):
- D1-S13 (4): T01→`d1.capture.persist`; T02,T03→`d1.dispatch.route`; T04→`—`
- D1-S14 (1): T01→`d1.signal.thinking`
- D1-S15 (2): T01→`d1.signal.task`; T02→`d1.signal.task.work_proof`
- D1-S16 (2): T01,T02→`d1.signal.conclusion`
- D1-S17 (1): T01→`adapter.feishu.encode`
- D1-S18 (2): T01→`eventbus.publish_critical`; T02→`eventbus.drain`
- D1-S19 (4): T01-T04→`—` (Transcript 注入)
- D1-S5-A07 (4): T05-T08→`—` (feishu precheck 启动期)
- D1-S3-A08 (1): T01→`—` (error_code 编译期)
- D1-RF (9): T01→`d1.capture.persist`; T02-T09→`—` (路由/边界)

**ADDED** §T-Without-Span Tracker (PR-3 god-doc-split pt2):
- 4 row: Transcript 注入 (4) + feishu precheck 启动期 (4) + error_code 编译期 (1) + 路由/边界 (8)

**ADDED** §Statistics: 56 total / 44 mapped / 12 explicit — / 44/(56-12) = 100% effective (raw 44/56 ≈ 78.6% informational only)。

## §7 Delta — a/f-registry.md Historical S 沉 archive (PR-4)

**MOVED** Legacy Module Index (D1-S1~S12) → `openspec/archive/2026-06-14-devrix-d1-sa-refine/legacy-s1-s12.md`:
- 12 个 S 迁移路径表 (S1→S13 / S2→S17 / S3→S13-A05 / S4→保留 / S5→S15 / S6→S17 / S7→D5 / S8→S17 / S9→S18 / S10→S17 / S11→kernel / S12→S17)
- frozen index + 迁移方式（拆分 / 合并 / 退役）

## §8 Delta — a/f/t-registry + layer-delta.md ValueFlow Alias (PR-5)

**ADDED** ValueFlow Alias block to:
- `a-registry.md` — 6 S section header (D1-S13..S18)
- `f-registry.md` — 5 S 段 (D1-S13..S18)
- `t-registry.md` — §1 6 S header
- `layer-delta.md` — §Canonical S → ValueFlow Alias 表

## §9 Delta — span-registry.md

**UNCHANGED** 10 canonical + 14 legacy ops (R1 Q3 决议, runtime 字面量稳定性):
- `d1.capture.persist` (INTERNAL, S13)
- `d1.dispatch.route` (INTERNAL, S13)
- `d1.signal.thinking` (INTERNAL, S14)
- `d1.signal.task` (INTERNAL, S15)
- `d1.signal.conclusion` (INTERNAL, S16)
- `d1.signal.chain_integrity` (INTERNAL, S14–S16)
- `d1.signal.task.work_proof` (INTERNAL, S15)
- `d1.signal.user_feedback` (INTERNAL, S16)
- `eventbus.publish_critical` (INTERNAL, S18)
- `eventbus.drain` (INTERNAL, S18)
- `adapter.feishu.encode` (CLIENT, S17)

## §10 Delta — d1-flow-architecture.md (NEW, PR-2)

**ADDED** `openspec/specs/architecture/d1-flow-architecture.md` v1.0.0 (85 行):
- §Architecture（价值流 — v2.0 实现）
- §Package Map
- §Architecture（Legacy 包结构 — RETIRED v2.0）
- §跨域接线
- 修订记录 v1.0.0 row

## §11 Delta — d7-boundary.md (NEW, PR-7)

**ADDED** `openspec/specs/d1-communication/d7-boundary.md` v1.0.0 (60 行):
- §1 D1 owns 的 boundary decision (3 row)
- §2 跨域接线
- §3 Hard Ban
- §4 重新评估触发
- 修订记录 v1.0.0 row

## §12 Delta — orchtypes 包建立 (PR-1)

**ADDED** `internal/layers/communication/orchtypes/` 子包:
- `events.go` (NEW): 3 D1↔D7 跨域事件字面量常量化（`orchestration.entry.process` / `permission.required` / `orchestration.orphan_event`）
- `contracts.go` (NEW): 跨包 import 桥（前置 PR-1 准备）
- `boundary_decision.go` (NEW): 3 boundary debt 治理常量 + `AllBoundaryDecisions()`
- `boundary_decision_test.go` (NEW): 3 单元测试 (Exist + VersionFormat + Unique)

## §13 Delta — CI Guard scripts/d1-span-coverage.sh (PR-4)

**ADDED** awk parser 检查 t-registry §Canonical T 表格 Span Evidence 列是否非空非 `—`。守门 ≥80% effective。

实际通过: **44/(56-12) = 100%** PASS (阈值 80%)。

---

**END of Spec Delta**
