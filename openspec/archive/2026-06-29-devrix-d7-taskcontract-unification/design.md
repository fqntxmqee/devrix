# Design: D7 TaskContract 统一 — interfaces 包 + 4-Layer × 3-Phase 落地

**Change ID:** devrix-d7-taskcontract-unification
**Demand ID:** DM-20260629-006
**Status:** S3_Design
**Parent Proposal:** `proposal.md`
**Parent Demand:** `demand.md`
**Template:** `docs/methodology/detail-design-framework.md`（六段式）
**Created:** 2026-06-29

---

## ① 架构目标

### 1.1 业务目标

解决 D7 v6.0.x **"机制层丰富 + 契约层分散 + 防御层缺失 + 治理层散落"** 四维不均衡问题。本 Change 是 D7 v6.0.x 维护阶段（`devrix-d7-dsaft-restructuring` DM-20260629-001 S7_Archived）收官后的 **v7.0 演进第一枪**，落地 23 AC：

| 痛点（来自 6/21 深度 Review 15+ 改进点）| 本 Change 对应 AC |
|----------------------------------------|-------------------|
| Plan / Channel / WorkItem 三处定义不统一（缺 TaskSpec）| AC1 |
| Verdict + Evidence + ExitReason 之外缺 Dissent / Blockage / Resource（缺 TaskReport）| AC2, AC3, AC4, AC5 |
| 资源耗尽时无降级输出 | AC11（Pessimistic Commit）|
| VERDICT 多轮 INDETERMINATE 无强制规则 | AC12（Rule-based Fallback）|
| 子层 Commit 可覆盖父层认知（无 CoW）| AC13（VersionChain）|
| 子层惰性层层转包烧 token | AC14（Similarity Check）|
| Verifier 可被"官话废话"骗过 | AC15（Hard Evidence）|
| 无收敛度量 span | AC6（`convergence.feasible_space_width`）|
| AdaptiveThreshold 留作孤儿代码 | AC7（接入 RunTurn，解 TD-WT-01）|
| Layout guard 未覆盖新包 / 跨域 consumer 不知新字段 / 高风险无灰度 / 新错误未挂 `ORCH_*` | AC8, AC16, AC21, AC22, AC23 |

### 1.2 技术目标（量化指标）

| 指标 | 目标值 | 触发 AC |
|------|-------|---------|
| interfaces 包 0 依赖 D7 子包 | Pure types（防 import cycle）| AC21 |
| TaskSpec / TaskReport 构造 P99 | < 1ms | AC19 |
| VersionChain 查找 | O(1)（hash 索引）| AC19 |
| Similarity Check embedding 命中 | O(1)（缓存）| AC19 |
| 新增包 Coverage | ≥ 80% | AC18 |
| 22/22 orchestration packages `-race` PASS | 100%（不退化）| AC9 |
| LP-1 / LP-2 / LP-5 闭环 | 100% 兼容 | AC10 |
| Fallback 默认策略 | FallbackPessimistic（不无限期挂起）| AC11 |
| Dissent entry 数量 | ≤ 3（top-3 截断）| AC3 |

### 1.3 约束条件

- **SemVer 兼容**：v6.0.x → v7.0.x 是 minor bump（不破坏老调用方）；v8.0 移除 type alias
- **不破坏现有 v6.0.x API**：Plan / ChannelRequest / LearnRequest 通过 type alias 保留 1 minor 版本
- **不破坏 LP-1/LP-2/LP-5 闭环**：5 节点管道完整保留，TaskReport 仅作为 Learn 节点入参增强
- **Pure types 原则**：`interfaces` 包 0 import D7 任何子包（AC21 防循环依赖，Layout guard 守护）
- **Feature Flag 灰度**：AC11（Pessimistic）+ AC13（CoW）必须 env-gated，默认 `disabled`
- **错误码闭合**：所有新错误必须挂 `ORCH_*` SentinelError（Code + Message + Remediation 三元组）
- **演进路径**：v1.0 主体落地 → v1.1 维护期补 Reference Adapter + Operator Runbook → v2.0 规划 `interfaces/v2` 子包

---

## ② 架构原则

### 2.1 设计原则

| 原则 | 落地方式 | 对应 AC |
|------|---------|---------|
| **契约钉死 > 机制灵活** | 单一 `interfaces` 包统一下行/上行 | AC1, AC2 |
| **不可变 API** | `With*` / `AppendDissent` 全部返回新副本（`c := *s` 浅拷贝）| AC1, AC2 |
| **Pure types 防 cycle** | `interfaces` 包 0 import D7 子包 | AC21 |
| **CoW 追加不覆盖** | Dissent 仅追加（`AppendDissent`）；VersionChain 仅追加（GC 压缩）| AC3, AC13 |
| **Hard Evidence 防共谋** | Verifier 必须有 test / log / artifact_hash 之一 | AC15 |
| **Fallback 不无限期挂起** | 资源耗尽必须走 Pessimistic Commit 或 Rule-based | AC11, AC12 |
| **错误码三层闭合** | ORCH_* SentinelError + Code + Message + Remediation | AC23 |
| **高风险变更 env-gated** | AC11 + AC13 必须 Feature Flag，默认 disabled | AC22 |
| **数据所有权原则** | `interfaces` 域 ≠ `escape` 域 ≠ `workmodel` 域；Layout guard 守护边界 | AC8 |
| **面向失败设计** | 5 层 CB + Pessimistic Commit + Similarity Check | AC11, AC12, AC14 |

