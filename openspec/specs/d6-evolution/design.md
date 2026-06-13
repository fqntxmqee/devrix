# D6 Evolution — 架构设计

**Domain:** D6 Evolution
**DSAFT Type:** Supporting
**Version:** 2.1.0
**Last Updated:** 2026-06-14
**Status:** Active
**Parent:** `openspec/specs/d6-evolution/spec.md`

---

## 目录结构

```
internal/layers/evolution/
├── eval/                                    # D6-S3 评测引擎
│   ├── engine.go                            # EvalEngine — 评测管道编排
│   ├── types.go                             # 所有类型定义
│   ├── probe.go                             # Probe 接口 + 全局注册表
│   ├── judge.go                             # JudgeManager — LLM-as-Judge
│   ├── delta.go                             # DeltaAnalyzer — 回归检测
│   ├── gate.go                              # CI Delta 门禁
│   ├── tune.go                              # TuneGenerator — 调优建议
│   ├── dataset.go                           # Dataset 加载/抽样/基线
│   ├── gateway_llm.go                       # GatewayLLMClient — 经 D3 的 LLM 调用
│   ├── mock_llm.go                          # StaticLLMClient — 测试用固定响应
│   ├── probe_helpers.go                     # 探针辅助函数
│   ├── compression_recall_probe.go          # Probe: compression_recall
│   ├── tool_accuracy_probe.go               # Probe: tool_accuracy
│   ├── provider_quality_probe.go            # Probe: provider_quality
│   ├── agent_forkjoin_probe.go              # Probe: agent_forkjoin
│   ├── path_regression_probe.go             # Probe: path_regression (v2.1.0)
│   ├── layer_violation_probe.go             # Probe: layer_violation (v2.1.0)
│   ├── session_isolation_probe.go           # Probe: session_isolation (v2.1.0)
│   └── *_test.go                            # 14 个测试文件
└── orchestration/                           # D6-S4 编排校验
    ├── validator.go                         # RuntimeOrchestrationValidator
    ├── intervention.go                      # InterventionExecutor
    ├── observer.go                          # OrchestrationObserver
    ├── judge_adapter.go                     # RuntimeJudge — 跨模型校验
    ├── types.go                             # DecisionRecord, ValidationResult 等
    ├── config.go                            # 配置类型别名
    ├── metrics.go                           # OpenTelemetry 指标
    └── validator_test.go                    # 校验器测试
```

---

## D6-S3: Eval Engine

### 管道编排 (EvalEngine)

```
EvalEngine.Run(ctx, opts)
  ├─ 1. Load dataset (YAML → *EvalDataset)
  ├─ 2. StratifiedSample (按 bucket 比例抽样)
  ├─ 3. For each item:
  │     probe.Run(ctx, item, judge) → *DomainScore
  ├─ 4. Aggregate: scoresByDimension → *EvalReport
  ├─ 5. If baseline set: DeltaAnalyzer.Compare(report)
  │     └─ If regressions: TuneGenerator.Suggest(delta)
  └─ 6. If SaveBaseline: SaveBaseline(path, report)
```

### 核心接口

```go
// Probe — 单维度评分器接口
type Probe interface {
    ID() string
    Run(ctx context.Context, item EvalItem, judge Judge) (*DomainScore, error)
}

// LLMClient — LLM 调用抽象（支持 mock 和真实 gateway）
type LLMClient interface {
    Chat(ctx context.Context, model string, systemPrompt string, userMsg string,
        temperature float64, maxTokens int) (string, TokenCost, error)
}
```

### 全局探针注册表

```go
var probeRegistry = map[string]Probe{}

func RegisterProbe(p Probe) { probeRegistry[p.ID()] = p }
func GetProbe(id string) Probe { return probeRegistry[id] }
```

7 个内置探针在 `init()` 中自动注册。

### JudgeManager 评分流程

```
Score(ctx, item, rubric)
  ├─ judgeOnce(primary, forward)   → primary score
  ├─ judgeOnce(primary, reversed)  → reversed score (position randomization)
  ├─ avg = (forward + reversed) / 2
  ├─ If secondary exists:
  │   ├─ judgeOnce(secondary, forward) → secondary score
  │   └─ If |avg - secondary| > 0.5: dispute detected, confidence = min(avg.conf, 0.3)
  └─ return avg
```

**Cohen's kappa 校准**：`Calibrate(goldSet, rubric)` 在人类标注集上计算 kappa 值（nBins=5 离散化）。

### DeltaAnalyzer 回归阈值

