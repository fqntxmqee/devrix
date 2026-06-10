# Design: D6 自演化评测引擎（Eval Framework）

## 1. Root Cause Analysis

### 1.1 问题树

```
Devrix 能力升级无质量反馈
  ├── 无评测框架：质量评判靠人工，不可重复
  │   ├── compression 升级 → 不知道是否丢事实
  │   ├── PEV prompt 调优 → 不知道 tool 选择是否退化
  │   └── provider 切换 → 无法量化对比
  ├── 无评测集：评估素材靠临时构造
  │   ├── 每次要评估时重新造数据
  │   └── 不同时期的评测结果不可比较
  ├── 无评分标准：LLM-as-Judge 无校准
  │   ├── Judge 评分漂移不可感知
  │   └── 不同 Judge 模型评分不一致
  └── 无反馈闭环：评测结果不驱动系统改进
      ├── 参数调整靠经验
      └── 调整效果靠下一轮人工观察
```

### 1.2 根因

Devrix 现有体系（L5 测试 + D5 可观察）覆盖了"功能正确性"和"信号采集"，但缺少"质量评判"这一层。这不是某个实现缺陷，而是**架构层的缺失**——D6 自演化层目前只有 Version 和 Config 两个场景，缺少 Eval 这个核心环节。

### 1.3 为什么现在做

1. D2/D3/D4 均已成型，进入了持续优化阶段，需要量化反馈
2. D5 可观察已提供 incident export 等结构化信号输入
3. D3 LLM Gateway 提供了 LLM-as-Judge 的调用通道
4. Harness Preflight 规则引擎提供了确定性评分的可扩展基础

## 2. Solution Design

### 2.1 架构全景

```
┌─────────────────────────────────────────────────────────────┐
│  D6-S3 Eval Engine                                           │
│                                                               │
│  ┌──────────────────────────────────────────────────────┐   │
│  │  EvalRun (编排)                                       │   │
│  │                                                       │   │
│  │  1. Load EvalDataset (YAML)                          │   │
│  │  2. For each item → Probe.Run(item)                  │   │
│  │  3. Probe 内部:                                       │   │
│  │     ├─ JudgeManager.Score(item, rubric)              │   │
│  │     ├─ 或 DeterministicCheck(item)                    │   │
│  │     └─ 返回 DomainScore                               │   │
│  │  4. Aggregate scores by bucket                       │   │
│  │  5. DeltaAnalyzer.Compare(current, baseline)          │   │
│  │  6. (可选) TuneGenerator.Suggest(delta)            │   │
│  │  7. 返回 EvalReport                                   │   │
│  └──────────────────────────────────────────────────────┘   │
│                                                               │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐  │
│  │ Probe Runner │  │Judge Manager │  │ Delta Analyzer  │  │
│  │              │  │              │  │                  │  │
│  │  probes/     │  │ 主 Judge     │  │ 基线加载         │  │
│  │  comp_recall │  │ 反方 Judge   │  │ 逐维对比         │  │
│  │  pev_tool    │  │ 分歧仲裁     │  │ 分桶对比         │  │
│  │  provider    │  │ 月度校准     │  │ 趋势历史         │  │
│  │  forkjoin    │  │ kappa 计算   │  │                  │  │
│  └──────────────┘  └──────────────┘  └──────────────────┘  │
└─────────────────────────────────────────────────────────────┘
          ▲                              │
          │ 生产 trace 输入              │ delta 报告 + 调优建议
          │ (D5 incident export)          │
          │                              ▼
  ┌───────────────┐             ┌──────────────────┐
  │ Production    │             │ Human Decision   │
  │ Trace Pool    │             │ (调参 / 忽略)     │
  └───────────────┘             └──────────────────┘
```

### 2.2 核心设计决策

**决策 1：Probe 模式**
每个评测维度是一个 Probe，实现统一接口：

```go
type Probe interface {
    ID() string                    // "compression_recall"
    Run(ctx context.Context, item EvalItem, judge JudgeManager) (*DomainScore, error)
}
```

