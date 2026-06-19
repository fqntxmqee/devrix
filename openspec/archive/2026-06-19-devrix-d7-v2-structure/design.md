# Design: D7 v2.0 结构重构

**Change ID:** devrix-d7-v2-structure  
**Demand ID:** DM-20260619-005  
**Status:** S3_Draft  
**Methodology:** DSAFT Refactoring Playbook §6 双锚点对齐

---

## 1. 架构目标

在 **不改变 North Star 与 T 契约** 的前提下，完成：

1. 规格锚点与物理锚点 1:1 对齐（S → scenario-slug 目录）
2. S2/S5 激励分离（sessionorchestrator vs decisionplanning）
3. WorkTree 单一语义 SoT（TD-WT-02/03）

---

## 2. Decision 记录

### Decision: 重构范围（Owner Q1）

| 方案 | 优点 | 缺点 |
|------|------|------|
| A: 仅文档 | 零风险 | 不解决根因 |
| B: 文档+路径 | 闭合双锚点 | WorkTree 双 SoT 残留 |
| **C: 文档+路径+清债** | 一次闭合结构+数据模型债 | PR 面宽，需分 Phase |

**选择:** C  
**理由:** Owner 显式确认；TD-WT-02/03 为 P1 技术债，与路径迁移同属 v2.0 Structure 范畴  
**影响:** 3 Phase、4+ PR；TD-WT-01/04/05/06 仍 defer

### Decision: 总编排权归属（Owner Q2）

| 方案 | 优点 | 缺点 |
|------|------|------|
| **A: sessionorchestrator 总编排** | 与 S2 Meta-Orchestrator 博弈角色一致 | 需定义 S5 接口边界 |
| B: 保留 coordinator facade | 迁移量小 | 延迟 S 层物理分离收益 |

**选择:** A  
**理由:** S2 = Screening + Turn Leader；S5 = Information Producer，不应拥有 ingress  
**影响:** `ProcessMessage` 迁至 `sessionorchestrator/orchestrator.go`；S5 暴露 `Classifier` / `Decomposer` / `Executor` 接口

### Decision: hubspoke 拆分（Owner Q3）

| 方案 | 优点 | 缺点 |
|------|------|------|
| A: 整包归 S2 | 改动少 | S4-A05 追溯链在物理层断裂 |
| **B: dispatch→S2, bridge→S4** | 与 A 层登记一致 | 需拆 import |

**选择:** B  
**理由:** `D7-S2-A04 DispatchWorker` vs `D7-S4-A05 SpokeBridge` 职责不同  
**影响:** `hubspoke/dispatch.go` → `sessionorchestrator/dispatch.go`；`hubspoke/agent_bridge.go` → `executionflow/bridge.go`

### Decision: T 层策略（Owner Q4）

**选择:** T ID 不变，测试文件随实现迁移  
**理由:** 66 IMPLEMENTED T 是硬约束（Playbook 原则 3）  
**影响:** 不新增 import-path T；验收靠既有 T 全绿

### Decision: D2 范围（Owner Q5）

**选择:** 本轮不动 D2  
**影响:** `PreparedTurnRunner` 链保持；`d7-boundary.md` 仅更新 Task/PlanMode 状态标注

---

## 3. 目标目录树

```text
internal/layers/orchestration/
├── workmodel/                    # D7-S1（不变）
├── sessionorchestrator/        # D7-S2
│   ├── entry.go                  # IOrchestrationEntry 实现
│   ├── orchestrator.go           # ProcessMessage
│   ├── fastpath.go
│   ├── orchestrate_path.go
│   ├── command_handler.go
│   ├── interrupt.go
│   ├── routing.go
│   └── dispatch.go               # 原 hubspoke/dispatch
├── turn/                         # D7-S2-A06/A07（不变位置）
├── decisionplanning/             # D7-S5
│   ├── classifier.go
│   ├── classifier_fallback.go
│   ├── shadow_classifier.go
│   ├── decomposer.go
│   ├── llm_decomposer.go
│   └── executor.go
├── wavescheduler/                # D7-S3（原 wave/）
│   ├── scheduler.go
│   ├── taskgraph.go
│   ├── context.go
│   └── conflict.go
├── executionflow/                # D7-S4
│   ├── hub/                      # 原 flow/
│   ├── workplan/                 # 原 workplan/
│   ├── imsink/                   # 原 imsink/
│   └── bridge.go                 # 原 hubspoke/agent_bridge
├── runregistry/                  # S1 支撑
├── delegatetools/                # F
├── sessionqueue/                 # F
├── toolpolicy/                   # F
├── milestone/                    # F
├── turn_adapter/                 # D2↔D7 适配（不变）
└── coordinator/                  # LEGACY SHIM（1 release）
    └── doc.go                    # Deprecated: use sessionorchestrator
```

