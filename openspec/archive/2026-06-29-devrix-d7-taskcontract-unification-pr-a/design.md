# Design: D7 TaskContract 统一 PR-A — interfaces 包 + TaskSpec/TaskReport 双契约

**Change ID:** devrix-d7-taskcontract-unification-pr-a
**Demand ID:** DM-20260629-007
**Status:** S3_Design
**Parent Proposal:** `proposal.md`
**Parent Demand:** `demand.md`
**Template:** `docs/methodology/detail-design-framework.md`（六段式 ①-⑥）
**Created:** 2026-06-29
**Parent Design Reference:** `openspec/archive/2026-06-29-devrix-d7-taskcontract-unification/design.md`（648 行全文版，本设计为 PR-A scope 紧凑版）

---

## ① 架构目标

### 1.1 业务目标

解决 D7 v6.0.x **"机制层丰富 + 契约层分散"** 二维不均衡的根因。本 PR-A 落地 6 AC（占 23 AC 总量的 26%），是 `devrix-d7-taskcontract-unification` (DM-20260629-006) v7.0 演进第一枪的**第一阶段**。

| 痛点（来自 6/21 D7 deep review 15+ 改进点）| 本 PR-A 对应 AC |
|----------------------------------------|----------------|
| Plan / Channel / WorkItem 三处定义不统一（缺 TaskSpec）| **AC1** |
| Verdict + Evidence + ExitReason 之外缺 Dissent / Blockage / Resource（缺 TaskReport）| **AC2, AC3, AC4, AC5** |
| Spec 文档漂移风险（23 AC 跨 3 PR 累积）| **AC17** |

### 1.2 技术目标（PR-A scope 量化指标）

| 指标 | 目标值 | 触发 AC |
|------|-------|---------|
| interfaces 包 0 依赖 D7 子包 | Pure types（防 import cycle）| AC1, AC2 |
| TaskSpec / TaskReport 构造 P99 | < 1ms | AC19 (preliminary) |
| 新增包 Coverage | ≥ 80% | AC18 (PR-C 正式) |
| 22/22 orchestration packages `-race` PASS | 100%（不退化）| AC9 (PR-B) |
| LP-1 / LP-2 / LP-5 闭环 | 100% 兼容 | AC10 (PR-B) |
| Dissent entry 数量 | ≤ 3（top-3 截断）| AC3 |

### 1.3 约束条件

- **SemVer 兼容**：v6.0.x → v7.0.x 是 minor bump（PR-A additive 字段嵌入，PR-B 完整迁移，PR-C 移除 type alias）
- **不破坏现有 v6.0.x API**：ChannelRequest / LearnRequest 通过**新增嵌入字段**保留 1 minor 版本（Decision 3）
- **不破坏 LP-1/LP-2/LP-5 闭环**：5 节点管道完整保留，TaskReport 仅作为 Learn 节点入参增强
- **Pure types 原则**：`interfaces` 包 0 import D7 任何子包（防循环依赖）
- **错误码基础集合**：PR-A 仅定义 5 个基础 `ORCH_*` SentinelError（GoalEmpty + TraceIDEmpty × 2 + DissentRejection + ResourceInvalid）

---

## ② 架构原则

### 2.1 设计原则（PR-A scope）

| 原则 | 落地方式 | 对应 AC |
|------|---------|---------|
| **契约钉死 > 机制灵活** | 单一 `interfaces` 包统一下行/上行 | AC1, AC2 |
| **不可变 API** | `With*` / `AppendDissent` 全部返回新副本（`c := *s` 浅拷贝）| AC1, AC2 |
| **Pure types 防 cycle** | `interfaces` 包 0 import D7 子包 | AC1, AC2 |
| **CoW 追加不覆盖** | Dissent 仅追加（`AppendDissent`）| AC3 |
| **Top-N 截断** | Dissent 保留 top-3（默认）+ summary hash | AC3 |
| **错误码三层闭合** | ORCH_* SentinelError + Code + Message + Remediation | AC23 (PR-A 基础 5 个) |

### 2.2 命名规范（PR-A scope）