### 2.2 命名规范

| 类别 | 规范 | 示例 |
|------|------|------|
| **DSAFT S 层** | 在 v6.0.x 6 S 基础上新增 D7-S16/17/18/19（不复用 S1-S6，避免过载）| `D7-S16 L1 Interface Layer` |
| **Activity ID** | `D7-S{16-19}-A{01-11}` | `D7-S16-A01 TaskSpec struct + builder` |
| **Function ID** | `D7-S{16-19}-A{XX}-F{XX}` | `D7-S16-A01-F01 NewTaskSpec` |
| **Test ID** | `D7-S{16-19}-A{XX}-T{XX}` | `D7-S16-A01-T01` |
| **Span Op** | `d7.s{16-19}.<component>.<verb>` | `d7.s18.pessimistic.commit.emit` |
| **Metric Name** | `<component>_<purpose>_<unit>`（snake_case）| `pessimistic_commit_trigger_count` |
| **Error Code** | `ORCH_<DOMAIN>_<CONDITION>`（UPPER_SNAKE）| `ORCH_PESSIMISTIC_TRIGGER` |
| **Type** | 顶层 type 用 PascalCase，子 struct 同 | `TaskSpec`, `DissentEntry`, `MVPArtifact` |
| **Builder** | `New<X>()` 构造器 + `With<X>(v) *X` 不可变 builder | `NewTaskSpec(goal)` / `s.WithConstraint(...)` |

### 2.3 代码风格

- **函数 < 50 行**：`NewTaskSpec` / `Validate` / `With*` 全部 < 20 行
- **文件 < 800 行**：`task_spec.go` ~180 行 / `task_report.go` ~280 行 / 子类型文件 ~30 行
- **异常不过模块边界**：`interfaces` 包定义的所有 error 在 boundary 处 translate 为 `ORCH_*` SentinelError（详见 §⑥.2）
- **统一 TraceID 贯穿**：TaskSpec.TraceID → ChildDownlink.TraceID → TaskReport.TraceID → Learn.Asset.SourceTraceID 全链路一致
- **不可变数据结构**：`c := *s; c.Field = newVal; return &c` 模式（无 `sync.Mutex`，靠 shallow copy）
- **Layout guard 白名单**：`interfaces` 仅允许 9 个白名单包 import（详见 §④.2）

---

## ③ 业务流程

### 3.1 核心用例 — Downlink 端到端（PR-A 落地，AC1）

```
D1 Gateway (feishu/cli)
    ↓ ProcessMessage(sessionID, directive)
D7 SessionOrchestrator.ProcessMessage
    ↓ ① [NEW] interfaces.NewTaskSpec(directive)
    ↓      .WithConstraint("scope_out", ...).WithBudget(...)
    ↓      → AC1 TaskSpec 构造（含 TraceID 自动生成 "ts_<uuid8>"）
    ↓ ② EnsureGoal(sessionID, taskSpec)
    ↓ ③ decompose.TaskGraphSynthesize(taskSpec) — 带 Similarity Check
    ↓      → AC14 相似度 > 80% 拦截 → Refine / 报错
    ↓ ④ WaveScheduler.dispatch(taskGraph)
    ↓      → ChannelRouter.Route(plan) [MODIFIED] ChannelRequest → TaskSpec
    ↓      → 4 Channel 1:1 路由：Commitment/Protocol/Scenario/Exploration
    ↓ ⑤ Channel.Execute(ctx, plan, spec) [MODIFIED] 接收 TaskSpec 而非裸 plan
    ↓ ⑥ WorkerAgent.Run(spec) [MODIFIED] 消费 TaskSpec
    ↓ ⑦ emit("child_task_started", spec.TraceID)
```

**时序标注**：
- ① TaskSpec 构造 < 1ms（构造器纯内存操作）
- ④ ChannelRouter 路由 P99 < 0.1ms（map 索引）
- ⑤ Channel.Execute 走 5 层 CB（P99 < 10ms，不含 worker 执行）

### 3.2 核心用例 — Uplink 端到端（PR-A 落地，AC2-AC5, AC11, AC15）

