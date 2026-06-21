# D6 Evolution Domain Specification

**Domain:** D6 Evolution
**DSAFT Type:** Supporting
**Version:** 2.4.0
**Last Updated:** 2026-06-21
**Status:** Canonical — source of truth
**Parent:** `openspec/specs/architecture/layering.md`
**Domain SoT:** `d6-domain.md`（2026-06-19 新建，对齐 D2/D4/D5/D7 结构）
**Change:** devrix-d3-sa-refine-v1.1（DM-20260614-017 / D6 探针 #1 / #2 / #4 落地；D2-B 决议 probe #3 推迟 v1.2）+ devrix-d5-d6-sa-refine-v2.0（DM-20260615-003 / 物理路径迁移 2026-06-15）+ devrix-spec-sync-d6-evolution-registration（DM-20260619-003 / 物理路径同步 + d6-domain.md 新建）+ **devrix-d6-evolution-review-fixes（DM-20260621-011 / 2026-06-21 落地 / bridge 清债 + Orchestration* → Guard* 重命名 + panic → log.Fatal + silent swallow 修复；PR-A #156 + PR-B #157）**

---

## Overview

D6 演化域负责 Devrix 系统的自我评估与运行时行为校验。包含两大子系统：

- **D6-S3 评测引擎**：离线评测管道，10 类探针覆盖各域质量维度（v2.2.0 新增 3 个 D3 探针），LLM-as-Judge 评分，Delta 回归检测，CI 门禁；v2.0 物理路径 `evaluate/`
- **D6-S4 GuardRuntime**：运行时校验智能体路由决策（tool_call / permit / fork），LLM Judge 交叉验证，自动干预执行；v2.0 重命名自 Orchestration，物理路径 `guard/`（**曾因重命名误删从 42bf1d7 恢复**）
- **D6-S5 VerifyInvariant**（v2.0 新增物理独立）：Invariant 验证 + Plan 验证；物理路径 `verify/`

D6-S1（版本检测）与 D6-S2（配置热更新）仍处于规划阶段。

---

## DSAFT 结构

| 层级 | 编号 | 名称 | 状态 |
|------|------|------|------|
| D | D6 | Evolution | Active |
| S | D6-S1 | Version | PLANNED |
| S | D6-S2 | Config | PLANNED |
| S | D6-S3 | Eval Engine | IMPLEMENTED |
| S | D6-S4 | GuardRuntime | IMPLEMENTED（v2.0 重命名自 Orchestration）|
| S | D6-S5 | VerifyInvariant | IMPLEMENTED（v2.0 物理独立自 evaluate）|

---

## D6-S3: Eval Engine

### 评测管道

```
LoadDataset → StratifiedSample → RunProbes(×N) → AggregateReport → DeltaCompare → SaveBaseline
                                                                        ↓
                                                                  CheckDeltaGate (CI)
                                                                        ↓
                                                                  TuneGenerator
```

### 核心组件

| 组件 | 文件 | 职责 |
|------|------|------|
| EvalEngine | `evaluate/engine.go` | 评测管道编排：加载→抽样→探针→聚合→delta→基线（v2.0 重命名自 `eval/`）|
| JudgeManager | `evaluate/judge.go` | LLM-as-Judge 评分，双模型交叉验证，Cohen's kappa 校准 |
| DeltaAnalyzer | `evaluate/delta.go` | 当前评分 vs 基线对比，回归检测 |
| TuneGenerator | `evaluate/tune.go` | 回归维度 → 配置调优建议映射 |
| DatasetManager | `evaluate/dataset.go` | YAML 评测集加载、分层抽样、基线读写 |
| ProbeRegistry | `evaluate/probe.go` | 全局探针注册表，按 ID 查找 |
| GatewayLLMClient | `evaluate/gateway_llm.go` | 经 D3 LLM Gateway 的真实 Judge 调用 |
| StaticLLMClient | `evaluate/mock_llm.go` | 固定响应 Judge，用于测试/CLI |

### 10 类探针（v2.2.0：7 + 3）

