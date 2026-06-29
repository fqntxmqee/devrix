# Proposal: devrix-d1-ac-restructuring (DM-20260629-005)

**Change ID:** `devrix-d1-ac-restructuring`
**Demand ID:** DM-20260629-005
**Status:** S7_Archived (2026-06-30)
**Title:** D1 通信域 AC 深度架构重构 — 6 子 Change 联动 (god-doc-split pt1+pt2 + registry-sync + value-flow-rename + gherkin-restructuring + boundary-decision)
**Template:** `devrix-d4-dsaft-restructuring` proposal.md (DM-20260629-004 S7_Archived 2026-06-29)

---

## §0 概述

D1 v4.1.1（2026-06-16 spec.md）已 stable "6 价值流 S13-S18 / 16 Canonical A / 18 F / 56 T（P0=26） / 10 canonical span ops" 的 SoT 状态。但 2026-06-29 全域 AC Review 暴露 **D1 是 7 域中唯一 AC 设计 outlier**（5.5/10），缺 5 类关键治理：

1. **Gherkin AC 形式不一致** — 24 缩写 bullet vs D2/D3/D4/D7 的 `#### Scenario:` 块
2. **Span Evidence 列缺失** — t-registry 56 T 行无 evidence 列
3. **d7-boundary.md 缺失** — D1 ↔ D7 跨域边界无 SoT
4. **spec.md / observability-guide.md god doc** — 176 + 230 行单文件堆叠
5. **ValueFlow Alias 缺失** — d1-domain.md §North Star 缺 6 alias

本 proposal 把 5 类治理映射到 6 子 Change + 1 S7_Archive 收口，对齐 D7/D2/D3/D4 6 子 Change 模板（D4 模板 8 PR 复用，DM-20260629-004）。

---

## §1 子 Change 提案汇总

| # | 子 Change | PR | T 范围 | 工作量 |
|---|----------|----|----|------|
| **#0** | orchtypes-bootstrap | PR-1 | T01-T03 | 1 PR / 1 天 |
| **#1** | god-doc-split pt1 | PR-2 | T04-T07 | 1 PR / 1 天 |
| **#1** | god-doc-split pt2 | PR-3 | T08-T11 | 1 PR / 1 天 |
| **#2** | registry-sync | PR-4 | T12-T16 | 1 PR / 1 天 |
| **#3** | value-flow-rename | PR-5 | T17-T20 | 1 PR / 1 天 |
| **#4** | gherkin-restructuring | PR-6 | T21-T26 | 1 PR / 1 天 |
| **#5** | boundary-decision | PR-7 | T27-T30 | 1 PR / 1 天 |
| **S7_Archive** | S7_Archive | PR-8 | T31-T34 | 1 PR / 1 天 |
| **总计** | **8 PR** | **~34 T** | **5-7 天** |

---

## §2 子 Change #0 orchtypes-bootstrap (PR-1, T01-T03)

**目标**：建立 `internal/layers/communication/orchtypes/` 治理包基础 + 3 boundary decision 治理常量。

### T01 建立 `orchtypes/` 包

- New dir `internal/layers/communication/orchtypes/`
- `orchtypes/events.go` (NEW) — 3 D1↔D7 跨域事件字面量常量化（`orchestration.entry.process` / `permission.required` / `orchestration.orphan_event`）
- `orchtypes/contracts.go` (NEW) — 跨包 import 桥（前置 PR-1 准备，PR-4/5 渐进迁入）

### T02 `boundary_decision.go` 前置 3 治理常量

```go
package orchtypes

const (
    BoundaryD1ToD7OrchestrationEntry = "boundary-debt:d1-to-d7-orchestration-entry-v1.0"
    BoundaryD1ToD4PermissionGate = "boundary-debt:d1-to-d4-permission-gate-v1.0"
    BoundaryD1ForbiddenOrchestrationImport = "boundary-debt:d1-forbidden-orchestration-import-v2.0"
)

func AllBoundaryDecisions() [3]string { ... }
```

