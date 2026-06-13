# D6 Evolution Domain Specification

**Domain:** D6 Evolution
**DSAFT Type:** Supporting
**Version:** 2.1.0
**Last Updated:** 2026-06-14
**Status:** Canonical — source of truth
**Parent:** `openspec/specs/architecture/layering.md`

---

## Overview

D6 演化域负责 Devrix 系统的自我评估与运行时行为校验。包含两大子系统：

- **D6-S3 评测引擎**：离线评测管道，7 类探针覆盖各域质量维度，LLM-as-Judge 评分，Delta 回归检测，CI 门禁
- **D6-S4 编排校验**：运行时校验智能体路由决策（tool_call / permit / fork），LLM Judge 交叉验证，自动干预执行

D6-S1（版本检测）与 D6-S2（配置热更新）仍处于规划阶段。

---

## DSAFT 结构

| 层级 | 编号 | 名称 | 状态 |
|------|------|------|------|
| D | D6 | Evolution | Active |
| S | D6-S1 | Version | PLANNED |
| S | D6-S2 | Config | PLANNED |
| S | D6-S3 | Eval Engine | IMPLEMENTED |
| S | D6-S4 | Orchestration | IMPLEMENTED |

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
| EvalEngine | `eval/engine.go` | 评测管道编排：加载→抽样→探针→聚合→delta→基线 |
| JudgeManager | `eval/judge.go` | LLM-as-Judge 评分，双模型交叉验证，Cohen's kappa 校准 |
| DeltaAnalyzer | `eval/delta.go` | 当前评分 vs 基线对比，回归检测 |
| TuneGenerator | `eval/tune.go` | 回归维度 → 配置调优建议映射 |
| DatasetManager | `eval/dataset.go` | YAML 评测集加载、分层抽样、基线读写 |
| ProbeRegistry | `eval/probe.go` | 全局探针注册表，按 ID 查找 |
| GatewayLLMClient | `eval/gateway_llm.go` | 经 D3 LLM Gateway 的真实 Judge 调用 |
| StaticLLMClient | `eval/mock_llm.go` | 固定响应 Judge，用于测试/CLI |

### 7 类探针

| Probe ID | 文件 | 目标域 | 评分方式 | 说明 |
|----------|------|--------|----------|------|
| compression_recall | `eval/compression_recall_probe.go` | D2 | Judge + 确定性 | 压缩前后事实保留率（must_keep recall + LLM 语义评分） |
| tool_accuracy | `eval/tool_accuracy_probe.go` | D2 | 确定性 | Tool 选择 precision/recall/F1（expected_tools vs actual_tools） |
| provider_quality | `eval/provider_quality_probe.go` | D3 | Judge + 确定性 | 语义相似度（wordJaccard）+ 指令遵循率 + Judge 保守融合 |
| agent_forkjoin | `eval/agent_forkjoin_probe.go` | D4 | 确定性 | 子 Agent 消息隔离（isolation）+ Join 结果完整度 |
| path_regression | `eval/path_regression_probe.go` | D2 | 确定性 | 代码路径快照对比（runtime.Snapshot() LegacyHarness=0） |
| layer_violation | `eval/layer_violation_probe.go` | D6 | 确定性 | 分层违规扫描（0 违规→1.0, 1→0.5, 2+→0.0） |
| session_isolation | `eval/session_isolation_probe.go` | D6 | 确定性 | COW 隔离评估（fork/join/metadata 计数 + D5 交叉校验） |

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

## D6-S4: Orchestration

### 校验管道

```
Agent Decision → OrchestrationObserver → preFilter → Judge Validation → Intervention
                     (D4 observer)        (allowlist)   (cross-model)    (terminate/reroute)
```

### 核心组件

| 组件 | 文件 | 职责 |
|------|------|------|
| RuntimeOrchestrationValidator | `orchestration/validator.go` | 决策入口：预过滤→Judge 校验→干预触发 |
| InterventionExecutor | `orchestration/intervention.go` | 干预执行：terminate / terminateAndReroute / updateState |
| OrchestrationObserver | `orchestration/observer.go` | D4 AgentObserver 桥接，捕获 agent 决策事件 |
| RuntimeJudge | `orchestration/judge_adapter.go` | 经 D3 LLM Gateway 的跨模型决策校验 |

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

OpenTelemetry 指标（`orchestration/metrics.go`）：
- `orch_decisions_total` — 决策计数（按 category/risk）
- `orch_validations_total` — 校验计数（按 result）
- `orch_interventions_total` — 干预计数（按 action）
- `orch_judge_latency_seconds` — Judge 调用延迟
- `orch_observer_active` — Observer 活跃状态
- `orch_decisions_by_stage` — 各阶段决策分布

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

### D6-S4: Orchestration

<!-- D6-S4-A01-T01 -->
#### Requirement: 运行时决策校验
编排校验器必须对 Agent 路由决策进行 LLM 交叉验证。

**Scenario: 决策校验通过**
- GIVEN Agent 发出 tool_call 决策
- WHEN RuntimeOrchestrationValidator.OnDecision 被调用
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
编排观察器必须作为 D4 AgentObserver 桥接决策事件。

**Scenario: 捕获 Fork 决策**
- GIVEN agent.forked 事件
- WHEN OrchestrationObserver 收到事件
- THEN 构造 DecisionRecord{Category: fork}
- AND 送入 RuntimeOrchestrationValidator

<!-- D6-S4-A04-T01 -->
#### Requirement: 跨模型 Judge 适配
RuntimeJudge 必须经 D3 LLM Gateway 进行决策校验。

**Scenario: 校验请求**
- GIVEN DecisionRecord
- WHEN RuntimeJudge.ValidateDecision 被调用
- THEN 构造 JSON prompt 发送到 D3 Gateway
- AND 主模型失败时自动 fallback

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
  orchestration:
    enabled: false
    judge:
      primary_model: "deepseek-v4-pro"
      secondary_model: "minimax-M2.7-highspeed"
    prefilter:
      trusted_tools: ["read_file", "list_directory", "search_code"]
      min_interval_between_judges: "2s"
      max_judge_calls_per_minute: 10
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