Probe 内部可组合使用 JudgeManager（语义评分）和确定性检查（格式匹配/F1）。新增维度只需实现该接口并注册。

**决策 2：Judge 不缓存，但可抽样**
每条评测用例单独调 Judge（不合并 batch），因为不同维度的 rubric 不同。但同一维度内可抽样——如果评测集有 500 条，可配置只评 100 条（stratified）。

**决策 3：基线存储在 git 管理的 YAML 中**
每次 EvalRun 产出评分后可选择"保存为基线"。基线与评测集版本绑定——评测集版本变了，旧基线失效。

```
openspec/eval-datasets/
├── v1/
│   ├── dataset.yaml        # 评测集（评测用例）
│   └── baseline.yaml       # 基线评分（可选）
├── v2/
│   ├── dataset.yaml
│   └── baseline.yaml
└── latest -> v2            # 当前活跃版本
```

**决策 4：评测引擎的生产者-消费者分离**
评测引擎只负责编排和评分。评测集的提取由独立脚本完成（`scripts/eval/extract-dataset.sh`），消费评测报告也由人工或外部工具完成。引擎本身不做"自动调参"决策。

### 2.3 配置设计

```yaml
evolution:
  eval:
    enabled: false                    # 全局开关
    judge:
      provider: "anthropic"           # 与 production 不同模型族
      model: "claude-sonnet-4-6"
      fallback_provider: "openai"     # 反方 Judge
      temperature: 0.0
      max_tokens: 2048
    dataset:
      path: "openspec/eval-datasets/latest"
    sampling:
      enabled: true                   # 是否抽样评分
      max_items: 200                  # 单次最多评 200 条
    calibration:
      enabled: true
      min_kappa: 0.6
      gold_set_path: "openspec/eval-datasets/calibration/gold-v1.yaml"
```

## 3. Key Interfaces / Types

### 3.1 核心类型

```go
// EvalOpts 评测运行参数
type EvalOpts struct {
    DatasetPath  string        // 评测集路径
    Sampling     *SamplingOpts // 抽样配置（nil=全量）
    SaveBaseline bool          // 是否保存基线
}

// EvalReport 评测报告
type EvalReport struct {
    ID          string         `json:"id" yaml:"id"`
    DatasetID   string         `json:"dataset_id" yaml:"dataset_id"`
    RunAt       time.Time      `json:"run_at" yaml:"run_at"`
    JudgeModel  string         `json:"judge_model" yaml:"judge_model"`
    Scores      []DomainScore  `json:"scores" yaml:"scores"`
    Dashboard   ScoreDashboard `json:"dashboard" yaml:"dashboard"`
    Delta       *EvalDelta     `json:"delta,omitempty" yaml:"delta,omitempty"`
    TuneSuggest []TuneSuggestion `json:"tune_suggest,omitempty" yaml:"tune_suggest,omitempty"`
}

// DomainScore 单维度评分
type DomainScore struct {
    Domain     string             `json:"domain" yaml:"domain"`
    Dimension  string             `json:"dimension" yaml:"dimension"`
    Score      float64            `json:"score" yaml:"score"`           // 0.0 - 1.0
    Confidence float64            `json:"confidence" yaml:"confidence"` // 0.0 - 1.0
    Buckets    map[string]float64 `json:"buckets,omitempty" yaml:"buckets,omitempty"` // 各分桶分数
    Details    map[string]float64 `json:"details,omitempty" yaml:"details,omitempty"` // 子维度分数
    JudgeLogs  []JudgeLog         `json:"judge_logs,omitempty" yaml:"judge_logs,omitempty"`
}

// ScoreDashboard 评分面板（一次性览）
type ScoreDashboard struct {
    OverallScore   float64           `json:"overall_score"`
    DimensionCount int               `json:"dimension_count"`
    ItemCount      int               `json:"item_count"`
    JudgeCost      TokenCost         `json:"judge_cost"`
    ByDomain       map[string]float64 `json:"by_domain"`
}

// EvalDelta 对比基线
type EvalDelta struct {
    BaselineID   string                `json:"baseline_id"`
    ByDimension  map[string]DeltaEntry `json:"by_dimension"`
    ByBucket     map[string]DeltaEntry `json:"by_bucket"`
    Regressions  []DeltaEntry           `json:"regressions"`  // delta < -0.02
}

// DeltaEntry 单条 delta
type DeltaEntry struct {
    Dimension    string  `json:"dimension"`
    Previous     float64 `json:"previous"`
    Current      float64 `json:"current"`
    Delta        float64 `json:"delta"`  // current - previous
    Severity     string  `json:"severity"` // "regression" | "improvement" | "stable"
}

// TuneSuggestion 调优建议
type TuneSuggestion struct {
    Target      string `json:"target"`       // "compression.budget" | "tool_pool.simple_mode"
    Reason      string `json:"reason"`
    CurrentVal  string `json:"current_val"`
    SuggestedVal string `json:"suggested_val"`
    Confidence  string `json:"confidence"`   // "high" | "medium" | "low"
}
```