---

## 4. 迁移映射表

| Legacy 路径 | Canonical 路径 | S 层 | PR |
|-------------|----------------|------|-----|
| `orchestration/wave/` | `orchestration/wavescheduler/` | S3 | B1 |
| `orchestration/flow/` | `orchestration/executionflow/hub/` | S4 | B2 |
| `orchestration/workplan/` | `orchestration/executionflow/workplan/` | S4 | B2 |
| `orchestration/imsink/` | `orchestration/executionflow/imsink/` | S4 | B2 |
| `coordinator/orchestrator.go` 等 S2 文件 | `sessionorchestrator/` | S2 | B3 |
| `coordinator/classifier*.go` 等 S5 文件 | `decisionplanning/` | S5 | B3 |
| `hubspoke/dispatch.go` | `sessionorchestrator/dispatch.go` | S2 | B4 |
| `hubspoke/agent_bridge.go` | `executionflow/bridge.go` | S4 | B4 |

**Shim 策略（1 release）：**

```go
// orchestration/coordinator/orchestrator.go (shim)
package coordinator

import "devrix/internal/layers/orchestration/sessionorchestrator"

type Entry = sessionorchestrator.Entry
// ... type aliases for backward compat
```

---

## 5. Phase C：WorkTree Legacy 清债

### TD-WT-02: TaskNode 退化为 dispatch 投影

**现状：** `wave.TaskGraph` 独立持有 `TaskNode`，与 `workmodel.WorkTree` 并行。

**目标：**

```text
WorkTree (语义 SoT)
    └── SyncWaveNodes() → wavescheduler.TaskGraph（内存投影，不写盘）
            └── WaveScheduler.dispatchLoop 只读投影
```

**约束：**
- `TaskNode` 字段从 WorkTree 节点映射，不独立 CRUD
- 删除 TaskNode 磁盘持久化路径（若有）
- 现有 `D7-S3-T01..T10` 行为不变

### TD-WT-03: sc.Todos 降级

**现状：** `todo_write` 已写 WorkTree checklist；`sc.Todos` 仍可能被 D2 prepare 读取。

**目标：**
- `sc.Todos` 标注 `Deprecated` 读投影
- 写入路径 audit：`grep` 确认无新写入 `sc.Todos` 作为 SoT
- D2 prepare 可从 WorkTree checklist 投影填充（只读）

---

## 6. bootstrap 接线变更

```text
WireD7 (wire_coordinator.go)
  ├── sessionorchestrator.NewEntry(decisionplanning.Classifier, ...)
  ├── turn.NewOrchestrator(...)
  ├── executionflow.NewHub(...)
  └── wavescheduler.NewScheduler(...)
```

`gateway.SetOrchestrationEntry` 接收 `sessionorchestrator.Entry`（实现 `IOrchestrationEntry`）。

---

## 7. 规格同步清单（Phase A）

| 文件 | 变更 |
|------|------|
| `design.md` | v2.3→v3.0；S1–S5 IMPLEMENTED；删除 PLANNED API 表 |
| `layer-delta.md` | 新增 §v2.0-Structure IMPLEMENTED；旧 §PLANNED → HISTORICAL |
| `d7-boundary.md` | Task/PlanMode 🔶→✅；更新包路径 |
| `code-layout.md` §4.2 | 5/5 ✅；hubspoke 拆分行 |
| `a-registry.md` | Code Location 列 |
| `dsaft-architecture.md` | Stub → 真实计数 |
| `task-planning-design.md` | 状态 ⬜→✅ |

---

## 8. 质量门

每 Phase 合并前：

- [ ] `go test ./...`
- [ ] `go test -race ./internal/layers/orchestration/...`
- [ ] layer-lint strict
- [ ] `tests/integration/d7/` 全绿
- [ ] `git grep 'orchestration/coordinator'` 仅 shim 目录命中（Phase B3 后）
- [ ] 66/66 T 状态保持 IMPLEMENTED

---

## 9. 风险

| # | 风险 | 概率 | 缓解 |
|---|------|------|------|
| R1 | import 遗漏导致编译失败 | 中 | 分 PR + shim |
| R2 | layer-lint 路径白名单过期 | 中 | 同步 `d7_boundary_test.go` |
| R3 | WorkTree 投影回归 | 低 | Phase C 在 B 稳定后；现有 wave 测试 |
| R4 | 文档 follow-up 遗漏 | 中 | Phase A 先行 merge |

---

## 10. Grill Review 预留

| # | 命题 | 状态 |
|---|------|------|
| G1 | S2 总编排不膨胀为「万能 S」 | 待 S3-Gate |
| G2 | Phase C 不引入 D2 变更 | ✅ Owner 确认 |
| G3 | shim 1 release 足够 | 待 S3-Gate |