| Probe ID | 文件 | 目标域 | 评分方式 | 说明 |
|----------|------|--------|----------|------|
| compression_recall | `evaluate/compression_recall_probe.go` | D2 | Judge + 确定性 | 压缩前后事实保留率（must_keep recall + LLM 语义评分） |
| tool_accuracy | `evaluate/tool_accuracy_probe.go` | D2 | 确定性 | Tool 选择 precision/recall/F1（expected_tools vs actual_tools） |
| provider_quality | `evaluate/provider_quality_probe.go` | D3 | Judge + 确定性 | 语义相似度（wordJaccard）+ 指令遵循率 + Judge 保守融合 |
| agent_forkjoin | `evaluate/agent_forkjoin_probe.go` | D4 | 确定性 | 子 Agent 消息隔离（isolation）+ Join 结果完整度 |
| path_regression | `evaluate/path_regression_probe.go` | D2 | 确定性 | 代码路径快照对比（runtime.Snapshot() LegacyHarness=0） |
| layer_violation | `evaluate/layer_violation_probe.go` | D6 | 确定性 | 分层违规扫描（0 违规→1.0, 1→0.5, 2+→0.0） |
| session_isolation | `evaluate/session_isolation_probe.go` | D6 | 确定性 | COW 隔离评估（fork/join/metadata 计数 + D5 交叉校验） |
| **tier_resolution** _(v2.2.0)_ | `evaluate/tier_resolution_probe.go` | D3 | 确定性 | Tier 解析正确性 ≥ 99%（D2-B 决议；接 `llm_tier_resolve_total{outcome=hit/fallback/error}` 桶） |
| **breaker_anomaly_transition** _(v2.2.0)_ | `evaluate/breaker_anomaly_transition_probe.go` | D3 | 确定性 | Breaker 状态切换异常告警（frequent-flip / 异常 open 序列；接 `llm_breaker_transitions_total{from,to}`） |
| **safety_latency** _(v2.2.0)_ | `evaluate/safety_latency_probe.go` | D3 | 确定性 | Safety filter P99 < 1ms（D5-A 决议；接 `safety.check.duration_ms` span event） |

> **probe #3 Token 预算触发率**：D2-B 决议推迟至 v1.2（依赖 D3-S4 BudgetTokens 注入 span event `budget.check.exceeded`，需先期落地）。v2.2.0 不实施。

### Judge 评分机制

- **双模型交叉验证**：primary + secondary LLMClient，位置随机化（forward/reversed averaging）
- **分歧仲裁**：两模型评分差 > 0.5 时触发 dispute resolution
- **Cohen's kappa 校准**：与人类标注 GoldLabel 一致性校验
- **评分量规**：ScoreRubric 定义 {Excellent: 1.0, Good: 0.75, Fair: 0.5, Poor: 0.25, Bad: 0.0}

### Delta 回归检测

- **阈值**：Red（严重回归）< -0.05，Yellow（轻微回归）< -0.02，Green（正常）≥ -0.02
- **维度**：ByDimension（按 DomainScore.Dimension）+ ByBucket（按 score 分桶）
- **基线**：`baseline.yaml` 版本化存储于评测集目录

### CI 门禁

- `CheckDeltaGate` 检测到回归时返回 `GateResult{Passed: false}` + 非零退出码
- `FormatDeltaSummary` 生成人类可读的 CI 日志
- `scripts/eval/run-eval.sh` 供 CI 快速抽检

### 调优建议

`TuneGenerator.Suggest` 映射回归维度到配置提示：

| 回归维度 | 建议 |
|----------|------|
| compression_recall | 增加 autocompact budget |
| tool_accuracy | 启用 simple_mode |
| provider_quality | 切换 provider |
| agent_forkjoin | 调整 fork 深度限制 |
| path_regression | 检查 D2 代码变更 |
| layer_violation | 检查 import 分层 |
| session_isolation | 检查 COW 实现 |

---

## D6-S4: GuardRuntime（v2.0 重命名自 Orchestration）

### 校验管道

