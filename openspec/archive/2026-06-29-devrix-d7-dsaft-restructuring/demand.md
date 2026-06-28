# Demand: devrix-d7-dsaft-restructuring (DM-20260629-001)

**Demand ID:** DM-20260629-001
**Status:** S1_Demand
**Priority:** P0 (深度架构重构)
**Created:** 2026-06-29
**Change ID:** devrix-d7-dsaft-restructuring
**Triggered By:** D7 域整体 DSAFT 方法论 Review（2026-06-28 会话）+ 后续深度盘点（2026-06-29 双 Agent 验证）
**Related:**
- `devrix-d7-six-s-simplification` (DM-20260626-001) — v6.0.0 6 S + 1 横切
- `devrix-d7-mups-v4-5node-coverage-orchestration` (DM-20260625-019) — 5-node Span + 目录治理
- `docs/methodology/dsaft-methodology.md` v4.0.0 — 6 原则
- `docs/methodology/dsaft-refactoring-playbook.md` v1.0.0 — 4 轴 / 6 阶段

---

## §1 背景

D7 v6.0.0（2026-06-26 S7_Archived）完成 4 轮物理迁移实现 0 函数签名变化。但 2026-06-28 的 DSAFT Review + 2026-06-29 双 Agent 全量盘点暴露 **6 类深度架构债**：

### 1.1 S 层语义偏离 DSAFT 原则 1（P0）

6 S 全部是 Go 包名而非用户价值流（详见 `pipeline-architecture.md v1.1.0` §2.1 仍按 13 S 描述）。违反 DSAFT 原则 1 "S 层回答用户要达成什么，不是代码在哪个包"。

**Top 3 真实问题**：
1. **S6 = 15 A 散在 5 物理位置**：`mups/execute/` + `mups/learn/` + `escape/` + `sessionorchestrator/{autoclose,observe_request,resume}.go` + `executionflow/verify/`。"Package name = S name" 完全打破——S6 不存在"mups"主入口。
2. **S5 = 8 A 散在 3 物理位置**：`decisionplanning/` + `orchtypes/{observation,uncertainty_*,intent_quantizer,anomaly_detector}.go` + `plan/`。S5 名称"DecisionPlanning+Observe"暗示两个 S 合成，但代码按 Observe 跨包拆。
3. **S4 = 9 A 散在 5 子包**：`executionflow/{hub,bridge,imsink,workplan,verify}/`。Hardening `circuit_breaker.go` 仍留 `escape/`。

### 1.2 turn_orchestrator.go God Function（P0）

```
$ wc -l internal/layers/orchestration/sessionorchestrator/turn_orchestrator.go
1551 turn_orchestrator.go  ← 超 800 行硬上限近 2 倍
```

36 T 全部挂 D7-S2-A06 RunTurnLoop **单一函数**——典型 god function。根因：DM-20260626-004 turn→sessionorchestrator 整包合并后，turn_orchestrator.go 吸收了原 turn/ 子树的多个文件但未拆分。

### 1.3 Registry 路径漂移（P0）

| F ID | 错误路径 | 实际路径 |
|---|---|---|
| `D7-S1-A02-F01..F06` | `contextengine/tasks/task_manager.go` | `workmodel/task_manager.go`（D2 thin 后 tasks 已迁 workmodel）|
| `D7-S2-A01-F04` | `sessionorchestrator/orchestrator.go + EventPublisher` | `turn_orchestrator.go`（orchestrator.go 仅 0 引用）|
| `D7-S2-A01-F03` | `orchestrate` | `orchestratePath`（`orchestrate_path.go`，命名漂移）|
| `D7-S5-A01-F01..F03` | `decisionplanning/classifier.go + classifier_fallback.go` | `classifier_fallback.go`（F02/F03 路径描述与实际不符）|
| t-registry `D7-S4-T01..T09` | `orchestration/workplan/` + `orchestration/imsink/` | `orchestration/executionflow/workplan/` + `executionflow/imsink/` |

