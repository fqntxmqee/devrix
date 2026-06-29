# Design: devrix-d1-ac-restructuring (DM-20260629-005)

**Change ID:** devrix-d1-ac-restructuring
**Demand ID:** DM-20260629-005
**Status:** S3_Design
**Template:** `devrix-d4-dsaft-restructuring` design.md (DM-20260629-004 S7_Archived 2026-06-29)

---

## §1 架构总览

### 1.1 6 子 Change 物理路径

```
devrix-d1-ac-restructuring (DM-20260629-005)
├── PR-1 #0 orchtypes-bootstrap     (T01-T03: orchtypes/ + 3 boundary 常量 + 3 单测)
├── PR-2 #1 god-doc-split pt1       (T04-T07: spec.md 176→90 + d1-flow-architecture.md NEW)
├── PR-3 #1 god-doc-split pt2       (T08-T11: observability-guide.md 230→100 + t-registry §T-Without-Span Tracker)
├── PR-4 #2 registry-sync           (T12-T16: 56 T Span Evidence + Historical S 沉 archive + d1-span-coverage.sh)
├── PR-5 #3 value-flow-rename       (T17-T20: 6 D1_* Alias + a/f/t/layer-delta 同步)
├── PR-6 #4 gherkin-restructuring   (T21-T26: 24 缩写 bullet → 90 #### Scenario: 块)
├── PR-7 #5 boundary-decision       (T27-T30: d1-domain §Boundary + d7-boundary.md NEW)
└── PR-8 S7_Archive                 (T31-T34: 6 artifacts + verify-archive 12/12 + d1-domain v1.1.0 + spec v5.0.0)
```

### 1.2 物理路径映射表

| PR | 旧路径 | 新路径 | T 范围 |
|----|--------|--------|--------|
| PR-1 | (无) | `internal/layers/communication/orchtypes/{events,contracts,boundary_decision,boundary_decision_test}.go` (NEW) | T01-T03 |
| PR-2 | `spec.md` (176 行) | `spec.md` (90 行) + `specs/architecture/d1-flow-architecture.md` (NEW) | T04-T07 |
| PR-3 | `observability-guide.md` (230 行) | `observability-guide.md` (100 行) + `t-registry.md §T-Without-Span Tracker` | T08-T11 |
| PR-4 | `t-registry.md` (无 Span Evidence) | `t-registry.md` (56 T 行 + Span Evidence 列) + `archive/2026-06-14-devrix-d1-sa-refine/legacy-s1-s12.md` (NEW) + `scripts/d1-span-coverage.sh` (NEW) | T12-T16 |
| PR-5 | `d1-domain.md` (无 ValueFlow Alias) | `d1-domain.md` (6 alias) + `{a,f,t}-registry.md` (同步) + `layer-delta.md` | T17-T20 |
| PR-6 | `spec.md` (24 缩写 bullet) | `spec.md` (90 `#### Scenario:` 块) | T21-T26 |
| PR-7 | `d1-domain.md` (无 §Boundary Debt) | `d1-domain.md §Boundary Debt Decisions` + `d7-boundary.md` (NEW) | T27-T30 |
| PR-8 | `openspec/changes/devrix-d1-ac-restructuring/` | `openspec/archive/2026-06-30-devrix-d1-ac-restructuring/` (7 artifacts) + `demand-archive-index.md` 3 处更新 | T31-T34 |

---

## §2 子 Change #0 orchtypes-bootstrap (PR-1) — T01-T03

### 2.1 `internal/layers/communication/orchtypes/events.go` (NEW, ~30 lines)

```go
package orchtypes

// D1↔D7 跨域事件字面量常量化（DM-20260629-005 PR-1 #0）
const (
    EventOrchestrationEntryProcess = "orchestration.entry.process"
    EventPermissionRequired = "permission.required"
    EventOrchestrationOrphanEvent = "orchestration.orphan_event"
)

// AllEvents 返回所有跨域事件常量（test helper）
func AllEvents() [3]string { ... }
```

### 2.2 `boundary_decision.go` (NEW, ~25 lines)