```
Agent Decision → GuardObserver → preFilter → Judge Validation → Intervention
                     (D4 observer)        (allowlist)   (cross-model)    (terminate/reroute)
```

### 核心组件

| 组件 | 文件 | 职责 |
|------|------|------|
| RuntimeGuardValidator | `guard/validator.go` | 决策入口：预过滤→Judge 校验→干预触发（v2.0 重命名自 `RuntimeOrchestrationValidator`） |
| InterventionExecutor | `guard/intervention.go` | 干预执行：terminate / terminateAndReroute / updateState |
| GuardObserver | `guard/observer.go` | D4 AgentObserver 桥接，捕获 agent 决策事件（v2.0 重命名自 `OrchestrationObserver`） |
| RuntimeJudge | `guard/judge_adapter.go` | 经 D3 LLM Gateway 的跨模型决策校验 |

### D6-S5: VerifyInvariant（v2.0 新增物理独立）

| 组件 | 文件 | 职责 |
|------|------|------|
| InvariantRegistry | `verify/invariant.go`（v2.4.0 由 `_invariant.go` 重命名 — 激活 Go 工具链忽略 `_` 前缀文件的 dead code） | 系统级不变量注册 + 校验（fail-closed，启动期 `init()` → `log.Fatalf` 替代 panic） |
| PlanVerifier | `verify/plan.go` | Plan 路径验证（与 D7 PlanMode 联动） |

### 决策分类与风险

| DecisionCategory | 说明 | 默认 RiskClass |
|------------------|------|----------------|
| tool_call | Agent 调用工具 | Evaluate |
| permit | 权限请求 | Evaluate |
| fork | 子 Agent 创建 | Critical |

### 预过滤器

- **可信工具允许列表**：特定 tool_name 直接放行（Low risk）
- **最小 Judge 间隔**：`minIntervalBetweenJudges` 防止过度调用
- **最大 Judge 速率**：`maxJudgeCallsPerMinute` 限流

### 干预动作

| 动作 | 说明 |
|------|------|
| terminate | 终止当前 agent 执行 |
| terminateAndReroute | 终止并路由到备用 agent |
| updateState | 更新 session 状态 |

### 可观测性

OpenTelemetry 指标（`guard/metrics.go`，v2.0 重命名自 `orchestration/metrics.go`）：
- `guard_decisions_total` — 决策计数（按 category/risk，v2.0 重命名自 `orch_decisions_total`）
- `guard_validations_total` — 校验计数（按 result）
- `guard_interventions_total` — 干预计数（按 action）
- `guard_judge_latency_seconds` — Judge 调用延迟
- `guard_observer_active` — Observer 活跃状态
- `guard_decisions_by_stage` — 各阶段决策分布

---

## REQUIREMENTS

### D6-S3: Eval Engine

<!-- D6-S3-A01-T01 -->
#### Requirement: EvalRun 编排
评测引擎必须支持从评测集到评分报告的完整编排。

**Scenario: 基本编排流程**
- GIVEN 一个包含评测用例的 YAML 数据集
- WHEN EvalRun 被调用
- THEN 返回的 EvalReport 包含所有维度的 DomainScore
- AND EvalReport 包含评分面板（ScoreDashboard）

<!-- D6-S3-A01-T03 -->
#### Requirement: Compression Recall Probe
Compression Recall Probe 必须评估压缩前后的事实保留率。

**Scenario: 压缩召回评估**
- GIVEN 包含 must_keep 标注的评测用例
- WHEN CompressionRecallProbe.Run 被调用
- THEN 返回 must_keep 关键词的 recall 分数
- AND 包含 LLM Judge 语义保留评分

<!-- D6-S3-A01-T06 -->
#### Requirement: Tool Accuracy Probe
Tool 准确率探针必须评估 tool 选择的 precision/recall/F1。

**Scenario: Tool 选择准确率**
- GIVEN 包含 expected_tools 的评测用例
- WHEN ToolAccuracyProbe.Run 被调用
- THEN 返回 precision/recall/F1 分数（确定性计算）

