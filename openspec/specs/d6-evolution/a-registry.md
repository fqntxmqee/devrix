# D6 Evolution Domain — A 层活动注册表

**Capability:** architecture-layering
**Status:** Active
**Version:** 3.0.0
**Last Updated:** 2026-06-15
**Parent:** `openspec/specs/architecture/layering.md`
**Change:** devrix-d6-sa-refine（DM-20260615-002 / v1.0 Canonical 重排；4 价值流 S 层；消除 D7 命名冲突）

---

## Overview

D6 演化域 A 层活动注册表（Canonical v3.0）。S 层重切为 4 价值流（2 IMPLEMENTED + 2 PLANNED），S4 "Orchestration" 重命名为 S12 GuardRuntime。

---

## D6-S11: RunEvaluation（评测执行）

**承诺 C1：** 给定 dataset + probes，返回 eval_report + delta + tune_suggestions

| A ID | Name | Type | Input | Output | State Change | Code Location | Legacy |
|------|------|------|-------|--------|--------------|---------------|--------|
| D6-S11-A01 | RunEval | A-BE | dataset, probes | eval_report | eval.{started,completed} | `evolution/eval/engine.go` | S3-A01 |
| D6-S11-A02 | JudgeResult | A-BE | eval_item, rubric | judge_score | -- | `evolution/eval/judge.go` | S3-A02 |
| D6-S11-A03 | CompareDelta | A-BE | current_report, baseline | eval_delta | -- | `evolution/eval/delta.go` | S3-A03 |
| D6-S11-A04 | GenerateTune | A-BE | eval_delta | tune_suggestions | -- | `evolution/eval/tune.go` | S3-A04 |
| D6-S11-A05 | ManageDataset | A-BE | path, items | dataset, baseline | -- | `evolution/eval/dataset.go` | S3-A05 |

## D6-S12: GuardRuntime（运行时守护）

**承诺 C2：** Agent 异常行为被 Observer 捕获 → Validator 判定 → Intervention 执行

| A ID | Name | Type | Input | Output | State Change | Code Location | Legacy |
|------|------|------|-------|--------|--------------|---------------|--------|
| D6-S12-A01 | ValidateDecision | A-BE | decision_record, session | validation_result | validation.{passed,failed} | `evolution/orchestration/validator.go` | S4-A01 |
| D6-S12-A02 | ExecuteIntervention | A-BE | intervention, session | -- | agent.{terminated,rerouted}, task.{failed,completed} | `evolution/orchestration/intervention.go` | S4-A02 |
| D6-S12-A03 | ObserveAgent | A-BE | agent_event | decision_record | validation_triggered | `evolution/orchestration/observer.go` | S4-A03 |

## D6-S13: TrackVersion（版本追踪 — PLANNED）

**承诺 C3：** 构建信息 → 版本报告

| A ID | Name | Type | Input | Output | State Change | Code Location | Status | Legacy |
|------|------|------|-------|--------|--------------|---------------|--------|--------|
| D6-S13-A01 | DetectVersion | A-BE | build_info | version_report | -- | PLANNED | PLANNED | S1-A01 |

## D6-S14: ReloadConfig（配置热更新 — PLANNED）

**承诺 C4：** 监控配置文件变更 → 热加载

| A ID | Name | Type | Input | Output | State Change | Code Location | Status | Legacy |
|------|------|------|-------|--------|--------------|---------------|--------|--------|
| D6-S14-A01 | HotReload | A-BE | config_watch | updated_config | config.reloaded | PLANNED | PLANNED | S2-A01 |

---

## Legacy Module Index（D6-S1–S4，冻结追溯）

| Legacy S | Module | Status | Canonical S | Scenario |
|----------|--------|--------|-------------|----------|
| D6-S1 | Version | Legacy（PLANNED） | S13 | TrackVersion |
| D6-S2 | Config | Legacy（PLANNED） | S14 | ReloadConfig |
| D6-S3 | Eval | Legacy | S11 | RunEvaluation |
| D6-S4 | Orchestration | Legacy（**D7 命名冲突已消除**） | S12 | GuardRuntime |

---

## Statistics

| Scenarios | Activities | IMPLEMENTED | PLANNED |
|-----------|------------|-------------|---------|
| 4 | 10 | 8 | 2 |

## Revision History

| 版本 | 日期 | 变更 |
|------|------|------|
| 2.0.0 | 2026-06-14 | 初版：4 技术模块 S 层（含 S4 "Orchestration"） |
| **3.0.0** | 2026-06-15 | **SA Refine v1.0**：Canonical S11–S14 重排；S4 Orchestration → S12 GuardRuntime；增 Legacy 列 + Module Index |
