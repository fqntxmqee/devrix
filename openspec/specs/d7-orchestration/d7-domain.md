# D7 Orchestration Domain

**Domain ID:** D7
**Slug:** `orchestration`
**Type:** Core Domain
**Status:** Active — Canonical S1–S6 + 1 横切（v6.0.0 博弈角色对齐精简，14 S → 6 S + 1 横切；MUPS 5 节点管道 + v5 EscapeEngine 完整保留；v7.0 TaskContract 统一 PR-A 落 `interfaces` 包 + PR-B 落 L3 防御运行时层 `PessimisticCommitGuard` + 4 候选规则 Rule-based Fallback）
**Version:** 2.9.0
**Last Updated:** 2026-07-01 (devrix-d7-physical-layout-alignment DM-20260701-004 PR-4: §North Star 新增 Cross-S Kernel (orchtypes/) 1 行 — types/sentinels/intent primitives single source of truth；PR-1 已落地 ## D7-X Cross-S Kernel (orchtypes/) 6 A + 6 F，本 PR 补全 doc.go package 注释语义对齐 + d7-domain.md §North Star §Cross-S Kernel 行；0 行为 / 0 函数签名变化)
**Depends On:** D1（ingress `ProcessMessage`）、D2（Follower 拆面）、D3（`IGateway`，D7 直调）、D4（Delegate Follower）
**Depended By:** D1（EngineEvent / Flow 展示）、D6（`ValidateOrchestration` advisory）
**Hard Ban:** D1→D2 直连 `IEngine.Process`（DM-007）；D2→D3 import（DM-020）；D4 直 Publish FlowEvent（DM-018）
**Cross-Domain SoT:** `../architecture/cross-domain-boundaries.md` §2.4 / §3.1 · `../d2-context-engine/d7-boundary.md`

---

## North Star

**作为 Orchestration Mediator / Turn Leader / 5 节点管道 Owner，决定做什么、按什么顺序、谁来做，并把执行进度信号化送达 D1；同时通过 Observe → Plan → Execute → Verify → Learn 5 节点管道闭环交付可信结论——不拥有 Session 上下文与 Agent 生命周期。**

| 可验证承诺 | Canonical S | 博弈角色 | ValueFlow Alias（用户感知） |
|-----------|-------------|----------|------------------------------|
| WorkItem 事实与状态机单一权威 + UncertaintyCoord/ReputationEvidence/AdaptivePrior 状态归属 | **D7-S1 WorkModel**（State Authority） | State Authority | Multi-Step Task Coordination |
| 用户消息统一入口 + Turn 主循环 + LLM 调用权 + RunTurn resolve/decompose/await + ResumeSession 3 决策路由 + AutoClose 4 规则 + EscapeEngine 调度 | **D7-S2 SessionOrchestrator**（Mediator + Turn Leader + Error Recovery） | Mediator + Turn Leader + Error Recovery | Turn-Based Conversation |
| 多 Worker 并行 DAG，冲突与上下文隔离 | **D7-S3 WaveScheduler** | Mechanism Designer | Parallel Worktree Execution |
| FlowEvent 聚合 + 4 态 Verdict + VerifyWithRetry + 14 ExitReason + SystemAnomaly 检测 | **D7-S4 ExecutionFlow + Verify** | Costly Signaler + Certifier | Trustworthy Conclusion Delivery |
| ClassifyIntent (Command-first) + UncertaintyReport + IntentQuantize + AnomalyDetector + 4 IntentKind | **D7-S5 DecisionPlanning + Observe** | Information Producer + Quantizer | Intent + Uncertainty Quantization |
| 4 Channel + ChannelRouter + C2/W8 1:1 映射 + 4 LearningClass + 3 通道记忆 + ReputationEvidence Bayesian | **D7-S6 MUPS Pipeline** | Pipeline Coordinator + Memory Curator | Learn from Outcome |
| metric 命名 spec/code 对齐 + 并发硬化 + CircuitBreaker 监控 + ErrorRecoveryPolicy | **Cross-cutting: Hardening**（非 S） | Discipline Keeper | (Discipline Keeper) |
| types / sentinels / intent primitives / Bayesian / Verdict / Observation / UncertaintyCoord / PlanKind / ChannelKind / ArtifactKind / 14 ExitReason — single source of truth for D7 contract | **Cross-S Kernel (orchtypes/)**（S5 intent + S6 types + S1-S6 共享） | (Kernel / 物理即 kernel) | (Single Source of Truth) |

---

## Out of Scope

| 能力 | 归属 | 备注 | Pending Boundary Decision (DM-20260629-001 PR-9) |
|------|------|------|----------------------------------------------|
| IM ingress / 卡片呈现 | D1 | D7 只消费 `InboundMessage`，产出 `EngineEvent` | — |
| Session 上下文 / 工具沙箱 / Persist | D2 | D7 拆面调用 Prepare / ToolRound / Persist | — |
| LLM Gateway 实现 / Breaker 执行 | D3 | D7 **拥有调用决策权**（DM-020），经 `InvokeLLM` | — |
| Worker 执行体 / Agent 生命周期 | D4 | D7 Dispatch → D4 RunAgent | — |
| 结论质量 / 信誉 | D6 | D7 可被 advisory 校验，不阻塞 | — |
| 可观测性基础设施 | D5 | D7 发 span，D5 聚合 | — |
| **ReputationEvidence** | **(pending)** | Bayesian reputation 数据结构在 D7 workmodel 内但跨 Learn/Observe 双向 | `boundary-debt:reputation-evidence-v7.0` |
| **SystemAnomaly** | **(pending)** | 阈值触发逻辑在 D7 hardening/ 但跨 Verify + Observe 双消费 | `boundary-debt:system-anomaly-v7.0` |
| **AdaptivePrior** | **(pending)** | Bayesian 状态在 D7 workmodel 但跨 Orchestrator + Learner 双读写 | `boundary-debt:adaptive-prior-v7.0` |

> **3 项 Pending Boundary Decision（DM-20260629-001 PR-9 T45）**：v6.0.0 临时放在 D7 域（含 orchtypes/ + workmodel/ 子目录），归属待 v7.0 重新评估。常量定义见 `internal/layers/orchestration/orchtypes/boundary_decision.go`。完整 Decision 表见 `design.md §Cross-Domain Boundary`。

---

## DSAFT 资产

### Canonical 价值流 — D7-S1–S6（v6.0.0 博弈角色对齐精简）

| S ID | Scenario | 博弈角色 | Status |
|------|----------|----------|--------|
| **D7-S1** | **WorkModel** | **State Authority** | **✅ IMPLEMENTED** |
| **D7-S2** | **SessionOrchestrator** | **Mediator + Turn Leader + Error Recovery** | **✅ IMPLEMENTED** |
| **D7-S3** | **WaveScheduler** | **Mechanism Designer** | **✅ IMPLEMENTED** |
| **D7-S4** | **ExecutionFlow + Verify** | **Costly Signaler + Certifier** | **✅ IMPLEMENTED** |
| **D7-S5** | **DecisionPlanning + Observe** | **Information Producer + Quantizer** | **✅ IMPLEMENTED** |
| **D7-S6** | **MUPS Pipeline** | **Pipeline Coordinator + Memory Curator** | **✅ IMPLEMENTED** |

### Cross-cutting（横切，不占 S 位）

| 组件 | 博弈角色 | 物理位置 | Status |
|------|----------|----------|--------|
| **Hardening** | **Discipline Keeper** | `orchestration/hardening/`（含 metrics.go + recovery.go subset；`circuit_breaker.go` 留 `escape/`，见 Decision 1） | **✅ IMPLEMENTED** |

### 登记规模（Canonical，v6.0.0 + v2.6.0 同步）

| 层 | 数量 | SoT 文件 |
|----|------|----------|
| A | **57**（49 v6.0.0 + 6 v7.0 TaskContract: D7-S20-A01..A03 + D7-S21-A01..A03 + 2 v7.0 PR-B: D7-S18-A11 + D7-S18-A12） | `a-registry.md` |
| F | **93 IMPLEMENTED**（68 v6.0.0 + 11 v7.0 PR-A + 14 v7.0 PR-B: D7-S18-A11 F01-F05 + D7-S18-A12 F01-F02 + 7 PR-B 增量在 contract/fallback_policy/convergence_budget） | `f-registry.md` |
| T | **246**（230 v6.0.0 + 11 v7.0 PR-A: 9 P0 IMPLEMENTED + 2 spec 同步 + 7 v7.0 PR-B: 6 IMPLEMENTED + 1 PLANNED T05） | `t-registry.md` |
| Span | **32 ops**（26 v6.0.0 + 5 v7.0 PR-A TaskContract + 1 v7.0 PR-B pessimistic_commit_emit）+ 9 sessionSpan attributes | `span-registry.md` |

> **6 S 精简说明（v6.0.0）：** 14 S → 6 S + 1 横切的合并依据见 `dsaft-architecture.md` §14 S 冗余分析。MUPS 5 节点管道（Observe/Plan/Execute/Verify/Learn）+ v5 EscapeEngine 完整保留；A/F 重映射，T 180 → 230（v2.6.0 + devrix-api-error-classification 等增量 50 T）；Span 18 → 26（5 个新 P0/P1 + 3 个内层 span）。

### MUPS 5 节点管道（v6.0.0，挂在 S5/S6 下）

```
D7-S5 Observe 节点 ── UncertaintyReport ──▶ D7-S5 Plan 节点 ── Plan ──▶ D7-S6 Execute
                                                                      │
                                                                      ▼
                                                            D7-S4 Verify 节点
                                                                      │
                                                                      ▼
                                                            D7-S6 Learn 节点
                                                                      │
                                                                      ▼
                    (下轮) D7-S5 Observe ◀── ReputationEvidence ─────┘
                                                                      ▲
                                                  D7-S2 buildObserveRequest（LP-1 闭环）
```

**约束：**

- 节点间依赖契约：每节点的输入必须能在上游节点的输出 Schema 中找到（UncertaintyReport、Plan、Artifact、Verdict、ReputationEvidence）
- 跨域类型上提（PR-C1）：`Artifact` 共享类型在 `internal/shared/types/` 引入，打破 import cycle
- Reverse Traceability（LP-5）：每个 Artifact.SourcePlanID 必须能反向追溯到 Plan；每个 Verdict.SourceArtifactID 必须能反向追溯到 Artifact；Learn 节点的追溯链必须覆盖 Observe

---

## Bootstrap Wire 拓扑 (v2.4.0)

InitOrchestration 是 D7 编排层的单点入口，6 S + 1 横切博弈角色在 `internal/bootstrap/` 包内通过 6 Wire 函数 + 1 BuildOrchestratePath helper 完成装配：

| S 层 | 博弈角色 | Wire 函数 | 物理位置 |
|------|----------|-----------|----------|
| S1 WorkModel | State Authority | 0 wire (inline) | InitOrchestration |
| S2 SessionOrchestrator | Mediator+Turn Leader | `WireTurnInvoker` | `bootstrap/turn_wiring.go` |
| S3 WaveScheduler | Mechanism Designer | `WireWaveScheduler` + `BuildOrchestratePath` | `bootstrap/wire_wave.go` |
| S4 ExecutionFlow+Verify | Costly Signaler+Certifier | `WireExecutionFlow` | `bootstrap/execution_flow.go` |
| S5 DecisionPlanning+Observe | Info Producer+Quantizer | `WireDecisionPlanning` (NEW) | `bootstrap/decision_planning.go` |
| S6 MUPS Pipeline | Pipeline Coord+Memory | `WireMUPSPipeline` (NEW) | `bootstrap/mups_pipeline.go` |
| 横切 Hardening | Discipline Keeper | 0 wire (隐式) | `hardening.SetBridge` 隐式注入 |

**总入口**: `InitOrchestration` (单点) ≤ 200 行 (现状 140 行)，6 S 组合入口清晰。`loadOrchestratorConfigs` + `resolveObsBridge` 2 辅助函数抽离 52 行 config 加载 + 4 行类型断言；3 个内嵌 adapter (`turnOrchExecutor` + `gatewayEventPublisher` + 已在 `turn_adapter.go` 的 `contextEngineAdapter`) 拆到 2 个独立文件 (`adapters.go` + `turn_adapter.go`)；4 个 util 函数 (`boolPtr` + `intPtr` + `strPtr` + `mapBackgroundStatus`) 抽到 `util.go`。

详见 `design.md` §"Bootstrap" v4.4.0 展开 + `t-registry.md` v4.6.0 D7-S2-A51 4 P0 T。

---

## §8 Layer 架构（v7.0 TaskContract, DM-20260629-006/007）

> **Why this section:** v7.0 TaskContract 用 **4-Layer × 3-Phase** 框架描述从 L1 纯类型契约到 L3 防御运行时到 L4 治理横切的完整演化路径——弥补 v6.0 缺契约不缺机制的 gap。
>
> **现状（v7.0 PR-A + PR-B 已落地）：** L1 接口层 + L2 字段语义层 + L3 防御运行时层（PR-B 6/7 P0 T IMPLEMENTED） + L4 spec 同步 9/11 P0 T。
>
> **下一步（PR-C，留待 v7.0 sprint 后半段）：** L3 防御运行时层 1 P0 T (T05 Span/Metric 完整 wire) + CoW VersionChain + Similarity Check。

### 4-Layer 架构定义

| Layer | 主题 | 物理位置 | 落地状态 |
|-------|------|----------|---------|
| **L1 接口层** | **TaskSpec / TaskReport struct 定义** + **NewTaskSpec / NewTaskReport 构造器** + 3 处创建点统一 (Plan/Channel/WorkItem) | `orchestration/interfaces/task_spec.go` + `task_report.go` | ✅ PR-A 落地 (3 P0 T) |
| **L2 字段语义层** | **Dissent** (top-3 + summary hash + Learn 沉淀) + **Blockage** (3 类 kind) + **Resource** (token/time/step) | `orchestration/interfaces/task_report.go` (Dissent+Resource) + `task_spec.go` (Blockage) | ✅ PR-A 落地 (3 P0 T) |
| **L3 防御运行时层** | **Pessimistic Commit**（5 类触发：resource_exhausted / cb_l1 / indeterminate_3x / empty_evidence / manual_abort → MVPArtifact best-effort 输出 + 风险警告）+ **Rule-based Fallback**（4 候选规则：most_tests_passed / compiled_clean / min_cost / min_uncertainty, default min_uncertainty）+ buildChainHash FNV-1a 16-char hex digest；后续 PR-C 接 Hard Evidence + CoW VersionChain + Similarity Check | `orchestration/interfaces/{contracts,fallback_policy,convergence_budget}.go` + `orchestration/escape/fallback.go` + `orchestration/escape/engine.go::NotifyPessimistic` + `orchestration/mups/execute/channel.go::ChannelRouter.SetPessimisticGuard/ApplyPessimisticCommit` + `internal/bootstrap/pessimistic_guard_wire.go` | ✅ **PR-B 落地 6/7 P0 T**（T05 Span/Metric 完整 wire PLANNED 留 PR-C） |
| **L4 治理横切层** | **spec sync** + **Coverage 95%** + **Perf P99 < 1ms** + **Security** (SentinelError) + **Cross-Domain Boundary** (pure types) + **Feature Flag** (always-on PR-A) + **Error Code** (7100-7104) + **Convergence Span** (5 new ops) + **AdaptiveThreshold** (Learn 闭环使用) + **Layout Guard** (interfaces/ 0 import D7) | 各 spec/registry + bootstrap lint | ✅ PR-A 部分（spec sync + 5 SentinelError + 5 span + 0 import lint 落地 4 项） + ⬜ PLANNED PR-B + PR-C 7 项 |

### 3-Phase 实施计划

| Phase | 时间 | 范围 | AC | T | PR |
|-------|------|------|----|----|----|
| **Phase 1 (PR-A)** | 2026-06-29 | L1 接口层 + L2 字段语义层 + L4 部分 spec 同步 | 6 AC | 9 单元/集成 + 2 spec = **11 T** | **#325 (本次)** |
| **Phase 2 (PR-B)** | 2026-06-29 | L3 Pessimistic Commit + Rule-based Fallback + Error Code 7110-7113 | **4 AC** (AC11 Pessimistic Commit + AC12 Rule-based + AC16 Feature Flag + AC18 Observability) | **7 T** (D7-S18-A11-T01..T05 + D7-S18-A12-T01/T02, 6 IMPLEMENTED + 1 PLANNED T05) | **本 PR (#TBD)** |
| **Phase 3 (PR-C)** | 2026-07 中 | L3 CoW VersionChain + Similarity Check + AdaptiveThreshold + Convergence Span + Layout Guard | 9 AC | 23 T (含 4 LP/RACE) | TBD |
| **Total 3-Phase** | 4.5 周 | 4 Layer × 3 Phase 完整闭环 | **23 AC** (PR-A 6 + PR-B 4 + PR-C 9 + L4 spec 同步 4) | **~52 T** (PR-A 11 + PR-B 7 + PR-C 23 + L4 11 共享) | **3 PR** |

### L1 ↔ L2 ↔ L3 ↔ L4 演进依据（博弈论 + 工程论）

| Layer | 博弈论定位 | 工程痛点 | 缺失后果 |
|-------|------------|---------|----------|
| L1 接口层 | **承诺机制**（Commitment Device） | 散落的 wire 数据（Plan.ID 字符串透传、Artifact.ID + Verdict.SourceArtifactID 反向追溯）| "我们为什么知道这个 Artifact 是这个 Plan 的？" —— 不可逆丢失上下文 |
| L2 字段语义层 | **可信信号**（Costly Signal） | 失败 silent swallow（资源耗尽 / 权限不足全部 throw 同一 err）| "为什么这个任务失败？" —— 追溯链断裂 |
| L3 防御运行时层 | **机制设计**（Mechanism Design） | false success commit（Verify 通过但实际侧效果丢失）| "为什么用户看到一个不存在的结论？" |
| L4 治理横切层 | **边界治理**（Cross-Domain Boundary） | interfaces/ 内嵌 D7 子包 → import cycle | "为什么跨域契约无法独立测试？" |

---

## §9 interfaces 包（v7.0 PR-A 落地的纯类型层）

> **Why this section:** TaskContract 统一需要一个独立的纯类型包，避免"interfaces/ 内部 import D7 子包"导致的循环引用 + 边界债务。
>
> **设计原则：** pure types（0 import D7 任何子包，仅依赖 `internal/shared/errors/` 用于 SentinelError）。
>
> **物理位置：** `internal/layers/orchestration/interfaces/`（PR-A 7 NEW + PR-B +3 NEW = 10 文件 + 0 MODIFIED 跨包）

### 包文件清单

| 文件 | 角色 | 行数（约） | 关键导出 | PR |
|------|------|-----------|----------|----|
| `doc.go` | 包文档 | 30+ | Package overview + L1/L2/L3 设计摘要 | PR-A |
| `errors.go` | **9 ORCH_* SentinelError + wrap helper**（PR-A 5 + PR-B 4） | 110+ | `ErrORCHTaskSpecEmpty` (7100) / `ErrORCHTaskSpecChannelUnknown` (7101) / `ErrORCHTaskReportEmpty` (7102) / `ErrORCHTaskReportVerdictEmpty` (7103) / `ErrORCHTaskContractTraceInvalid` (7104) / `ErrORCHPessimisticTriggered` (7110) / `ErrORCHPessimisticMVPEmpty` (7111) / `ErrORCHFallbackRuleInvalid` (7112) / `ErrORCHFallbackAbortTimeout` (7113) | PR-A + PR-B |
| `task_spec.go` | TaskSpec struct + 3 创建点 + 不可变 builder | 250+ | `NewTaskSpec` / `Validate` / `WithPlan` / `WithChannel` / `WithWorkItem` / `WithBlockage` | PR-A |
| `task_report.go` | TaskReport struct + 不可变 builder + AppendDissent + MVPArtifact | 300+ | `NewTaskReport` / `WithVerdict` / `WithResource` / `WithBlockage` / `AppendDissent` / `HashDissentSummary` / `WithMVPArtifact` | PR-A |
| `contracts.go` | **PessimisticCommitGuard interface**（PR-B）+ 5 Trigger* 常量 + FallbackPolicy enum | 110+ | `PessimisticCommitGuard` (Evaluate / ResolveFallback / BuildMVPArtifact) + `FallbackPolicy` (3 路径) + `TriggerResourceExhausted/CircuitBreakerL1/Indeterminate3x/EmptyEvidence/ManualAbort` | **PR-B** |
| `fallback_policy.go` | **4 候选规则 FallbackPolicy helpers**（PR-B） | 80+ | `FallbackPolicyRuleNames` / `ParseFallbackRuleName` / `DefaultFallbackRule` = `"min_uncertainty"` / `Valid` / `ValidNonLegacy` | **PR-B** |
| `convergence_budget.go` | **ConvergenceBudget helpers**（PR-B，与 fallback_policy 共生） | 100+ | `NewConvergenceBudget` / `WithMaxDepth/MaxSteps/MaxTokens` / `Validate` / `RemainingBelowReserve` / `ToFields` | **PR-B** |
| `task_spec_test.go` | TaskSpec 单元测试 | 250+ | 8 子测试 | PR-A |
| `task_report_test.go` | TaskReport 单元测试 | 300+ | 10 子测试 | PR-A |
| `taskcontract_test.go` | 集成测试（round-trip） | 150+ | TestContract_RoundTrip + TestChannelRequest_SpecEmbed + TestLearnRequest_ReportEmbed | PR-A |
| `contracts_test.go` | **PessimisticCommitGuard interface + 9 ORCH_* 错误 helper 单测**（PR-B） | 150+ | TestInterfaceCompiles + 4 NewORCH* helpers + TestTriggerConstants_Stable + TestUniqueCodes (6 tests) | **PR-B** |
| `fallback_policy_test.go` | **4 候选规则 + ClosedSet + Default 稳定**（PR-B） | 120+ | TestValid + TestValidNonLegacy + TestParseFallbackRuleName (9 cases) + TestClosedSet + TestDefaultFallbackRule_Stable (5 tests) | **PR-B** |
| `convergence_budget_test.go` | **NewConvergenceBudget 系列 + Validate + RemainingBelowReserve**（PR-B） | 115+ | TestNewConvergenceBudget + TestWithBuilders + TestValidate (7 cases) + TestRemainingBelowReserve (6 cases) + TestToFields (5 tests) | **PR-B** |

### Additive 嵌入策略（**老路径 0 变更**）

| 现有类型 | 新增字段（optional 指针） | 类型 | PR |
|---------|---------------------------|------|----|
| `mups/execute/channel.go::ChannelRequest` | `Spec *interfaces.TaskSpec` + `pessimisticGuard interfaces.PessimisticCommitGuard` | 嵌入 | PR-A + **PR-B** |
| `mups/learn/asset/asset_builder.go::LearnRequest` | `Report *interfaces.TaskReport` | 嵌入 | PR-A |
| `escape/engine.go::Engine` | `pessimisticGuard interfaces.PessimisticCommitGuard` + `NotifyPessimistic` 方法 | 字段 + 方法 | **PR-B** |

**关键不变量：**
- 老路径调用方（`Channel.Execute(req)` / `AssetBuilder.Build(req)`）**0 变更**——只看老字段。
- 新调用方可同时设置 `req.Spec` 或 `req.Report`，下游 ConsumeNode 自动捕获并落 `interfaces` 包的 span。
- Additive 验证：`grep -n 'req\.Spec' mups/execute/channel.go` 之外的位置不应有新调用——确保**不是 PR-A 强加的迁移**。

### 4 个不变式（PR-A 必保）+ 1 个新增（PR-B）

| Invariant | 物理约束 | 验证方式 |
|-----------|---------|---------|
| **IV-1:** `interfaces` 包 0 import D7 任何子包 | `go vet ./internal/layers/orchestration/interfaces/` 无 output + 人工 grep 0 结果 | scripts/ci-lint-invariant/TestInterfacesZeroImportD7 |
| **IV-2:** TaskSpec / TaskReport 不可变（无 setter） | `go vet` + 单元测试覆盖 With* 浅拷贝 | TestTaskSpec_Immutable + TestTaskReport_Immutable |
| **IV-3:** `AppendDissent` top-3 silent truncate | 单元测试覆盖"添加第 4 个 Dissent 不改变切片" | TestTaskReport_AppendDissent_Truncation |
| **IV-4:** TraceID 格式 `ts_<8 hex>` | NewTaskSpec/NewTaskReport fail-fast 校验 + 单元测试覆盖 | TestNewTaskSpec_TraceIDFormat 等 5 个 |
| **IV-5（PR-B 新增）:** `PessimisticCommitGuard` interface 必须 nil-safe / disabled-safe | guard==nil 或 guard.Enabled=false → Evaluate 直接 return (true, "", nil) | TestDefaultPessimisticCommitGuard_NilReceiver + TestDisabled_NilReport + TestEnabled_HappyPath 等 14 tests |

### 已落地的 SentinelError + Code 范围

| 包内 Err 常量 | Code | 触发条件 | 返回方式 | PR |
|----------------|------|---------|----------|----|
| `ErrORCHTaskSpecEmpty` | 7100 | `NewTaskSpec(sessionID="", …)` | `sharederrors.WithCode` | PR-A |
| `ErrORCHTaskSpecChannelUnknown` | 7101 | `Spec.Channel.Kind == ""` 或不在 `sync/async/probe/explore` | `sharederrors.WithCode` | PR-A |
| `ErrORCHTaskReportEmpty` | 7102 | `NewTaskReport(sessionID="", …)` | `sharederrors.WithCode` | PR-A |
| `ErrORCHTaskReportVerdictEmpty` | 7103 | `Report.Verdict.Kind == ""` 或不在 4 VerdictKind | `sharederrors.WithCode` | PR-A |
| `ErrORCHTaskContractTraceInvalid` | 7104 | `TraceID == ""` 或格式 `≠ ts_<8 hex>` | `sharederrors.WithCode` | PR-A |
| `ErrORCHPessimisticTriggered` | 7110 | Evaluate 返回 blocked（5 类触发条件之一命中） | `sharederrors.WithCode` | **PR-B** |
| `ErrORCHPessimisticMVPEmpty` | 7111 | `BuildMVPArtifact` 输出空（producer 须保证 Output 非空） | `sharederrors.WithCode` | **PR-B** |
| `ErrORCHFallbackRuleInvalid` | 7112 | env `D7_RULE_FALLBACK_STRATEGY` 不在 4 候选规则内 | `sharederrors.WithCode` | **PR-B** |
| `ErrORCHFallbackAbortTimeout` | 7113 | FallbackAbort 超时（producer 须 respect `time_budget_ms`） | `sharederrors.WithCode` | **PR-B** |

后续 PR-C 增 `ErrORCH*` (Code 7120+) 在同 errors.go 续编，不另开文件。

---

## 规格文档索引

| 文档 | 用途 |
|------|------|
| `spec.md` | Gherkin 验收规格 |
| `terminal-state-guide.md` | 终态流程、IntentKind 四链、A→F 编排树、跨域时序、14 ExitReason、Auto-Close 4 规则、ResumeSession 3 决策路由 |
| `observability-guide.md` | Span↔T、Trace 树、FastPath SLA、P0 Runbook、5 节点 Trace 树、9 sessionSpan attributes |
| `design.md` | 六段式详细设计（Wave、Hub、PlanMode、5 节点管道等） |
| `d7-requirements-clarifications.md` | Review R1/R2 完整澄清（历史归档） |
| `dsaft-architecture.md` | Stub — DSAFT 五层计数 |
| `a-registry.md` / `f-registry.md` / `t-registry.md` | A/F/T 登记 SoT |
| `span-registry.md` | Span operation 登记 SoT（含 MUPS 5 节点 + 9 sessionSpan attributes） |
| `layer-delta.md` | V1→V5 演进 Delta（含 MUPS 5 节点管道 IMPLEMENTED 段） |
| `workitem-pipeline-unification-design.md` | WorkItem × MUPS Pipeline 统一（Turn Loop + SpawnPolicy） |
| `workitem-context-graph-design.md` | WorkItem × ContextGraph 分层透传（ContextScope + Link/Bubble 规则） |
| `../../tech-debt/worktree-v2-deferred.md` | WorkTree v2.1+ 技术债务（TD-WT-01..06） |
| `../architecture/code-layout.md` §4.2 | scenario-slug 物理路径 |

---

## 修订记录

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-06-16 | 初版：薄领域 SoT；厚版迁至 `d7-requirements-clarifications.md`；对齐 D1 `*-domain.md` 模式 |
| 1.1.0 | 2026-06-18 | DM-20260617-009 闭环：WorkItem/WorkTree 写入 North Star；RunTurn resolve/decompose/await；tech-debt 索引 |
| 1.2.0 | 2026-06-25 | MUPS v4.3 5 节点管道 + v5 EscapeEngine 升格：14 S 层（D7-S1~S14）全部 IMPLEMENTED；North Star 新增 8 行可验证承诺（S6-S14）；DSAFT 资产规模 24 A → 56 A，66 T → 180 T，9 Span → 18 Span；新增 §MUPS 5 节点管道依赖契约；跨域类型上提 PR-C1；Reverse Traceability LP-5 |
| **2.0.0** | **2026-06-26** | **6 S 博弈角色对齐精简**（DM-20260626-001）：(1) 14 S → **6 S + 1 横切**（State Authority / Mediator+Turn Leader+Error Recovery / Mechanism Designer / Costly Signaler+Certifier / Information Producer+Quantizer / Pipeline Coordinator+Memory Curator / 横切 Discipline Keeper）；(2) North Star 14 行 → 6 行（每个 S 含全部合并角色）；(3) DSAFT 资产规模 56 A → **49 A**（S1:4 · S2:7 · S3:4 · S4:9 · S5:8 · S6:15 + Hardening:2），75 F → **68 F**，180 T 持平（重归类不删），18 Span → **23 Span**（**+5 新 P0/P1**：channel.route / memory.persist / system.anomaly_detect / taskgraph.synthesize / executor.select）；(4) MUPS 5 节点管道挂载调整（Observe/Plan 归 S5，Execute/Learn 归 S6，Verify 归 S4，ResumeSession 归 S2）；(5) 新增 §Cross-cutting Hardening 表，定位明确为非 S；(6) 14 S 冗余分析详见 `dsaft-architecture.md` |
| **2.1.0** | **2026-06-26** | **Hardening 横切包物理落地**（DM-20260626-003）：(1) `orchestration/hardening/` 目录新建，5 .go 文件（doc.go + metrics.go + metrics_test.go + recovery.go + recovery_test.go）；(2) `sessionorchestrator/metrics.go` + `turn/recovery.go` subset（4 纯函数 + 1 const）git mv 迁 hardening/；(3) `escape/circuit_breaker.go` **留 escape/**（Decision 1：V5 EscapeEngine 核心机制）；(4) `turn/recovery.go` KEEP：receiver methods（compressMessagesForRecovery + invokeStreamWithRecovery）+ partialStreamEmit + emitStreamRecoveryTombstones（Decision 2：紧耦合 *DefaultOrchestrator）；(5) D7-S7 4 新 P0 T IMPLEMENTED（D7-S7-A01-T01..T04）→ 域 t-registry v4.3.0 / 根 v5.3.0；(6) 23/23 orchestration packages go test -race PASS |
| **2.2.0** | **2026-06-26** | **turn/ → sessionorchestrator/ 整包物理合并**（DM-20260626-004）：(1) `orchestration/turn/` 整包 git mv → `orchestration/sessionorchestrator/`（24 .go 文件，6467 行），5 同名冲突文件加 turn_ 前缀（contracts/doc/orchestrator/orchestrator_test/tracing）；(2) 跨包 import cycle 打破：`LLMInvoker + LLMInvokeRequest + ToolSchema` 上提至 `orchtypes/llm_invoker.go`，sessionorchestrator 用 type alias；(3) 14 importer 文件 import path + identifier 全替换（10 bootstrap + 2 decisionplanning + 2 sessionorchestrator）；(4) `sessionorchestrator/{exit_reason,verdict_to_exit_reason}.go` 临时留 sessionorchestrator/（follow-up #4 promote）；(5) D7-S2-A50 4 新 P0 T IMPLEMENTED（D7-S2-A50-T01..T04）→ 域 t-registry v4.4.0 / 根 v5.4.0；(6) **0 函数签名变化**（pure physical migration + import path replace），(7) `hardening/` + `escape/circuit_breaker.go` + `sessionorchestrator/autoclose.go` 0 变更，22/22 orchestration packages go test -race PASS |
| **2.3.0** | **2026-06-26** | **verify-promotion 包归属迁移**（DM-20260626-005）：(1) DM-20260626-004 临时留存的 `sessionorchestrator/{exit_reason.go (72 行) + verdict_to_exit_reason.go (49 行) + verdict_to_exit_reason_test.go (97 行)}` 3 文件 (218 行) git mv → `executionflow/verify/`；(2) 3 文件 `package sessionorchestrator` → `package verify` 改名 + sessionorchestrator/turn_orchestrator.go 11 处 `ExitReason*` 跨包引用替换为 `verify.ExitReason*`（state 字段 + 6 常量 + 2 函数参数 + 1 type assertion）+ turn_orchestrator_test.go 2 处 `ExitReasonNatural` → `verify.ExitReasonNatural`；(3) S4 ExecutionFlow + Verify (Costly Signaler + Certifier) 角色的可验证承诺（14 ExitReason + VerdictToExitReason 4 态映射）在 spec/code 完全对齐；(4) 跨包 DAG 单向（sessionorchestrator → verify，无反向 import cycle 风险）；(5) D7-S4-A50 4 新 P0 T PLANNED（D7-S4-A50-T01..T04）→ 域 t-registry v4.5.0 / 根 v5.5.0；(6) **0 函数签名变化**（pure physical migration，安全网与 DM-20260626-004 一致），14 ExitReason 字符串值 + 6 测试函数测试矩阵全不变；(7) `hardening/` + `escape/circuit_breaker.go` + `sessionorchestrator/autoclose.go` git diff 0 变化（baseline stability），22/22 orchestration packages go test -race PASS |
| **2.4.0** | **2026-06-26** | **Bootstrap Wire 拓扑收口**（DM-20260626-007 / devrix-d7-6s-bootstrap-slim）：(1) `internal/bootstrap/wire_coordinator.go` InitOrchestration (275 → 215 行) 内部 4 util 函数 (`boolPtr` / `intPtr` / `strPtr` / `mapBackgroundStatus`) 抽至 `internal/bootstrap/util.go`（30 行）；(2) 2 内嵌 adapter 类型 (`turnOrchExecutor` / `gatewayEventPublisher`) 抽至 `internal/bootstrap/adapters.go`（48 行）；(3) 6 S × WireFunc 命名一致：新增 `WireDecisionPlanning` (S5 / `decision_planning.go` 16 行) + `WireMUPSPipeline` + `MUPSPipelinesDeps` (S6 / `mups_pipeline.go` 75 行) 包装；3rd adapter `contextEngineAdapter` 已在 `turn_adapter.go`（502 行）独立；(4) `loadOrchestratorConfigs` + `resolveObsBridge` 辅助函数抽离 52 行 config 加载 + 4 行类型断言；(5) InitOrchestration 函数体 275 → 140 行（≤ 200 目标达成）；(6) `cmd/devrix` + `cmd/obs-verify` + `tests/testutil/d7_stack.go` 0 变化（调用方 0 变化），`hardening/` + `escape/circuit_breaker.go` + `sessionorchestrator/autoclose.go` git diff 0 变化（baseline stability）；(7) D7-S2-A51 4 新 P0 T IMPLEMENTED（D7-S2-A51-T01..T04）→ 域 t-registry v4.6.0 / 根 v5.6.0；(8) **0 函数签名变化**（pure physical refactor，InitOrchestration 外部接口 100% 不变）；(9) 4 PR 落地（PR-1 #225 util + PR-2 #226 adapters + PR-3 #227 S5+S6 wire + PR-4 #228 config+obsBridge+docs），22/22 orchestration packages go test -race PASS；(10) v6.0.0 follow-up 序列收官（5/6 S7_Archived + 1/6 S1_Cancelled + 1/1 S7_Archived = #007 bootstrap-slim） |
| **2.5.0** | **2026-06-26** | **WorkItem ContextGraph**（DM-20260626-020 / devrix-d7-workitem-context-graph PR #244）：(1) WorkTree 正交维度 ContextGraph — `ContextScope` + `ContextLinkKind` / `ContextBubbleKind` + sibling taxonomy R1–R6；(2) `ContextLinkEvaluator` CL0–CL8 + `ContextBubbleEvaluator` CB0–CB6；(3) `ApplyPipelineDecide` 接入 Item Pipeline（Bubble → Links → Spawn）；(4) BlockedBy → Wave `ContextUpstream`；(5) `/task context show` + `ContextResolveHint` 运维审计；(6) 设计 SoT `workitem-context-graph-design.md` v0.3.0 |
| **2.5.1** | **2026-06-26** | **默认开启 WorkItem Pipeline + ContextGraph**（PR #246）：移除 `D7_WORKITEM_PIPELINE` / `D7_WORKITEM_CONTEXT_GRAPH` 环境变量门控；`FeatureWorkItemPipelineEnabled` / `FeatureWorkItemContextGraphEnabled` 恒为 true；`EnsureGoal` 同步绑定 ContextScope |
| **2.6.0** | **2026-06-29** | **devrix-d7-dsaft-restructuring DM-20260629-001 S7_Archived**：(1) §Out of Scope 3 boundary debt Decision 标注（ReputationEvidence / SystemAnomaly / AdaptivePrior）+ 治理常量 `orchtypes/boundary_decision.go`；(2) §Cross-cutting Hardening 物理位置记录（`orchestration/hardening/` 5 .go + `escape/circuit_breaker.go` 留 escape/）；(3) §DSAFT 资产 T 186 → **230**（devrix-d7-dsaft-restructuring + devrix-api-error-classification 等增量）；(4) §登记规模 Span 18 → 26（5 个新 P0/P1 span：channel.route / memory.persist / system.anomaly_detect / taskgraph.synthesize / executor.select）；(5) `t-registry.md` v4.12.0 Span Evidence 列填充 **94% 覆盖率**（235/248 T）+ `observability-guide.md` v2.2.0 §8.1 T-Without-Span Tracker；(6) god function 治理：`turn_orchestrator.go` 1551 行拆 4 文件（turn_orchestrator / turn_loop / turn_invoke / turn_recovery），36 T 重映射；(7) WorkTree 上行反馈治理：`RollupReport` typed struct（5 字段聚合） + `sessionRootGoal` 确定性排序 + 3 governance T（D7-S15-A50..A55）；(8) 10 PR / 55 T / 15 G 全部 PASS，22/22 orchestration packages -race PASS，v6.0.x 维护阶段收官, v7.0 演进起点 |
| **2.7.0** | **2026-06-29** | **v7.0 TaskContract 统一 PR-A 部分（DM-20260629-007 S4 part）**：(1) **新增 §8 Layer 架构**（4-Layer × 3-Phase：L1 接口层 + L2 字段语义层 + L3 防御运行时层（PR-B/C）+ L4 治理横切层）+ 3-Phase 实施计划表（PR-A 6 AC + 11 T / PR-B 8 AC + 18 T / PR-C 9 AC + 23 T）+ L1-L4 演进依据（博弈论 + 工程论双视角）；(2) **新增 §9 interfaces 包**章节（pure types 原则 + 7 NEW 文件清单 + Additive 嵌入策略 ChannelRequest.Spec/LearnRequest.Report + 4 个不变式 IV-1..IV-4 + 5 ORCH_* SentinelError 7100-7104）；(3) **§DSAFT 资产规模更新**：A 49 → **55**（+6 v7.0 A），F 75 → **86**（+11 v7.0 IMPLEMENTED + 2 PLANNED），T 230 → **241**（+11 v7.0 T 9 IMPLEMENTED + 2 spec 同步），Span 26 → **31 ops**（+5 v7.0 TaskContract span）；(4) PR-A 9/11 P0 T IMPLEMENTED：`interfaces` 包 7 NEW + 2 MODIFIED 0 race + 95% coverage（详见 `a-registry.md` §D7-S20/S21 + `f-registry.md` §D7-S20-A01-F01 等 + `t-registry.md` §D7-S20/S21 + `span-registry.md` §D7-S20/S21 + `spec.md` ADDED 3 Requirement）|
| **2.8.0** | **2026-06-29** | **v7.0 TaskContract 统一 PR-B L3 防御运行时层（DM-20260629-008 S4 part）**：(1) **新增 D7-S18 Pessimistic Commit + Rule-based Fallback 段**（2 A + 7 F + 7 T + 1 Span = `a-registry.md` §D7-S18-A11/A12 + `f-registry.md` §D7-S18-A11/F01-F05 + §D7-S18-A12/F01-F02 + `t-registry.md` §D7-S18-A11-T01..T05 + §D7-S18-A12-T01/T02 + `span-registry.md` §D7-S18 pessimistic_commit_emit）；(2) **新增 §8.5 L3 防御运行时层 PR-B 落地章节**：PessimisticCommitGuard interface + 5 类触发条件（resource_exhausted / cb_l1 / indeterminate_3x / empty_evidence / manual_abort）+ 3 FallbackPolicy 路径（Pessimistic / RuleBased / Abort）+ 4 候选规则（most_tests_passed / compiled_clean / min_cost / min_uncertainty, default min_uncertainty）+ buildChainHash FNV-1a 16-char hex；(3) **§DSAFT 资产规模更新**：A 55 → **57**（+2 v7.0 PR-B: D7-S18-A11 EvaluatePessimistic + D7-S18-A12 ResolveRuleFallback），F 86 → **93 IMPLEMENTED**（+7 v7.0 PR-B），T 241 → **246**（+7 v7.0 PR-B: 6 IMPLEMENTED + 1 PLANNED T05 留 PR-C），Span 31 → **32 ops**（+1 v7.0 PR-B pessimistic_commit_emit）；(4) **新增 4 ORCH_* SentinelError (7110-7113)**：ORCH_PESSIMISTIC_TRIGGERED + ORCH_PESSIMISTIC_MVP_EMPTY + ORCH_FALLBACK_RULE_INVALID + ORCH_FALLBACK_ABORT_TIMEOUT；(5) **新增 3 interfaces/ 文件**（contracts.go PessimisticCommitGuard interface + fallback_policy.go FallbackPolicyRuleNames + convergence_budget.go NewConvergenceBudget 系列）+ **escape/fallback.go** (~310 LOC) + **bootstrap/pessimistic_guard_wire.go** (~75 LOC) + **engine.go +NotifyPessimistic** (5 层 fail-safe) + **mups/execute/channel.go +ChannelRouter.SetPessimisticGuard/ApplyPessimisticCommit**；(6) **6/7 T 点 IMPLEMENTED**（T05 Span/Metric 完整 wire 留 PR-C，本 PR 仅 slog.Info 占位）；(7) **Feature Flag D7_PESSIMISTIC_COMMIT_ENABLED 默认 disabled, 0 行为变更**（所有方法 nil/disabled 守门 no-op）；(8) interfaces coverage **96.9%** / escape coverage **85.0%** / 22/22 orchestration packages go test -race PASS |
| **2.9.0** | **2026-07-01** | **D7 Physical Layout Alignment（DM-20260701-004）PR-4**：§North Star 新增 **Cross-S Kernel (orchtypes/)** 1 行 — `types / sentinels / intent primitives / Bayesian / Verdict / Observation / UncertaintyCoord / PlanKind / ChannelKind / ArtifactKind / 14 ExitReason — single source of truth for D7 contract`，与已落地的 `a-registry.md` `## D7-X Cross-S Kernel（orchtypes/）段` 6 A + `f-registry.md` `## D7-X Cross-S Kernel F 段` 6 F + `code-layout.md` §4.2 Cross-S Kernel 行 + `orchtypes/doc.go` package 注释补全 5 处规范对齐（PR-1 已做完大部分，本 PR 收尾 d7-domain.md North Star 最后一行）；**0 行为 / 0 函数签名变化**（purely doc-only 收尾）。与 PR-3 同源：`plan/` S5 doc-only dual registration 配套收口。 |
