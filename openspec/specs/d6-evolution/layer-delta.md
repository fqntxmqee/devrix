# Delta: Domain D6 (Evolution)

**Change ID:** devrix-d6-evolution
**Affects:** evolution layer — eval engine, orchestration validator
**Last Updated:** 2026-06-14

---

## V1 (devrix-foundation)

### ADDED

#### Requirement: Placeholder for V1

V1 仅包含最小演化层占位。

**Scenario: Report metric (V1)**
- GIVEN 系统产生各种事件
- WHEN 可观测性层发出指标
- THEN 演化层无活跃处理
- AND 输出占位日志: "Evolution layer not yet active"

---

## V2 (2026-06-10, devrix-d6-eval-phase3)

### ADDED

#### D6-S3: Eval Engine (Pilot)

完整评测管道实现。

| 组件 | 文件 | 说明 |
|------|------|------|
| EvalEngine | `eval/engine.go` | 评测管道编排：加载→抽样→探针→聚合→delta→基线 |
| JudgeManager | `eval/judge.go` | LLM-as-Judge 双模型交叉验证，Cohen's kappa 校准 |
| DeltaAnalyzer | `eval/delta.go` | 当前 vs 基线回归检测（Red: <-5%, Yellow: <-2%） |
| TuneGenerator | `eval/tune.go` | 回归维度 → 配置调优建议 |
| DatasetManager | `eval/dataset.go` | YAML 评测集加载、分层抽样、基线读写 |
| ProbeRegistry | `eval/probe.go` | 全局探针注册表 |
| GatewayLLMClient | `eval/gateway_llm.go` | 经 D3 Gateway 的真实 LLM Judge |
| StaticLLMClient | `eval/mock_llm.go` | 固定响应 Judge（测试/CLI） |
| CI Delta Gate | `eval/gate.go` | CheckDeltaGate + FormatDeltaSummary |
| Probe Helpers | `eval/probe_helpers.go` | wordJaccard, instructionFollowingRate 等 |

4 类初始探针：

| Probe | 文件 | 目标域 | 评分方式 |
|-------|------|--------|----------|
| CompressionRecallProbe | `eval/compression_recall_probe.go` | D2 | Judge + 确定性 |
| ToolAccuracyProbe | `eval/tool_accuracy_probe.go` | D2 | 确定性 |
| ProviderQualityProbe | `eval/provider_quality_probe.go` | D3 | Judge + 确定性 |
| AgentForkJoinProbe | `eval/agent_forkjoin_probe.go` | D4 | 确定性 |

CLI 子命令：`devrix eval run`

CI 脚本：`scripts/eval/run-eval.sh`

---

## V2.1 (2026-06-14)

### ADDED

#### D6-S3: 3 个新探针

| Probe | 文件 | 目标域 | 评分方式 | 说明 |
|-------|------|--------|----------|------|
| PathRegressionProbe | `eval/path_regression_probe.go` | D2 | 确定性 | 代码路径快照对比（LegacyHarness=0 检测） |
| LayerViolationProbe | `eval/layer_violation_probe.go` | D6 | 确定性 | 分层违规扫描（0→1.0, 1→0.5, 2+→0.0） |
| SessionIsolationProbe | `eval/session_isolation_probe.go` | D6 | 确定性 | COW 隔离评估 + D5 交叉校验 |

#### D6-S4: Orchestration 运行时校验

全新子系统，包含 4 个核心组件：

| 组件 | 文件 | 说明 |
|------|------|------|
| RuntimeOrchestrationValidator | `orchestration/validator.go` | 决策入口：preFilter → Judge → Intervention |
| InterventionExecutor | `orchestration/intervention.go` | 干预执行：terminate / reroute / update_state |
| OrchestrationObserver | `orchestration/observer.go` | D4 AgentObserver 桥接，捕获 fork/permit 事件 |
| RuntimeJudge | `orchestration/judge_adapter.go` | 经 D3 Gateway 的跨模型决策校验 |

支持特性：
- 预过滤器（可信工具允许列表、最小 Judge 间隔、最大 Judge 速率）
- 三种干预动作（terminate、terminateAndReroute、updateState）
- OpenTelemetry 追踪 + 指标（6 个指标计数器）
- 可注入 Hooks（DecisionHook、ValidationHook、InterventionHook）
- 配置开关（`evolution.orchestration.enabled`，默认 false）

#### 配置扩展

```yaml
evolution:
  orchestration:
    enabled: false
    auto_intervene: false
    prefilter:
      enabled: true
      trusted_tools: ["read_file", "list_directory", "search_code"]
      min_interval_between_judges: "2s"
      max_judge_calls_per_minute: 10
    judge:
      primary_model: "deepseek-v4-pro"
      secondary_model: "minimax-M2.7-highspeed"
```

### MODIFIED

- **EvalEngine**：`WithProbeRegistry` 支持注入自定义探针注册表；`WithBaseline` 设置基线
- **JudgeManager**：位置随机化（forward+reversed averaging）优化评分一致性
- **DeltaAnalyzer**：微调回归阈值常量命名（RegressionRedThreshold / RegressionYellowThreshold）
- **TuneGenerator**：新增 path_regression、layer_violation、session_isolation 维度的调优建议

### REMOVED

(None — V2.1 为纯增量)
