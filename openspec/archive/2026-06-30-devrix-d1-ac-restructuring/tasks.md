# Tasks: devrix-d1-ac-restructuring (DM-20260629-005)

**Change ID:** devrix-d1-ac-restructuring
**Demand ID:** DM-20260629-005
**Status:** S2_Proposal
**Total PR:** 8 / Total T: 34
**Template:** `devrix-d4-dsaft-restructuring` tasks.md (DM-20260629-004 S7_Archived 2026-06-29)

---

## §1 T 总览

| PR | 子 Change | T 范围 | 工作量 | 验收 |
|----|----------|--------|--------|------|
| PR-1 | #0 orchtypes-bootstrap | T01-T03 | 1 PR / 1 天 | `go test ./internal/layers/communication/orchtypes/... -race` PASS |
| PR-2 | #1 god-doc-split pt1 | T04-T07 | 1 PR / 1 天 | spec.md ≤ 100 LOC + d1-flow-architecture.md 80-100 LOC |
| PR-3 | #1 god-doc-split pt2 | T08-T11 | 1 PR / 1 天 | observability-guide.md ≤ 120 LOC + t-registry §T-Without-Span Tracker 5 row |
| PR-4 | #2 registry-sync | T12-T16 | 1 PR / 1 天 | t-registry 56 T + d1-span-coverage.sh ≥80% PASS + legacy-s1-s12.md 存在 |
| PR-5 | #3 value-flow-rename | T17-T20 | 1 PR / 1 天 | d1-domain 6 alias + a/f/t/layer-delta 同步 |
| PR-6 | #4 gherkin-restructuring | T21-T26 | 1 PR / 1 天 | spec.md 90 `#### Scenario:` 块 |
| PR-7 | #5 boundary-decision | T27-T30 | 1 PR / 1 天 | d1-domain §Boundary + d7-boundary.md NEW + 3 单测 PASS |
| PR-8 | S7_Archive | T31-T34 | 1 PR / 1 天 | 6 artifacts + verify-archive 12/12 + d1-domain v1.1.0 + spec v5.0.0 |

---

## §2 PR-1 #0 orchtypes-bootstrap (T01-T03)

**目标**：建立 `internal/layers/communication/orchtypes/` 治理包基础 + 3 boundary decision 治理常量。

### T01 `orchtypes/events.go` (NEW, ~30 lines)

- 3 跨域事件字面量常量化：`EventOrchestrationEntryProcess` / `EventPermissionRequired` / `EventOrchestrationOrphanEvent`
- `AllEvents()` 函数返回 3 字符串
- 与 D4 `multiagent/orchtypes/events.go` (7 EngineEvent) 命名一致

### T02 `orchtypes/boundary_decision.go` (NEW, ~25 lines)

```go
package orchtypes

const (
    BoundaryD1ToD7OrchestrationEntry = "boundary-debt:d1-to-d7-orchestration-entry-v1.0"
    BoundaryD1ToD4PermissionGate = "boundary-debt:d1-to-d4-permission-gate-v1.0"
    BoundaryD1ForbiddenOrchestrationImport = "boundary-debt:d1-forbidden-orchestration-import-v2.0"
)

func AllBoundaryDecisions() [3]string { ... }
```

### T03 `orchtypes/boundary_decision_test.go` (NEW, ~50 lines)

- `TestBoundaryDecisions_Exist` — 3 名称在 `AllBoundaryDecisions()` 中
- `TestBoundaryDecisions_VersionFormat` — regex `^boundary-debt:[a-z0-9\-]+-v\d+\.\d+$`
- `TestBoundaryDecisions_Unique` — 3 名称互不重复

**验证**：
- `go test ./internal/layers/communication/orchtypes/... -race -count=1` 3/3 PASS
- 0 跨包 import 新增（D1 内部）
- `wc -l internal/layers/communication/orchtypes/*.go` ≤ 200 LOC