| 类别 | 规范 | 示例 |
|------|------|------|
| **DSAFT S 层** | **D7-S20~S23**（重映射，详见 Decision 1）| `D7-S20 L1 Interface Layer` |
| **Activity ID** | `D7-S{20-23}-A{01-09}` | `D7-S20-A01 TaskSpec struct + builder` |
| **Function ID** | `D7-S{20-23}-A{XX}-F{XX}` | `D7-S20-A01-F01 NewTaskSpec` |
| **Test ID** | `D7-S{20-23}-A{XX}-T{XX}` | `D7-S20-A01-T01` |
| **Span Op** | `d7.s{20-23}.<component>.<verb>` | `d7.s20.interfaces.task_spec.created` |
| **Metric Name** | `<component>_<purpose>_<unit>`（snake_case）| `task_spec_constructed_total` |
| **Error Code** | `ORCH_<DOMAIN>_<CONDITION>`（UPPER_SNAKE）| `ORCH_TASK_SPEC_GOAL_EMPTY` |
| **Type** | 顶层 type 用 PascalCase | `TaskSpec`, `DissentEntry` |
| **Builder** | `New<X>(required_args) (*X, error)` + `With<Field>(v) *X` 不可变 | `NewTaskSpec(goal)` / `s.WithConstraint(...)` |

### 2.3 T 编号重映射说明（Decision 1 ⚠️）

> **本设计与父 DESIGN §2.2 的关键差异**：父 DESIGN 写 `D7-S16/17/18/19`，但 `D7-S16` 已被 `devrix-d7-layer-subcontext` (DM-20260627-003) Layer SubContext 占用（18 T 点全 IMPLEMENTED）。本 Change **重映射为 `D7-S20~S23`**：

| S ID | Layer | PR-A scope | 后续 PR scope |
|------|-------|-----------|--------------|
| **D7-S20** | L1 接口层 | TaskSpec + TaskReport（本 PR-A）| — |
| **D7-S21** | L2 字段语义层 | Dissent + Blockage + Resource（本 PR-A）| — |
| **D7-S22** | L3 防御运行时层 | — | PR-B: Pessimistic + HardEvidence / PR-C: CoW + Fallback + Similarity |
| **D7-S23** | L4 治理横切层 | AC17 spec 同步（本 PR-A）| PR-B: race + LP + Migration + Boundary + Flag + ErrorCode / PR-C: convergence span + AdaptiveThreshold + Layout guard + Coverage + Perf + Security |

**S3-Gate reviewer 注意**：本 Change 编号策略与父 DESIGN 显式不一致，已在 proposal.md Decision 1 记录。父 DESIGN 归档在 `openspec/archive/2026-06-29-devrix-d7-taskcontract-unification/`，不影响历史。

### 2.4 代码风格

- **函数 < 50 行**：`NewTaskSpec` / `Validate` / `With*` 全部 < 20 行
- **文件 < 800 行**：`task_spec.go` ~180 行 / `task_report.go` ~280 行
- **统一 TraceID 贯穿**：TaskSpec.TraceID → ChildDownlink.TraceID → TaskReport.TraceID → Learn.Asset.SourceTraceID 全链路一致
- **不可变数据结构**：`c := *s; c.Field = newVal; return &c` 模式（无 `sync.Mutex`）
- **Layout guard 准备**：PR-A 仅在注释里声明"interfaces 包 0 import D7 子包"，PR-C 完整实施 layout guard

---

## ③ 业务流程

### 3.1 核心用例 — Downlink 端到端（PR-A 落地，AC1）

```
D1 Gateway (feishu/cli)
    ↓ ProcessMessage(sessionID, directive)
D7 SessionOrchestrator.ProcessMessage
    ↓ ① [NEW] interfaces.NewTaskSpec(directive)
    ↓      .WithConstraint("scope_out", ...).WithBudget(...)
    ↓      → AC1 TaskSpec 构造（TraceID 自动生成 "ts_<uuid8>"）
    ↓      → Span emit "d7.s20.interfaces.task_spec.created"
    ↓ ② EnsureGoal(sessionID, taskSpec)
    ↓      → ChannelRequest.Spec = taskSpec（additive，PR-B 完整迁移）
    ↓ ③ decompose.TaskGraphSynthesize(taskSpec)
    ↓ ④ WaveScheduler.dispatch(taskGraph)
    ↓      → ChannelRouter.Route(spec) — ChannelRequest 携带 spec 引用
    ↓ ⑤ Channel.Execute(ctx, spec)
    ↓      → 接收 TaskSpec 而非裸 plan
    ↓ ⑥ WorkerAgent.Run(spec)
    ↓ ⑦ emit("child_task_started", spec.TraceID)
```

