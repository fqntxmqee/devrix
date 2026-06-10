---
demand-id: DM-20260610-006
title: D6 自演化评测引擎（Eval Framework）
priority: P1
status: S1_Proposal
l1-domain: evolution
created: 2026-06-10
---

# D6 自演化评测引擎（Eval Framework）

## 1. 背景

Devrix 已有完备的 L5 验收测试体系（110+ 测试点）和 D5 可观察层（trace/metrics/log/incident export），但它们解决的是不同的需求：

- **L5 测试**：功能是否工作，pass/fail 二值，用于 S5 门禁
- **D5 可观察**：发生了什么，信号采集，用于人工/AI 排查
- **缺失**：能力升级后效果是涨是跌，连续质量评分，驱动自演化闭环

经过 D2（Context Engine）、D3（LLM Gateway）、D4（Multi-Agent）多轮能力升级，团队需要一个**评测框架**来量化每次升级的影响，避免"修了 A 功能、降了 B 质量"的问题。

## 2. 问题陈述

### 核心问题

Devrix 在能力升级过程中缺乏**效果回退检测**机制。具体痛点：

| 痛点 | 场景 | 当前状态 |
|------|------|----------|
| Compression 升级无质量验证 | 调整 compression budget 或策略后，无法知道关键事实是否被丢掉 | 仅有 token 减少量统计 |
| PEV 推理质量退化不可感知 | Prompt 调优或 tool 增减后，plan 质量或 tool 选择准确率是否下降 | 仅靠人工观察 |
| Provider 更换无量化对比 | 新增或切换 LLM provider 时，响应质量差异只能主观判断 | 无结构化对比报告 |
| 多 Agent 协调质量不可见 | Fork/Join 逻辑变更后，任务分解和结果合并质量是否退化 | 仅有功能测试 |
| 无自演化闭环 | 评测结果目前不反馈到系统配置调整中 | 全人工决策 |

### 目标

建立 D6-S3 Eval 评测引擎，作为 Devrix 自演化层的核心能力，实现：

1. **评测**：对 D2/D3/D4 各环节做连续质量评分
2. **回归检测**：能力升级前后自动对比，输出 delta 报告
3. **自演化**：评测结果驱动配置调优建议（人工确认后生效）

## 3. 验收标准

| ID | 标准 | 优先级 |
|----|------|--------|
| AC1 | 评测引擎可从 production trace 半自动抽取评测用例，经人工审核后入库 | P0 |
| AC2 | LLM-as-Judge 评分与人类判断的 Cohen's kappa ≥ 0.7（月度校准） | P0 |
| AC3 | 支持压缩 Recall Probe 评测维度（压缩前后 P0 事实召回率 F1） | P0 |
| AC4 | delta 报告能清晰对比当前评分与基线，输出各维度分差 | P0 |
| AC5 | 评测引擎默认关闭，不与 production Process 路径耦合 | P0 |
| AC6 | 支持 PEV Tool 选择准确率评测（precision/recall/F1） | P1 |
| AC7 | 支持 Provider 响应质量对比评测（语义一致性 + 指令遵循率） | P1 |
| AC8 | 支持多 Agent Fork/Join 协调质量评测（消息隔离、Join 完整度） | P1 |
| AC9 | 支持评测集版本化管理（YAML + git，SHAD 锁定） | P1 |
| AC10 | delta 报告超出阈值时自动生成配置调优建议 | P2 |

## 4. 依赖与约束

| 类型 | 内容 |
|------|------|
| 依赖 | D5 Observability incident export（评测信号输入） |
| 依赖 | D3 LLM Gateway（LLM-as-Judge 调用通道，需不同模型族） |
| 依赖 | D2/D3/D4 各域 L5 测试点（评测集基础素材） |
| 约束 | 评测引擎不得 import communication / adapters |
| 约束 | 评测默认关闭（enabled=false），走独立路径 |
| 约束 | LLM-as-Judge 必须与 production generator 使用不同模型族 |