**关键文件**：
- `internal/layers/communication/orchtypes/{events,contracts,boundary_decision,boundary_decision_test}.go` (NEW)

---

## §3 PR-2 #1 god-doc-split pt1 (T04-T07)

**目标**：`spec.md` 176 → 90 行（保留 Gherkin AC + 跨域依赖 + Key Patterns），拆出 `architecture/d1-flow-architecture.md` (NEW)。

### T04 拆 `spec.md` 176 LOC

- → `spec.md` (90 行) — 保留 Overview / Scenarios / Cross-Domain / Requirements / Registries / Guides
- → `architecture/d1-flow-architecture.md` (NEW, ~80 行) — 包含 Architecture（价值流流图）/ Package Map / Architecture（Legacy 包结构 - 保留追溯）

### T05 spec.md 头部加 "See also"

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

## §4 PR-3 #1 god-doc-split pt2 (T08-T11)

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

## §5 PR-4 #2 registry-sync (T12-T16)

**目标**：t-registry 56 T 行加 Span Evidence 列 + Historical S 沉 archive + scripts/d1-span-coverage.sh CI 守门。

### T12 56 T 行加 Span Evidence 列

- 44 T 映射到 10 canonical span op / Span Event
- 12 显式 `—`（注入模式 6 + 启动期 3 + 编译期 3 — 详见 t-registry §T-Without-Span Tracker）
- 列宽与 D3 t-registry v3.4.0 对齐

### T13 Historical S 沉 archive

- `openspec/specs/d1-communication/{a,f,t}-registry.md` Legacy S 章节 (D1-S1~S12) 整段下沉
- 新建 `openspec/archive/2026-06-14-devrix-d1-sa-refine/legacy-s1-s12.md` 含 frozen index + 迁移路径表

### T14 d1-domain.md 物理路径表与 code 100% 对齐

- `d1-domain.md §DSAFT 资产` 表 4 行（layer / count / SoT）核对 a/f/t/span 文件

### T15 `scripts/d1-span-coverage.sh` (NEW, ~100 lines)

- awk 解析 t-registry §Canonical T 表格
- 守门: effective 覆盖率 ≥ 80% (实际 100% effective)

### T16 验证

- t-registry 56 T 行全标注 ✓
- `./scripts/d1-span-coverage.sh` PASS ✓
- `archive/2026-06-14-devrix-d1-sa-refine/legacy-s1-s12.md` 存在 ✓

**关键文件**：
- `openspec/specs/d1-communication/t-registry.md` (56 T 行 + Span Evidence 列)
- `openspec/specs/d1-communication/{a,f,t}-registry.md` (Historical S 沉 archive)
- `openspec/archive/2026-06-14-devrix-d1-sa-refine/legacy-s1-s12.md` (NEW)
- `scripts/d1-span-coverage.sh` (NEW)

---

## §6 PR-5 #3 value-flow-rename (T17-T20)

**目标**：d1-domain.md §North Star 加 6 ValueFlow Alias + a/f/t/layer-delta 同步。

### T17 d1-domain.md §North Star 加 ValueFlow Alias 列

- 6 S + 6 alias:
  - S13 CaptureUserIntent → `D1_Capture_User_Intent`
  - S14 PresentThinking → `D1_Present_Thinking`
  - S15 PresentTaskProgress → `D1_Present_Task_Progress`
  - S16 DeliverConclusion → `D1_Deliver_Conclusion`
  - S17 ConnectChannel → `D1_Connect_Channel`
  - S18 GuaranteeDelivery → `D1_Guarantee_Delivery`

### T18 a-registry.md 加 ValueFlow Alias block

- 6 S section header (D1-S13..S18) 各加 `> **ValueFlow Alias (用户感知):**` 行

### T19 f-registry.md 加 ValueFlow Alias block

- 5 S 段 (D1-S13..S18) 各加 `> **ValueFlow Alias:**` 行

