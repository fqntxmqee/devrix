# Proposal: D7 TaskContract 统一 PR-A — interfaces 包 + TaskSpec/TaskReport 双契约接口层 + 字段语义层

**Change ID:** devrix-d7-taskcontract-unification-pr-a
**Demand ID:** DM-20260629-007
**Parent Demand:** DM-20260629-006 (DESIGN ONLY, S6_Archived)
**Status:** S7_Archived (2026-06-29)
**Priority:** P0
**Reporter:** 2026-06-29 启动 v7.0 演进第一枪
**DSAFT Domain:** D7 Orchestration (核心域)
**DSAFT Layer:** L1 (接口) + L2 (字段语义) + L4 (治理/spec 同步)

---

## 1. Background

`devrix-d7-taskcontract-unification` (DM-20260629-006) 已完成 DESIGN ONLY S6 归档（4-Layer × 3-Phase × 23 AC）。本 Change 启动**第一阶段实施 PR-A**，落地：

- **L1 接口层**：`interfaces` 包 + TaskSpec struct (4+2 字段) + TaskReport struct (5+2 字段)
- **L2 字段语义层**：Dissent + Blockage + Resource 3 字段填充逻辑
- **L4 治理层（PR-A scope）**：5+1 个 spec 文档同步（AC17）

**PR-A 在 4-Layer × 3-Phase 矩阵**：6 AC，~1 周，风险等级"低"，可独立合入。

## 2. Problem Statement

| ID | 问题 | 现状 | PR-A 目标 |
|----|------|------|-----------|
| **P1** | TaskSpec 在 Plan / Channel / WorkItem 三处定义，类型不安全 | `map[string]interface{}` 推断 | AC1: 3 处创建点统一返回 TaskSpec |
| **P2** | TaskReport 缺 Dissent / Blockage / Resource 三元素 | Verdict + Evidence only | AC2 + AC3 + AC4 + AC5: 5+2 字段补齐 |
| **P3** | AdaptiveThreshold 接入 RunTurn (TD-WT-01) 需类型推断 | 孤儿代码 | AC1 解前置依赖（PR-C 实施接入）|
| **P4** | Spec 文档漂移风险 | 23 AC 跨 3 PR，文档不同步累积 | AC17: PR-A 提交前置 spec 同步 |

## 3. Proposed Solution

### 3.1 方案对比

| 方案 | 优点 | 缺点 | 结论 |
|------|------|------|------|
| **A. PR-A scope (推荐)** — 仅 L1 + L2 + AC17 | 风险低；6 AC 紧凑；可独立合入；不影响 PR-B/C | 不解 L3 防御运行时 | ✅ **采用** |
| B. 一次性 23 AC 全做 | 一次性 | 工作量 4.5 周；PR 太大难 review | ❌ |
| C. 仅做 L1 接口层（不做 L2 字段语义）| scope 更小 | 字段语义空缺，consumer 不会调用 | ❌ |

### 3.2 PR-A 核心架构

```
┌────────────────────────────────────────────────────────┐
│         interfaces (Pure types, 0 import D7)           │
│   ┌──────────────────┐  ┌──────────────────┐          │
│   │  TaskSpec (L1)   │  │ TaskReport (L1)  │          │
│   │ 4+2 字段 + builder│  │ 5+2 字段 + builder│          │
│   │ TraceID auto-gen │  │ With* / AppendDissent│      │
│   └──────────────────┘  └──────────────────┘          │
│   ┌──────────────────────────────────────────┐         │
│   │ 5+2 字段语义 (L2)                          │         │
│   │  - Dissent (ExplorationChannel 全量)      │         │
│   │  - Blockage (Verifier 拒绝 + LLM 二次分析)│         │
│   │  - Resource (ContextBudget metric 抽取)   │         │
│   └──────────────────────────────────────────┘         │
└────────────────────────────────────────────────────────┘
         │                           │
         ▼                           ▼
   D7 子包（仅修改 3 创建点）     Learn 节点（消费 TaskReport）
```

### 3.3 PR-A 不做事项（明确划线）

- ❌ L3 防御运行时（AC11/AC12/AC13/AC14/AC15）→ PR-B + PR-C
- ❌ Migration Plan（AC16）→ PR-B
- ❌ Cross-Domain Boundary test（AC21）→ PR-B
- ❌ Feature Flag 灰度（AC22）→ PR-B
- ❌ Convergence span（AC6）→ PR-C
- ❌ AdaptiveThreshold wiring（AC7）→ PR-C
- ❌ Layout guard（AC8）→ PR-C
- ❌ Coverage ≥ 80% 治理（AC18）→ PR-C
- ❌ Performance Budget 治理（AC19 后续 PR 灰度）→ PR-C
- ❌ Security Classification（AC20）→ PR-C

### 3.4 关键决策

#### Decision 1: T 编号策略

**问题：** 父设计文档 §2.2 写 `D7-S16/17/18/19`，但 `D7-S16` 已被 `devrix-d7-layer-subcontext` (DM-20260627-003) 占用。