### T03 `boundary_decision_test.go` (NEW) — 3 单测

- `TestBoundaryDecisions_Exist` — 3 名称在 `AllBoundaryDecisions()` 中
- `TestBoundaryDecisions_VersionFormat` — regex `^boundary-debt:[a-z0-9\-]+-v\d+\.\d+$`
- `TestBoundaryDecisions_Unique` — 3 名称互不重复

**验证**：
- `go test ./internal/layers/communication/orchtypes/... -race` PASS
- 0 跨包 import 新增（D1 内部）

**关键文件**：
- `internal/layers/communication/orchtypes/{events,contracts,boundary_decision,boundary_decision_test}.go` (NEW)

---

## §3 子 Change #1 god-doc-split pt1 (PR-2, T04-T07)

**目标**：`spec.md` 176 → 90 行（保留 Gherkin AC + 跨域依赖 + Key Patterns），拆出 `architecture/d1-flow-architecture.md` (NEW)。

### T04 拆 `spec.md` 176 LOC

- → `spec.md` (90 行) — 保留 Overview / Scenarios / Cross-Domain / Requirements (Gherkin) / Registries / Guides
- → `specs/architecture/d1-flow-architecture.md` (NEW, ~80 行) — 包含 Architecture（价值流流图）/ Package Map / Architecture（Legacy 包结构 - 保留追溯）

### T05 spec.md 头部加 See also

```markdown
## See also
- **Flow architecture**：`../architecture/d1-flow-architecture.md`（价值流流图 + Package Map + Legacy 包结构）
- **End-to-end flows**：`terminal-state-guide.md`
- **Observability & Runbook**：`observability-guide.md`
```

### T06 同步引用

- `terminal-state-guide.md` 引用 `d1-flow-architecture.md` 而非 `spec.md §Architecture`
- `observability-guide.md` 引用 `d1-flow-architecture.md` Package Map

### T07 验证

- spec.md ≤ 100 LOC ✓
- d1-flow-architecture.md 80-100 LOC ✓
- `grep -E "spec\.md §Architecture|spec\.md §Package Map" openspec/specs/d1-communication/ -r` → 0 命中

**关键文件**：
- `openspec/specs/d1-communication/spec.md` (176 → 90 行)
- `openspec/specs/architecture/d1-flow-architecture.md` (NEW)

---

## §4 子 Change #1 god-doc-split pt2 (PR-3, T08-T11)

**目标**：`observability-guide.md` 230 → 100 行（保留 Trace 树 + EventBus 必达 + Runbook），删 §1 Span↔T 矩阵（迁 t-registry Span Evidence 列），§5 T 验收摘要精简。

### T08 拆 observability-guide.md 230 LOC

- 删 §1 Span↔T 绑定矩阵 18 行（已迁 t-registry Span Evidence 列）
- 删 §7 已知缺口 5 行（迁 t-registry §T-Without-Span Tracker）
- 精简 §5 T 验收摘要（指向 t-registry §Statistics）

### T09 头部加 "See also"

```markdown
## See also
- **Span ↔ T 绑定**：`t-registry.md §T-Without-Span Tracker + Span Evidence 列`
- **T 验收摘要**：`t-registry.md §Statistics`
```

### T10 同步 t-registry.md

- 加 §T-Without-Span Tracker 章节（5 row：3 注入 + 1 启动 + 1 显式 `—`）
- 修订记录 v3.3.0 row

### T11 验证

- observability-guide.md ≤ 120 LOC ✓
- t-registry §T-Without-Span Tracker 5 row ✓
- `grep "Span↔T\|T 验收摘要" observability-guide.md` → 0 命中（已迁）

**关键文件**：
- `openspec/specs/d1-communication/observability-guide.md` (230 → 100 行)
- `openspec/specs/d1-communication/t-registry.md` (§T-Without-Span Tracker 新增)

---

