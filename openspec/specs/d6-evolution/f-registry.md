# D6 Evolution Domain — F 层功能点注册表

**Capability:** architecture-layering
**Status:** Active
**Version:** 1.0.0
**Last Updated:** 2026-06-13
**Parent:** `openspec/specs/architecture/layering.md`
**Depends On:** `openspec/specs/d6-evolution/a-registry.md`

---

## Overview

D6 演化域 F 层功能点注册表。

---

## D6-S3-A01 RunEval

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D6-S3-A01-F01 | LoadDataset | F-BE | path | *EvalDataset | `eval/dataset.go` |
| D6-S3-A01-F02 | RunProbe | F-BE | item, judge | *DomainScore | `eval/probe.go` |
| D6-S3-A01-F03 | AggregateReport | F-BE | []DomainScore | *EvalReport | `eval/engine.go` |
| D6-S3-A01-F04 | CheckDeltaGate | F-BE | delta | GateResult | `eval/delta.go` |

## D6-S3-A02 JudgeResult

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D6-S3-A02-F01 | ScoreWithRubric | F-BE | item, rubric | score | `eval/judge.go` |
| D6-S3-A02-F02 | ResolveDispute | F-BE | dispute | resolved_score | `eval/judge.go` |

---

## Statistics

| Activities with F | Total F Points |
|-------------------|----------------|
| 2 | 6 |
