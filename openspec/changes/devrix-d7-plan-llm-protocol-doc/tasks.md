# Tasks: D7 Plan ↔ LLM 5 场景输入输出协议沉淀

**T-layer registration path**: `openspec/specs/d7-orchestration/t-registry.md`
**Status legend**: PLANNED / IMPLEMENTED

## A. Spec Doc (P0)

| T ID | Status | Description | Evidence |
|------|--------|-------------|----------|
| D7-S6-A121-T01 | IMPLEMENTED | §3 输入协议: 18 字段 frame 全表 + 7 data/11 control 分层 + 字段过滤条件 | `d7-plan-llm-io-protocol-spec.md:69-128` |
| D7-S6-A121-T02 | IMPLEMENTED | §4 输出协议: rawStrategicPlan JSON schema 8 字段 + execution_mode 3 选 1 enum + 解析路径 | `d7-plan-llm-io-protocol-spec.md:130-216` |
| D7-S6-A121-T03 | IMPLEMENTED | §5 场景 1: single + 1 step → CommitmentPlan (direct commit) | `d7-plan-llm-io-protocol-spec.md:218-273` |
| D7-S6-A121-T04 | IMPLEMENTED | §5 场景 2: command + multi-step ≤3 → ProtocolPlan (multi-step async) | `d7-plan-llm-io-protocol-spec.md:275-318` |
| D7-S6-A121-T05 | IMPLEMENTED | §5 场景 3: parallel_probe → ScenarioPlan (read-only probe) | `d7-plan-llm-io-protocol-spec.md:320-353` |
| D7-S6-A121-T06 | IMPLEMENTED | §5 场景 4: decompose + anomalies≥3 → ExplorationPlan (parallel sandbox) | `d7-plan-llm-io-protocol-spec.md:355-393` |
| D7-S6-A121-T07 | IMPLEMENTED | §5 场景 5 (混合): single + 高 UncertaintyMean + hasHighStrengthFact → applySingleModeUncertaintyGate bypass | `d7-plan-llm-io-protocol-spec.md:395-440` |
| D7-S6-A121-T08 | IMPLEMENTED | §7 Go-side invariants 表 (enum / decompose child_specs / single 清空 / clamp / cap / budget / bypass) | `d7-plan-llm-io-protocol-spec.md:442-455` |
| D7-S6-A121-T09 | IMPLEMENTED | §8 QuantizedKind → MatchKind 4 Rules 路由表 + strengthFloor 公式 | `d7-plan-llm-io-protocol-spec.md:457-471` |
| D7-S6-A121-T10 | IMPLEMENTED | §9 Test 覆盖表 (5 cases → 5 场景映射 + 跨 spec 重叠说明) | `d7-plan-llm-io-protocol-spec.md:473-487` |

## B. Test 补充（P0）

| T ID | Status | Description | Evidence |
|------|--------|-------------|----------|
| D7-S6-A121-T11 | PLANNED | TestPlanTraceE2E_FrameStructure_18Fields (场景 0 输入) | `internal/layers/orchestration/sessionorchestrator/plan_trace_e2e_test.go` (NEW) |
| D7-S6-A121-T12 | PLANNED | TestPlanTraceE2E_JSONSchema_ExecutionModeEnum (场景 0 输出) | 同上 |
| D7-S6-A121-T13 | PLANNED | TestPlanTraceE2E_SingleMode_CommitmentPlan (场景 1) | 同上 |
| D7-S6-A121-T14 | PLANNED | TestPlanTraceE2E_DecomposeMode_ProtocolPlan (场景 2) | 同上 |
| D7-S6-A121-T15 | PLANNED | TestPlanTraceE2E_SingleModeFastPathBypass (场景 5 混合) | 同上 |

> 注: 场景 3 (parallel_probe → ScenarioPlan) 和场景 4 (decompose + anomalies≥3 → ExplorationPlan) 已被 `plan_test.go::TestMatchKind_4Rules` 覆盖 (lines 306-364), 不重复添加。

## C. 验证（P0）

| T ID | Status | Description | Evidence |
|------|--------|-------------|----------|
| D7-S6-A121-T16 | PLANNED | `go test -v -run TestPlanTraceE2E ./internal/layers/orchestration/sessionorchestrator/...` 5/5 PASS | TBD |
| D7-S6-A121-T17 | PLANNED | `go test -race ./internal/layers/orchestration/...` 22/22 packages PASS (回归) | TBD |
| D7-S6-A121-T18 | PLANNED | `go vet ./...` 0 warning | TBD |
| D7-S6-A121-T19 | PLANNED | PR 提交 | TBD |

**Total**: 19 T-points (10 spec sections + 5 test cases + 4 verification).