```
WorkerAgent / ToolRunner
    ↓ return ToolResult
Channel.Execute 出口
    ↓ ① [NEW] interfaces.NewTaskReport(plan.TraceID)
    ↓ ② [NEW] .WithHardEvidence(ev) [AC15] — 若 evidence 为空则拒绝 PASS
    ↓      → 拒绝逻辑：TestCoveragePct==0 && LogExcerpt=="" && ArtifactHash==""
    ↓ ③ [NEW] .AppendDissent(minorityEntry) [AC3] — ExplorationChannel 全量结果
    ↓      → 仅 INDETERMINATE 或 fallback_used=true 时填充
    ↓ ④ .Result / .Evidence / .Resource [AC2/4/5] — 从 worker 返回抽取
    ↓ ↓ 5 层 CB.Evaluate(report) [MODIFIED] 读 report.Blockage 作为升级信号
    ↓ ↓ EscapeEngine.Evaluate(report) [MODIFIED] 读 report.Blockage
    ↓ ⑤ Verifier.Verify(artifact, report) [MODIFIED] 拒绝"空证 PASS"（AC15）
    ↓      → Kind=Pass && !HardEvidence.Verified → 改为 Kind=Partial
    ↓      → + Blockage.RequiredExternal=["hard_evidence"]
    ↓ ⑥ emit("task_completed", report)
SessionOrchestrator
    ↓ ⑦ WaitForCompletion(sessionID)
    ↓ ⑧ [NEW] DecisionPlanning.observe(report) [AC14 Similarity Check 在 decompose 入口]
    ↓ ⑨ [NEW] mups/learn/learner.go :: Learn(report) [MODIFIED] LearnRequest 接收 TaskReport
    ↓      → BayesianUpdate(report.TraceID, report.Result.Confidence)
    ↓      → SkillMemory / FeedbackMemory / ScheduledMemory.Store(asset)
    ↓      → Dissent 沉淀至 LearningSOP（AC3）
    ↓ ⑩ [NEW] if report.FallbackUsed { EscapeEngine.NotifyPessimistic() } [AC11]
```

### 3.3 异常补偿 — 4 类 Fallback 路径

| Fallback 触发条件 | 行为 | 对应 AC |
|------------------|------|---------|
| **资源耗尽**（`tokens_remaining <= cost_budget.min_reserve`）| 走 Pessimistic Commit，输出 MVPArtifact + 风险警告 | AC11 |
| **EscapeForceExit**（CircuitBreaker L1 触发）| 走 Pessimistic Commit | AC11 |
| **VERDICT 连续 ≥ 3 轮 INDETERMINATE** | 走 Rule-based Fallback，候选规则可插拔 | AC12 |
| **Verifier "空证 PASS"**（无 test/log/artifact_hash）| 强制降级 Kind=Partial + Blockage.RequiredExternal | AC15 |
| **Similarity Check > 80%** | 拦截，触发 Refine / 报错 | AC14 |
| **Hard Evidence 误伤**（chat 任务无 test） | Verifier kind-specific 配置：`code` 要 test，chat 要 entity_hash | AC15 缓解 #10 |

**幂等保障**：
- `With*` 浅拷贝 → 同一 spec 多次构造结果一致
- `AppendDissent` 仅追加 → 同 entry 多次添加产生重复 entry（Learn 节点 dedup by hash）
- CoW VersionChain GC 周期 24h → 历史版本可回滚（O(1) hash 索引）

### 3.4 分支处理 — 资源耗尽决策树

```
Channel.Execute 结束
    ↓
[Check] tokens_remaining vs cost_budget.min_reserve
    ↓
    ├── 充足 ─→ 正常返回 TaskReport（Result.Kind = Pass / Partial）
    ↓
    └── 不足 ─→ 走 FallbackPolicy 分支
         ↓
         ├── FallbackPessimistic [default, AC11]
         │     ↓
         │     生成 MVPArtifact {Output, RiskWarnings, Trigger, Traceback}
         │     ↓
         │     TaskReport.MVPArtifact = &mvp; FallbackUsed = true
         │     ↓
         │     Result.Kind = Indeterminate（强制）
         │
         ├── FallbackRuleBased [AC12]
         │     ↓
         │     VERDICT 连续 ≥ 3 轮 INDETERMINATE 才触发
         │     ↓
         │     4 候选规则（most_tests_passed / compiled_clean / min_cost / min_uncertainty）
         │     ↓
         │     env D7_RULE_FALLBACK_STRATEGY 切换（默认 min_uncertainty）
         │     ↓
         │     选中规则 → Kind = Pass（强制，覆盖 INDETERMINATE）
         │
         └── FallbackAbort [v6.0.x 默认，向后兼容]
               ↓
               直接 abort，返回 Result.Kind = Failed
```

---

## ④ 领域模型

### 4.1 聚合根（4 个）

| 聚合根 | 路径 | 职责 | 不可变性 |
|--------|------|------|---------|
| **TaskSpec** | `interfaces/task_spec.go` | 下行传播契约（4+2 字段）| 不可变（`With*` 浅拷贝）|
| **TaskReport** | `interfaces/task_report.go` | 上行反馈契约（5+2 字段 + 防御元数据）| 不可变（`With*` + `AppendDissent`）|
| **WorkItem.VersionChain** | `workmodel/version_chain.go` | CoW 版本链状态 | 不可变追加（GC 周期 24h）|
| **ConvergenceBudget** | `interfaces/convergence_budget.go` | 收敛预算（含 FallbackPolicy 枚举）| 值对象，不可变 |

### 4.2 限界上下文（6 子包 + 1 横切）

```
┌─────────────────────────────────────────────────────────────┐
│                    interfaces (Pure types)                   │
│   Pure types: 0 import D7 子包 / 防循环依赖 / Layout guard 守护 │
└─────────────────────────┬───────────────────────────────────┘
                          │ 仅允许白名单 9 包 import
                          ↓
┌─────────────────────────────────────────────────────────────┐
│  6 限界上下文（v7.0 新增 Layer + D7 6 S 映射）                 │
├─────────────────────────────────────────────────────────────┤
│  L1 接口层 ─ interfaces + workmodel + executionflow + mups  │
│     S1 WorkModel + S4 ExecutionFlow + S6 MUPS              │
├─────────────────────────────────────────────────────────────┤
│  L2 字段语义层 ─ workmodel + mups                           │
│     S1 WorkModel + S6 MUPS                                 │
├─────────────────────────────────────────────────────────────┤
│  L3 防御运行时层 ─ escape + workmodel + executionflow/verify │
│     S2 SessionOrchestrator + S4 Verify + S1 WorkModel      │
├─────────────────────────────────────────────────────────────┤
│  L4 治理横切层 ─ hardening + sessionorchestrator + 跨域      │
│     Hardening 横切 + S2 + D2/D4/D6                         │
└─────────────────────────────────────────────────────────────┘
```