<!-- D6-S3-A02-T02 -->
#### Requirement: LLM-as-Judge 评分校准
Judge 管理器必须支持双模型交叉验证与 Cohen's kappa 校准。

**Scenario: 交叉验证**
- GIVEN primary 和 secondary LLMClient
- WHEN Score 被调用
- THEN 执行位置随机化的 forward+reversed 评分
- AND 返回平均分数

**Scenario: 分歧仲裁**
- GIVEN 两模型评分差 > 0.5
- WHEN 分歧被检测到
- THEN 触发 dispute resolution

**Scenario: 校准**
- GIVEN 人类标注 GoldLabel
- WHEN Calibrate 被调用
- THEN 返回 Cohen's kappa 一致性分数

<!-- D6-S3-A01-T04 -->
#### Requirement: Delta 回归检测
Delta 分析器必须对比当前评分与基线，标记回归。

**Scenario: 回归检测**
- GIVEN 当前 EvalReport 与 baseline
- WHEN DeltaAnalyzer 运行
- THEN 维度分数下降 > 5% 标记为 Red（严重回归）
- AND 维度分数下降 > 2% 标记为 Yellow（轻微回归）

<!-- D6-S3-A01-T09 -->
#### Requirement: Provider Quality Probe
Provider 质量探针必须评估语义相似度与指令遵循率。

**Scenario: Provider 质量对比**
- GIVEN 包含 expected_output 和 instructions 的评测用例
- WHEN ProviderQualityProbe.Run 被调用
- THEN 计算 wordJaccard 语义相似度
- AND 计算指令遵循率
- AND 保守融合（conservativeMin）确定性分数与 Judge 分数

<!-- D6-S3-A01-T10 -->
#### Requirement: Agent Fork/Join Probe
Fork/Join 探针必须评估子 Agent 消息隔离与 Join 结果完整度。

**Scenario: Fork/Join 质量评估**
- GIVEN 包含 forbidden_content 和 must_include 的评测用例
- WHEN AgentForkJoinProbe.Run 被调用
- THEN 验证 forbidden_content 未泄露到子 agent（isolation）
- AND 验证 must_include 出现在 join 结果中（completeness）

<!-- D6-S3-A01-T12 -->
#### Requirement: 调优建议生成
Delta 报告出现 regression 时必须生成 TuneSuggestion 列表。

**Scenario: 调优建议**
- GIVEN 包含回归维度的 EvalDelta
- WHEN TuneGenerator.Suggest 被调用
- THEN 返回与回归维度对应的配置调优建议

<!-- D6-S3-A01-T14 -->
#### Requirement: CI Delta 门禁
CI 管道必须在检测到回归时非零退出。

**Scenario: 门禁通过**
- GIVEN delta 无回归或仅有轻微回归
- WHEN CheckDeltaGate 被调用
- THEN GateResult.Passed = true

**Scenario: 门禁失败**
- GIVEN delta 包含严重回归（分数下降 > 5%）
- WHEN CheckDeltaGate 被调用
- THEN GateResult.Passed = false
- AND 返回非零退出码

<!-- D6-S3-A01-T15 -->
#### Requirement: 评测基线管理
评测集目录必须包含可版本化的基线文件。

**Scenario: 基线保存与加载**
- GIVEN EvalReport 且 --baseline flag 启用
- WHEN SaveBaseline 被调用
- THEN 写入 baseline.yaml
- AND LoadBaseline 可成功加载

<!-- D6-S3-A01-T07 -->
#### Requirement: 功能开关
评测引擎必须可通过配置关闭。

**Scenario: enabled=false**
- GIVEN evolution.eval.enabled=false
- WHEN 任意评测调用被触发
- THEN 评测引擎不执行任何操作
- AND 返回 nil

### 新增探针 (v2.1.0)

<!-- D6-S3-A01-T16 -->
#### Requirement: Path Regression Probe
Path Regression Probe 必须检测代码路径变更导致的回归。