### 1.4 ~80 LOC 死代码 + 老链路（P0）

**双 Agent grep 验证 0 外部调用者的死代码**：

| 文件:行 | 死符号 | LOC |
|---|---|---|
| `orchtypes/uncertainty_coord.go:180-183` | `ErrInvalidVerdictKind` alias | 4 |
| `orchtypes/config.go:13-20` | `LLMFallback` field | 1 |
| `orchtypes/config.go:23-29` | `ShadowLLMClassify` + `ShadowLLMTimeoutMs` | 4 |
| `orchtypes/config.go:44-50` | `RuleOrchestrateConfig()` | 6 |
| `orchtypes/config.go:103-107` | `FastPathThreshold` field | 2 |
| `orchtypes/routing.go:10-12,17-18` | `RoutingModeRuleOrchestrate` enum + dead switch | 6 |
| `orchtypes/artifact_kind_alias.go:11-16` | 4 alias constants | 6 |
| `decisionplanning/prompts.go` | `TurnSystemPrompt` + `loopFirstSystemPrompt` 整文件 | 26 |
| `delegatetools/subquery_fallback.go:73-76` | `BuildSubQueryRunner`（仅 bootstrap 调用）| 4 |
| `escape/engine.go:175` | `var _ = time.Now` stub | 1 |
| `sessionorchestrator/turn_orchestrator.go:917-922` | legacy explicit-code path（DM-20260628-001 已废弃）| 6 |
| `workmodel/aggregate_verdicts.go:42-97,179` | `AggregateVerdicts` + 4 个 serialization helpers（仅 test 调用）| 60 |

**总计 ~126 LOC 死代码 + 老链路**。

### 1.5 文档漂移（P1）

| 文件:行 | 漂移 |
|---|---|
| `sessionorchestrator/turn_doc.go:1-17` | "v2.0 Slice plan" + "FastPath calls TurnOrchestrator"（FastPath 已删） |
| `orchtypes/config.go:5-13` header | 描述 v1.0 dead fields |
| `orchtypes/routing.go:24-31` | `rule_orchestrate` arm 描述 |
| `delegatetools/subquery_fallback.go:14-24` | AC6/AC10 引用（AC 已变更） |

### 1.6 T↔Span 覆盖率仅 ~30%（P1）

180 T 中明确有 Span/acceptance 证据的约 54 个。DSAFT playbook §阶段 4 v1.1 要求 ≥80%。

**主要缺口**：LP-1 Bayesian 长期信誉 / SystemAnomaly 生产触发 / AdaptivePrior 跨 S5/S6 数据契约 / ResumeSession 3 决策路径 / D7→D1 飞书卡片可观测。

### 1.7 3 项能力跨域越界未标注（P1）

违反 DSAFT 原则 4：ReputationEvidence（D7-S6 vs D6）/ SystemAnomaly（D7-S4 vs D5）/ AdaptivePrior（跨 S5+S6）。

### 1.8 WorkTree 传播/反馈机制清晰度不足（P0）

`demand.md` §1.1-1.7 是已盘点债，**用户 2026-06-29 复盘追问"WorkTree 的向下传播和向上反馈机制是否清晰？"**——双 Agent 深度审计暴露新债：

#### 1.8.1 向下传播：基本清晰（已满足）

| 证据 | 文件:行 | 评价 |
|---|---|---|
| **单一决策入口** `ApplyPipelineDecide` | `workmodel/context_decide.go:4` | ✅ ContextGraph + Spawn decide 4 步按 design §8.3 顺序执行 |
| **强类型 ChildDownlink** | `workmodel/child_downlink.go:4-13` | ✅ 显式字段 Directive/ScopeIn/ScopeOut/ExpectedReturn/FailureCriteria/ContextPolicy |
| **作用域自动继承** | `child_downlink.go:26-33` | ✅ 父 ScopeContract.InScope/OutOfScope 自动 fallback |
| **Mandatory R2 edges** | `context_decide.go:25-83` `ApplyAcceptedContextLinks` | ✅ CL0-CL8 + BlockedBy 双向 enforce |