### 3.2 Judge 管理器

```go
type JudgeManager interface {
    Score(ctx context.Context, item EvalItem, rubric ScoreRubric) (*JudgeScore, error)
    Calibrate(ctx context.Context, goldSet []GoldLabel) (*CalibrationReport, error)
    Close() error
}

type JudgeScore struct {
    Score      float64           `json:"score"`
    Confidence float64           `json:"confidence"`
    Reasoning  string            `json:"reasoning"`
    Details    map[string]float64 `json:"details"`
    TokenUsage TokenCost         `json:"token_usage"`
}

type ScoreRubric struct {
    Dimension   string  `json:"dimension"`
    Instruction string  `json:"instruction"`     // Judge 指令
    Scale       string  `json:"scale"`           // "0-1", "1-5", "pass/fail"
    Reference   string  `json:"reference"`       // 参考示例
}

type GoldLabel struct {
    ItemID     string            `json:"item_id"`
    HumanScore float64           `json:"human_score"`
    Reason     string            `json:"reason"`
    Tags       map[string]string `json:"tags"`
}

type CalibrationReport struct {
    Kappa        float64 `json:"kappa"`
    JudgeModel   string  `json:"judge_model"`
    GoldSetSize  int     `json:"gold_set_size"`
    Passed       bool    `json:"passed"` // kappa >= minKappa
    LastCalibrated time.Time `json:"last_calibrated"`
}
```

### 3.3 评测集类型

```go
type EvalDataset struct {
    ID        string      `yaml:"id"`
    Version   string      `yaml:"version"`
    CreatedAt time.Time   `yaml:"created_at"`
    Buckets   []BucketDef `yaml:"buckets"`
    Items     []EvalItem  `yaml:"items"`
}

type EvalItem struct {
    ID          string         `yaml:"id"`
    Bucket      string         `yaml:"bucket"`       // "production"|"adversarial"|"edge"|"failure"
    Domain      string         `yaml:"domain"`       // "d2"|"d3"|"d4"
    Dimension   string         `yaml:"dimension"`    // "compression_recall"|...
    Input       map[string]any `yaml:"input"`        // 评测输入
    Expectation map[string]any `yaml:"expectation"`  // 期望结果
    RubricRef   string         `yaml:"rubric_ref"`    // 关联 rubric
    Tags        map[string]string `yaml:"tags,omitempty"`
}
```

### 3.4 Probe 接口

```go
type Probe interface {
    ID() string
    Run(ctx context.Context, item EvalItem, judge JudgeManager) (*DomainScore, error)
}
```

内置 Probe 注册表：

```go
var Probes = map[string]Probe{
    "compression_recall":   &CompressionRecallProbe{},
    "pev_tool_accuracy":    &PEVToolAccuracyProbe{},
    "provider_quality":     &ProviderQualityProbe{},
    "agent_forkjoin":       &AgentForkJoinProbe{},
}
```