```go
package orchtypes

// D1 跨域 boundary debt 治理常量（DM-20260629-005 PR-1 #0）
const (
    BoundaryD1ToD7OrchestrationEntry = "boundary-debt:d1-to-d7-orchestration-entry-v1.0"
    BoundaryD1ToD4PermissionGate = "boundary-debt:d1-to-d4-permission-gate-v1.0"
    BoundaryD1ForbiddenOrchestrationImport = "boundary-debt:d1-forbidden-orchestration-import-v2.0"
)

// AllBoundaryDecisions 返回所有 boundary 治理常量
func AllBoundaryDecisions() [3]string { ... }
```

### 2.3 `boundary_decision_test.go` (NEW, ~50 lines)

- `TestBoundaryDecisions_Exist` — 3 名称在 `AllBoundaryDecisions()` 中
- `TestBoundaryDecisions_VersionFormat` — regex `^boundary-debt:[a-z0-9\-]+-v\d+\.\d+$`
- `TestBoundaryDecisions_Unique` — 3 名称互不重复

---

## §3 子 Change #1 god-doc-split pt1 (PR-2) — T04-T07

### 3.1 spec.md 拆分后结构（90 行）

```markdown
# D1 Communication Domain Specification

**Capability:** communication
**Status:** Active
**Version:** 5.0.0
**Last Updated:** 2026-06-30
**Domain SoT:** `d1-domain.md`

---

## See also
- **Flow architecture**：`../architecture/d1-flow-architecture.md`（价值流流图 + Package Map + Legacy 包结构）
- **End-to-end flows**：`terminal-state-guide.md`
- **Observability & Runbook**：`observability-guide.md`

## Overview (10 行)
- 通信域职责 + 信号分层博弈论 + Canonical SoT

## Scenarios — Canonical（价值流）(8 行)
- 6 S 表 (D1-S13..S18) + Status

## Cross-Domain Dependencies (15 行)
- D2 / D4 / D5 / D7 依赖表

## Key Design Patterns (10 行)
- 5 个核心 pattern (Gateway-Adapter / EventBus / PermissionManager / Card / Session / CardKit)

## Requirements — Canonical Gherkin（S13–S18）(45 行)
- 90 个 #### Scenario: 块 (PR-6 完成)

## Registries (4 行)
- A/F/T/Span SoT 索引
```

### 3.2 `architecture/d1-flow-architecture.md` (NEW, ~85 行)

```markdown
# D1 Communication Flow Architecture

**Capability:** d1-flow-architecture
**Status:** Active
**Version:** 1.0.0
**Last Updated:** 2026-06-30

---

## Architecture（价值流 — v2.0 实现）
- 价值流流图 [User IM] → S17 → S13 → Dispatch (D7|Agent) → Agent events → S18 → present/ (S14|S15|S16) → S17 Encode → [User IM]

## Package Map
- 6 scenario-slug → 当前路径 → Canonical S 表

## Architecture（Legacy 包结构 — RETIRED v2.0）
- User/IM → Adapters → Gateway → Context Engine
- Renderers ← EventBus ←┘

## 跨域接线
- D1 capture → D7 orchestration entry (IOrchestrationEntry.ProcessMessage)
- D1 capture → D4 sessionagents (leader lifecycle)
- D1 channel/adapters → 零 D7 import (lint-d1-imports.sh 守门)

## 修订记录
| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-06-30 | 初版：从 spec.md 拆出（DM-20260629-005 PR-2） |
```

---

## §4 子 Change #1 god-doc-split pt2 (PR-3) — T08-T11

### 4.1 observability-guide.md 拆分后结构（100 行）