**选项：**
| 方案 | 优点 | 缺点 |
|------|------|------|
| A. 复用 D7-S16~S19（覆写 Layer SubContext）| 贴父设计 | 覆写 18 个 IMPLEMENTED T 点；reviewer 困惑 |
| B. 重映射 D7-S20~S23 | 保留历史；语义清晰 | 与父设计 §2.2 不一致 |
| C. 重映射 + 在 design.md §2.2 增 footnote 解释 | 兼容 | 文档冗余 |

**选择:** B
**理由:** Layer SubContext 是已合入的 v6.0.x 维护产物，覆写有数据丢失风险；D7-S20~S23 留 v7.0 sprint 专属编号段；父设计归档到 archive/，新 change 用新编号独立闭环。S3-Gate review 时向 reviewer highlight 此决策。

#### Decision 2: interfaces 包路径

**选项：**
| 方案 | 优点 | 缺点 |
|------|------|------|
| A. `internal/layers/orchestration/interfaces/` | 与 D7 同包树；layout guard 自然 | 与未来跨域 interfaces 协调需重命名 |
| B. `internal/layers/orchestration/contracts/` | D7 内部专用 | 与 `decisionplanning/filter_adapter.go` 重名 |
| C. `internal/shared/orchestration/interfaces/` | 跨域共享 | 当前无跨域需求 |

**选择:** A
**理由:** D7 是 TaskSpec/TaskReport 唯一 owner；A 是最少惊讶方案；与父设计 Decision 1 一致。

#### Decision 3: PR-A 期间 ChannelRequest/LearnRequest 兼容性

**问题:** PR-A 引入 TaskSpec/TaskReport 后，调用方（Channel.Execute / mups/learn）需要适配，但 PR-B 才做完整迁移。

**选项：**
| 方案 | 优点 | 缺点 |
|------|------|------|
| A. PR-A 直接 breaking change 替换 | 干净 | 风险大；PR-B 才能完整测 |
| B. PR-A 嵌入新字段（additive），PR-B 完整迁移 | 渐进 | 字段冗余 |
| C. PR-A 仅在 create 路径返回 TaskSpec，调用方维持原类型 | 中间态 | 调用方不知新类型存在 |

**选择:** B
**理由:** PR-A 阶段 ChannelRequest 增加 `Spec *TaskSpec` 字段（additive），LearnRequest 增加 `Report *TaskReport` 字段；老字段保留 1 minor 版本（PR-B 完整迁移 + type alias，PR-C 移除）。这是 PR-A → PR-B 的标准过渡。

#### Decision 4: Dissent top-N 默认值

**选项：** top-3 / top-5 / top-10

**选择:** top-3
**理由:** SkillMemory.SOP 写入延迟与 entry 数线性相关；top-3 平衡"保留少数派价值"与"沉淀成本"。

## 4. Success Metrics (PR-A scope)

| 指标 | 目标值 | 测量方式 |
|------|-------|---------|
| `interfaces.TaskSpec` 创建点统一率 | 100% | `grep -rn "interfaces.NewTaskSpec" internal/layers/orchestration/` 必须 ≥ 3 处 |
| `interfaces.TaskReport` 出口 + 入口统一 | 100% | `grep -rn "interfaces.NewTaskReport" internal/layers/orchestration/` 必须 ≥ 2 处 |
| Dissent 字段填充率（VERDICT=INDETERMINATE 时）| 100% | LP-3 测试 + span `taskreport.dissent_recorded` |
| Blockage 字段结构化分类 | 100% | 3 类 kind (missing/infeasible/required_external) 单元测试 |
| Resource 字段 per-Plan 度量抽取 | 100% | ContextBudget metric 桥接测试 |
| `interfaces` 包测试覆盖率 | ≥ 80% | `go test -cover ./internal/layers/orchestration/interfaces/` |
| `interfaces` 包 0 import D7 任何子包 | 100% | layout guard check |
| 22/22 orchestration packages `-race` PASS | 100%（不退化）| `go test -race -count=1` |
| LP-1/LP-2/LP-5 100% 兼容 | 100% | 回归测试集 |
| TaskSpec / TaskReport 构造 P99 | < 1ms | `go test -bench` + Jaeger histogram |
| Spec 文档同步 | 5+1 文件 | git diff `openspec/specs/d7-orchestration/` 非空 |

## 5. Implementation Plan