### 3.5 Configuration

```go
type EvalConfig struct {
    Enabled    bool            `yaml:"enabled"`       // 默认 false
    Judge      JudgeConfig     `yaml:"judge"`
    Dataset    DatasetConfig   `yaml:"dataset"`
    Sampling   SamplingConfig  `yaml:"sampling"`
    Calibration CalibrationConfig `yaml:"calibration"`
}

type JudgeConfig struct {
    Provider         string  `yaml:"provider"`
    Model            string  `yaml:"model"`
    FallbackProvider string  `yaml:"fallback_provider"`  // 反方 Judge
    Temperature      float64 `yaml:"temperature"`         // 0.0
    MaxTokens        int     `yaml:"max_tokens"`
}
```

## 4. Data Flow

### 4.1 EvalRun 核心流程

```
EvalRun(ctx, dataset, opts)
│
├─ 1. 加载评测集 YAML
│
├─ 2. 应用抽样（如有）
│   └─ StratifiedSample(items, opts.Sampling)
│
├─ 3. For each item in items:
│   ├─ 3a. 查找 Probe: Probes[item.Dimension]
│   ├─ 3b. 加载 Rubric（rubric 决定了 Judge 指令和评分标准）
│   ├─ 3c. JudgeManager.Score(item, rubric)
│   │   ├─ 构建 Judge prompt（含 item input + rubric + 参考示例）
│   │   ├─ 调 D3 LLM Gateway → 获取评分
│   │   ├─ Position randomization（两次评分取平均）
│   │   └─ 返回 JudgeScore
│   ├─ 3d. 分歧检测（反方 Judge）
│   │   └─ 如主/反方差 > 1σ → ResolveDispute → 人工仲裁队列
│   ├─ 3e. Probe.Run(item, judgeScore) → DomainScore
│   └─ 3f. 累加到 Aggregator
│
├─ 4. Aggregator.Finalize()
│   └─ 按 domain/dimension/bucket 聚合，计算平均 / P50 / P95
│
├─ 5. DeltaAnalyzer.Compare(scores, baseline)
│   └─ 加载基线（同 dataset ID 的上次保存结果）
│   └─ 逐维对比 → 标记 regression / improvement / stable
│
├─ 6. (可选) TuneGenerator.Suggest(delta)
│   └─ 预定义规则 → 生成调优建议
│
└─ 7. 返回 EvalReport
```

### 4.2 评测集提取流程

```
scripts/eval/extract-dataset.sh
│
├─ 1. 从 D5 incident export 读取生产 trace（JSONL）
├─ 2. 聚类（按意图 embedding + tool 调用序列）
├─ 3. 按四桶比例取样（60/15/15/10）
├─ 4. 自动标注：
│   ├─ compression: ["压缩前 context", "压缩后 context"]
│   └─ 标注 P0 事实列表（LLM-as-Judge 辅助提取）
├─ 5. 输出待审核 YAML
├─ 6. 人工审核 → 合入 openspec/eval-datasets/
└─ 7. git commit + tag
```

### 4.3 校准流程

```
Calibrate(goldSet)
│
├─ 1. JudgeManager 对 goldSet 中每条评分
├─ 2. 计算 Judge Score vs Human Label 的 Cohen's kappa
├─ 3. kappa >= 0.6 → 校准通过
├─ 4. kappa < 0.6 → 分析失败模式
│   ├─ (a) 调整 Judge prompt（rubric 不够清晰）
│   ├─ (b) 增加 rubric 中的参考示例
│   ├─ (c) 切换 Judge 模型
│   └─ (d) 重复 1-4
└─ 5. 记录校准报告
```

## 5. File Manifest

### 新增文件