### T20 t-registry.md + layer-delta.md §Canonical S 加 ValueFlow Alias 表

- t-registry.md §1 6 S header 加 `> **ValueFlow Alias:**` 行
- layer-delta.md §Canonical S → ValueFlow Alias 表

**验证**：
- `grep "D1_*" d1-domain.md` ≥ 6 ✓
- `grep "ValueFlow Alias" a-registry.md` ≥ 6 ✓
- `grep "ValueFlow Alias" f-registry.md` ≥ 6 ✓

**关键文件**：
- `openspec/specs/d1-communication/d1-domain.md` (6 alias 块)
- `openspec/specs/d1-communication/{a,f,t}-registry.md` (ValueFlow Alias 同步)
- `openspec/specs/d1-communication/layer-delta.md` (§Canonical S → ValueFlow Alias 表)

---

## §7 PR-6 #4 gherkin-restructuring (T21-T26)

**目标**：spec.md 24 缩写 bullet → 90 个 `#### Scenario:` Gherkin 块（每 S 8-15 个），覆盖 happy / sad / boundary / concurrent / timeout。

### T21 D1-S13 CaptureUserIntent — 16 Scenario

- happy 5: 入站非空消息持久化 / 飞书入站解析 / D7 路由 / Session 复用 / 命令解析
- sad 4: 空 content 拒绝 / 配置缺失 / Dispatch 失败 / 权限拒绝
- boundary 3: Session TTL 边界 / sequence 重置 / 跨平台一致性
- concurrent 2: 100 并发入站 / Session 锁竞争
- timeout 2: Persist 超时 / 路由超时

### T22 D1-S14 PresentThinking — 12 Scenario

- happy 4: thinking 流式首 chunk / 折叠 / 序列递增 / 链完整性
- sad 3: thinking 超长截断 / 编码失败 / 折叠丢失
- boundary 2: 空 thinking 拒绝 / Unicode 边界
- concurrent 1: 多 Worker 并行 thinking
- timeout 2: 流式超时 / 折叠超时

### T23 D1-S15 PresentTaskProgress — 15 Scenario

- happy 5: tool_call 映射 / Worker 隔离 / 序列递增 / Milestone 展示 / chain 锚点
- sad 4: tool 错误 / Worker 失败 / Milestone 失败 / chain 断裂
- boundary 3: tool 重试 / Worker 取消 / Milestone 跳过
- concurrent 2: N Worker 并行 / tool 并发调用
- timeout 1: tool 慢响应

### T24 D1-S16 DeliverConclusion — 16 Scenario

- happy 6: complete 终态 / text 流式 / error 必达 / sequence 终止 / chain 收口 / footer 兜底
- sad 4: 错误传播 / 终态重入 / 流式中断 / 编码失败
- boundary 3: text 与 thinking 边界 / error 与 complete 互斥 / chain 边界
- concurrent 1: 多 turn Conclusion 隔离
- timeout 2: 流式超时 / 终态超时

### T25 D1-S17 ConnectChannel — 16 Scenario

- happy 5: 飞书 Parse / DingTalk Parse / CLI Parse / 限流通过 / 编码
- sad 5: Parse 错误 / 限流拒绝 / 编码失败 / CardKit 不可用 / Token 失效
- boundary 3: 跨平台一致性 / 限流恢复 / Card 边界
- concurrent 2: 并发 webhook / 并发 CLI
- timeout 1: webhook 超时

### T26 D1-S18 GuaranteeDelivery — 15 Scenario

- happy 5: Critical 必达 / Publish / Drain / Compact / Reconnect
- sad 4: Drain 排空 / 背压拒绝 / Reconnect 失败 / Close 后拒绝
- boundary 4: Critical 顺序 / Normal vs Critical / Drain 阈值 / 状态转换
- concurrent 1: 多 subscriber 并发
- timeout 1: Reconnect 超时