## 5. 变更范围

### 新增

- `internal/layers/evolution/eval/` 包：评测引擎核心
  - `engine.go`：评测编排
  - `judge.go`：LLM-as-Judge 管理器 + 校准
  - `dataset.go`：评测集加载 / 版本化
  - `delta.go`：delta 分析器
  - `tune.go`：调优建议生成
  - `types.go`：核心类型定义
  - `probes/` 子包：各维度探针
    - `compression_recall.go`
    - `pev_tool_accuracy.go`
    - `provider_quality.go`
    - `agent_forkjoin.go`
- 评测集目录 `openspec/eval-datasets/`
- D6-S3 相关 L5 测试点注册
- 配置块 `evolution.eval`

### 修改

- `internal/layers/evolution/` 下无现有代码冲突（仅 version + config 子包）
- 无 D2/D3/D4 现有代码修改

### 不变更

- 不修改 D2/D3/D4 现有 Process 路径
- 不修改 D5 可观察层的现有实现（但消费 incident export 输出）
- 不修改现有 L5 测试体系

## 6. 设计要点

### 6.1 Pilot 范围

Pilot 实现一个完整闭环——**Compression Recall Probe**：

1. 从 production trace 抽 50 条 compression 场景（半自动脚本）
2. 人工审核标注 P0 事实列表
3. 评测引擎 → D3 LLM-as-Judge → Recall F1 评分
4. delta 报告与基线对比
5. 人工根据结果调整 compression budget
6. 下一轮评测验证调整效果

Pilot 成功标志：
- 评测集可半自动抽取 + 人工审核
- LLM-as-Judge vs Human kappa ≥ 0.7
- delta 报告可读且正确反映变化
- 有人根据 delta 报告做了配置调整决策
- 下一轮验证调整有效

### 6.2 后续扩展顺序

1. **PEV Tool 选择准确率**（改动最频繁，回归检测需求最迫切）
2. **PEV Plan 质量**
3. **Provider 响应质量**
4. **Tool 面适配度** → **Agent Fork/Join** → 更复杂维度

### 6.3 LLM-as-Judge 策略（精度优先）

- 主 Judge 与 Production 不同模型族
- 反方 Judge（第三模型），仅在主 Judge 与反方分歧 > 1σ 时启用人工仲裁
- Position randomization（A/B 交换取平均）
- Chain-of-thought before scoring
- Per-dimension 评分（不 aggregate）
- 月度校准（Cohen's kappa ≥ 0.6）

### 6.4 评测集四桶结构

| 分桶 | 比例 | 来源 |
|------|------|------|
| Production Core | 60% | 从 production trace 按意图分层采样 |
| Adversarial | 15% | 已知边界/错误案例 |
| Edge Cases | 15% | 长尾场景（超大上下文、极简指令） |
| Failure Replays | 10% | 历史上出过问题的场景 |

## 7. Out of Scope

- 评测结果自动调参（V7 再考虑，Pilot 阶段仅出建议 + 人工确认）
- 评测引擎的自评测（自举）
- Production trace 的实时评分（仅离线批量）
- 评测集的自动扩增（仅半自动 + 人工）
- Error-biased sampling（属于 D5 范畴）

## 8. 风险评估

| 风险 | 影响 | 缓解 |
|------|------|------|
| LLM-as-Judge 评分漂移 | 高 | 月度校准 + 双 Judge 分歧仲裁 |
| 评测集与 production 分布偏差 | 中 | 四桶结构 + 月度从 production trace 刷新 |
| 评测成本失控 | 中 | 抽样评分（非全量）、Judge 成本单独跟踪 |
| 评测结果被用作门禁 | 低 | 明确评测非门禁，仅 delta 参考 |
| 与 D5 职责重叠 | 中 | D6 负责评分，D5 负责采集，接口通过 incident export 解耦 |