**时序标注：**
- ① TaskSpec 构造 < 1ms（构造器纯内存操作）
- ⑤ Channel.Execute 走 5 层 CB（P99 < 10ms，不含 worker 执行）

### 3.2 核心用例 — Uplink 端到端（PR-A 落地，AC2-AC5）

```
WorkerAgent / ToolRunner
    ↓ return ToolResult
Channel.Execute 出口
    ↓ ① [NEW] interfaces.NewTaskReport(spec.TraceID)
    ↓      → Span emit "d7.s20.interfaces.task_report.created"
    ↓ ② [NEW] .WithResult(...).WithEvidence(...) [AC2]
    ↓ ③ [NEW ExplorationChannel only] .AppendDissent(minorityEntry) [AC3]
    ↓      → 触发条件：VERDICT=INDETERMINATE 或 fallback_used=true
    ↓      → top-3 截断 + summary hash
    ↓      → Span emit "d7.s21.taskreport.dissent_recorded"
    ↓ ④ [NEW on Verifier reject] .WithBlockage(...) [AC4]
    ↓      → 3 类 kind：missing / infeasible / required_external
    ↓      → Span emit "d7.s21.taskreport.blockage_recorded"
    ↓ ⑤ [NEW] .WithResource(spec.Resource) [AC5]
    ↓      → 从 ContextBudget Phase B 抽取 per-Plan token/time/step
    ↓      → Span emit "d7.s21.taskreport.resource_recorded"
    ↓ 5 层 CB.Evaluate(report) — 读 report.Blockage 作为升级信号
    ↓ Verifier.Verify(artifact, report) — 拒绝"空证 PASS"（AC15 实施在 PR-B）
    ↓ ⑥ emit("task_completed", report)
SessionOrchestrator
    ↓ ⑦ WaitForCompletion(sessionID)
    ↓ ⑧ mups/learn/learner.go :: Learn(report)
    ↓      → LearnRequest.Report = report（additive，PR-B 完整迁移）
    ↓      → BayesianUpdate(report.TraceID, report.Result.Confidence)
    ↓      → Dissent 沉淀至 SkillMemory.SOP
```

### 3.3 Dissent 填充规则（AC3 关键逻辑）

```go
// mups/execute/exploration.go 改造（PR-A scope）
func (c *ExplorationChannel) Execute(ctx context.Context, spec *interfaces.TaskSpec) (*interfaces.TaskReport, error) {
    // 全量结果保存（已有逻辑保留）
    candidates := c.runAllCandidates(ctx, spec)

    // [NEW PR-A] 构造 TaskReport
    report := interfaces.NewTaskReport(spec.TraceID).
        WithResult(aggregateResult(candidates))

    // [NEW PR-A] 填充 Dissent（top-3 截断）
    if report.Result.Kind == interfaces.ResultIndeterminate || c.fallbackUsed {
        for i, minority := range minorityCandidates(candidates, 3) {  // top-3
            entry := interfaces.DissentEntry{
                Source:    minority.Source,
                Decision:  minority.Decision,
                Reason:    minority.Reason,
                Summary:   hash(minority.Content),  // summary hash 引用
                Timestamp: now(),
            }
            report = report.AppendDissent(entry)
        }
    }
    return report, nil
}
```

### 3.4 Blockage 填充规则（AC4 关键逻辑）

```go
// decisionplanning/filter.go 改造（PR-A scope）
func blockFromVerifierRejection(rej verifier.Rejection) interfaces.Blockage {
    kind := interfaces.BlockageMissing  // default
    switch rej.Category {
    case "missing_input":
        kind = interfaces.BlockageMissing
    case "infeasible_path":
        kind = interfaces.BlockageInfeasible
    case "required_external":
        kind = interfaces.BlockageRequiredExternal
    }
    return interfaces.Blockage{
        Kind:        kind,
        Description: rej.Message,
        Source:      rej.VerifierID,
        Traceback:   rej.CallChain,
    }
}
```

### 3.5 Resource 抽取规则（AC5 关键逻辑）

```go
// decisionplanning/decomposer.go 改造（PR-A scope）
func resourceFromBudget(spec *interfaces.TaskSpec, sessionCtx context.Context) interfaces.Resource {
    budget := contextBudget.FromContext(sessionCtx)
    return interfaces.Resource{
        TokensUsed:    budget.TokensUsed(),
        TokensBudget:  spec.CostBudget.Tokens,
        TimeElapsed:   budget.Elapsed(),
        StepCount:     budget.StepCount(),
        ToolInvocations: budget.ToolCalls(),
    }
}
```