## §5 子 Change #2 registry-sync (PR-4, T12-T16)

**目标**：t-registry 56 T 行加 Span Evidence 列 + Historical S 沉 archive + scripts/d1-span-coverage.sh CI 守门。

### T12 56 T 行加 Span Evidence 列

- 44 T 映射到 10 canonical span op / Span Event：`d1.capture.persist` / `d1.dispatch.route` / `d1.signal.{thinking,task,conclusion,chain_integrity,task.work_proof,user_feedback}` / `eventbus.{publish_critical,drain}` / `adapter.feishu.encode`
- 12 显式 `—`（注入模式 6 + 启动期 3 + 编译期 3 — 详见 t-registry §T-Without-Span Tracker）
- 列宽与 D3 t-registry v3.4.0 对齐

### T13 Historical S 沉 archive

- `openspec/specs/d1-communication/{a,f,t}-registry.md` Legacy S 章节 (D1-S1~S12) 整段下沉
- 新建 `openspec/archive/2026-06-14-devrix-d1-sa-refine/legacy-s1-s12.md` 含 frozen index + 迁移路径表（D1-S1→S13 / D1-S2→S17 / D1-S3→S13-A05 / D1-S5→S15 / D1-S6→S17 / D1-S8→S17 / D1-S9→S18 / D1-S10→S17 / D1-S11→kernel / D1-S12→S17）

### T14 d1-domain.md 物理路径表与 code 100% 对齐

- `d1-domain.md §DSAFT 资产` 表 4 行（layer / count / SoT）核对 a/f/t/span 文件

### T15 `scripts/d1-span-coverage.sh` (NEW, ~100 lines)

```bash
#!/usr/bin/env bash
# awk 解析 t-registry §Canonical T 表格，统计 Span Evidence 列
# 守门: effective 覆盖率 ≥ 80%
# 实际: 44/(56-12) = 100% effective (raw 44/56 ≈ 78.6% informational only)
```

### T16 验证

- t-registry 56 T 行全标注 ✓
- d1-span-coverage.sh ≥ 80% PASS ✓
- archive/2026-06-14-devrix-d1-sa-refine/legacy-s1-s12.md 存在 ✓

**关键文件**：
- `openspec/specs/d1-communication/t-registry.md` (56 T 行 + Span Evidence 列)
- `openspec/specs/d1-communication/{a,f,t}-registry.md` (Historical S 沉 archive)
- `openspec/archive/2026-06-14-devrix-d1-sa-refine/legacy-s1-s12.md` (NEW)
- `scripts/d1-span-coverage.sh` (NEW)

---

## §6 子 Change #3 value-flow-rename (PR-5, T17-T20)

**目标**：d1-domain.md §North Star 加 6 ValueFlow Alias + a/f/t/layer-delta 同步。

### T17 d1-domain.md §North Star 加 ValueFlow Alias 列

| 可验证承诺 | Canonical S | ValueFlow Alias (用户感知) |
|-----------|-------------|------------------------------|
| 指令不丢、可追、可续聊 | D1-S13 CaptureUserIntent | **D1_Capture_User_Intent** |
| 思考过程可见（信号① Costly） | D1-S14 PresentThinking | **D1_Present_Thinking** |
| 任务/工具/Worker 进度可见（信号②） | D1-S15 PresentTaskProgress | **D1_Present_Task_Progress** |
| 结论/错误必达用户（信号③ Costly） | D1-S16 DeliverConclusion | **D1_Deliver_Conclusion** |
| 多 IM 平台结构一致 | D1-S17 ConnectChannel | **D1_Connect_Channel** |
| 背压/弱网下 Critical 不丢 | D1-S18 GuaranteeDelivery | **D1_Guarantee_Delivery** |

### T18 a-registry.md 加 ValueFlow Alias block

6 S section header (D1-S13..S18) 各加 `> **ValueFlow Alias (用户感知):**` 行。

### T19 f-registry.md 加 ValueFlow Alias block