**Scenario: 路径回归检测**
- GIVEN runtime.Snapshot() 返回 LegacyHarness=0
- WHEN PathRegressionProbe.Run 被调用
- THEN 确定性评分，无需 Judge

<!-- D6-S3-A01-T17 -->
#### Requirement: Layer Violation Probe
Layer Violation Probe 必须检测分层架构违规。

**Scenario: 分层违规扫描**
- GIVEN 代码库包含 import 关系
- WHEN LayerViolationProbe.Run 被调用
- THEN 0 违规 → Score 1.0
- AND 1 违规 → Score 0.5
- AND 2+ 违规 → Score 0.0

<!-- D6-S3-A01-T18 -->
#### Requirement: Session Isolation Probe
Session Isolation Probe 必须评估 COW 隔离正确性。

**Scenario: Session 隔离评估**
- GIVEN fork/join 操作计数与 metadata 写入记录
- WHEN SessionIsolationProbe.Run 被调用
- THEN 验证 fork/join 一致性
- AND 交叉校验 D5 observability 计数器

### 新增探针 (v2.2.0)

<!-- D6-S3-A01-T20 -->
#### Requirement: Tier Resolution Probe
Tier Resolution Probe 必须评估 D3 Tier 解析正确性 ≥ 99%。

**Scenario: Tier 解析覆盖率**
- GIVEN D3 Gateway 路由决策序列（带 `tier` 属性）
- WHEN TierResolutionProbe.Run 被调用
- THEN 统计 `llm_tier_resolve_total{outcome=hit}` 占比
- AND `hit / (hit + fallback + error) ≥ 99%` 时 Score = 1.0
- AND `< 99%` 时 Score = hit_ratio，标记 Yellow（轻微回归）
- AND `error > 0` 时触发 Red（严重回归）

**Scenario: 桶分布校验**
- GIVEN `llm_tier_resolve_total` 三桶计数（hit / fallback / error）
- WHEN Probe 跨 bucket 聚合
- THEN 记录 `tier.fallback_ratio` 与 `tier.error_ratio` 到 DomainReport
- AND 上报到 D5 dashboard `d3_tier_resolution` 面板

> **依赖**：D3-S1-A01 F06 `ProbeTierResolution` emit `llm_tier_resolve_total`（D2-B 决议，v1.1 落地）。

<!-- D6-S3-A01-T21 -->
#### Requirement: Breaker Anomaly Transition Probe
Breaker Anomaly Transition Probe 必须检测 Breaker 状态切换异常模式。

**Scenario: 频繁翻转告警**
- GIVEN `llm_breaker_transitions_total{from, to}` 时间序列
- WHEN BreakerAnomalyTransitionProbe.Run 被调用
- THEN 滚动窗口（默认 5min）内翻转次数 > 3 标记 Yellow
- AND 同一 provider `open→closed` 与 `closed→open` 在 30s 内交替 2 次以上标记 Red

**Scenario: 异常状态序列**
- GIVEN Breaker 状态序列
- WHEN 解析状态转移图
- THEN `open→open`（自环，异常）或 `half_open→open` 连续 2 次无 `closed` 介入标记 Red
- AND 异常事件写入 `breaker.anomaly_events` DomainReport 字段

> **依赖**：D3-S3-A01 F07 `OnStateTransitionEmit` emit `llm_breaker_transitions_total{provider, from, to}`（v1.1 新增）。

<!-- D6-S3-A01-T22 -->
#### Requirement: Safety Filter Latency Probe
Safety Filter Latency Probe 必须验证 D3-S5 safety filter P99 < 1ms。

**Scenario: P99 延迟分布**
- GIVEN `safety.check.duration_ms` span event 时间序列
- WHEN SafetyLatencyProbe.Run 被调用
- THEN 计算 P50 / P95 / P99 / max 四个分位数
- AND P99 < 1ms（目标 1000µs）时 Score = 1.0
- AND P99 ∈ [1ms, 2ms) 标记 Yellow（轻微回归）
- AND P99 ≥ 2ms 标记 Red（严重回归）