### 3.6 异常补偿（PR-A scope 限定）

PR-A 阶段**不实施** L3 防御运行时（AC11/AC12/AC13/AC14/AC15），但保留字段以便 PR-B 接入：

- **TaskReport.MVPArtifact 字段**（PR-A 预留 nil，PR-B 接入 AC11）
- **TaskReport.FallbackUsed bool**（PR-A 预留 false，PR-B 接入 AC11）
- **TaskReport.Dissent / Blockage / Resource**（PR-A 完整实施）
- **HardEvidence 验证**（PR-A 仅预留字段，PR-B 接入 AC15）

---

## ④ 领域模型

### 4.1 聚合根（4 个 — PR-A scope）

| 聚合根 | 路径 | 职责 | 不可变性 |
|--------|------|------|---------|
| **TaskSpec** | `interfaces/task_spec.go` | 下行传播契约（4+2 字段）| 不可变（`With*` 浅拷贝）|
| **TaskReport** | `interfaces/task_report.go` | 上行反馈契约（5+2 字段）| 不可变（`With*` + `AppendDissent`）|
| **DissentEntry** | `interfaces/dissent.go` | 少数派报告载体 | 不可变值对象 |
| **Blockage** | `interfaces/blockage.go` | 阻塞信号载体 | 不可变值对象 |

### 4.2 限界上下文（PR-A scope — Pure types 边界）

```
┌─────────────────────────────────────────────────────────────┐
│                    interfaces (Pure types)                   │
│   Pure types: 0 import D7 子包 / 防循环依赖 / Layout guard 守护 │
└─────────────────────────────────────────────────────────────┘
         │ 仅允许白名单 9 包 import（PR-A 阶段尚未启用 guard，PR-C 完整实施）
         ↓
┌─────────────────────────────────────────────────────────────┐
│  D7 6 S + 1 横切（PR-A 修改的子包）                              │
├─────────────────────────────────────────────────────────────┤
│  L1 接口层消费方                                               │
│    S1 WorkModel + S2 SessionOrchestrator + S3 WaveScheduler   │
│    S4 ExecutionFlow + S5 DecisionPlanning + S6 MUPS Pipeline   │
│  + Hardening 横切（Discipline Keeper）                          │
└─────────────────────────────────────────────────────────────┘
```

**Layout guard 白名单 import 列表**（PR-A 阶段注释声明，PR-C 实施）：
- `mups/execute`, `mups/learn`, `mups/observe`
- `workmodel`
- `decisionplanning`
- `escape`
- `hardening`
- `sessionorchestrator`
- `executionflow`
- `d7-bootstrap`

### 4.3 领域事件（5 个 span — PR-A scope）

| Span | Op | 触发点 | AC |
|------|----|----|-----|
| `interfaces.task_spec.created` | `d7.s20.interfaces.task_spec.created` | `NewTaskSpec()` 出口 | AC1 |
| `interfaces.task_report.created` | `d7.s20.interfaces.task_report.created` | `NewTaskReport()` 出口 | AC2 |
| `taskreport.dissent_recorded` | `d7.s21.taskreport.dissent_recorded` | `AppendDissent()` | AC3 |
| `taskreport.blockage_recorded` | `d7.s21.taskreport.blockage_recorded` | `WithBlockage()` | AC4 |
| `taskreport.resource_recorded` | `d7.s21.taskreport.resource_recorded` | `WithResource()` | AC5 |

### 4.4 跨域消费模型（PR-A 暂不实施 boundary test，PR-B 完整）

| 域 | 消费字段 | 行为 | boundary test（PR-B） |
|----|---------|------|----------------------|
| D2 上下文引擎 | `TaskSpec.Goal` + `ConvergenceBudget` | 注入 context budget | `TestBoundary_D2_ConsumeTaskSpec` |
| D4 多智能体 | `TaskSpec.HardConstraints` | worker 阻塞违反约束 | `TestBoundary_D4_ConsumeTaskSpec` |
| D6 演化层 | `TaskReport.Result` + `Dissent` | advisory 校验 | `TestBoundary_D6_ConsumeTaskSpec` |

---

## ⑤ 核心链路图

### 5.1 Downlink 链路端到端（详见 §3.1）