**白名单 import 列表**（9 个包，AC21 强制）：
- `mups/execute`, `mups/learn`, `mups/observe`
- `workmodel`
- `decisionplanning`
- `escape`
- `hardening`
- `sessionorchestrator`
- `executionflow`
- `d7-bootstrap`

### 4.3 领域事件（11 个 span）

| Span | Op | 触发点 | AC |
|------|----|----|-----|
| `interfaces.task_spec.created` | `d7.s16.interfaces.task_spec.created` | `NewTaskSpec()` 出口 | AC1 |
| `interfaces.task_report.created` | `d7.s16.interfaces.task_report.created` | `NewTaskReport()` 出口 | AC2 |
| `taskreport.dissent_recorded` | `d7.s17.taskreport.dissent_recorded` | `AppendDissent()` | AC3 |
| `taskreport.blockage_recorded` | `d7.s17.taskreport.blockage_recorded` | `WithBlockage()` | AC4 |
| `taskreport.resource_recorded` | `d7.s17.taskreport.resource_recorded` | `WithResource()` | AC5 |
| `pessimistic.commit.emit` | `d7.s18.pessimistic.commit.emit` | `Evaluate()` 返回 MVP | AC11 |
| `hard.evidence.reject` | `d7.s18.hard.evidence.reject` | Verifier 拒绝空证 | AC15 |
| `worktree.versionchain.append` | `d7.s18.worktree.versionchain.append` | VersionChain 追加 | AC13 |
| `worktree.versionchain.gc` | `d7.s18.worktree.versionchain.gc` | 24h GC 周期 | AC13 |
| `similarity.check.intercept` | `d7.s18.similarity.check.intercept` | 相似度 > 80% | AC14 |
| `convergence.feasible_space_width` | `d7.s19.convergence.feasible_space_width` | Observe 节点聚合结束 | AC6 |

### 4.4 跨域消费模型（D2 / D4 / D6，AC21）

```
                  ┌─ D2 context engine: CostActual 入 metric
interfaces ←──────┤
                  ├─ D4 multi-agent worker: consume TaskSpec + return TaskReport
                  │
                  └─ D6 evolution observer: 监听 report.TraceID 跨 session 趋势
```

**消费契约**（每个跨域消费点必须写 `boundary_test.go`）：

| 域 | 消费字段 | 行为 | boundary test |
|----|---------|------|---------------|
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

**节点 SLA 承诺**：

| 节点 | 职责 | P99 上限 | 单点风险 |
|------|------|---------|---------|
| `interfaces.NewTaskSpec` | 强类型下行契约 | < 1ms | **interfaces 包崩溃 = 整个 v7.0 崩溃**（Layout guard 守护）|
| `SessionOrchestrator.ProcessMessage` | 主入口 | < 5ms（不含 LLM）| 已有 TurnState 串行化（DM-20260628-003）|
| `decompose.TaskGraphSynthesize` | 任务拆解 | < 50ms | Similarity Check 拦截后可能降深度 |
| `WaveScheduler.dispatch` | DAG 调度 | < 10ms | 5-slot WorkerPool（v6.0.x 已就位）|
| `Channel.Execute` | 4 PlanKind 1:1 路由 | < 100ms | 5 层 CB（v6.0.x 已就位）|

### 5.2 Uplink 链路端到端（详见 §3.2）

```
Worker → Channel.Execute → 5 层 CB → Verifier → SessionOrchestrator → Learn
   ↓ 入口   ↓ ① NewReport    ↓ ② 升级    ↓ ⑤ 空证拒绝  ↓ ⑦ WaitForCompletion  ↓ ⑨ BayesianUpdate
```

**节点 SLA 承诺**：

| 节点 | 职责 | P99 上限 | 单点风险 |
|------|------|---------|---------|
| `interfaces.NewTaskReport` | 强类型上行契约 | < 1ms | 同 NewTaskSpec |
| `Verdict.Aggregate` | 4 态聚合 | < 1ms | 已就位 |
| `5 层 CB.Evaluate` | 异常拦截 | < 5ms | 已就位 |
| `Verifier.Verify` | AC15 硬证据校验 | < 2ms | kind-specific 配置错配 |
| `Learner.Learn` | 5 节点闭环 | < 100ms | LP-1 兼容（AC10）|

### 5.3 CoW VersionChain 链路（PR-C 落地，AC13）

```
WorkItem.Store.Save(workItem, hash)
    ↓
VersionChain[hash] = workItem.State.Snapshot     ← 写时 COW
    ↓
ChildWorkItem = parent.Snapshot() (Read-Only CoW)
    ↓
Child.Commit() → New Hash → VersionChain[newhash] = newState   ← 仅追加
    ↓
oldVersion GC (24h 后台 worker)                  ← hash-only 归档
    ↓
RollbackTo(hash) → O(1) hash 索引查表 → 替换当前 snapshot
```

