# Tasks: devrix-d7-dsaft-restructuring (DM-20260629-001)

**Change ID:** `devrix-d7-dsaft-restructuring`
**Total Tasks:** 55 (按 6 子 Change 拆分 + #1 WorkTree 扩展 4 任务)
**Sprint:** d7-v6.x 维护阶段 → v7.0 演进起点
**DSAFT 阶段:** §阶段 3 North Star 重整 + §阶段 4 v1.1 Traceability + §阶段 6 双锚点对齐

---

## 子 Change #0：dead-code-cleanup（PR-1，P0，先行）

> **目标**：~126 LOC 死代码 + 老链路 + 4 处 doc drift 全删，0 业务逻辑改动。

### T01: 删 ErrInvalidVerdictKind alias

**File:** `internal/layers/orchestration/orchtypes/uncertainty_coord.go:180-183`
**Effort:** 0.1 天
**AC:** grep `ErrInvalidVerdictKind` 0 命中（除自身定义）；22/22 -race PASS
**Verify:** `rg "ErrInvalidVerdictKind" internal/layers/orchestration/`

### T02: 删 orchtypes/config.go 3 dead fields + RuleOrchestrateConfig

**File:** `internal/layers/orchestration/orchtypes/config.go`
**Effort:** 0.3 天
**AC:** `LLMFallback` / `ShadowLLMClassify` / `ShadowLLMTimeoutMs` / `FastPathThreshold` / `RuleOrchestrateConfig()` 全删；test 文件 5 处迁移到 `DefaultConfig`
**Verify:** `rg "LLMFallback|ShadowLLMClassify|FastPathThreshold|RuleOrchestrateConfig" internal/` 应为 0

### T03: 删 RoutingModeRuleOrchestrate enum + 死 switch arm

**File:** `internal/layers/orchestration/orchtypes/routing.go:10-12,17-18`
**Effort:** 0.2 天
**AC:** `RoutingModeRuleOrchestrate` 常量删；`normalizeRoutingMode` switch case 删；docstring 去 rule_orchestrate arm 描述
**Verify:** `rg "rule_orchestrate|RuleOrchestrate" internal/` 应为 0

### T04: 删 orchtypes/artifact_kind_alias.go 4 alias constants

**File:** `internal/layers/orchestration/orchtypes/artifact_kind_alias.go:11-16`
**Effort:** 0.1 天
**AC:** 4 alias constants 删；type aliases (line 9, 19) 保留；test 文件改用 `types.Artifact*`
**Verify:** `rg "orchtypes\.ArtifactStateChangeCert|orchtypes\.ArtifactProbeReport|orchtypes\.ArtifactExperimentData" internal/` 应为 0

### T05: 删 decisionplanning/prompts.go 整文件

**File:** `internal/layers/orchestration/decisionplanning/prompts.go`
**Effort:** 0.1 天
**AC:** 整文件删；`TurnSystemPrompt` / `loopFirstSystemPrompt` 全删
**Verify:** `rg "TurnSystemPrompt|loopFirstSystemPrompt" internal/` 应为 0

### T06: 删 BuildSubQueryRunner

**File:** `internal/layers/orchestration/delegatetools/subquery_fallback.go:73-76`
**Effort:** 0.2 天
**AC:** `BuildSubQueryRunner` 函数删；bootstrap 调用方迁移；docstring 去 AC6/AC10 引用
**Verify:** `rg "BuildSubQueryRunner" internal/` 应为 0

### T07: 删 escape/engine.go var _ = time.Now stub

**File:** `internal/layers/orchestration/escape/engine.go:175`
**Effort:** 0.1 天
**AC:** `var _ = time.Now` 删；time import 删（确认 0 使用）
**Verify:** `rg "time\.Now" internal/layers/orchestration/escape/engine.go` 应为 0

### T08: 删 sessionorchestrator/turn_orchestrator.go legacy explicit-code path

**File:** `internal/layers/orchestration/sessionorchestrator/turn_orchestrator.go:917-922`
**Effort:** 0.2 天
**AC:** `case len(code) > 0 && code[0] != ""` 分支删；variadic `code ...string` 参数强制 enum path；3-arg 调用方迁移到 enum
**Verify:** `rg "DM-20260620-003|Legacy explicit-code" internal/` 应为 0

### T09: 删 workmodel/aggregate_verdicts.go AggregateVerdicts + 4 serialization helpers

**File:** `internal/layers/orchestration/workmodel/aggregate_verdicts.go:42-97,179`
**Effort:** 0.3 天
**AC:** `AggregateVerdicts` 函数 + `String()` / `ParseAggregationStrategy` / `MarshalJSON` / `UnmarshalJSON` 全删；test 文件 `aggregate_verdicts_test.go` 删；executionflow/doc.go 引用移除
**Verify:** `rg "AggregateVerdicts|AggregationStrategy" internal/layers/orchestration/` 应为 0（除 executionflow/verify/ 已用替代实现）

### T10: 重写 sessionorchestrator/turn_doc.go v6.0 6S 状态描述

**File:** `internal/layers/orchestration/sessionorchestrator/turn_doc.go`
**Effort:** 0.2 天
**AC:** v2.0 Slice plan 描述删；FastPath 描述删；改写为 v6.0 6S + 1 横切当前状态
**Verify:** `rg "v2.0 Slice|FastPath calls" internal/layers/orchestration/sessionorchestrator/turn_doc.go` 应为 0

### T11: 重写 orchtypes/config.go header

**File:** `internal/layers/orchestration/orchtypes/config.go:5-13`
**Effort:** 0.1 天
**AC:** header 描述去 v1.0 dead fields；保留当前 active fields（RoutingMode / IntentKind 等）
**Verify:** 文档与代码字段一致

### T12: 重写 orchtypes/routing.go docstring

**File:** `internal/layers/orchestration/orchtypes/routing.go:24-31`
**Effort:** 0.1 天
**AC:** `IsLoopFirst` docstring 去 `rule_orchestrate` arm 描述
**Verify:** 文档与代码一致

### T13: 重写 delegatetools/subquery_fallback.go docstring

**File:** `internal/layers/orchestration/delegatetools/subquery_fallback.go:14-24`
**Effort:** 0.1 天
**AC:** docstring 去 AC6/AC10 引用；改为 D4 delegate canonical + D2 fallback deprecated 描述
**Verify:** 文档与代码一致

### T14: 5 处 test 调用方迁移

**File:** `tests/integration/d7/` + `decisionplanning/classifier_test.go`（line 87, 100, 134, 214 等）
**Effort:** 0.3 天
**AC:** `RuleOrchestrateConfig()` 5 处调用 → `DefaultConfig()`；0 test 失败
**Verify:** `go test ./...` PASS

**PR-1 总工作量**：~2.1 天

---

## 子 Change #1：turn-fn-split（PR-2 + PR-3，P0）

> **目标**：`turn_orchestrator.go` 1551 行按职责拆 4 文件；36 T 重映射到 4 子活动。

### T15: 拆 turn_loop.go（PR-2）

**File:** `internal/layers/orchestration/sessionorchestrator/turn_loop.go`（NEW）
**Effort:** 1.5 天
**AC**：
- `turn_orchestrator.go` 1551 → <1200 行
- `turn_loop.go` NEW <500 行（SessionTurnLoop + iter loop + escape 检查 + ItemPipelineRunner 调用）
- 0 函数签名变化
- D7-S15 TurnState 接口不变
- 22/22 -race PASS

**Verify:** `wc -l internal/layers/orchestration/sessionorchestrator/turn_*.go` 每个 <800 行

### T16: 拆 turn_invoke.go（PR-3 第一批）

**File:** `internal/layers/orchestration/sessionorchestrator/turn_invoke.go`（NEW）
**Effort:** 1.5 天
**AC**：
- `turn_orchestrator.go` <800 行
- `turn_invoke.go` NEW <500 行（LLMInvoke + ToolRound + ReAct iter）
- 0 函数签名变化
- 22/22 -race PASS

**Verify:** `wc -l` 检查

### T17: 拆 turn_recovery.go（PR-3 第二批）

**File:** `internal/layers/orchestration/sessionorchestrator/turn_recovery.go`（NEW）
**Effort:** 1.5 天
**AC**：
- `turn_orchestrator.go` <300 行（仅主入口）
- `turn_recovery.go` NEW <500 行（EscapeEngine 接入 + ResumeSession 3 决策 + Error Recovery）
- 0 函数签名变化
- 22/22 -race PASS

**Verify:** `wc -l` 检查

### T18: 36 T 重映射到 D7-S2-A06/A07/A08/A09 4 子活动

**File:** `openspec/specs/d7-orchestration/t-registry.md`（用 rg 定位行号，不硬编码 S4 实施时位置）
**Effort:** 0.5 天
**AC**：
- D7-S2-A06 god activity 拆 D7-S2-A06 (RunTurn 主入口, ~10 T) + D7-S2-A07 (SessionTurnLoop, ~10 T) + D7-S2-A08 (LLM Invoke + ReAct, ~8 T) + D7-S2-A09 (Error Recovery + Resume, ~8 T)
- 36 T 全部归属正确（不再单挂 RunTurnLoop 单一函数）
- t-registry Statistics 总数 230（重映射不增 ID）

**Verify（动态定位，不依赖硬编码行号）**：
- `rg "^D7-S2-A06-T0[1-9]" openspec/specs/d7-orchestration/t-registry.md | wc -l` = 36
- `rg "^D7-S2-A07-T0[1-9]" openspec/specs/d7-orchestration/t-registry.md | wc -l` ≈ 10
- `rg "^D7-S2-A08-T0[1-9]" openspec/specs/d7-orchestration/t-registry.md | wc -l` ≈ 8
- `rg "^D7-S2-A09-T0[1-9]" openspec/specs/d7-orchestration/t-registry.md | wc -l` ≈ 8

### T19: a-registry 加 D7-S2-A07/A08/A09 3 子活动

**File:** `openspec/specs/d7-orchestration/a-registry.md`
**Effort:** 0.3 天
**AC**：3 个新 A 登记；Input/Output/State Change/Code Location 完整
**Verify:** a-registry 头部统计 A 数 49 → 52

**PR-2 + PR-3 总工作量**：~5.3 天

---

### 子 Change #1 扩展：WorkTree 上行反馈治理（PR-3-extended，P0，用户 2026-06-29 复盘触发）

> **目标**：3 项治理（RollupReport struct + deterministic root + 3 T 点），消除 child.LastRound 隐式 envelope + sessionRootGoal 非确定性 + 缺 T 守门 3 类债。

### T52: 新建 workmodel/rollup_report.go RollupReport struct

**File:** `internal/layers/orchestration/workmodel/rollup_report.go`（NEW）
**Effort:** 0.5 天
**AC**：
- RollupReport struct **7 字段（5 数据 + 2 元数据）**：`VerdictKind / ArtifactSummary / UncertaintyMean / SpawnPolicy / BubbleKind`（5 数据）+ `ChildID / GeneratedAt`（2 元数据）
- 构造器 `NewRollupReportFromRound(childID string, round *WorkItemPipelineRound) *RollupReport` 替代 4 处 `child.LastRound` 直接取字段
- 0 业务逻辑改动；仅聚合读取
- `go build ./internal/layers/orchestration/workmodel/` PASS

**Verify:** `rg "RollupReport" internal/layers/orchestration/workmodel/rollup_report.go` ≥ 8 处（struct 7 字段 + 构造器 + receiver）

### T53: ReevaluateParentAfterChild 返回 *RollupReport + 3 调用点迁移

**File:** `internal/layers/orchestration/workmodel/resolve.go` + 3 调用点：`sessionorchestrator/session_turn_loop.go:194` + `workmodel/run_spawn.go:51` + `workmodel/cli_commands.go:329`
**Effort:** 0.5 天
**AC**：
- `ReevaluateParentAfterChild` 函数签名变化 `(struct{}, error) → (*RollupReport, error)`（**向后兼容**：调用方丢弃返回值即可）
- 3 调用点全部保留现有调用形式（不强制改用返回值）
- context_bubble_apply.go:67/188 + rollup_gate.go:36/135/154 共 5 处 `child.LastRound.*` 字段读取全部替换为 `NewRollupReportFromRound(childID, child.LastRound).*`
- 22/22 -race PASS

**Verify:** `rg "child\.LastRound\.(ContextBubbleKind|ArtifactSummary|SpawnPolicy)" internal/layers/orchestration/workmodel/` = 0（除 `NewRollupReportFromRound` 构造器内）

### T54: sessionRootGoal 确定性排序（fix DM-20260628-003 残留）

**File:** `internal/layers/orchestration/workmodel/rollup_gate.go:122-129`
**Effort:** 0.3 天
**AC**：
- `sessionRootGoal` 函数体改为先 `sort.Slice(stable=true)` 按 `item.ID` 排序再返回首个
- 多 root unit test `TestSessionRootGoal_DeterministicOrder` PASS（构造 3 root 顺序打乱 → 始终返回 ID 最小的 root）
- 现有 rollup 行为不变（ID 排序的"first root"语义与现有 v6.0.0 行为兼容）

**Verify:** `go test -race ./internal/layers/orchestration/workmodel/ -run TestSessionRootGoal` PASS

### T55: t-registry 加 D7-S0-A07-T01..T03 WorkTree 治理 T 点

**File:** `openspec/specs/d7-orchestration/t-registry.md`
**Effort:** 0.2 天
**AC**：
- D7-S0-A07-T01: ApplyPipelineDecide 4 步顺序不变式（ContextBubbleDecision → AcceptedContextLinks → SpawnPolicy → ScopeContractSpawnGate）—— 对应实现：`workmodel/context_decide.go:4` `ApplyPipelineDecide`
- D7-S0-A07-T02: ReevaluateParentAfterChild 3 调用点幂等性（同一 child 多次 terminal 仅触发 1 次 rollup）—— 对应实现：`workmodel/resolve.go:7` + 3 调用点
- D7-S0-A07-T03: Path A vs Path B rollup trigger 选择矩阵：
  - **Path A** (eager rollup)：`workmodel/rollup_gate.go:26` `ShouldRollupAfterChildren(parent, policy, stats)` — 3 policies (all_pass/best_effort/min_coverage) × 2 needs_rollup (true/false) = 6 组合 unit test
  - **Path B** (root fallback)：`workmodel/rollup_gate.go:89` `MaybeRootRollupFallback(sessionID, tm)` — 2 has_ephemeral (true/false) × 2 needs_rollup (true/false) = 4 组合 unit test
  - **合计**：6 + 4 = 10 组合 unit test 覆盖
- t-registry Statistics 总数 276 → 279（46 A00-A06 + 3 A07 新增）

**Verify:** `rg "^D7-S0-A07-T0[1-3]" openspec/specs/d7-orchestration/t-registry.md | wc -l` = 3

**PR-3-extended 总工作量**：~1.5 天

**子 Change #1 总工作量（含扩展）**：~6.8 天（5.3 + 1.5）

---

## 子 Change #2：registry-sync（PR-4，P0）

> **目标**：6 F 路径修复 + t-registry Statistics 补完 + pipeline-architecture v1.2.0 + Legacy ghost count + d7-domain.md 错配

### T20: F 路径修复 D7-S1-A02-F01..F06

**File:** `openspec/specs/d7-orchestration/f-registry.md` line 29-34
**Effort:** 0.1 天
**AC**：`contextengine/tasks/task_manager.go` → `workmodel/task_manager.go`（6 行）
**Verify:** `rg "contextengine/tasks/task_manager" openspec/` 应为 0

### T21: F 命名修复 D7-S2-A01-F03/F04

**File:** `openspec/specs/d7-orchestration/f-registry.md` line 71-72
**Effort:** 0.1 天
**AC**：`orchestrate` → `orchestratePath`；F04 路径 `orchestrator.go + EventPublisher` → `turn_orchestrator.go`（EventPublisher 在 turn_orchestrator.go）
**Verify:** `rg "orchestrator\.go.*EventPublisher" openspec/` 应为 0

### T22: F 路径修复 D7-S5-A01-F01..F03

**File:** `openspec/specs/d7-orchestration/f-registry.md` line 147-149
**Effort:** 0.1 天
**AC**：F02/F03 路径修正为 `classifier_fallback.go`
**Verify:** 路径与代码一致

### T23: t-registry 路径修复 D7-S4-T01..T09

**File:** `openspec/specs/d7-orchestration/t-registry.md` line 30-37
**Effort:** 0.1 天
**AC**：`orchestration/workplan/` → `orchestration/executionflow/workplan/`；`orchestration/imsink/` → `executionflow/imsink/`（9 行）
**Verify:** 路径与代码一致

### T24: t-registry Statistics 表补完

**File:** `openspec/specs/d7-orchestration/t-registry.md` line 444-448
**Effort:** 0.3 天
**AC**：Scenario 列表补 S12/S13/S15/S16；总数 177 → 230；按 S 求和 = Statistics 表数
**Verify:** Statistics 总数与 Scenario 求和一致

### T25: pipeline-architecture.md v1.1.0 → v1.2.0

**File:** `openspec/specs/d7-orchestration/pipeline-architecture.md`
**Effort:** 1 天
**AC**：
- §2.1 13 S → 6 S + 1 横切
- §1.1 ASCII 图标签同步（Observe 归 S5 / Plan 归 S5 / Execute 归 S6 / Verify 归 S4 / Learn 归 S6）
- §6.3 路径加 `executionflow/` 前缀
- 版本号 v1.2.0

**Verify:** 与 d7-domain.md v2.5.1 完全一致

### T26: f-registry Legacy ghost count 修复

**File:** `openspec/specs/d7-orchestration/f-registry.md` line 13, 332
**Effort:** 0.2 天
**AC**："Legacy 41 + Canonical 27" → "deprecated 2 + canonical 66 = 68"
**Verify:** Statistics 总数仍为 68

### T27: a-registry Legacy 双轨段删除

**File:** `openspec/specs/d7-orchestration/a-registry.md` line 32-50
**Effort:** 0.1 天
**AC**：Legacy 双轨段删（已无意义）
**Verify:** `rg "Legacy 双轨" openspec/specs/d7-orchestration/a-registry.md` 应为 0

### T28: d7-domain.md line 147 错配修复

**File:** `openspec/specs/d7-orchestration/d7-domain.md` line 147
**Effort:** 0.1 天
**AC**："D7-S6-A14 T 层仍 PLANNED" → IMPLEMENTED
**Verify:** 与 t-registry D7-S6-A14 状态一致

### T29: span-registry 路径同步

**File:** `openspec/specs/d7-orchestration/span-registry.md`
**Effort:** 0.2 天
**AC**：路径加 `executionflow/` 前缀（与 t-registry 一致）
**Verify:** 路径与代码一致

**PR-4 总工作量**：~2.2 天

---

## 子 Change #3：value-flow-rename（PR-5，P1）

> **目标**：6 S 配 ValueFlow Alias；a/f/t-registry 加 ValueFlow Semantic 列

### T30: d7-domain.md §North Star 6 S 加 ValueFlow Alias 列

**File:** `openspec/specs/d7-orchestration/d7-domain.md`
**Effort:** 0.3 天
**AC**：

| S 名 | ValueFlow Alias |
|---|---|
| S1 WorkModel | Multi-Step Task Coordination |
| S2 SessionOrchestrator | Turn-Based Conversation |
| S3 WaveScheduler | Parallel Worktree Execution |
| S4 ExecutionFlow + Verify | Trustworthy Conclusion Delivery |
| S5 DecisionPlanning + Observe | Intent + Uncertainty Quantization |
| S6 MUPS Pipeline | Learn from Outcome |

**Verify:** §North Star 表新增列；6/6 S 配 Alias

### T31: a-registry 加 ValueFlow Semantic 列

**File:** `openspec/specs/d7-orchestration/a-registry.md`
**Effort:** 0.3 天
**AC**：49 A 全部加 ValueFlow Semantic 列；每个 A 标注用户动作语义
**Verify:** 表头新增列；49 行填值

### T32: f-registry 加 ValueFlow Semantic 列

**File:** `openspec/specs/d7-orchestration/f-registry.md`
**Effort:** 0.3 天
**AC**：68 F 全部加 ValueFlow Semantic 列
**Verify:** 表头新增列；68 行填值

### T33: t-registry 加 ValueFlow Semantic + Span Evidence 列

**File:** `openspec/specs/d7-orchestration/t-registry.md`
**Effort:** 0.5 天
**AC**：
- ValueFlow Semantic 列 230 T 全部填值
- Span Evidence 列 230 T 全部填值；覆盖率 ≥80%
- 缺口 T 在 observability-guide.md §"T-Without-Span Tracker" 列出

**Verify:** 覆盖率 ≥80%

### T34: terminal-state-guide.md §3 IntentKind 四链加"用户感知层"

**File:** `openspec/specs/d7-orchestration/terminal-state-guide.md`
**Effort:** 0.3 天
**AC**：每个 IntentKind 分支新增"用户感知"子节（Skip/Command/Fast/Orchestrate）
**Verify:** 文档与代码一致

**PR-5 总工作量**：~1.7 天

---

## 子 Change #4：t-span-coverage（PR-6 + PR-7 + PR-8，P1）

> **目标**：T↔Span 覆盖率 ~30% → ≥80%；5 ops span + 4 acceptance test

### T35: 新增 5 个 ops span 常量

**File:** `internal/layers/observability/instrument/telemetry/names.go`
**Effort:** 0.3 天
**AC**：

```go
OpD7_Orchestration_LongTerm_Reputation_Update = "D7_LongTerm_Reputation_Update"
OpD7_Orchestration_Anomaly_Trigger             = "D7_Anomaly_Trigger"
OpD7_Orchestration_AdaptivePrior_Inject        = "D7_AdaptivePrior_Inject"
OpD7_Orchestration_Resume_Decision_Path        = "D7_Resume_Decision_Path"
OpD7_Orchestration_Feishu_Card_Render          = "D7_Feishu_Card_Render"
```

**Verify:** `rg "OpD7_Orchestration_(LongTerm|Anomaly|AdaptivePrior|Resume|Feishu)" internal/layers/observability/instrument/telemetry/names.go`

### T36: 注册 5 个 SpanMeta

**File:** `internal/layers/orchestration/sessionorchestrator/spans.go`
**Effort:** 0.3 天
**AC**：5 SpanMeta 注册（SinceVersion: "2.6.0", Instrumented: true）+ Key Attributes
**Verify:** spans.go 5 个新 SpanMeta

### T37: coverage/registry_test.go 加 5 新 Op 期望列表

**File:** `internal/layers/observability/diagnose/coverage/registry_test.go`
**Effort:** 0.2 天
**AC**：期望列表加 5 新 Op；CI 守门未来加 T 必须补 Span
**Verify:** `go test ./internal/layers/observability/diagnose/coverage/...` PASS

### T38: ApplyResumeSession 3 决策路径发独立 span

**File:** `internal/layers/orchestration/sessionorchestrator/orchestrator.go::ApplyResumeSession`
**Effort:** 0.5 天
**AC**：A fall through / B user_accept→ForceExit / C user_cancel→AbortWithAudit 3 决策路径发 `D7_Orchestration_Resume_Decision_Path` span；attributes: `resume.decision` / `resume.user_choice` / `resume.circuit_level` / `resume.exit_reason`
**Verify:** integration test 验证 span 触发

### T39: BayesianUpdate 后发长程信誉 span

**File:** `internal/layers/orchestration/mups/learn/reputation/store.go::BayesianUpdate`
**Effort:** 0.3 天
**AC**：BayesianUpdate 调用后发 `D7_Orchestration_LongTerm_Reputation_Update` span；attributes: `reputation.{prior,posterior}_{alpha,beta}` / `reputation.wilson_{lower,upper}` / `reputation.verifier_failure_count` / `reputation.track_mode`
**Verify:** integration test LP-1 acceptance

### T40: SystemAnomaly 触发时发 anomaly span

**File:** `internal/layers/orchestration/executionflow/verify/anomaly.go::SystemAnomalyDetector.Trigger`
**Effort:** 0.3 天
**AC**：SystemAnomalyDetector 触发时发 `D7_Orchestration_Anomaly_Trigger` span；attributes: `anomaly.{kind,severity,threshold}` / `anomaly.evidence_id`
**Verify:** integration test 验证 span 触发

### T41: buildObserveRequest 注入 AdaptivePrior 时发 inject span

**File:** `internal/layers/orchestration/sessionorchestrator/observe.go::buildObserveRequest`
**Effort:** 0.3 天
**AC**：learner.Inject 调用后发 `D7_Orchestration_AdaptivePrior_Inject` span；attributes: `prior.{adaptive_kind,beta_alpha,beta_beta}` / `prior.source_learner`
**Verify:** integration test 验证跨 S5/S6 数据契约

### T42: feishu finalizeReplyCardStreaming 发卡片渲染 span

**File:** `internal/layers/communication/feishu/progress.go::finalizeReplyCardStreaming`
**Effort:** 0.3 天
**AC**：finalizeReplyCardStreaming 发 `D7_Orchestration_Feishu_Card_Render` span；attributes: `feishu.{card_type,update_method}` / `d7.{last_verdict,last_exit_reason}`
**Verify:** integration test 验证 D7→D1 可观测

### T43: 4 个 acceptance test（PR-7）

**Files:**
- `tests/integration/d7/d7_acceptance_lp1_test.go` — LP-1 长期 Bayesian 信誉
- `tests/integration/d7/d7_acceptance_lp2_test.go` — LP-2 Pending 隔离
- `tests/integration/d7/d7_acceptance_lp5_test.go` — LP-5 反向追溯
- `tests/integration/d7/d7_acceptance_resume_test.go` — ResumeSession 3 决策

**Effort:** 2 天
**AC**：4 acceptance test PASS；覆盖 D7-S12-A42 / D7-S13-A49 / D7-S14-A57-A59 / D7-S4-A08/A09 等缺口 T 点
**Verify:** `go test -tags=acceptance ./tests/integration/d7/` PASS

### T44: t-registry Span Evidence 列填充 + T-Without-Span Tracker（PR-8）

**Files:**
- `openspec/specs/d7-orchestration/t-registry.md`
- `openspec/specs/d7-orchestration/observability-guide.md`

**Effort:** 1 天
**AC**：
- t-registry.md 230 T Span Evidence 列填充；覆盖率 ≥80%
- observability-guide.md 新增 §"T-Without-Span Tracker"，列出剩余 ~46 个缺口 T

**Verify:** 覆盖率 ≥80%

**PR-6 + PR-7 + PR-8 总工作量**：~5.5 天

---

## 子 Change #5：boundary-decision（PR-9，P1）

> **目标**：3 项 boundary debt Decision 表 + orchtypes governance 文件

### T45: d7-domain.md §Out of Scope 加 Pending Boundary Decision 列

**File:** `openspec/specs/d7-orchestration/d7-domain.md`
**Effort:** 0.3 天
**AC**：3 项越界能力显式标注（ReputationEvidence / SystemAnomaly / AdaptivePrior）
**Verify:** §Out of Scope 表新增列

### T46: design.md §Cross-Domain Boundary 加 Decision 表（3 项）

**File:** `openspec/specs/d7-orchestration/design.md`
**Effort:** 0.5 天
**AC**：3 个 Decision 完整（归属 D / 跨域性质 / 当前状态 / 推荐 / Future re-evaluate）
**Verify:** Decision 表 3/3

### T47: orchtypes/boundary_decision.go governance 文件

**File:** `internal/layers/orchestration/orchtypes/boundary_decision.go`
**Effort:** 0.5 天
**AC**：3 个 boundary debt 常量：

```go
BoundaryReputationEvidence = "boundary-debt:reputation-evidence-v7.0"
BoundarySystemAnomaly = "boundary-debt:system-anomaly-v7.0"
BoundaryAdaptivePrior = "boundary-debt:adaptive-prior-v7.0"
```

**Verify:** 编译通过

### T48: orchtypes/boundary_decision_test.go governance 测试

**File:** `internal/layers/orchestration/orchtypes/boundary_decision_test.go`
**Effort:** 0.3 天
**AC**：测试断言 3 个常量存在 + 版本号格式 `boundary-debt:{name}-v{major}.{minor}`
**Verify:** `go test ./internal/layers/orchestration/orchtypes/...` PASS

**PR-9 总工作量**：~1.6 天

---

## 子 Change #6：verify-archive（S7_Archive）

> **目标**：verify-archive.sh 12/12 PASS + S7 归档

### T49: 准备 archive 目录 + verify-archive.sh

**Files:**
- `openspec/archive/2026-06-29-devrix-d7-dsaft-restructuring/`
- 6 产物：proposal + tasks + design + spec + acceptance-report + delta-spec

**Effort:** 0.5 天
**AC**：`./scripts/verify-archive.sh openspec/changes/devrix-d7-dsaft-restructuring/` 12/12 PASS
**Verify:** verify-archive.sh 输出

### T50: demand-archive-index.md 更新

**File:** `openspec/specs/project/demand-archive-index.md`
**Effort:** 0.2 天
**AC**：新增 DM-20260629-001 行（含 9 PR 列表 + S7_Archived 状态）
**Verify:** 索引行存在

### T51: 主规格同步 v2.5.1 → v2.6.0

**Files:**
- `openspec/specs/d7-orchestration/d7-domain.md` v2.5.1 → v2.6.0
- `openspec/specs/d7-orchestration/{a,f,t,span}-registry.md` 版本号同步

**Effort:** 0.3 天
**AC**：版本号统一升级；修订记录加 v2.6.0 段
**Verify:** 版本号一致

**S7 总工作量**：~1 天

---

## 总工作量

| 子 Change | PR 数 | 任务数 | 工作日 |
|---|---|---|---|
| **#0 dead-code-cleanup** | 1 | 14 | 2.1 |
| **#1 turn-fn-split** | 2 | 5 | 5.3 |
| **#2 registry-sync** | 1 | 10 | 2.2 |
| **#3 value-flow-rename** | 1 | 5 | 1.7 |
| **#4 t-span-coverage** | 3 | 10 | 5.5 |
| **#5 boundary-decision** | 1 | 4 | 1.6 |
| **#6 verify-archive** | 0 | 3 | 1.0 |
| **总计** | **10 PR** | **55 任务** | **~21.2 天** |

## T 点登记

| 子 Change | T 编号 | 数量 |
|---|---|---|
| #0 | D7-S0-A00-T01..T14 (Cleanup Meta-Scenario) | 14 |
| #1 | D7-S2-A06-T01..T36 重映射到 A06-A09 | 36 |
| **#1 扩展** | **D7-S0-A07-T01..T03 (WorkTree 治理)** | **3** |
| #2 | D7-S0-A02-T01..T10 (Registry Sync) | 10 |
| #3 | D7-S0-A03-T01..T05 (ValueFlow) | 5 |
| #4 | D7-S0-A04-T01..T10 (Span Coverage) | 10 |
| #5 | D7-S0-A05-T01..T04 (Boundary) | 4 |
| #6 | D7-S0-A06-T01..T03 (Archive) | 3 |
| **总计** | **D7-S0-A00..A07 × 任务** | **55 任务** |

注：T 编号采用 **D7-S0**（Meta-Scenario for DSAFT 治理 + 清理 + WorkTree 治理）作为新号段；D7-S2-A06 的 36 T 在 T18 重映射到 A06-A09（保持 S2 命名空间）；D7-S0-A07 为 WorkTree 上行反馈治理新增号段。

## 验收守门

- **每 PR 后**：22/22 orchestration packages `go test -race ./...` PASS
- **最终**：verify-archive.sh 12/12 PASS
- **10 PR 全部**：squash merge + auto-merge（按 `feedback-devrix-pr-auto-merge.md`）
- **S7 archive**：archive/2026-06-29-devrix-d7-dsaft-restructuring/ 模板就位
- **demand-archive-index.md**：新增 DM-20260629-001 行
- **P0 T 100%**：193/193 PASS
- **6 子 Change AC + 6 AC-WT**：~60 AC 全表（详见 spec.md）

---

## 顺序依赖图

```
PR-1 (#0 dead-code-cleanup)
   │
   ├─► PR-2 (#1 turn-fn-split 第一批: turn_loop.go)
   │      │
   │      ▼
   │   PR-3 (#1 turn-fn-split 第二批: turn_invoke.go + turn_recovery.go)
   │      │
   │      ▼
   │   PR-3-extended (#1 turn-fn-split 第三批: WorkTree 治理 — RollupReport + deterministic root + 3 T 点)
   │
   ├─► PR-4 (#2 registry-sync)
   │      │
   │      ▼
   │   PR-5 (#3 value-flow-rename)
   │      │
   │      ├─► PR-6 (#4 t-span-coverage 第一批: 5 ops span + coverage)
   │      │      │
   │      │      ▼
   │      │   PR-7 (#4 t-span-coverage 第二批: 4 acceptance test)
   │      │      │
   │      │      ▼
   │      │   PR-8 (#4 t-span-coverage 第三批: t-registry 填充)
   │      │
   │      └─► PR-9 (#5 boundary-decision)
   │
   ▼
verify-archive + S7_Archive (T49-T51)
```

并行机会：
- PR-1 与 PR-4 可并行（互不依赖）
- PR-6 与 PR-9 可并行
- PR-2 完成后 PR-3 / PR-5 / PR-6 可并行（不依赖 PR-3 拆分细节）
- **PR-3-extended 仅依赖 PR-3**（god function 拆完后才能稳定修改 ReevaluateParentAfterChild 调用上下文）