```
D1 Gateway → D7 SessionOrchestrator → Decomposer → WaveScheduler → Channel → Worker
   ↓ 入口      ↓ ① NewTaskSpec            ↓ ③ Synthesize   ↓ ④ dispatch     ↓ ⑤ Execute   ↓ ⑥ Run
```

**节点 SLA 承诺（PR-A 阶段）：**

| 节点 | 职责 | P99 上限 | 单点风险 |
|------|------|---------|---------|
| `interfaces.NewTaskSpec` | 强类型下行契约 | < 1ms | **interfaces 包崩溃 = 整个 v7.0 崩溃**（layout guard 守护）|
| `SessionOrchestrator.ProcessMessage` | 主入口 | < 5ms（不含 LLM）| 已有 TurnState 串行化（DM-20260628-004）|
| `decompose.TaskGraphSynthesize` | 任务拆解 | < 50ms | Similarity Check 拦截（PR-C 实施）|
| `WaveScheduler.dispatch` | DAG 调度 | < 10ms | 5-slot WorkerPool（v6.0.x 已就位）|
| `Channel.Execute` | 4 PlanKind 1:1 路由 | < 100ms | 5 层 CB（v6.0.x 已就位）|

### 5.2 Uplink 链路端到端（详见 §3.2）

```
Worker → Channel.Execute → 5 层 CB → Verifier → SessionOrchestrator → Learn
   ↓ 入口   ↓ ① NewReport    ↓ ② 升级    ↓ ⑤ 验证  ↓ ⑦ WaitForCompletion  ↓ ⑨ BayesianUpdate
```

**节点 SLA 承诺（PR-A 阶段）：**

| 节点 | 职责 | P99 上限 | 单点风险 |
|------|------|---------|---------|
| `interfaces.NewTaskReport` | 强类型上行契约 | < 1ms | 同 NewTaskSpec |
| `Verdict.Aggregate` | 4 态聚合 | < 1ms | 已就位 |
| `5 层 CB.Evaluate` | 异常拦截 | < 5ms | 已就位 |
| `Verifier.Verify` | 硬证据校验 | < 2ms | AC15 实施在 PR-B |
| `Learner.Learn` | 5 节点闭环 | < 100ms | LP-1 兼容（AC10 PR-B 验证）|

### 5.3 单点风险与缓解

| 单点 | 影响范围 | 缓解 | 对应 AC |
|------|---------|------|---------|
| **`interfaces` 包本身**（Pure types 破坏）| 整个 v7.0 崩溃 | PR-A 注释声明 + PR-C 完整 Layout guard | AC21 (PR-B) |
| **`TaskSpec.TraceID` 自动生成** | 全链路追踪断裂 | uuid 库 + 浅拷贝不可变 | AC1 |
| **`AppendDissent` 重复添加** | 同 entry 多次添加 | Learn 节点按 hash dedup | AC3 |
| **Dissent top-N 截断边界** | 关键少数派被丢弃 | 默认 3，env flag 可调 | AC3 |

---

## ⑥ 接口 / API 设计

### 6.1 风格：Pure types 模式

- **`interfaces` 包目录结构**（PR-A scope，7 文件）：

```
internal/layers/orchestration/interfaces/
├── doc.go                      (30 LOC)  包文档 + 接口设计动机
├── task_spec.go                (180 LOC) TaskSpec struct + builder + Validate
├── task_report.go              (280 LOC) TaskReport struct + 5 子类型 + builder
├── errors.go                   (80 LOC)  ORCH_* SentinelError（5 基础）
├── task_spec_test.go           (200 LOC) TaskSpec 单测（AC1）
├── task_report_test.go         (280 LOC) TaskReport 单测（AC2）
└── taskcontract_test.go        (150 LOC) Round-trip 测试（AC1, AC2）
```

PR-A **不做**：boundary_test.go（PR-B）/ benchmark_test.go（PR-C）/ security_test.go（PR-C）/ hard_evidence.go（PR-B）/ version_chain.go（PR-C）/ mvp_artifact.go（PR-B）

### 6.2 契约（错误码 + Trace + Cost）

**统一响应结构**：`{TaskSpec, TaskReport}` 双契约 + `ORCH_*` 错误码三元组。

**PR-A 范围 `ORCH_*` SentinelError（5 个）**：