**缺口**：无专属 T 守护 ApplyPipelineDecide 4 步顺序不变式。

#### 1.8.2 向上反馈：3 个具体不清晰点

| 问题 | 证据 | 风险 |
|---|---|---|
| **3 并发调用点缺统一治理** | `ReevaluateParentAfterChild` 在 `session_turn_loop.go:194` + `run_spawn.go:51` + `cli_commands.go:329` 3 处 | 并发场景下同一 child 多次 terminal 可能重复 rollup trigger |
| **`child.LastRound` 作隐式 envelope** | 4 处取字段：`context_bubble_apply.go:67/188` + `rollup_gate.go:36/135/154` | 无强类型约束，新增字段需改多处 |
| **`sessionRootGoal` 非确定性遍历** | `rollup_gate.go:122-129` 直接 `for range map` 返回"first" | DM-20260628-003 修过 locked-root priority bug，但**根选择逻辑本身仍非确定** |

#### 1.8.3 3 项改进建议（用户已批准纳入 #1 turn-fn-split）

| # | 改进 | 工作量 | 优先级 |
|---|---|---|---|
| **A** | 引入 `RollupReport` struct（`VerdictKind / ArtifactSummary / UncertaintyMean / SpawnPolicy / BubbleKind` 5 字段聚合） | 1 PR / 1 天 | P0 |
| **B** | `sessionRootGoal` 改为 `sort.Slice` 按 `item.ID` 排序保证确定性 | 0.5 天 | P0 |
| **C** | 补 3 个 T 点：D7-S0-A07-T01（ApplyPipelineDecide 4 步顺序不变式）+ D7-S0-A07-T02（ReevaluateParentAfterChild 3 调用点幂等性）+ D7-S0-A07-T03（PathA vs PathB trigger 选择矩阵） | 1 天 | P1 |

---

## §2 范围

### 2.1 In Scope（本次 Change 解决）

**6 个子 Change（S→A→F 逐层 + 清理）**：

| 子 Change | 层级 | 内容 | 工作量 |
|---|---|---|---|
| **#0** dead-code-cleanup | 横切 | ~126 LOC 死代码 + 老链路删除 + 4 处 doc drift | 1 PR / 1 天 |
| **#1** turn-fn-split | S2 + F | turn_orchestrator.go 1551 行拆 4 文件 + 36 T 重映射 + **WorkTree 上行反馈治理（A typed RollupReport + B deterministic root + C 3 T 点）** | **2-3 PR / 5.5-7.5 天** |
| **#2** registry-sync | F + 横切 | 6 个 F 路径修复 + t-registry Statistics 补完 + pipeline-architecture v1.2.0 升级 | 1 PR / 1-2 天 |
| **#3** value-flow-rename | S | 6 S 配 ValueFlow Alias + a/f/t-registry 加列 | 1 PR / 1 天 |
| **#4** t-span-coverage | A + F | 5 ops span + 4 acceptance test + Span Evidence 列填充 | 2-3 PR / 5-7 天 |
| **#5** boundary-decision | S + 横切 | 3 项 boundary debt Decision 表 + orchtypes governance 文件 | 1 PR / 1 天 |

**总计 7-9 PR / 12-18 天**（WorkTree 上行反馈治理纳入 #1 后，PR-2/3 升级为 PR-2/3/3-extended，总数 8-10 PR / 14-20 天）。

### 2.2 Out of Scope

- 不删 S 旧编号（DSAFT 原则 3：T 是安全网）
- 不下沉 ReputationEvidence / SystemAnomaly / AdaptivePrior（保留实现，仅 Decision 标注）
- 不动 5 节点管道数据契约（UncertaintyReport → Plan → Artifact → Verdict → ReputationEvidence 稳定）
- 不动 D7 bootstrap wire 拓扑（已 ≤200 行）
- 不动 v6.0.0 已 S7_Archived 的 Hard Ban 三连 + Out of Scope 6 项
- **不重构 WorkTree v2 升级**（TD-WT-01..06 单独 change）