| 步骤 | 内容 | 行数 | AC |
|------|------|------|-----|
| 1 | `interfaces/{doc.go, errors.go}` 包骨架 | ~110 | AC1, AC2 |
| 2 | `interfaces/task_spec.go` TaskSpec struct + builder + Validate | ~180 | AC1 |
| 3 | `interfaces/task_report.go` TaskReport struct + 5 子类型 + builder | ~280 | AC2 |
| 4 | `interfaces/task_spec_test.go` 单测 | ~200 | AC1 |
| 5 | `interfaces/task_report_test.go` 单测 | ~280 | AC2 |
| 6 | `interfaces/taskcontract_test.go` Round-trip 测试 | ~150 | AC1, AC2 |
| 7 | `mups/execute/channel.go` 创建路径 → TaskSpec | ~20 | AC1 |
| 8 | `mups/execute/{commit,scenario,protocol}.go` 出口 → TaskReport | ~30 | AC2 |
| 9 | `mups/execute/exploration.go` 全量结果 → TaskReport.Dissent | ~20 | AC3 |
| 10 | `mups/learn/learner.go` 入口 → TaskReport | ~20 | AC2, AC3 |
| 11 | `workmodel/workitem.go` 创建路径 → TaskSpec | ~15 | AC1 |
| 12 | `workmodel/child_downlink.go` 嵌入 TaskSpec 引用 | ~10 | AC1 |
| 13 | `decisionplanning/decomposer.go` 分解产出 → TaskSpec | ~15 | AC1 |
| 14 | `decisionplanning/filter.go` Dissent 过滤 | ~15 | AC3 |
| 15 | `sessionorchestrator/{orchestrator,turn_orchestrator}.go` 消费 TaskSpec/TaskReport | ~30 | AC1, AC2 |
| 16 | `wavescheduler/scheduler.go` dispatchOne 接收 TaskSpec | ~10 | AC1 |
| 17 | Spec 文档同步（spec.md + d7-domain.md + a/f/t/span-registry.md）| ~50 | AC17 |

**总预估：~1435 行新增 / ~50 行修改**（含测试与 spec）

## 6. Risks & Mitigations

完整风险表见 `demand.md` §4。架构级 P0 风险：

| 风险 | 影响 | 缓解 |
|------|------|------|
| interfaces 包 import cycle | 编译失败 | 仅放 type + builder，0 import D7 |
| TaskSpec/TaskReport 引入后老调用方断裂 | 大面积编译失败 | Decision 3: additive 字段嵌入，PR-B 完整迁移 |
| Dissent 写入慢 | Learn 节点延迟 | top-3 截断 + summary hash 引用 |
| Resource 字段需重新埋点 | 增加 instrumentation | 复用 ContextBudget Phase B 现有 metric |
| 6 AC 跨 14 文件改动 | review scope 大 | 每个文件改动静默 < 30 行；reviewer 可逐步消化 |
| **T 编号冲突（Decision 1）** | reviewer 困惑父设计与本设计 §2.2 不一致 | S3-Gate review 时 highlight；design.md §2.2 增 footnote |

## 7. Out of Scope

- L3 防御运行时全部 5 个 AC（AC11/AC12/AC13/AC14/AC15）
- L4 治理横切层除 AC17 外的 8 个 AC（AC6/AC7/AC8/AC9/AC10/AC16/AC18/AC19/AC20/AC21/AC22/AC23）
- interfaces/v2 子包演进（v8.0 规划）
- Reference Adapter 全量实现（v7.0.x 维护期）
- Operator Runbook（v7.0.x 维护期）

## 8. 关联变更

| Change ID | DM ID | 关系 | 说明 |
|-----------|-------|------|------|
| devrix-d7-taskcontract-unification | DM-20260629-006 | 父 DESIGN | archive/2026-06-29-devrix-d7-taskcontract-unification/ |
| devrix-d7-dsaft-restructuring | DM-20260629-001 | 前置 | v6.0.x 维护收官 |
| devrix-d7-six-s-simplification | DM-20260626-001 | 前置 | 14 S → 6 S |
| devrix-d7-mups-v5-escape-engine | DM-20260625-003 | 前置 | 5 层 CB（PR-B 接入）|
| devrix-d7-layer-subcontext | DM-20260627-003 | **T 编号冲突** | D7-S16 占用 |
| devrix-d7-multiturn-session-state | DM-20260628-004 | 并行 | S3-Gate 互审 |

## 9. 备注

本次 S2 proposal 同步推进的子规范符合度：
- ✅ `architecture-design.md §1.1` DSAFT 五层（D + S + A + F + T）已纳入 §1 + §3.2
- ✅ `architecture-design.md §3` proposal 9 sections 已完整（+ §3.4 关键决策 4 条 + §8 关联变更 + §9 备注）
- ✅ `architecture-design.md §5` 禁止工时估算：本提案无 hours 估算，仅 line counts（§5 步骤表）
- ✅ `architecture-design.md §7` 设计决策记录：§3.4 已列 4 个 Decision（其中 Decision 1 显式说明 T 编号重映射）
- ✅ `architecture-design.md §8` S2 检查清单：`.openspec.yaml` 字段 + `dsaft_*` 标注见 S3 design.md
- ✅ `feedback-devrix-bugfix-pr-grouping.md`：PR-A scope 紧凑，单 PR 可独立合入

下一阶段 S3 design.md 将产出：
- 根因分析（4 个 P 问题如何被 6 AC 系统解决）
- 关键接口/类型完整定义（TaskSpec/TaskReport 5+1 子结构）
- 数据流图（TaskSpec 创建点 → Channel.Execute → Learn 节点）
- 文件清单（新增 7 文件 + 修改 19 文件 + spec 同步 6 文件）
- 回归风险评估（22/22 -race + LP-1/LP-2/LP-5）
- S3-Gate review-design.md 高亮 Decision 1（T 编号冲突）