| Severity | 条件 | 含义 |
|----------|------|------|
| SeverityImprovement | Δ > +0.02 | 质量提升 |
| SeverityStable | -0.02 ≤ Δ ≤ +0.02 | 稳定 |
| SeverityRegressionYellow | -0.05 ≤ Δ < -0.02 | 轻微回归 |
| SeverityRegressionRed | Δ < -0.05 | 严重回归 |

### 7 类探针

| Probe | 评分方式 | 依赖 | 输出指标 |
|-------|----------|------|----------|
| compression_recall | Judge score × must_keep recall | D2 | recall, similarity |
| tool_accuracy | 确定性 precision/recall/F1 | — | precision, recall, f1 |
| provider_quality | conservativeMin(deterministic, judge) | D3 | semantic_sim, instruction_following |
| agent_forkjoin | 确定性 isolation/completeness | D4 | isolation_rate, completeness_rate |
| path_regression | 确定性 (runtime.Snapshot) | D2 | legacy_harness |
| layer_violation | 确定性 (layer-lint scanner) | D6 | violation_count |
| session_isolation | 确定性 (COW counters) | D5, D6 | fork_count, join_count, violations |

---

## D6-S4: Orchestration

### 校验管道

```
RuntimeOrchestrationValidator.OnDecision(ctx, rec, session)
  ├─ enabled=false: return immediately
  ├─ Start tracing span ("orch.OnDecision")
  ├─ Record metrics (orch_decisions_total)
  ├─ preFilter(rec):
  │   ├─ Trusted tool allowlist match → skip (orch_decisions_by_stage: prefilter_skip)
  │   ├─ Min interval since last judge → skip
  │   └─ Max calls per minute exceeded → skip
  ├─ judge.ValidateDecision(ctx, rec) → *ValidationResult
  ├─ Record metrics (orch_validations_total, orch_judge_latency_seconds)
  ├─ If valid OR confidence ≥ InterventionThreshold: return
  └─ Build Intervention → if AutoIntervene: executor.Execute(ctx, iv, session)
```

### 核心类型

```go
type DecisionCategory string  // tool_call | permit | fork
type RiskClass int            // Low (0) | Evaluate (1) | Critical (2)

type DecisionRecord struct {
    ID, SessionID, AgentID, ParentAgentID string
    Category    DecisionCategory
    RiskClass   RiskClass
    ToolName    string
    TargetAgentID string
    ForkConfig  *multiagent.AgentConfig
}

type ValidationResult struct {
    DecisionID, Reasoning     string
    Valid            bool
    Confidence       float64
    SuggestedAction  string  // "terminate" | "reroute" | "update_state"
    SuggestedAgentID string
    Duration         time.Duration
}

type Intervention struct {
    DecisionID, Action, Reason, TargetAgentID string
    AgentConfig  *multiagent.AgentConfig
    MilestoneFail, TaskFail bool
    FailReason   string
}
```

### InterventionExecutor 动作

```
Execute(ctx, iv, session)
  ├─ "terminate": agent.Terminate(ctx)
  ├─ "reroute":
  │   ├─ current.Terminate + Wait
  │   ├─ If MilestoneFail/TaskFail: tasks.Fail(id, reason)
  │   └─ factory.Create → RegisterSessionAgent → Run
  └─ "update_state": tasks.Fail(sessionID, reason)
```

### OrchestrationObserver 事件桥接

实现 `multiagent.AgentObserver`，捕获两类事件：

| AgentEvent | → DecisionCategory | RiskClass |
|------------|-------------------|-----------|
| "permission_required" | permit | Critical |
| "agent.forked" | fork | Evaluate |

### 配置

```go
type OrchestrationConfig struct {
    Enabled                bool
    AutoIntervene          bool
    PreFilterEnabled       bool
    InterventionThreshold  float64
    TrustedToolAllowlist   []string
    MinIntervalBetweenJudges time.Duration
    MaxJudgeCallsPerMinute int
    Judge                  JudgeConfig
}
```

---

## 依赖关系

```
D6 Evolution
  ├── D2 Context Engine  (compression_recall, path_regression 探针目标)
  ├── D3 LLM Gateway     (GatewayLLMClient, RuntimeJudge 底层)
  ├── D4 Multi-Agent     (agent_forkjoin 探针目标, OrchestrationObserver 事件源)
  ├── D5 Observability   (session_isolation 交叉校验, OpenTelemetry 集成)
  └── D7 Orchestration   (InterventionExecutor 状态变更)
```

---

## 相关文档

- [spec.md](./spec.md) — D6 域规范
- [layer-delta.md](./layer-delta.md) — 层增量变更记录
- [a-registry.md](./a-registry.md) — A 层活动注册表
- [f-registry.md](./f-registry.md) — F 层功能点注册表
- [t-registry.md](./t-registry.md) — T 层测试点注册表