---

## §3 Goals

| Goal | Metric | Target |
|---|---|---|
| **G1**：~126 LOC 死代码 + 老链路全删 | grep 验证 + test PASS | 0 死符号 |
| **G2**：turn_orchestrator.go 拆完 | wc -l + 文件数 | 每个文件 <800 行；3-4 个文件 |
| **G3**：Registry 路径全对 | f-registry 路径 + grep | 6/6 wrong path 修正 |
| **G4**：6 S 配 ValueFlow Alias | d7-domain.md §North Star | 6/6 |
| **G5**：T↔Span 覆盖率 ≥80% | t-registry Span Evidence 列 | ≥80%（当前 ~30%）|
| **G6**：跨域越界 Decision 表 | d7-domain.md §Out of Scope | 3/3 显式决策 |
| **G7**：Legacy 段 + ghost count 全删 | a/f-registry 头部 | 0 ghost entry |
| **G8**：pipeline-architecture.md v1.2.0 | 同步 6 S + 1 横切 | ✅ |
| **G9**：orchestration packages -race PASS | regression | 22/22 |
| **G10**：verify-archive.sh | acceptance | 12/12 PASS |
| **G11**：P0 T 100% PASS | acceptance | 193/193 |
| **G12**：god function 36 T 重映射 | t-registry 更新 | 36/36 归属正确 |
| **G13**：WorkTree 上行反馈 typed RollupReport | 4 处 child.LastRound 调用 → 1 处 RollupReport struct | 4/4 |
| **G14**：`sessionRootGoal` 确定性 | 多 root 场景下 unit test | 1/1 |
| **G15**：D7-S0-A07 3 T 点登记 | t-registry Statistics | 3/3 |

---

## §4 解决思路（6 子 Change 拆分）

### 4.1 #0 dead-code-cleanup（先行 PR）

**核心动作**：~126 LOC 死代码 + 老链路 + 4 处 doc drift 全删，0 业务逻辑改动。

**清理清单**：
1. 删 `orchtypes/uncertainty_coord.go:180-183` `ErrInvalidVerdictKind` alias
2. 删 `orchtypes/config.go` 3 dead fields（`LLMFallback` / `ShadowLLMClassify` / `ShadowLLMTimeoutMs` / `FastPathThreshold`）
3. 删 `orchtypes/config.go:54-59` `RuleOrchestrateConfig()`
4. 删 `orchtypes/routing.go:10-12,17-18` `RoutingModeRuleOrchestrate` + 死 switch arm
5. 删 `orchtypes/artifact_kind_alias.go:11-16` 4 alias constants（保留 type aliases）
6. 删 `decisionplanning/prompts.go` 整文件
7. 删 `delegatetools/subquery_fallback.go:73-76` `BuildSubQueryRunner`
8. 删 `escape/engine.go:175` `var _ = time.Now`
9. 删 `sessionorchestrator/turn_orchestrator.go:917-922` legacy explicit-code path
10. 删 `workmodel/aggregate_verdicts.go:42-97,179` `AggregateVerdicts` + 4 serialization helpers
11. 重写 4 处 doc drift（`turn_doc.go` / `config.go` header / `routing.go` / `subquery_fallback.go`）

**AC**：grep 验证 0 外部调用者；22/22 orchestration packages -race PASS；0 test 失败。

### 4.2 #1 turn-fn-split（god function 拆分）

**核心动作**：`turn_orchestrator.go` 1551 行按"职责子模块"拆 4 文件。

**拆分方案**（DM-20260626-004 合并后的解构）：

| 新文件 | 职责 | 估算 LOC |
|---|---|---|
| `turn_orchestrator.go` | 主入口 RunTurn + 路由（保留） | <300 |
| `turn_loop.go` | SessionTurnLoop + 迭代 | <500 |
| `turn_invoke.go` | LLMInvoke + ToolRound + ReAct | <500 |
| `turn_recovery.go` | EscapeEngine + ResumeSession + Error Recovery | <500 |