**节点 SLA 承诺**：

| 节点 | 职责 | P99 上限 | 单点风险 |
|------|------|---------|---------|
| `VersionChain.Append` | 追加 + 父只读 | < 0.5ms | 链长度 > 10 → 触发 GC |
| `VersionChain.RollbackTo` | O(1) hash 索引 | < 0.1ms | 历史版本被 GC → 报错 |
| `VersionChain.GC` | 周期任务（24h）| < 1s | 误 GC 当前版本（不发生，原子操作）|

### 5.4 单点风险与缓解

| 单点 | 影响范围 | 缓解 | 对应 AC |
|------|---------|------|---------|
| **`interfaces` 包本身**（Pure types 破坏）| 整个 v7.0 崩溃 | Layout guard 白名单 + 0 import 守护 | AC21 |
| **`TaskSpec.TraceID` 自动生成** | 全链路追踪断裂 | uuid 库 + 浅拷贝不可变 | AC1 |
| **`VersionChain` GC 误删当前版本** | WorkItem 状态回滚失败 | 原子操作 + hash-only 归档（不删当前指针）| AC13 |
| **`FallbackPolicy` 配置错误**（prod 全 abort）| Fallback 失效 | Feature Flag + RolloutDisable 自动回滚 | AC22 |
| **Hard Evidence kind-specific 误配** | 合法任务被拒 | code vs chat 分开配置 + 灰度期 review | AC15 缓解 |
| **Feature Flag 灰度触发生产事故** | 整个 v7.0 不可用 | RolloutDisable + 灰度 1% → 100% 节奏 | AC22 |

---

## ⑥ 接口 / API 设计

### 6.1 风格：Pure types 模式

- **`interfaces` 包目录结构**（11 个文件）：

```
internal/layers/orchestration/interfaces/
├── doc.go                      (30 LOC)  包文档 + 设计动机
├── task_spec.go                (180 LOC) TaskSpec struct + builder + Validate
├── task_report.go              (280 LOC) TaskReport struct + 5 子类型 + builder
├── convergence_budget.go       (40 LOC)  ConvergenceBudget + FallbackPolicy
├── constraint.go               (30 LOC)  Constraint
├── preference.go               (30 LOC)  Preference
├── cost.go                     (40 LOC)  CostQuota + CostActual
├── result.go                   (50 LOC)  Result + ResultKind 枚举
├── evidence.go                 (50 LOC)  Evidence + TestResult
├── dissent.go                  (60 LOC)  DissentEntry + AppendDissent 填充规则
├── blockage.go                 (50 LOC)  Blockage + 3 类 kind 分类
├── mvp_artifact.go             (40 LOC)  MVPArtifact（AC11 输出）
├── hard_evidence.go            (40 LOC)  HardEvidence（AC15 验证）
├── version_chain.go            (50 LOC)  Hash + VersionChain entry
├── errors.go                   (80 LOC)  ORCH_* SentinelError（AC23）
├── task_spec_test.go           (200 LOC) TaskSpec 单测
├── task_report_test.go         (280 LOC) TaskReport 单测
├── taskcontract_test.go        (150 LOC) Round-trip 测试
├── boundary_test.go            (200 LOC) 跨域消费点 boundary test（AC21）
├── benchmark_test.go           (150 LOC) Performance Budget（AC19）
├── security_test.go            (100 LOC) Classification 标签（AC20）
└── interfaces_test.go          (50 LOC)  Pure types + 0 import 守护
```

- **构造器约定**：`New<X>(required_args) (*X, error)`，必填字段校验失败返回 `ORCH_*` SentinelError
- **Builder 约定**：`func (x *X) With<Field>(v T) *X` 全部返回新副本（不可变）
- **Pure types**：`interfaces` 包自身 0 import D7 子包（防循环依赖）

### 6.2 契约（错误码 + Trace + Cost）

**统一响应结构**：`{TaskSpec, TaskReport}` 双契约 + `ORCH_*` 错误码三元组。

**`ORCH_*` SentinelError（7 个，AC23）**：

```go
var (
    ErrTaskSpecGoalEmpty          = &SentinelError{Code: "ORCH_TASK_SPEC_GOAL_EMPTY",        Message: "...", Remediation: "set non-empty goal"}
    ErrTaskSpecTraceIDEmpty       = &SentinelError{Code: "ORCH_TASK_SPEC_TRACE_ID_EMPTY",   Message: "...", Remediation: "auto-generate via NewTaskSpec()"}
    ErrTaskReportTraceIDEmpty     = &SentinelError{Code: "ORCH_TASK_REPORT_TRACE_ID_EMPTY", Message: "...", Remediation: "pass non-empty traceID"}
    ErrDissentRejection           = &SentinelError{Code: "ORCH_DISSENT_REJECTION",          Message: "...", Remediation: "append entry with non-empty Reason"}
    ErrPessimisticTrigger         = &SentinelError{Code: "ORCH_PESSIMISTIC_TRIGGER",        Message: "...", Remediation: "consumer reads MVPArtifact.RiskWarnings"}
    ErrRuleBasedFallback          = &SentinelError{Code: "ORCH_RULE_BASED_FALLBACK",         Message: "...", Remediation: "consumer reads TaskReport.FallbackUsed"}
    ErrCoWVersionChainBroken      = &SentinelError{Code: "ORCH_COW_VERSION_CHAIN_BROKEN",   Message: "...", Remediation: "trigger VersionChain.GC and rollback"}
    ErrSimilarityCheckIntercepted = &SentinelError{Code: "ORCH_SIMILARITY_INTERCEPTED",      Message: "...", Remediation: "refine directive to <80% similarity"}
    ErrHardEvidenceMissing        = &SentinelError{Code: "ORCH_HARD_EVIDENCE_MISSING",       Message: "...", Remediation: "provide at least one: test, log, artifact_hash"}
)
```