```markdown
# D1 Communication — 可观测性与必达指南

**Capability:** d1-communication
**Status:** Active
**Version:** 2.0.0
**Last Updated:** 2026-06-30
**Parent:** `d1-domain.md` · `span-registry.md` · `t-registry.md`

---

## 0. 文档定位
## See also
- **Span ↔ T 绑定**：`t-registry.md §T-Without-Span Tracker + Span Evidence 列`
- **T 验收摘要**：`t-registry.md §Statistics`
- **Span operation 全表**：`span-registry.md`

## 1. IntentFast Trace 树 (25 行)
- 树形结构图（删除 Span↔T 矩阵 18 行后）
- 延迟 SLO 表

## 2. EventBus 双通道与必达 (30 行)
- 路由规则 + PublishCritical 语义 + 状态机

## 3. 弱网 Drain 故障注入场景 (20 行)
- 时序图 + 测试 ↔ T 引用表

## 4. 生产 Trace 检查清单 (15 行)
- 6 个检查项 + 4 类告警

## 5. 关联文档
| 文档 | 关系 |
|------|------|

## 修订记录
| Version | Date | Changes |
|---------|------|---------|
| 2.0.0 | 2026-06-30 | god-doc-split: 230 → 100 行（DM-20260629-005 PR-3）|
```

### 4.2 t-registry.md §T-Without-Span Tracker (NEW 章节, ~30 行)

```markdown
## T-Without-Span Tracker

> DM-20260629-005 PR-3 god-doc-split pt2: 从 observability-guide.md §7 已知缺口迁入

| T ID | 原因 | 说明 |
|------|------|------|
| D1-S19-A01-T01..T03 | 注入模式 | Transcript Writer 注入到 `d1.capture.persist` span 内的 attribute |
| D1-S19-A01-T04 | 并发注入 | Transcript 100 goroutine 追加与 capture.persist 并发 |
| D1-S5-A07-T05..T08 | 启动期 | feishu precheck 在 adapter 启动期生效，不进入 trace |
| D1-S3-A08-T01 | 编译期 | error_code 文案映射在 emit 时查表，不单独 span |
```

---

## §5 子 Change #2 registry-sync (PR-4) — T12-T16

### 5.1 t-registry.md Span Evidence 列映射表

| S | T 范围 | Span Evidence |
|---|--------|---------------|
| S13 | T01-T04 | `d1.capture.persist` (T01) / `d1.dispatch.route` (T02, T03) / `—` (T04 permission) |
| S14 | T01 | `d1.signal.thinking` |
| S15 | T01-T02 | `d1.signal.task` (T01) / `d1.signal.task.work_proof` (T02) |
| S16 | T01-T02 | `d1.signal.conclusion` (T01, T02) |
| S17 | T01 | `adapter.feishu.encode` |
| S18 | T01-T02 | `eventbus.publish_critical` (T01) / `eventbus.drain` (T02) |
| S19 | T01-T04 | `—` (Transcript 注入) |
| S5-A07 | T05-T08 | `—` (feishu precheck 启动期) |
| S3-A08 | T01 | `—` (error_code 编译期) |
| RF | T01-T09 | `d1.capture.persist` (T01) / `—` (T02-T09) |

**44 T 直接映射 + 12 显式 `—` = 56 total**
**有效覆盖率 = 44 / (56 - 12) = 100%**
**raw 44/56 ≈ 78.6% informational only**

### 5.2 Historical S 沉 archive — `legacy-s1-s12.md` (NEW, ~60 行)

```markdown
# D1 Communication — Historical S1-S12 (ARCHIVED 2026-06-14)

**Status:** Frozen Index — DM-20260614-006 Phase 3
**Migration:** 全部 S 已迁 Canonical S13-S18 (DSAFT v2.0)

## 迁移路径表

| Legacy S | Canonical S | 迁移方式 |
|----------|-------------|---------|
| D1-S1 Gateway | D1-S13 CaptureUserIntent | A 拆分到 S13-A01/A02/A03；A04 迁 D4 |
| D1-S2 Adapters | D1-S17 ConnectChannel | A 拆分到 S17-A01..A06 |
| D1-S3 Commands | D1-S13-A05 ParseCommand | A 归到 S13 |
| D1-S4 Auth | (PLANNED, 未实现) | 留 D1-S4 占位 |
| D1-S5 Milestone | D1-S15-A01-F03 EmitMilestoneCardProgress | A 归到 S15 |
| D1-S6 RateLimit | D1-S17-A06 CheckRateLimit | A 归到 S17 |
| D1-S7 Metrics | [DEPRECATED] D5 observability.Bridge | 完全迁移 |
| D1-S8 Renderers | D1-S17 Encode F | F 归到 S17 |
| D1-S9 EventBus | D1-S18 GuaranteeDelivery | A 归到 S18 |
| D1-S10 Connection | D1-S17-A04 ManageConnection | A 归到 S17 |
| D1-S11 Core | D1 kernel (Card 模型) | 不进 S |
| D1-S12 Instance | D1-S17-A05 RegisterInstance | A 归到 S17 |
```