```go
var (
    ErrTaskSpecGoalEmpty      = &SentinelError{Code: "ORCH_TASK_SPEC_GOAL_EMPTY",        Message: "...", Remediation: "set non-empty goal"}
    ErrTaskSpecTraceIDEmpty   = &SentinelError{Code: "ORCH_TASK_SPEC_TRACE_ID_EMPTY",   Message: "...", Remediation: "auto-generate via NewTaskSpec()"}
    ErrTaskReportTraceIDEmpty = &SentinelError{Code: "ORCH_TASK_REPORT_TRACE_ID_EMPTY", Message: "...", Remediation: "pass non-empty traceID"}
    ErrDissentRejection       = &SentinelError{Code: "ORCH_DISSENT_REJECTION",          Message: "...", Remediation: "append entry with non-empty Reason"}
    ErrResourceInvalid        = &SentinelError{Code: "ORCH_RESOURCE_INVALID",           Message: "...", Remediation: "producer must validate positive values"}
)
```

**PR-B 范围**（追加 4 个）：ErrPessimisticTrigger / ErrRuleBasedFallback / ErrHardEvidenceMissing / ErrMigrationAliasDeprecated
**PR-C 范围**（追加 2 个）：ErrCoWVersionChainBroken / ErrSimilarityCheckIntercepted

**TraceID 全链路贯穿**：

```
TaskSpec.TraceID (UUID8 "ts_<uuid8>")
    ↓ ChildDownlink.TraceID (继承父)
    ↓ Worker.Run() 内部传播
    ↓ TaskReport.TraceID (1:1 对账)
    ↓ Learn.Asset.SourceTraceID (沉淀)
    ↓ AuditLog 跨 session 追溯
```

**Cost 度量契约**：`CostBudget` → `Resource.TokensUsed / TimeElapsed / StepCount` 自动计算。

### 6.3 幂等保障

| 操作 | 幂等机制 | 重复执行结果 |
|------|---------|-------------|
| `NewTaskSpec(goal)` | uuid 每次新生成，但 `goal` 字段一致 | TraceID 不同（设计意图：每次构造独立）|
| `WithConstraint(k, v, r)` | 浅拷贝 + append | 多次调用产生多个 Constraint（设计意图：累计约束）|
| `AppendDissent(entry)` | 浅拷贝 + append | 多次调用产生多个 entry（Learn 节点按 hash dedup）|

### 6.4 版本演进路径

| 版本 | 范围 | 实施 PR |
|------|------|--------|
| **v1.0 (本 PR-A)** | L1 + L2 + AC17（6 AC + 7 T 点）| PR-A |
| **v1.1 (PR-B)** | L3 低风险（AC11/AC15）+ L4 基础（AC9/AC10/AC16/AC21/AC22/AC23）| PR-B |
| **v1.2 (PR-C)** | L3 高风险（AC13/AC12/AC14）+ L4 收口（AC6/AC7/AC8/AC18/AC19/AC20）| PR-C |

---

## 附录 A：File Manifest（PR-A scope）

### A.1 新增文件（PR-A 7 文件）

| 文件 | 行数预估 | 内容 | AC |
|------|---------|------|-----|
| `internal/layers/orchestration/interfaces/doc.go` | 30 | 包文档 + Pure types 设计动机 | AC1, AC2 |
| `internal/layers/orchestration/interfaces/task_spec.go` | 180 | TaskSpec struct + 4+2 字段 + builder + Validate | AC1 |
| `internal/layers/orchestration/interfaces/task_report.go` | 280 | TaskReport struct + 5+2 字段 + 5 子类型 + builder | AC2, AC3, AC4, AC5 |
| `internal/layers/orchestration/interfaces/errors.go` | 80 | 5 个 ORCH_* SentinelError | AC1, AC2, AC3, AC5 |
| `internal/layers/orchestration/interfaces/task_spec_test.go` | 200 | TaskSpec 单测 | AC1 |
| `internal/layers/orchestration/interfaces/task_report_test.go` | 280 | TaskReport 单测 + Dissent + Blockage + Resource | AC2, AC3, AC4, AC5 |
| `internal/layers/orchestration/interfaces/taskcontract_test.go` | 150 | Round-trip 测试 | AC1, AC2 |

### A.2 修改文件（PR-A 14 文件）