**TraceID 全链路贯穿**：

```
TaskSpec.TraceID (UUID8)
    ↓ ChildDownlink.TraceID (继承父)
    ↓ Worker.Run() 内部传播
    ↓ TaskReport.TraceID (1:1 对账)
    ↓ Learn.Asset.SourceTraceID (沉淀)
    ↓ AuditLog 跨 session 追溯
```

**Cost 度量契约**：`CostBudget` → `CostActual.Delta` 自动计算（负数 = 超出预算）。

### 6.3 幂等保障

| 操作 | 幂等机制 | 重复执行结果 |
|------|---------|-------------|
| `NewTaskSpec(goal)` | uuid 每次新生成，但 `goal` 字段一致 | TraceID 不同（设计意图：每次构造独立）|
| `WithConstraint(k, v, r)` | 浅拷贝 + append | 多次调用产生多个 Constraint（设计意图：累计约束）|
| `AppendDissent(entry)` | 浅拷贝 + append | 多次调用产生多个 entry（Learn 节点按 hash dedup）|
| `VersionChain.Append(delta)` | 仅追加（CoW） | 同 delta 产生同 hash（hash 内容寻址）|
| `VersionChain.RollbackTo(h)` | O(1) hash 索引 | 同 h 多次调用结果一致 |
| `VersionChain.GC()` | hash-only 归档，不删当前指针 | 多次调用 idempotent |
| Pessimistic Commit 触发 | 资源耗尽检测 → 输出 MVP | 重复触发产生相同 MVP（资源状态决定）|

### 6.4 版本演进路径

| 版本 | 范围 | SemVer 路径 |
|------|------|-------------|
| **v1.0** | PR-A + PR-B + PR-C 主体（4-Layer × 3-Phase × 23 AC）| v6.0.x → v7.0.0 minor bump |
| **v1.1** | v7.0.x 维护期：Reference Adapter + Operator Runbook | v7.0.0 → v7.1.0 patch |
| **v2.0** | v8.0 规划 `interfaces/v2` 子包路径 | v7.x → v8.0.0 major bump，type alias 移除 |

---

## 附录 A：File Manifest（新增/修改/删除文件清单）

### A.1 新增文件（PR-A）

| 文件 | 行数预估 | 内容 |
|------|---------|------|
| `internal/layers/orchestration/interfaces/doc.go` | 30 | 包文档 + 接口设计动机 |
| `internal/layers/orchestration/interfaces/task_spec.go` | 180 | TaskSpec struct + builder + Validate |
| `internal/layers/orchestration/interfaces/task_report.go` | 280 | TaskReport struct + 5 子类型 + builder |
| `internal/layers/orchestration/interfaces/errors.go` | 80 | ORCH_* SentinelError（AC23） |
| `internal/layers/orchestration/interfaces/task_spec_test.go` | 200 | TaskSpec 单测（AC18 覆盖） |
| `internal/layers/orchestration/interfaces/task_report_test.go` | 280 | TaskReport 单测 |
| `internal/layers/orchestration/interfaces/taskcontract_test.go` | 150 | Round-trip 测试（spec → report → spec） |

### A.2 修改文件

| 文件 | 改动 | Layer | AC |
|------|------|-------|-----|
| `mups/execute/channel.go` | ChannelRequest → TaskSpec | L1 | AC1 |
| `mups/execute/exploration.go` | 全量结果 → TaskReport.Dissent | L2 | AC3 |
| `mups/execute/commit.go` | 1 步同步 → TaskReport 出口 | L2 | AC2 |
| `mups/execute/scenario.go` | 并行投票 → TaskReport 出口 | L2 | AC2 |
| `mups/execute/protocol.go` | 多步序列 → TaskReport 出口 | L2 | AC2 |
| `mups/learn/learner.go` | LearnRequest → TaskReport | L1+L2 | AC2 |
| `workmodel/workitem.go` | 创建路径统一返回 TaskSpec | L1 | AC1 |
| `workmodel/child_downlink.go` | ChildDownlink 嵌入 TaskSpec 引用 | L1 | AC1 |
| `workmodel/uncertainty.go` | AdaptiveThreshold 接入 RunTurn | L4 | AC7 |
| `workmodel/work_tree.go` | VersionChain []Hash + CoW 接口 | L3 | AC13 |
| `decisionplanning/decomposer.go` | 分解产出 TaskSpec + Similarity Check | L3 | AC1, AC14 |
| `decisionplanning/filter.go` | 过滤 Dissent 后的少数派 | L2 | AC3 |
| `escape/circuit_breaker.go` | 5 层 CB 读 TaskReport.Blockage 升级 | L3 | AC4, AC11 |
| `escape/engine.go` | EscapeDecision 新增 Pessimistic action | L3 | AC11 |
| `sessionorchestrator/orchestrator.go` | RunTurn 主循环消费 TaskSpec + 产出 TaskReport | L1+L2 | AC1, AC2 |
| `sessionorchestrator/turn_orchestrator.go` | 5 层 observe/plan/execute/verify/learn 全部传 TaskSpec/TaskReport | L1+L2+L3 | AC1, AC2, AC11-15 |
| `sessionorchestrator/session_turn_loop.go` | 接口变更（minor）| L1 | AC1 |
| `sessionorchestrator/spans.go` | 新增 11 个 span emit helper | L4 | AC6, AC17 |
| `wavescheduler/scheduler.go` | dispatchOne 接收 TaskSpec | L1 | AC1 |
| `hardening/emitter.go` | Span emit + Coverage 检查 | L4 | AC17, AC18 |
| `hardening/metrics.go` | 8 个新 metric 注册 | L4 | AC4 (Success Metrics) |
| `orchtypes/errors.go` | 新增 ORCH_* SentinelError（共享给 interfaces） | L4 | AC23 |

