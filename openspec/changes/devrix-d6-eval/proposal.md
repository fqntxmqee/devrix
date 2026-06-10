# Proposal: D6 自演化评测引擎（Eval Framework）

**Change ID:** devrix-d6-eval
**Demand ID:** DM-20260610-006
**Status:** S2_Proposal
**Based on:** `demand.md`（DM-20260610-006），业界实践参考 MLflow Trace-Aware Evaluation、AEMA、MemAlign、FutureAGI CI/CD for AI Agents 2026

---

## 1. Background

Devrix 经过多轮能力升级（D2 Context Engine V1-V5、D3 LLM Gateway、D4 Multi-Agent、D5 Observability），已具备完整的对话认知管线。但缺少一个关键环节：**量化质量与检测退化**。

当前的状态：

- **L5 测试**覆盖功能正确性（110+ 测试点），但 pass/fail 无法反映质量连续变化
- **D5 可观察**记录 trace/metrics/log，但"信号采集"不等于"质量评判"
- **配置调优**依赖人工经验——compression budget、tool_pool 裁剪、provider fallback 权重等参数的调整没有量化反馈

业界 2025-2026 的共识是：生产级 AI 系统需要 **trace-aware、multi-dimensional、process-level 的评估**，且评估结果应反馈到系统的持续优化中（MLflow、AEMA、Amazon 三层框架）。

## 2. Problem Statement

### 2.1 核心矛盾

Devrix 的能力在持续增长，但缺乏回答以下问题的能力：

| 问题 | 影响 | 当前手段 |
|------|------|----------|
| "这次 compression 升级后，关键信息的保留率是多少？" | 可能丢掉事实而不自知 | 仅看 token 减少量 |
| "PEV prompt 修改后，tool 选择准确率变化了多少？" | 推理质量退化直到用户反馈才发现 | 人工观察 |
| "新 provider 比旧的好多少？" | Provider 切换决策靠感觉 | 无结构化对比 |
| "Fork/Join 重构后，子 Agent 的消息隔离是否依旧可靠？" | 回归问题被 L5 测试覆盖，但质量趋势不可见 | 仅 pass/fail |

### 2.2 需求范围

D6 Eval 要解决的是**质量可量化 + 退化可检测 + 结果可反馈**，与现有体系互补而非替代：

```
L5 测试（功能正确性） + D5 可观察（信号采集） + D6 Eval（质量评分+自演化）
```

## 3. Proposed Solution

### 3.1 架构总览

```
┌──────────────────────────────────────────────────┐
│                D6 自演化层                         │
│                                                    │
│  ┌──────────────┐    ┌──────────────────┐         │
│  │ D6-S1 Version│    │  D6-S2 Config    │         │
│  └──────────────┘    └──────────────────┘         │
│                                                    │
│  ┌──────────────────────────────────────────────┐  │
│  │  D6-S3 Eval Engine                           │  │
│  │                                              │  │
│  │  EvalRun(ctx, dataset, opts) → EvalReport    │  │
│  │       │                                       │  │
│  │  ┌────┴─────────┬──────────┬──────────┐      │  │
│  │  │ Probe Runner │ Judge    │ Delta    │      │  │
│  │  │ (各维度探针)  │ Mgr      │ Analyzer │      │  │
│  │  └────┬─────────┴──────────┴──────────┘      │  │
│  │       │                                       │  │
│  │       └── D3 LLM Gateway (Judge 调用)        │  │
│  └──────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────┘
         ▲                          │
         │ D5 Incident Export       │ Tune Suggestions
         │ (评测信号输入)            │ (到配置或人工)
         │                          ▼
  ┌───────────────┐         ┌──────────────┐
  │ Production    │         │ Config       │
  │ Trace Pool    │         │ Adjustment   │
  └───────────────┘         └──────────────┘
```

### 3.2 核心接口

```go
// 评测引擎入口
type EvalEngine interface {
    // Run 执行一次评测，返回评分报告
    Run(ctx context.Context, dataset *EvalDataset, opts EvalOpts) (*EvalReport, error)
}

// 评测集
type EvalDataset struct {
    ID      string          // 版本化 ID（git SHA）
    Name    string
    Buckets []EvalBucket    // 四桶
    Items   []EvalItem      // 评测用例
}

// 单条评测用例
type EvalItem struct {
    ID          string
    Bucket      string          // "production" | "adversarial" | "edge" | "failure"
    Input       EvalInput       // 原始输入（prompt, context, trace bundle）
    Expectation EvalExpectation // 期望输出/事实/轨迹
    Tags        map[string]string
}

// 评分报告
type EvalReport struct {
    DatasetID   string
    RunAt       time.Time
    JudgeModel  string
    Scores      []DomainScore   // 各域各维度评分
    Delta       *EvalDelta      // vs 基线（nil 表示首次运行）
    TuneSuggest []TuneSuggestion
}

// 评分
type DomainScore struct {
    Domain      string  // "d2", "d3", "d4"
    Dimension   string  // "compression_recall", "pev_tool_accuracy", ...
    Score       float64 // 0.0 - 1.0
    Confidence  float64 // 评分置信度
    Details     map[string]float64 // 子维度分数
}
```