| 文件 | 改动 | Layer | AC |
|------|------|-------|-----|
| `mups/execute/channel.go` | ChannelRequest 新增 `Spec *TaskSpec`（additive）| L1 | AC1 |
| `mups/execute/exploration.go` | 全量结果 → TaskReport.Dissent | L2 | AC3 |
| `mups/execute/commit.go` | 1 步同步 → TaskReport 出口 | L2 | AC2 |
| `mups/execute/scenario.go` | 并行投票 → TaskReport 出口 | L2 | AC2 |
| `mups/execute/protocol.go` | 多步序列 → TaskReport 出口 | L2 | AC2 |
| `mups/learn/learner.go` | LearnRequest 新增 `Report *TaskReport`（additive）| L1+L2 | AC2, AC3 |
| `workmodel/workitem.go` | 创建路径返回 TaskSpec | L1 | AC1 |
| `workmodel/child_downlink.go` | ChildDownlink 嵌入 TaskSpec 引用 | L1 | AC1 |
| `decisionplanning/decomposer.go` | 分解产出 TaskSpec + Resource 抽取 | L2 | AC1, AC5 |
| `decisionplanning/filter.go` | Dissent 过滤 | L2 | AC3 |
| `sessionorchestrator/orchestrator.go` | RunTurn 主循环消费 TaskSpec + 产出 TaskReport | L1+L2 | AC1, AC2 |
| `sessionorchestrator/turn_orchestrator.go` | 5 节点 observe/plan/execute/verify/learn 全部传 TaskSpec/TaskReport | L1+L2 | AC1, AC2 |
| `wavescheduler/scheduler.go` | dispatchOne 接收 TaskSpec | L1 | AC1 |
| `orchtypes/errors.go` | 新增 5 个 ORCH_* SentinelError（共享）| L4 | AC1, AC2, AC3, AC5 |

### A.3 新增测试文件（PR-A 0 文件 — 测试与源码同包）

PR-A 阶段测试文件与源码同包（`_test.go` 后缀），不创建独立 test 文件。

### A.4 文档修改（PR-A 6 文件）

| 文件 | 改动 | AC |
|------|------|-----|
| `openspec/specs/d7-orchestration/spec.md` | v6.0.x → v7.0 ADDED 3 Requirement（TaskSpec + TaskReport + Dissent/Blockage/Resource）| AC17 |
| `openspec/specs/d7-orchestration/d7-domain.md` | 新增 §8 Layer 架构说明 + §9 interfaces 包章节 | AC17 |
| `openspec/specs/d7-orchestration/a-registry.md` | 新增 A01-A06（6 个新 Activity）| AC17 |
| `openspec/specs/d7-orchestration/f-registry.md` | 新增 F01-F11（11 个新 Function）| AC17 |
| `openspec/specs/d7-orchestration/t-registry.md` | 新增 7 个 T 点 + 4 个 S（D7-S20~S23）| AC17 |
| `openspec/specs/d7-orchestration/span-registry.md` | 新增 5 个 span | AC17 |

### A.5 删除文件（PR-A 0 — 渐进迁移）

PR-A 阶段**不删除**任何文件，所有老类型保留（additive 字段嵌入）。删除工作在 PR-C 收官。

---

## 附录 B：Rollback Plan（PR-A scope）

### B.1 Layer 1 — Feature Flag（PR-B 引入）

PR-A 阶段**不引入** Feature Flag，所有 TaskSpec/TaskReport 调用都是 additive，不影响老路径。

### B.2 Layer 2 — Additive 字段（PR-A 引入）

```go
// PR-A: ChannelRequest 新增字段（additive）
type ChannelRequest struct {
    // ... existing fields preserved
    Spec *interfaces.TaskSpec  // [NEW PR-A] additive
}

// PR-B: ChannelRequest 完整迁移
// - 老字段保留 1 minor 版本（type alias 引导）
// - 调用方全部用 Spec 替换
// - v8.0 移除老字段
```

### B.3 Layer 3 — Code-level 回滚

PR-A 阶段任何问题可通过 `git revert` 单 PR 回滚，**不需要** Feature Flag（因为 additive 不影响老路径）。

### B.4 数据兼容性回滚

- TaskSpec / TaskReport 是新增 type → 回滚后老代码不用 → 兼容
- ChannelRequest / LearnRequest 新增嵌入字段 → 回滚后字段不存在 → 老代码仍编译 → 兼容

### B.5 回滚不恢复的内容（不可逆）

PR-A 阶段**没有不可逆内容**（纯 additive 嵌入 + 新增 type）。Dissent 沉淀至 SkillMemory.SOP 在 PR-A 不发生（仅在字段填充，不写库），因此 SkillMemory 数据兼容性不受影响。