### 5.3 `scripts/d1-span-coverage.sh` (NEW, ~100 lines)

```bash
#!/usr/bin/env bash
# scripts/d1-span-coverage.sh — D1 t-registry Span Evidence 覆盖率守门
# 守门: effective 覆盖率 ≥ 80% (实际 100% effective)
set -euo pipefail

T_REG="openspec/specs/d1-communication/t-registry.md"
MIN_EFFECTIVE=80

# awk 解析 Canonical T 表格，统计 Span Evidence 列
total=$(awk -F'|' '/^\| D1-S.*-T[0-9]+/ && $0 !~ /Legacy/ {count++} END {print count}' "$T_REG")
mapped=$(awk -F'|' '/^\| D1-S.*-T[0-9]+/ && $0 !~ /Legacy/ && $0 !~ /—|（/ {count++} END {print count}' "$T_REG")
explicit_dash=$(awk -F'|' '/^\| D1-S.*-T[0-9]+/ && $0 !~ /Legacy/ && /—|（/ {count++} END {print count}' "$T_REG")

effective=$(awk "BEGIN {printf \"%.1f\", ($mapped / ($total - $explicit_dash)) * 100}")
raw=$(awk "BEGIN {printf \"%.1f\", ($mapped / $total) * 100}")

echo "D1 t-registry Span Evidence 覆盖率"
echo "  Total: $total"
echo "  Mapped: $mapped"
echo "  Explicit —: $explicit_dash"
echo "  Effective: ${effective}% (≥ ${MIN_EFFECTIVE}% PASS)"
echo "  Raw: ${raw}% (informational only)"

if awk "BEGIN {exit !($effective >= $MIN_EFFECTIVE)}"; then
  echo "✅ PASS"
  exit 0
else
  echo "❌ FAIL: effective 覆盖率 < ${MIN_EFFECTIVE}%"
  exit 1
fi
```

---

## §6 子 Change #3 value-flow-rename (PR-5) — T17-T20

### 6.1 d1-domain.md §North Star 新增列

```markdown
| 可验证承诺 | Canonical S | ValueFlow Alias (用户感知) |
|-----------|-------------|------------------------------|
| 指令不丢、可追、可续聊 | D1-S13 CaptureUserIntent | **D1_Capture_User_Intent** |
| 思考过程可见（信号① Costly） | D1-S14 PresentThinking | **D1_Present_Thinking** |
| 任务/工具/Worker 进度可见（信号②） | D1-S15 PresentTaskProgress | **D1_Present_Task_Progress** |
| 结论/错误必达用户（信号③ Costly） | D1-S16 DeliverConclusion | **D1_Deliver_Conclusion** |
| 多 IM 平台结构一致 | D1-S17 ConnectChannel | **D1_Connect_Channel** |
| 背压/弱网下 Critical 不丢 | D1-S18 GuaranteeDelivery | **D1_Guarantee_Delivery** |
```

### 6.2 a/f/t-registry ValueFlow Alias 同步

- a-registry.md 6 S section header (D1-S13..S18) 各加 `> **ValueFlow Alias (用户感知):**` 行
- f-registry.md 5 S 段 (D1-S13..S18) 各加 `> **ValueFlow Alias:**` 行
- t-registry.md §1 6 S header 加 `> **ValueFlow Alias:**` 行

### 6.3 layer-delta.md §Canonical S → ValueFlow Alias 表