### A.3 新增测试文件

| 文件 | 行数预估 | 内容 |
|------|---------|------|
| `internal/layers/orchestration/interfaces/boundary_test.go` | 200 | 跨域消费点 boundary test（AC21）|
| `internal/layers/orchestration/interfaces/benchmark_test.go` | 150 | Performance Budget 测试（AC19）|
| `internal/layers/orchestration/interfaces/security_test.go` | 100 | Classification 标签测试（AC20）|
| `internal/layers/orchestration/workmodel/cow_test.go` | 250 | CoW VersionChain 测试（AC13）|
| `internal/layers/orchestration/escape/fallback_test.go` | 200 | Pessimistic + Rule-based Fallback 测试（AC11, AC12）|
| `internal/layers/orchestration/decisionplanning/similarity_test.go` | 180 | Similarity Check 测试（AC14）|

### A.4 文档修改

| 文件 | 改动 | AC |
|------|------|-----|
| `openspec/specs/d7-orchestration/spec.md` | v6.0.x → v7.0（ADDED 4 个 Requirement）| AC17 |
| `openspec/specs/d7-orchestration/d7-domain.md` | 新增 §8 Layer 架构说明 + §9 interfaces 包章节 | AC17 |
| `openspec/specs/d7-orchestration/f-registry.md` | 新增 F01-F34 | AC17 |
| `openspec/specs/d7-orchestration/a-registry.md` | 新增 A01-A19 | AC17 |
| `openspec/specs/d7-orchestration/t-registry.md` | 新增 24+ T | AC17 |
| `openspec/specs/d7-orchestration/span-registry.md` | 新增 11 span | AC17 |
| `scripts/check-orch-taskcontract.sh` | NEW | AC8, AC23 |

### A.5 删除文件（PR-C 收官）

| 文件 | 原因 | AC |
|------|------|-----|
| `internal/layers/orchestration/coordinator/aliases.go` | legacy shim 已由 TaskSpec 替代 | AC16 |
| `internal/layers/orchestration/workmodel/legacy_task_spec.go`（如存在）| 旧裸 struct | AC16 |

---

## 附录 B：Rollback Plan（三层回滚机制）

### B.1 Layer 1 — Feature Flag 灰度（PR-B 引入）

```bash
# AC22 RolloutDisable / Enable
./scripts/devrix.sh rollback-flag pessimistic_commit
./scripts/devrix.sh rollback-flag cow_version_chain

# 全局 flag 默认 disabled
export DEVRIX_FEATURE_PESSIMISTIC_COMMIT=disabled
export DEVRIX_FEATURE_COW_VERSION_CHAIN=disabled
```

### B.2 Layer 2 — Type Alias 兼容（PR-A 引入）

```go
// AC16 type alias 保留 1 minor 版本
package v6types
type TaskSpec = interfaces.TaskSpecV1  // v6 → v7 平滑过渡

// v8.0 移除 alias → 强制 v7 types
```

### B.3 Layer 3 — VersionChain GC（PR-C 引入）

```go
// AC13 24h GC；紧急触发
./scripts/devrix.sh gc-version-chain --force --before=2026-06-29
```

### B.4 紧急回滚触发条件

| 触发条件 | 自动动作 | 手动动作 |
|----------|---------|---------|
| AC11 MVP emit > 100/分钟（异常降级风暴）| Auto RolloutDisable Pessimistic Commit | 检查上游 ChainHash |
| AC13 VersionChain 平均长度 > 50（存储膨胀）| Auto 加速 GC（24h → 1h）| 人工 review CoW 调用点 |
| AC15 Hard Evidence 误伤率 > 20%（合法任务被拒）| Auto 关闭 Verifier.kind-specific 强制 | 调整 kind-specific 配置 |
| AC22 race test fail | Auto 暂停 PR-B/C | 修复 |
| AC9 race test fail | 立即 block 整个 PR | 紧急修复 |

### B.5 数据兼容性回滚