---

## 附录 C：回归风险评估

### C.1 与 v6.0.x baseline 对比

| 指标 | v6.0.x | PR-A 目标 | 风险等级 |
|------|--------|-----------|----------|
| 22/22 orchestration packages `-race` PASS | 100% | 100%（不退化）| **P0 必须** |
| LP-1/LP-2/LP-5 兼容 | 100% | 100%（TaskReport 仅作入参增强）| **P0 必须** |
| Test Coverage（新增 interfaces 包）| N/A | ≥ 80% | P0 必须 |
| Performance P99（TaskSpec 构造）| N/A | < 1ms | P1 |
| Dissent 沉淀 | 0% | 100%（PR-A 仅填充字段，Learn 节点消费在 PR-A 已启用）| P1 |
| Spec 文档同步 | 已存在 | + 6 文件增量 | P0 必须 |

### C.2 高风险改动点

| 改动 | 风险 | 缓解 |
|------|------|------|
| `ChannelRequest.Spec` 字段新增（additive）| 老调用方仍可工作 | PR-A 仅新增字段，不删除老字段 |
| `LearnRequest.Report` 字段新增（additive）| 同上 | 同上 |
| `interfaces` 包 import cycle | 编译失败 | 0 import D7 子包（Pure types）|
| Dissent 字段填充逻辑 | SkillMemory 写入慢 | top-3 截断 + summary hash |
| Resource 字段需新增 metric 桥接 | 增加 instrumentation | 复用 ContextBudget Phase B 现有 metric |

### C.3 回归测试策略

- LP-1/LP-2/LP-5 100% 兼容（PR-A 实施后回归测试集完整保留）
- 22/22 orchestration packages `-race` 不退化
- interfaces 包新增测试覆盖率 ≥ 80%
- TaskSpec / TaskReport 构造 P99 < 1ms（benchmark）

---

## 附录 D：S3 检查清单自检

按 `architecture-design.md §8` S3 完成前：

- [x] `design.md` 包含根因（§① 业务目标）+ 方案（§③ 业务流程）+ 文件清单（附录 A）+ 回归风险（附录 C）
- [x] `dsaft_activities` 已标注（`.openspec.yaml` 列出 6 个 A）
- [x] `design.md` 明确每个 A 的 F 编排关系（§④.2 限界上下文 + `tasks.md` §6 F-T 映射）
- [x] `specs/d7-orchestration/spec.md` 包含所有 Gherkin Scenario（见 `specs/d7-orchestration/spec.md` delta — 3 ADDED Requirement + 12 Scenarios）
- [x] 每个 Requirement 有对应的 T 层注释（spec.md 11 个 `<!-- T: -->` + tasks.md 7 T 点章节）
- [x] 重大决策已记录（proposal.md §3.4 4 个 Decision；design.md §①/§②/§⑥ 设计原则 + §2.3 T 编号重映射说明）
- [x] `detail-design-framework.md` 六段式已完整（①目标 ②原则 ③流程 ④模型 ⑤链路 ⑥接口）
- [ ] Draft PR 已创建（S4 阶段，本 S3 不要求）

---

## 附录 E：下一步

- **S3 完成 → S3-Gate Review**：本 design.md + proposal.md + demand.md + tasks.md + specs/d7-orchestration/spec.md delta 提交 `review-design.md`
- **S4 实施**：按 `tasks.md` 拆 17 个步骤 → +800/-50 LOC + 14 文件修改 + 6 spec 文件同步
- **S5 验收**：22/22 race + LP 100% 兼容 + Coverage ≥ 80% + P99 < 1ms 全绿
- **S6 归档**：通过 `./scripts/verify-archive.sh devrix-d7-taskcontract-unification-pr-a` 12/12 PASS

**关联引用：**
- `demand.md` §2（PR-A 6 AC 范围 + T 编号策略）
- `proposal.md` §3.4（4 个 Decision）+ §6（风险评估）
- `specs/d7-orchestration/spec.md`（3 ADDED Requirements + 12 Gherkin Scenarios）
- `tasks.md`（7 T 点 + F-T 映射 + 17 步骤拆分）
- `.openspec.yaml`（7 T 点 + 6 A + 11 F + 5 span + 4 metric + 5 error code）
- 父 DESIGN：`openspec/archive/2026-06-29-devrix-d7-taskcontract-unification/`（648 行全文版）