```markdown
## §Canonical S → ValueFlow Alias

| Canonical S | ValueFlow Alias | 用户感知 |
|-------------|----------------|---------|
| D1-S13 CaptureUserIntent | D1_Capture_User_Intent | 指令不丢、可追 |
| D1-S14 PresentThinking | D1_Present_Thinking | 思考过程可见 |
| D1-S15 PresentTaskProgress | D1_Present_Task_Progress | 任务/工具进度 |
| D1-S16 DeliverConclusion | D1_Deliver_Conclusion | 结论/错误必达 |
| D1-S17 ConnectChannel | D1_Connect_Channel | 多 IM 一致 |
| D1-S18 GuaranteeDelivery | D1_Guarantee_Delivery | Critical 不丢 |
```

---

## §7 子 Change #4 gherkin-restructuring (PR-6) — T21-T26

### 7.1 90 Scenario 分布

| S | T21-T26 范围 | Scenario 数量 | 类别分布 |
|---|-------------|------------|----------|
| S13 CaptureUserIntent | T21 | 16 | happy 5 / sad 4 / boundary 3 / concurrent 2 / timeout 2 |
| S14 PresentThinking | T22 | 12 | happy 4 / sad 3 / boundary 2 / concurrent 1 / timeout 2 |
| S15 PresentTaskProgress | T23 | 15 | happy 5 / sad 4 / boundary 3 / concurrent 2 / timeout 1 |
| S16 DeliverConclusion | T24 | 16 | happy 6 / sad 4 / boundary 3 / concurrent 1 / timeout 2 |
| S17 ConnectChannel | T25 | 16 | happy 5 / sad 5 / boundary 3 / concurrent 2 / timeout 1 |
| S18 GuaranteeDelivery | T26 | 15 | happy 5 / sad 4 / boundary 4 / concurrent 1 / timeout 1 |
| **总计** | T21-T26 | **90** | **happy 30 / sad 24 / boundary 18 / concurrent 9 / timeout 9** |

### 7.2 Scenario 模板（happy 示例）

```markdown
### Feature: Inbound user message persistence

#### Scenario: Inbound non-empty message persists and updates session

- **Given** a Feishu non-empty user message arrives
- **When** D1 Accept + Persist the message
- **Then** session.last_activity is updated
- **And** user turn is traceable via `d1.capture.persist` span
<!-- T: D1-S13-A02-T01 -->
```

### 7.3 Scenario 模板（sad 示例）

```markdown
#### Scenario: Empty content rejected without dispatch

- **Given** an inbound message with empty content
- **When** D1 Accept validates the message
- **Then** Accept returns validation error
- **And** no Dispatch to D7 occurs
<!-- T: D1-S13-A01-T01 -->
```

### 7.4 Scenario 模板（boundary / concurrent / timeout 示例）

```markdown
#### Scenario: 100 concurrent inbound messages all persist (concurrent)

- **Given** 100 inbound Feishu messages arrive in burst
- **When** D1 Accept + Persist handles all 100 concurrently
- **Then** all 100 turns are persisted with unique session.last_activity
- **And** no race condition causes lost turn
<!-- T: D1-S13-A02-T02 -->

#### Scenario: Accept times out when Persist hangs (timeout)

- **Given** Persist SessionStore call blocks for >5s
- **When** D1 Accept is called
- **Then** Accept returns ErrPersistTimeout after 5s deadline
- **And** no partial turn state remains in session
<!-- T: D1-S13-A01-T03 -->
```

---

## §8 子 Change #5 boundary-decision (PR-7) — T27-T30

### 8.1 d1-domain.md §Boundary Debt Decisions 章节 (NEW)

```markdown
## Boundary Debt Decisions

> DM-20260629-005 PR-7 #5 boundary-decision

| Boundary ID | 状态 | 内容 | 重新评估 |
|-------------|------|------|----------|
| `boundary-debt:d1-to-d7-orchestration-entry-v1.0` | ✅ RESOLVED | D1-S13-A03-F02 routeD7 走 `IOrchestrationEntry.ProcessMessage`（零 D2 直连）| — |
| `boundary-debt:d1-to-d4-permission-gate-v1.0` | ✅ RESOLVED | D1-S13-A04 ResolvePermissionGate 调 D4 sessionagents/manager.ResolveAgentPermission | — |
| `boundary-debt:d1-forbidden-orchestration-import-v2.0` | ✅ RESOLVED | D1 capture 禁止 import orchestration/（lint-d1-imports.sh 守门） | — |

> 治理常量在 `internal/layers/communication/orchtypes/boundary_decision.go`。
> 3 单元测试: `internal/layers/communication/orchtypes/boundary_decision_test.go` (Exist + VersionFormat + Unique)。
```