- WorkItem.VersionChain 字段新增（不删除）→ 回滚后字段仍存在但不使用 → 兼容
- TaskSpec / TaskReport 是新增 type → 回滚后老代码不用 → 兼容
- ChannelRequest / LearnRequest 签名变更 → v6.0.x 老调用方在 v7.0 编译失败 → 必须保留 type alias（AC16）→ 兼容

### B.6 回滚不恢复的内容（不可逆）

- Dissent 字段已沉淀至 SkillMemory.SOP（PR-A 落地后即使回滚，Dissent 数据仍在）→ 这是设计意图：Dissent 是事实，不是行为
- CoW VersionChain 历史版本（GC 后即丢失）→ 设计意图
- Feature Flag 触发的 fallback 路径埋点 → 历史 metric 保留

### B.7 S7_Archive 后的清理

- S7_Archived 后 v8.0 移除 type alias → 强制 v7 types
- `coordinator/aliases.go` 删除（PR-C 收官）
- 文档：`openspec/specs/d7-orchestration/spec.md` v8.0 标注 "v7 interfaces deprecated, use v8"

---

## 附录 C：回归风险评估

### C.1 与 v6.0.x baseline 对比

| 指标 | v6.0.x | v7.0 目标 | 风险等级 |
|------|--------|-----------|---------|
| 22/22 orchestration packages `-race` PASS | 100% | 100%（不退化）| **P0 必须** |
| LP-1/LP-2/LP-5 兼容 | 100% | 100% | **P0 必须** |
| Test Coverage（orchestration 总包）| 70%+ | ≥ 80% | P1 |
| 新增 interfaces 包 Coverage | N/A | ≥ 80% | P1 |
| Performance P99（TaskSpec 构造）| N/A | < 1ms | P1 |
| 现有 Verifier 拒绝率 | < 5% | < 7%（Hard Evidence 可能略升）| P2 |
| DecisionPlanning 分解深度 | 平均 3 | 平均 3（Similarity Check 拦截后可能降）| P2 |

### C.2 高风险改动点

| 改动 | 风险 | 缓解 |
|------|------|------|
| `ChannelRequest` → `TaskSpec`（breaking）| 老调用方编译失败 | AC16 type alias 保留 1 minor + deprecation warning |
| `LearnRequest` → `TaskReport`（breaking）| Learn 节点接口变更 | 同上 |
| `WorkItem.VersionChain []Hash` 新增字段 | WorkItem 序列化兼容 | JSON tag `omitempty` + 序列化兼容测试 |
| `escape.CircuitBreaker.Evaluate` 签名加 report | CB 5 层实现需同步改 | 5 层共享 base，签名加可选参数 |
| Feature Flag 灰度失败 | 生产事故 | AC22 自动 RolloutDisable + rollback script |

### C.3 回归测试策略

- LP-1/LP-2/LP-5 100% 兼容（AC10）= regression 测试集完整保留
- 22/22 orchestration packages `-race` 不退化（AC9）
- 跨域消费点 boundary test 全覆盖（AC21）
- 每个 PR 配独立 S3/S4 Gate（AC22 + AC23）

---

## 附录 D：S3 检查清单自检

按 `architecture-design.md §8` S3 完成前：

- [x] `design.md` 包含根因（§① 业务目标）+ 方案（§③ 业务流程）+ 文件清单（附录 A）+ 回归风险（附录 C）
- [x] `dsaft_activities` 已标注（`.openspec.yaml` 列出 19 个 A）
- [x] `design.md` 明确每个 A 的 F 编排关系（§④.2 限界上下文 + `tasks.md` §6 F-T 映射）
- [x] `specs/*/spec.md` 包含所有 Gherkin Scenario（见 `specs/d7-orchestration/spec.md` 27 Scenarios）
- [x] 每个 Requirement 有对应的 T 层注释（spec.md 21 个 `<!-- T: -->` + tasks.md 24 T 点章节）
- [x] 重大决策已记录（proposal.md §3.4 6 个 Decision；design.md §①/§②/§⑥ 设计原则）
- [x] `detail-design-framework.md` 六段式已完整（①目标 ②原则 ③流程 ④模型 ⑤链路 ⑥接口）
- [ ] Draft PR 已创建（S4 阶段，本 S3 不要求）

---

## 附录 E：下一步

- **S3 完成 → S3-Gate Review**：本 design.md + spec.md + tasks.md 提交 `review-design.md`
- **S4 实施**：按 `tasks.md` 拆 PR-A → PR-B → PR-C（4.5 周）
- **S5 验收**：AC9（22/22 race）+ AC10（LP 兼容）+ AC18（Coverage ≥ 80%）+ AC19（P99 < 1ms）全绿
- **S6 归档**：通过 `./scripts/verify-archive.sh devrix-d7-taskcontract-unification` 12/12 PASS

**关联引用**：
- `demand.md` §3（23 AC 表 + 4-Layer × 3-Phase 矩阵）
- `proposal.md` §3.4（6 个 Decision）+ §6（风险评估）
- `specs/d7-orchestration/spec.md`（4 ADDED Requirements + 27 Gherkin Scenarios）
- `tasks.md`（24 T 点 + F-T 映射 + PR-A/B/C 拆分）
- `.openspec.yaml`（24+ T 点 + 19 A + 8 metrics + 11 spans）
- 前置：`openspec/archive/2026-06-29-devrix-d7-dsaft-restructuring/`（v6.0.x 收官）