**Scenario: 延迟趋势告警**
- GIVEN P99 时间序列（至少 100 样本）
- WHEN Probe 检测上升趋势
- THEN 连续 3 个滚动窗口 P99 上升 > 10% 触发 pre-Red 告警（写入 DomainReport.warning）
- AND 不计入 Delta 回归（pre-Red 仅为预警）

> **依赖**：D3-S5-A01 F04 `EmitSafetyLatencyEvent` 在 `llm.stream` span 上 emit `safety.check.duration_ms`（D5-A 决议，v1.1 落地，默认 `d3_safety_latency_event_enabled` ON）。

### D6-S4: GuardRuntime（v2.0 重命名）

<!-- D6-S4-A01-T01 -->
#### Requirement: 运行时决策校验
Guard 校验器必须对 Agent 路由决策进行 LLM 交叉验证。

**Scenario: 决策校验通过**
- GIVEN Agent 发出 tool_call 决策
- WHEN RuntimeGuardValidator.OnDecision 被调用
- THEN preFilter 检查通过后调用 Judge 校验
- AND 校验通过时决策正常执行

**Scenario: 决策校验失败**
- GIVEN Judge 判定决策无效
- WHEN ValidationResult.Valid = false
- THEN 触发 InterventionExecutor 执行干预

<!-- D6-S4-A02-T01 -->
#### Requirement: 自动干预执行
干预执行器必须支持 terminate / reroute / updateState 三种动作。

**Scenario: 终止 Agent**
- GIVEN ValidationResult 失败 + Intervention.Action = terminate
- WHEN InterventionExecutor.Execute 被调用
- THEN 调用 AgentController.Terminate

**Scenario: 重路由**
- GIVEN ValidationResult 失败 + Intervention.Action = terminateAndReroute
- WHEN InterventionExecutor.Execute 被调用
- THEN 终止当前 agent 并路由到备用 agent

<!-- D6-S4-A03-T01 -->
#### Requirement: Agent 事件观测
Guard 观察器必须作为 D4 AgentObserver 桥接决策事件。

**Scenario: 捕获 Fork 决策**
- GIVEN agent.forked 事件
- WHEN GuardObserver 收到事件
- THEN 构造 DecisionRecord{Category: fork}
- AND 送入 RuntimeGuardValidator

<!-- D6-S4-A04-T01 -->
#### Requirement: 跨模型 Judge 适配
RuntimeJudge 必须经 D3 LLM Gateway 进行决策校验。

**Scenario: 校验请求**
- GIVEN DecisionRecord
- WHEN RuntimeJudge.ValidateDecision 被调用
- THEN 构造 JSON prompt 发送到 D3 Gateway
- AND 主模型失败时自动 fallback

### D6-S5: VerifyInvariant（v2.0 新增）

<!-- D6-S5-A01-T01 -->
#### Requirement: 系统级不变量注册与校验
InvariantRegistry 必须支持 fail-closed 校验。

**Scenario: 不变量注册**
- GIVEN 启动时通过 `RegisterInvariant` 注册 N 条不变量
- WHEN 系统状态变更触发 `Check()`
- THEN 逐条执行不变量校验
- AND 任意不变量失败 → 返回 error + 触发干预

**Scenario: Plan 路径验证**
- GIVEN Plan 工具调用 (D7 PlanMode)
- WHEN PlanVerifier.Verify 被调用
- THEN 校验 Plan 路径不违反不变量（如文件写入白名单）

### D6-S11: 韧性域新增需求 (v2.4.0, DM-20260621-011)

<!-- D6-S11-A02-T09 -->
#### Requirement: Verify Invariant 启动期 fail-safe
`InvariantRegistry` 必须在启动期 fail-safe, 解析失败时进程退出码非 0 由 systemd 重启。

**Scenario: ParseStruct 失败 → log.Fatalf**
- GIVEN `verifyInvariants` struct 标签格式错误
- WHEN `init()` 调用 `parseVerifyInvariants()`
- THEN 返回 error 而非 panic
- AND `init()` 调用 `log.Fatalf` 退出码 1
- AND `git grep "panic.*verify invariant"` 0 命中