### 8.2 `d7-boundary.md` (NEW, ~60 行)

```markdown
# D1 ↔ D7 跨域边界契约

**Capability:** d1-communication
**Status:** Active
**Version:** 1.0.0
**Last Updated:** 2026-06-30
**Domain SoT:** `d1-domain.md` · `d7-domain.md`

---

## §1 D1 owns 的 boundary decision (3 row)

| Boundary ID | 状态 | 内容 |
|-------------|------|------|
| `boundary-debt:d1-to-d7-orchestration-entry-v1.0` | ✅ RESOLVED | D1-S13-A03-F02 routeD7 走 `IOrchestrationEntry.ProcessMessage` |
| `boundary-debt:d1-to-d4-permission-gate-v1.0` | ✅ RESOLVED | D1-S13-A04 ResolvePermissionGate 调 D4 sessionagents/manager |
| `boundary-debt:d1-forbidden-orchestration-import-v2.0` | ✅ RESOLVED | D1 capture 禁止 import orchestration/（lint-d1-imports.sh 守门） |

## §2 跨域接线

| From | To | 接线 | 守卫 |
|------|----|------|------|
| D1-S13-A03-F02 routeD7 | D7 IOrchestrationEntry.ProcessMessage | composition root (bootstrap) | D1 capture import boundary |
| D1-S13-A04 ResolvePermissionGate | D4 sessionagents/manager | bootstrap wire | D1 capture lint |
| D1-S14/S15/S16 EmitOutboundSignal | D7 orchestrator (signal consumer) | EventBus channel | 双向命名空间 |

## §3 Hard Ban

- D1 capture 禁止 import `internal/layers/orchestration/`
- D1 capture 禁止 import `internal/layers/multiagent/`
- D1 channel/adapters 禁止 import `internal/layers/orchestration/`
- 守门脚本: `scripts/lint-d1-imports.sh`

## §4 重新评估触发

- 当 D7 v6+ 引入新 IOrchestrationEntry 入口点时
- 当 D4 sessionagents/manager 拆分为更细粒度权限入口时
- 当 D1 capture 拆 ingress/outbound 子包时

## 修订记录

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-06-30 | 初版：3 boundary decision 登记（DM-20260629-005 PR-7）|
```

---

## §9 S7_Archive (PR-8) — T31-T34

### 9.1 7 artifacts 复制

```
openspec/archive/2026-06-30-devrix-d1-ac-restructuring/
├── .openspec.yaml (NEW)
├── acceptance-report.md (NEW)
├── demand.md (from changes/)
├── design.md (from changes/)
├── proposal.md (from changes/)
├── tasks.md (from changes/)
└── specs/
    ├── d1-communication/
    │   └── spec.md (PR-6 gherkin + PR-2 spec.md 90 行版本)
    └── architecture/
        └── d1-flow-architecture.md (NEW, PR-2)
```

### 9.2 verify-archive.sh 12/12 PASS

### 9.3 d1-domain.md v1.0.0 → v1.1.0 + 修订记录 v1.1.0 row

### 9.4 spec.md v4.1.1 → v5.0.0 + 修订记录 v5.0.0 row

### 9.5 demand-archive-index.md 3 处更新

- Active Changes DM row (DM-20260629-005 → archived)
- Archive Locations table (2026-06-30-devrix-d1-ac-restructuring)
- Historical Batch list

---

## §10 Spec 同步目标（v1.0.0 → v1.1.0, v4.1.1 → v5.0.0）