**合计 90 Scenario**，每 Scenario 末尾加 `<!-- T: D1-S{N}-A{NN}-T{XX} -->` 注释。

**验证**：
- `grep -c "^#### Scenario:" spec.md` = 90 ✓
- 5 类别分布: happy 30 / sad 24 / boundary 18 / concurrent 9 / timeout 9 ✓
- 每 Scenario 末尾含 `<!-- T:` 注释 ✓

**关键文件**：
- `openspec/specs/d1-communication/spec.md` (24 缩写 bullet → 90 `#### Scenario:` Gherkin 块)

---

## §8 PR-7 #5 boundary-decision (T27-T30)

**目标**：d1-domain.md §Boundary Debt Decisions + d7-boundary.md (NEW) + 3 单元测试。

### T27 跨域 boundary 审计

- D1 emit `permission.required` → D4 sessionagents/manager (resolvePermission) — RESOLVED
- D1 capture ↔ D7 orchestration entry (D1-S13-A03-F02 routeD7) — RESOLVED
- D1 forbidden orchestration/ import (lint-d1-imports.sh 守门) — RESOLVED
- D1 ↔ D5 observability (10 d1.* span ops) — RESOLVED
- D1 ↔ D2 硬禁直连 IEngine.Process (DM-007) — RESOLVED

### T28 d1-domain.md §Boundary Debt Decisions 章节

- 3 row 表格 (D1→D7 / D1→D4 / D1 forbidden)
- 治理常量位置
- 重新评估触发条件

### T29 d7-boundary.md (NEW, ~60 行)

- §1 D1 owns 的 boundary decision (3 row)
- §2 跨域接线
- §3 Hard Ban
- §4 重新评估触发
- 修订记录 v1.0.0 row

### T30 d1-domain.md v1.0.0 → v1.1.0 + 修订记录 v1.1.0 row

**验证**：
- d1-domain.md §Boundary Debt Decisions 3 row ✓
- d7-boundary.md 存在 ✓
- `go test ./internal/layers/communication/orchtypes/... -race -count=1` 3/3 PASS ✓
- d1-domain v1.0.0 → v1.1.0 ✓

**关键文件**：
- `openspec/specs/d1-communication/d1-domain.md` (§Boundary Debt Decisions 章节 + 修订记录)
- `openspec/specs/d1-communication/d7-boundary.md` (NEW)
- `internal/layers/communication/orchtypes/boundary_decision_test.go` (3 单测已就位)

---

## §9 PR-8 S7_Archive (T31-T34)

**目标**：6 artifacts 复制 + verify-archive 12/12 PASS + d1-domain v1.1.0 + spec v5.0.0。

### T31 6 artifacts 复制到 `openspec/archive/2026-06-30-devrix-d1-ac-restructuring/`

- `.openspec.yaml` (NEW)
- `acceptance-report.md` (NEW)
- `demand.md` / `design.md` / `proposal.md` / `tasks.md` (从 changes/)
- `specs/d1-communication/spec.md` (从 changes/) + `specs/architecture/d1-flow-architecture.md` (NEW)

### T32 verify-archive.sh 12/12 PASS

```bash
./scripts/verify-archive.sh devrix-d1-ac-restructuring
# 期望：12/12 PASS
```

### T33 d1-domain.md v1.0.0 → v1.1.0 + 修订记录 v1.1.0 row

```markdown
| 1.1.0 | 2026-06-30 | **DM-20260629-005 S7_Archive ACCEPTED** (PR-5 value-flow-rename + PR-7 boundary-decision): (1) §North Star 6 ValueFlow Alias 加 D1_ 前缀；(2) §Boundary Debt Decisions 3 row + 治理常量 + 3 单测 PASS；(3) §DSAFT 资产表 4 行物理路径与 code 100% 对齐；(4) §Change line + 修订记录 v1.1.0 row |
```