**Scenario: 正常启动路径**
- GIVEN 默认 `verifyInvariants` struct 标签合法
- WHEN `init()` 调用 `parseVerifyInvariants()`
- THEN `verifyInvariantSet` 包含 N 条 invariant
- AND `CheckVerifyInvariants(state)` 可正常调用

### D6-S12: Guard 名空间收敛 (v2.4.0, DM-20260621-011)

<!-- D6-S12-A02-T04 -->
#### Requirement: bridge 文件零残留
`internal/layers/evolution/eval/bridge.go` + `orchestration/bridge.go` 必须不存在。

**Scenario: bridge 文件 git ls-files 0 命中**
- GIVEN spec.md v2.2.0 已声明 bridge 在 v2.0.1 cleanup 后全部删除
- WHEN PR-B 完成清理
- THEN `git ls-files internal/layers/evolution/eval/bridge.go` 0 命中
- AND `git ls-files internal/layers/evolution/orchestration/bridge.go` 0 命中

<!-- D6-S12-A03-T03 -->
#### Requirement: Orchestration* → Guard* 全量重命名
guard/ 内 `Orchestration*` 标识符仅允许出现在 type alias 定义点。

**Scenario: guard/ 内仅 alias 点**
- GIVEN spec.md v2.4.0 引入 Guard* 命名
- WHEN PR-B 完成 rename
- THEN 全仓 `git grep "Orchestration" internal/layers/evolution/guard/` 仅命中 alias 定义点（≤ 6 处）
- AND `bash scripts/check-orch-rename.sh` exit 0

<!-- D6-S12-A03-T04 -->
#### Requirement: orch_* OTel 指标 → guard_*
guard/ 内 6 个 OTel 指标名 `orch_*` → `guard_*` 全量重命名, 与 spec v2.4.0 一致。

**Scenario: 指标名注册使用新名**
- GIVEN metrics.go 中已迁移
- WHEN `initGuardMetrics(obs)` 被调用
- THEN 注册 6 个 `guard_*` 指标, 不注册 `orch_*`
- AND `orch_decisions_total` 等旧名仅在 Deprecated 注释中保留

**Scenario: 全仓 grep 0 命中旧指标名**
- WHEN CI 运行 `check-orch-rename.sh`
- THEN 全仓 `grep -r '"orch_[a-z_]"'` 在 metrics.go 之外 0 命中

<!-- D6-S12-A03-T05 -->
#### Requirement: type alias 向后兼容
v2.4 引入新名 + `//go:deprecated` type alias, v2.5 删除。

**Scenario: 旧构造函数仍可用**
- GIVEN `OrchestrationConfig` / `RuntimeOrchestrationValidator` / `OrchestrationObserver` 是 alias
- WHEN 外部调用方使用旧名
- THEN 编译通过, `go vet` 给出 `//go:deprecated` 告警
- AND 实例与新名构造的实例底层类型相同（type alias 而非 type def）

<!-- D6-S12-A01-T01 -->
#### Requirement: InterventionExecutor Wait 失败可观测
`InterventionExecutor.terminateAndReroute` 中 Wait 失败必须有 metric + slog.Warn + errors.Join 上抛。

**Scenario: Wait 失败三联固化**
- GIVEN agent.Wait 返回 error
- WHEN `terminateAndReroute` 执行
- THEN `metrics.WaitFailed.Add(1)`（nil-safe）
- AND `slog.Warn("wait current agent failed", ...)`
- AND errors.Join 聚合后上抛, errors.Is 仍可命中原始 error

**Scenario: 静默吞错反模式根除**
- GIVEN 全仓 grep `_, _ = current.Wait\|_, _ = ie.tasks.Fail`
- WHEN PR-A 修复后
- THEN 0 命中

<!-- D6-S12-A01-T02 -->
#### Requirement: InterventionExecutor tasks.Fail 失败可观测
`InterventionExecutor.terminateAndReroute` 中 `tasks.Fail` 失败必须有 metric + slog.Warn 上抛。