### 3.3 LLM-as-Judge 管理

```go
type JudgeManager interface {
    // Score 对单条评测用例评分
    Score(ctx context.Context, item EvalItem, rubric ScoreRubric) (*JudgeScore, error)

    // Calibrate 在人工标注集上校准，返回 kappa
    Calibrate(ctx context.Context, goldSet []GoldLabel) (*CalibrationReport, error)

    // ResolveDispute 主 Judge 与反方 Judge 分歧时仲裁
    ResolveDispute(ctx context.Context, primary, secondary *JudgeScore) (*JudgeScore, error)
}
```

**Judge 策略（精度优先）：**
- 主 Judge：与 production generator 不同模型族
- 反方 Judge：第三模型族，仅在分歧时启用
- Position randomization：A/B 交换各评一次，取平均
- CoT before scoring：推理链在前，分数在后
- 月度校准：Cohen's kappa ≥ 0.6，不足则调整 Judge prompt

### 3.4 评测集自动抽取管线

```
Production Trace Pool（D5 incident export bundle）
  │
  ├── 聚类引擎（按意图 embedding + 工具调用序列聚类）
  ├── 每簇按 stratification 采样（60/15/15/10）
  ├── 自动标注：
  │   ├─ compression: 压缩前 context = ground truth
  │   ├─ PEV: tool 调用序列 = 期望轨迹
  │   └─ Provider: 多 provider 输出待对比
  ├── 人工审核（diff review 模式）
  └── 合入评测集 git 仓库（YAML）
```

## 4. 方案对比

### 4.1 评测引擎架构选择

| 方案 | 优点 | 缺点 |
|------|------|------|
| **A: 独立 D6 子包 `evolution/eval/`** | 不耦合任何域，可消费所有域信号 | 需定义与 D5 的接口契约 |
| B: 放在 D5 Observability 内 | 信号采集 + 评分在同一层 | 职责混淆，D5 不关心自演化 |
| C: 放在各域内各自实现 | 每域可独立迭代 | 碎片化，无统一评测框架 |

**选择：A**
**理由：** 评测是 D6 自演化层的核心职责，独立放置避免与 D5 信号采集耦合，同时保持消费所有域信号的灵活性。

### 4.2 LLM-as-Judge 策略选择

| 方案 | 优点 | 缺点 |
|------|------|------|
| **A: 双 Judge + 分歧仲裁 + 月度校准** | 精度最高，可检测 judge drift | 成本 2-3x 单 Judge |
| B: 单 Judge + 无需校准 | 成本低，实现简单 | 评分漂移不可感知 |
| C: 确定性评分 + 少量 LLM Judge | 成本可控 | 语义维度无法覆盖 |

**选择：A**
**理由：** 精度优先的要求决定了需要冗余 Judge 和校准机制。

### 4.3 评测集存储选择

| 方案 | 优点 | 缺点 |
|------|------|------|
| **A: YAML 文件 + git 版本化** | 透明、可 review、SHA 锁定 | 大文件不友好（>1000 条） |
| B: 数据库/对象存储 | 适合大规模 | 版本对比需额外工具 |
| C: YAML + 大文件 git-lfs | 适合混合规模 | 额外依赖 |

**选择：A**
**理由：** Pilot 阶段 50-500 条，YAML + git 足够。后续扩展可加 LFS。

## 5. Capabilities

