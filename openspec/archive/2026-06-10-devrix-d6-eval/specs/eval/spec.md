# D6-S3 Eval Engine Specification

## ADDED

<!-- L5: L5-6-3-01 -->
### Requirement: EvalRun 编排

评测引擎必须支持从评测集到评分报告的完整编排。

#### Scenario: 基本编排流程
- GIVEN 一个包含 10 条评测用例的 YAML 数据集
- WHEN EvalRun 被调用
- THEN 返回的 EvalReport 包含所有维度的 DomainScore
- AND 每条评测用例都被评分
- AND EvalReport 包含评分面板（ScoreDashboard）

#### Scenario: 空评测集返回空报告
- GIVEN 一个空评测集
- WHEN EvalRun 被调用
- THEN 返回的 EvalReport.Scores 为空列表
- AND 不报错

<!-- L5: L5-6-3-07 -->
### Requirement: 功能开关

评测引擎必须可通过配置关闭，关闭时零行为变化。

#### Scenario: enabled=false 时不执行任何操作
- GIVEN evolution.eval.enabled=false
- WHEN 任意评测调用被触发
- THEN 评测引擎不执行任何操作
- AND 返回的 EvalReport 为 nil
- AND 不产生任何 Judge 调用

#### Scenario: enabled=true 时正常执行
- GIVEN evolution.eval.enabled=true
- WHEN EvalRun 被调用
- THEN 评测引擎正常执行编排
- AND 返回完整的 EvalReport

<!-- L5: L5-6-3-02 -->
### Requirement: LLM-as-Judge 评分校准

Judge 管理器必须支持评分与人类标注的一致性校验。

#### Scenario: 评分与人类标注一致
- GIVEN 一个包含 50 条人类标注的校准集
- WHEN Calibrate 被调用
- THEN 返回的 CalibrationReport 包含 Cohen's kappa 值
- AND 当 kappa >= 0.6 时 Passed=true
- AND 当 kappa < 0.6 时 Passed=false

#### Scenario: 分歧仲裁逻辑
- GIVEN 主 Judge 评分 0.9、反方 Judge 评分 0.4（差值 > 1σ）
- WHEN ResolveDispute 被调用
- THEN 返回的评分包含仲裁标记
- AND 该条用例被标记为待人工审核

<!-- L5: L5-6-3-03 -->
### Requirement: Compression Recall Probe

Compression Recall Probe 必须能正确评估压缩前后的事实保留率。

#### Scenario: 压缩保留全部关键事实
- GIVEN 压缩后的上下文保留了所有 P0 事实
- WHEN CompressionRecallProbe.Run 被调用
- THEN 返回的 Recall F1 >= 0.95

#### Scenario: 压缩丢弃部分关键事实
- GIVEN 压缩后的上下文丢失了 50% 的 P0 事实
- WHEN CompressionRecallProbe.Run 被调用
- THEN 返回的 Recall F1 <= 0.6

#### Scenario: 评分与压缩率负相关
- GIVEN 同一原始上下文、不同压缩率的压缩结果
- WHEN 对每个压缩结果执行 Recall Probe
- THEN 压缩率越高，Recall F1 越低或持平
- AND 不出现低压缩率低 recall 的反常情况

<!-- L5: L5-6-3-04 -->
### Requirement: Delta 报告

Delta 分析器必须能正确对比当前评分与基线。

#### Scenario: 同配置下 delta 无显著变化
- GIVEN 相同评测集、相同配置下两次运行
- WHEN 第二次评分对比第一次基线
- THEN 所有维度的 delta 绝对值 < 0.02

#### Scenario: 有意识退化可被检测
- GIVEN 将 compression budget 从 0.3 改为 0.1（更激进压缩）
- WHEN 运行评测并与基线对比
- THEN compression_recall 维度 delta 为负值
- AND Regressions 列表包含 compression_recall 条目