### T34 spec.md v4.1.1 → v5.0.0 + 修订记录 v5.0.0 row

```markdown
| 5.0.0 | 2026-06-30 | **DM-20260629-005 S7_Archive ACCEPTED** (PR-2 god-doc-split pt1 + PR-6 gherkin-restructuring): (1) 176 → 90 行 god-doc-split；(2) 24 缩写 bullet → 90 `#### Scenario:` Gherkin 块（happy 30 / sad 24 / boundary 18 / concurrent 9 / timeout 9）；(3) d1-flow-architecture.md 拆出 (NEW 80-100 行)；(4) §Change line + 修订记录 v5.0.0 row |
```

**验证**：
- 7 artifacts 存在 (含 1 子 spec dir) ✓
- verify-archive.sh 12/12 PASS ✓
- d1-domain v1.0.0 → v1.1.0 ✓
- spec v4.1.1 → v5.0.0 ✓
- demand-archive-index.md 3 处更新 ✓

**关键文件**：
- `openspec/archive/2026-06-30-devrix-d1-ac-restructuring/` (NEW 7 artifacts)
- `openspec/changes/devrix-d1-ac-restructuring/` (删除)
- `openspec/demand-archive-index.md` (3 处更新：DM row + Archive Locations + 历史批次)
- `openspec/specs/d1-communication/{d1-domain.md, spec.md}` (修订记录 v1.1.0 / v5.0.0 row + §Change line)

---

## §10 验证（end-to-end）

```bash
# 1. 全量 Go 编译
go build ./cmd/devrix/... ./cmd/obs-verify/...

# 2. D1 域全量 -race
go test ./internal/layers/communication/... -race -count=1
# 期望：10+ D1 packages PASS

# 3. Span Evidence 覆盖率
./scripts/d1-span-coverage.sh
# 期望：≥80% (44/44 = 100% effective PASS)

# 4. 跨域 import 门禁
./scripts/lint-d1-imports.sh
# 期望：D1 capture/orchestration/ import 0 命中

# 5. boundary_decision 治理常量
go test ./internal/layers/communication/orchtypes/... -race -count=1
# 期望：3/3 PASS (Exist + VersionFormat + Unique)

# 6. verify-archive
./scripts/verify-archive.sh devrix-d1-ac-restructuring
# 期望：12/12 PASS

# 7. 飞书 E2E 实测
# - 启动 devrix (./scripts/devrix.sh restart)
# - 发送消息验证 D1 Capture→Dispatch→Present→Deliver→Guarantee 链路
# - Jaeger 验证 10 d1.* span ops + 3 orchtypes events
```

---

## §11 PR 落地序列

| Day | PR | 范围 | 验收 |
|-----|----|----|------|
| 1 | PR-1 #0 orchtypes-bootstrap | orchtypes/ + 3 boundary 常量 + 3 单测 | orchtypes -race PASS |
| 2 | PR-2 #1 god-doc-split pt1 | spec.md 176→90 + d1-flow-architecture.md | spec.md ≤ 100 LOC |
| 3 | PR-3 #1 god-doc-split pt2 | observability 230→100 + t-registry §T-Without-Span Tracker | observability ≤ 120 LOC |
| 4 | PR-4 #2 registry-sync | 56 T Span Evidence + Historical S 沉 archive + d1-span-coverage.sh | coverage ≥80% |
| 5 | PR-5 #3 value-flow-rename | 6 D1_* Alias + a/f/t/layer-delta | d1-domain v1.1.0 |
| 6 | PR-6 #4 gherkin-restructuring | 24 缩写 bullet → 90 #### Scenario: 块 | spec.md 90 块 |
| 7 | PR-7 #5 boundary-decision | d1-domain §Boundary + d7-boundary.md | 3 单测 PASS |
| 8 | PR-8 S7_Archive | 6 artifacts + verify-archive 12/12 | spec v5.0.0 |

---

**END of Tasks**