**36 T 重映射**：D7-S2-A06 RunTurnLoop god activity → 拆 D7-S2-A06/A07/A08/A09 4 子活动。

**AC**：每个新文件 <800 行；36 T 全部归属正确（不再单挂一函数）；22/22 -race PASS。

### 4.3 #2 registry-sync（F + 文档）

**核心动作**：
1. 6 个 F 路径修复（D7-S1-A02-F01..F06 + D7-S2-A01-F03/F04）
2. t-registry Statistics 表补完（S12/S13/S15/S16）
3. pipeline-architecture.md v1.1.0 → v1.2.0（同步 6 S + 1 横切 + LP/Trace 树）
4. Legacy 41 ghost count → 实际 deprecated 2 + canonical 66 = 68
5. d7-domain.md line 147 错写"S6-A14 PLANNED" → 改 IMPLEMENTED

**AC**：f-registry 路径全部正确；t-registry Statistics 14 S 全列；pipeline-architecture v1.2.0 与 d7-domain.md v2.5.1 一致。

### 4.4 #3 value-flow-rename（S 层语义升级）

**核心动作**：6 S 加 ValueFlow Alias 列（同上次 proposal，本次作为独立子 Change）。

### 4.5 #4 t-span-coverage（A + F）

**核心动作**：5 ops span + 4 acceptance test + Span Evidence 列填充（同上次 proposal）。

### 4.6 #5 boundary-decision（S + 横切）

**核心动作**：3 项 boundary debt Decision 表 + orchtypes governance 文件（同上次 proposal）。

---

## §5 Risks

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| 死代码删除破坏 hidden 引用 | Low | High | 双 Agent grep 0 外部调用者 + 22/22 -race PASS 守门 |
| turn_orchestrator.go 拆分破坏 turn 序列化（D7-S15） | Mid | High | D7-S15 TurnState 接口不变；按"职责切面"拆分；逐 PR 后 -race PASS |
| Registry 路径修复导致 spec drift | Low | Mid | 同步更新 t-registry + pipeline-architecture + span-registry |
| ValueFlow Alias 引入后文档阅读混乱 | Low | Mid | Alias 作为语义层叠加；不改 S 编号 |
| 5 ops span 与 telemetry/names.go 命名冲突 | Low | High | 前缀 `D7_Orchestration_*` + coverage registry 测试守门 |
| 4 acceptance test 触发 D5 路径异常 | Low | Low | 仅在 D7 域内 -race 测试 |
| 跨域 Decision 表引发其他域争议 | Mid | Mid | Decision 表显式"保留当前归属"；不推翻 v6.0.0 共识 |
| 8 PR 联动回归测试成本 | Mid | Mid | 每 PR 后 22/22 -race PASS 守门；最终 verify-archive.sh 12/12 |
| WorkTree 改动引入 race | Mid | Mid | RollupReport struct 内部加 sync.Mutex 字段；sessionRootGoal 排序 + 单测覆盖；3 调用点幂等性 unit test |
| RollupReport 引入破坏 v6.0.0 TurnState 序列化（D7-S15） | Low | High | RollupReport 不参与 TurnState 序列化（仅 process 内传递），保持 TurnState 接口不变 |

---

## §6 相关

- `docs/methodology/dsaft-methodology.md` v4.0.0
- `docs/methodology/dsaft-refactoring-playbook.md` v1.0.0
- `openspec/specs/d7-orchestration/d7-domain.md` v2.5.1
- `openspec/specs/d7-orchestration/{a,f,t,span}-registry.md`
- `openspec/specs/d7-orchestration/pipeline-architecture.md` v1.1.0（待 v1.2.0）
- `internal/layers/orchestration/` — 10 包 / 178 prod / 141 test / 28.9K LOC