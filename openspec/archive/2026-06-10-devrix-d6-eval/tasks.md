# Tasks: D6 自演化评测引擎（Eval Framework）

**Change ID:** devrix-d6-eval
**Demand ID:** DM-20260610-006
**Status:** S4 实现中
**Phase:** Pilot — Compression Recall Probe 闭环

---

## M1: 核心类型定义

**Files:** `internal/layers/evolution/eval/types.go`
**Effort:** 参考

| ID | Task | Status |
|----|------|--------|
| M1.1 | 定义 EvalItem / EvalDataset / EvalReport / DomainScore 核心类型 | DONE |
| M1.2 | 定义 JudgeScore / ScoreRubric / GoldLabel / CalibrationReport | DONE |
| M1.3 | 定义 EvalDelta / DeltaEntry / TuneSuggestion | DONE |
| M1.4 | 定义 EvalConfig / JudgeConfig / SamplingConfig | DONE |

## M2: Judge 管理器

**Files:** `internal/layers/evolution/eval/judge.go`, `judge_test.go`
**Effort:** 参考

| ID | Task | Status |
|----|------|--------|
| M2.1 | LLMClient 接口（抽象 LLM 调用，支持 mock） | DONE |
| M2.2 | JudgeManager 核心：Score / Calibrate / ResolveDispute | DONE |
| M2.3 | Position randomization（两次评分取平均） | DONE |
| M2.4 | Judge prompt 构建（rubric + CoT） | DONE |
| M2.5 | 分歧检测 + 仲裁逻辑 | DONE |
| M2.6 | 单元测试：评分、校准、分歧 | DONE |

## M3: 评测集管理

**Files:** `internal/layers/evolution/eval/dataset.go`, `dataset_test.go`
**Effort:** 参考

| ID | Task | Status |
|----|------|--------|
| M3.1 | YAML 加载 + schema 校验 | DONE |
| M3.2 | StratifiedSample 抽样逻辑 | DONE |
| M3.3 | 版本化路径解析（支持 latest symlink） | DONE |
| M3.4 | 单元测试：加载/校验/抽样 | DONE |

## M4: Delta 分析器

**Files:** `internal/layers/evolution/eval/delta.go`, `delta_test.go`
**Effort:** 参考

| ID | Task | Status |
|----|------|--------|
| M4.1 | DeltaAnalyzer.Compare — 逐维对比 | DONE |
| M4.2 | 分桶对比 | DONE |
| M4.3 | Regression 标记（three-band） | DONE |
| M4.4 | 单元测试：同配置无变化、有意识退化可检测 | DONE |

## M5: 评测引擎编排

**Files:** `internal/layers/evolution/eval/engine.go`, `engine_test.go`
**Effort:** 参考

| ID | Task | Status |
|----|------|--------|
| M5.1 | EvalRun 核心编排 | DONE |
| M5.2 | enabled=false 短路 | DONE |
| M5.3 | 单元测试：基本流程、空数据集、enabled=false | DONE |

## M6: Compression Recall Probe

**Files:** `internal/layers/evolution/eval/probes/probe.go`, `probes/compression_recall.go`, `probes/compression_recall_test.go`
**Effort:** 参考

| ID | Task | Status |
|----|------|--------|
| M6.1 | Probe 接口 + 注册表 | DONE |
| M6.2 | CompressionRecallProbe 探针逻辑 | DONE |
| M6.3 | 单元测试：全部保留、部分丢失、与压缩率负相关 | DONE |

## M7: 初版评测集 + 集成验证

**Files:** `openspec/eval-datasets/v1/dataset.yaml`
**Effort:** 参考

| ID | Task | Status |
|----|------|--------|
| M7.1 | 初版评测集 YAML（10 条 compression 场景） | DONE |
| M7.2 | 集成测试：dataset → EvalRun → report | DONE |
| M7.3 | 集成测试：dataset → EvalRun → report | DONE |