**Scenario: tasks.Fail 失败三联固化**
- GIVEN TaskController.Fail 返回 error
- WHEN `terminateAndReroute` 在 `iv.MilestoneFail` 分支执行
- THEN `metrics.TaskFailFailed.Add(1)`
- AND `slog.Warn("task fail failed", ...)`
- AND errors.Join 聚合后上抛

---

## 配置参考

```yaml
evolution:
  eval:
    enabled: true
    dataset_dir: "evaldata/"
    judge:
      primary_model: "deepseek-v4-flash"
      secondary_model: "minimax-M2.7-highspeed"
      max_concurrency: 4
  guard:
    enabled: false
    judge:
      primary_model: "deepseek-v4-pro"
      secondary_model: "minimax-M2.7-highspeed"
    prefilter:
      trusted_tools: ["read_file", "list_directory", "search_code"]
      min_interval_between_judges: "2s"
      max_judge_calls_per_minute: 10
  verify:
    enabled: true
    fail_closed: true  # v2.0 物理独立；D6-S5
```

---

## 相关文档

- [design.md](./design.md) — D6 演化层架构设计
- [layer-delta.md](./layer-delta.md) — 层增量变更记录
- [a-registry.md](./a-registry.md) — A 层活动注册表
- [f-registry.md](./f-registry.md) — F 层功能点注册表
- [t-registry.md](./t-registry.md) — T 层测试点注册表
- [../d3-llm-gateway/spec.md](../d3-llm-gateway/spec.md) — D3 LLM Gateway（Judge 依赖）
- [../d4-multi-agent/spec.md](../d4-multi-agent/spec.md) — D4 Multi-Agent（Orchestration 观测源）

---

## Revision History

| 版本 | 日期 | 变更 |
|------|------|------|
| 2.0.0 | 2026-06-14 | 初版：7 类探针 + S4 Orchestration |
| 2.1.0 | 2026-06-14 | 新增 Path Regression / Layer Violation / Session Isolation 3 类探针（T16/T17/T18） |
| 2.2.0 | 2026-06-14 | 落地 devrix-d3-sa-refine-v1.1 D6 探针 #1 / #2 / #4：Tier Resolution ≥ 99%（T19，D2-B 决议）+ Breaker Anomaly Transition（T20）+ Safety Latency P99 < 1ms（T21，D5-A 决议）；probe #3 Token 预算触发率 推迟至 v1.2（D2-B 决议） |
| **2.3.0** | **2026-06-19** | **v2.0 物理路径迁移同步**（DM-20260615-003, 2026-06-15 落地；DM-20260619-003 spec 同步）：§D6-S3 组件路径 `eval/` → `evaluate/`（8 个文件）；§D6-S4 重命名 Orchestration → GuardRuntime，路径 `orchestration/` → `guard/`（4 个文件 + 6 metric 重命名 `orch_*` → `guard_*`）；新增 §D6-S5 VerifyInvariant（v2.0 物理独立自 `evaluate/`）；DSAFT 表格同步；Domain SoT `d6-domain.md` 新建（对齐 D2/D4/D5/D7 结构） |
| **2.4.0** | **2026-06-21** | **deep review 修复**（DM-20260621-011 / devrix-d6-evolution-review-fixes PR-A #156 + PR-B #157）：C-1 删除残留 `eval/bridge.go` + `orchestration/bridge.go`；H-1 guard/ 内 6 处 `Orchestration*` 类型/函数重命名为 `Guard*`（type alias 保留 v2.5 删）；6 个 OTel 指标 `orch_*` → `guard_*`；H-2 `verify/_invariant.go` panic → `log.Fatalf`（同时 `_invariant.go` → `invariant.go` 重命名激活 dead code）；H-3 `intervention.go` Wait 失败 metric + slog.Warn + errors.Join；新增 D6-S11-A02-T09 + D6-S12-A01-T01/T02 + D6-S12-A02-T04 + D6-S12-A03-T03/T04/T05 6 个 P0 T 点；scripts/check-orch-rename.sh CI guard |
