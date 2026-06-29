# D7 Orchestration Domain

**Domain ID:** D7
**Slug:** `orchestration`
**Type:** Core Domain
**Status:** Active — Canonical S1–S6 + 1 横切（v6.0.0 博弈角色对齐精简，14 S → 6 S + 1 横切；MUPS 5 节点管道 + v5 EscapeEngine 完整保留）
**Version:** 2.6.0
**Last Updated:** 2026-06-29 (devrix-d7-dsaft-restructuring DM-20260629-001 S7_Archived: 10 PR / 55 T / 15 G, Span Evidence 94% coverage)
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
| A | **49**（S1:4 · S2:7 · S3:4 · S4:9 · S5:8 · S6:15 + Hardening:2）| `a-registry.md` |
| F | **68**（deprecated 2 + canonical 66） | `f-registry.md` |
| T | **230**（v4.9.1 2026-06-28 闭环，6 S 重归类不删测试点，devrix-api-error-classification 2 新 T PLANNED→IMPLEMENTED） | `t-registry.md` |
| Span | **26 ops**（18 旧 + 5 新 P0/P1 + 3 内层 observability span）+ 9 sessionSpan attributes | `span-registry.md` |

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
