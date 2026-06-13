# D6 Evolution Domain — A 层活动注册表

**Capability:** architecture-layering
**Status:** Active
**Version:** 1.0.0
**Last Updated:** 2026-06-13
**Parent:** `openspec/specs/architecture/layering.md`

---

## Overview

D6 演化域 A 层活动注册表。

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
| D6-S3-A02 | JudgeResult | A-BE | eval_item, rubric | score | — | `evolution/eval/judge.go` |

## D6-S4: Orchestration

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D6-S4-A01 | ValidateOrchestration | A-BE | orchestration_event | validation_result | validation.{passed,failed} | `evolution/orchestration/` |

---

## Statistics

| Scenarios | Activities | PLANNED |
|-----------|------------|---------|
| 4 | 5 | 2 |
