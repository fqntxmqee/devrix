# D6 Evolution Domain — A 层活动注册表

**Capability:** architecture-layering
**Status:** Active
**Version:** 2.0.0
**Last Updated:** 2026-06-14
**Parent:** `openspec/specs/architecture/layering.md`

---

## Overview

D6 演化域 A 层活动注册表。D6-S1（Version）与 D6-S2（Config）仍为规划阶段。

---

## D6-S1: Version

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D6-S1-A01 | DetectVersion | A-BE | build_info | version_report | — | PLANNED |

## D6-S2: Config

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D6-S2-A01 | HotReload | A-BE | config_watch | updated_config | config.reloaded | PLANNED |

## D6-S3: Eval

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D6-S3-A01 | RunEval | A-BE | dataset, probes | eval_report | eval.{started,completed} | `evolution/eval/engine.go` |
| D6-S3-A02 | JudgeResult | A-BE | eval_item, rubric | judge_score | — | `evolution/eval/judge.go` |
| D6-S3-A03 | CompareDelta | A-BE | current_report, baseline | eval_delta | — | `evolution/eval/delta.go` |
| D6-S3-A04 | GenerateTune | A-BE | eval_delta | tune_suggestions | — | `evolution/eval/tune.go` |
| D6-S3-A05 | ManageDataset | A-BE | path, items | dataset, baseline | — | `evolution/eval/dataset.go` |

## D6-S4: Orchestration

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D6-S4-A01 | ValidateDecision | A-BE | decision_record, session | validation_result | validation.{passed,failed} | `evolution/orchestration/validator.go` |
| D6-S4-A02 | ExecuteIntervention | A-BE | intervention, session | — | agent.{terminated,rerouted}, task.{failed,completed} | `evolution/orchestration/intervention.go` |
| D6-S4-A03 | ObserveAgent | A-BE | agent_event | decision_record | validation_triggered | `evolution/orchestration/observer.go` |

---

## Statistics

| Scenarios | Activities | IMPLEMENTED | PLANNED |
|-----------|------------|-------------|---------|
| 4 | 10 | 8 | 2 |