5 S 段 (D1-S13..S18) 各加 `> **ValueFlow Alias:**` 行。

### T20 t-registry.md + layer-delta.md §Canonical S 加 ValueFlow Alias 表

- t-registry.md §1 6 S header 加 `> **ValueFlow Alias:**` 行
- layer-delta.md §Canonical S → ValueFlow Alias 表

**关键文件**：
- `openspec/specs/d1-communication/d1-domain.md` (6 alias 块)
- `openspec/specs/d1-communication/{a,f,t}-registry.md` (ValueFlow Alias 同步)
- `openspec/specs/d1-communication/layer-delta.md` (§Canonical S → ValueFlow Alias 表)

---

## §7 子 Change #4 gherkin-restructuring (PR-6, T21-T26)

**目标**：spec.md 24 缩写 bullet → 90 个 `#### Scenario:` Gherkin 块（每 S 8-15 个），覆盖 happy / sad / boundary / concurrent / timeout。

### T21 D1-S13 CaptureUserIntent — 16 Scenario

### T22 D1-S14 PresentThinking — 12 Scenario

### T23 D1-S15 PresentTaskProgress — 15 Scenario

### T24 D1-S16 DeliverConclusion — 16 Scenario

### T25 D1-S17 ConnectChannel — 16 Scenario

### T26 D1-S18 GuaranteeDelivery — 15 Scenario

**合计 90 Scenario**，每 Scenario 末尾加 `<!-- T: D1-S{N}-A{NN}-T{XX} -->` 注释。

**Scenario 类别分布（90 块）**：

| 类别 | 数量 | 覆盖 |
|------|------|------|
| happy | 30 | 正常路径 |
| sad | 24 | 错误/拒绝/失败 |
| boundary | 18 | 边界值/状态转换 |
| concurrent | 9 | 并发/race |
| timeout | 9 | 超时/重试 |

**关键文件**：
- `openspec/specs/d1-communication/spec.md` (24 缩写 bullet → 90 `#### Scenario:` Gherkin 块)

---

## §8 子 Change #5 boundary-decision (PR-7, T27-T30)

**目标**：d1-domain.md §Boundary Debt Decisions + d7-boundary.md (NEW) + 3 单元测试。

### T27 跨域 boundary 审计

- D1 emit `permission.required` → D4 sessionagents/manager (resolvePermission) — RESOLVED
- D1 capture ↔ D7 orchestration entry (D1-S13-A03-F02 routeD7) — RESOLVED
- D1 forbidden orchestration/ import (lint-d1-imports.sh 守门) — RESOLVED
- D1 ↔ D5 observability (10 d1.* span ops) — RESOLVED
- D1 ↔ D2 硬禁直连 IEngine.Process (DM-007) — RESOLVED

### T28 d1-domain.md §Boundary Debt Decisions 章节

| Boundary ID | 状态 | 内容 | 重新评估 |
|-------------|------|------|----------|
| `boundary-debt:d1-to-d7-orchestration-entry-v1.0` | ✅ RESOLVED | D1-S13-A03-F02 routeD7 走 `IOrchestrationEntry.ProcessMessage` | — |
| `boundary-debt:d1-to-d4-permission-gate-v1.0` | ✅ RESOLVED | D1-S13-A04 ResolvePermissionGate 调 D4 sessionagents/manager | — |
| `boundary-debt:d1-forbidden-orchestration-import-v2.0` | ✅ RESOLVED | D1 capture 禁止 import orchestration/（lint-d1-imports.sh 守门） | — |

### T29 d7-boundary.md (NEW)

抄 D3 `d7-boundary.md v1.1.0` 模板；D1 侧只登记 D1 拥有的 boundary decision（3 row 同 T28 表）。

### T30 d1-domain.md v1.0.0 → v1.1.0 + 修订记录 v1.1.0 row