| Capability | L4 映射 | 优先级 | 说明 |
|------------|---------|--------|------|
| eval-engine | L4-EVAL-ENGINE | P0 | 评测编排 + 批量跑分 |
| eval-judge | L4-EVAL-JUDGE | P0 | LLM-as-Judge 管理器 + 校准 + 分歧仲裁 |
| eval-dataset | L4-EVAL-DATASET | P0 | 评测集管理 + 半自动抽取 |
| eval-delta | L4-EVAL-DELTA | P1 | delta 报告 + 趋势分析 |
| eval-tune | L4-EVAL-TUNE | P2 | 配置调优建议生成 |
| probe-compression-recall | L4-EVAL-PROBE-COMP | P0 | Compression Recall Probe |
| probe-pev-tool-accuracy | L4-EVAL-PROBE-PEV-TOOL | P1 | PEV Tool 选择准确率 |
| probe-provider-quality | L4-EVAL-PROBE-PROVIDER | P1 | Provider 响应质量对比 |
| probe-agent-forkjoin | L4-EVAL-PROBE-AGENT-FJ | P2 | 多 Agent Fork/Join 质量 |

## 6. Implementation Plan

### Pilot（Phase 1）：框架搭建 + Compression Recall Probe

**目标**：跑通 D6 闭环——从评测集到 delta 报告到调优决策

| 里程碑 | 内容 | 产出 |
|--------|------|------|
| M1 核心类型 | `types.go` 定义 EvalRun/EvalReport/DomainScore 等 | 类型定义 + 单元测试 |
| M2 评测编排 | `engine.go` 评测循环逻辑 | 编排 + 单元测试 |
| M3 Judge 管理器 | `judge.go` 单 Judge + CoT + 分歧仲裁 | Judge 调用 + 校准 |
| M4 评测集管理 | `dataset.go` YAML 加载/版本化/半自动抽取脚本 | 评测集管理 + 抽取脚本 |
| M5 Compression Probe | `probes/compression_recall.go` 探针 | 探针 + 单元测试 |
| M6 delta 分析器 | `delta.go` 基线对比 | delta 报告 |
| M7 Pilot 验证 | 50 条评测集 → 全链路跑通 | Pilot 报告 |

### Phase 2：维度扩展

| 里程碑 | 内容 |
|--------|------|
| M8 PEV Tool 准确率探针 | `probes/pev_tool_accuracy.go` |
| M9 评测集治理 | 四个分桶完整化、月度刷新机制 |
| M10 PEV Plan 质量探针 | `probes/pev_plan_quality.go` |

### Phase 3：跨域扩展

| 里程碑 | 内容 |
|--------|------|
| M11 Provider 质量探针 | `probes/provider_quality.go` |
| M12 Agent Fork/Join 探针 | `probes/agent_forkjoin.go` |
| M13 调优建议 | `tune.go` 配置调优建议生成 |

### Phase 4：CI/CD 集成（按需）

| 里程碑 | 内容 |
|--------|------|
| M14 PR-time fast check | 确定性 mini eval（10-50 条，< 90s） |
| M15 自动 delta 报告 | PR comment 含质量变化摘要 |

## 7. Success Metrics（S3 准出）

- [ ] 评测引擎核心类型 + 编排 + Judge 管理器单元测试通过
- [ ] Compression Recall Probe 在 50 条评测集上可运行并产出评分
- [ ] LLM-as-Judge vs Human 校准达到 kappa ≥ 0.7
- [ ] delta 报告能正确反映"同配置 → 无变化，调低 budget → Recall 下降"
- [ ] 评测集 YAML 可版本化、可 review
- [ ] L5 测试点预登记（PLANNED）
- [ ] EvalRun 不与 production Process 路径耦合（默认关闭）

## 8. Risks & Mitigations

| 风险 | 概率 | 影响 | 缓解 |
|------|------|------|------|
| LLM-as-Judge 评分不可靠 | 中 | 高 | 双 Judge + 月度校准 + 分歧仲裁 |
| 评测集与生产流量分布偏差 | 中 | 中 | 四桶结构 + 月度刷新 + 人工审核 |
| 评测成本（Judge 调用） | 高 | 中 | 抽样评分、Judge 成本单独跟踪、Judge 用较便宜模型 |
| Pilot 维度过窄导致框架设计偏 | 中 | 中 | Pilot 验证通过后立即扩展第二维度验证框架通用性 |
| 评测集标注质量 | 中 | 高 | Diff review 模式 + Cohen's kappa ≥ 0.7 才能发布 |
| 与 D5 职责边界模糊 | 低 | 低 | 明确 D5=采集，D6=评分；接口通过 incident export 解耦 |

## 9. Out of Scope（本 Change）

- 评测结果自动调参（Phase 4 或独立 change，Pilot 仅出建议 + 人工确认）
- 评测引擎自评测（自举）
- Production trace 实时评分（仅离线批量）
- 评测集自动扩增（仅半自动 + 人工）
- Error-biased sampling（D5 范畴）
- LLM-as-Judge 的 fine-tune（用 prompt-based + 校准方式，无需微调）