- `d1-domain.md`: Version 1.0.0 → **1.1.0** + Last Updated 2026-06-30 + v1.1.0 row + §ValueFlow Alias 列 + §Boundary Debt Decisions 章节
- `spec.md`: Version 4.1.1 → **5.0.0** + 176 → 90 行 (PR-2) + 24 缩写 bullet → 90 `#### Scenario:` 块 (PR-6)
- `a-registry.md`: v3.1.0 → v3.2.0 + 同步 PR-4 (Historical S 沉 archive) + PR-5 (ValueFlow Alias)
- `f-registry.md`: v3.0.0 → v3.1.0 + 同步 PR-4 (Historical S 沉 archive) + PR-5 (ValueFlow Alias)
- `t-registry.md`: v3.2.0 → v3.3.0 + 同步 PR-3 (§T-Without-Span Tracker) + PR-4 (Span Evidence 列) + PR-5 (ValueFlow Alias)
- `span-registry.md`: v3.1.0 → v3.2.0 (UNCHANGED, 10 canonical + 14 legacy ops)
- `observability-guide.md`: v1.0.0 → v2.0.0 + 230 → 100 行 (PR-3) + 删 §1 Span↔T 矩阵
- `d1-flow-architecture.md`: NEW v1.0.0 (PR-2)
- `d7-boundary.md`: NEW v1.0.0 (PR-7)
- `layer-delta.md`: v1.x → v1.1.0 + §Canonical S → ValueFlow Alias 表 (PR-5)
- `legacy-s1-s12.md`: NEW (PR-4)
- `d1-span-coverage.sh`: NEW ~100 lines (PR-4)

---

## §11 关键文件路径清单

| 类别 | 路径 |
|------|------|
| **D1 域核心代码** | `internal/layers/communication/{capture,channel,delivery,kernel,transcript}/` |
| **D1 域 spec** | `openspec/specs/d1-communication/{d1-domain,spec,a-registry,f-registry,t-registry,span-registry,observability-guide,layer-delta}.md` |
| **D1 域 spec (NEW)** | `openspec/specs/d1-communication/d7-boundary.md` + `openspec/specs/architecture/d1-flow-architecture.md` |
| **跨域 boundary** | `internal/layers/communication/orchtypes/boundary_decision.go` (NEW) + `events.go` (NEW) |
| **Telemetry** | `internal/layers/observability/instrument/telemetry/names.go` (D1_* 常量已存在，0 改动) |
| **Archive 目录（已存在可复用）** | `openspec/archive/2026-06-14-devrix-d1-sa-refine/` (legacy-s1-s12.md NEW) |
| **新 Archive 目录** | `openspec/archive/2026-06-30-devrix-d1-ac-restructuring/` |
| **需求索引** | `openspec/demand-archive-index.md` (3 处更新) |
| **CI 守门** | `scripts/verify-archive.sh` (复用) + `scripts/d1-span-coverage.sh` (NEW) |

---

## §12 风险与缓解

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Gherkin 90 Scenario 改造破坏 t-registry T 引用 | Mid | Mid | 每 Scenario 末尾加 `<!-- T: D1-S{N}-A{NN}-T{XX} -->` 注释；`grep` 守门 |
| Span Evidence 列新增破坏 t-registry 列宽 | Low | Low | 表格化，列宽对齐；按 D3 t-registry v3.4.0 模板复刻 |
| d7-boundary.md 新建涉及 D7 跨域同步 | Mid | Mid | 抄 D3 `d7-boundary.md v1.1.0` 模板；D1 侧只登记 D1 owns |
| orchtypes/boundary_decision.go 重复 D4 orchtypes | Low | Low | 命名空间独立 `communication/orchtypes` |
| Historical S 沉 archive 丢失追溯 | Low | Mid | 复用 `openspec/archive/2026-06-14-devrix-d1-sa-refine/` dir |
| spec.md 拆 2 文件破坏旧链接 | Mid | Mid | spec.md 头部加 "See also" 引用 d1-flow-architecture.md；grep 0 死链 |
| observability-guide.md 拆破坏 §1 Span↔T 引用 | Low | Mid | 同步更新 t-registry.md §T-Without-Span Tracker；删 §1 时同步 |
| 8 子 Change 跨 5-7 天 baseline 不稳 | Mid | Mid | 每日 1 PR squash auto-merge；CI gate 即时验证 |

---

**END of Design**