**关键文件**：
- `openspec/specs/d1-communication/d1-domain.md` (§Boundary Debt Decisions 章节 + 修订记录)
- `openspec/specs/d1-communication/d7-boundary.md` (NEW)
- `internal/layers/communication/orchtypes/boundary_decision_test.go` (3 单测已就位)

---

## §9 S7_Archive (PR-8, T31-T34)

### T31 6 artifacts 复制

- `.openspec.yaml` (NEW)
- `acceptance-report.md` (NEW)
- `demand.md` / `design.md` / `proposal.md` / `tasks.md` (从 changes/)
- `specs/d1-communication/spec.md` (从 changes/) + `specs/architecture/d1-flow-architecture.md` (NEW)

### T32 verify-archive.sh 12/12 PASS

### T33 d1-domain.md v1.0.0 → v1.1.0

### T34 spec.md v4.1.1 → v5.0.0

**关键文件**：
- `openspec/archive/2026-06-30-devrix-d1-ac-restructuring/` (NEW 7 artifacts)
- `openspec/changes/devrix-d1-ac-restructuring/` (删除)
- `openspec/demand-archive-index.md` (3 处更新：DM row + Archive Locations + 历史批次)
- `openspec/specs/d1-communication/{d1-domain.md, spec.md}` (修订记录 v1.1.0 / v5.0.0 row + §Change line)

---

## §10 关键设计决策

1. **God doc 拆分策略**：spec.md 176 → 90 行（保留 Gherkin AC）+ observability-guide.md 230 → 100 行（保留 Trace + Runbook）；两次拆分（PR-2 + PR-3）各拆 1 文件
2. **Historical S 沉 archive**：复用 `openspec/archive/2026-06-14-devrix-d1-sa-refine/` dir（已存在）
3. **orchtypes 包建立**：命名空间 `communication/orchtypes`（独立于 D4 `multiagent/orchtypes`）
4. **Span 覆盖率目标**：≥80% effective（44/(56-12) = 100% effective；raw 44/56 ≈ 78.6% informational only）
5. **ValueFlow Alias 命名**：`D1_` 前缀 + Suffix 用完整短语（`D1_Capture_User_Intent` 等），与 D4 `D4_Provision_Agent` 命名模式对齐
6. **Gherkin 90 Scenario 拆分**：每 S 8-15 个；happy 30 / sad 24 / boundary 18 / concurrent 9 / timeout 9
7. **d7-boundary.md 新建**：抄 D3 v1.1.0 模板；D1 侧只登记 D1 owns 的 3 boundary decision
8. **PR-2/3 god doc 拆与 PR-4 Span Evidence 同步**：PR-4 t-registry 加 Span Evidence 列时同步删 observability-guide.md §1 矩阵（避免双 SoT）

---

## §11 与 D7/D2/D3/D4 模板对比

| 维度 | D7 (DM-001) | D2 (DM-002) | D3 (DM-003) | D4 (DM-004) | **D1 (本 DM)** |
|------|------------|------------|------------|------------|---------------|
| LOC | ~15000+ | ~9000 | 4826 | 4108 | **~5500** |
| PR | 11 | 8 | 8 | 8 | **8** |
| T | 55 | 44 | 40 | 34 | **~34** |
| Span Coverage | 94% | 88% | 100% | 100% | **≥80% effective** |
| God fn / God doc | 4 god fn | 5 god fn | 4 god fn | 2 god fn | **2 god doc** (spec.md 176 + observability 230) |
| Boundary Debt | 3 RESOLVED | 2 (1+1 待定) | 4 RESOLVED | 3 RESOLVED | **3 RESOLVED** |
| **D1 独有债务** | RollupReport envelope + WorkTree 上行反馈 | DM-018 slice-c + 跨域 fixture | runtime span op 字面量稳定 | 18 F 路径漂移 + 7 EngineEvent 字面量 | **24 缩写 bullet AC + Span Evidence 列缺失 + d7-boundary.md 缺失 + 2 god doc** |

---

**END of Proposal**
