# D6 Evolution Domain — F 层功能点注册表

**Capability:** architecture-layering
**Status:** Active
**Version:** 2.0.0
**Last Updated:** 2026-06-14
**Parent:** `openspec/specs/architecture/layering.md`
**Depends On:** `openspec/specs/d6-evolution/a-registry.md`

---

## Overview

D6 演化域 F 层功能点注册表。

---

## D6-S3-A01 RunEval

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D6-S3-A01-F01 | DispatchProbe | F-BE | item, dimension | *DomainScore | `eval/engine.go`, `eval/probe.go` |
| D6-S3-A01-F02 | AggregateReport | F-BE | []DomainScore | *EvalReport | `eval/engine.go` |
| D6-S3-A01-F03 | ComputeDashboard | F-BE | scores, items | ScoreDashboard | `eval/engine.go` |

## D6-S3-A02 JudgeResult

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D6-S3-A02-F01 | ScoreWithRubric | F-BE | item, rubric | *JudgeScore | `eval/judge.go` |
| D6-S3-A02-F02 | CrossValidate | F-BE | primary_score, secondary_llm | validated_score | `eval/judge.go` |
| D6-S3-A02-F03 | ResolveDispute | F-BE | item_id, final_score | — | `eval/judge.go` |
| D6-S3-A02-F04 | CalibrateKappa | F-BE | gold_labels, rubric | *CalibrationReport | `eval/judge.go` |

## D6-S3-A03 CompareDelta

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D6-S3-A03-F01 | CompareWithBaseline | F-BE | current, baseline | *EvalDelta | `eval/delta.go` |
| D6-S3-A03-F02 | ClassifySeverity | F-BE | delta_value | SeverityLevel | `eval/delta.go` |
| D6-S3-A03-F03 | CheckDeltaGate | F-BE | delta | GateResult | `eval/gate.go` |

## D6-S3-A04 GenerateTune

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D6-S3-A04-F01 | SuggestTune | F-BE | eval_delta | []TuneSuggestion | `eval/tune.go` |

## D6-S3-A05 ManageDataset

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D6-S3-A05-F01 | LoadDataset | F-BE | path | *EvalDataset | `eval/dataset.go` |
| D6-S3-A05-F02 | LoadDatasetVersion | F-BE | path, version | *EvalDataset | `eval/dataset.go` |
| D6-S3-A05-F03 | StratifiedSample | F-BE | items, max_items | []EvalItem | `eval/dataset.go` |
| D6-S3-A05-F04 | SaveBaseline | F-BE | path, report | — | `eval/dataset.go` |
| D6-S3-A05-F05 | LoadBaseline | F-BE | path | *EvalReport | `eval/dataset.go` |
| D6-S3-A05-F06 | ValidateDataset | F-BE | *EvalDataset | error | `eval/dataset.go` |

---

## D6-S4-A01 ValidateDecision

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D6-S4-A01-F01 | PreFilter | F-BE | decision_record | bool (skip) | `orchestration/validator.go` |
| D6-S4-A01-F02 | ValidateWithJudge | F-BE | decision_record | *ValidationResult | `orchestration/validator.go`, `orchestration/judge_adapter.go` |
| D6-S4-A01-F03 | DetermineIntervention | F-BE | validation_result | *Intervention | `orchestration/validator.go` |

## D6-S4-A02 ExecuteIntervention

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D6-S4-A02-F01 | TerminateAgent | F-BE | session | — | `orchestration/intervention.go` |
| D6-S4-A02-F02 | TerminateAndReroute | F-BE | session, intervention | — | `orchestration/intervention.go` |
| D6-S4-A02-F03 | UpdateTaskState | F-BE | session, intervention | — | `orchestration/intervention.go` |

## D6-S4-A03 ObserveAgent

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D6-S4-A03-F01 | MapAgentEvent | F-BE | agent_event | *DecisionRecord | `orchestration/observer.go` |
| D6-S4-A03-F02 | FeedPipeline | F-BE | decision_record | — | `orchestration/observer.go` |

---

## Bridge (D6→D3)

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D6-BR01-F01 | GatewayLLMChat | F-BE | model, system_prompt, user_msg | response_text, TokenCost | `eval/gateway_llm.go` |
| D6-BR02-F01 | RuntimeJudgeValidate | F-BE | decision_record | *ValidationResult | `orchestration/judge_adapter.go` |

---

## Statistics

| Activities with F | Total F Points |
|-------------------|----------------|
| 8 | 27 |