#### Scenario: delta 报告包含分桶详情
- GIVEN 四桶评测集下的评分结果
- WHEN DeltaAnalyzer.Compare 被调用
- THEN delta 报告包含每个分桶的独立对比
- AND 报告按 bucket 输出分数

<!-- L5: L5-6-3-05 -->
### Requirement: 评测集管理

评测集必须支持 YAML 加载、版本化和 schema 校验。

#### Scenario: 合法 YAML 加载成功
- GIVEN 一个符合 schema 的评测集 YAML
- WHEN EvalDataset.Load 被调用
- THEN 返回成功
- AND Items 数量与 YAML 一致

#### Scenario: 非法 YAML 报错
- GIVEN 一个缺少必填字段的 YAML（如缺少 ID）
- WHEN EvalDataset.Load 被调用
- THEN 返回错误
- AND 错误信息指明缺失的字段

#### Scenario: 版本化路径解析
- GIVEN 评测集路径 "openspec/eval-datasets/v1/dataset.yaml"
- WHEN 指定 version="v1" 加载
- THEN 正确加载 v1 版本
- AND 可根据 latest symlink 加载最新版本

<!-- L5: L5-6-3-06 -->
### Requirement: PEV Tool 选择准确率探针

PEV Tool 准确率探针必须能评估 tool 选择的 precision/recall/F1。

#### Scenario: 全部 tool 选择正确
- GIVEN 预期 tool 调用序列与实际序列完全一致
- WHEN PEVToolAccuracyProbe.Run 被调用
- THEN Precision=1.0, Recall=1.0, F1=1.0

#### Scenario: 部分 tool 选择错误
- GIVEN 预期调用 tool A 但实际调用了 tool B
- WHEN PEVToolAccuracyProbe.Run 被调用
- THEN 相应 tool 的 Precision 和 Recall 下降
- AND F1 < 1.0

<!-- L5: L5-6-3-08 -->
### Requirement: 分歧仲裁

Judge Manager 必须能正确处理主 Judge 与反方 Judge 的分歧。

#### Scenario: 分歧低于阈值自动取平均
- GIVEN 主 Judge 评分 0.85、反方 Judge 评分 0.75（差值 <= 1σ）
- WHEN 评分完成
- THEN 最终评分为 0.80（两者平均）
- AND 不进入人工仲裁队列

#### Scenario: 分歧超过阈值进入仲裁
- GIVEN 主 Judge 评分 0.9、反方 Judge 评分 0.3（差值 > 1σ）
- WHEN 评分完成
- THEN 该条标记为 DISPUTED
- AND 添加到人工仲裁队列
- AND 报告中标注该条评分置信度低

<!-- L5: L5-6-3-09 -->
### Requirement: Provider 质量对比探针

Provider 质量探针必须能评估不同 provider 的语义一致性和指令遵循率。

#### Scenario: 同 provider 间语义一致
- GIVEN 同一 provider 对相同 prompt 的两次响应
- WHEN ProviderQualityProbe.Run 被调用
- THEN 语义一致性评分 >= 0.9

#### Scenario: 不同 provider 间可比
- GIVEN Provider A 和 Provider B 对相同 prompt 的响应
- WHEN ProviderQualityProbe.Run 被调用
- THEN 返回两个 provider 的指令遵循率
- AND 报告语义相似度分数

<!-- L5: L5-6-3-10 -->
### Requirement: Agent Fork/Join 质量探针

Fork/Join 探针必须能评估多 Agent 协调的消息隔离和结果合并质量。

#### Scenario: 消息隔离正确
- GIVEN Fork 子 Agent A 和 B 分别处理独立任务
- WHEN AgentForkJoinProbe.Run 被调用
- THEN A 的消息不包含 B 的内容
- AND B 的消息不包含 A 的内容

#### Scenario: Join 结果完整
- GIVEN 两个子 Agent 分别产出了部分结果
- WHEN 父 Agent 执行 Join
- THEN Join 结果包含所有子 Agent 的关键输出
- AND 不引入未在子 Agent 中出现的信息

## MODIFIED

(None)

## REMOVED

(None)