```
internal/layers/evolution/eval/
├── engine.go              # EvalRun 编排核心
├── engine_test.go         # 编排测试（含 enabled=false）
├── judge.go               # JudgeManager 实现
├── judge_test.go          # Judge 测试（含校准）
├── dataset.go             # EvalDataset 加载/版本化
├── dataset_test.go        # 评测集加载测试
├── delta.go               # DeltaAnalyzer
├── delta_test.go          # delta 分析测试
├── tune.go                # TuneGenerator（Pilot 版本）
├── tune_test.go           # 调优建议测试
├── types.go               # 所有核心类型定义
├── types_test.go          # 类型序列化/校验测试
├── judge_rubric.go        # Rubric 定义与加载
├── judge_rubric_test.go
├── judge_calibrate.go     # 校准逻辑
├── judge_calibrate_test.go
│
└── probes/
    ├── probe.go           # Probe 接口 + 注册表
    ├── compression_recall.go      # D2-E1 Compression Recall Probe
    ├── compression_recall_test.go
    ├── pev_tool_accuracy.go       # D2-E2 PEV Tool 准确率（Phase 2）
    ├── pev_tool_accuracy_test.go
    ├── provider_quality.go        # D3-E1 Provider 质量（Phase 3）
    ├── provider_quality_test.go
    ├── agent_forkjoin.go          # D4-E2 Fork/Join 质量（Phase 3）
    └── agent_forkjoin_test.go

scripts/eval/
├── extract-dataset.sh     # 评测集自动抽取
└── run-eval.sh            # 命令行跑评测

openspec/eval-datasets/
├── README.md              # 评测集使用说明
├── v1/
│   ├── dataset.yaml       # 初版评测集
│   └── baseline.yaml      # 初版基线
├── calibration/
│   └── gold-v1.yaml       # 人工标注校准集
└── rubrics/
    ├── compression_recall.yaml    # Compression Recall rubric
    ├── pev_tool_accuracy.yaml     # PEV Tool rubric
    ├── provider_quality.yaml      # Provider rubric
    └── agent_forkjoin.yaml        # Fork/Join rubric
```

### 修改文件

- `internal/layers/evolution/`：无冲突，新增 eval/ 子包
- `openspec/l5-registry.md`：已新增 D6-S3 条目

### 无修改

- `internal/layers/contextengine/`（D2）
- `internal/layers/llmgateway/`（D3）
- `internal/layers/multiagent/`（D4）
- `internal/layers/observability/`（D5）

## 6. Regression Risk Assessment

| 风险 | 影响范围 | 概率 | 缓解 |
|------|----------|------|------|
| 评测引擎与 D5 信号格式耦合 | 仅 Eval 自身 | 中 | 依赖 incident export 的稳定 schema，非内部私有类型 |
| LLM-as-Judge 调用影响 D3 配额 | D3 Gateway | 低 | Judge 走独立 provider config，不竞争 production 配额 |
| 评测集 YAML 损坏 | 仅 Eval 自身 | 低 | 加载时 schema 校验 + 单元测试 |
| 基线数据丢失 | 仅 delta 报告 | 低 | 基线存 git，可恢复 |
| Pilot 阶段维度覆盖不足导致框架设计偏差 | 后续扩展可能需重构 | 中 | Pilot 验证通过后立即扩展第二维度验证通用性 |
| Judge 评分漂移导致误报 regression | 误导调参决策 | 中 | 双 Judge + 月度校准 + delta 报告标注置信度 |

## 7. Rollback Plan

| 场景 | 操作 | 恢复时间 |
|------|------|----------|
| eval 配置导致 Process 异常 | `evolution.eval.enabled: false` 立即生效 | < 1 分钟 |
| 评测集 YAML schema 不兼容 | git revert 回退评测集版本 | < 5 分钟 |
| Judge 模型不可用 | 切换 `judge.provider` 配置 | < 5 分钟 |
| delta 报告误导决策 | 人工核对校准报告，确认 kappa | 即时（查看报告） |
| Framework 设计需要重构 | Pilot 阶段代码量小，重构成本低 | — |
