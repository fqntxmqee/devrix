# Delta: Domain D6 (Evolution)

**Change ID:** devrix-d6-evolution
**Affects:** evolution layer — eval engine, guard validator, verify invariant
**Last Updated:** 2026-06-19

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

> **v2.0 物理路径迁移后**（见下文 V2.2）：`orchestration/` → `guard/`，`RuntimeOrchestrationValidator` → `RuntimeGuardValidator`，`OrchestrationObserver` → `GuardObserver`，指标名 `orch_*` → `guard_*`。

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

---

## V2.2 (2026-06-15, devrix-d6-evolution-v2.0-physical-paths)

> docs-only 同步条目（DM-20260619-003 同步到 layer-delta.md）。代码 v2.0 已于 2026-06-15 落地（DM-20260615-003），本章节仅为路径迁移的事实记录。

### ADDED — D6 v2.0 物理路径迁移

v2.0 物理路径迁移落地 3 包重命名 + 1 包新增：

| 旧路径 | 新路径 | 原因 |
|--------|--------|------|
| `eval/` | `evaluate/` | 与 D3 evaluate/ 命名对齐；避免 eval 关键字歧义 |
| `orchestration/` | `guard/` | 与 D7 orchestration/ 同名冲突；guard 更准确反映"决策入口 + 干预触发"职责 |
| `exporter/` | `export/` | 命名统一（其他域均无 -er 后缀） |
| (无) | `verify/` | Invariant 验证从 evaluate 物理独立 |

#### Scenario: 路径迁移完整

- GIVEN 6 个重命名/新增子包
- WHEN v2.0 落地（DM-20260615-003, 2026-06-15）
- THEN `internal/layers/evolution/` 含 `eval/ + evaluate/ + guard/ + orchestration/ + verify/ + export/ + exporter/ + spans.go`（含 bridge 桥接）
- AND bridge.go 在 v2.0.1 cleanup 后全部删除（11 个 bridge.go 移除）

#### Scenario: guard 误删恢复

- GIVEN guard 子包曾因 orchestration→guard 重命名误删
- WHEN 42bf1d7 提交恢复
- THEN guard 子包 7 个 .go 文件完整存在（validator.go / intervention.go / observer.go / judge_adapter.go / types.go / config.go / metrics.go）
- AND spec `d6-domain.md` §历史留痕 显式标注该事件
- AND `validator_test.go` 在恢复范围内

#### Scenario: D6-S4 名称与组件映射

- GIVEN v2.0 路径迁移
- WHEN spec 文档同步
- THEN 章节名 `D6-S4: Orchestration` → `D6-S4: GuardRuntime`
- AND 组件 `RuntimeOrchestrationValidator` → `RuntimeGuardValidator`
- AND 组件 `OrchestrationObserver` → `GuardObserver`
- AND 配置 `evolution.orchestration.*` → `evolution.guard.*`
- AND 指标名 `orch_*` → `guard_*`（6 个指标计数器）

#### Scenario: D6-S5 VerifyInvariant 物理独立

- GIVEN v2.0 新增 `verify/` 子包
- WHEN spec 同步
- THEN 新增章节 `D6-S5: VerifyInvariant`
- AND 列出 `_invariant.go` (Invariant 接口 + 注册表) + `plan.go` (VerifyPlan 编排)
- AND 与 D6-S4 Guard 联动：invariant 失败时 emit `DecisionCategory = "invariant_violation"`, `RiskClass = Critical`
