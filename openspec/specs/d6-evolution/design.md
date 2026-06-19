# D6 Evolution — 架构设计

**Domain:** D6 Evolution
**DSAFT Type:** Supporting
**Version:** 2.2.0
**Last Updated:** 2026-06-19
**Status:** Active
**Parent:** `openspec/specs/d6-evolution/spec.md`

> **v2.2.0 状态**（DM-20260619-003 同步）：v2.0 物理路径迁移已完成（DM-20260615-003, 2026-06-15），`eval/` → `evaluate/`、`orchestration/` → `guard/`、`exporter/` → `export/`、新增 `verify/`。`bridge.go` 桥接文件在 v2.0.1 cleanup 后全部删除（11 个）。

---

## 目录结构

```
internal/layers/evolution/
├── evaluate/                                 # D6-S3 评测引擎（v2.0 改名前 eval/）
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
├── guard/                                    # D6-S4 Guard 韧性（v2.0 改名前 orchestration/）
│   ├── validator.go                         # RuntimeGuardValidator
│   ├── intervention.go                      # InterventionExecutor
│   ├── observer.go                          # GuardObserver
│   ├── judge_adapter.go                     # RuntimeJudge — 跨模型校验
│   ├── types.go                             # DecisionRecord, ValidationResult 等
│   ├── config.go                            # 配置类型别名
│   ├── metrics.go                           # OpenTelemetry 指标
│   └── validator_test.go                    # 校验器测试
└── verify/                                   # D6-S5 Invariant 验证（v2.0 新增物理独立）
    ├── _invariant.go                        # Invariant 接口 + 注册表
    └── plan.go                              # VerifyPlan — 验证计划编排
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

## D6-S4: GuardRuntime

### 校验管道

```
RuntimeGuardValidator.OnDecision(ctx, rec, session)
  ├─ enabled=false: return immediately
  ├─ Start tracing span (`D6_Validation_Decision`)
  ├─ Record metrics (guard_decisions_total)
  ├─ preFilter(rec):
  │   ├─ Trusted tool allowlist match → skip (guard_decisions_by_stage: prefilter_skip)
  │   ├─ Min interval since last judge → skip
  │   └─ Max calls per minute exceeded → skip
  ├─ judge.ValidateDecision(ctx, rec) → *ValidationResult
  ├─ Record metrics (guard_validations_total, guard_judge_latency_seconds)
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

### GuardObserver 事件桥接

实现 `multiagent.AgentObserver`，捕获两类事件：

| AgentEvent | → DecisionCategory | RiskClass |
|------------|-------------------|-----------|
| "permission_required" | permit | Critical |
| "agent.forked" | fork | Evaluate |

### 配置

```go
type GuardConfig struct {
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

## D6-S5: VerifyInvariant

Invariant 验证子系统，v2.0 物理独立（从 evaluate/ 拆出）。

### 验证管道

```
VerifyPlan.Run(ctx, target)
  ├─ Load invariant set from registry
  ├─ For each invariant:
  │     invariant.Check(ctx, target) → *InvariantResult
  ├─ Aggregate → *VerifyReport
  └─ If fail: emit Guard event (D6-S4 联动)
```

### 核心接口

```go
// Invariant — 不变量检查器接口
type Invariant interface {
    ID() string
    Check(ctx context.Context, target Target) (*InvariantResult, error)
}

// VerifyPlan — 验证计划
type VerifyPlan struct {
    Invariants []Invariant
    OnFail     func(*InvariantResult)
}
```

### 与 D6-S4 Guard 联动

当 VerifyPlan 检测到 invariant 失败时，emit Guard 事件：
- `DecisionCategory = "invariant_violation"`
- `RiskClass = Critical`
- 由 `RuntimeGuardValidator` 走相同校验管道

---

## 依赖关系

```
D6 Evolution
  ├── D2 Context Engine  (compression_recall, path_regression 探针目标)
  ├── D3 LLM Gateway     (GatewayLLMClient, RuntimeJudge 底层)
  ├── D4 Multi-Agent     (agent_forkjoin 探针目标, GuardObserver 事件源)
  ├── D5 Observability   (session_isolation 交叉校验, OpenTelemetry 集成)
  └── D7 Orchestration   (InterventionExecutor 状态变更)
```

---

## 修订历史

| 版本 | 日期 | 变更摘要 | 关联 DM |
|------|------|---------|---------|
| 2.2.0 | 2026-06-19 | v2.0 物理路径迁移同步：eval→evaluate / orchestration→guard / 新增 verify/；RuntimeOrchestrationValidator→RuntimeGuardValidator；新增 D6-S5 VerifyInvariant 章节 | DM-20260619-003 |
| 2.1.0 | 2026-06-14 | path_regression + layer_violation + session_isolation 三个新探针 | DM-20260614-XXX |
| 2.0.0 | 2026-06-XX | 初版 | — |

---

## 相关文档

- [spec.md](./spec.md) — D6 域规范
- [layer-delta.md](./layer-delta.md) — 层增量变更记录
- [d6-domain.md](./d6-domain.md) — D6 域描述 + 价值流 + 跨域契约
- [a-registry.md](./a-registry.md) — A 层活动注册表
- [f-registry.md](./f-registry.md) — F 层功能点注册表
- [t-registry.md](./t-registry.md) — T 层测试点注册表
