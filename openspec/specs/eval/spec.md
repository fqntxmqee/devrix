# D6-S3 Eval Engine Specification

**Capability:** eval
**Change ID:** devrix-d6-eval (archived 2026-06-10, Pilot)
**Layer:** Evolution
**Version:** 1.1.0-phase2
**Status:** Canonical — phase 2 partial

---

## Overview

D6-S3 评测引擎：EvalRun 编排、JudgeManager、Compression Recall Probe、PEV Tool Accuracy Probe、DeltaAnalyzer、TuneGenerator、YAML 评测集 v1、`devrix eval run` CLI。

**已实现：** L5-6-3-01/02/03/04/06/07/11/12/13  
**后续扩展：** provider / forkjoin 探针

---

## ADDED Requirements (Pilot)

<!-- L5: L5-6-3-01 -->
### Requirement: EvalRun 编排

评测引擎必须支持从评测集到评分报告的完整编排。

#### Scenario: 基本编排流程
- GIVEN 一个包含 10 条评测用例的 YAML 数据集
- WHEN EvalRun 被调用
- THEN 返回的 EvalReport 包含所有维度的 DomainScore
- AND 每条评测用例都被评分
- AND EvalReport 包含评分面板（ScoreDashboard）

<!-- L5: L5-6-3-07 -->
### Requirement: 功能开关

评测引擎必须可通过配置关闭，关闭时零行为变化。

#### Scenario: enabled=false 时不执行任何操作
- GIVEN evolution.eval.enabled=false
- WHEN 任意评测调用被触发
- THEN 评测引擎不执行任何操作
- AND 返回的 EvalReport 为 nil

<!-- L5: L5-6-3-02 -->
### Requirement: LLM-as-Judge 评分校准

Judge 管理器必须支持评分与人类标注的一致性校验及分歧仲裁。

<!-- L5: L5-6-3-03 -->
### Requirement: Compression Recall Probe

Compression Recall Probe 必须能评估压缩前后的事实保留率（Recall F1）。

<!-- L5: L5-6-3-04 -->
### Requirement: Delta 报告

Delta 分析器必须能对比当前评分与基线，标记 regression。

<!-- L5: L5-6-3-05 -->
### Requirement: 评测集管理

评测集必须支持 YAML 加载、版本化和 schema 校验。

---

<!-- L5: L5-6-3-06 -->
### Requirement: PEV Tool 选择准确率探针

PEV Tool 准确率探针必须能评估 tool 选择的 precision/recall/F1（确定性，基于 expected_tools / actual_tools）。

<!-- L5: L5-6-3-12 -->
### Requirement: 调优建议生成

Delta 报告出现 regression 时必须生成 TuneSuggestion 列表（预定义规则映射）。

## Deferred Requirements

- L5-6-3-09 Provider 质量对比探针
- L5-6-3-10 Agent Fork/Join 质量探针

完整需求见 `openspec/archive/2026-06-10-devrix-d6-eval/specs/eval/spec.md`。
